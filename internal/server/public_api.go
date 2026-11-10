package server

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

// Pages that work without an account:
//
//   - /o/<key>: one recipient's own link. Before the opening it shows the
//     outside of the capsule and how long is left; once it opens (or on the
//     day, for a capsule set to go out then) everything inside.
//   - /give/<key>: the maker's "add something" link. Anyone with it can put
//     a letter and a photo in — and, like everyone else, see none of it
//     until the opening. Ten a hour per address.

func (s *Server) outside(c *store.Capsule) map[string]any {
	return map[string]any{"title": c.Title, "about": c.About, "emoji": c.Emoji, "color": c.Color, "owner_name": c.OwnerName, "state": c.State,
		"opens_at": c.OpensAt, "opens_in": untilWords(c.Opens(), now()), "sealed_at": c.SealedAt, "opened_at": c.OpenedAt, "for_name": c.ForName, "items": c.Items, "template": c.Template}
}

func (s *Server) handlePublicCapsule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rc, err := s.store.RecipientByKey(ctx, chi.URLParam(r, "key"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "this link does not work (any more)")
		return
	}
	c, err := s.store.GetCapsule(ctx, rc.CapsuleID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "this link does not work (any more)")
		return
	}
	out := map[string]any{"capsule": s.outside(c), "name": rc.Name, "open": false}
	if recipientMayOpen(c) {
		items, _ := s.store.Items(ctx, c.ID)
		for i := range items {
			items[i].UserID = nil // nobody's account number leaves the box
		}
		out["open"], out["items"] = true, items
		out["reflection"], out["then_now"] = c.Reflection, c.ThenNow
		if m := mapOfRaw(c.Snapshot); m != nil {
			out["snapshot"] = m
		}
		_ = s.store.MarkRecipient(ctx, rc.ID, "viewed")
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePublicUpload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rc, err := s.store.RecipientByKey(ctx, chi.URLParam(r, "key"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	c, err := s.store.GetCapsule(ctx, rc.CapsuleID)
	if err != nil || !recipientMayOpen(c) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	up, err := s.store.GetUpload(ctx, idParam(r, "id"))
	if err != nil || up.CapsuleID != c.ID {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	s.serveUpload(w, r, up)
}

// ---- the "add something" link ----

func (s *Server) giveCapsule(w http.ResponseWriter, r *http.Request) (*store.Capsule, bool) {
	c, err := s.store.CapsuleByGiveKey(r.Context(), chi.URLParam(r, "key"))
	if err != nil || !c.GiveOpen {
		writeErr(w, http.StatusNotFound, "this link does not work (any more)")
		return nil, false
	}
	return c, true
}

func (s *Server) handleGivePage(w http.ResponseWriter, r *http.Request) {
	c, ok := s.giveCapsule(w, r)
	if !ok {
		return
	}
	t := templateByID(c.Template)
	open := c.State == store.StateFilling || (c.State == store.StateSealed && c.Slot)
	writeJSON(w, http.StatusOK, map[string]any{"capsule": s.outside(c), "accepting": open, "prompts": t.Prompts})
}

// guestAllowed: ten things an hour from one address.
func (s *Server) guestAllowed(r *http.Request) bool {
	ip := r.RemoteAddr
	if h, _, err := net.SplitHostPort(ip); err == nil {
		ip = h
	}
	s.guestMu.Lock()
	defer s.guestMu.Unlock()
	cut := time.Now().Add(-time.Hour)
	keep := []time.Time{}
	for _, t := range s.guestHits[ip] {
		if t.After(cut) {
			keep = append(keep, t)
		}
	}
	if len(keep) >= 10 {
		s.guestHits[ip] = keep
		return false
	}
	s.guestHits[ip] = append(keep, time.Now())
	return true
}

// handleGiveAdd takes a letter (and a photo) from someone without an account.
func (s *Server) handleGiveAdd(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c, ok := s.giveCapsule(w, r)
	if !ok {
		return
	}
	if !(c.State == store.StateFilling || (c.State == store.StateSealed && c.Slot)) {
		writeErr(w, http.StatusBadRequest, "this capsule does not take anything new")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+(1<<20))
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		writeErr(w, http.StatusBadRequest, "a photo can be up to 25 MB")
		return
	}
	name := cleanText(r.FormValue("name"), maxName)
	letter := clip(strings.TrimSpace(r.FormValue("letter")), 20000)
	if name == "" {
		writeErr(w, http.StatusBadRequest, "say who it is from")
		return
	}
	file, hdr, ferr := r.FormFile("photo")
	if letter == "" && ferr != nil {
		writeErr(w, http.StatusBadRequest, "write a letter or add a photo")
		return
	}
	if !s.guestAllowed(r) {
		writeErr(w, http.StatusTooManyRequests, "that is a lot at once: try again in an hour")
		return
	}
	if c.Items >= 500 {
		writeErr(w, http.StatusBadRequest, "this capsule is full")
		return
	}
	after := c.State == store.StateSealed
	added := []string{}
	if letter != "" {
		it := &store.Item{CapsuleID: c.ID, AuthorName: name, Kind: "letter", Title: cleanText(r.FormValue("title"), 200), Body: letter, Guest: true, AfterSeal: after}
		if err := s.store.CreateItem(ctx, it); err != nil {
			s.fail(w, err, "letter")
			return
		}
		s.afterAdded(ctx, c, it, nil)
		added = append(added, "letter")
	}
	if ferr == nil {
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxUpload+1))
		if err != nil || len(data) > maxUpload {
			writeErr(w, http.StatusBadRequest, "a photo can be up to 25 MB")
			return
		}
		mime := strings.Split(http.DetectContentType(data), ";")[0]
		if !strings.HasPrefix(mime, "image/") {
			writeErr(w, http.StatusBadRequest, "that is not a picture")
			return
		}
		up := &store.Upload{UserID: c.UserID, CapsuleID: c.ID, Name: safeName(hdr.Filename), Mime: mime, Data: data}
		up.Width, _ = strconv.Atoi(r.FormValue("width"))
		up.Height, _ = strconv.Atoi(r.FormValue("height"))
		if tf, _, err := r.FormFile("thumb"); err == nil {
			up.Thumb, _ = io.ReadAll(io.LimitReader(tf, 1<<20))
			tf.Close()
		}
		if err := s.store.SaveUpload(ctx, up); err != nil {
			s.fail(w, err, "photo")
			return
		}
		it := &store.Item{CapsuleID: c.ID, AuthorName: name, Kind: "photo", Body: cleanText(r.FormValue("caption"), 2000), UploadID: &up.ID, Guest: true, AfterSeal: after}
		if err := s.store.CreateItem(ctx, it); err != nil {
			s.fail(w, err, "photo")
			return
		}
		s.afterAdded(ctx, c, it, nil)
		added = append(added, "photo")
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "added": added, "opens_at": c.OpensAt})
}
