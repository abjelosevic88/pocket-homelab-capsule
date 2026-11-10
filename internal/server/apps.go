package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

// The other Pocket apps. None of it is needed: when an app is not there,
// the page greys the button and says what to do.

func linkTo(sib *siblingConn, path string) string {
	if sib == nil || sib.public == "" {
		return ""
	}
	return strings.TrimRight(sib.public, "/") + path
}

func plainErr(a pocketApp, err error) string {
	if isNotConnected(err) {
		return err.Error()
	}
	return a.name + ": " + err.Error()
}

func (s *Server) appFail(w http.ResponseWriter, a pocketApp, err error) {
	if isNotConnected(err) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if ue, ok := err.(*userErr); ok {
		writeErr(w, http.StatusBadRequest, ue.msg)
		return
	}
	writeErr(w, http.StatusBadGateway, plainErr(a, err))
}

// handleApps lists how the person reaches each app, in words they can act on.
func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := s.clients(ctx, actorID(ctx))
	out := []appState{}
	for _, a := range pocketApps {
		out = append(out, c.status(ctx, a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": out, "core": s.core.Configured()})
}

func (s *Server) sib(w http.ResponseWriter, r *http.Request, a pocketApp, scopes ...string) (*siblingConn, bool) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, a, scopes...)
	if err != nil {
		s.appFail(w, a, err)
		return nil, false
	}
	return sib, true
}

// capsuleBody decodes the request and finds the capsule it is about.
func (s *Server) capsuleBody(w http.ResponseWriter, r *http.Request, v any, id func() int64) (*access, bool) {
	if v != nil {
		if err := decode(r, v); err != nil {
			writeErr(w, http.StatusBadRequest, "invalid json")
			return nil, false
		}
	}
	a, err := s.capsuleAccess(r.Context(), userFrom(r.Context()), id())
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return nil, false
	}
	return a, true
}

func (s *Server) capsuleURL(c *store.Capsule) string { return s.link(fmt.Sprintf("/c/%d", c.ID)) }

// ---- Pocket Calendar ----

// toCalendar puts the opening day in the maker's calendar.
func (s *Server) toCalendar(ctx context.Context, a *access) (string, error) {
	sib, err := s.clients(ctx, a.u.ID).conn(ctx, appCalendar, scopeCalendarWrite)
	if err != nil {
		return "", err
	}
	c := a.c
	day := c.Opens().Local().Format("2006-01-02")
	desc := "A time capsule you sealed opens today."
	if c.About != "" {
		desc += "\n\n" + c.About
	}
	desc += "\n\nOpen it: " + s.capsuleURL(c)
	res, err := postJSONTo(ctx, sib, "/api/v1/events", map[string]any{"title": clip(c.Emoji+" "+c.Title+" opens", 200), "description": desc, "url": s.capsuleURL(c),
		"all_day": true, "start": day, "end": day, "categories": []string{"Time capsules"}})
	if err != nil {
		return "", err
	}
	link := linkTo(sib, "/?event="+url.QueryEscape(firstNonEmpty(str(res, "event_id"), idOf(res))))
	c.CalendarURL = link
	_ = s.store.UpdateCapsule(ctx, c)
	return link, nil
}

