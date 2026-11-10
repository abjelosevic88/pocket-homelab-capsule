package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/ai"
	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

// Pocket AI (or the person's own OpenAI-compatible key) helps write, and
// at the opening compares then with now. It never sees what is inside a
// sealed capsule: before sealing only what the person sends, after the
// opening the whole capsule.

func (s *Server) aiClient(w http.ResponseWriter, r *http.Request) (*ai.Client, bool) {
	ctx := r.Context()
	st := s.settingsOf(ctx, actorID(ctx))
	if !st.AIHelp {
		writeErr(w, http.StatusBadRequest, "AI help is switched off in Settings → AI")
		return nil, false
	}
	cl, _, why := s.aiFor(ctx, actorID(ctx), st)
	if cl == nil {
		writeErr(w, http.StatusBadRequest, firstNonEmpty(why, "connect Pocket AI or an AI key in Settings → AI"))
		return nil, false
	}
	select {
	case s.aiSem <- struct{}{}:
	default:
		writeErr(w, http.StatusTooManyRequests, "the AI is busy: try again in a moment")
		return nil, false
	}
	return cl, true
}

func (s *Server) aiDone() { <-s.aiSem }

func yearsWords(c *store.Capsule) string {
	from := now()
	y, m, _ := diffYMD(from, c.Opens())
	switch {
	case y > 1:
		return fmt.Sprintf("%d years", y)
	case y == 1:
		return "a year"
	case m > 1:
		return fmt.Sprintf("%d months", m)
	}
	return "a little while"
}

// handleAIWrite helps write a letter: a first draft from a few words, or a
// gentle tidy of what is there. The person's own words win.
func (s *Server) handleAIWrite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		CapsuleID int64  `json:"capsule_id"`
		Prompt    string `json:"prompt"`
		Text      string `json:"text"`
		Mode      string `json:"mode"` // draft | tidy | more
	}
	a, ok := s.capsuleBody(w, r, &in, func() int64 { return in.CapsuleID })
	if !ok {
		return
	}
	if a.c.State == store.StateOpened {
		writeErr(w, http.StatusBadRequest, "this capsule is already open")
		return
	}
	cl, ok := s.aiClient(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	reader := "their future self"
	if a.c.ForName != "" {
		reader = a.c.ForName
	}
	t := templateByID(a.c.Template)
	sys := fmt.Sprintf(`You help someone write a letter for a time capsule called "%s" (%s). It will be opened in about %s by %s. Write in the first person, as the writer, in their language (the language of their text; else English). Warm, concrete and plain: real details of today, no clichés, no poetry, no headings. Never invent facts about the writer's life: where you need a detail, leave a short [bracketed gap] for them to fill. Answer with the letter only.`,
		a.c.Title, t.Name, yearsWords(a.c), reader)
	var prompt string
	switch in.Mode {
	case "tidy":
		if strings.TrimSpace(in.Text) == "" {
			writeErr(w, http.StatusBadRequest, "write something first")
			return
		}
		prompt = "Tidy this letter: fix spelling and flow, keep every thought and the writer's voice, do not add new content:\n\n" + clip(in.Text, 20000)
	case "more":
		prompt = "Here is the letter so far. Suggest the next paragraph (only the new paragraph), about: " + firstNonEmpty(in.Prompt, "what else is worth remembering about today") + "\n\n" + clip(in.Text, 20000)
	default:
		prompt = "Write a first draft of about 150–250 words about: " + firstNonEmpty(strings.TrimSpace(in.Prompt), "what life is like right now") + "."
		if strings.TrimSpace(in.Text) != "" {
			prompt += "\n\nUse these notes from the writer:\n" + clip(in.Text, 10000)
		}
	}
	out, err := cl.Complete(ctx, sys, prompt, false)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": strings.TrimSpace(out)})
}

