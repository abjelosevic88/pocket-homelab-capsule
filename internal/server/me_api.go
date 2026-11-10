package server

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

// handleHome is the first page: what is ready, what opens next, what is
// being filled, and the counts for the sidebar.
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	list, err := s.store.CapsulesFor(ctx, u.ID)
	if err != nil {
		s.fail(w, err, "home")
		return
	}
	t := now()
	counts := map[string]int{"ready": 0, "filling": 0, "sealed": 0, "opened": 0}
	var next *store.Capsule
	for i := range list {
		c := &list[i]
		switch {
		case c.Ready(t):
			counts["ready"]++
		case c.State == store.StateSealed:
			counts["sealed"]++
			if next == nil || c.Opens().Before(next.Opens()) {
				next = c
			}
		default:
			counts[c.State]++
		}
	}
	out := map[string]any{"counts": counts, "total": len(list)}
	if next != nil {
		next.Snapshot = nil
		out["next"] = map[string]any{"capsule": next, "opens_in": untilWords(next.Opens(), t)}
	}
	rituals, _ := s.store.Rituals(ctx, u.ID)
	due := []store.Ritual{}
	for _, r := range rituals {
		if r.Active && r.LastYear >= t.Year() && !s.wroteSince(list, r, t) {
			due = append(due, r)
		}
	}
	out["yearly_due"] = due
	writeJSON(w, http.StatusOK, out)
}

// wroteSince: a capsule was started since this year's nudge.
func (s *Server) wroteSince(list []store.Capsule, r store.Ritual, t time.Time) bool {
	day := time.Date(t.Year(), time.Month(r.Month), r.Day, 0, 0, 0, 0, time.Local)
	if t.Sub(day) > 14*24*time.Hour {
		return true // the nudge is old news
	}
	for _, c := range list {
		if c.Template == r.Template {
			if made, err := time.Parse(time.RFC3339Nano, c.CreatedAt); err == nil && !made.Before(day) {
				return true
			}
		}
	}
	return false
}

// handleSummary is a short overview for Pocket AI, Day and the Hub.
func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	list, err := s.store.CapsulesFor(ctx, u.ID)
	if err != nil {
		s.fail(w, err, "summary")
		return
	}
	t := now()
	ready, upcoming, filling := []map[string]any{}, []map[string]any{}, []map[string]any{}
	opened := 0
	for _, c := range list {
		row := map[string]any{"id": c.ID, "title": c.Title, "emoji": c.Emoji, "opens_at": c.OpensAt, "opens_in": untilWords(c.Opens(), t), "items": c.Items, "url": s.capsuleURL(&c)}
		switch {
		case c.Ready(t):
			ready = append(ready, row)
		case c.State == store.StateSealed:
			upcoming = append(upcoming, row)
		case c.State == store.StateFilling:
			filling = append(filling, row)
		default:
			opened++
		}
	}
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i]["opens_at"].(string) < upcoming[j]["opens_at"].(string) })
	if len(upcoming) > 5 {
		upcoming = upcoming[:5]
	}
	parts := []string{}
	if len(ready) > 0 {
		parts = append(parts, fmt.Sprintf("%d ready to open", len(ready)))
	}
	if len(upcoming) > 0 {
		parts = append(parts, fmt.Sprintf("next opens %s (“%s”)", upcoming[0]["opens_in"], upcoming[0]["title"]))
	}
	if len(filling) > 0 {
		parts = append(parts, fmt.Sprintf("%d being filled", len(filling)))
	}
	text := "No time capsules yet."
	if len(parts) > 0 {
		text = strings.ToUpper(parts[0][:1]) + parts[0][1:]
		if len(parts) > 1 {
			text += "; " + strings.Join(parts[1:], "; ")
		}
		text += "."
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": text, "ready": ready, "upcoming": upcoming, "filling": filling, "opened": opened, "url": s.link("/")})
}

