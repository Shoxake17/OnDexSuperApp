package storage

import (
	"context"
	"strings"
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

// GetProductsByIDs — takroriy ID'lar uchun mahsulot BIR MARTA
// qaytariladi.
//
// Nima uchun muhim: Postgres (`= ANY($1)`) va Mongo (`$in`) tabiiy
// ravishda dedupe qiladi, bu implementatsiya esa qilmasdi. Natijada
// chaqiruvchilardagi `len(products) != len(ids)` shaklidagi tekshiruvlar
// (masalan aksiya handleridagi "tanlangan mahsulotlardan biri topilmadi")
// backend'ga qarab HAR XIL ishlardi: memory'da o'tib ketardi, DB'da rad
// etilardi. Xatti-harakat uch backend'da ham bir xil bo'lishi shart.
func (r *MemoryCatalogRepo) GetProductsByIDs(_ context.Context, ids []string) ([]*catalog.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*catalog.Product
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if x, ok := r.products[id]; ok {
			cp := x
			list = append(list, &cp)
		}
	}
	return list, nil
}

func (r *MemoryCatalogRepo) SearchProducts(_ context.Context, query string) ([]*catalog.ProductSearchResult, error) {
	nq := catalog.NormalizeForSearch(query)
	if nq == "" {
		return nil, nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []*catalog.ProductSearchResult
	for _, p := range r.products {
		if !p.Available {
			continue
		}
		if !strings.Contains(catalog.NormalizeForSearch(p.Name), nq) &&
			!strings.Contains(catalog.NormalizeForSearch(p.Category), nq) {
			continue
		}
		rest, ok := r.restaurants[p.RestaurantID]
		if !ok {
			continue
		}
		list = append(list, &catalog.ProductSearchResult{
			Product:           p,
			RestaurantName:    rest.Name,
			RestaurantLogoURL: rest.LogoURL,
			RestaurantOpen:    rest.Open,
		})
	}
	return list, nil
}

func (r *MemoryCatalogRepo) SaveProduct(_ context.Context, x *catalog.Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.products[x.ID] = *x
	return nil
}

func (r *MemoryCatalogRepo) DeleteProduct(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.products[id]; !ok {
		return catalog.ErrNotFound
	}
	delete(r.products, id)
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
		{ID: "p1", RestaurantID: "r1", Name: "Osh", Category: "Milliy taomlar", PriceTiyin: 3500000, Available: true},
		{ID: "p2", RestaurantID: "r1", Name: "Lag'mon", Category: "Milliy taomlar", PriceTiyin: 3000000, Available: true},
		{ID: "p3", RestaurantID: "r1", Name: "Choy (damlama)", Category: "Ichimliklar", PriceTiyin: 500000, Available: true},
	}
}
