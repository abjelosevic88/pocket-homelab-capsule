package server

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

// The export is everything a person made: their capsules with what is in
// them, the files, who each is for, the history and the yearly capsules.
// What is inside a sealed capsule is packed (gzip + base64) so that opening
// the file does not spoil it — that is not a lock, just a lid.

type exportFile struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Mime  string `json:"mime"`
	W     int    `json:"width,omitempty"`
	H     int    `json:"height,omitempty"`
	Data  string `json:"data"`
	Thumb string `json:"thumb,omitempty"`
}

type exportInside struct {
	Items []store.Item `json:"items"`
	Files []exportFile `json:"files"`
}

type exportCapsule struct {
	Capsule    store.Capsule     `json:"capsule"`
	Inside     *exportInside     `json:"inside,omitempty"`
	Sealed     string            `json:"sealed,omitempty"` // packed inside, while sealed
	Recipients []store.Recipient `json:"recipients"`
	Members    []string          `json:"members"`
	History    []store.Event     `json:"history"`
}

type exportDoc struct {
	App        string          `json:"app"`
	Version    string          `json:"version"`
	ExportedAt string          `json:"exported_at"`
	Capsules   []exportCapsule `json:"capsules"`
	Rituals    []store.Ritual  `json:"rituals"`
}

func pack(v any) string {
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	_ = json.NewEncoder(zw).Encode(v)
	_ = zw.Close()
	return base64.StdEncoding.EncodeToString(b.Bytes())
}

func unpack(s string, v any) error {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	defer zr.Close()
	return json.NewDecoder(io.LimitReader(zr, 2<<30)).Decode(v)
}

