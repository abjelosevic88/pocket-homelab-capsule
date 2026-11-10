package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"math/rand"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

// seedDemo fills the owner's account with capsules in every state: one
// ready to open today, one opening soon, a child's capsule sealed for
// years with its slot open, a family capsule being filled, and opened
// ones with letters, photos, guesses that came true (or did not), goals,
// lists and a letter back. Everything relative to today.
func (s *Server) seedDemo(ctx context.Context, owner int64) error {
	ctx = context.WithoutCancel(ctx)
	if n, _ := s.store.OwnCapsules(ctx, owner); len(n) > 0 {
		return nil
	}
	rng := rand.New(rand.NewSource(42))
	u, err := s.store.GetUser(ctx, owner)
	if err != nil {
		return err
	}
	me := s.actorName(u)
	ana := s.demoUser(ctx, "ana@example.com", "Ana")
	t := now()
	at := func(d time.Time) string { return d.UTC().Format(time.RFC3339Nano) }
	ptr := func(d time.Time) *string { v := at(d); return &v }
	days := func(n int) time.Time { return t.AddDate(0, 0, n) }
	atHour := func(d time.Time, h int) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day(), h, 0, 0, 0, time.Local)
	}

	type item struct {
		kind, title, body, outcome, answer, author string
		conf                                       int
		entries                                    []store.Entry
		photo                                      int // 0 = none, else a picture style
		audio                                      bool
		guest, after                               bool
		uid                                        int64
	}
	snapWeather := func(place, cond string, temp float64) map[string]any {
		return map[string]any{"place": place, "condition": cond, "emoji": weatherEmoji(cond), "temp": temp, "text": fmt.Sprintf("%s, %.0f °C", cond, temp)}
	}
	add := func(c *store.Capsule, made time.Time, items []item, members map[int64]string, recipients [][2]string) error {
		c.UserID = owner
		if c.Emoji == "" {
			c.Emoji = templateByID(c.Template).Emoji
		}
		if c.OpenRule == "" {
			c.OpenRule = "date"
		}
		if c.Deliver == "" {
			c.Deliver = "on_open"
		}
		if c.Sections == nil {
			c.Sections = append([]string(nil), store.DefaultSections...)
		}
		c.CreatedAt = at(made)
		c.ReadySent = c.State == store.StateOpened
		c.RemindSent = c.State == store.StateOpened
		if err := s.store.CreateCapsule(ctx, c); err != nil {
			return err
		}
		s.store.LogAt(ctx, c.ID, &owner, me, "made", "", at(made))
		for uid, role := range members {
			_ = s.store.SetMember(ctx, c.ID, uid, role)
		}
		for _, rc := range recipients {
			_ = s.store.AddRecipient(ctx, &store.Recipient{UserID: owner, CapsuleID: c.ID, Name: rc[0], Email: rc[1]})
		}
		for i, x := range items {
			when := made.Add(time.Duration(i*3+1) * time.Hour)
			if x.after && c.SealedAt != nil {
				sealed, _ := time.Parse(time.RFC3339Nano, *c.SealedAt)
				when = sealed.AddDate(0, i%11+1, 0)
			}
			it := &store.Item{CapsuleID: c.ID, Kind: x.kind, Title: x.title, Body: x.body, Outcome: x.outcome, Answer: x.answer, Confidence: x.conf, Entries: x.entries,
				AuthorName: firstNonEmpty(x.author, me), Guest: x.guest, AfterSeal: x.after, CreatedAt: at(when)}
			if !x.guest {
				id := firstPos64(x.uid, owner)
				it.UserID = &id
			}
			if x.photo > 0 {
				data, thumb := demoPicture(rng, x.photo, 960, 720), demoPicture(rand.New(rand.NewSource(int64(x.photo))), x.photo, 320, 240)
				up := &store.Upload{UserID: owner, CapsuleID: c.ID, Name: fmt.Sprintf("photo-%d.jpg", i+1), Mime: "image/jpeg", Width: 960, Height: 720, Data: data, Thumb: thumb}
				if err := s.store.SaveUpload(ctx, up); err != nil {
					return err
				}
				it.UploadID = &up.ID
			}
			if x.audio {
				up := &store.Upload{UserID: owner, CapsuleID: c.ID, Name: "hello.wav", Mime: "audio/wav", Data: demoChime()}
				if err := s.store.SaveUpload(ctx, up); err != nil {
					return err
				}
				it.UploadID = &up.ID
			}
			if err := s.store.CreateItem(ctx, it); err != nil {
				return err
			}
			detail := kindWord(x.kind)
			if x.after {
				detail += " through the slot"
			}
			s.store.LogAt(ctx, c.ID, it.UserID, it.AuthorName, "added", detail, at(when))
		}
		if c.SealedAt != nil {
			s.store.LogAt(ctx, c.ID, &owner, me, "sealed", "opens "+c.Opens().Local().Format("2 Jan 2006"), *c.SealedAt)
		}
		if c.State == store.StateOpened && c.OpenedAt != nil {
			what := "opened"
			if c.Early {
				what = "early"
			}
			s.store.LogAt(ctx, c.ID, &owner, me, what, c.EarlyReason, *c.OpenedAt)
		}
		return nil
	}

	// 1. ready to open today
	sealed1 := days(-400)
	c1 := &store.Capsule{Title: "A letter to me, a year on", Template: "self", About: "Written on a rainy Sunday after the move.", State: store.StateSealed,
		OpensAt: at(atHour(days(-1), 9)), SealedAt: ptr(sealed1), RemindDays: 7, Surprise: true, Slot: true,
		Snapshot: mustJSON(map[string]any{"taken_at": at(sealed1), "weather": snapWeather("Sarajevo", "rain", 9), "reading": []map[string]any{{"title": "The Overstory", "author": "Richard Powers"}},
			"listening": map[string]any{"artists": []map[string]any{{"name": "Bon Iver", "plays": 41}, {"name": "Dubioza kolektiv", "plays": 22}}, "plays": 310},
			"places":    map[string]any{"year": sealed1.Year(), "countries": []string{"Bosnia and Herzegovina", "Croatia", "Austria"}, "places": 64, "cities": 9, "top": []string{"Home", "Office", "Vijećnica"}}})}
	if err := add(c1, days(-402), []item{
		{kind: "letter", title: "Hello, future me", body: "We just moved into the flat on the hill. The boxes are still in the hall and the kitchen smells of new paint. I am a bit scared about the new job and very happy about the light in the mornings.\n\nI hope by the time you read this the bookshelf is up, you have found a good bakery and you are still taking the long way home along the river."},
		{kind: "prediction", body: "I will still be in the same job", conf: 70},
		{kind: "prediction", body: "We will have a dog", conf: 30},
		{kind: "question", body: "Did you ever fix the squeaky balcony door?"},
		{kind: "goal", body: "Run a 10 km race"},
		{kind: "list", title: "Right now", entries: []store.Entry{{Label: "Favourite song", Value: "Holocene"}, {Label: "Favourite food", Value: "Grandma's burek"}, {Label: "What I do most evenings", Value: "Unpacking, mostly"}, {Label: "Something I want to learn", Value: "Guitar"}}},
		{kind: "photo", title: "The view on the first day", photo: 1},
	}, nil, nil); err != nil {
		return err
	}
	c1.ReadySent = true
	_ = s.store.UpdateCapsule(ctx, c1)
	s.store.LogAt(ctx, c1.ID, nil, "Pocket Capsule", "ready", "", at(atHour(days(-1), 9)))

	// 2. opens in ten days
	sealed2 := days(-355)
	c2 := &store.Capsule{Title: "Birthday letter", Template: "birthday", OpenRule: "date", About: "From my last birthday to the next.", State: store.StateSealed,
		OpensAt: at(atHour(days(10), 9)), SealedAt: ptr(sealed2), RemindDays: 14, Slot: true, Color: "#c2588a",
		Snapshot: mustJSON(map[string]any{"taken_at": at(sealed2), "weather": snapWeather("Mostar", "clear", 24)})}
	if err := add(c2, sealed2.Add(-2*time.Hour), []item{
		{kind: "letter", title: "Another year", body: "Dinner on the terrace with everyone. Ana made the walnut cake again."},
		{kind: "prediction", body: "I will celebrate the next one somewhere by the sea", conf: 55},
		{kind: "photo", title: "The cake", photo: 4},
	}, nil, nil); err != nil {
		return err
	}

	// 3. a child's capsule, sealed for years, the slot still open
	sealed3 := days(-560)
	birth := time.Date(t.Year()-2, 5, 14, 0, 0, 0, 0, time.Local)
	c3 := &store.Capsule{Title: "For Mila, on her 18th", Template: "child", OpenRule: "birthday", ForName: "Mila", ForDate: birth.Format("2006-01-02"), ForAge: 18,
		About: "Letters, photos and voices from your first years.", State: store.StateSealed, OpensAt: at(time.Date(birth.Year()+18, 5, 14, 9, 0, 0, 0, time.Local)),
		SealedAt: ptr(sealed3), RemindDays: 30, Surprise: true, Slot: true, GiveOpen: true, Color: "#d5a154",
		Snapshot: mustJSON(map[string]any{"taken_at": at(sealed3), "weather": snapWeather("Sarajevo", "partly", 18)})}
	if err := add(c3, days(-600), []item{
		{kind: "letter", title: "The day you were born", body: "It was a Tuesday and it snowed in May. You were tiny and furious and perfect."},
		{kind: "photo", title: "Your first walk", photo: 2},
		{kind: "audio", title: "Your first laugh", audio: true},
		{kind: "letter", title: "From your mum", body: "Right now you fall asleep holding my finger. I hope you still let me hold your hand sometimes.", author: "Ana", uid: ana},
		{kind: "list", title: "Your favourites", entries: []store.Entry{{Label: "Toy", Value: "The blue teddy"}, {Label: "Food", Value: "Banana"}, {Label: "Word", Value: "Mama"}}},
		{kind: "letter", title: "From your nona", body: "My dear Mila, today you ate a whole plate of my pita and asked for more.", author: "Nona Vesna", guest: true, after: true},
		{kind: "photo", title: "Second birthday", photo: 5, after: true},
	}, map[int64]string{ana: "contributor"}, [][2]string{{"Mila", "mila@example.com"}}); err != nil {
		return err
	}

	// 4. a family capsule being filled
	c4 := &store.Capsule{Title: "Family capsule " + fmt.Sprint(t.Year()), Template: "family", About: "Everyone adds something; we open it together in five years.", State: store.StateFilling,
		OpensAt: at(time.Date(t.Year()+5, 6, 1, 18, 0, 0, 0, time.Local)), RemindDays: 7, Surprise: true, Slot: true, GiveOpen: true, Color: "#4a7a5e"}
	if err := add(c4, days(-12), []item{
		{kind: "letter", title: "What we are like right now", body: "Sunday lunches at ours, too much ćevapi, the cat on the warm laptop. Everyone talks at once and nobody minds."},
		{kind: "prediction", body: "We will still live in this house", conf: 60},
		{kind: "list", title: "What things cost", entries: []store.Entry{{Label: "A loaf of bread", Value: "1.60 KM"}, {Label: "A litre of milk", Value: "2.20 KM"}, {Label: "A coffee in town", Value: "2.50 KM"}, {Label: "A litre of petrol", Value: "2.45 KM"}}},
		{kind: "photo", title: "Lunch on the balcony", photo: 3},
		{kind: "letter", title: "Something I never said", body: "Thank you for every Sunday.", author: "Ana", uid: ana},
	}, map[int64]string{ana: "contributor"}, [][2]string{{"Ana", "ana@example.com"}}); err != nil {
		return err
	}

	// 5. opened last New Year's Day, guesses marked
	sealed5 := time.Date(t.Year()-1, 1, 1, 0, 30, 0, 0, time.Local)
	opened5 := time.Date(t.Year(), 1, 1, 11, 0, 0, 0, time.Local)
	c5 := &store.Capsule{Title: fmt.Sprintf("New Year %d", t.Year()-1), Template: "newyear", State: store.StateOpened, OpensAt: at(time.Date(t.Year(), 1, 1, 0, 0, 0, 0, time.Local)),
		SealedAt: ptr(sealed5), OpenedAt: ptr(opened5), OpenedBy: &owner, About: "Hopes for the year, written just after midnight.",
		Reflection: "Funny to read this. I worried about all the wrong things. The year was kinder than I thought, and the guitar is still in its case.",
		ThenNow:    "You sealed this in the snow at -3 °C and opened it on a mild, grey morning. You guessed right about the trip and the books, wrong about March, and the job you worried about turned into the part of the year you liked most.",
		Snapshot: mustJSON(map[string]any{"taken_at": at(sealed5), "weather": snapWeather("Sarajevo", "snow", -3),
			"reading":   []map[string]any{{"title": "Tomorrow, and Tomorrow, and Tomorrow", "author": "Gabrielle Zevin"}, {"title": "Na Drini ćuprija", "author": "Ivo Andrić"}},
			"listening": map[string]any{"artists": []map[string]any{{"name": "Fleetwood Mac", "plays": 58}, {"name": "Arsen Dedić", "plays": 31}, {"name": "Khruangbin", "plays": 27}}, "plays": 402},
			"places":    map[string]any{"year": t.Year() - 2, "countries": []string{"Bosnia and Herzegovina", "Italy", "Montenegro", "Slovenia"}, "places": 112, "cities": 17, "top": []string{"Home", "Office", "Baščaršija"}}})}
	if err := add(c5, sealed5.Add(-40*time.Minute), []item{
		{kind: "letter", title: "Three things for this year", body: "1. Travel somewhere new, just the two of us.\n2. Read twenty books.\n3. Be less tired.\n\nLast year was loud. I want this one to be slower.", answer: "Two out of three, and I am not tired. Mostly."},
		{kind: "prediction", body: "I will keep my resolution past March", conf: 40, outcome: "no"},
		{kind: "prediction", body: "We will go to Portugal", conf: 75, outcome: "yes"},
		{kind: "prediction", body: "I will read more than 20 books", conf: 50, outcome: "yes"},
		{kind: "prediction", body: "The biggest change this year will be at work", conf: 85, outcome: "partly"},
		{kind: "goal", body: "Learn ten songs on the guitar", outcome: "no"},
		{kind: "goal", body: "Cook a new recipe every month", outcome: "partly"},
		{kind: "question", body: "What surprised you most this year?", answer: "How much I liked the new team."},
		{kind: "list", title: "Goals for the year", entries: []store.Entry{{Label: "Health", Value: "Swim twice a week"}, {Label: "Work", Value: "Lead the redesign"}, {Label: "Money", Value: "Save for the trip"}, {Label: "People", Value: "Call grandma every Sunday"}}},
		{kind: "photo", title: "Fireworks from the balcony", photo: 6},
	}, nil, nil); err != nil {
		return err
	}

	// 6. ten years on: opened this summer, lots of guesses
	sealed6 := time.Date(t.Year()-10, 6, 20, 15, 0, 0, 0, time.Local)
	opened6 := time.Date(t.Year(), 6, 20, 19, 0, 0, 0, time.Local)
	c6 := &store.Capsule{Title: "Class of " + fmt.Sprint(t.Year()-10), Template: "graduation", State: store.StateOpened, OpensAt: at(time.Date(t.Year(), 6, 20, 18, 0, 0, 0, time.Local)),
		SealedAt: ptr(sealed6), OpenedAt: ptr(opened6), OpenedBy: &owner, About: "Ten years after the last exam.", Color: "#3d6fb0",
		Reflection: "Half of us guessed we would live abroad. Two of us do."}
	guesses := []item{
		{kind: "prediction", body: "I will live in another city", conf: 90, outcome: "yes"},
		{kind: "prediction", body: "I will have a job I can't imagine yet", conf: 80, outcome: "yes"},
		{kind: "prediction", body: "I will still play basketball on Thursdays", conf: 65, outcome: "no"},
		{kind: "prediction", body: "I will be married", conf: 45, outcome: "yes"},
		{kind: "prediction", body: "I will speak German", conf: 70, outcome: "partly"},
		{kind: "prediction", body: "Phones will fold", conf: 20, outcome: "yes"},
		{kind: "prediction", body: "We will all meet again for this", conf: 95, outcome: "partly"},
		{kind: "prediction", body: "I will have written a book", conf: 15, outcome: "no"},
		{kind: "prediction", body: "I will own a car", conf: 60, outcome: "yes"},
		{kind: "prediction", body: "I will have been to Japan", conf: 35, outcome: "no"},
		{kind: "letter", title: "To us in ten years", body: "We sat on the school steps all afternoon. Nobody wanted to go home first."},
		{kind: "photo", title: "The school steps", photo: 7},
		{kind: "question", body: "Who are you still in touch with?", answer: "Amra, Dino and — surprisingly — Mr Hadžić."},
	}
	if err := add(c6, sealed6.Add(-3*time.Hour), guesses, nil, nil); err != nil {
		return err
	}

	// 7. a team capsule opened early
	sealed7 := days(-200)
	c7 := &store.Capsule{Title: "Launch day", Template: "team", State: store.StateOpened, OpensAt: at(days(165)), SealedAt: ptr(sealed7), OpenedAt: ptr(days(-20)), OpenedBy: &owner,
		Early: true, EarlyReason: "The project ended sooner than planned.", About: "What the team expected, before the first release."}
	if err := add(c7, sealed7.Add(-time.Hour), []item{
		{kind: "prediction", body: "We ship by the end of the year", conf: 70, outcome: "yes"},
		{kind: "prediction", body: "We will have more than 1000 users", conf: 55, outcome: "no"},
		{kind: "letter", title: "What I hope we are proud of", body: "That it is simple. That my mum could use it."},
	}, nil, nil); err != nil {
		return err
	}

	// yearly capsules
	_ = s.store.SaveRitual(ctx, &store.Ritual{UserID: owner, Title: "New Year's capsule", Template: "newyear", Month: 12, Day: 31, Years: 1, Active: true})
	_ = s.store.SaveRitual(ctx, &store.Ritual{UserID: owner, Title: "Birthday letter", Template: "birthday", Month: 11, Day: 20, Years: 1, Active: true})
	s.log.Info("demo data added", "capsules", 7)
	return nil
}

