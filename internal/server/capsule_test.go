package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

func (a *app) capsule(auth, title string, extra map[string]any) int64 {
	a.t.Helper()
	body := map[string]any{"title": title, "template": "self", "opens_at": time.Now().AddDate(1, 0, 0).Format("2006-01-02"), "sections": []string{}}
	for k, v := range extra {
		body[k] = v
	}
	m := a.expect(201, auth, "POST", "/api/v1/capsules", body)
	return int64(num(obj(m, "capsule"), "id"))
}

func (a *app) letter(auth string, c int64, text string) int64 {
	a.t.Helper()
	m := a.expect(201, auth, "POST", fmt.Sprintf("/api/v1/capsules/%d/items", c), map[string]any{"kind": "letter", "body": text})
	return int64(num(m, "id"))
}

func (a *app) upload(auth string, c int64) (int, int64) {
	a.t.Helper()
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("capsule", fmt.Sprint(c))
	fw, _ := mw.CreateFormFile("file", "pic.jpg")
	_, _ = fw.Write(demoPicture(nil2(), 1, 40, 30))
	_ = mw.Close()
	req, _ := http.NewRequest("POST", a.url+"/api/v1/uploads", &b)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+auth)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		a.t.Fatal(err)
	}
	defer res.Body.Close()
	var m map[string]any
	_ = json.NewDecoder(res.Body).Decode(&m)
	return res.StatusCode, int64(num(obj(m, "upload"), "id"))
}

func view(m map[string]any) (state string, items int, hidden int) {
	l, _ := m["items"].([]any)
	return str(obj(m, "capsule"), "state"), len(l), int(num(m, "hidden"))
}

// withClock moves the app's clock.
func withClock(t *testing.T, at time.Time) {
	old := now
	now = func() time.Time { return at }
	t.Cleanup(func() { now = old })
}

func TestCapsulesArePerPerson(t *testing.T) {
	a := newApp(t, nil)
	c := a.capsule(a.alice, "Alice's", nil)
	a.letter(a.alice, c, "hello")
	p := fmt.Sprintf("/api/v1/capsules/%d", c)
	// Bob — and Alice is the box admin, which gives her nothing either
	a.expect(404, a.bob, "GET", p, nil)
	a.expect(404, a.bob, "PUT", p, map[string]any{"title": "x"})
	a.expect(404, a.bob, "DELETE", p, nil)
	a.expect(404, a.bob, "POST", p+"/items", map[string]any{"kind": "letter", "body": "x"})
	a.expect(404, a.bob, "POST", p+"/seal", nil)
	a.expect(404, a.bob, "GET", p+"/history", nil)
	if l := a.ok(a.bob, "GET", "/api/v1/capsules", nil)["capsules"].([]any); len(l) != 0 {
		t.Fatalf("Bob sees %d capsules", len(l))
	}
	b := a.capsule(a.bob, "Bob's", nil)
	a.expect(404, a.alice, "GET", fmt.Sprintf("/api/v1/capsules/%d", b), nil)
	// someone else's item, by id
	a.expect(404, a.bob, "PUT", "/api/v1/items/1", map[string]any{"body": "x"})
	a.expect(404, a.bob, "DELETE", "/api/v1/items/1", nil)
}

