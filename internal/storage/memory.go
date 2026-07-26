// Package storage — in-memory implementatsiyalar. Faqat MVP skelet uchun;
// keyingi qadam: shu interface'larni PostgreSQL (+PostGIS) bilan almashtirish.
package storage

import (
	"context"
	"math"
	"sort"
	"sync"

	"chustapp/internal/couriers"
	"chustapp/internal/orders"
)

type MemoryOrderRepo struct {
	mu   sync.RWMutex
	data map[string]orders.Order
}

func NewMemoryOrderRepo() *MemoryOrderRepo {
	return &MemoryOrderRepo{data: make(map[string]orders.Order)}
}

func (r *MemoryOrderRepo) GetByID(_ context.Context, id string) (*orders.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.data[id]
	if !ok {
		return nil, orders.ErrNotFound
	}
	cp := o // nusxa qaytariladi, chaqiruvchi map ichini buzmasin
	return &cp, nil
}

func (r *MemoryOrderRepo) Save(_ context.Context, o *orders.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[o.ID] = *o
	return nil
}

func (r *MemoryOrderRepo) HasActiveByRestaurant(_ context.Context, restaurantID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, o := range r.data {
		if o.RestaurantID == restaurantID && !o.IsTerminal() {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryOrderRepo) ListRecent(_ context.Context, limit int) ([]*orders.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*orders.Order
	for _, o := range r.data {
		cp := o
		list = append(list, &cp)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

type MemoryCourierRepo struct {
	mu   sync.RWMutex
	data map[string]couriers.Courier
}

func NewMemoryCourierRepo(seed ...couriers.Courier) *MemoryCourierRepo {
	r := &MemoryCourierRepo{data: make(map[string]couriers.Courier)}
	for _, c := range seed {
		r.data[c.ID] = c
	}
	return r
}

func (r *MemoryCourierRepo) GetByID(_ context.Context, id string) (*couriers.Courier, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.data[id]
	if !ok {
		return nil, couriers.ErrNoCourier
	}
	cp := c
	return &cp, nil
}

func (r *MemoryCourierRepo) Create(_ context.Context, c *couriers.Courier) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[c.ID] = *c
	return nil
}

func (r *MemoryCourierRepo) ListAll(_ context.Context) ([]*couriers.Courier, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*couriers.Courier
	for _, c := range r.data {
		cp := c
		list = append(list, &cp)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list, nil
}

func (r *MemoryCourierRepo) SetApproved(_ context.Context, id string, approved bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.data[id]
	if !ok {
		return couriers.ErrNoCourier
	}
	c.Approved = approved
	r.data[id] = c
	return nil
}

func (r *MemoryCourierRepo) FindNearby(_ context.Context, lat, lng float64, limit int) ([]*couriers.Courier, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*couriers.Courier
	for _, c := range r.data {
		if c.Available && c.Approved {
			cp := c
			list = append(list, &cp)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return dist(lat, lng, list[i].Lat, list[i].Lng) < dist(lat, lng, list[j].Lat, list[j].Lng)
	})
	if len(list) > limit {
		list = list[:limit]
	}
	return list, nil
}

func (r *MemoryCourierRepo) SetAvailable(_ context.Context, id string, available bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.data[id]
	if !ok {
		return couriers.ErrNoCourier
	}
	c.Available = available
	r.data[id] = c
	return nil
}

// dist — taxminiy evklid masofa; shahar ichida saralash uchun yetarli.
// PostGIS'ga o'tganda bu funksiya umuman kerak bo'lmaydi.
func dist(lat1, lng1, lat2, lng2 float64) float64 {
	dLat := lat1 - lat2
	dLng := (lng1 - lng2) * math.Cos(lat1*math.Pi/180)
	return dLat*dLat + dLng*dLng
}
