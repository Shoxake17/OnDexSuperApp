package storage

import (
	"context"
	"encoding/json"
	"time"

	"chustapp/internal/stats"
)

var _ stats.Source = (*PgOrderRepo)(nil)

// restaurantVisibleSQL — restoranga KO'RINADIGAN buyurtma sharti.
//
// `ListByRestaurant` (panel ro'yxati) va statistika AYNAN shu satrdan
// foydalanadi: ikkalasida alohida yozilsa, bir kun kelib ular ajralib,
// panel ko'rsatmagan to'lanmagan buyurtma tushumga qo'shilib ketardi.
// Xotira omboridagi mos qoida — `Order.AwaitingPayment()`.
const restaurantVisibleSQL = `NOT (payment_method = 'card' AND payment_state NOT IN ('held','paid'))`

// StatRows — `stats.Source` ning Postgres implementatsiyasi.
//
// ┌─ NEGA BUTUN BUYURTMA EMAS ─────────────────────────────────────────┐
// `orderColumns` bilan o'qilsa, har qatorga manzil, tarix va boshqa
// JSONB'lar ham tushardi — bir yillik davrda bu o'nlab MB. Bu yerda faqat
// hisob uchun kerak bo'lgan ustunlar olinadi:
//   - tarixdan faqat ikki vaqt (`jsonb_path_query_first`); tarix buzuq
//     bo'lsa ham so'rov yiqilmaydi — bo'sh satr qaytadi;
//   - taomlar faqat joriy davrdagi bajarilgan buyurtmalar uchun.
//
// └────────────────────────────────────────────────────────────────────┘
func (r *PgOrderRepo) StatRows(ctx context.Context, restaurantID string, from, to, itemsFrom time.Time, limit int) ([]stats.Row, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT customer_id, status, total_tiyin, created_at,
		       COALESCE(jsonb_path_query_first(history, '$[*] ? (@.to == "accepted").at') #>> '{}', ''),
		       COALESCE(jsonb_path_query_first(history, '$[*] ? (@.to == "ready").at') #>> '{}', ''),
		       CASE WHEN created_at >= $4 AND status = ANY($5) THEN items END
		FROM orders
		WHERE restaurant_id = $1
		  AND created_at >= $2 AND created_at < $3
		  AND `+restaurantVisibleSQL+`
		LIMIT $6`,
		restaurantID, from, to, itemsFrom, stats.CompletedStatusStrings(), limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []stats.Row
	for rows.Next() {
		var row stats.Row
		var acceptedAt, readyAt string
		var itemsJSON []byte
		if err := rows.Scan(&row.CustomerID, &row.Status, &row.TotalTiyin, &row.CreatedAt,
			&acceptedAt, &readyAt, &itemsJSON); err != nil {
			return nil, err
		}
		row.AcceptedAt = parseHistoryTime(acceptedAt)
		row.ReadyAt = parseHistoryTime(readyAt)
		if len(itemsJSON) > 0 {
			if err := json.Unmarshal(itemsJSON, &row.Items); err != nil {
				return nil, err
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// FirstOrderAt — `stats.Source` ning Postgres implementatsiyasi.
func (r *PgOrderRepo) FirstOrderAt(ctx context.Context, restaurantID string, customerIDs []string) (map[string]time.Time, error) {
	out := make(map[string]time.Time, len(customerIDs))
	if len(customerIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT customer_id, MIN(created_at)
		FROM orders
		WHERE restaurant_id = $1
		  AND customer_id = ANY($2)
		  AND status <> ALL($3)
		  AND `+restaurantVisibleSQL+`
		GROUP BY customer_id`,
		restaurantID, customerIDs, stats.CancelledStatusStrings())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var first time.Time
		if err := rows.Scan(&id, &first); err != nil {
			return nil, err
		}
		out[id] = first
	}
	return out, rows.Err()
}

// parseHistoryTime — tarixdagi vaqt (Go `time.Time` ning JSON shakli).
// O'qib bo'lmasa nol qaytadi: bitta buzuq yozuv butun statistikani
// to'xtatmasin, u shunchaki o'rtachaga kirmaydi.
func parseHistoryTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
