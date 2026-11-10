package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// NewKey is a long random key for a link that works without an account.
func NewKey() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

const capsuleCols = `c.id, c.user_id, COALESCE(u.name, ''), c.title, c.about, c.template, c.emoji, c.color, c.state, c.opens_at, c.open_rule, c.for_name, c.for_date, c.for_age,
	c.surprise, c.slot, c.deliver, c.remind_days, c.sections, c.snapshot, c.sealed_at, c.opened_at, c.opened_by, c.early, c.early_reason, c.ready_sent, c.remind_sent,
	c.delivered_at, c.reflection, c.then_now, c.reply_to, c.give_key, c.give_open, c.calendar_url, c.source, c.ext_id, c.created_at, c.updated_at,
	(SELECT COUNT(1) FROM items i WHERE i.capsule_id = c.id), (SELECT COUNT(1) FROM members m WHERE m.capsule_id = c.id)`

const capsuleFrom = ` FROM capsules c LEFT JOIN users u ON u.id = c.user_id`

func scanCapsule(row interface{ Scan(...any) error }) (*Capsule, error) {
	var c Capsule
	var sections, snap string
	var surprise, slot, early, ready, remind, give int
	err := row.Scan(&c.ID, &c.UserID, &c.OwnerName, &c.Title, &c.About, &c.Template, &c.Emoji, &c.Color, &c.State, &c.OpensAt, &c.OpenRule, &c.ForName, &c.ForDate, &c.ForAge,
		&surprise, &slot, &c.Deliver, &c.RemindDays, &sections, &snap, &c.SealedAt, &c.OpenedAt, &c.OpenedBy, &early, &c.EarlyReason, &ready, &remind,
		&c.DeliveredAt, &c.Reflection, &c.ThenNow, &c.ReplyTo, &c.GiveKey, &give, &c.CalendarURL, &c.Source, &c.ExtID, &c.CreatedAt, &c.UpdatedAt,
		&c.Items, &c.Members)
	if err != nil {
		return nil, notFound(err)
	}
	c.Surprise, c.Slot, c.Early, c.ReadySent, c.RemindSent, c.GiveOpen = surprise == 1, slot == 1, early == 1, ready == 1, remind == 1, give == 1
	_ = json.Unmarshal([]byte(sections), &c.Sections)
	if c.Sections == nil {
		c.Sections = []string{}
	}
	if snap != "" && snap != "{}" {
		c.Snapshot = json.RawMessage(snap)
	}
	return &c, nil
}

