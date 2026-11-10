package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

// now is the clock (tests move it).
var now = time.Now

// ---- the opening day ----

// parseOpens reads a day and time: RFC 3339, local "YYYY-MM-DDTHH:MM", or a
// bare date (at the person's opening hour).
func parseOpens(v string, hour int) (time.Time, bool) {
	v = strings.TrimSpace(v)
	if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", v, time.Local); err == nil {
		return t, true
	}
	if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
		return t.Add(time.Duration(hour) * time.Hour), true
	}
	return time.Time{}, false
}

// ruleOpens is the day for "on their Nth birthday / anniversary"; age 0 =
// the next one to come.
func ruleOpens(forDate string, age, hour int, from time.Time) (time.Time, bool) {
	d, err := time.ParseInLocation("2006-01-02", forDate, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	at := func(y int) time.Time {
		return time.Date(y, d.Month(), d.Day(), hour, 0, 0, 0, time.Local)
	}
	if age > 0 {
		return at(d.Year() + age), true
	}
	t := at(from.Year())
	if !t.After(from) {
		t = at(from.Year() + 1)
	}
	return t, true
}

// ---- what a person sees of a capsule ----

type kindCount struct {
	Kind string `json:"kind"`
	N    int    `json:"n"`
}

// capsuleView is a capsule as one person may see it.
func (s *Server) capsuleView(ctx context.Context, a *access, full bool) map[string]any {
	c := a.c
	t := now()
	out := map[string]any{
		"capsule": c, "role": a.role, "ready": c.Ready(t), "opens_in": opensIn(c, t),
		"url": s.link(fmt.Sprintf("/c/%d", c.ID)),
	}
	if !full {
		return out
	}
	items, _ := s.store.Items(ctx, c.ID)
	visible := []store.Item{}
	kinds := map[string]int{}
	authors := map[string]int{}
	hidden := 0
	for _, it := range items {
		kinds[it.Kind]++
		authors[firstNonEmpty(it.AuthorName, "Someone")]++
		if a.seesItem(&it) {
			visible = append(visible, it)
		} else {
			hidden++
		}
	}
	out["items"] = visible
	out["hidden"] = hidden
	byKind := []kindCount{}
	for k, n := range kinds {
		byKind = append(byKind, kindCount{k, n})
	}
	sort.Slice(byKind, func(i, j int) bool {
		return byKind[i].N > byKind[j].N || (byKind[i].N == byKind[j].N && byKind[i].Kind < byKind[j].Kind)
	})
	out["kinds"] = byKind
	from := []map[string]any{}
	for name, n := range authors {
		from = append(from, map[string]any{"name": name, "n": n})
	}
	sort.Slice(from, func(i, j int) bool { return from[i]["name"].(string) < from[j]["name"].(string) })
	out["from"] = from
	members, _ := s.store.Members(ctx, c.ID)
	out["members"] = members
	if a.owner() {
		rec, _ := s.store.Recipients(ctx, c.ID)
		links := []map[string]any{}
		for _, r := range rec {
			links = append(links, map[string]any{"id": r.ID, "name": r.Name, "email": r.Email, "sent_at": r.SentAt, "viewed_at": r.ViewedAt, "url": s.link("/o/" + r.Key)})
		}
		out["recipients"] = links
		if c.GiveOpen {
			out["give_url"] = s.link("/give/" + c.GiveKey)
		}
	} else {
		out["recipients"] = []any{}
	}
	if c.State != store.StateOpened {
		// the world today is kept at sealing and shown at the opening
		c2 := *c
		c2.Snapshot = nil
		out["capsule"] = &c2
	}
	can, why := a.canAdd()
	out["can_add"], out["why_not_add"] = can, why
	open, whyOpen := a.mayOpen(t)
	out["can_open"], out["why_not_open"] = open, whyOpen
	out["template"] = templateByID(c.Template)
	return out
}

func opensIn(c *store.Capsule, t time.Time) string {
	if c.State == store.StateOpened {
		return ""
	}
	return untilWords(c.Opens(), t)
}

// untilWords: "in 2 years, 3 months", "tomorrow", "in 5 hours", "" when past.
func untilWords(at, from time.Time) string {
	if !at.After(from) {
		return ""
	}
	d := at.Sub(from)
	if d < time.Hour {
		m := int(d.Minutes()) + 1
		return fmt.Sprintf("in %d %s", m, plural(m, "minute", "minutes"))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		if h == 0 {
			h = 1
		}
		return fmt.Sprintf("in %d %s", h, plural(h, "hour", "hours"))
	}
	y, m, dd := diffYMD(from, at)
	parts := []string{}
	if y > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", y, plural(y, "year", "years")))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", m, plural(m, "month", "months")))
	}
	if y == 0 && dd > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", dd, plural(dd, "day", "days")))
	}
	if len(parts) == 0 {
		return "tomorrow"
	}
	if len(parts) > 2 {
		parts = parts[:2]
	}
	return "in " + strings.Join(parts, ", ")
}

