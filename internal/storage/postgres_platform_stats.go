package storage

import (
	"context"

	"chustapp/internal/stats"
)

var _ stats.PlatformSource = (*PgOrderRepo)(nil)

// PlatformStats — `stats.PlatformSource` ning Postgres implementatsiyasi.
//
// Hisob SQL ichida: superadmin BARCHA restoranlarni ko'radi, qatorlarni
// serverga olib kelish yuz minglab yozuv bo'lardi. `restaurantVisibleSQL`
// restoran panelidagi ro'yxat va restoran statistikasi bilan bir xil:
// to'lanmagan karta buyurtmasi hech qayerda hisoblanmaydi.
func (r *PgOrderRepo) PlatformStats(ctx context.Context, q stats.PlatformQuery) (stats.PlatformData, error) {
	var data stats.PlatformData
	completed := stats.CompletedStatusStrings()
	cancelled := stats.CancelledStatusStrings()

	rows, err := r.pool.Query(ctx, `
		SELECT restaurant_id,
		       COUNT(*),
		       COUNT(*) FILTER (WHERE status = 'created'),
		       COUNT(*) FILTER (WHERE status <> 'created' AND status <> ALL($3) AND status <> ALL($4)),
		       COUNT(*) FILTER (WHERE status = ANY($3)),
		       COUNT(*) FILTER (WHERE status = ANY($4)),
		       COALESCE(SUM(total_tiyin) FILTER (WHERE status = ANY($3)), 0)::bigint
		FROM orders
		WHERE created_at >= $1 AND created_at < $2
		  AND `+restaurantVisibleSQL+`
		GROUP BY restaurant_id`,
		q.From, q.To, completed, cancelled)
	if err != nil {
		return data, err
	}
	for rows.Next() {
		var t stats.RestaurantTotals
		if err := rows.Scan(&t.RestaurantID, &t.Orders, &t.New, &t.InProgress,
			&t.Completed, &t.Cancelled, &t.RevenueTiyin); err != nil {
			rows.Close()
			return data, err
		}
		data.Restaurants = append(data.Restaurants, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return data, err
	}

	data.Statuses = map[string]int{}
	srows, err := r.pool.Query(ctx, `
		SELECT status, COUNT(*)
		FROM orders
		WHERE created_at >= $1 AND created_at < $2
		  AND `+restaurantVisibleSQL+`
		GROUP BY status`, q.From, q.To)
	if err != nil {
		return data, err
	}
	for srows.Next() {
		var status string
		var n int
		if err := srows.Scan(&status, &n); err != nil {
			srows.Close()
			return data, err
		}
		data.Statuses[status] = n
	}
	srows.Close()
	if err := srows.Err(); err != nil {
		return data, err
	}

	// Kunlik grafik: kun raqami = (created_at - DailyFrom) / 24 soat. Vaqt
	// zonasi qat'iy +05:00 (`stats.Location`), DailyFrom esa o'sha zonadagi
	// yarim tun — shuning uchun kunlar chegarasi aynan mos tushadi.
	data.Daily = stats.NewDaily(q.DailyFrom, q.DailyDays)
	dailyTo := q.DailyFrom.AddDate(0, 0, q.DailyDays)
	drows, err := r.pool.Query(ctx, `
		SELECT floor(extract(epoch FROM (created_at - $1::timestamptz)) / 86400)::int AS d,
		       COUNT(*),
		       COUNT(*) FILTER (WHERE status = ANY($3)),
		       COALESCE(SUM(total_tiyin) FILTER (WHERE status = ANY($3)), 0)::bigint
		FROM orders
		WHERE created_at >= $1 AND created_at < $2
		  AND `+restaurantVisibleSQL+`
		GROUP BY d`, q.DailyFrom, dailyTo, completed)
	if err != nil {
		return data, err
	}
	defer drows.Close()
	for drows.Next() {
		var d, orders, done int
		var revenue int64
		if err := drows.Scan(&d, &orders, &done, &revenue); err != nil {
			return data, err
		}
		if d < 0 || d >= len(data.Daily) {
			continue
		}
		data.Daily[d].Orders = orders
		data.Daily[d].Completed = done
		data.Daily[d].RevenueTiyin = revenue
	}
	return data, drows.Err()
}
