package storage

import (
	"context"

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
		 ORDER BY created_at LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrderRows(rows)
}
