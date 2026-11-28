package main

import (
	"testing"
	"time"
)

func TestTiyinAndQty(t *testing.T) {
	cases := map[float64]int64{12500: 1250000, 12500.5: 1250050, 0.005: 1, -3: 0, 0: 0}
	for som, want := range cases {
		if got := tiyin(som); got != want {
			t.Fatalf("tiyin(%v) = %d, kutilgan %d", som, got, want)
		}
	}
	if qty(1.23456) != 1.235 || qty(-2) != 0 {
		t.Fatal("qty yaxlitlash")
	}
}

func TestBuildStores(t *testing.T) {
	stores, skipped := buildStores([]mUser{
		{Login: "Baraka", EntityID: "s1", StoreCategory: "store"},
		{Login: "qurtchi", EntityID: "s2", StoreCategory: "qurt"},
		{Login: "apteka", EntityID: "s3", StoreCategory: "pharmacy"},
		{Login: "eski", EntityID: "s4", IsDeleted: true},
		{Login: "boglanmagan"},
		{Login: "restoran", EntityID: "r1", PartnerRole: "restaurant"},
		{Login: "kategoriyasiz", EntityID: "s5"},
	})
	if len(stores) != 4 || stores["baraka"].ID != "s1" || stores["qurtchi"].ShopType != "qurt" ||
		stores["apteka"].ShopType != "dorixona" || stores["kategoriyasiz"].Category != "store" {
		t.Fatalf("do'konlar: %+v", stores)
	}
	if len(skipped) != 3 {
		t.Fatalf("o'tkazilganlar: %v", skipped)
	}
}

func TestAggregateSalesMergesPerDayAndPayment(t *testing.T) {
	tz, _ := time.LoadLocation("Asia/Tashkent")
	stores := map[string]storeOf{"baraka": {ID: "s1"}}
	d := func(h int) time.Time { return time.Date(2026, 11, 27, h, 0, 0, 0, tz) }
	sales := []mSale{
		{ID: "a", Login: "Baraka", Barcode: "p1", Name: "Non", Quantity: 2, Price: 5000, Total: 10000, PaymentMethod: "naqd", SoldAt: d(10)},
		{ID: "b", Login: "baraka", Barcode: "p1", Name: "Non yangi", Quantity: 1, Price: 5500, Total: 5500, PaymentMethod: "naqd", SoldAt: d(12)},
		{ID: "c", Login: "baraka", Barcode: "p1", Quantity: 1, Price: 5500, PaymentMethod: "plastik", SoldAt: d(13)},
		// 23:30 Toshkent = 18:30 UTC — baribir 27-noyabr kuni.
		{ID: "d", Login: "baraka", Barcode: "p1", Quantity: 1, Price: 5500, Total: 5500, PaymentMethod: "naqd", SoldAt: time.Date(2026, 11, 27, 18, 30, 0, 0, time.UTC)},
		// 00:30 Toshkent ertasi kun.
		{ID: "e", Login: "baraka", Barcode: "p1", Quantity: 1, Price: 5500, Total: 5500, PaymentMethod: "naqd", SoldAt: time.Date(2026, 11, 27, 19, 30, 0, 0, time.UTC)},
		{ID: "f", Login: "begona", Barcode: "p1", Quantity: 1, SoldAt: d(9)},
		{ID: "g", Login: "baraka", Barcode: "yomon kod", Quantity: 1, SoldAt: d(9)},
	}
	agg, skipped := aggregateSales(sales, stores, tz)
	if skipped != 2 || len(agg) != 3 {
		t.Fatalf("yig'indilar: %d, o'tkazildi %d", len(agg), skipped)
	}
	cash := agg[saleKey{Store: "s1", Barcode: "p1", Payment: "naqd", Day: "2026-11-27"}]
	if cash == nil || cash.Quantity != 4 || cash.Total != 2100000 || cash.Price != 550000 || cash.Name != "Non yangi" || cash.ID != "a" {
		t.Fatalf("naqd yig'indi: %+v", cash)
	}
	card := agg[saleKey{Store: "s1", Barcode: "p1", Payment: "plastik", Day: "2026-11-27"}]
	if card == nil || card.Total != 550000 || card.Name != "p1" {
		t.Fatalf("plastik (total bo'sh — narx x miqdor): %+v", card)
	}
	if agg[saleKey{Store: "s1", Barcode: "p1", Payment: "naqd", Day: "2026-11-28"}] == nil {
		t.Fatal("ertangi kun alohida bo'lishi kerak")
	}
}

func TestPharmPayment(t *testing.T) {
	if pharmPayment("plastik") != "plastik" || pharmPayment("cash") != "naqd" || pharmPayment("") != "naqd" {
		t.Fatal("to'lov turi")
	}
}
