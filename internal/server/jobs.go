package server

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"time"

	"github.com/abjelosevic88/pocket-homelab-capsule/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-capsule/internal/store"
)

const notifyTestBody = "This is how Pocket Capsule tells you a capsule is ready to open."

func htmlEsc(v string) string { return html.EscapeString(v) }

// push sends a phone message through the person's own notification URL.
func (s *Server) push(ctx context.Context, uid int64, st store.Settings, title, body, path string) bool {
	if st.NotifyURL == "" {
		return false
	}
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: title, Body: body, Priority: "info", Tags: []string{"hourglass"}, Link: s.link(path)}); err != nil {
		s.log.Debug("notify failed", "user", uid, "err", err)
		return false
	}
	return true
}

// tellMembers tells everyone in a capsule but one (setting = which switch
// they turned on for it).
func (s *Server) tellMembers(ctx context.Context, c *store.Capsule, except int64, setting, title, body, path string) {
	ids := []int64{c.UserID}
	members, _ := s.store.Members(ctx, c.ID)
	for _, m := range members {
		ids = append(ids, m.UserID)
	}
	for _, id := range ids {
		if id == except {
			continue
		}
		st := s.settingsOf(ctx, id)
		on := map[string]bool{"notify_ready": st.NotifyReady, "notify_opened": st.NotifyOpened, "notify_remind": st.NotifyRemind}[setting]
		if on {
			s.push(ctx, id, st, title, body, path)
		}
	}
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st := s.settingsOf(ctx, actorID(ctx))
	if st.NotifyURL == "" {
		writeErr(w, http.StatusBadRequest, "add a notification URL first")
		return
	}
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: "Pocket Capsule", Body: notifyTestBody, Priority: "info", Tags: []string{"hourglass"}}); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// recipientMayOpen: the people a capsule is for see inside once it is
// opened — or on the day itself when it is set to go out then.
func recipientMayOpen(c *store.Capsule) bool {
	return c.State == store.StateOpened || (c.Deliver == "on_date" && c.Ready(now()))
}

// deliver emails everyone a capsule is for who has not had it yet.
func (s *Server) deliver(ctx context.Context, c *store.Capsule) {
	if !recipientMayOpen(c) {
		return
	}
	maker, err := s.store.GetUser(ctx, c.UserID)
	if err != nil {
		return
	}
	rec, _ := s.store.Recipients(ctx, c.ID)
	sent, failed := 0, ""
	for i := range rec {
		r := &rec[i]
		if r.SentAt != nil && c.DeliveredAt != nil {
			continue
		}
		if err := s.emailRecipient(ctx, maker, c, r); err != nil {
			failed = err.Error()
			break
		}
		sent++
	}
	if sent > 0 {
		at := store.Now()
		c.DeliveredAt = &at
		_ = s.store.UpdateCapsule(ctx, c)
		s.store.Log(ctx, c.ID, nil, "Pocket Capsule", "delivered", fmt.Sprintf("emailed %d %s", sent, plural(sent, "person", "people")))
	}
	if failed != "" {
		s.store.Log(ctx, c.ID, nil, "Pocket Capsule", "delivered", "could not email everyone: "+clip(failed, 200))
	}
}

func (s *Server) deliverLater(id int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if c, err := s.store.GetCapsule(ctx, id); err == nil {
		s.deliver(ctx, c)
	}
}

// openingDays tells people when a capsule's day has come (and, for one set
// to go out on the day, sends it to the people it is for).
func (s *Server) openingDays(ctx context.Context) {
	due, err := s.store.DueCapsules(ctx, now())
	if err != nil {
		return
	}
	for i := range due {
		c := &due[i]
		c.ReadySent = true
		if err := s.store.UpdateCapsule(ctx, c); err != nil {
			continue
		}
		s.store.Log(ctx, c.ID, nil, "Pocket Capsule", "ready", "")
		path := fmt.Sprintf("/c/%d", c.ID)
		since := ""
		if c.SealedAt != nil {
			if t, err := time.Parse(time.RFC3339Nano, *c.SealedAt); err == nil {
				since = "Sealed on " + t.Local().Format("2 January 2006") + ". "
			}
		}
		s.tellMembers(ctx, c, 0, "notify_ready", c.Emoji+" “"+c.Title+"” is ready to open", since+"Today is the day.", path)
		if st := s.settingsOf(ctx, c.UserID); st.AlertsOnReady {
			s.alert(ctx, c.UserID, c.Emoji+" “"+c.Title+"” is ready to open", since+"Today is the day.", path, "high")
		}
		s.publish(ctx, c.UserID, "capsule.ready", c.Title, map[string]any{"capsule_id": c.ID, "title": c.Title, "sealed_at": c.SealedAt, "url": s.link(path)})
		if c.Deliver == "on_date" {
			s.deliver(ctx, c)
		}
	}
}

// reminders: "opens next week", at the person's hour.
func (s *Server) reminders(ctx context.Context) {
	t := now()
	soon, err := s.store.SoonCapsules(ctx, t)
	if err != nil {
		return
	}
	for i := range soon {
		c := &soon[i]
		if t.Before(c.Opens().AddDate(0, 0, -c.RemindDays)) {
			continue
		}
		st := s.settingsOf(ctx, c.UserID)
		if t.Local().Hour() < st.NotifyHour && c.Opens().Sub(t) > 24*time.Hour {
			continue
		}
		c.RemindSent = true
		if err := s.store.UpdateCapsule(ctx, c); err != nil {
			continue
		}
		s.store.Log(ctx, c.ID, nil, "Pocket Capsule", "reminded", "")
		when := untilWords(c.Opens(), t)
		path := fmt.Sprintf("/c/%d", c.ID)
		s.tellMembers(ctx, c, 0, "notify_remind", c.Emoji+" “"+c.Title+"” opens "+when, "On "+c.Opens().Local().Format("Monday 2 January at 15:04")+".", path)
		if st.AlertsOnReady {
			s.alert(ctx, c.UserID, c.Emoji+" “"+c.Title+"” opens "+when, "On "+c.Opens().Local().Format("Monday 2 January at 15:04")+".", path, "default")
		}
	}
}

// yearly nudges people to write this year's capsule on its day.
func (s *Server) yearly(ctx context.Context) {
	t := now().Local()
	list, err := s.store.Rituals(ctx, 0)
	if err != nil {
		return
	}
	for i := range list {
		r := &list[i]
		if r.LastYear >= t.Year() || int(t.Month()) != r.Month || t.Day() != r.Day {
			continue
		}
		st := s.settingsOf(ctx, r.UserID)
		if t.Hour() < st.NotifyHour {
			continue
		}
		r.LastYear = t.Year()
		if err := s.store.SaveRitual(ctx, r); err != nil {
			continue
		}
		path := fmt.Sprintf("/new?ritual=%d", r.ID)
		title := "Time for this year's capsule: " + r.Title
		body := fmt.Sprintf("Write it today; it opens in %d %s.", r.Years, plural(r.Years, "year", "years"))
		if st.NotifyYearly {
			s.push(ctx, r.UserID, st, title, body, path)
		}
		if st.AlertsOnReady {
			s.alert(ctx, r.UserID, title, body, path, "default")
		}
	}
}
