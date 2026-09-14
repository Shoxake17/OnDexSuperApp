package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/support"
)

// PgSupportStore — aloqa ma'lumotlari va restoran ↔ admin chati
// (migration 0050, rasmlar — 0051).
type PgSupportStore struct{ pool *pgxpool.Pool }

func NewPgSupportStore(pool *pgxpool.Pool) *PgSupportStore { return &PgSupportStore{pool: pool} }

func (r *PgSupportStore) GetContacts(ctx context.Context) (*support.Contacts, error) {
	var c support.Contacts
	err := r.pool.QueryRow(ctx, `
		SELECT phone, phone_hours, telegram, email, email_note, updated_by, updated_at
		FROM support_contacts WHERE id = 1`).
		Scan(&c.Phone, &c.PhoneHours, &c.Telegram, &c.Email, &c.EmailNote, &c.UpdatedBy, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *PgSupportStore) SaveContacts(ctx context.Context, c *support.Contacts) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO support_contacts (id, phone, phone_hours, telegram, email, email_note, updated_by, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET phone = EXCLUDED.phone, phone_hours = EXCLUDED.phone_hours,
			telegram = EXCLUDED.telegram, email = EXCLUDED.email, email_note = EXCLUDED.email_note,
			updated_by = EXCLUDED.updated_by, updated_at = EXCLUDED.updated_at`,
		c.Phone, c.PhoneHours, c.Telegram, c.Email, c.EmailNote, c.UpdatedBy, c.UpdatedAt)
	return err
}

// supportMessageSelect — rasm METAMA'LUMOTI bilan; baytlar (`data`) ATAYLAB
// olinmaydi: ro'yxat har ochilganda megabaytlab ma'lumot tortmasin.
const supportMessageSelect = `
	SELECT m.seq, m.id, m.restaurant_id, m.sender, m.sender_id, m.sender_name, m.body, m.client_id, m.created_at,
		a.id, a.content_type, a.width, a.height, a.size_bytes
	FROM support_messages m
	LEFT JOIN support_attachments a ON a.id = m.attachment_id`

func scanSupportMessage(row pgx.Row, m *support.Message) error {
	var sender string
	var aID, aType *string
	var aW, aH, aSize *int
	if err := row.Scan(&m.Seq, &m.ID, &m.RestaurantID, &sender, &m.SenderID, &m.SenderName,
		&m.Body, &m.ClientID, &m.CreatedAt, &aID, &aType, &aW, &aH, &aSize); err != nil {
		return err
	}
	m.Sender = support.Side(sender)
	m.Attachment = nil
	if aID != nil && aType != nil && aW != nil && aH != nil && aSize != nil {
		m.Attachment = &support.AttachmentMeta{ID: *aID, ContentType: *aType, Width: *aW, Height: *aH, Size: *aSize}
	}
	return nil
}

// readColumn — ustun nomi FAQAT shu ikki qiymatdan biri (so'rov matniga
// foydalanuvchi qiymati hech qachon qo'shilmaydi).
func readColumn(side support.Side) string {
	if side == support.SideAdmin {
		return "admin_read_seq"
	}
	return "restaurant_read_seq"
}

func (r *PgSupportStore) InsertMessage(ctx context.Context, m *support.Message, att *support.Attachment) (bool, error) {
	if !m.Sender.Valid() {
		return false, errors.New("support: yuboruvchi tomoni noto'g'ri")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit'dan keyin no-op

	var attachmentID *string
	if att != nil {
		if _, err := tx.Exec(ctx, `
			INSERT INTO support_attachments (id, restaurant_id, content_type, width, height, size_bytes, data, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			att.ID, att.RestaurantID, att.ContentType, att.Width, att.Height, len(att.Data), att.Data, att.CreatedAt,
		); err != nil {
			return false, err
		}
		attachmentID = &att.ID
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO support_messages (id, restaurant_id, sender, sender_id, sender_name, body, client_id, attachment_id, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (restaurant_id, sender_id, client_id) DO NOTHING
		RETURNING seq`,
		m.ID, m.RestaurantID, string(m.Sender), m.SenderID, m.SenderName, m.Body, m.ClientID, attachmentID, m.CreatedAt,
	).Scan(&m.Seq)
	if errors.Is(err, pgx.ErrNoRows) {
		// Takror: yangi rasm yozuvi ham BEKOR qilinadi (yetim bayt qolmaydi).
		if err := tx.Rollback(ctx); err != nil {
			return false, err
		}
		existing := r.pool.QueryRow(ctx, supportMessageSelect+`
			WHERE m.restaurant_id = $1 AND m.sender_id = $2 AND m.client_id = $3`, m.RestaurantID, m.SenderID, m.ClientID)
		return false, scanSupportMessage(existing, m)
	}
	if err != nil {
		return false, err
	}
	col := readColumn(m.Sender)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO support_threads (restaurant_id, message_count, first_at, last_at, last_seq, %[1]s)
		VALUES ($1, 1, $2, $2, $3, $3)
		ON CONFLICT (restaurant_id) DO UPDATE SET
			message_count = support_threads.message_count + 1,
			last_at = CASE WHEN EXCLUDED.last_seq > support_threads.last_seq
				THEN EXCLUDED.last_at ELSE support_threads.last_at END,
			last_seq = GREATEST(support_threads.last_seq, EXCLUDED.last_seq),
			%[1]s = GREATEST(support_threads.%[1]s, EXCLUDED.%[1]s)`, col),
		m.RestaurantID, m.CreatedAt, m.Seq); err != nil {
		return false, err
	}
	if att != nil {
		meta := att.AttachmentMeta
		meta.Size = len(att.Data)
		m.Attachment = &meta
	}
	return true, tx.Commit(ctx)
}

