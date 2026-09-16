package storage

import (
	"context"
	"sync"
	"time"

	"chustapp/internal/tracking"
)

// MemoryRouteRepo — DATABASE_URL berilmagan holat uchun (PgRouteRepo bilan
// bir xil `tracking.Repository` shartnomasi).
type MemoryRouteRepo struct {
	mu   sync.RWMutex
	data map[string]tracking.Route
}

func NewMemoryRouteRepo() *MemoryRouteRepo {
	return &MemoryRouteRepo{data: make(map[string]tracking.Route)}
}

func (r *MemoryRouteRepo) GetRoute(_ context.Context, orderID string) (*tracking.Route, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rt, ok := r.data[orderID]
	if !ok {
		return nil, tracking.ErrNotFound
	}
	// Nusxa: chaqiruvchi nuqtalarni o'zgartirsa ombordagisi buzilmasin.
	rt.Points = append([]tracking.Point(nil), rt.Points...)
	return &rt, nil
}

func (r *MemoryRouteRepo) SaveRoute(_ context.Context, rt *tracking.Route) error {
	if err := rt.Validate(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.data[rt.OrderID]; exists {
		return nil // birinchi yozuv qoladi
	}
	saved := *rt
	saved.Points = append([]tracking.Point(nil), rt.Points...)
	if saved.CreatedAt.IsZero() {
		saved.CreatedAt = time.Now().UTC()
	}
	r.data[rt.OrderID] = saved
	return nil
}
