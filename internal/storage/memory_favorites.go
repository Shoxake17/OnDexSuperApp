package storage

import (
	"context"
	"sync"
	"time"
)

type memFavorite struct {
	productID string
	addedAt   time.Time
}

// MemoryFavoritesRepo — DATABASE_URL berilmagan (masalan test/lokal dev)
// holatdagi zaxira implementatsiya (PgFavoritesRepo bilan bir xil
// Repository interfeysini qanoatlantiradi).
type MemoryFavoritesRepo struct {
	mu   sync.RWMutex
	data map[string][]memFavorite // customer_id -> ro'yxat
}

func NewMemoryFavoritesRepo() *MemoryFavoritesRepo {
	return &MemoryFavoritesRepo{data: make(map[string][]memFavorite)}
}

func (r *MemoryFavoritesRepo) Add(_ context.Context, customerID, productID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range r.data[customerID] {
		if f.productID == productID {
			return nil // allaqachon bor — idempotent
		}
	}
	r.data[customerID] = append(r.data[customerID], memFavorite{productID: productID, addedAt: time.Now()})
	return nil
}

func (r *MemoryFavoritesRepo) Remove(_ context.Context, customerID, productID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	list := r.data[customerID]
	for i, f := range list {
		if f.productID == productID {
			r.data[customerID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *MemoryFavoritesRepo) ListProductIDs(_ context.Context, customerID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.data[customerID]
	// ENG YANGISI birinchi (PgFavoritesRepo'dagi ORDER BY created_at DESC
	// bilan bir xil tartib).
	ids := make([]string, len(list))
	for i, f := range list {
		ids[len(list)-1-i] = f.productID
	}
	return ids, nil
}