func (r *PgSupportStore) ListMessages(ctx context.Context, restaurantID string, q support.MessageQuery) ([]*support.Message, error) {
	sql := supportMessageSelect + ` WHERE m.restaurant_id = $1`
	args := []any{restaurantID}
	order := "m.seq DESC"
	switch {
	case q.AfterSeq > 0:
		args = append(args, q.AfterSeq)
		sql += " AND m.seq > $2"
		order = "m.seq ASC"
	case q.BeforeSeq > 0:
		args = append(args, q.BeforeSeq)
		sql += " AND m.seq < $2"
	}
	args = append(args, q.Limit)
	sql += fmt.Sprintf(" ORDER BY %s LIMIT $%d", order, len(args))
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*support.Message
	for rows.Next() {
		var m support.Message
		if err := scanSupportMessage(rows, &m); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// GetAttachment — `restaurant_id` sharti ATAYLAB: begona restoran rasmining
// ID'si bilan uning baytlarini olib bo'lmaydi.
func (r *PgSupportStore) GetAttachment(ctx context.Context, restaurantID, id string) (*support.Attachment, error) {
	var a support.Attachment
	err := r.pool.QueryRow(ctx, `
		SELECT id, restaurant_id, content_type, width, height, size_bytes, data, created_at
		FROM support_attachments WHERE id = $1 AND restaurant_id = $2`, id, restaurantID).
		Scan(&a.ID, &a.RestaurantID, &a.ContentType, &a.Width, &a.Height, &a.Size, &a.Data, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, support.ErrAttachmentNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

const supportThreadSelect = `
	SELECT t.restaurant_id, t.message_count, t.first_at, t.last_at, t.last_seq,
		t.restaurant_read_seq, t.admin_read_seq, COALESCE(m.sender, ''), COALESCE(m.body, ''),
		COALESCE(m.attachment_id IS NOT NULL, false),
		(SELECT count(*) FROM support_messages u WHERE u.restaurant_id = t.restaurant_id
			AND u.sender = 'admin' AND u.seq > t.restaurant_read_seq)::int,
		(SELECT count(*) FROM support_messages u WHERE u.restaurant_id = t.restaurant_id
			AND u.sender = 'restaurant' AND u.seq > t.admin_read_seq)::int
	FROM support_threads t
	LEFT JOIN support_messages m ON m.seq = t.last_seq`

func scanSupportThread(row pgx.Row) (*support.Thread, error) {
	var t support.Thread
	var sender string
	if err := row.Scan(&t.RestaurantID, &t.MessageCount, &t.FirstAt, &t.LastAt, &t.LastSeq,
		&t.RestaurantReadSeq, &t.AdminReadSeq, &sender, &t.LastBody, &t.LastHasImage,
		&t.UnreadRestaurant, &t.UnreadAdmin); err != nil {
		return nil, err
	}
	t.LastSender = support.Side(sender)
	return &t, nil
}

func (r *PgSupportStore) GetThread(ctx context.Context, restaurantID string) (*support.Thread, error) {
	t, err := scanSupportThread(r.pool.QueryRow(ctx, supportThreadSelect+` WHERE t.restaurant_id = $1`, restaurantID))
	if errors.Is(err, pgx.ErrNoRows) {
		return &support.Thread{RestaurantID: restaurantID}, nil
	}
	return t, err
}

func (r *PgSupportStore) ListThreads(ctx context.Context, limit int) ([]*support.Thread, error) {
	rows, err := r.pool.Query(ctx, supportThreadSelect+` ORDER BY t.last_seq DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*support.Thread
	for rows.Next() {
		t, err := scanSupportThread(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *PgSupportStore) MarkRead(ctx context.Context, restaurantID string, side support.Side, upToSeq int64) (bool, error) {
	if !side.Valid() {
		return false, errors.New("support: tomon noto'g'ri")
	}
	col := readColumn(side)
	tag, err := r.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE support_threads SET %[1]s = LEAST(last_seq, $2)
		WHERE restaurant_id = $1 AND %[1]s < LEAST(last_seq, $2)`, col), restaurantID, upToSeq)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *PgSupportStore) AdminUnreadTotal(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `
		SELECT count(*)::int FROM support_messages u
		JOIN support_threads t ON t.restaurant_id = u.restaurant_id
		WHERE u.sender = 'restaurant' AND u.seq > t.admin_read_seq`).Scan(&n)
	return n, err
}
