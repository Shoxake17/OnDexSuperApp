package catalog

import (
	"context"
	"testing"
)

// `PriceOrder` narxlash qatlamiga IKKALA narxni ham xom holda
// uzatishini tekshiradi: `PriceTiyin` — har doim ASL narx,
// `DiscountPriceTiyin` — mahsulot chegirmasi (0 = yo'q).
//
// Nima uchun asl narx: avval bu yerga to'g'ridan-to'g'ri chegirma narxi
// qo'yilardi va narxlash qatlami uning ustiga YANA aksiya chegirmasini
// qo'shardi — jami nolga tushib ketardi. Endi tanlash narxlash
// qatlamida (`orders.priceCart`), bu yerda emas.
func TestPriceOrderCarriesBothPrices(t *testing.T) {
	repo := &fakeRepo{
		restaurants: map[string]*Restaurant{"r1": {ID: "r1", Open: true}},
		products: map[string]*Product{
			"p1": {
				ID: "p1", RestaurantID: "r1", Name: "Osh", Available: true,
				PriceTiyin: 1000000, DiscountPriceTiyin: 600000,
			},
			"p2": {
				ID: "p2", RestaurantID: "r1", Name: "Choy", Available: true,
				PriceTiyin: 500000, // chegirmasiz
			},
		},
	}
	svc := NewService(repo)

	_, items, err := svc.PriceOrder(context.Background(), []ItemRequest{
		{ProductID: "p1", Qty: 2},
		{ProductID: "p2", Qty: 1},
	})
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("2 ta element kutilgan, olindi %d", len(items))
	}
	if items[0].PriceTiyin != 1000000 {
		t.Errorf("asl narx uzatilishi kerak edi: %d, kutilgan 1000000", items[0].PriceTiyin)
	}
	if items[0].DiscountPriceTiyin != 600000 {
		t.Errorf("chegirma narxi uzatilmadi: %d, kutilgan 600000", items[0].DiscountPriceTiyin)
	}
	if items[1].PriceTiyin != 500000 {
		t.Errorf("chegirmasiz mahsulot narxi o'zgarib ketdi: %d", items[1].PriceTiyin)
	}
	if items[1].DiscountPriceTiyin != 0 {
		t.Errorf("chegirmasiz mahsulotda chegirma paydo bo'ldi: %d", items[1].DiscountPriceTiyin)
	}
}

// Noto'g'ri (narxdan katta yoki teng) chegirma E'TIBORGA OLINMAYDI —
// bu holat yozishda bloklanadi, lekin eski/buzilgan yozuv bo'lsa ham
// mijozdan ortiqcha pul olinmasligi kerak.
func TestPriceOrderIgnoresInvalidDiscount(t *testing.T) {
	repo := &fakeRepo{
		restaurants: map[string]*Restaurant{"r1": {ID: "r1", Open: true}},
		products: map[string]*Product{
			"p1": {
				ID: "p1", RestaurantID: "r1", Name: "Osh", Available: true,
				PriceTiyin: 500000, DiscountPriceTiyin: 900000, // narxdan KATTA
			},
		},
	}
	svc := NewService(repo)
	_, items, err := svc.PriceOrder(context.Background(), []ItemRequest{{ProductID: "p1", Qty: 1}})
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if items[0].PriceTiyin != 500000 {
		t.Errorf("asosiy narx ishlatilishi kerak edi: %d", items[0].PriceTiyin)
	}
	if items[0].DiscountPriceTiyin != 0 {
		t.Errorf("noto'g'ri chegirma tashlanishi kerak edi: %d", items[0].DiscountPriceTiyin)
	}
}

// Savatdagi TURLAR soni chegarasi — chegarasiz massiv katta DB
// so'roviga va int64 toshib ketishiga olib kelishi mumkin edi.
func TestPriceOrderRejectsTooManyDistinctItems(t *testing.T) {
	repo := &fakeRepo{
		restaurants: map[string]*Restaurant{"r1": {ID: "r1", Open: true}},
		products:    map[string]*Product{},
	}
	svc := NewService(repo)
	reqs := make([]ItemRequest, 101)
	for i := range reqs {
		reqs[i] = ItemRequest{ProductID: "p", Qty: 1}
	}
	if _, _, err := svc.PriceOrder(context.Background(), reqs); err == nil {
		t.Error("101 tur mahsulot rad etilishi kerak edi")
	}
}