// handleDay is one day for Pocket Day: capsules sealed, opened, opening and
// things added that day — never what is inside a sealed one.
func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	date := chiParam(r, "date")
	d, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "the date is YYYY-MM-DD")
		return
	}
	from, to := d.UTC().Format(time.RFC3339Nano), d.AddDate(0, 0, 1).UTC().Format(time.RFC3339Nano)
	evs, err := s.store.HistoryBetween(ctx, u.ID, from, to)
	if err != nil {
		s.fail(w, err, "day")
		return
	}
	items := []map[string]any{}
	lines := []string{}
	added := map[int64]int{}
	for _, e := range evs {
		if e.What == "added" {
			added[e.CapsuleID]++
			continue
		}
		text := ""
		switch e.What {
		case "made":
			text = fmt.Sprintf("Started the time capsule “%s”", e.Title)
		case "sealed":
			text = fmt.Sprintf("Sealed “%s” (%s)", e.Title, e.Detail)
		case "opened":
			text = fmt.Sprintf("%s opened “%s”", e.Who, e.Title)
		case "early":
			text = fmt.Sprintf("%s broke the seal of “%s” early", e.Who, e.Title)
		case "ready":
			text = fmt.Sprintf("“%s” was ready to open", e.Title)
		case "reflected":
			text = fmt.Sprintf("Wrote back to the past in “%s”", e.Title)
		default:
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, e.At)
		items = append(items, map[string]any{"time": at.Local().Format("15:04"), "kind": e.What, "text": text, "emoji": e.Emoji, "capsule_id": e.CapsuleID, "url": s.link(fmt.Sprintf("/c/%d", e.CapsuleID))})
		lines = append(lines, e.Emoji+" "+text)
	}
	for id, n := range added {
		c, err := s.store.GetCapsule(ctx, id)
		if err != nil {
			continue
		}
		text := fmt.Sprintf("Put %d %s in “%s”", n, plural(n, "thing", "things"), c.Title)
		items = append(items, map[string]any{"kind": "added", "text": text, "emoji": c.Emoji, "capsule_id": id, "url": s.capsuleURL(c)})
		lines = append(lines, c.Emoji+" "+text)
	}
	// capsules that open on this day (unless it already said so)
	readyOn := map[int64]bool{}
	for _, e := range evs {
		if e.What == "ready" {
			readyOn[e.CapsuleID] = true
		}
	}
	list, _ := s.store.CapsulesFor(ctx, u.ID)
	for _, c := range list {
		if c.State != store.StateOpened && !readyOn[c.ID] && c.Opens().Local().Format("2006-01-02") == date {
			text := fmt.Sprintf("“%s” opens", c.Title)
			items = append(items, map[string]any{"time": c.Opens().Local().Format("15:04"), "kind": "opens", "text": text, "emoji": c.Emoji, "capsule_id": c.ID, "url": s.capsuleURL(&c)})
			lines = append(lines, c.Emoji+" "+text)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "items": items, "text": strings.Join(lines, "\n"), "url": s.link("/")})
}