func diffYMD(a, b time.Time) (y, m, d int) {
	a, b = a.Local(), b.Local()
	y = b.Year() - a.Year()
	m = int(b.Month()) - int(a.Month())
	d = b.Day() - a.Day()
	if d < 0 {
		m--
		d += time.Date(b.Year(), b.Month(), 0, 0, 0, 0, 0, time.Local).Day()
	}
	if m < 0 {
		y--
		m += 12
	}
	return
}

// ---- lists ----

func (s *Server) handleCapsules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	list, err := s.store.CapsulesFor(ctx, u.ID)
	if err != nil {
		s.fail(w, err, "capsules")
		return
	}
	state := r.URL.Query().Get("state")
	t := now()
	out := []map[string]any{}
	for i := range list {
		c := &list[i]
		ready := c.Ready(t)
		switch state {
		case "ready":
			if !ready {
				continue
			}
		case store.StateFilling, store.StateOpened:
			if c.State != state {
				continue
			}
		case store.StateSealed:
			if c.State != state || ready {
				continue
			}
		}
		role := "owner"
		if c.UserID != u.ID {
			role = s.store.Membership(ctx, c.ID, u.ID)
		}
		c.Snapshot = nil
		out = append(out, map[string]any{"capsule": c, "role": role, "ready": ready, "opens_in": opensIn(c, t), "url": s.link(fmt.Sprintf("/c/%d", c.ID))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"capsules": out})
}

func (s *Server) handleGetCapsule(w http.ResponseWriter, r *http.Request) {
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.capsuleView(r.Context(), a, true))
}

// ---- making and changing ----

