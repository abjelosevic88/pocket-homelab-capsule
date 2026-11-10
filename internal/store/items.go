package store

import (
	"context"
	"encoding/json"
)

const itemCols = `i.id, i.capsule_id, i.user_id, i.author_name, i.kind, i.title, i.body, i.upload_id, i.confidence, i.outcome, i.answer, i.entries, i.url, i.from_app, i.from_url,
	i.after_seal, i.guest, i.position, i.source, i.created_at, i.updated_at, COALESCE(up.mime, ''), COALESCE(up.name, ''), COALESCE(up.bytes, 0)`

const itemFrom = ` FROM items i LEFT JOIN uploads up ON up.id = i.upload_id`

func scanItem(row interface{ Scan(...any) error }) (*Item, error) {
	var it Item
	var entries string
	var after, guest int
	err := row.Scan(&it.ID, &it.CapsuleID, &it.UserID, &it.AuthorName, &it.Kind, &it.Title, &it.Body, &it.UploadID, &it.Confidence, &it.Outcome, &it.Answer, &entries, &it.URL,
		&it.FromApp, &it.FromURL, &after, &guest, &it.Position, &it.Source, &it.CreatedAt, &it.UpdatedAt, &it.Mime, &it.Name, &it.Bytes)
	if err != nil {
		return nil, notFound(err)
	}
	it.AfterSeal, it.Guest = after == 1, guest == 1
	_ = json.Unmarshal([]byte(entries), &it.Entries)
	if it.Entries == nil {
		it.Entries = []Entry{}
	}
	return &it, nil
}

// Items lists everything in a capsule, in order (no visibility rules: the
// server decides who sees what).
func (s *Store) Items(ctx context.Context, capsuleID int64) ([]Item, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+itemCols+itemFrom+` WHERE i.capsule_id = ? ORDER BY i.position, i.id`, capsuleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *it)
	}
	return out, rows.Err()
}

// ItemsOfKind lists one kind of item over the opened capsules a person is in (predictions, goals).
func (s *Store) ItemsOfKind(ctx context.Context, uid int64, kinds ...string) ([]Item, error) {
	q := `SELECT ` + itemCols + itemFrom + ` JOIN capsules c ON c.id = i.capsule_id WHERE c.state = 'opened' AND (c.user_id = ? OR c.id IN (SELECT capsule_id FROM members WHERE user_id = ?)) AND i.kind IN (`
	args := []any{uid, uid}
	for n, k := range kinds {
		if n > 0 {
			q += ","
		}
		q += "?"
		args = append(args, k)
	}
	rows, err := s.DB.QueryContext(ctx, q+`) ORDER BY c.opened_at DESC, i.position, i.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *it)
	}
	return out, rows.Err()
}

// GetItem loads one item.
func (s *Store) GetItem(ctx context.Context, id int64) (*Item, error) {
	return scanItem(s.DB.QueryRowContext(ctx, `SELECT `+itemCols+itemFrom+` WHERE i.id = ?`, id))
}

// CreateItem puts something in a capsule (at the end).
func (s *Store) CreateItem(ctx context.Context, it *Item) error {
	if it.Entries == nil {
		it.Entries = []Entry{}
	}
	if it.Position == 0 {
		_ = s.DB.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) + 1 FROM items WHERE capsule_id = ?`, it.CapsuleID).Scan(&it.Position)
	}
	now := Now()
	if it.CreatedAt == "" {
		it.CreatedAt = now
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO items (capsule_id, user_id, author_name, kind, title, body, upload_id, confidence, outcome, answer, entries, url, from_app, from_url,
		after_seal, guest, position, source, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		it.CapsuleID, nullI(it.UserID), it.AuthorName, it.Kind, it.Title, it.Body, nullI(it.UploadID), it.Confidence, it.Outcome, it.Answer, toJSON(it.Entries), it.URL, it.FromApp, it.FromURL,
		boolInt(it.AfterSeal), boolInt(it.Guest), it.Position, it.Source, it.CreatedAt, now)
	if err != nil {
		return err
	}
	it.ID, _ = res.LastInsertId()
	it.UpdatedAt = now
	if it.UploadID != nil {
		_, _ = s.DB.ExecContext(ctx, `UPDATE uploads SET capsule_id = ? WHERE id = ?`, it.CapsuleID, *it.UploadID)
	}
	return nil
}

// UpdateItem writes an item back.
func (s *Store) UpdateItem(ctx context.Context, it *Item) error {
	if it.Entries == nil {
		it.Entries = []Entry{}
	}
	it.UpdatedAt = Now()
	res, err := s.DB.ExecContext(ctx, `UPDATE items SET title=?, body=?, upload_id=?, confidence=?, outcome=?, answer=?, entries=?, url=?, position=?, updated_at=? WHERE id=?`,
		it.Title, it.Body, nullI(it.UploadID), it.Confidence, it.Outcome, it.Answer, toJSON(it.Entries), it.URL, it.Position, it.UpdatedAt, it.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteItem takes something out (its upload goes with it).
func (s *Store) DeleteItem(ctx context.Context, id int64) error {
	var up *int64
	_ = s.DB.QueryRowContext(ctx, `SELECT upload_id FROM items WHERE id = ?`, id).Scan(&up)
	res, err := s.DB.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if up != nil {
		_, _ = s.DB.ExecContext(ctx, `DELETE FROM uploads WHERE id = ? AND NOT EXISTS (SELECT 1 FROM items WHERE upload_id = ?)`, *up, *up)
	}
	return nil
}

// ReorderItems sets the order of a capsule's items.
func (s *Store) ReorderItems(ctx context.Context, capsuleID int64, ids []int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for n, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE items SET position = ? WHERE id = ? AND capsule_id = ?`, n+1, id, capsuleID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ---- uploads ----

// SaveUpload stores a file.
func (s *Store) SaveUpload(ctx context.Context, up *Upload) error {
	up.Bytes = len(up.Data)
	var thumb any
	if len(up.Thumb) > 0 {
		thumb = up.Thumb
	}
	var capsule any
	if up.CapsuleID > 0 {
		capsule = up.CapsuleID
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO uploads (user_id, capsule_id, name, mime, bytes, width, height, data, thumb) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		up.UserID, capsule, up.Name, up.Mime, up.Bytes, up.Width, up.Height, up.Data, thumb)
	if err != nil {
		return err
	}
	up.ID, _ = res.LastInsertId()
	up.CreatedAt = Now()
	return nil
}

// GetUpload loads a file with its bytes.
func (s *Store) GetUpload(ctx context.Context, id int64) (*Upload, error) {
	var up Upload
	var capsule *int64
	err := s.DB.QueryRowContext(ctx, `SELECT id, user_id, capsule_id, name, mime, bytes, width, height, data, COALESCE(thumb, X''), created_at FROM uploads WHERE id = ?`, id).
		Scan(&up.ID, &up.UserID, &capsule, &up.Name, &up.Mime, &up.Bytes, &up.Width, &up.Height, &up.Data, &up.Thumb, &up.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	if capsule != nil {
		up.CapsuleID = *capsule
	}
	return &up, nil
}

// PurgeUploads removes files no item uses that are older than before.
func (s *Store) PurgeUploads(ctx context.Context, before string) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM uploads WHERE created_at < ? AND id NOT IN (SELECT upload_id FROM items WHERE upload_id IS NOT NULL)`, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
