package storage

import (
	"context"
	"sync"

	"chustapp/internal/catalog"
)

type MemoryCatalogRepo struct {
	mu          sync.RWMutex
	restaurants map[string]catalog.Restaurant
	products    map[string]catalog.Product
}

func NewMemoryCatalogRepo(restaurants []catalog.Restaurant, products []catalog.Product) *MemoryCatalogRepo {
	r := &MemoryCatalogRepo{
		restaurants: make(map[string]catalog.Restaurant),
		products:    make(map[string]catalog.Product),
	}
	for _, x := range restaurants {
		r.restaurants[x.ID] = x
	}
	for _, x := range products {
		r.products[x.ID] = x
	}
	return r
}

func (r *MemoryCatalogRepo) ListRestaurants(_ context.Context) ([]*catalog.Restaurant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*catalog.Restaurant
	for _, x := range r.restaurants {
		cp := x
		list = append(list, &cp)
	}
	return list, nil
}

func (r *MemoryCatalogRepo) GetRestaurant(_ context.Context, id string) (*catalog.Restaurant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	x, ok := r.restaurants[id]
	if !ok {
		return nil, catalog.ErrNotFound
	}
	cp := x
	return &cp, nil
}

func (r *MemoryCatalogRepo) SaveRestaurant(_ context.Context, x *catalog.Restaurant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.restaurants[x.ID] = *x
	return nil
}

func (r *MemoryCatalogRepo) DeleteRestaurant(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.restaurants[id]; !ok {
		return catalog.ErrNotFound
	}
	delete(r.restaurants, id)
	for pid, p := range r.products {
		if p.RestaurantID == id {
			delete(r.products, pid)
		}
	}
	return nil
}

func (r *MemoryCatalogRepo) ListProducts(_ context.Context, restaurantID string) ([]*catalog.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*catalog.Product
	for _, x := range r.products {
		if x.RestaurantID == restaurantID {
			cp := x
			list = append(list, &cp)
		}
	}
	return list, nil
}

func (r *MemoryCatalogRepo) GetProductsByIDs(_ context.Context, ids []string) ([]*catalog.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*catalog.Product
	for _, id := range ids {
		if x, ok := r.products[id]; ok {
			cp := x
			list = append(list, &cp)
		}
	}
	return list, nil
}

func (r *MemoryCatalogRepo) SaveProduct(_ context.Context, x *catalog.Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.products[x.ID] = *x
	return nil
}

// DemoRestaurants / DemoProducts — dev muhit uchun boshlang'ich katalog.
func DemoRestaurants() []catalog.Restaurant {
	return []catalog.Restaurant{
		{ID: "r1", Name: "Chust Osh Markazi", Address: "Chust, A.Navoiy ko'chasi", Lat: 41.0030, Lng: 71.2360, Open: true},
	}
}

func DemoProducts() []catalog.Product {
	return []catalog.Product{
		{ID: "p1", RestaurantID: "r1", Name: "Osh", PriceTiyin: 3500000, Available: true},
		{ID: "p2", RestaurantID: "r1", Name: "Lag'mon", PriceTiyin: 3000000, Available: true},
		{ID: "p3", RestaurantID: "r1", Name: "Choy (damlama)", PriceTiyin: 500000, Available: true},
	}
}
