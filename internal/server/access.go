package server

import (
	"context"
	"net/http"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

// Who may do what with a capsule.
//
//   - The maker (owner) and the people in it (members: contributors and
//     viewers) reach it; anyone else gets 404 — box admins included.
//   - While it is being filled, you see what you put in yourself; the maker
//     also sees what others added unless it is a surprise capsule.
//   - Once sealed, nobody sees anything inside — not even the maker — until
//     it is opened. Only how many things of each kind, and from whom.
//   - Opened: everyone in it sees everything.

type access struct {
	c    *store.Capsule
	u    *store.User
	role string // owner | contributor | viewer
}

func (a *access) owner() bool { return a.role == "owner" }

// canAdd: put something in (filling: maker and contributors; sealed: also
// through the slot when it is open).
func (a *access) canAdd() (bool, string) {
	if a.role == "viewer" {
		return false, "you can open this capsule with the others, not add to it"
	}
	switch a.c.State {
	case store.StateFilling:
		return true, ""
	case store.StateSealed:
		if a.c.Slot {
			return true, ""
		}
		return false, "this capsule is sealed"
	}
	return false, "this capsule is already open"
}

// mayOpen: the day came (or the maker breaks the seal, which is another path).
func (a *access) mayOpen(now time.Time) (bool, string) {
	if a.c.State == store.StateOpened {
		return false, "it is already open"
	}
	if a.c.State != store.StateSealed {
		return false, "seal it first"
	}
	if !a.c.Ready(now) {
		return false, "it opens on " + a.c.Opens().Local().Format("2 January 2006 at 15:04")
	}
	return true, ""
}

// seesItem says whether the person sees what is inside an item.
func (a *access) seesItem(it *store.Item) bool {
	switch a.c.State {
	case store.StateOpened:
		return true
	case store.StateFilling:
		if it.UserID != nil && *it.UserID == a.u.ID {
			return true
		}
		return a.owner() && !a.c.Surprise
	}
	return false
}

// mayEditItem: its author while the capsule is being filled; after the
// opening, its author or the maker may add the outcome or an answer.
func (a *access) mayEditItem(ctx context.Context, it *store.Item) bool {
	if !mayChange(ctx, it.Source) {
		return false
	}
	mine := it.UserID != nil && *it.UserID == a.u.ID
	switch a.c.State {
	case store.StateFilling:
		return mine || (a.owner() && !a.c.Surprise && it.Guest)
	case store.StateOpened:
		return mine || a.owner()
	}
	return false
}

// mayRemoveItem: its author while filling; the maker can take out a guest's
// letter or anything while it is not a surprise.
func (a *access) mayRemoveItem(ctx context.Context, it *store.Item) bool {
	if !mayChange(ctx, it.Source) || a.c.State != store.StateFilling {
		return false
	}
	mine := it.UserID != nil && *it.UserID == a.u.ID
	return mine || (a.owner() && (it.Guest || !a.c.Surprise))
}

// capsuleAccess finds a capsule the person may reach.
func (s *Server) capsuleAccess(ctx context.Context, u *store.User, id int64) (*access, error) {
	if u == nil {
		return nil, store.ErrNotFound
	}
	c, err := s.store.GetCapsule(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.UserID == u.ID {
		return &access{c: c, u: u, role: "owner"}, nil
	}
	if role := s.store.Membership(ctx, c.ID, u.ID); role != "" {
		return &access{c: c, u: u, role: role}, nil
	}
	return nil, store.ErrNotFound
}

func (s *Server) capsuleParam(w http.ResponseWriter, r *http.Request) (*access, bool) {
	a, err := s.capsuleAccess(r.Context(), userFrom(r.Context()), idParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return nil, false
	}
	return a, true
}

func (s *Server) ownerParam(w http.ResponseWriter, r *http.Request) (*access, bool) {
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return nil, false
	}
	if !a.owner() {
		writeErr(w, http.StatusForbidden, "only "+firstNonEmpty(a.c.OwnerName, "its maker")+" can do that")
		return nil, false
	}
	return a, true
}

// itemParam finds an item and its capsule.
func (s *Server) itemParam(w http.ResponseWriter, r *http.Request) (*access, *store.Item, bool) {
	ctx := r.Context()
	it, err := s.store.GetItem(ctx, idParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return nil, nil, false
	}
	a, err := s.capsuleAccess(ctx, userFrom(ctx), it.CapsuleID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return nil, nil, false
	}
	return a, it, true
}

func (s *Server) actorName(u *store.User) string {
	if u == nil {
		return "Someone"
	}
	return firstNonEmpty(u.Name, u.Email)
}

func uidPtr(u *store.User) *int64 {
	if u == nil {
		return nil
	}
	id := u.ID
	return &id
}
