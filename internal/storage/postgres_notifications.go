package storage

import (
	"context"
	"encoding/json"

	"chustapp/internal/notify"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Bildirishnomalar va push tokenlari (migration 0030).

type PgNotificationStore struct{ pool *pgxpool.Pool }

func NewPgNotificationStore(pool *pgxpool.Pool) *PgNotificationStore {
	return &PgNotificationStore{pool: pool}
}

func (r *PgNotificationStore) Save(ctx context.Context, n *notify.Notification) error {
	data, err := json.Marshal(n.Data)
	if err != nil {
		return err
	}
	if n.Data == nil {
		data = []byte(`{}`)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO notifications (id, user_id, module, kind, title, body, data, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		n.ID, n.UserID, n.Module, n.Kind, n.Title, n.Body, data, n.CreatedAt)
	return err
}

func (r *PgNotificationStore) List(ctx context.Context, userID string, limit int) ([]*notify.Notification, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, module, kind, title, body, data, read_at, created_at
		FROM notifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*notify.Notification
	for rows.Next() {
		var n notify.Notification
		var data []byte
		if err := rows.Scan(&n.ID, &n.UserID, &n.Module, &n.Kind,
			&n.Title, &n.Body, &data, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		if len(data) > 0 {
			_ = json.Unmarshal(data, &n.Data)
		}
		list = append(list, &n)
	}
	return list, rows.Err()
}

func (r *PgNotificationStore) UnreadCount(ctx context.Context, userID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM notifications WHERE user_id = $1 AND read_at IS NULL`,
		userID).Scan(&n)
	return n, err
}

// MarkRead — `user_id` shart QASDDAN so'rovda: busiz istalgan
// foydalanuvchi ID'sini bilgan holda BEGONA bildirishnomani
// o'qilgan qilib qo'ya olardi.
func (r *PgNotificationStore) MarkRead(ctx context.Context, userID, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notifications SET read_at = now()
		  WHERE id = $1 AND user_id = $2 AND read_at IS NULL`, id, userID)
	return err
}

func (r *PgNotificationStore) MarkAllRead(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notifications SET read_at = now()
		  WHERE user_id = $1 AND read_at IS NULL`, userID)
	return err
}

// ---------- Push tokenlari ----------

type PgTokenStore struct{ pool *pgxpool.Pool }

func NewPgTokenStore(pool *pgxpool.Pool) *PgTokenStore { return &PgTokenStore{pool: pool} }

// SaveToken — token boshqa foydalanuvchida bo'lsa YANGI egasiga
// o'tadi (bitta telefonda ikki hisob almashgan holat). Busiz chiqib
// ketgan foydalanuvchi begona buyurtmalar haqida push olishda davom
// etardi.
func (r *PgTokenStore) SaveToken(ctx context.Context, userID, token, platform string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO device_tokens (token, user_id, platform)
		VALUES ($1,$2,$3)
		ON CONFLICT (token) DO UPDATE
		   SET user_id = EXCLUDED.user_id,
		       platform = EXCLUDED.platform,
		       updated_at = now()`, token, userID, platform)
	return err
}

func (r *PgTokenStore) DeleteToken(ctx context.Context, token string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM device_tokens WHERE token = $1`, token)
	return err
}

func (r *PgTokenStore) TokensFor(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT token FROM device_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
