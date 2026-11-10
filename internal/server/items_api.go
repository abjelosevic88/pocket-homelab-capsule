package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

type itemIn struct {
	Kind       string        `json:"kind"`
	Title      *string       `json:"title"`
	Body       *string       `json:"body"`
	UploadID   *int64        `json:"upload_id"`
	Confidence *int          `json:"confidence"`
	Outcome    *string       `json:"outcome"`
	Answer     *string       `json:"answer"`
	Entries    []store.Entry `json:"entries"`
	URL        *string       `json:"url"`
	FromApp    string        `json:"from_app"`
	FromURL    string        `json:"from_url"`
}

// needsUpload are the kinds that are a file.
var needsUpload = map[string]bool{"photo": true, "audio": true, "video": true, "file": true}

// applyItem checks what was sent and copies the parts that may change now:
// the contents while the capsule is being filled, the outcome and answer
// after it opened.
func (s *Server) applyItem(ctx context.Context, a *access, it *store.Item, in *itemIn, uid int64) error {
	filling := a.c.State != store.StateOpened
	if filling {
		if in.Title != nil {
			it.Title = cleanText(*in.Title, 200)
		}
		if in.Body != nil {
			it.Body = clip(strings.TrimSpace(*in.Body), maxNotes)
		}
		if in.Confidence != nil {
			it.Confidence = clampInt(*in.Confidence, 0, 100)
		}
		if in.URL != nil {
			v, ok := cleanURL(*in.URL)
			if !ok {
				return &userErr{"a link starts with http:// or https://"}
			}
			it.URL = v
		}
		if in.Entries != nil {
			out := []store.Entry{}
			for _, e := range in.Entries {
				l, v := cleanText(e.Label, 120), cleanText(e.Value, 600)
				if (l != "" || v != "") && len(out) < 60 {
					out = append(out, store.Entry{Label: l, Value: v})
				}
			}
			it.Entries = out
		}
		if in.UploadID != nil {
			if *in.UploadID == 0 {
				it.UploadID = nil
			} else {
				up, err := s.store.GetUpload(ctx, *in.UploadID)
				if err != nil || up.UserID != uid || (up.CapsuleID != 0 && up.CapsuleID != a.c.ID) {
					return &userErr{"that file is not yours to add"}
				}
				it.UploadID = &up.ID
			}
		}
	} else {
		if in.Outcome != nil {
			if !oneOf(*in.Outcome, []string{"", "yes", "partly", "no"}) {
				return &userErr{"it came true: yes, partly or no"}
			}
			it.Outcome = *in.Outcome
		}
		if in.Answer != nil {
			it.Answer = clip(strings.TrimSpace(*in.Answer), maxNotes)
		}
	}
	// what each kind needs
	switch it.Kind {
	case "letter":
		if it.Body == "" {
			return &userErr{"write something first"}
		}
	case "prediction", "question", "goal":
		if it.Body == "" {
			return &userErr{map[string]string{"prediction": "write down your guess", "question": "write down the question", "goal": "write down the goal"}[it.Kind]}
		}
	case "list":
		if len(it.Entries) == 0 {
			return &userErr{"fill in at least one line"}
		}
	case "link":
		if it.URL == "" {
			return &userErr{"add the link"}
		}
	default:
		if needsUpload[it.Kind] && it.UploadID == nil {
			return &userErr{"add the file"}
		}
	}
	return nil
}