func TestSealedMeansSealed(t *testing.T) {
	a := newApp(t, nil)
	c := a.capsule(a.alice, "Sealed", nil)
	p := fmt.Sprintf("/api/v1/capsules/%d", c)
	a.expect(400, a.alice, "POST", p+"/seal", nil) // nothing in it yet
	it := a.letter(a.alice, c, "a secret")
	code, up := a.upload(a.alice, c)
	if code != 201 {
		t.Fatalf("upload %d", code)
	}
	a.expect(201, a.alice, "POST", p+"/items", map[string]any{"kind": "photo", "upload_id": up})
	a.expect(200, a.alice, "GET", fmt.Sprintf("/api/v1/uploads/%d", up), nil)
	if _, n, _ := view(a.ok(a.alice, "GET", p, nil)); n != 2 {
		t.Fatalf("filling: %d items", n)
	}
	a.ok(a.alice, "POST", p+"/seal", nil)
	st, n, hidden := view(a.ok(a.alice, "GET", p, nil))
	if st != "sealed" || n != 0 || hidden != 2 {
		t.Fatalf("sealed: %s %d visible %d hidden", st, n, hidden)
	}
	// nothing inside, even for its maker: not the upload, not the item
	a.expect(404, a.alice, "GET", fmt.Sprintf("/api/v1/uploads/%d", up), nil)
	a.expect(403, a.alice, "PUT", fmt.Sprintf("/api/v1/items/%d", it), map[string]any{"body": "changed"})
	a.expect(403, a.alice, "DELETE", fmt.Sprintf("/api/v1/items/%d", it), nil)
	a.expect(400, a.alice, "POST", p+"/open", nil) // not its day
	// the search and the day page do not read inside either
	if l := a.ok(a.alice, "GET", "/api/v1/search?q=secret", nil)["results"].([]any); len(l) != 0 {
		t.Fatalf("search found %v inside a sealed capsule", l)
	}
	// an export does not show it at a glance
	code2, _, _ := a.call(a.alice, "GET", "/api/v1/export.json", nil)
	if code2 != 200 {
		t.Fatal(code2)
	}
	doc, _ := a.s.export(context.Background(), &store.User{ID: a.ids["Alice"]})
	raw, _ := json.Marshal(doc)
	if strings.Contains(string(raw), "a secret") {
		t.Fatal("the export shows what is inside a sealed capsule")
	}
	// the slot: still something can go in, unseen
	m := a.expect(201, a.alice, "POST", p+"/items", map[string]any{"kind": "letter", "body": "through the slot"})
	if m["hidden"] != true {
		t.Fatalf("slot item came back visible: %v", m)
	}
	// breaking the seal: typed, and on record
	a.expect(400, a.alice, "POST", p+"/break", map[string]any{"confirm": "yes"})
	a.expect(404, a.bob, "POST", p+"/break", map[string]any{"confirm": "open"}) // not in it at all
	m = a.ok(a.alice, "POST", p+"/break", map[string]any{"confirm": "open", "reason": "could not wait"})
	if st, n, _ := view(m); st != "opened" || n != 3 || obj(m, "capsule")["early"] != true {
		t.Fatalf("after breaking: %s %d %v", st, n, obj(m, "capsule")["early"])
	}
	a.expect(200, a.alice, "GET", fmt.Sprintf("/api/v1/uploads/%d", up), nil)
}

func TestSurpriseContributorsAndOpening(t *testing.T) {
	a := newApp(t, nil)
	c := a.capsule(a.alice, "Family", map[string]any{"template": "family", "surprise": true})
	p := fmt.Sprintf("/api/v1/capsules/%d", c)
	a.expect(400, a.alice, "POST", p+"/members", map[string]any{"email": "nobody@example.com"})
	a.ok(a.alice, "POST", p+"/members", map[string]any{"email": "bob@example.com", "role": "contributor"})
	a.letter(a.alice, c, "from alice")
	bobs := a.letter(a.bob, c, "from bob")
	// a surprise: each sees only their own
	if _, n, hidden := view(a.ok(a.alice, "GET", p, nil)); n != 1 || hidden != 1 {
		t.Fatalf("alice sees %d, %d hidden", n, hidden)
	}
	if _, n, _ := view(a.ok(a.bob, "GET", p, nil)); n != 1 {
		t.Fatalf("bob sees %d", n)
	}
	// Bob cannot seal or change the capsule, Alice cannot change Bob's letter
	a.expect(403, a.bob, "POST", p+"/seal", nil)
	a.expect(403, a.bob, "PUT", p, map[string]any{"title": "mine now"})
	a.expect(403, a.alice, "PUT", fmt.Sprintf("/api/v1/items/%d", bobs), map[string]any{"body": "x"})
	a.ok(a.bob, "PUT", fmt.Sprintf("/api/v1/items/%d", bobs), map[string]any{"body": "from bob, edited"})
	a.ok(a.alice, "POST", p+"/seal", nil)

	// the day comes: the janitor tells, then Bob may open it
	later := time.Now().AddDate(1, 0, 2)
	withClock(t, later)
	a.s.openingDays(context.Background())
	m := a.ok(a.bob, "GET", p, nil)
	if m["ready"] != true || m["can_open"] != true {
		t.Fatalf("not ready on its day: %v %v", m["ready"], m["why_not_open"])
	}
	m = a.ok(a.bob, "POST", p+"/open", nil)
	if st, n, _ := view(m); st != "opened" || n != 2 {
		t.Fatalf("opened: %s %d", st, n)
	}
	// after the opening: outcomes and answers, not the words themselves
	a.ok(a.alice, "PUT", fmt.Sprintf("/api/v1/items/%d", bobs), map[string]any{"answer": "still true"})
	it := a.ok(a.bob, "PUT", fmt.Sprintf("/api/v1/items/%d", bobs), map[string]any{"body": "rewritten"})
	if it["body"] != "from bob, edited" {
		t.Fatalf("the words changed after the opening: %v", it["body"])
	}
	a.ok(a.alice, "PUT", p+"/reflection", map[string]any{"reflection": "what a year"})
	h := a.ok(a.alice, "GET", p+"/history", nil)["history"].([]any)
	whats := []string{}
	for _, e := range h {
		whats = append(whats, str(e.(map[string]any), "what"))
	}
	for _, w := range []string{"made", "member", "added", "sealed", "ready", "opened", "reflected"} {
		if !strings.Contains(strings.Join(whats, " "), w) {
			t.Fatalf("history misses %q: %v", w, whats)
		}
	}
}

