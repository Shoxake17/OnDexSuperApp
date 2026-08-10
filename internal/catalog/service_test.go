package catalog

import (
	"context"
	"errors"
	"testing"
)

type fakeRepo struct {
	restaurants map[string]*Restaurant
	products    map[string]*Product
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		restaurants: map[string]*Restaurant{
			"r1": {ID: "r1", Name: "Test Osh", Open: true},
			"r2": {ID: "r2", Name: "Yopiq Kafe", Open: false},
		},
		products: map[string]*Product{
			"p1": {ID: "p1", RestaurantID: "r1", Name: "Osh", PriceTiyin: 3500000, Available: true},
			"p2": {ID: "p2", RestaurantID: "r1", Name: "Choy", PriceTiyin: 500000, Available: true},
			"p3": {ID: "p3", RestaurantID: "r1", Name: "Tugagan taom", PriceTiyin: 100, Available: false},
			"p4": {ID: "p4", RestaurantID: "r2", Name: "Lag'mon", PriceTiyin: 3000000, Available: true},
		},
	}
}

func (r *fakeRepo) ListRestaurants(_ context.Context) ([]*Restaurant, error) { return nil, nil }
func (r *fakeRepo) GetRestaurant(_ context.Context, id string) (*Restaurant, error) {
	if x, ok := r.restaurants[id]; ok {
		return x, nil
	}
	return nil, ErrNotFound
}
func (r *fakeRepo) SaveRestaurant(_ context.Context, _ *Restaurant) error { return nil }
func (r *fakeRepo) DeleteRestaurant(_ context.Context, id string) error {
	delete(r.restaurants, id)
	return nil
}
func (r *fakeRepo) ListProducts(_ context.Context, _ string) ([]*Product, error) {
	return nil, nil
}
func (r *fakeRepo) GetProductsByIDs(_ context.Context, ids []string) ([]*Product, error) {
	var out []*Product
	for _, id := range ids {
		if p, ok := r.products[id]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *fakeRepo) SearchProducts(_ context.Context, _ string) ([]*ProductSearchResult, error) {
	return nil, nil
}
func (r *fakeRepo) SaveProduct(_ context.Context, _ *Product) error { return nil }
func (r *fakeRepo) DeleteProduct(_ context.Context, id string) error {
	delete(r.products, id)
	return nil
}

func TestPriceOrderComputesFromCatalog(t *testing.T) {
	s := NewService(newFakeRepo())
	restID, items, err := s.PriceOrder(context.Background(), []ItemRequest{
		{ProductID: "p1", Qty: 2},
		{ProductID: "p2", Qty: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if restID != "r1" {
		t.Errorf("restoran r1 kutilgan, olindi %s", restID)
	}
	var total int64
	for _, it := range items {
		total += it.PriceTiyin * int64(it.Qty)
	}
	if total != 2*3500000+500000 {
		t.Errorf("jami 7500000 kutilgan, olindi %d", total)
	}
}

func TestPriceOrderRejectsBadInput(t *testing.T) {
	s := NewService(newFakeRepo())
	cases := []struct {
		name  string
		items []ItemRequest
		want  error // nil bo'lsa faqat xato borligini tekshiramiz
	}{
		{"bo'sh buyurtma", nil, nil},
		{"nol miqdor", []ItemRequest{{ProductID: "p1", Qty: 0}}, nil},
		{"manfiy miqdor", []ItemRequest{{ProductID: "p1", Qty: -1}}, nil},
		{"mavjud bo'lmagan taom", []ItemRequest{{ProductID: "yoq", Qty: 1}}, nil},
		{"tugagan taom", []ItemRequest{{ProductID: "p3", Qty: 1}}, ErrUnavailable},
		{"ikki restoran aralash", []ItemRequest{{ProductID: "p1", Qty: 1}, {ProductID: "p4", Qty: 1}}, ErrMixedRestaurants},
		{"yopiq restoran", []ItemRequest{{ProductID: "p4", Qty: 1}}, ErrRestaurantClosed},
	}
	for _, c := range cases {
		_, _, err := s.PriceOrder(context.Background(), c.items)
		if err == nil {
			t.Errorf("%s: xato kutilgan edi", c.name)
			continue
		}
		if c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%s: %v kutilgan, olindi %v", c.name, c.want, err)
		}
	}
}
