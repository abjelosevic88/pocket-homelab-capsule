package server

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

// "The world today": when a capsule is sealed it keeps a few facts of the
// day from the person's other Pocket apps — the weather, what they were
// reading and listening to, the countries of their year, and (only when
// picked) their health and net worth. At the opening the same facts are
// read again, for "then and now". Every part is optional: an app that is
// not connected is just left out, with why.

// sectionApps says which app each part comes from.
var sectionApps = map[string]pocketApp{"weather": appWeather, "reading": appBookshelf, "listening": appMusic, "places": appMovement, "health": appHealth, "wealth": appWealth}

var sectionScopes = map[string]string{"weather": scopeWeatherRead, "reading": scopeBooksRead, "listening": scopeListeningRead, "places": scopeMovementRead, "health": scopeHealthRead, "wealth": scopeWealthRead}

// takeSnapshot reads the parts at once, each with a few seconds to answer.
func (s *Server) takeSnapshot(ctx context.Context, u *store.User, sections []string) map[string]any {
	out := map[string]any{"taken_at": store.Now()}
	missing := map[string]string{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, sec := range cleanSections(sections) {
		wg.Add(1)
		go func(sec string) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
			defer cancel()
			v, err := s.snapshotPart(cctx, u, sec)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				missing[sec] = plainErr(sectionApps[sec], err)
				return
			}
			if v != nil {
				out[sec] = v
			}
		}(sec)
	}
	wg.Wait()
	if len(missing) > 0 {
		out["missing"] = missing
	}
	return out
}

func (s *Server) snapshotPart(ctx context.Context, u *store.User, sec string) (any, error) {
	a := sectionApps[sec]
	sib, err := s.clients(ctx, u.ID).conn(ctx, a, sectionScopes[sec])
	if err != nil {
		return nil, err
	}
	switch sec {
	case "weather":
		v, err := getJSONFrom(ctx, sib, "/api/v1/summary", nil)
		if err != nil {
			return nil, err
		}
		m, _ := v.(map[string]any)
		place, nw := obj(m, "place"), obj(m, "now")
		if place == nil {
			return nil, &notConnected{"Pocket Weather has no place yet"}
		}
		cond := str(nw, "condition")
		out := map[string]any{"place": str(place, "name"), "condition": cond, "emoji": weatherEmoji(cond), "text": firstNonEmpty(str(nw, "text"), str(m, "text"))}
		if t, ok := nw["temp"].(float64); ok {
			out["temp"] = t
		}
		if today := obj(m, "today"); today != nil {
			if t, ok := today["t_max"].(float64); ok {
				out["t_max"] = t
			}
			if t, ok := today["t_min"].(float64); ok {
				out["t_min"] = t
			}
		}
		return out, nil
	case "reading":
		v, err := getJSONFrom(ctx, sib, "/api/v1/books", url.Values{"status": {"reading"}, "limit": {"8"}})
		if err != nil {
			return nil, err
		}
		books := []map[string]any{}
		for _, b := range rows(v, "books") {
			books = append(books, map[string]any{"title": str(b, "title"), "author": str(b, "authors", "author")})
		}
		if len(books) == 0 {
			return nil, nil
		}
		return books, nil
	case "listening":
		v, err := getJSONFrom(ctx, sib, "/api/v1/stats", url.Values{"period": {"month"}, "limit": {"10"}})
		if err != nil {
			return nil, err
		}
		m, _ := v.(map[string]any)
		artists, songs := []map[string]any{}, []map[string]any{}
		for _, a := range rows(m["artists"]) {
			if len(artists) < 5 {
				artists = append(artists, map[string]any{"name": str(a, "name"), "plays": num(a, "plays")})
			}
		}
		for _, a := range rows(m["songs"]) {
			if len(songs) < 5 {
				songs = append(songs, map[string]any{"name": str(a, "name"), "artist": str(a, "artist"), "plays": num(a, "plays")})
			}
		}
		if len(artists) == 0 && len(songs) == 0 {
			return nil, nil
		}
		return map[string]any{"artists": artists, "songs": songs, "plays": num(obj(m, "totals"), "plays")}, nil
	case "places":
		year := now().Year()
		v, err := getJSONFrom(ctx, sib, "/api/v1/insights", url.Values{"year": {strconv.Itoa(year)}})
		if err != nil {
			return nil, err
		}
		m, _ := v.(map[string]any)
		countries := []string{}
		for _, c := range rows(m["countries"]) {
			countries = append(countries, str(c, "name"))
		}
		top := []string{}
		for _, p := range rows(m["top_places"]) {
			if n := str(obj(p, "place"), "name"); n != "" && len(top) < 5 {
				top = append(top, n)
			}
		}
		t := obj(m, "totals")
		return map[string]any{"year": year, "countries": countries, "places": num(t, "places"), "cities": num(t, "cities"), "top": top}, nil
	case "health":
		v, err := getJSONFrom(ctx, sib, "/api/v1/dashboard", nil)
		if err != nil {
			return nil, err
		}
		m, _ := v.(map[string]any)
		out := map[string]any{}
		if n := num(m, "steps"); n > 0 {
			out["steps"] = n
		}
		if t := str(m, "sleep_text"); t != "" {
			out["sleep"] = t
		}
		if t := str(m, "weight_text"); t != "" {
			out["weight"] = t
		}
		if n := num(m, "resting_hr"); n > 0 {
			out["resting_hr"] = n
		}
		if len(out) == 0 {
			return nil, nil
		}
		return out, nil
	case "wealth":
		v, err := getJSONFrom(ctx, sib, "/api/v1/summary", nil)
		if err != nil {
			return nil, err
		}
		m, _ := v.(map[string]any)
		cur := str(m, "currency")
		if cur == "" {
			return nil, nil
		}
		return map[string]any{"net": num(m, "net"), "currency": cur}, nil
	}
	return nil, nil
}

