package storage

import (
	"context"

	"chustapp/internal/stats"
)

var _ stats.PlatformSource = (*MemoryOrderRepo)(nil)

// PlatformStats — `stats.PlatformSource` ning xotira implementatsiyasi
// (dev rejimi va testlar). Qoidalar Postgres bilan bir xil: to'lanmagan
// karta buyurtmasi hisoblanmaydi (`Order.AwaitingPayment`), tasnif —
// `stats.ClassifyStatus`.
func (r *MemoryOrderRepo) PlatformStats(_ context.Context, q stats.PlatformQuery) (stats.PlatformData, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data := stats.PlatformData{
		Statuses: map[string]int{},
		Daily:    stats.NewDaily(q.DailyFrom, q.DailyDays),
	}
	byRestaurant := map[string]*stats.RestaurantTotals{}
	dailyTo := q.DailyFrom.AddDate(0, 0, q.DailyDays)

	for _, o := range r.data {
		if o.AwaitingPayment() {
			continue
		}
		class := stats.ClassifyStatus(o.Status)

		if !o.CreatedAt.Before(q.From) && o.CreatedAt.Before(q.To) {
			t := byRestaurant[o.RestaurantID]
			if t == nil {
				t = &stats.RestaurantTotals{RestaurantID: o.RestaurantID}
				byRestaurant[o.RestaurantID] = t
			}
			t.Orders++
			data.Statuses[string(o.Status)]++
			switch class {
			case "new":
				t.New++
			case "in_progress":
				t.InProgress++
			case "completed":
				t.Completed++
				t.RevenueTiyin += o.TotalTiyin
			case "cancelled":
				t.Cancelled++
			}
		}

		if !o.CreatedAt.Before(q.DailyFrom) && o.CreatedAt.Before(dailyTo) {
			if i := stats.DayIndex(q.DailyFrom, q.DailyDays, o.CreatedAt); i >= 0 {
				data.Daily[i].Orders++
				if class == "completed" {
					data.Daily[i].Completed++
					data.Daily[i].RevenueTiyin += o.TotalTiyin
				}
			}
		}
	}
	for _, t := range byRestaurant {
		data.Restaurants = append(data.Restaurants, *t)
	}
	return data, nil
}
