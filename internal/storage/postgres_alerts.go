package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/alerts"
)

// PgAlertStore — restoran bildirishnomalari (migration 0047).
type PgAlertStore struct{ pool *pgxpool.Pool }

func NewPgAlertStore(pool *pgxpool.Pool) *PgAlertStore { return &PgAlertStore{pool: pool} }

const alertColumns = `seq, id, restaurant_id, kind, category, title, body, data, dedupe_key, read_at, created_at`

func (r *PgAlertStore) Insert(ctx context.Context, n *alerts.Notification) (bool, error) {
	data, err := json.Marshal(n.Data)
	if err != nil {
		return false, err
	}
	if n.Data == nil {
		data = []byte(`{}`)
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO restaurant_notifications (id, restaurant_id, kind, category, title, body, data, dedupe_key, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (restaurant_id, dedupe_key) DO NOTHING
		RETURNING seq`,
		n.ID, n.RestaurantID, n.Kind, string(n.Category), n.Title, n.Body, data, n.DedupeKey, n.CreatedAt,
	).Scan(&n.Seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// likePattern — foydalanuvchi matnidagi `%`, `_`, `\` so'zma-so'z qidiriladi.
func likePattern(s string) string {
	s = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
	return "%" + s + "%"
}

func (r *PgAlertStore) List(ctx context.Context, restaurantID string, q alerts.Query) ([]*alerts.Notification, error) {
	conds := []string{"restaurant_id = $1"}
	args := []any{restaurantID}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if q.Category != "" {
		add("category = ?", string(q.Category))
	}
	if !q.Since.IsZero() {
		add("created_at >= ?", q.Since)
	}
	if q.Search != "" {
		add(`(title ILIKE ? ESCAPE '\' OR body ILIKE ? ESCAPE '\')`, likePattern(q.Search))
	}
	if q.BeforeSeq > 0 {
		add("seq < ?", q.BeforeSeq)
	}
	if q.AfterSeq > 0 {
		add("seq > ?", q.AfterSeq)
	}
	if q.UnreadOnly {
		conds = append(conds, "read_at IS NULL")
	}
	order := "seq DESC"
	if q.AfterSeq > 0 {
		order = "seq ASC"
	}
	args = append(args, q.Limit)
	sql := fmt.Sprintf(`SELECT %s FROM restaurant_notifications WHERE %s ORDER BY %s LIMIT $%d`,
		alertColumns, strings.Join(conds, " AND "), order, len(args))
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*alerts.Notification
	for rows.Next() {
		var n alerts.Notification
		var category string
		var data []byte
		if err := rows.Scan(&n.Seq, &n.ID, &n.RestaurantID, &n.Kind, &category, &n.Title, &n.Body,
			&data, &n.DedupeKey, &n.ReadAt, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.Category = alerts.Category(category)
		if len(data) > 0 {
			_ = json.Unmarshal(data, &n.Data)
		}
		list = append(list, &n)
	}
	return list, rows.Err()
}

func (r *PgAlertStore) Counts(ctx context.Context, restaurantID string, since time.Time) (alerts.Counts, error) {
	var sincePtr *time.Time
	if !since.IsZero() {
		sincePtr = &since
	}
	rows, err := r.pool.Query(ctx, `
		SELECT category, count(*)::int, (count(*) FILTER (WHERE read_at IS NULL))::int
		FROM restaurant_notifications
		WHERE restaurant_id = $1 AND ($2::timestamptz IS NULL OR created_at >= $2)
		GROUP BY category`, restaurantID, sincePtr)
	if err != nil {
		return alerts.Counts{}, err
	}
	defer rows.Close()
	c := alerts.Counts{ByCategory: map[alerts.Category]int{}}
	for rows.Next() {
		var category string
		var total, unread int
		if err := rows.Scan(&category, &total, &unread); err != nil {
			return alerts.Counts{}, err
		}
		c.ByCategory[alerts.Category(category)] = total
		c.Total += total
		c.Unread += unread
	}
	return c, rows.Err()
}

func (r *PgAlertStore) UnreadCount(ctx context.Context, restaurantID string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM restaurant_notifications WHERE restaurant_id = $1 AND read_at IS NULL`,
		restaurantID).Scan(&n)
	return n, err
}

func (r *PgAlertStore) LatestSeq(ctx context.Context, restaurantID string) (int64, error) {
	var seq int64
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(seq), 0) FROM restaurant_notifications WHERE restaurant_id = $1`,
		restaurantID).Scan(&seq)
	return seq, err
}

// MarkRead — `restaurant_id` sharti ATAYLAB: begona ID bilan boshqa
// restoranning yozuviga tegib bo'lmaydi.
func (r *PgAlertStore) MarkRead(ctx context.Context, restaurantID, id string, at time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE restaurant_notifications SET read_at = $3
		WHERE id = $1 AND restaurant_id = $2 AND read_at IS NULL`, id, restaurantID, at)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *PgAlertStore) MarkAllRead(ctx context.Context, restaurantID string, upToSeq int64, at time.Time) (int, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE restaurant_notifications SET read_at = $3
		WHERE restaurant_id = $1 AND read_at IS NULL AND seq <= $2`, restaurantID, upToSeq, at)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (r *PgAlertStore) DeleteOlderThan(ctx context.Context, before time.Time) (int, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM restaurant_notifications WHERE created_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