// handleSearch finds capsules by their outside, and opened ones by what is in them.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"results": []any{}})
		return
	}
	list, err := s.store.CapsulesFor(ctx, u.ID)
	if err != nil {
		s.fail(w, err, "search")
		return
	}
	out := []map[string]any{}
	for _, c := range list {
		if len(out) >= 50 {
			break
		}
		if strings.Contains(strings.ToLower(c.Title+" "+c.About+" "+c.ForName), q) {
			out = append(out, map[string]any{"type": "capsule", "capsule_id": c.ID, "title": c.Title, "emoji": c.Emoji, "state": c.State, "snippet": clip(c.About, 200), "url": s.capsuleURL(&c)})
		}
		if c.State != store.StateOpened {
			continue
		}
		items, _ := s.store.Items(ctx, c.ID)
		for _, it := range items {
			text := it.Title + " " + it.Body + " " + it.Answer
			for _, e := range it.Entries {
				text += " " + e.Label + " " + e.Value
			}
			if i := strings.Index(strings.ToLower(text), q); i >= 0 {
				start := max(0, i-60)
				out = append(out, map[string]any{"type": "item", "capsule_id": c.ID, "item_id": it.ID, "kind": it.Kind, "title": firstNonEmpty(it.Title, kindWord(it.Kind)) + " — " + c.Title,
					"emoji": c.Emoji, "snippet": clip(strings.TrimSpace(text[start:]), 200), "url": s.link(fmt.Sprintf("/c/%d#item-%d", c.ID, it.ID))})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

// handleEvents is what happened lately in the person's capsules.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	days := clampInt(firstPos(queryInt(r, "days"), 30), 1, 400)
	t := now()
	evs, err := s.store.HistoryBetween(ctx, u.ID, t.AddDate(0, 0, -days).UTC().Format(time.RFC3339Nano), t.Add(time.Minute).UTC().Format(time.RFC3339Nano))
	if err != nil {
		s.fail(w, err, "events")
		return
	}
	for i, j := 0, len(evs)-1; i < j; i, j = i+1, j-1 {
		evs[i], evs[j] = evs[j], evs[i]
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": evs})
}

// handlePredictions: how the guesses in opened capsules turned out, and
// how sure people were (are you right when you are 90% sure?), plus goals.
func (s *Server) handlePredictions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	u := userFrom(ctx)
	items, err := s.store.ItemsOfKind(ctx, u.ID, "prediction", "goal")
	if err != nil {
		s.fail(w, err, "predictions")
		return
	}
	titles := map[int64]map[string]any{}
	type bucket struct {
		Label        string  `json:"label"`
		From         int     `json:"from"`
		To           int     `json:"to"`
		N            int     `json:"n"`
		Yes          float64 `json:"yes"`
		Rate         float64 `json:"rate"`
		AvgConfident float64 `json:"avg_confidence"`
	}
	buckets := []*bucket{{Label: "Unsure (0–40%)", From: 0, To: 40}, {Label: "Maybe (41–60%)", From: 41, To: 60}, {Label: "Likely (61–80%)", From: 61, To: 80}, {Label: "Sure (81–100%)", From: 81, To: 100}}
	preds, goals := []map[string]any{}, []map[string]any{}
	marked, right := 0, 0.0
	goalsDone, goalsMarked := 0.0, 0
	for _, it := range items {
		ct, ok := titles[it.CapsuleID]
		if !ok {
			if c, err := s.store.GetCapsule(ctx, it.CapsuleID); err == nil {
				ct = map[string]any{"title": c.Title, "emoji": c.Emoji, "opened_at": c.OpenedAt, "sealed_at": c.SealedAt}
				titles[it.CapsuleID] = ct
			}
		}
		row := map[string]any{"item": it, "capsule": ct}
		score := map[string]float64{"yes": 1, "partly": 0.5, "no": 0}[it.Outcome]
		if it.Kind == "goal" {
			goals = append(goals, row)
			if it.Outcome != "" {
				goalsMarked++
				goalsDone += score
			}
			continue
		}
		preds = append(preds, row)
		if it.Outcome == "" {
			continue
		}
		marked++
		right += score
		for _, b := range buckets {
			if it.Confidence >= b.From && it.Confidence <= b.To {
				b.N++
				b.Yes += score
				b.AvgConfident += float64(it.Confidence)
			}
		}
	}
	for _, b := range buckets {
		if b.N > 0 {
			b.Rate = b.Yes / float64(b.N) * 100
			b.AvgConfident /= float64(b.N)
		}
	}
	out := map[string]any{"predictions": preds, "goals": goals, "buckets": buckets, "marked": marked, "unmarked": len(preds) - marked, "goals_marked": goalsMarked}
	if marked > 0 {
		out["right_pct"] = right / float64(marked) * 100
	}
	if goalsMarked > 0 {
		out["goals_pct"] = goalsDone / float64(goalsMarked) * 100
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- yearly capsules ----

func (s *Server) handleRituals(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.Rituals(r.Context(), actorID(r.Context()))
	if err != nil {
		s.fail(w, err, "yearly")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rituals": list})
}

func (s *Server) handleSaveRitual(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	rt := &store.Ritual{UserID: uid, Active: true, Years: 1, Template: "self"}
	if id := idParam(r, "id"); id > 0 {
		cur, err := s.store.GetRitual(ctx, uid, id)
		if err != nil {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		rt = cur
	}
	var in struct {
		Title    *string `json:"title"`
		Template *string `json:"template"`
		Month    *int    `json:"month"`
		Day      *int    `json:"day"`
		Years    *int    `json:"years"`
		Active   *bool   `json:"active"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Title != nil {
		rt.Title = cleanText(*in.Title, maxName)
	}
	if in.Template != nil {
		rt.Template = templateByID(*in.Template).ID
	}
	if in.Month != nil {
		rt.Month = *in.Month
	}
	if in.Day != nil {
		rt.Day = *in.Day
	}
	if in.Years != nil {
		rt.Years = clampInt(*in.Years, 1, 50)
	}
	if in.Active != nil {
		rt.Active = *in.Active
	}
	if rt.Title == "" {
		rt.Title = templateByID(rt.Template).Name
	}
	if rt.Month < 1 || rt.Month > 12 || rt.Day < 1 || rt.Day > time.Date(2024, time.Month(rt.Month)+1, 0, 0, 0, 0, 0, time.UTC).Day() {
		writeErr(w, http.StatusBadRequest, "pick a day of the year")
		return
	}
	if err := s.store.SaveRitual(ctx, rt); err != nil {
		s.fail(w, err, "yearly")
		return
	}
	writeJSON(w, http.StatusOK, rt)
}

func (s *Server) handleDeleteRitual(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteRitual(r.Context(), actorID(r.Context()), idParam(r, "id")); err != nil {
		s.fail(w, err, "yearly")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