func (s *Store) listCapsules(ctx context.Context, where string, args ...any) ([]Capsule, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+capsuleCols+capsuleFrom+` WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Capsule{}
	for rows.Next() {
		c, err := scanCapsule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// CreateCapsule adds a capsule (state filling unless set).
func (s *Store) CreateCapsule(ctx context.Context, c *Capsule) error {
	if c.State == "" {
		c.State = StateFilling
	}
	if c.GiveKey == "" {
		c.GiveKey = NewKey()
	}
	if c.Sections == nil {
		c.Sections = []string{}
	}
	snap := "{}"
	if len(c.Snapshot) > 0 {
		snap = string(c.Snapshot)
	}
	now := Now()
	if c.CreatedAt == "" {
		c.CreatedAt = now
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO capsules (user_id, title, about, template, emoji, color, state, opens_at, open_rule, for_name, for_date, for_age,
		surprise, slot, deliver, remind_days, sections, snapshot, sealed_at, opened_at, opened_by, early, early_reason, ready_sent, remind_sent, delivered_at, reflection, then_now,
		reply_to, give_key, give_open, calendar_url, source, ext_id, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.UserID, c.Title, c.About, c.Template, c.Emoji, c.Color, c.State, c.OpensAt, c.OpenRule, c.ForName, c.ForDate, c.ForAge,
		boolInt(c.Surprise), boolInt(c.Slot), c.Deliver, c.RemindDays, toJSON(c.Sections), snap, c.SealedAt, c.OpenedAt, nullI(c.OpenedBy), boolInt(c.Early), c.EarlyReason,
		boolInt(c.ReadySent), boolInt(c.RemindSent), c.DeliveredAt, c.Reflection, c.ThenNow, nullI(c.ReplyTo), c.GiveKey, boolInt(c.GiveOpen), c.CalendarURL, c.Source, c.ExtID, c.CreatedAt, now)
	if err != nil {
		return conflict(err)
	}
	c.ID, _ = res.LastInsertId()
	c.UpdatedAt = now
	return nil
}

// UpdateCapsule writes every column of a capsule back.
func (s *Store) UpdateCapsule(ctx context.Context, c *Capsule) error {
	snap := "{}"
	if len(c.Snapshot) > 0 {
		snap = string(c.Snapshot)
	}
	if c.Sections == nil {
		c.Sections = []string{}
	}
	c.UpdatedAt = Now()
	res, err := s.DB.ExecContext(ctx, `UPDATE capsules SET title=?, about=?, template=?, emoji=?, color=?, state=?, opens_at=?, open_rule=?, for_name=?, for_date=?, for_age=?,
		surprise=?, slot=?, deliver=?, remind_days=?, sections=?, snapshot=?, sealed_at=?, opened_at=?, opened_by=?, early=?, early_reason=?, ready_sent=?, remind_sent=?,
		delivered_at=?, reflection=?, then_now=?, reply_to=?, give_key=?, give_open=?, calendar_url=?, updated_at=? WHERE id=?`,
		c.Title, c.About, c.Template, c.Emoji, c.Color, c.State, c.OpensAt, c.OpenRule, c.ForName, c.ForDate, c.ForAge,
		boolInt(c.Surprise), boolInt(c.Slot), c.Deliver, c.RemindDays, toJSON(c.Sections), snap, c.SealedAt, c.OpenedAt, nullI(c.OpenedBy), boolInt(c.Early), c.EarlyReason,
		boolInt(c.ReadySent), boolInt(c.RemindSent), c.DeliveredAt, c.Reflection, c.ThenNow, nullI(c.ReplyTo), c.GiveKey, boolInt(c.GiveOpen), c.CalendarURL, c.UpdatedAt, c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetCapsule loads one capsule (no access check: the server does that).
func (s *Store) GetCapsule(ctx context.Context, id int64) (*Capsule, error) {
	return scanCapsule(s.DB.QueryRowContext(ctx, `SELECT `+capsuleCols+capsuleFrom+` WHERE c.id = ?`, id))
}

// CapsuleByGiveKey finds the capsule an "add something" link belongs to.
func (s *Store) CapsuleByGiveKey(ctx context.Context, key string) (*Capsule, error) {
	if len(key) < 20 {
		return nil, ErrNotFound
	}
	return scanCapsule(s.DB.QueryRowContext(ctx, `SELECT `+capsuleCols+capsuleFrom+` WHERE c.give_key = ?`, key))
}

// CapsuleByExt finds an imported capsule.
func (s *Store) CapsuleByExt(ctx context.Context, uid int64, ext string) (*Capsule, error) {
	return scanCapsule(s.DB.QueryRowContext(ctx, `SELECT `+capsuleCols+capsuleFrom+` WHERE c.user_id = ? AND c.ext_id = ?`, uid, ext))
}

// CapsulesFor lists the capsules a person made or is in.
func (s *Store) CapsulesFor(ctx context.Context, uid int64) ([]Capsule, error) {
	return s.listCapsules(ctx, `c.user_id = ? OR c.id IN (SELECT capsule_id FROM members WHERE user_id = ?) ORDER BY c.opens_at, c.id`, uid, uid)
}

// OwnCapsules lists the capsules a person made.
func (s *Store) OwnCapsules(ctx context.Context, uid int64) ([]Capsule, error) {
	return s.listCapsules(ctx, `c.user_id = ? ORDER BY c.id`, uid)
}

// DueCapsules are sealed capsules whose day came and nobody was told yet.
func (s *Store) DueCapsules(ctx context.Context, now time.Time) ([]Capsule, error) {
	return s.listCapsules(ctx, `c.state = 'sealed' AND c.ready_sent = 0 AND c.opens_at <= ? ORDER BY c.opens_at`, now.UTC().Format(time.RFC3339Nano))
}

// SoonCapsules are sealed capsules within their reminder window, not reminded yet.
func (s *Store) SoonCapsules(ctx context.Context, now time.Time) ([]Capsule, error) {
	return s.listCapsules(ctx, `c.state = 'sealed' AND c.remind_sent = 0 AND c.remind_days > 0 AND c.opens_at > ? AND c.opens_at <= ? ORDER BY c.opens_at`,
		now.UTC().Format(time.RFC3339Nano), now.AddDate(0, 0, 400).UTC().Format(time.RFC3339Nano))
}

// DeleteCapsule removes a capsule and everything in it.
func (s *Store) DeleteCapsule(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM capsules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- members ----

// Membership is a person's role in a capsule ("" = not in it).
func (s *Store) Membership(ctx context.Context, capsuleID, uid int64) string {
	var role string
	_ = s.DB.QueryRowContext(ctx, `SELECT role FROM members WHERE capsule_id = ? AND user_id = ?`, capsuleID, uid).Scan(&role)
	return role
}

// Members lists the people in a capsule.
func (s *Store) Members(ctx context.Context, capsuleID int64) ([]Member, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT m.capsule_id, m.user_id, u.name, u.email, m.role, m.added_at FROM members m JOIN users u ON u.id = m.user_id WHERE m.capsule_id = ? ORDER BY m.added_at`, capsuleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.CapsuleID, &m.UserID, &m.Name, &m.Email, &m.Role, &m.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetMember adds a person to a capsule or changes their role.
func (s *Store) SetMember(ctx context.Context, capsuleID, uid int64, role string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO members (capsule_id, user_id, role) VALUES (?, ?, ?) ON CONFLICT(capsule_id, user_id) DO UPDATE SET role = excluded.role`, capsuleID, uid, role)
	return err
}

// RemoveMember takes a person out of a capsule.
func (s *Store) RemoveMember(ctx context.Context, capsuleID, uid int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM members WHERE capsule_id = ? AND user_id = ?`, capsuleID, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- recipients ----

const recipientCols = `id, user_id, capsule_id, name, email, key, sent_at, viewed_at, created_at`

func scanRecipient(row interface{ Scan(...any) error }) (*Recipient, error) {
	var r Recipient
	if err := row.Scan(&r.ID, &r.UserID, &r.CapsuleID, &r.Name, &r.Email, &r.Key, &r.SentAt, &r.ViewedAt, &r.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	return &r, nil
}

// Recipients lists who a capsule is for.
func (s *Store) Recipients(ctx context.Context, capsuleID int64) ([]Recipient, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+recipientCols+` FROM recipients WHERE capsule_id = ? ORDER BY id`, capsuleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Recipient{}
	for rows.Next() {
		r, err := scanRecipient(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// AddRecipient adds someone a capsule is for.
func (s *Store) AddRecipient(ctx context.Context, r *Recipient) error {
	if r.Key == "" {
		r.Key = NewKey()
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO recipients (user_id, capsule_id, name, email, key) VALUES (?, ?, ?, ?, ?)`, r.UserID, r.CapsuleID, r.Name, strings.ToLower(r.Email), r.Key)
	if err != nil {
		return conflict(err)
	}
	r.ID, _ = res.LastInsertId()
	r.CreatedAt = Now()
	return nil
}

// GetRecipient loads one recipient of a capsule.
func (s *Store) GetRecipient(ctx context.Context, capsuleID, id int64) (*Recipient, error) {
	return scanRecipient(s.DB.QueryRowContext(ctx, `SELECT `+recipientCols+` FROM recipients WHERE capsule_id = ? AND id = ?`, capsuleID, id))
}

// RecipientByKey finds a recipient by their link.
func (s *Store) RecipientByKey(ctx context.Context, key string) (*Recipient, error) {
	if len(key) < 20 {
		return nil, ErrNotFound
	}
	return scanRecipient(s.DB.QueryRowContext(ctx, `SELECT `+recipientCols+` FROM recipients WHERE key = ?`, key))
}

// MarkRecipient records that the email went (sent) or the page was opened (viewed).
func (s *Store) MarkRecipient(ctx context.Context, id int64, what string) error {
	col := "sent_at"
	if what == "viewed" {
		col = "viewed_at"
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE recipients SET `+col+` = COALESCE(`+col+`, ?) WHERE id = ?`, Now(), id)
	return err
}

// DeleteRecipient removes someone a capsule was for.
func (s *Store) DeleteRecipient(ctx context.Context, capsuleID, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM recipients WHERE capsule_id = ? AND id = ?`, capsuleID, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- history ----

// Log adds a line to a capsule's history.
func (s *Store) Log(ctx context.Context, capsuleID int64, uid *int64, who, what, detail string) {
	_, _ = s.DB.ExecContext(ctx, `INSERT INTO history (capsule_id, user_id, who, what, detail) VALUES (?, ?, ?, ?, ?)`, capsuleID, nullI(uid), who, what, detail)
}

// LogAt adds a line with its own time (demo data, imports).
func (s *Store) LogAt(ctx context.Context, capsuleID int64, uid *int64, who, what, detail, at string) {
	_, _ = s.DB.ExecContext(ctx, `INSERT INTO history (capsule_id, user_id, who, what, detail, at) VALUES (?, ?, ?, ?, ?, ?)`, capsuleID, nullI(uid), who, what, detail, at)
}

func scanEvents(rows *sql.Rows) ([]Event, error) {
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.CapsuleID, &e.UserID, &e.Who, &e.What, &e.Detail, &e.At, &e.Title, &e.Emoji); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// History lists what happened to a capsule, newest first.
func (s *Store) History(ctx context.Context, capsuleID int64) ([]Event, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT h.id, h.capsule_id, h.user_id, h.who, h.what, h.detail, h.at, c.title, c.emoji FROM history h JOIN capsules c ON c.id = h.capsule_id WHERE h.capsule_id = ? ORDER BY h.at DESC, h.id DESC LIMIT 200`, capsuleID)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// HistoryBetween lists what happened between two times in the capsules a person is in.
func (s *Store) HistoryBetween(ctx context.Context, uid int64, from, to string) ([]Event, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT h.id, h.capsule_id, h.user_id, h.who, h.what, h.detail, h.at, c.title, c.emoji FROM history h JOIN capsules c ON c.id = h.capsule_id
		WHERE (c.user_id = ? OR c.id IN (SELECT capsule_id FROM members WHERE user_id = ?)) AND h.at >= ? AND h.at < ? ORDER BY h.at, h.id LIMIT 500`, uid, uid, from, to)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

// ---- rituals ----

func scanRitual(row interface{ Scan(...any) error }) (*Ritual, error) {
	var r Ritual
	var active int
	if err := row.Scan(&r.ID, &r.UserID, &r.Title, &r.Template, &r.Month, &r.Day, &r.Years, &active, &r.LastYear, &r.CreatedAt); err != nil {
		return nil, notFound(err)
	}
	r.Active = active == 1
	return &r, nil
}

const ritualCols = `id, user_id, title, template, month, day, years, active, last_year, created_at`

// Rituals lists a person's yearly capsules (uid 0: everybody's, for the janitor).
func (s *Store) Rituals(ctx context.Context, uid int64) ([]Ritual, error) {
	q, args := `SELECT `+ritualCols+` FROM rituals WHERE user_id = ? ORDER BY month, day, id`, []any{uid}
	if uid == 0 {
		q, args = `SELECT `+ritualCols+` FROM rituals WHERE active = 1 ORDER BY id`, nil
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ritual{}
	for rows.Next() {
		r, err := scanRitual(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// GetRitual loads one of a person's yearly capsules.
func (s *Store) GetRitual(ctx context.Context, uid, id int64) (*Ritual, error) {
	return scanRitual(s.DB.QueryRowContext(ctx, `SELECT `+ritualCols+` FROM rituals WHERE user_id = ? AND id = ?`, uid, id))
}

// SaveRitual adds or updates a yearly capsule.
func (s *Store) SaveRitual(ctx context.Context, r *Ritual) error {
	if r.ID == 0 {
		res, err := s.DB.ExecContext(ctx, `INSERT INTO rituals (user_id, title, template, month, day, years, active, last_year) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			r.UserID, r.Title, r.Template, r.Month, r.Day, r.Years, boolInt(r.Active), r.LastYear)
		if err != nil {
			return err
		}
		r.ID, _ = res.LastInsertId()
		r.CreatedAt = Now()
		return nil
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE rituals SET title=?, template=?, month=?, day=?, years=?, active=?, last_year=? WHERE id=? AND user_id=?`,
		r.Title, r.Template, r.Month, r.Day, r.Years, boolInt(r.Active), r.LastYear, r.ID, r.UserID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteRitual removes a yearly capsule.
func (s *Store) DeleteRitual(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM rituals WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
