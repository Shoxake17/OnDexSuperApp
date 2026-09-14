package storage

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/jackc/pgx/v5"

	"chustapp/internal/orders"
	"chustapp/internal/tables"
)

var (
	_ tables.OrdersSource = (*PgOrderRepo)(nil)
	_ tables.OrdersSource = (*MemoryOrderRepo)(nil)
)

// maxActiveDineIn — bitta so'rovda o'qiladigan yakunlanmagan stol
// buyurtmalari chegarasi: panel so'rovi cheksiz o'smasin (katta oshxonada
// ham bir vaqtda buncha ochiq stol buyurtmasi bo'lmaydi).
const maxActiveDineIn = 2000

func itemCount(items []orders.Item) int {
	n := 0
	for _, it := range items {
		if it.Qty > 0 {
			n += it.Qty
		}
	}
	return n
}

func snapshotOf(o *orders.Order) tables.OrderSnapshot {
	return tables.OrderSnapshot{
		ID:          o.ID,
		OrderNumber: o.OrderNumber,
		TableID:     o.TableID,
		Status:      string(o.Status),
		Items:       itemCount(o.Items),
		TotalTiyin:  o.TotalTiyin,
		CreatedAt:   o.CreatedAt,
	}
}

// ActiveDineInOrders — `tables.OrdersSource` ning Postgres
// implementatsiyasi. Indeks: `idx_orders_dine_in (restaurant_id, status)
// WHERE order_type = 'dine_in'` (0032).
func (r *PgOrderRepo) ActiveDineInOrders(ctx context.Context, restaurantID string) ([]tables.OrderSnapshot, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, COALESCE(order_number, ''), table_id, status, total_tiyin, items, created_at
		FROM orders
		WHERE restaurant_id = $1
		  AND order_type = 'dine_in'
		  AND table_id IS NOT NULL
		  AND status <> ALL($2)
		  AND `+restaurantVisibleSQL+`
		ORDER BY created_at DESC
		LIMIT $3`,
		restaurantID, orders.TerminalStatusStrings(), maxActiveDineIn)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSnapshots(rows)
}

// LatestDineInOrders — har bir joy uchun bitta indeks qidiruvi
// (`idx_orders_table_created`, 0044): restoranning butun buyurtmalar
// tarixi saralanmaydi.
func (r *PgOrderRepo) LatestDineInOrders(ctx context.Context, restaurantID string, tableIDs []string) (map[string]tables.OrderSnapshot, error) {
	out := make(map[string]tables.OrderSnapshot, len(tableIDs))
	if len(tableIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT o.id, o.order_number, o.table_id, o.status, o.total_tiyin, o.items, o.created_at
		FROM unnest($2::text[]) AS t(id)
		CROSS JOIN LATERAL (
			SELECT id, COALESCE(order_number, '') AS order_number, table_id, status,
			       total_tiyin, items, created_at
			FROM orders
			WHERE table_id = t.id
			  AND restaurant_id = $1
			  AND order_type = 'dine_in'
			  AND `+restaurantVisibleSQL+`
			ORDER BY created_at DESC
			LIMIT 1
		) o`,
		restaurantID, tableIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanSnapshots(rows)
	if err != nil {
		return nil, err
	}
	for _, s := range list {
		out[s.TableID] = s
	}
	return out, nil
}

func scanSnapshots(rows pgx.Rows) ([]tables.OrderSnapshot, error) {
	var out []tables.OrderSnapshot
	for rows.Next() {
		var s tables.OrderSnapshot
		var itemsJSON []byte
		if err := rows.Scan(&s.ID, &s.OrderNumber, &s.TableID, &s.Status, &s.TotalTiyin,
			&itemsJSON, &s.CreatedAt); err != nil {
			return nil, err
		}
		if len(itemsJSON) > 0 {
			var items []orders.Item
			if err := json.Unmarshal(itemsJSON, &items); err != nil {
				return nil, err
			}
			s.Items = itemCount(items)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ActiveDineInOrders — xotira implementatsiyasi (qoidalar Postgres bilan
// bir xil: faqat stol buyurtmasi, yakunlanmagan, to'lanmagan karta emas).
func (r *MemoryOrderRepo) ActiveDineInOrders(_ context.Context, restaurantID string) ([]tables.OrderSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []tables.OrderSnapshot
	for _, o := range r.data {
		cp := o
		if cp.RestaurantID != restaurantID || !cp.IsDineIn() || cp.TableID == "" ||
			orders.IsTerminal(cp.Status) || cp.AwaitingPayment() {
			continue
		}
		out = append(out, snapshotOf(&cp))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > maxActiveDineIn {
		out = out[:maxActiveDineIn]
	}
	return out, nil
}

// LatestDineInOrders — xotira implementatsiyasi.
func (r *MemoryOrderRepo) LatestDineInOrders(_ context.Context, restaurantID string, tableIDs []string) (map[string]tables.OrderSnapshot, error) {
	out := make(map[string]tables.OrderSnapshot, len(tableIDs))
	if len(tableIDs) == 0 {
		return out, nil
	}
	want := make(map[string]bool, len(tableIDs))
	for _, id := range tableIDs {
		want[id] = true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, o := range r.data {
		cp := o
		if cp.RestaurantID != restaurantID || !cp.IsDineIn() || !want[cp.TableID] || cp.AwaitingPayment() {
			continue
		}
		if prev, ok := out[cp.TableID]; !ok || cp.CreatedAt.After(prev.CreatedAt) {
			out[cp.TableID] = snapshotOf(&cp)
		}
	}
	return out, nil
}