func TestPredictionsScoreboard(t *testing.T) {
	a := newApp(t, nil)
	c := a.capsule(a.alice, "Guesses", nil)
	p := fmt.Sprintf("/api/v1/capsules/%d", c)
	g := a.expect(201, a.alice, "POST", p+"/items", map[string]any{"kind": "prediction", "body": "It snows", "confidence": 90})
	a.expect(400, a.alice, "POST", p+"/items", map[string]any{"kind": "prediction", "body": ""})
	a.ok(a.alice, "POST", p+"/seal", nil)
	a.ok(a.alice, "POST", p+"/break", map[string]any{"confirm": "open"})
	a.expect(400, a.alice, "PUT", fmt.Sprintf("/api/v1/items/%d", int64(num(g, "id"))), map[string]any{"outcome": "maybe"})
	a.ok(a.alice, "PUT", fmt.Sprintf("/api/v1/items/%d", int64(num(g, "id"))), map[string]any{"outcome": "yes"})
	m := a.ok(a.alice, "GET", "/api/v1/predictions", nil)
	if num(m, "marked") != 1 || num(m, "right_pct") != 100 {
		t.Fatalf("scoreboard: %v", m)
	}
	if l := a.ok(a.bob, "GET", "/api/v1/predictions", nil)["predictions"].([]any); len(l) != 0 {
		t.Fatal("Bob sees Alice's guesses")
	}
}

