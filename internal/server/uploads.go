package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

const maxUpload = 25 << 20

var allowedMime = map[string]bool{"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true, "application/pdf": true, "text/plain": true,
	"application/zip": true, "text/csv": true, "audio/mpeg": true, "audio/mp4": true, "audio/webm": true, "audio/ogg": true, "audio/wav": true, "audio/x-wav": true, "audio/aac": true,
	"video/mp4": true, "video/webm": true, "video/quicktime": true, "image/heic": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": true,
	"application/vnd.oasis.opendocument.text": true, "application/msword": true, "application/vnd.ms-excel": true}

// handleUpload stores a picture (shrunk by the browser), a recording or a
// file for a capsule the person may add to.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+(1<<20))
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		writeErr(w, http.StatusBadRequest, "a file can be up to 25 MB")
		return
	}
	up := &store.Upload{UserID: u.ID}
	id, _ := strconv.ParseInt(r.FormValue("capsule"), 10, 64)
	a, err := s.capsuleAccess(ctx, u, id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if ok, why := a.canAdd(); !ok {
		writeErr(w, http.StatusForbidden, why)
		return
	}
	up.CapsuleID = a.c.ID
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "no file")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxUpload+1))
	if err != nil || len(data) > maxUpload {
		writeErr(w, http.StatusBadRequest, "a file can be up to 25 MB")
		return
	}
	mime := http.DetectContentType(data)
	if i := strings.Index(mime, ";"); i > 0 {
		mime = mime[:i]
	}
	if mime == "application/octet-stream" || mime == "text/plain" || mime == "application/zip" {
		if guess := mimeByExt(hdr.Filename); guess != "" {
			mime = guess
		}
	}
	if !allowedMime[mime] {
		writeErr(w, http.StatusBadRequest, "pictures, recordings, videos, PDFs, documents, spreadsheets, text and zip files only")
		return
	}
	up.Mime, up.Data, up.Name = mime, data, safeName(hdr.Filename)
	up.Width, _ = strconv.Atoi(r.FormValue("width"))
	up.Height, _ = strconv.Atoi(r.FormValue("height"))
	if tf, _, err := r.FormFile("thumb"); err == nil {
		up.Thumb, _ = io.ReadAll(io.LimitReader(tf, 1<<20))
		tf.Close()
	}
	if err := s.store.SaveUpload(ctx, up); err != nil {
		s.fail(w, err, "upload")
		return
	}
	url := fmt.Sprintf("/api/v1/uploads/%d", up.ID)
	md := fmt.Sprintf("[%s](%s)", up.Name, url)
	if strings.HasPrefix(mime, "image/") {
		md = fmt.Sprintf("![%s](%s)", strings.TrimSuffix(up.Name, filepath.Ext(up.Name)), url)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"upload": up, "url": url, "markdown": md})
}

func mimeByExt(name string) string {
	return map[string]string{".pdf": "application/pdf", ".txt": "text/plain", ".md": "text/plain", ".csv": "text/csv", ".zip": "application/zip", ".mp3": "audio/mpeg",
		".m4a": "audio/mp4", ".mp4": "video/mp4", ".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".odt": "application/vnd.oasis.opendocument.text", ".doc": "application/msword", ".xls": "application/vnd.ms-excel"}[strings.ToLower(filepath.Ext(name))]
}

func safeName(n string) string {
	n = filepath.Base(strings.ReplaceAll(n, "\\", "/"))
	n = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*[]()`, r) {
			return '_'
		}
		return r
	}, n)
	return firstNonEmpty(clip(strings.TrimSpace(n), 120), "file")
}

// mayGetUpload: someone in its capsule who may see the item it belongs to
// (nobody while the capsule is sealed — not even whoever put it up); a file
// no item uses yet only to whoever put it up, while the capsule is open to
// changes.
func (s *Server) mayGetUpload(ctx context.Context, u *store.User, up *store.Upload) bool {
	if u == nil || up.CapsuleID == 0 {
		return u != nil && up.UserID == u.ID
	}
	a, err := s.capsuleAccess(ctx, u, up.CapsuleID)
	if err != nil {
		return false
	}
	items, _ := s.store.Items(ctx, a.c.ID)
	used := false
	for _, it := range items {
		if it.UploadID != nil && *it.UploadID == up.ID {
			used = true
			if a.seesItem(&it) {
				return true
			}
		}
	}
	return !used && up.UserID == u.ID && a.c.State == store.StateFilling
}

func (s *Server) serveUpload(w http.ResponseWriter, r *http.Request, up *store.Upload) {
	data := up.Data
	if queryBool(r, "thumb") && len(up.Thumb) > 0 {
		data = up.Thumb
	}
	w.Header().Set("Content-Type", up.Mime)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	disp := "inline"
	if !strings.HasPrefix(up.Mime, "image/") && up.Mime != "application/pdf" && !strings.HasPrefix(up.Mime, "audio/") && !strings.HasPrefix(up.Mime, "video/") {
		disp = "attachment"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disp, strings.ReplaceAll(up.Name, `"`, "")))
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	http.ServeContent(w, r, up.Name, time.Time{}, bytes.NewReader(data))
}

func (s *Server) handleGetUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	up, err := s.store.GetUpload(ctx, idParam(r, "id"))
	if err != nil || !s.mayGetUpload(ctx, userFrom(ctx), up) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	s.serveUpload(w, r, up)
}
