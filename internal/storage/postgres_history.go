package storage

import (
	"context"
	"strconv"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/stats"
)

var _ stats.HistorySource = (*PgOrderRepo)(nil)

// OrderHistory — `stats.HistorySource` ning Postgres implementatsiyasi.
//
// Indeks: `idx_orders_restaurant_created (restaurant_id, created_at)`
// (0043) — davr sharti ham, tartib ham shu ustunda. id ATAYLAB
// `COLLATE "C"` bilan solishtiriladi: kursor Go'da bayt tartibida
// quriladi, bazaning lokal tartibi (masalan en_US) esa harf registrini
// boshqacha tartiblab, sahifa chegarasida buyurtmani tashlab ketishi
// mumkin edi.
func (r *PgOrderRepo) OrderHistory(ctx context.Context, restaurantID string, q stats.HistoryQuery, limit int) ([]*orders.Order, error) {
	args := []any{restaurantID}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	where := `restaurant_id = $1 AND ` + restaurantVisibleSQL
	switch q.Status {
	case stats.HistoryCompleted:
		where += ` AND status = ANY(` + arg(stats.CompletedStatusStrings()) + `)`
	case stats.HistoryCancelled:
		where += ` AND status = ANY(` + arg(stats.CancelledStatusStrings()) + `)`
	case stats.HistoryInProgress:
		where += ` AND status <> ALL(` + arg(stats.TerminalStatusStrings()) + `)`
	}
	where += periodSQL(q.Period, arg)
	if q.After != nil {
		at := arg(q.After.CreatedAt)
		id := arg(q.After.ID)
		where += ` AND (created_at, id COLLATE "C") < (` + at + `::timestamptz, ` + id + `::text COLLATE "C")`
	}
	lim := arg(limit)

	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+` FROM orders
		 WHERE `+where+`
		 ORDER BY created_at DESC, id COLLATE "C" DESC
		 LIMIT `+lim, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows)
}

// Lifetime — `stats.HistorySource` ning Postgres implementatsiyasi.
// Guruhlar va "faqat musbat summa" qoidasi `stats.Lifetime.Add` bilan
// bir xil.
func (r *PgOrderRepo) Lifetime(ctx context.Context, restaurantID string, p stats.Period) (stats.Lifetime, error) {
	args := []any{restaurantID, stats.CompletedStatusStrings(), stats.CancelledStatusStrings(),
		stats.TerminalStatusStrings()}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	query := `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE status = ANY($2)),
		       COUNT(*) FILTER (WHERE status = ANY($3)),
		       COALESCE(SUM(total_tiyin) FILTER (WHERE status = ANY($2) AND total_tiyin > 0), 0)::bigint,
		       COALESCE(SUM(total_tiyin) FILTER (WHERE status <> ALL($4) AND total_tiyin > 0), 0)::bigint,
		       MIN(created_at),
		       MAX(created_at)
		FROM orders
		WHERE restaurant_id = $1
		  AND ` + restaurantVisibleSQL + periodSQL(p, arg)

	var l stats.Lifetime
	var first, last *time.Time
	if err := r.pool.QueryRow(ctx, query, args...).
		Scan(&l.Orders, &l.Completed, &l.Cancelled, &l.RevenueTiyin, &l.InProgressTiyin, &first, &last); err != nil {
		return stats.Lifetime{}, err
	}
	l.InProgress = l.Orders - l.Completed - l.Cancelled
	l.FirstOrderAt, l.LastOrderAt = first, last
	l.Finish()
	return l, nil
}

// periodSQL — davr sharti. Faqat qiymat parametr sifatida qo'shiladi,
// SQL matniga hech qanday foydalanuvchi qiymati tushmaydi.
func periodSQL(p stats.Period, arg func(any) string) string {
	var s string
	if !p.From.IsZero() {
		s += ` AND created_at >= ` + arg(p.From)
	}
	if !p.To.IsZero() {
		s += ` AND created_at < ` + arg(p.To)
	}
	return s
}