func (s *Server) export(ctx context.Context, u *store.User) (*exportDoc, error) {
	list, err := s.store.OwnCapsules(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	doc := &exportDoc{App: "pocket-capsule", Version: Version, ExportedAt: store.Now(), Capsules: []exportCapsule{}}
	for _, c := range list {
		items, _ := s.store.Items(ctx, c.ID)
		in := &exportInside{Items: items, Files: []exportFile{}}
		for _, it := range items {
			if it.UploadID == nil {
				continue
			}
			up, err := s.store.GetUpload(ctx, *it.UploadID)
			if err != nil {
				continue
			}
			f := exportFile{ID: up.ID, Name: up.Name, Mime: up.Mime, W: up.Width, H: up.Height, Data: base64.StdEncoding.EncodeToString(up.Data)}
			if len(up.Thumb) > 0 {
				f.Thumb = base64.StdEncoding.EncodeToString(up.Thumb)
			}
			in.Files = append(in.Files, f)
		}
		ec := exportCapsule{Capsule: c, Recipients: []store.Recipient{}, Members: []string{}}
		if c.State == store.StateOpened {
			ec.Inside = in
		} else {
			ec.Sealed = pack(in)
		}
		ec.Recipients, _ = s.store.Recipients(ctx, c.ID)
		members, _ := s.store.Members(ctx, c.ID)
		for _, m := range members {
			ec.Members = append(ec.Members, m.Email+" "+m.Role)
		}
		ec.History, _ = s.store.History(ctx, c.ID)
		doc.Capsules = append(doc.Capsules, ec)
	}
	doc.Rituals, _ = s.store.Rituals(ctx, u.ID)
	return doc, nil
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	doc, err := s.export(r.Context(), userFrom(r.Context()))
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-capsule-%s.json"`, time.Now().Format("2006-01-02")))
	_ = json.NewEncoder(w).Encode(doc)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	list, err := s.store.CapsulesFor(ctx, actorID(ctx))
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-capsule-%s.csv"`, time.Now().Format("2006-01-02")))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "title", "state", "made_by", "opens", "sealed", "opened", "things_inside", "people_in_it", "about"})
	day := func(p *string) string {
		if p == nil {
			return ""
		}
		t, _ := time.Parse(time.RFC3339Nano, *p)
		return t.Local().Format("2006-01-02")
	}
	for _, c := range list {
		_ = cw.Write([]string{strconv.FormatInt(c.ID, 10), c.Title, c.State, c.OwnerName, c.Opens().Local().Format("2006-01-02 15:04"), day(c.SealedAt), day(c.OpenedAt),
			strconv.Itoa(c.Items), strconv.Itoa(c.Members), c.About})
	}
	cw.Flush()
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var doc exportDoc
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<30)).Decode(&doc); err != nil {
		writeErr(w, http.StatusBadRequest, "that is not a Pocket Capsule export")
		return
	}
	if doc.App != "pocket-capsule" {
		writeErr(w, http.StatusBadRequest, "that is not a Pocket Capsule export")
		return
	}
	res, err := s.importDoc(ctx, userFrom(ctx), &doc)
	if err != nil {
		s.userFail(w, err, "import")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// importDoc adds what is not here yet: a capsule imported before (same
// origin id) is skipped, so importing twice changes nothing.
func (s *Server) importDoc(ctx context.Context, u *store.User, doc *exportDoc) (map[string]any, error) {
	made, skipped, things := 0, 0, 0
	for _, ec := range doc.Capsules {
		c := ec.Capsule
		ext := firstNonEmpty(c.ExtID, fmt.Sprintf("import:%d:%s", c.ID, c.CreatedAt))
		if _, err := s.store.CapsuleByExt(ctx, u.ID, ext); err == nil {
			skipped++
			continue
		}
		in := ec.Inside
		if in == nil && ec.Sealed != "" {
			in = &exportInside{}
			if err := unpack(ec.Sealed, in); err != nil {
				return nil, &userErr{"a sealed capsule in the file is damaged: " + c.Title}
			}
		}
		if in == nil {
			in = &exportInside{}
		}
		old := c.ID
		c.ID, c.UserID, c.ExtID, c.GiveKey, c.GiveOpen, c.ReplyTo, c.OpenedBy, c.Source = 0, u.ID, ext, "", false, nil, nil, ""
		if c.Title == "" || c.OpensAt == "" || !oneOf(c.State, []string{store.StateFilling, store.StateSealed, store.StateOpened}) {
			return nil, &userErr{fmt.Sprintf("capsule %d in the file is incomplete", old)}
		}
		if err := s.store.CreateCapsule(ctx, &c); err != nil {
			return nil, err
		}
		files := map[int64]int64{}
		for _, f := range in.Files {
			data, err := base64.StdEncoding.DecodeString(f.Data)
			if err != nil {
				continue
			}
			up := &store.Upload{UserID: u.ID, CapsuleID: c.ID, Name: safeName(f.Name), Mime: f.Mime, Width: f.W, Height: f.H, Data: data}
			if f.Thumb != "" {
				up.Thumb, _ = base64.StdEncoding.DecodeString(f.Thumb)
			}
			if err := s.store.SaveUpload(ctx, up); err == nil {
				files[f.ID] = up.ID
			}
		}
		for _, it := range in.Items {
			it.ID, it.CapsuleID, it.Source = 0, c.ID, ""
			if it.UserID != nil {
				it.UserID = uidPtr(u)
				if it.AuthorName == "" {
					it.AuthorName = s.actorName(u)
				}
			}
			if it.UploadID != nil {
				if nid, ok := files[*it.UploadID]; ok {
					it.UploadID = &nid
				} else {
					it.UploadID = nil
				}
			}
			if !oneOf(it.Kind, store.ItemKinds) {
				continue
			}
			if err := s.store.CreateItem(ctx, &it); err == nil {
				things++
			}
		}
		for _, rc := range ec.Recipients {
			_ = s.store.AddRecipient(ctx, &store.Recipient{UserID: u.ID, CapsuleID: c.ID, Name: rc.Name, Email: rc.Email})
		}
		for i := len(ec.History) - 1; i >= 0; i-- {
			e := ec.History[i]
			s.store.LogAt(ctx, c.ID, nil, e.Who, e.What, e.Detail, e.At)
		}
		made++
	}
	for _, rt := range doc.Rituals {
		have, _ := s.store.Rituals(ctx, u.ID)
		dup := false
		for _, h := range have {
			if h.Title == rt.Title && h.Month == rt.Month && h.Day == rt.Day {
				dup = true
			}
		}
		if !dup && rt.Month >= 1 && rt.Month <= 12 && rt.Day >= 1 && rt.Day <= 31 {
			rt.ID, rt.UserID = 0, u.ID
			_ = s.store.SaveRitual(ctx, &rt)
		}
	}
	return map[string]any{"capsules": made, "skipped": skipped, "items": things}, nil
}
