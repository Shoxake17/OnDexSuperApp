package storage

import (
	"context"
	"sort"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/stats"
)

var _ stats.HistorySource = (*MemoryOrderRepo)(nil)

// OrderHistory — `stats.HistorySource` ning xotira implementatsiyasi.
// Tartib, davr va kursor qoidasi Postgres'dagi bilan bir xil.
func (r *MemoryOrderRepo) OrderHistory(_ context.Context, restaurantID string, q stats.HistoryQuery, limit int) ([]*orders.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var list []*orders.Order
	for _, o := range r.data {
		cp := o
		if cp.RestaurantID != restaurantID || cp.AwaitingPayment() ||
			!q.Status.Matches(cp.Status) || !q.Period.Contains(cp.CreatedAt) {
			continue
		}
		if q.After != nil && !q.After.Precedes(cp.CreatedAt, cp.ID) {
			continue
		}
		list = append(list, &cp)
	}
	sort.Slice(list, func(i, j int) bool {
		ti := list[i].CreatedAt.Truncate(time.Microsecond)
		tj := list[j].CreatedAt.Truncate(time.Microsecond)
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return list[i].ID > list[j].ID
	})
	if limit >= 0 && len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

// Lifetime — `stats.HistorySource` ning xotira implementatsiyasi.
func (r *MemoryOrderRepo) Lifetime(_ context.Context, restaurantID string, p stats.Period) (stats.Lifetime, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var l stats.Lifetime
	for _, o := range r.data {
		cp := o
		if cp.RestaurantID != restaurantID || cp.AwaitingPayment() || !p.Contains(cp.CreatedAt) {
			continue
		}
		l.Add(cp.Status, cp.TotalTiyin, cp.CreatedAt)
	}
	l.Finish()
	return l, nil
}
