package storage

import (
	"context"
	"time"

	"chustapp/internal/orders"
)

// ListAwaitingCourier — kuryer kutayotgan yetkazish buyurtmalari. Kuryer
// qidiruvi nazoratchisi (`httpapi/dispatch_watchdog.go`) har 15 soniyada
// o'qiydi.
//
// Shart `orders.Order.AwaitsCourier` bilan AYNAN bir xil va migration
// 0048 dagi qisman indeks predikatiga so'zma-so'z mos. Holatlar
// parametr emas, LITERAL: parametr bilan rejalashtiruvchi qisman
// indeksni ishlatmaydi va har tsiklda butun jadval o'qiladi.
func (r *PgOrderRepo) ListAwaitingCourier(ctx context.Context, limit int) ([]*orders.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+` FROM orders
		 WHERE courier_id IS NULL
		   AND status IN ('accepted', 'preparing', 'ready')
		   AND order_type <> 'dine_in'
		   AND order_type <> 'pickup'
		 ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows)
}

// ListReadyPickups — tayyor, hali olinmagan olib ketish buyurtmalari
// (eslatma va 2 soatlik "olib ketilmadi" nazoratchisi; migratsiya 0080
// qisman indeksi bilan bir xil predikat).
func (r *PgOrderRepo) ListReadyPickups(ctx context.Context, limit int) ([]*orders.Order, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+` FROM orders
		 WHERE order_type = 'pickup' AND status = 'ready'
		 ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows)
}

// ListAbandonedUnpaid — `before` dan oldin yaratilgan, to'lovi hali
// kutilayotgan KARTA buyurtmalari ID'lari (audit 480: to'lov sahifasi
// ochilmay tashlab ketilganlar abadiy `created` bo'lib qolardi).
// Yakuniy qaror `payments.Service.CloseAbandoned` da (urinishlar tekshiriladi).
func (r *PgOrderRepo) ListAbandonedUnpaid(ctx context.Context, before time.Time, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id FROM orders
		 WHERE status = 'created' AND payment_method = 'card' AND payment_state = 'awaiting'
		   AND created_at < $1
		 ORDER BY created_at LIMIT $2`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