func TestGiveLinkAndRecipients(t *testing.T) {
	a := newApp(t, nil)
	c := a.capsule(a.alice, "For Mila", map[string]any{"recipients": []map[string]string{{"name": "Mila", "email": "mila@example.com"}}})
	p := fmt.Sprintf("/api/v1/capsules/%d", c)
	a.letter(a.alice, c, "for you")
	m := a.ok(a.alice, "PUT", p+"/give", map[string]any{"open": true})
	give := str(m, "give_url")
	if give == "" {
		t.Fatal("no give link")
	}
	key := give[strings.LastIndex(give, "/")+1:]
	// a guest adds a letter
	form := func(name, letter string) int {
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		_ = mw.WriteField("name", name)
		_ = mw.WriteField("letter", letter)
		_ = mw.Close()
		req := httptest.NewRequest("POST", "/api/v1/public/give/"+key, &b)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		rec := httptest.NewRecorder()
		a.s.Router().ServeHTTP(rec, req)
		return rec.Code
	}
	if code := form("", "no name"); code != 400 {
		t.Fatalf("nameless letter → %d", code)
	}
	if code := form("Nona", "from nona"); code != 201 {
		t.Fatalf("guest letter → %d", code)
	}
	if _, n, hidden := view(a.ok(a.alice, "GET", p, nil)); n != 1 || hidden != 1 {
		t.Fatalf("guest letter not a surprise: %d / %d", n, hidden)
	}
	for i := 0; i < 12; i++ {
		form("Spam", "x")
	}
	if code := form("Spam", "x"); code != 429 {
		t.Fatalf("no rate limit: %d", code)
	}
	// the recipient's link: the outside before, everything after
	recs := a.ok(a.alice, "GET", p, nil)["recipients"].([]any)
	if len(recs) != 1 {
		t.Fatalf("recipients %v", recs)
	}
	link := str(recs[0].(map[string]any), "url")
	rkey := link[strings.LastIndex(link, "/")+1:]
	pub := a.expect(200, "", "GET", "/api/v1/public/o/"+rkey, nil)
	if pub["open"] != false || pub["items"] != nil {
		t.Fatalf("recipient sees inside before the opening: %v", pub)
	}
	a.ok(a.alice, "POST", p+"/seal", nil)
	a.ok(a.alice, "POST", p+"/break", map[string]any{"confirm": "open"})
	pub = a.expect(200, "", "GET", "/api/v1/public/o/"+rkey, nil)
	if pub["open"] != true || len(pub["items"].([]any)) < 1 {
		t.Fatalf("recipient cannot open it: %v", pub)
	}
	a.expect(404, "", "GET", "/api/v1/public/o/"+strings.Repeat("0", 36), nil)
	// once opened, the give link takes nothing
	if code := form("Late", "too late"); code != 400 && code != 429 {
		t.Fatalf("late guest → %d", code)
	}
}

func TestLinkScopes(t *testing.T) {
	r := func(method, path string) *http.Request { return httptest.NewRequest(method, path, nil) }
	read, write := []string{"capsule:read"}, []string{"capsule:write"}
	cases := []struct {
		scopes []string
		req    *http.Request
		want   bool
	}{
		{read, r("GET", "/api/v1/auth/me"), true},
		{read, r("GET", "/api/v1/capsules"), true},
		{read, r("GET", "/api/v1/capsules/3"), true},
		{read, r("GET", "/api/v1/day/2026-11-10"), true},
		{read, r("GET", "/api/v1/templates"), true},
		{read, r("POST", "/api/v1/capsules"), false},
		{write, r("POST", "/api/v1/capsules"), true},
		{write, r("POST", "/api/v1/capsules/3/items"), true},
		{write, r("PUT", "/api/v1/items/5"), true},
		{write, r("POST", "/api/v1/capsules/3/seal"), false},
		{write, r("POST", "/api/v1/capsules/3/open"), false},
		{write, r("POST", "/api/v1/capsules/3/break"), false},
		{append(read, write...), r("DELETE", "/api/v1/capsules/3"), false},
		{append(read, write...), r("GET", "/api/v1/export.json"), false},
		{append(read, write...), r("GET", "/api/v1/settings"), false},
	}
	for _, c := range cases {
		if got := linkAllows(c.scopes, c.req); got != c.want {
			t.Errorf("%v %s %s = %v, want %v", c.scopes, c.req.Method, c.req.URL.Path, got, c.want)
		}
	}
	// a linked app changes only what it put in itself
	ctx := withLinkApp(context.Background(), "voice")
	if !mayChange(ctx, "voice") || mayChange(ctx, "") || mayChange(ctx, "photos") {
		t.Fatal("mayChange")
	}
}