func firstPos64(v ...int64) int64 {
	for _, x := range v {
		if x > 0 {
			return x
		}
	}
	return 0
}

// demoUser is a second person on the box for the shared capsules (no password: cannot sign in).
func (s *Server) demoUser(ctx context.Context, email, name string) int64 {
	if u, err := s.store.GetUserByEmail(ctx, email); err == nil {
		return u.ID
	}
	u := &store.User{Email: email, Name: name, Role: "member"}
	if err := s.store.CreateUser(ctx, u); err != nil {
		return 0
	}
	return u.ID
}

// demoPicture draws a small landscape: a sky, a sun or moon, hills, a
// little house — different colours for each style.
func demoPicture(rng *rand.Rand, style, w, h int) []byte {
	palettes := [][4]color.RGBA{
		{{255, 196, 140, 255}, {255, 236, 200, 255}, {104, 140, 96, 255}, {64, 96, 70, 255}},
		{{150, 200, 240, 255}, {225, 240, 252, 255}, {120, 170, 100, 255}, {80, 130, 80, 255}},
		{{250, 220, 170, 255}, {255, 248, 230, 255}, {200, 170, 120, 255}, {150, 120, 90, 255}},
		{{120, 170, 210, 255}, {200, 225, 240, 255}, {90, 140, 120, 255}, {60, 100, 90, 255}},
		{{240, 170, 190, 255}, {255, 225, 230, 255}, {170, 120, 150, 255}, {120, 80, 110, 255}},
		{{20, 24, 60, 255}, {70, 60, 120, 255}, {40, 40, 70, 255}, {25, 25, 45, 255}},
		{{140, 190, 230, 255}, {230, 240, 250, 255}, {170, 160, 150, 255}, {120, 110, 100, 255}},
	}
	p := palettes[(style-1)%len(palettes)]
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	sunX, sunY, sunR := float64(w)*(0.2+0.6*rng.Float64()), float64(h)*(0.2+0.15*rng.Float64()), float64(h)*0.09
	phase1, phase2 := rng.Float64()*6, rng.Float64()*6
	for y := 0; y < h; y++ {
		f := float64(y) / float64(h)
		sky := mix(p[0], p[1], f*1.4)
		for x := 0; x < w; x++ {
			c := sky
			fx := float64(x) / float64(w)
			if d := math.Hypot(float64(x)-sunX, float64(y)-sunY); d < sunR {
				c = color.RGBA{255, 250, 225, 255}
				if style == 6 {
					c = color.RGBA{235, 235, 215, 255}
				}
			} else if d < sunR*1.6 {
				c = mix(c, color.RGBA{255, 250, 230, 255}, (sunR*1.6-d)/(sunR*0.6)*0.35)
			}
			hill1 := 0.62 + 0.08*math.Sin(fx*5+phase1)
			hill2 := 0.75 + 0.06*math.Sin(fx*8+phase2)
			if f > hill1 {
				c = mix(p[2], p[3], (f-hill1)*2)
			}
			if f > hill2 {
				c = mix(p[3], color.RGBA{30, 40, 30, 255}, (f-hill2)*1.5)
			}
			img.SetRGBA(x, y, c)
		}
	}
	// a little house on the hill
	hx, hy, hw := int(float64(w)*0.62), int(float64(h)*0.6), w/12
	for y := hy; y < hy+hw; y++ {
		for x := hx; x < hx+hw; x++ {
			img.SetRGBA(x, y, color.RGBA{240, 232, 220, 255})
		}
	}
	for i := 0; i < hw/2+2; i++ {
		for x := hx - 2 + i; x < hx+hw+2-i; x++ {
			img.SetRGBA(x, hy-i, color.RGBA{170, 70, 60, 255})
		}
	}
	for y := hy + hw/3; y < hy+hw/3+hw/4; y++ {
		for x := hx + hw/4; x < hx+hw/4+hw/4; x++ {
			img.SetRGBA(x, y, color.RGBA{250, 215, 120, 255})
		}
	}
	var b bytes.Buffer
	_ = jpeg.Encode(&b, img, &jpeg.Options{Quality: 82})
	return b.Bytes()
}

