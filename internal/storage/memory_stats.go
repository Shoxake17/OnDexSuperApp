package storage

import (
	"context"
	"slices"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/stats"
)

var _ stats.Source = (*MemoryOrderRepo)(nil)

// StatRows — `stats.Source` ning xotira implementatsiyasi. Ko'rinish
// qoidasi `ListByRestaurant` bilan bir xil (`AwaitingPayment`), tarixdan
// esa Postgres'dagi kabi BIRINCHI mos yozuv olinadi.
func (r *MemoryOrderRepo) StatRows(_ context.Context, restaurantID string, from, to, itemsFrom time.Time, limit int) ([]stats.Row, error) {
	completed := stats.CompletedStatusStrings()
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []stats.Row
	for _, o := range r.data {
		cp := o
		if cp.RestaurantID != restaurantID || cp.AwaitingPayment() {
			continue
		}
		if cp.CreatedAt.Before(from) || !cp.CreatedAt.Before(to) {
			continue
		}
		row := stats.Row{
			CustomerID: cp.CustomerID,
			Status:     cp.Status,
			TotalTiyin: cp.TotalTiyin,
			CreatedAt:  cp.CreatedAt,
		}
		for _, h := range cp.History {
			if h.To == orders.StatusAccepted && row.AcceptedAt.IsZero() {
				row.AcceptedAt = h.At
			}
			if h.To == orders.StatusReady && row.ReadyAt.IsZero() {
				row.ReadyAt = h.At
			}
		}
		if !cp.CreatedAt.Before(itemsFrom) && slices.Contains(completed, string(cp.Status)) {
			row.Items = slices.Clone(cp.Items)
		}
		out = append(out, row)
		if len(out) > limit {
			break
		}
	}
	return out, nil
}

// FirstOrderAt — `stats.Source` ning xotira implementatsiyasi.
func (r *MemoryOrderRepo) FirstOrderAt(_ context.Context, restaurantID string, customerIDs []string) (map[string]time.Time, error) {
	out := make(map[string]time.Time, len(customerIDs))
	if len(customerIDs) == 0 {
		return out, nil
	}
	want := make(map[string]bool, len(customerIDs))
	for _, id := range customerIDs {
		want[id] = true
	}
	cancelled := stats.CancelledStatusStrings()

	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, o := range r.data {
		cp := o
		if cp.RestaurantID != restaurantID || !want[cp.CustomerID] || cp.AwaitingPayment() ||
			slices.Contains(cancelled, string(cp.Status)) {
			continue
		}
		if t, ok := out[cp.CustomerID]; !ok || cp.CreatedAt.Before(t) {
			out[cp.CustomerID] = cp.CreatedAt
		}
	}
	return out, nil
}
