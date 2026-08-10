package storage

import (
	"context"
	"sync"

	"chustapp/internal/promotions"
)

type MemoryPromotionsRepo struct {
	mu   sync.RWMutex
	data map[string]promotions.Promotion
}

func NewMemoryPromotionsRepo() *MemoryPromotionsRepo {
	return &MemoryPromotionsRepo{data: make(map[string]promotions.Promotion)}
}

func (r *MemoryPromotionsRepo) ListByRestaurant(_ context.Context, restaurantID string) ([]*promotions.Promotion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*promotions.Promotion
	for _, x := range r.data {
		if x.RestaurantID == restaurantID {
			cp := x
			list = append(list, &cp)
		}
	}
	return list, nil
}

func (r *MemoryPromotionsRepo) GetByID(_ context.Context, id string) (*promotions.Promotion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	x, ok := r.data[id]
	if !ok {
		return nil, promotions.ErrNotFound
	}
	cp := x
	return &cp, nil
}

func (r *MemoryPromotionsRepo) Save(_ context.Context, x *promotions.Promotion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[x.ID] = *x
	return nil
}

func (r *MemoryPromotionsRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.data[id]; !ok {
		return promotions.ErrNotFound
	}
	delete(r.data, id)
	return nil
}

// DeleteByRestaurant — qarang: promotions.Repository izohi.
func (r *MemoryPromotionsRepo) DeleteByRestaurant(_ context.Context, restaurantID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for id, x := range r.data {
		if x.RestaurantID == restaurantID {
			delete(r.data, id)
			n++
		}
	}
	return n, nil
}

func (r *MemoryPromotionsRepo) IncrementUsage(_ context.Context, id string, amountTiyin int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	x, ok := r.data[id]
	if !ok {
		return promotions.ErrNotFound
	}
	x.UsageCount++
	x.SalesTotalTiyin += amountTiyin
	r.data[id] = x
	return nil
}