func weatherEmoji(cond string) string {
	return firstNonEmpty(map[string]string{"clear": "☀️", "mostly-clear": "🌤️", "partly": "⛅", "cloudy": "☁️", "fog": "🌫️", "drizzle": "🌦️", "freezing": "🧊",
		"rain": "🌧️", "snow": "❄️", "storm": "⛈️"}[cond], "🌡️")
}

// snapshotLines is the world of a snapshot in short lines (for a note, an
// email, the AI).
func snapshotLines(raw map[string]any) []string {
	out := []string{}
	if w := mapOf(raw["weather"]); w != nil {
		line := fmt.Sprintf("Weather in %s: %s", str(w, "place"), str(w, "text"))
		if t, ok := w["temp"].(float64); ok {
			line = fmt.Sprintf("Weather in %s: %.0f °C, %s", str(w, "place"), t, firstNonEmpty(str(w, "condition"), str(w, "text")))
		}
		out = append(out, line)
	}
	if b, ok := raw["reading"].([]any); ok && len(b) > 0 {
		names := []string{}
		for _, x := range b {
			if m := mapOf(x); m != nil {
				names = append(names, joinNonEmpty(" by ", str(m, "title"), str(m, "author")))
			}
		}
		out = append(out, "Reading: "+strings.Join(names, "; "))
	}
	if l := mapOf(raw["listening"]); l != nil {
		names := []string{}
		for _, x := range rows(l["artists"]) {
			names = append(names, str(x, "name"))
		}
		if len(names) > 0 {
			out = append(out, "Listening to: "+strings.Join(names, ", "))
		}
	}
	if p := mapOf(raw["places"]); p != nil {
		if c, ok := p["countries"].([]any); ok && len(c) > 0 {
			names := []string{}
			for _, x := range c {
				if v, ok := x.(string); ok {
					names = append(names, v)
				}
			}
			out = append(out, fmt.Sprintf("Countries in %v so far: %s", p["year"], strings.Join(names, ", ")))
		}
		if t, ok := p["top"].([]any); ok && len(t) > 0 {
			names := []string{}
			for _, x := range t {
				if v, ok := x.(string); ok {
					names = append(names, v)
				}
			}
			out = append(out, "Places you went to most: "+strings.Join(names, ", "))
		}
	}
	if h := mapOf(raw["health"]); h != nil {
		parts := []string{}
		if v, ok := h["steps"].(float64); ok {
			parts = append(parts, fmt.Sprintf("%.0f steps", v))
		}
		if v := str(h, "sleep"); v != "" {
			parts = append(parts, "slept "+v)
		}
		if v := str(h, "weight"); v != "" {
			parts = append(parts, v)
		}
		if len(parts) > 0 {
			out = append(out, "Health that day: "+strings.Join(parts, ", "))
		}
	}
	if m := mapOf(raw["wealth"]); m != nil {
		if v, ok := m["net"].(float64); ok {
			out = append(out, fmt.Sprintf("Net worth: %s %s", money(v/100), str(m, "currency")))
		}
	}
	return out
}

func mapOf(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func money(v float64) string {
	s := strconv.FormatFloat(v, 'f', 0, 64)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// handleSnapshotPreview shows what sealing would keep today (filling only).
func (s *Server) handleSnapshotPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return
	}
	secs := a.c.Sections
	if q := r.URL.Query().Get("sections"); q != "" {
		secs = strings.Split(q, ",")
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshot": s.takeSnapshot(ctx, a.u, secs), "sections": sectionInfo()})
}

// handleNow reads the same parts again at the opening, for "then and now".
func (s *Server) handleNow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.capsuleParam(w, r)
	if !ok {
		return
	}
	if a.c.State != store.StateOpened {
		writeErr(w, http.StatusBadRequest, "open it first")
		return
	}
	secs := []string{}
	if m := mapOfRaw(a.c.Snapshot); m != nil {
		for k := range m {
			if oneOf(k, store.Sections) {
				secs = append(secs, k)
			}
		}
	}
	sort.Strings(secs)
	writeJSON(w, http.StatusOK, map[string]any{"now": s.takeSnapshot(ctx, a.u, secs)})
}

func sectionInfo() []map[string]any {
	names := map[string][2]string{"weather": {"The weather", "Pocket Weather"}, "reading": {"What you are reading", "Pocket Bookshelf"}, "listening": {"What you listen to", "Pocket Music"},
		"places": {"Countries and places of your year", "Pocket Movement"}, "health": {"Steps, sleep and weight", "Pocket Health"}, "wealth": {"Your net worth", "Pocket Wealth"}}
	out := []map[string]any{}
	for _, k := range store.Sections {
		out = append(out, map[string]any{"id": k, "name": names[k][0], "app": names[k][1], "slug": sectionApps[k].slug, "scope": sectionScopes[k]})
	}
	return out
}

func (s *Server) handleSections(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"sections": sectionInfo()})
}