func TestExportImportIsIdempotent(t *testing.T) {
	a := newApp(t, nil)
	c := a.capsule(a.alice, "Keep me", nil)
	a.letter(a.alice, c, "kept")
	a.upload(a.alice, c)
	s2 := a.capsule(a.alice, "Sealed one", nil)
	a.letter(a.alice, s2, "sealed words")
	a.ok(a.alice, "POST", fmt.Sprintf("/api/v1/capsules/%d/seal", s2), nil)
	a.ok(a.alice, "POST", "/api/v1/rituals", map[string]any{"title": "New Year", "template": "newyear", "month": 12, "day": 31})
	doc, err := a.s.export(context.Background(), &store.User{ID: a.ids["Alice"]})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(doc)
	imp := func() map[string]any {
		req, _ := http.NewRequest("POST", a.url+"/api/v1/import", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+a.bob)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		if res.StatusCode != 200 {
			t.Fatalf("import %d %s", res.StatusCode, b)
		}
		return m
	}
	m := imp()
	if num(m, "capsules") != 2 || num(m, "items") != 2 {
		t.Fatalf("first import: %v", m)
	}
	m = imp()
	if num(m, "capsules") != 0 || num(m, "skipped") != 2 {
		t.Fatalf("second import: %v", m)
	}
	list := a.ok(a.bob, "GET", "/api/v1/capsules", nil)["capsules"].([]any)
	if len(list) != 2 {
		t.Fatalf("Bob has %d capsules", len(list))
	}
	// the sealed one stays sealed in its new home
	for _, x := range list {
		cp := obj(x.(map[string]any), "capsule")
		if str(cp, "title") == "Sealed one" {
			_, n, hidden := view(a.ok(a.bob, "GET", fmt.Sprintf("/api/v1/capsules/%d", int64(num(cp, "id"))), nil))
			if n != 0 || hidden != 1 {
				t.Fatalf("imported sealed capsule shows %d / %d", n, hidden)
			}
		}
	}
	if l := a.ok(a.bob, "GET", "/api/v1/rituals", nil)["rituals"].([]any); len(l) != 1 {
		t.Fatalf("rituals %v", l)
	}
}

func TestSettingsArePersonal(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"open_hour": 7, "sections": []string{"weather", "wealth", "nonsense"}, "notes_folder": "Time"})
	m := a.ok(a.alice, "GET", "/api/v1/settings", nil)
	if num(m, "open_hour") != 7 || str(m, "notes_folder") != "Time" || len(m["sections"].([]any)) != 2 {
		t.Fatalf("alice: %v %v %v", m["open_hour"], m["notes_folder"], m["sections"])
	}
	m = a.ok(a.bob, "GET", "/api/v1/settings", nil)
	if num(m, "open_hour") != 9 || str(m, "notes_folder") != "Capsules" {
		t.Fatalf("bob got alice's settings: %v %v", m["open_hour"], m["notes_folder"])
	}
	a.expect(400, a.alice, "PUT", "/api/v1/settings", map[string]any{"open_hour": 30})
}

func TestOpeningDayRules(t *testing.T) {
	from := time.Date(2026, 11, 10, 12, 0, 0, 0, time.Local)
	d, ok := ruleOpens("2024-05-14", 18, 9, from)
	if !ok || d.Format("2006-01-02 15") != "2042-05-14 09" {
		t.Fatalf("18th birthday: %v", d)
	}
	d, _ = ruleOpens("1990-03-01", 0, 9, from)
	if d.Format("2006-01-02") != "2027-03-01" {
		t.Fatalf("next birthday: %v", d)
	}
	if w := untilWords(from.AddDate(2, 3, 0), from); w != "in 2 years, 3 months" {
		t.Fatalf("untilWords %q", w)
	}
	a := newApp(t, nil)
	a.expect(400, a.alice, "POST", "/api/v1/capsules", map[string]any{"title": "Past", "opens_at": "2001-01-01"})
	m := a.expect(201, a.alice, "POST", "/api/v1/capsules", map[string]any{"title": "Mila", "template": "child", "for_name": "Mila", "for_date": "2024-05-14"})
	if !strings.HasPrefix(str(obj(m, "capsule"), "opens_at"), "2042-05-14") {
		t.Fatalf("child capsule opens %v", obj(m, "capsule")["opens_at"])
	}
	// sealed: later is fine, sooner is not
	c := int64(num(obj(m, "capsule"), "id"))
	a.letter(a.alice, c, "x")
	a.ok(a.alice, "POST", fmt.Sprintf("/api/v1/capsules/%d/seal", c), nil)
	a.expect(400, a.alice, "PUT", fmt.Sprintf("/api/v1/capsules/%d", c), map[string]any{"opens_at": "2030-01-01", "open_rule": "date"})
	a.ok(a.alice, "PUT", fmt.Sprintf("/api/v1/capsules/%d", c), map[string]any{"opens_at": "2050-01-01", "open_rule": "date"})
}

func nil2() *rand.Rand { return rand.New(rand.NewSource(1)) }
