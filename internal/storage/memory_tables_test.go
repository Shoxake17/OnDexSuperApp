package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/tables"
)

func TestMemoryTableRepoCreateManyIsAtomic(t *testing.T) {
	repo := NewMemoryTableRepo()
	ctx := context.Background()
	mk := func(id, label string, kind tables.Kind) *tables.Table {
		return &tables.Table{ID: id, RestaurantID: "r1", Zone: tables.DefaultZone, Kind: kind,
			Label: label, QRToken: "tok-" + id, Active: true}
	}
	if err := repo.Create(ctx, mk("a", "1", tables.KindTable)); err != nil {
		t.Fatal(err)
	}
	// Ro'yxat ICHIDA takror ("2" ikki marta) — hech biri yozilmasin.
	err := repo.CreateMany(ctx, []*tables.Table{mk("b", "2", tables.KindTable), mk("c", "2", tables.KindTable)})
	if !errors.Is(err, tables.ErrDuplicate) {
		t.Fatalf("ichki takror: %v", err)
	}
	// Bazadagi bilan takror.
	err = repo.CreateMany(ctx, []*tables.Table{mk("d", "3", tables.KindTable), mk("e", "1", tables.KindTable)})
	if !errors.Is(err, tables.ErrDuplicate) {
		t.Fatalf("mavjud bilan takror: %v", err)
	}
	list, _ := repo.ListByRestaurant(ctx, "r1")
	if len(list) != 1 {
		t.Fatalf("yiqilgan partiyadan qatorlar qoldi: %d", len(list))
	}
	// Boshqa tur — takror emas.
	if err := repo.CreateMany(ctx, []*tables.Table{mk("f", "1", tables.KindCabin)}); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryTableRepoUpdateKeepsTokenAndScanTime(t *testing.T) {
	repo := NewMemoryTableRepo()
	ctx := context.Background()
	orig := &tables.Table{ID: "a", RestaurantID: "r1", Label: "1", QRToken: "secret", Active: true,
		CreatedAt: time.Unix(100, 0)}
	if err := repo.Create(ctx, orig); err != nil {
		t.Fatal(err)
	}
	scanned := time.Unix(500, 0)
	if err := repo.TouchScanned(ctx, "a", scanned, time.Minute); err != nil {
		t.Fatal(err)
	}

	cp, _ := repo.GetByID(ctx, "a")
	cp.QRToken = "hijacked"
	cp.LastScannedAt = nil
	cp.CreatedAt = time.Unix(1, 0)
	cp.RestaurantID = "r2"
	c := 8
	cp.Capacity = &c
	cp.Kind = tables.KindLounge
	if err := repo.Update(ctx, cp); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(ctx, "a")
	if got.QRToken != "secret" || got.LastScannedAt == nil || !got.LastScannedAt.Equal(scanned) ||
		!got.CreatedAt.Equal(time.Unix(100, 0)) || got.RestaurantID != "r1" {
		t.Fatalf("o'zgarmas maydonlar o'zgardi: %+v", got)
	}
	if got.Kind != tables.KindLounge || *got.Capacity != 8 {
		t.Fatalf("tahrir yozilmadi: %+v", got)
	}

	// Qaytgan nusxani o'zgartirish omborga ta'sir qilmaydi.
	*got.Capacity = 999
	again, _ := repo.GetByID(ctx, "a")
	if *again.Capacity != 8 {
		t.Fatal("ombor ichki ko'rsatkichi tashqariga chiqib ketdi")
	}
}

func TestMemoryTableRepoTouchScannedThrottle(t *testing.T) {
	repo := NewMemoryTableRepo()
	ctx := context.Background()
	_ = repo.Create(ctx, &tables.Table{ID: "a", RestaurantID: "r1", Label: "1", QRToken: "t"})
	t0 := time.Unix(1000, 0)
	_ = repo.TouchScanned(ctx, "a", t0, time.Minute)
	_ = repo.TouchScanned(ctx, "a", t0.Add(30*time.Second), time.Minute)
	got, _ := repo.GetByID(ctx, "a")
	if !got.LastScannedAt.Equal(t0) {
		t.Fatalf("cheklov ishlamadi: %v", got.LastScannedAt)
	}
	_ = repo.TouchScanned(ctx, "a", t0.Add(time.Minute), time.Minute)
	got, _ = repo.GetByID(ctx, "a")
	if !got.LastScannedAt.Equal(t0.Add(time.Minute)) {
		t.Fatalf("daqiqadan keyin yangilanmadi: %v", got.LastScannedAt)
	}
	if err := repo.TouchScanned(ctx, "yoq", t0, time.Minute); err != nil {
		t.Fatalf("yo'q joy xato bermasligi kerak: %v", err)
	}
}

func TestMemoryDineInSnapshots(t *testing.T) {
	repo := NewMemoryOrderRepo()
	ctx := context.Background()
	base := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	items := []orders.Item{{Name: "Osh", Qty: 2}, {Name: "Choy", Qty: 3}}
	list := []orders.Order{
		{ID: "o1", OrderNumber: "N1", RestaurantID: "r1", Type: orders.TypeDineIn, TableID: "t1",
			Status: orders.StatusPreparing, Items: items, TotalTiyin: 500, CreatedAt: base},
		{ID: "o2", RestaurantID: "r1", Type: orders.TypeDineIn, TableID: "t1",
			Status: orders.StatusServed, Items: items, CreatedAt: base.Add(time.Hour)},
		// Kirmasligi kerak: yetkazish, begona restoran, stolsiz, to'lanmagan karta.
		{ID: "x1", RestaurantID: "r1", Type: orders.TypeDelivery, TableID: "t1",
			Status: orders.StatusPreparing, CreatedAt: base},
		{ID: "x2", RestaurantID: "r2", Type: orders.TypeDineIn, TableID: "t1",
			Status: orders.StatusPreparing, CreatedAt: base},
		{ID: "x3", RestaurantID: "r1", Type: orders.TypeDineIn, TableID: "",
			Status: orders.StatusPreparing, CreatedAt: base},
		{ID: "x4", RestaurantID: "r1", Type: orders.TypeDineIn, TableID: "t2", Status: orders.StatusCreated,
			CreatedAt: base.Add(2 * time.Hour), PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentAwaiting},
	}
	for i := range list {
		if err := repo.Save(ctx, &list[i]); err != nil {
			t.Fatal(err)
		}
	}

	active, err := repo.ActiveDineInOrders(ctx, "r1")
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != "o1" || active[0].Items != 5 || active[0].TableID != "t1" ||
		active[0].OrderNumber != "N1" {
		t.Fatalf("faol stol buyurtmalari: %+v", active)
	}

	latest, err := repo.LatestDineInOrders(ctx, "r1", []string{"t1", "t2"})
	if err != nil {
		t.Fatal(err)
	}
	if latest["t1"].ID != "o2" {
		t.Fatalf("t1 oxirgisi (yakunlangan ham hisob): %+v", latest["t1"])
	}
	if _, ok := latest["t2"]; ok {
		t.Fatal("to'lanmagan karta buyurtmasi oxirgi buyurtma deb olindi")
	}
	if empty, _ := repo.LatestDineInOrders(ctx, "r1", nil); len(empty) != 0 {
		t.Fatal("bo'sh ro'yxat")
	}
}