func (s *Server) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return
	}
	if can, why := a.canAdd(); !can {
		writeErr(w, http.StatusBadRequest, why)
		return
	}
	var in itemIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !oneOf(in.Kind, store.ItemKinds) {
		writeErr(w, http.StatusBadRequest, "unknown kind of thing")
		return
	}
	if a.c.Items >= 500 {
		writeErr(w, http.StatusBadRequest, "a capsule holds up to 500 things")
		return
	}
	it := &store.Item{CapsuleID: a.c.ID, UserID: uidPtr(a.u), AuthorName: s.actorName(a.u), Kind: in.Kind, AfterSeal: a.c.State == store.StateSealed,
		FromApp: clip(strings.TrimSpace(in.FromApp), 40), Source: sourceOf(ctx)}
	if v, ok := cleanURL(in.FromURL); ok {
		it.FromURL = v
	}
	if it.Source != "" && it.FromApp == "" {
		it.FromApp = it.Source
	}
	if err := s.applyItem(ctx, a, it, &in, a.u.ID); err != nil {
		s.userFail(w, err, "item")
		return
	}
	if err := s.store.CreateItem(ctx, it); err != nil {
		s.fail(w, err, "item")
		return
	}
	s.afterAdded(ctx, a.c, it, a.u)
	if !a.seesItem(it) {
		// dropped through the slot: it went in, and that is all anyone sees now
		writeJSON(w, http.StatusCreated, map[string]any{"id": it.ID, "kind": it.Kind, "hidden": true})
		return
	}
	writeJSON(w, http.StatusCreated, it)
}

// afterAdded records and tells the maker when someone else put something in.
func (s *Server) afterAdded(ctx context.Context, c *store.Capsule, it *store.Item, u *store.User) {
	detail := kindWord(it.Kind)
	if it.AfterSeal {
		detail += " through the slot"
	}
	s.store.Log(ctx, c.ID, uidPtr(u), it.AuthorName, "added", detail)
	if u == nil || u.ID != c.UserID {
		st := s.settingsOf(ctx, c.UserID)
		if st.NotifyAdded {
			s.push(ctx, c.UserID, st, it.AuthorName+" added "+kindArticle(it.Kind)+" to “"+c.Title+"”", "You will see it when the capsule opens.", fmt.Sprintf("/c/%d", c.ID))
		}
	}
	s.publish(ctx, c.UserID, "capsule.item_added", c.Title, map[string]any{"capsule_id": c.ID, "title": c.Title, "kind": it.Kind, "by": it.AuthorName, "after_seal": it.AfterSeal})
}

func kindWord(k string) string {
	return map[string]string{"letter": "a letter", "photo": "a photo", "audio": "a recording", "video": "a video", "file": "a file", "prediction": "a prediction",
		"question": "a question", "goal": "a goal", "list": "a list", "link": "a link"}[k]
}

func kindArticle(k string) string { return firstNonEmpty(kindWord(k), "something") }

func (s *Server) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, it, ok := s.itemParam(w, r)
	if !ok {
		return
	}
	if !a.seesItem(it) || !a.mayEditItem(ctx, it) {
		writeErr(w, http.StatusForbidden, "you cannot change this now")
		return
	}
	var in itemIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.applyItem(ctx, a, it, &in, a.u.ID); err != nil {
		s.userFail(w, err, "item")
		return
	}
	if err := s.store.UpdateItem(ctx, it); err != nil {
		s.fail(w, err, "item")
		return
	}
	it, _ = s.store.GetItem(ctx, it.ID)
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, it, ok := s.itemParam(w, r)
	if !ok {
		return
	}
	if !a.mayRemoveItem(ctx, it) {
		writeErr(w, http.StatusForbidden, "you cannot take this out now")
		return
	}
	if err := s.store.DeleteItem(ctx, it.ID); err != nil {
		s.fail(w, err, "item")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReorder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return
	}
	if a.c.State == store.StateSealed || (a.c.State == store.StateFilling && a.c.Surprise && !a.owner()) {
		writeErr(w, http.StatusBadRequest, "the order cannot change now")
		return
	}
	var in struct {
		IDs []int64 `json:"ids"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	// only the items this person sees may move
	items, _ := s.store.Items(ctx, a.c.ID)
	seen := map[int64]bool{}
	for _, it := range items {
		if a.seesItem(&it) {
			seen[it.ID] = true
		}
	}
	ids := []int64{}
	for _, id := range in.IDs {
		if seen[id] {
			ids = append(ids, id)
		}
	}
	if err := s.store.ReorderItems(ctx, a.c.ID, ids); err != nil {
		s.fail(w, err, "order")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