type capsuleIn struct {
	Title      *string  `json:"title"`
	About      *string  `json:"about"`
	Template   *string  `json:"template"`
	Emoji      *string  `json:"emoji"`
	Color      *string  `json:"color"`
	OpensAt    *string  `json:"opens_at"`
	OpenRule   *string  `json:"open_rule"`
	ForName    *string  `json:"for_name"`
	ForDate    *string  `json:"for_date"`
	ForAge     *int     `json:"for_age"`
	Surprise   *bool    `json:"surprise"`
	Slot       *bool    `json:"slot"`
	Deliver    *string  `json:"deliver"`
	RemindDays *int     `json:"remind_days"`
	Sections   []string `json:"sections"`
	ReplyTo    int64    `json:"reply_to"`
	Members    []string `json:"members"`
	Recipients []struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"recipients"`
}

// applyCapsule checks and copies what was sent onto c.
func (s *Server) applyCapsule(c *store.Capsule, in *capsuleIn, keys map[string]bool, st store.Settings, sealed bool) error {
	if in.Title != nil {
		c.Title = cleanText(*in.Title, maxName)
		if c.Title == "" {
			return &userErr{"give the capsule a name"}
		}
	}
	if in.About != nil {
		c.About = cleanText(*in.About, 600)
	}
	if in.Emoji != nil && !sealed {
		c.Emoji = firstNonEmpty(clip(strings.TrimSpace(*in.Emoji), 16), "⏳")
	}
	if in.Color != nil {
		if v := strings.TrimSpace(*in.Color); v == "" || reColor.MatchString(v) {
			c.Color = v
		}
	}
	if in.Template != nil && !sealed {
		c.Template = templateByID(*in.Template).ID
	}
	if in.Surprise != nil && !sealed {
		c.Surprise = *in.Surprise
	}
	if in.Slot != nil {
		c.Slot = *in.Slot
	}
	if in.Deliver != nil {
		if !oneOf(*in.Deliver, []string{"on_open", "on_date"}) {
			return &userErr{"the people it is for get it when it is opened, or on the day"}
		}
		c.Deliver = *in.Deliver
	}
	if in.RemindDays != nil {
		c.RemindDays = clampInt(*in.RemindDays, 0, 365)
		c.RemindSent = false
	}
	if keys["sections"] && !sealed {
		c.Sections = cleanSections(in.Sections)
	}
	// the day it opens
	if in.OpenRule != nil {
		if !oneOf(*in.OpenRule, []string{"date", "birthday", "anniversary"}) {
			return &userErr{"it opens on a date, a birthday or an anniversary"}
		}
		c.OpenRule = *in.OpenRule
	}
	if in.ForName != nil {
		c.ForName = cleanText(*in.ForName, maxName)
	}
	if in.ForDate != nil {
		if !validDate(*in.ForDate) {
			return &userErr{"the birthday or anniversary is YYYY-MM-DD"}
		}
		c.ForDate = *in.ForDate
	}
	if in.ForAge != nil {
		c.ForAge = clampInt(*in.ForAge, 0, 120)
	}
	var opens time.Time
	got := false
	if c.OpenRule != "date" && (in.OpenRule != nil || in.ForDate != nil || in.ForAge != nil) {
		if c.ForDate == "" {
			return &userErr{"add the date of the birthday or anniversary"}
		}
		opens, got = ruleOpens(c.ForDate, c.ForAge, st.OpenHour, now())
	}
	if !got && in.OpensAt != nil {
		var ok bool
		if opens, ok = parseOpens(*in.OpensAt, st.OpenHour); !ok {
			return &userErr{"the opening day is YYYY-MM-DD (and a time)"}
		}
		got = true
	}
	if got {
		if !opens.After(now()) {
			return &userErr{"the opening day has to be in the future"}
		}
		if opens.After(now().AddDate(120, 0, 0)) {
			return &userErr{"pick a day within the next 120 years"}
		}
		if sealed && opens.Before(c.Opens()) {
			return &userErr{"a sealed capsule can wait longer, but not open sooner (break the seal if you have to)"}
		}
		c.OpensAt = opens.UTC().Format(time.RFC3339Nano)
		c.RemindSent = false
	}
	return nil
}

func cleanSections(in []string) []string {
	out := []string{}
	for _, v := range store.Sections {
		if oneOf(v, in) {
			out = append(out, v)
		}
	}
	return out
}

func (s *Server) handleCreateCapsule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	var in capsuleIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st := s.settingsOf(ctx, u.ID)
	t := templateByID(firstNonEmpty(deref(in.Template), "self"))
	c := &store.Capsule{UserID: u.ID, Template: t.ID, Emoji: t.Emoji, Title: t.Name, OpenRule: "date", Deliver: "on_open",
		Surprise: st.Surprise, Slot: st.Slot, RemindDays: st.RemindDays, Sections: append([]string(nil), st.Sections...), Source: sourceOf(ctx)}
	if t.Rule == "birthday" || t.Rule == "anniversary" {
		c.OpenRule = t.Rule
		c.ForAge = t.Years
	}
	if t.Together && in.Surprise == nil {
		c.Surprise = true
	}
	c.OpensAt = defaultOpens(t, st.OpenHour, now()).UTC().Format(time.RFC3339Nano)
	if c.OpenRule != "date" && in.ForDate == nil {
		c.OpenRule = "date" // no date to count from yet: a plain date until one is given
	}
	if err := s.applyCapsule(c, &in, keys, st, false); err != nil {
		s.userFail(w, err, "capsule")
		return
	}
	if in.ReplyTo > 0 {
		if prev, err := s.capsuleAccess(ctx, u, in.ReplyTo); err == nil && prev.c.State == store.StateOpened {
			c.ReplyTo = &prev.c.ID
		}
	}
	if err := s.store.CreateCapsule(ctx, c); err != nil {
		s.fail(w, err, "capsule")
		return
	}
	s.store.Log(ctx, c.ID, uidPtr(u), s.actorName(u), "made", "")
	for _, e := range in.Members {
		_, _ = s.addMember(ctx, &access{c: c, u: u, role: "owner"}, e, "contributor")
	}
	for _, rc := range in.Recipients {
		_, _ = s.addRecipient(ctx, &access{c: c, u: u, role: "owner"}, rc.Name, rc.Email)
	}
	c, _ = s.store.GetCapsule(ctx, c.ID)
	writeJSON(w, http.StatusCreated, s.capsuleView(ctx, &access{c: c, u: u, role: "owner"}, true))
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s *Server) handleUpdateCapsule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	if a.c.State == store.StateOpened {
		var in struct {
			Title *string `json:"title"`
			Color *string `json:"color"`
		}
		if err := decode(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if in.Title != nil && cleanText(*in.Title, maxName) != "" {
			a.c.Title = cleanText(*in.Title, maxName)
		}
		if in.Color != nil && (*in.Color == "" || reColor.MatchString(*in.Color)) {
			a.c.Color = *in.Color
		}
	} else {
		var in capsuleIn
		keys, err := decodeKeys(r, &in)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return
		}
		if err := s.applyCapsule(a.c, &in, keys, s.settingsOf(ctx, a.u.ID), a.c.State == store.StateSealed); err != nil {
			s.userFail(w, err, "capsule")
			return
		}
	}
	if err := s.store.UpdateCapsule(ctx, a.c); err != nil {
		s.fail(w, err, "capsule")
		return
	}
	writeJSON(w, http.StatusOK, s.capsuleView(ctx, a, true))
}

func (s *Server) handleDeleteCapsule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	if a.c.State == store.StateSealed && !queryBool(r, "confirm") {
		writeErr(w, http.StatusBadRequest, "this capsule is sealed: confirm to throw it away unopened")
		return
	}
	if err := s.store.DeleteCapsule(ctx, a.c.ID); err != nil {
		s.fail(w, err, "delete")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- sealing and opening ----

func (s *Server) handleSeal(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	c := a.c
	if c.State != store.StateFilling {
		writeErr(w, http.StatusBadRequest, "it is already sealed")
		return
	}
	if c.Items == 0 {
		writeErr(w, http.StatusBadRequest, "put something in it first")
		return
	}
	if !c.Opens().After(now().Add(time.Minute)) {
		writeErr(w, http.StatusBadRequest, "pick an opening day in the future first")
		return
	}
	// the world today, from the other apps (each has a few seconds)
	if len(c.Sections) > 0 {
		snap := s.takeSnapshot(ctx, a.u, c.Sections)
		c.Snapshot = mustJSON(snap)
	}
	at := store.Now()
	c.State, c.SealedAt = store.StateSealed, &at
	if err := s.store.UpdateCapsule(ctx, c); err != nil {
		s.fail(w, err, "seal")
		return
	}
	s.store.Log(ctx, c.ID, uidPtr(a.u), s.actorName(a.u), "sealed", "opens "+c.Opens().Local().Format("2 Jan 2006"))
	st := s.settingsOf(ctx, a.u.ID)
	calendar := ""
	if st.CalendarOnSeal {
		if url, err := s.toCalendar(ctx, a); err == nil {
			calendar = url
		}
	}
	payload := map[string]any{"capsule_id": c.ID, "title": c.Title, "opens_at": c.OpensAt, "items": c.Items, "url": s.link(fmt.Sprintf("/c/%d", c.ID))}
	s.publish(ctx, a.u.ID, "capsule.sealed", c.Title, payload)
	s.tellMembers(ctx, c, a.u.ID, "notify_opened", "“"+c.Title+"” is sealed", s.actorName(a.u)+" sealed it. It opens "+untilWords(c.Opens(), now())+".", fmt.Sprintf("/c/%d", c.ID))
	out := s.capsuleView(ctx, a, true)
	out["calendar_url"] = calendar
	writeJSON(w, http.StatusOK, out)
}

// openCapsule opens a sealed capsule (on the day, or early) and tells
// everyone in it.
func (s *Server) openCapsule(ctx context.Context, a *access, early bool, reason string) error {
	c := a.c
	at := store.Now()
	c.State, c.OpenedAt, c.OpenedBy = store.StateOpened, &at, uidPtr(a.u)
	if early {
		c.Early, c.EarlyReason = true, clip(strings.TrimSpace(reason), 300)
	}
	if err := s.store.UpdateCapsule(ctx, c); err != nil {
		return err
	}
	what, detail := "opened", ""
	if early {
		what, detail = "early", firstNonEmpty(c.EarlyReason, untilWords(c.Opens(), now())+" too soon")
	}
	s.store.Log(ctx, c.ID, uidPtr(a.u), s.actorName(a.u), what, detail)
	s.publish(ctx, c.UserID, "capsule.opened", c.Title, map[string]any{"capsule_id": c.ID, "title": c.Title, "sealed_at": c.SealedAt, "early": early, "opened_by": s.actorName(a.u), "url": s.link(fmt.Sprintf("/c/%d", c.ID))})
	body := s.actorName(a.u) + " opened it"
	if c.SealedAt != nil {
		if t, err := time.Parse(time.RFC3339Nano, *c.SealedAt); err == nil {
			body += ", sealed on " + t.Local().Format("2 January 2006")
		}
	}
	if early {
		body += " — before its day"
	}
	s.tellMembers(ctx, c, a.u.ID, "notify_opened", "“"+c.Title+"” is open", body+".", fmt.Sprintf("/c/%d", c.ID))
	if c.Deliver == "on_open" || early {
		go s.deliverLater(c.ID)
	}
	return nil
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return
	}
	if ok, why := a.mayOpen(now()); !ok {
		writeErr(w, http.StatusBadRequest, why)
		return
	}
	if err := s.openCapsule(ctx, a, false, ""); err != nil {
		s.fail(w, err, "open")
		return
	}
	writeJSON(w, http.StatusOK, s.capsuleView(ctx, a, true))
}

// handleBreak opens a sealed capsule before its day: only its maker, only
// after typing "open", and it stays on record.
func (s *Server) handleBreak(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	var in struct {
		Confirm string `json:"confirm"`
		Reason  string `json:"reason"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if a.c.State != store.StateSealed {
		writeErr(w, http.StatusBadRequest, "only a sealed capsule has a seal to break")
		return
	}
	if strings.ToLower(strings.TrimSpace(in.Confirm)) != "open" {
		writeErr(w, http.StatusBadRequest, `type "open" to break the seal`)
		return
	}
	if err := s.openCapsule(ctx, a, !a.c.Ready(now()), in.Reason); err != nil {
		s.fail(w, err, "open")
		return
	}
	writeJSON(w, http.StatusOK, s.capsuleView(ctx, a, true))
}

