package storage

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"chustapp/internal/alerts"
)

// MemoryAlertStore — DATABASE_URL berilmaganda (dev va testlar). Postgres
// bilan bir xil qoidalar: o'suvchi `seq`, restoran bo'yicha `dedupe_key`
// unikal, `after` so'rovi o'sish tartibida.
type MemoryAlertStore struct {
	mu    sync.RWMutex
	seq   int64
	items []alerts.Notification
}

func NewMemoryAlertStore() *MemoryAlertStore { return &MemoryAlertStore{} }

func cloneAlert(n alerts.Notification) alerts.Notification {
	if n.Data != nil {
		d := make(map[string]string, len(n.Data))
		for k, v := range n.Data {
			d[k] = v
		}
		n.Data = d
	}
	if n.ReadAt != nil {
		v := *n.ReadAt
		n.ReadAt = &v
	}
	return n
}

func (r *MemoryAlertStore) Insert(_ context.Context, n *alerts.Notification) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.items {
		if x.RestaurantID == n.RestaurantID && x.DedupeKey == n.DedupeKey {
			return false, nil
		}
	}
	r.seq++
	n.Seq = r.seq
	r.items = append(r.items, cloneAlert(*n))
	return true, nil
}

func (r *MemoryAlertStore) List(_ context.Context, restaurantID string, q alerts.Query) ([]*alerts.Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	search := strings.ToLower(q.Search)
	var out []*alerts.Notification
	for _, x := range r.items {
		if x.RestaurantID != restaurantID ||
			(q.Category != "" && x.Category != q.Category) ||
			(!q.Since.IsZero() && x.CreatedAt.Before(q.Since)) ||
			(q.BeforeSeq > 0 && x.Seq >= q.BeforeSeq) ||
			(q.AfterSeq > 0 && x.Seq <= q.AfterSeq) ||
			(q.UnreadOnly && x.ReadAt != nil) ||
			(search != "" && !strings.Contains(strings.ToLower(x.Title+"\n"+x.Body), search)) {
			continue
		}
		c := cloneAlert(x)
		out = append(out, &c)
	}
	if q.AfterSeq > 0 {
		sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	} else {
		sort.Slice(out, func(i, j int) bool { return out[i].Seq > out[j].Seq })
	}
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func (r *MemoryAlertStore) Counts(_ context.Context, restaurantID string, since time.Time) (alerts.Counts, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c := alerts.Counts{ByCategory: map[alerts.Category]int{}}
	for _, x := range r.items {
		if x.RestaurantID != restaurantID || (!since.IsZero() && x.CreatedAt.Before(since)) {
			continue
		}
		c.Total++
		c.ByCategory[x.Category]++
		if x.ReadAt == nil {
			c.Unread++
		}
	}
	return c, nil
}

func (r *MemoryAlertStore) UnreadCount(_ context.Context, restaurantID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for _, x := range r.items {
		if x.RestaurantID == restaurantID && x.ReadAt == nil {
			n++
		}
	}
	return n, nil
}

func (r *MemoryAlertStore) LatestSeq(_ context.Context, restaurantID string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var seq int64
	for _, x := range r.items {
		if x.RestaurantID == restaurantID && x.Seq > seq {
			seq = x.Seq
		}
	}
	return seq, nil
}

func (r *MemoryAlertStore) MarkRead(_ context.Context, restaurantID, id string, at time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.items {
		x := &r.items[i]
		if x.ID == id && x.RestaurantID == restaurantID && x.ReadAt == nil {
			t := at
			x.ReadAt = &t
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryAlertStore) MarkAllRead(_ context.Context, restaurantID string, upToSeq int64, at time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for i := range r.items {
		x := &r.items[i]
		if x.RestaurantID == restaurantID && x.ReadAt == nil && x.Seq <= upToSeq {
			t := at
			x.ReadAt = &t
			n++
		}
	}
	return n, nil
}

func (r *MemoryAlertStore) DeleteOlderThan(_ context.Context, before time.Time) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.items[:0]
	n := 0
	for _, x := range r.items {
		if x.CreatedAt.Before(before) {
			n++
			continue
		}
		kept = append(kept, x)
	}
	r.items = kept
	return n, nil
}
