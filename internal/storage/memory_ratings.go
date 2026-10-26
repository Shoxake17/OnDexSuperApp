package storage

import (
	"context"
	"sync"

	"chustapp/internal/ratings"
)

// MemoryRatingRepo — Postgres ulanmagan rejim (lokal sinov, e2e) uchun.
type MemoryRatingRepo struct {
	mu      sync.RWMutex
	byOrder map[string]ratings.Rating
}

func NewMemoryRatingRepo() *MemoryRatingRepo {
	return &MemoryRatingRepo{byOrder: map[string]ratings.Rating{}}
}

func (r *MemoryRatingRepo) Create(_ context.Context, rt *ratings.Rating) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byOrder[rt.OrderID]; ok {
		return ratings.ErrAlreadyRated
	}
	r.byOrder[rt.OrderID] = *rt
	return nil
}

func (r *MemoryRatingRepo) GetByOrder(_ context.Context, orderID string) (*ratings.Rating, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rt, ok := r.byOrder[orderID]
	if !ok {
		return nil, ratings.ErrNotFound
	}
	cp := rt
	return &cp, nil
}

func (r *MemoryRatingRepo) Summary(_ context.Context, restaurantID string) (float64, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var sum, count int
	for _, rt := range r.byOrder {
		if rt.RestaurantID != restaurantID {
			continue
		}
		sum += rt.Stars
		count++
	}
	if count == 0 {
		return 0, 0, nil
	}
	return float64(sum) / float64(count), count, nil
}