// handleAIIdeas suggests questions for whoever opens it, guesses to make and
// things to write about, fitting this capsule.
func (s *Server) handleAIIdeas(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		CapsuleID int64  `json:"capsule_id"`
		About     string `json:"about"`
	}
	a, ok := s.capsuleBody(w, r, &in, func() int64 { return in.CapsuleID })
	if !ok {
		return
	}
	cl, ok := s.aiClient(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	t := templateByID(a.c.Template)
	sys := `You suggest ideas for a time capsule. Answer as JSON: {"prompts":[5 short things to write about],"questions":[5 questions for whoever opens it],"predictions":[5 guesses about the future, each a plain statement that will be clearly true or false on the opening day]}. Plain, concrete, personal; no clichés. In English unless the capsule's text is in another language.`
	prompt := fmt.Sprintf("Capsule: %s (%s). %s Opens in about %s.", a.c.Title, t.Name, a.c.About, yearsWords(a.c))
	if a.c.ForName != "" {
		prompt += " It is for " + a.c.ForName + "."
	}
	if v := strings.TrimSpace(in.About); v != "" {
		prompt += " More about it: " + clip(v, 2000)
	}
	raw, err := cl.Complete(ctx, sys, prompt, true)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
		return
	}
	var out struct {
		Prompts     []string `json:"prompts"`
		Questions   []string `json:"questions"`
		Predictions []string `json:"predictions"`
	}
	if err := json.Unmarshal([]byte(jsonPart(raw)), &out); err != nil {
		writeErr(w, http.StatusBadGateway, "the AI's answer could not be read; try again")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func jsonPart(s string) string {
	if i, j := strings.Index(s, "{"), strings.LastIndex(s, "}"); i >= 0 && j > i {
		return s[i : j+1]
	}
	return s
}

// handleAIThenNow writes a short "then and now" at the opening: the world
// kept on sealing day next to today's, and what the capsule said.
func (s *Server) handleAIThenNow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var in struct {
		CapsuleID int64          `json:"capsule_id"`
		Now       map[string]any `json:"now"`
		Save      bool           `json:"save"`
	}
	a, ok := s.capsuleBody(w, r, &in, func() int64 { return in.CapsuleID })
	if !ok {
		return
	}
	if a.c.State != store.StateOpened {
		writeErr(w, http.StatusBadRequest, "open it first")
		return
	}
	cl, ok := s.aiClient(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	text, err := s.thenNow(ctx, cl, a.c, in.Now)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
		return
	}
	if in.Save {
		a.c.ThenNow = text
		_ = s.store.UpdateCapsule(ctx, a.c)
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": text})
}

func (s *Server) thenNow(ctx context.Context, cl *ai.Client, c *store.Capsule, nowSnap map[string]any) (string, error) {
	var b strings.Builder
	sealed := "an unknown day"
	if c.SealedAt != nil {
		if t, err := time.Parse(time.RFC3339Nano, *c.SealedAt); err == nil {
			sealed = t.Local().Format("2 January 2006")
		}
	}
	fmt.Fprintf(&b, "Capsule “%s”, sealed on %s, opened on %s.\n\n", c.Title, sealed, time.Now().Format("2 January 2006"))
	if snap := mapOfRaw(c.Snapshot); snap != nil {
		b.WriteString("THEN (on sealing day):\n")
		for _, l := range snapshotLines(snap) {
			b.WriteString("- " + l + "\n")
		}
	}
	if len(nowSnap) > 0 {
		b.WriteString("\nNOW:\n")
		for _, l := range snapshotLines(nowSnap) {
			b.WriteString("- " + l + "\n")
		}
	}
	items, _ := s.store.Items(ctx, c.ID)
	b.WriteString("\nINSIDE:\n")
	for _, it := range items {
		switch it.Kind {
		case "letter":
			fmt.Fprintf(&b, "- Letter from %s: %s\n", it.AuthorName, clip(it.Body, 1500))
		case "prediction":
			fmt.Fprintf(&b, "- Guess (%d%% sure): %s → came true: %s\n", it.Confidence, it.Body, firstNonEmpty(it.Outcome, "not marked yet"))
		case "goal":
			fmt.Fprintf(&b, "- Goal: %s → %s\n", it.Body, firstNonEmpty(it.Outcome, "not marked yet"))
		case "question":
			fmt.Fprintf(&b, "- Question: %s → %s\n", it.Body, firstNonEmpty(it.Answer, "not answered yet"))
		case "list":
			parts := []string{}
			for _, e := range it.Entries {
				parts = append(parts, e.Label+": "+e.Value)
			}
			fmt.Fprintf(&b, "- %s: %s\n", firstNonEmpty(it.Title, "List"), strings.Join(parts, "; "))
		default:
			fmt.Fprintf(&b, "- %s from %s %s\n", kindWord(it.Kind), it.AuthorName, clip(it.Title+" "+it.Body, 200))
		}
	}
	sys := `You write a short "then and now" for someone who just opened their time capsule: 4–7 sentences, warm and plain, in the second person ("you"), in the language the capsule is written in. Compare what was kept on sealing day with today where both are given (weather, reading, music, places, numbers), note which guesses came true, and point at one or two things in the letters worth thinking about. Only use the facts given; never invent. No headings, no lists.`
	out, err := cl.Complete(ctx, sys, clip(b.String(), 30000), false)
	return strings.TrimSpace(out), err
}