// handleReflection keeps the letter back to the past (after opening) and,
// separately, the "then and now".
func (s *Server) handleReflection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return
	}
	if a.c.State != store.StateOpened {
		writeErr(w, http.StatusBadRequest, "open it first")
		return
	}
	if a.role == "viewer" && !a.owner() {
		writeErr(w, http.StatusForbidden, "only the people who filled it can write back")
		return
	}
	var in struct {
		Reflection *string `json:"reflection"`
		ThenNow    *string `json:"then_now"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Reflection != nil {
		a.c.Reflection = clip(strings.TrimSpace(*in.Reflection), maxNotes)
		s.store.Log(ctx, a.c.ID, uidPtr(a.u), s.actorName(a.u), "reflected", "")
	}
	if in.ThenNow != nil {
		a.c.ThenNow = clip(strings.TrimSpace(*in.ThenNow), maxNotes)
	}
	if err := s.store.UpdateCapsule(ctx, a.c); err != nil {
		s.fail(w, err, "save")
		return
	}
	writeJSON(w, http.StatusOK, s.capsuleView(ctx, a, true))
}

// handleReply starts the next capsule from an opened one: same people,
// same sections, a link back.
func (s *Server) handleReply(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return
	}
	if a.c.State != store.StateOpened {
		writeErr(w, http.StatusBadRequest, "open it first")
		return
	}
	prev := a.c
	st := s.settingsOf(ctx, a.u.ID)
	t := templateByID(prev.Template)
	years := 1
	if prev.SealedAt != nil {
		if sealed, err := time.Parse(time.RFC3339Nano, *prev.SealedAt); err == nil {
			if y := int(prev.Opens().Sub(sealed).Hours() / 24 / 365); y > 1 {
				years = y
			}
		}
	}
	opens := time.Date(now().Year()+years, now().Month(), now().Day(), st.OpenHour, 0, 0, 0, time.Local)
	c := &store.Capsule{UserID: a.u.ID, Template: t.ID, Emoji: prev.Emoji, Title: clip(prev.Title+" — the next one", maxName), OpenRule: "date", Deliver: prev.Deliver,
		Surprise: prev.Surprise, Slot: prev.Slot, RemindDays: prev.RemindDays, Sections: prev.Sections, ReplyTo: &prev.ID, Color: prev.Color,
		OpensAt: opens.UTC().Format(time.RFC3339Nano), About: "Written after opening “" + prev.Title + "”."}
	if err := s.store.CreateCapsule(ctx, c); err != nil {
		s.fail(w, err, "capsule")
		return
	}
	s.store.Log(ctx, c.ID, uidPtr(a.u), s.actorName(a.u), "made", "after “"+prev.Title+"”")
	if a.owner() {
		members, _ := s.store.Members(ctx, prev.ID)
		for _, m := range members {
			_ = s.store.SetMember(ctx, c.ID, m.UserID, m.Role)
		}
	} else {
		_ = s.store.SetMember(ctx, c.ID, prev.UserID, "contributor")
	}
	c, _ = s.store.GetCapsule(ctx, c.ID)
	writeJSON(w, http.StatusCreated, s.capsuleView(ctx, &access{c: c, u: a.u, role: "owner"}, true))
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return
	}
	h, err := s.store.History(r.Context(), a.c.ID)
	if err != nil {
		s.fail(w, err, "history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"history": h})
}

// ---- people in it ----

func (s *Server) addMember(ctx context.Context, a *access, email, role string) (*store.Member, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !oneOf(role, []string{"contributor", "viewer"}) {
		role = "contributor"
	}
	u, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, &userErr{"nobody on this box has that email: add them as someone it is for instead, or send them the “add something” link"}
	}
	if u.ID == a.c.UserID {
		return nil, &userErr{"that is you"}
	}
	if err := s.store.SetMember(ctx, a.c.ID, u.ID, role); err != nil {
		return nil, err
	}
	s.store.Log(ctx, a.c.ID, uidPtr(a.u), s.actorName(a.u), "member", firstNonEmpty(u.Name, u.Email))
	st := s.settingsOf(ctx, u.ID)
	if st.NotifyAdded {
		what := "add to it"
		if role == "viewer" {
			what = "open it together"
		}
		s.push(ctx, u.ID, st, s.actorName(a.u)+" put you in a time capsule", "“"+a.c.Title+"”: "+what+". It opens "+untilWords(a.c.Opens(), now())+".", fmt.Sprintf("/c/%d", a.c.ID))
	}
	return &store.Member{CapsuleID: a.c.ID, UserID: u.ID, Name: u.Name, Email: u.Email, Role: role}, nil
}

func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	if a.c.State == store.StateOpened {
		writeErr(w, http.StatusBadRequest, "it is already open")
		return
	}
	var in struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	m, err := s.addMember(ctx, a, in.Email, in.Role)
	if err != nil {
		s.userFail(w, err, "member")
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return
	}
	uid := idParam(r, "uid")
	if !a.owner() && uid != a.u.ID {
		writeErr(w, http.StatusForbidden, "only its maker can take someone out")
		return
	}
	if err := s.store.RemoveMember(ctx, a.c.ID, uid); err != nil {
		s.fail(w, err, "member")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- who it is for ----

func (s *Server) addRecipient(ctx context.Context, a *access, name, email string) (*store.Recipient, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(email, "@") || len(email) > 200 {
		return nil, &userErr{"add an email address"}
	}
	have, _ := s.store.Recipients(ctx, a.c.ID)
	for _, r := range have {
		if r.Email == email {
			return nil, &userErr{"they are already on the list"}
		}
	}
	if len(have) >= 50 {
		return nil, &userErr{"a capsule can be for up to 50 people"}
	}
	r := &store.Recipient{UserID: a.c.UserID, CapsuleID: a.c.ID, Name: cleanText(name, maxName), Email: email}
	if err := s.store.AddRecipient(ctx, r); err != nil {
		return nil, err
	}
	s.store.Log(ctx, a.c.ID, uidPtr(a.u), s.actorName(a.u), "recipient", firstNonEmpty(r.Name, r.Email))
	return r, nil
}

func (s *Server) handleAddRecipient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	var in struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	rc, err := s.addRecipient(ctx, a, in.Name, in.Email)
	if err != nil {
		s.userFail(w, err, "recipient")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": rc.ID, "name": rc.Name, "email": rc.Email, "url": s.link("/o/" + rc.Key)})
}

func (s *Server) handleDeleteRecipient(w http.ResponseWriter, r *http.Request) {
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	if err := s.store.DeleteRecipient(r.Context(), a.c.ID, idParam(r, "rid")); err != nil {
		s.fail(w, err, "recipient")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleSendRecipient emails one person their link now: the countdown
// before the opening, the capsule after.
func (s *Server) handleSendRecipient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	rc, err := s.store.GetRecipient(ctx, a.c.ID, idParam(r, "rid"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err := s.emailRecipient(ctx, a.u, a.c, rc); err != nil {
		s.userFail(w, err, "email")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": "Sent to " + firstNonEmpty(rc.Name, rc.Email)})
}

// handleGive switches the "add something" link on or off, or makes a new one.
func (s *Server) handleGive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	var in struct {
		Open   *bool `json:"open"`
		NewKey bool  `json:"new_key"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if a.c.State == store.StateOpened {
		writeErr(w, http.StatusBadRequest, "it is already open")
		return
	}
	if in.Open != nil {
		a.c.GiveOpen = *in.Open
	}
	if in.NewKey {
		a.c.GiveKey = store.NewKey()
	}
	if err := s.store.UpdateCapsule(ctx, a.c); err != nil {
		s.fail(w, err, "link")
		return
	}
	writeJSON(w, http.StatusOK, s.capsuleView(ctx, a, true))
}