func mix(a, b color.RGBA, f float64) color.RGBA {
	f = math.Max(0, math.Min(1, f))
	l := func(x, y uint8) uint8 { return uint8(float64(x)*(1-f) + float64(y)*f) }
	return color.RGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), 255}
}

// demoChime is a soft three-note chime, as a small WAV.
func demoChime() []byte {
	const rate = 16000
	notes := []float64{523.25, 659.25, 783.99}
	n := rate * 3
	pcm := make([]int16, n)
	for i := range pcm {
		t := float64(i) / rate
		v := 0.0
		for k, f := range notes {
			start := float64(k) * 0.45
			if t >= start {
				d := t - start
				v += math.Sin(2*math.Pi*f*d) * math.Exp(-d*2.2) * 0.28
			}
		}
		pcm[i] = int16(v * 32767)
	}
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(36+n*2))
	b.WriteString("WAVEfmt ")
	_ = binary.Write(&b, binary.LittleEndian, uint32(16))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint16(1))
	_ = binary.Write(&b, binary.LittleEndian, uint32(rate))
	_ = binary.Write(&b, binary.LittleEndian, uint32(rate*2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	_ = binary.Write(&b, binary.LittleEndian, uint16(16))
	b.WriteString("data")
	_ = binary.Write(&b, binary.LittleEndian, uint32(n*2))
	_ = binary.Write(&b, binary.LittleEndian, pcm)
	return b.Bytes()
}