func (s *Server) handleToCalendar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	if a.c.State == store.StateOpened {
		writeErr(w, http.StatusBadRequest, "it is already open")
		return
	}
	link, err := s.toCalendar(ctx, a)
	if err != nil {
		s.appFail(w, appCalendar, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": link, "text": "The opening day is in Pocket Calendar"})
}

// ---- Pocket Contacts and Family: people and their birthdays ----

func (s *Server) handleContacts(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appContacts, scopeContactsRead)
	if !ok {
		return
	}
	q := url.Values{"limit": {"40"}}
	if t := strings.TrimSpace(r.URL.Query().Get("q")); t != "" {
		q.Set("q", t)
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/contacts", q)
	if err != nil {
		s.appFail(w, appContacts, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "contacts") {
		emails := []string{}
		if arr, ok := m["emails"].([]any); ok {
			for _, e := range arr {
				switch x := e.(type) {
				case string:
					emails = append(emails, x)
				case map[string]any:
					if v := str(x, "value", "email", "addr"); v != "" {
						emails = append(emails, v)
					}
				}
			}
		}
		out = append(out, map[string]any{"id": idOf(m), "name": firstNonEmpty(str(m, "fn"), str(m, "name"), joinNonEmpty(" ", str(m, "given"), str(m, "family"))), "emails": emails,
			"birthday": fullDate(str(m, "birthday", "bday")), "anniversary": fullDate(str(m, "anniversary"))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"contacts": out})
}

// fullDate keeps a date with its year (YYYY-MM-DD or YYYYMMDD), else "".
func fullDate(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 10 && validDate(v[:10]) {
		return v[:10]
	}
	if len(v) == 8 {
		if d := v[:4] + "-" + v[4:6] + "-" + v[6:]; validDate(d) {
			return d
		}
	}
	return ""
}

func (s *Server) handleFamily(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appFamily, scopeFamilyRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/family", nil)
	if err != nil {
		s.appFail(w, appFamily, err)
		return
	}
	m, _ := v.(map[string]any)
	out := []map[string]any{}
	for _, p := range rows(m["people"]) {
		if p["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": idOf(p), "name": str(p, "name"), "emoji": str(p, "emoji"), "role": str(p, "role"), "email": str(p, "email"), "birthday": fullDate(str(p, "birthday"))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"family": str(m, "name"), "people": out})
}

// ---- Pocket Email ----

// emailRecipient sends one person their link from the maker's own mail
// account: what is waiting before the opening, the capsule after it.
func (s *Server) emailRecipient(ctx context.Context, maker *store.User, c *store.Capsule, rc *store.Recipient) error {
	sib, err := s.clients(ctx, maker.ID).conn(ctx, appEmail, scopeMailSend)
	if err != nil {
		return err
	}
	link := s.link("/o/" + rc.Key)
	name := s.actorName(maker)
	var subject, text, button string
	if recipientMayOpen(c) {
		subject = "A time capsule from " + name + " has opened"
		text = fmt.Sprintf("Hello%s,\n\n%s made a time capsule for you%s: “%s”. It is open now.", greetName(rc.Name), name, sealedWhen(c), c.Title)
		button = "Open the capsule"
	} else {
		subject = name + " made a time capsule for you"
		text = fmt.Sprintf("Hello%s,\n\n%s is keeping a time capsule for you: “%s”. It opens on %s — this link shows how long is left, and the capsule itself once it opens.",
			greetName(rc.Name), name, c.Title, c.Opens().Local().Format("2 January 2006"))
		button = "See the capsule"
	}
	if c.About != "" {
		text += "\n\n" + c.About
	}
	html := `<div style="font-family:-apple-system,Segoe UI,Roboto,sans-serif;line-height:1.5;font-size:15px">` + strings.ReplaceAll(htmlEsc(text), "\n", "<br>") +
		`<p style="margin-top:20px"><a href="` + htmlEsc(link) + `" style="display:inline-block;padding:10px 18px;background:#7a4a9e;color:#fff;border-radius:8px;text-decoration:none">` + c.Emoji + " " + htmlEsc(button) + `</a></p></div>`
	if _, err := postJSONTo(ctx, sib, "/api/v1/send", map[string]any{"to": []map[string]string{{"name": rc.Name, "addr": rc.Email}}, "subject": clip(subject, 200),
		"body_text": text + "\n\n" + link, "body_html": html}); err != nil {
		return err
	}
	_ = s.store.MarkRecipient(ctx, rc.ID, "sent")
	return nil
}

func greetName(n string) string {
	if f := strings.Fields(n); len(f) > 0 {
		return " " + f[0]
	}
	return ""
}

func sealedWhen(c *store.Capsule) string {
	if c.SealedAt == nil {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, *c.SealedAt)
	if err != nil {
		return ""
	}
	return " and sealed it on " + t.Local().Format("2 January 2006")
}

// handleAsk emails people the "add something" link (switching it on if needed).
func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.ownerParam(w, r)
	if !ok {
		return
	}
	var in struct {
		Text string `json:"text"`
		To   []struct {
			Name string `json:"name"`
			Addr string `json:"addr"`
		} `json:"to"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if ok, why := a.canAdd(); !ok {
		writeErr(w, http.StatusBadRequest, why)
		return
	}
	to := []map[string]string{}
	for _, t := range in.To {
		if ad := strings.TrimSpace(t.Addr); strings.Contains(ad, "@") && len(to) < 50 {
			to = append(to, map[string]string{"name": strings.TrimSpace(t.Name), "addr": ad})
		}
	}
	if len(to) == 0 {
		writeErr(w, http.StatusBadRequest, "add at least one email address")
		return
	}
	sib, ok := s.sib(w, r, appEmail, scopeMailSend)
	if !ok {
		return
	}
	if !a.c.GiveOpen {
		a.c.GiveOpen = true
		_ = s.store.UpdateCapsule(ctx, a.c)
	}
	link := s.link("/give/" + a.c.GiveKey)
	name := s.actorName(a.u)
	text := firstNonEmpty(strings.TrimSpace(in.Text), fmt.Sprintf("I am making a time capsule, “%s”, that opens on %s. Would you add a letter or a photo? Nobody will see it until then.", a.c.Title, a.c.Opens().Local().Format("2 January 2006")))
	html := `<div style="font-family:-apple-system,Segoe UI,Roboto,sans-serif;line-height:1.5;font-size:15px">` + strings.ReplaceAll(htmlEsc(text), "\n", "<br>") +
		`<p style="margin-top:20px"><a href="` + htmlEsc(link) + `" style="display:inline-block;padding:10px 18px;background:#7a4a9e;color:#fff;border-radius:8px;text-decoration:none">` + a.c.Emoji + ` Add something</a></p></div>`
	if _, err := postJSONTo(ctx, sib, "/api/v1/send", map[string]any{"to": to, "subject": clip(name+" asks you to add to a time capsule", 200), "body_text": text + "\n\n" + link, "body_html": html}); err != nil {
		s.appFail(w, appEmail, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": fmt.Sprintf("Sent to %d %s", len(to), plural(len(to), "person", "people"))})
}

// ---- Pocket Chat ----

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appChat, scopeChatRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/conversations", nil)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "conversations") {
		if m["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": idOf(m), "name": firstNonEmpty(str(m, "name"), str(m, "title")), "kind": str(m, "kind"), "emoji": str(m, "emoji")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": out})
}

// handleShareChat tells a chat about a capsule (filling: add something;
// sealed: when it opens; opened: come and see).
func (s *Server) handleShareChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		CapsuleID int64  `json:"capsule_id"`
		To        string `json:"to"`
		Text      string `json:"text"`
	}
	a, ok := s.capsuleBody(w, r, &in, func() int64 { return in.CapsuleID })
	if !ok {
		return
	}
	sib, ok := s.sib(w, r, appChat, scopeChatWrite)
	if !ok {
		return
	}
	c := a.c
	var sub, line, text string
	switch c.State {
	case store.StateOpened:
		sub, line, text = "Opened "+firstNonEmpty(sinceWords(c.OpenedAt), "today"), fmt.Sprintf("%d %s inside", c.Items, plural(c.Items, "thing", "things")), "Our time capsule is open!"
	case store.StateSealed:
		sub, line, text = "Sealed · opens "+c.Opens().Local().Format("2 Jan 2006"), "Opens "+untilWords(c.Opens(), now()), "The time capsule is sealed."
	default:
		sub, line, text = "Opens "+c.Opens().Local().Format("2 Jan 2006"), firstNonEmpty(c.About, "Add something before it is sealed"), "I am filling a time capsule."
	}
	card := map[string]any{"title": c.Title, "subtitle": sub, "lines": []string{clip(line, 160)}, "icon": c.Emoji, "url": s.capsuleURL(c), "app": "capsule", "kind": "card"}
	st := s.settingsOf(ctx, a.u.ID)
	body := map[string]any{"to": firstNonEmpty(strings.TrimSpace(in.To), st.ChatTo, "me"), "text": clip(firstNonEmpty(strings.TrimSpace(in.Text), text), 2000), "card": card,
		"client_id": fmt.Sprintf("capsule-%d-%d-%s-%d", a.u.ID, c.ID, c.State, now().Unix()/30)}
	res, err := postJSONTo(ctx, sib, "/api/v1/send", body)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": linkTo(sib, "/c/"+str(res, "conversation_id")), "text": "Sent to Pocket Chat"})
}

func sinceWords(at *string) string {
	if at == nil {
		return ""
	}
	t, err := time.Parse(time.RFC3339Nano, *at)
	if err != nil {
		return ""
	}
	return "on " + t.Local().Format("2 Jan 2006")
}

// ---- Pocket Photos ----

func (s *Server) handlePhotosList(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appPhotos, scopePhotosRead)
	if !ok {
		return
	}
	body := map[string]any{"filter": map[string]any{"trashedAt": map[string]any{"eq": nil}, "type": map[string]any{"eq": "IMAGE"}, "visibility": map[string]any{"eq": "timeline"}},
		"size": 60, "withExif": false, "orderBy": map[string]any{"field": "fileCreatedAt", "direction": "desc"}}
	path := "/api/v1/immich/search/metadata"
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		body["query"] = q
		path = "/api/v1/immich/search/smart"
	}
	if c := r.URL.Query().Get("cursor"); c != "" {
		body["cursor"] = c
	}
	res, err := postJSONTo(r.Context(), sib, path, body)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	assets := obj(res, "assets")
	out := []map[string]any{}
	for _, m := range rows(assets["items"]) {
		id := str(m, "id")
		if !reUUID.MatchString(id) {
			continue
		}
		out = append(out, map[string]any{"id": id, "name": str(m, "originalFileName"), "date": day10(m["fileCreatedAt"]), "thumb_url": "/api/v1/apps/photos/" + id + "/image"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"photos": out, "cursor": assets["nextCursor"]})
}

func (s *Server) handlePhotoImage(w http.ResponseWriter, r *http.Request) {
	id := chiParam(r, "uuid")
	if !reUUID.MatchString(id) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	sib, ok := s.sib(w, r, appPhotos, scopePhotosRead)
	if !ok {
		return
	}
	b, ctype, err := sib.client.Bytes(r.Context(), "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {"thumbnail"}}, 16<<20)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	w.Header().Set("Content-Type", firstNonEmpty(ctype, http.DetectContentType(b)))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(b)
}

// handlePhotoCopy puts copies of Pocket Photos pictures in a capsule (a
// capsule has to outlive the library), one item each.
func (s *Server) handlePhotoCopy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		CapsuleID int64    `json:"capsule_id"`
		IDs       []string `json:"ids"`
		Caption   string   `json:"caption"`
	}
	a, ok := s.capsuleBody(w, r, &in, func() int64 { return in.CapsuleID })
	if !ok {
		return
	}
	if can, why := a.canAdd(); !can {
		writeErr(w, http.StatusBadRequest, why)
		return
	}
	if len(in.IDs) == 0 || len(in.IDs) > 30 {
		writeErr(w, http.StatusBadRequest, "pick 1 to 30 photos")
		return
	}
	sib, ok := s.sib(w, r, appPhotos, scopePhotosRead)
	if !ok {
		return
	}
	added := 0
	for _, id := range in.IDs {
		if !reUUID.MatchString(id) {
			continue
		}
		b, ctype, err := sib.client.Bytes(ctx, "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {"preview"}}, maxUpload)
		if err != nil {
			s.appFail(w, appPhotos, err)
			return
		}
		thumb, _, _ := sib.client.Bytes(ctx, "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {"thumbnail"}}, 4<<20)
		mime := firstNonEmpty(strings.Split(ctype, ";")[0], http.DetectContentType(b))
		if !strings.HasPrefix(mime, "image/") {
			mime = "image/jpeg"
		}
		up := &store.Upload{UserID: a.u.ID, CapsuleID: a.c.ID, Name: "photo.jpg", Mime: mime, Data: b, Thumb: thumb}
		if err := s.store.SaveUpload(ctx, up); err != nil {
			s.fail(w, err, "photo")
			return
		}
		it := &store.Item{CapsuleID: a.c.ID, UserID: uidPtr(a.u), AuthorName: s.actorName(a.u), Kind: "photo", Body: cleanText(in.Caption, 2000), UploadID: &up.ID,
			FromApp: "photos", FromURL: linkTo(sib, "/photos/"+id), AfterSeal: a.c.State == store.StateSealed}
		if err := s.store.CreateItem(ctx, it); err != nil {
			s.fail(w, err, "photo")
			return
		}
		s.afterAdded(ctx, a.c, it, a.u)
		added++
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "added": added, "text": fmt.Sprintf("%d %s put in", added, plural(added, "photo", "photos"))})
}

// ---- Pocket Notes ----

func (s *Server) handleNotesSearch(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appNotes, scopeNotesRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/search", url.Values{"q": {r.URL.Query().Get("q")}, "limit": {"30"}})
	if err != nil {
		s.appFail(w, appNotes, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "hits", "notes") {
		n := m
		if inner, ok := m["note"].(map[string]any); ok {
			n = inner
		}
		out = append(out, map[string]any{"id": idOf(n), "title": firstNonEmpty(str(n, "title"), "Untitled"), "folder": str(n, "category"), "snippet": clip(firstNonEmpty(str(m, "snippet"), str(n, "excerpt")), 200)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": out})
}

// handleNoteCopy puts a copy of a note in a capsule, as a letter.
func (s *Server) handleNoteCopy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		CapsuleID int64  `json:"capsule_id"`
		NoteID    string `json:"note_id"`
	}
	a, ok := s.capsuleBody(w, r, &in, func() int64 { return in.CapsuleID })
	if !ok {
		return
	}
	if can, why := a.canAdd(); !can {
		writeErr(w, http.StatusBadRequest, why)
		return
	}
	if _, err := strconv.ParseInt(in.NoteID, 10, 64); err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	sib, ok := s.sib(w, r, appNotes, scopeNotesRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/notes/"+in.NoteID, nil)
	if err != nil {
		s.appFail(w, appNotes, err)
		return
	}
	m, _ := v.(map[string]any)
	if inner, ok := m["note"].(map[string]any); ok {
		m = inner
	}
	body := strings.TrimSpace(str(m, "content"))
	if body == "" {
		writeErr(w, http.StatusBadRequest, "that note is empty")
		return
	}
	it := &store.Item{CapsuleID: a.c.ID, UserID: uidPtr(a.u), AuthorName: s.actorName(a.u), Kind: "letter", Title: cleanText(str(m, "title"), 200), Body: clip(body, maxNotes),
		FromApp: "notes", FromURL: linkTo(sib, "/notes/"+in.NoteID), AfterSeal: a.c.State == store.StateSealed}
	if err := s.store.CreateItem(ctx, it); err != nil {
		s.fail(w, err, "note")
		return
	}
	s.afterAdded(ctx, a.c, it, a.u)
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "text": "The note is in the capsule"})
}

// capsuleMarkdown is an opened capsule as Markdown (for a note or a file).
func (s *Server) capsuleMarkdown(ctx context.Context, c *store.Capsule, absolute bool) string {
	items, _ := s.store.Items(ctx, c.ID)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s %s\n\n", c.Emoji, c.Title)
	sealed, opened := "", ""
	if c.SealedAt != nil {
		if t, err := time.Parse(time.RFC3339Nano, *c.SealedAt); err == nil {
			sealed = "Sealed " + t.Local().Format("2 January 2006")
		}
	}
	if c.OpenedAt != nil {
		if t, err := time.Parse(time.RFC3339Nano, *c.OpenedAt); err == nil {
			opened = "opened " + t.Local().Format("2 January 2006")
		}
	}
	if line := joinNonEmpty(", ", sealed, opened); line != "" {
		fmt.Fprintf(&b, "*%s*\n\n", line)
	}
	if c.About != "" {
		fmt.Fprintf(&b, "%s\n\n", c.About)
	}
	upURL := func(id int64) string {
		u := fmt.Sprintf("/api/v1/uploads/%d", id)
		if absolute {
			return s.link(u)
		}
		return u
	}
	for _, it := range items {
		by := ""
		if it.AuthorName != "" {
			by = " — " + it.AuthorName
		}
		switch it.Kind {
		case "letter":
			fmt.Fprintf(&b, "## ✉️ %s%s\n\n%s\n\n", firstNonEmpty(it.Title, "A letter"), by, it.Body)
		case "photo":
			fmt.Fprintf(&b, "## 🖼️ %s%s\n\n", firstNonEmpty(it.Title, "A photo"), by)
			if it.UploadID != nil {
				fmt.Fprintf(&b, "![%s](%s)\n\n", firstNonEmpty(it.Title, "photo"), upURL(*it.UploadID))
			}
			if it.Body != "" {
				fmt.Fprintf(&b, "%s\n\n", it.Body)
			}
		case "audio", "video", "file":
			fmt.Fprintf(&b, "## %s %s%s\n\n", map[string]string{"audio": "🎙️", "video": "🎬", "file": "📎"}[it.Kind], firstNonEmpty(it.Title, it.Name, kindWord(it.Kind)), by)
			if it.UploadID != nil {
				fmt.Fprintf(&b, "[%s](%s)\n\n", firstNonEmpty(it.Name, "the file"), upURL(*it.UploadID))
			}
			if it.Body != "" {
				fmt.Fprintf(&b, "%s\n\n", it.Body)
			}
		case "prediction":
			fmt.Fprintf(&b, "## 🔮 %s%s\n\n%d%% sure.", it.Body, by, it.Confidence)
			if it.Outcome != "" {
				fmt.Fprintf(&b, " Came true: **%s**.", it.Outcome)
			}
			b.WriteString("\n\n")
		case "question":
			fmt.Fprintf(&b, "## ❓ %s%s\n\n", it.Body, by)
			if it.Answer != "" {
				fmt.Fprintf(&b, "> %s\n\n", strings.ReplaceAll(it.Answer, "\n", "\n> "))
			}
		case "goal":
			fmt.Fprintf(&b, "## 🎯 %s%s\n\n", it.Body, by)
			if it.Outcome != "" {
				fmt.Fprintf(&b, "Done: **%s**\n\n", it.Outcome)
			}
		case "list":
			fmt.Fprintf(&b, "## 📝 %s%s\n\n", firstNonEmpty(it.Title, "A list"), by)
			for _, e := range it.Entries {
				fmt.Fprintf(&b, "- **%s**: %s\n", e.Label, e.Value)
			}
			b.WriteString("\n")
		case "link":
			fmt.Fprintf(&b, "## 🔗 [%s](%s)%s\n\n%s\n\n", firstNonEmpty(it.Title, it.URL), it.URL, by, it.Body)
		}
		if it.Answer != "" && it.Kind != "question" {
			fmt.Fprintf(&b, "*On opening:* %s\n\n", it.Answer)
		}
	}
	if snap := mapOfRaw(c.Snapshot); snap != nil {
		if lines := snapshotLines(snap); len(lines) > 0 {
			b.WriteString("## 🌍 The world when it was sealed\n\n")
			for _, l := range lines {
				fmt.Fprintf(&b, "- %s\n", l)
			}
			b.WriteString("\n")
		}
	}
	if c.ThenNow != "" {
		fmt.Fprintf(&b, "## Then and now\n\n%s\n\n", c.ThenNow)
	}
	if c.Reflection != "" {
		fmt.Fprintf(&b, "## Written back after opening\n\n%s\n\n", c.Reflection)
	}
	fmt.Fprintf(&b, "---\n\n[Open in Pocket Capsule](%s)\n", s.capsuleURL(c))
	return b.String()
}

func mapOfRaw(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil || len(m) == 0 {
		return nil
	}
	return m
}

func (s *Server) openedBody(w http.ResponseWriter, r *http.Request) (*access, bool) {
	var in struct {
		CapsuleID int64 `json:"capsule_id"`
	}
	a, ok := s.capsuleBody(w, r, &in, func() int64 { return in.CapsuleID })
	if !ok {
		return nil, false
	}
	if a.c.State != store.StateOpened {
		writeErr(w, http.StatusBadRequest, "it can be kept once it is open")
		return nil, false
	}
	return a, true
}

// handleToNotes keeps an opened capsule as a note.
func (s *Server) handleToNotes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.openedBody(w, r)
	if !ok {
		return
	}
	sib, ok := s.sib(w, r, appNotes, scopeNotesWrite)
	if !ok {
		return
	}
	folder := firstNonEmpty(s.settingsOf(ctx, a.u.ID).NotesFolder, "Capsules")
	res, err := postJSONTo(ctx, sib, "/api/v1/notes", map[string]any{"title": clip(a.c.Title, 200), "content": s.capsuleMarkdown(ctx, a.c, true), "category": folder})
	if err != nil {
		s.appFail(w, appNotes, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": linkTo(sib, "/notes/"+idOf(res)), "text": "Kept in Pocket Notes"})
}

// ---- Pocket Files ----

// handleToFiles keeps a copy of an opened capsule in Pocket Files: the
// Markdown, with every picture, recording and file beside it.
func (s *Server) handleToFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	a, ok := s.openedBody(w, r)
	if !ok {
		return
	}
	sib, ok := s.sib(w, r, appFiles, scopeFilesWrite)
	if !ok {
		return
	}
	root := strings.Trim(firstNonEmpty(s.settingsOf(ctx, a.u.ID).FilesFolder, "Capsules"), "/")
	dir := root + "/" + fileSafe(a.c.Title)
	put := func(name string, data []byte, mime string) (string, error) {
		var res map[string]any
		q := url.Values{"space": {"home"}, "path": {"/"}, "name": {dir + "/" + name}, "conflict": {"rename"}, "mtime": {strconv.FormatInt(time.Now().UnixMilli(), 10)}}
		if err := sib.client.PutRaw(ctx, "/api/v1/fs/upload", q, data, mime, &res); err != nil {
			return "", err
		}
		return firstNonEmpty(str(res, "path"), "/"+dir+"/"+name), nil
	}
	md := s.capsuleMarkdown(ctx, a.c, false)
	items, _ := s.store.Items(ctx, a.c.ID)
	n := 0
	for _, it := range items {
		if it.UploadID == nil {
			continue
		}
		up, err := s.store.GetUpload(ctx, *it.UploadID)
		if err != nil {
			continue
		}
		name := fmt.Sprintf("%02d-%s", it.Position, fileSafe(firstNonEmpty(up.Name, "file")))
		if _, err := put(name, up.Data, up.Mime); err != nil {
			s.appFail(w, appFiles, err)
			return
		}
		md = strings.ReplaceAll(md, fmt.Sprintf("(/api/v1/uploads/%d)", up.ID), "("+url.PathEscape(name)+")")
		n++
	}
	path, err := put("capsule.md", []byte(md), "text/markdown")
	if err != nil {
		s.appFail(w, appFiles, err)
		return
	}
	text := "Kept in Pocket Files"
	if n > 0 {
		text = fmt.Sprintf("Kept in Pocket Files with %d %s", n, plural(n, "file", "files"))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": text, "url": linkTo(sib, "/f/home"+strings.TrimSuffix(path, "/capsule.md"))})
}

func fileSafe(v string) string {
	return firstNonEmpty(strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 32 {
			return '-'
		}
		return r
	}, clip(strings.TrimSpace(v), 90)), "capsule")
}

// ---- Pocket Voice ----

func (s *Server) handleVoiceList(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appVoice, scopeVoiceRead)
	if !ok {
		return
	}
	q := url.Values{"limit": {"40"}}
	if t := strings.TrimSpace(r.URL.Query().Get("q")); t != "" {
		q.Set("q", t)
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/memos", q)
	if err != nil {
		s.appFail(w, appVoice, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "items", "memos") {
		if m["has_audio"] == false {
			continue
		}
		out = append(out, map[string]any{"id": idOf(m), "title": firstNonEmpty(str(m, "title"), "Voice memo"), "date": day10(m["recorded_at"]), "seconds": int(num(m, "duration_ms") / 1000),
			"summary": clip(firstNonEmpty(str(m, "summary"), str(m, "transcript")), 200)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"memos": out})
}

// handleVoiceCopy puts a copy of a voice memo (the recording, and what was
// said) in a capsule.
func (s *Server) handleVoiceCopy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		CapsuleID int64  `json:"capsule_id"`
		MemoID    string `json:"memo_id"`
	}
	a, ok := s.capsuleBody(w, r, &in, func() int64 { return in.CapsuleID })
	if !ok {
		return
	}
	if can, why := a.canAdd(); !can {
		writeErr(w, http.StatusBadRequest, why)
		return
	}
	if _, err := strconv.ParseInt(in.MemoID, 10, 64); err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	sib, ok := s.sib(w, r, appVoice, scopeVoiceRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/memos/"+in.MemoID, nil)
	if err != nil {
		s.appFail(w, appVoice, err)
		return
	}
	m, _ := v.(map[string]any)
	if inner, ok := m["memo"].(map[string]any); ok {
		m = inner
	}
	b, ctype, err := sib.client.Bytes(ctx, "/api/v1/memos/"+in.MemoID+"/audio", nil, maxUpload)
	if err != nil {
		s.appFail(w, appVoice, &userErr{"Pocket Voice has no recording for that memo (or it is over 25 MB)"})
		return
	}
	mime := firstNonEmpty(strings.Split(firstNonEmpty(str(m, "audio_type"), ctype), ";")[0], "audio/webm")
	ext := map[string]string{"audio/webm": ".webm", "audio/mp4": ".m4a", "audio/mpeg": ".mp3", "audio/ogg": ".ogg", "audio/wav": ".wav"}[mime]
	title := firstNonEmpty(cleanText(str(m, "title"), 200), "Voice memo")
	up := &store.Upload{UserID: a.u.ID, CapsuleID: a.c.ID, Name: fileSafe(title) + ext, Mime: mime, Data: b}
	if err := s.store.SaveUpload(ctx, up); err != nil {
		s.fail(w, err, "memo")
		return
	}
	it := &store.Item{CapsuleID: a.c.ID, UserID: uidPtr(a.u), AuthorName: s.actorName(a.u), Kind: "audio", Title: title, Body: clip(strings.TrimSpace(str(m, "transcript")), maxNotes),
		UploadID: &up.ID, FromApp: "voice", FromURL: str(m, "url"), AfterSeal: a.c.State == store.StateSealed}
	if err := s.store.CreateItem(ctx, it); err != nil {
		s.fail(w, err, "memo")
		return
	}
	s.afterAdded(ctx, a.c, it, a.u)
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "text": "The voice memo is in the capsule"})
}

// ---- Pocket Tasks and Blog: after the opening ----

func (s *Server) openedItem(w http.ResponseWriter, r *http.Request, in any, id func() int64) (*access, *store.Item, bool) {
	if err := decode(r, in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return nil, nil, false
	}
	ctx := r.Context()
	it, err := s.store.GetItem(ctx, id())
	if err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return nil, nil, false
	}
	a, err := s.capsuleAccess(ctx, userFrom(ctx), it.CapsuleID)
	if err != nil || !a.seesItem(it) {
		writeErr(w, http.StatusNotFound, "not found")
		return nil, nil, false
	}
	if a.c.State != store.StateOpened {
		writeErr(w, http.StatusBadRequest, "open the capsule first")
		return nil, nil, false
	}
	return a, it, true
}

// handleToTasks turns a goal (or anything) from an opened capsule into a task.
func (s *Server) handleToTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		ItemID int64  `json:"item_id"`
		Title  string `json:"title"`
		Due    string `json:"due"`
	}
	a, it, ok := s.openedItem(w, r, &in, func() int64 { return in.ItemID })
	if !ok {
		return
	}
	if !validDate(in.Due) {
		writeErr(w, http.StatusBadRequest, "the date is YYYY-MM-DD")
		return
	}
	sib, ok := s.sib(w, r, appTasks, scopeTasksWrite)
	if !ok {
		return
	}
	title := firstNonEmpty(cleanText(in.Title, 200), clip(firstLine(firstNonEmpty(it.Body, it.Title)), 200))
	notes := "From the time capsule “" + a.c.Title + "”."
	task, err := sib.client.CreateTask(ctx, title, notes, in.Due, s.capsuleURL(a.c))
	if err != nil {
		s.appFail(w, appTasks, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": "Added to Pocket Tasks", "url": linkTo(sib, "/tasks/"+strconv.FormatInt(task.ID, 10))})
}

// handleToBlog turns a letter from an opened capsule into a blog post draft.
func (s *Server) handleToBlog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		ItemID int64 `json:"item_id"`
	}
	a, it, ok := s.openedItem(w, r, &in, func() int64 { return in.ItemID })
	if !ok {
		return
	}
	if it.Kind != "letter" {
		writeErr(w, http.StatusBadRequest, "a letter makes a blog post")
		return
	}
	sib, ok := s.sib(w, r, appBlog, scopeBlogWrite)
	if !ok {
		return
	}
	written := ""
	if a.c.SealedAt != nil {
		if t, err := time.Parse(time.RFC3339Nano, *a.c.SealedAt); err == nil {
			written = " in " + t.Local().Format("January 2006")
		}
	}
	title := firstNonEmpty(it.Title, "A letter from the past")
	body := it.Body + "\n\n---\n*Written" + written + " and kept in a time capsule until " + time.Now().Format("2 January 2006") + ".*\n"
	res, err := postJSONTo(ctx, sib, "/api/v1/posts", map[string]any{"title": clip(title, 200), "body": body, "excerpt": clip(firstLine(it.Body), 200), "tags": []string{"time capsule"},
		"source_ref": fmt.Sprintf("capsule:%d:%d", a.c.ID, it.ID)})
	if err != nil {
		s.appFail(w, appBlog, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": "A draft post is waiting in Pocket Blog", "url": linkTo(sib, "/posts/"+idOf(res))})
}

// ---- Pocket Alerts ----

// alert tells the person through Pocket Alerts (best effort, never in the way).
func (s *Server) alert(ctx context.Context, uid int64, title, body, path, priority string) bool {
	sib, err := s.clients(ctx, uid).conn(ctx, appAlerts, scopeAlertsWrite)
	if err != nil {
		return false
	}
	_, err = postJSONTo(ctx, sib, "/api/v1/alerts", map[string]any{"title": clip(title, 200), "body": clip(body, 2000), "priority": priority, "tags": []string{"hourglass"}, "click": s.link(path)})
	if err != nil {
		s.log.Debug("pocket alerts: send failed", "err", err)
		return false
	}
	return true
}
