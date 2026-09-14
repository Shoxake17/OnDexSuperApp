package storage

import (
	"context"
	"testing"
	"time"

	"chustapp/internal/orders"
)

func saveOrders(t *testing.T, repo *MemoryOrderRepo, list []orders.Order) {
	t.Helper()
	for i := range list {
		if err := repo.Save(context.Background(), &list[i]); err != nil {
			t.Fatal(err)
		}
	}
}

// Restoranga ko'rinmagan buyurtma (to'lanmagan karta) statistikaga ham
// kirmasligi SHART — aks holda tushum panel ko'rsatmagan pul bilan
// shishardi.
func TestMemoryStatRowsVisibilityRangeAndItems(t *testing.T) {
	repo := NewMemoryOrderRepo()
	base := time.Date(2026, 9, 10, 7, 0, 0, 0, time.UTC)
	items := []orders.Item{{ProductID: "p1", Name: "Osh", Qty: 1, PriceTiyin: 3_000_000}}

	saveOrders(t, repo, []orders.Order{
		{ID: "cash-done", RestaurantID: "r1", CustomerID: "c1", Status: orders.StatusDelivered,
			TotalTiyin: 3_000_000, CreatedAt: base, Items: items,
			History: []orders.StatusChange{
				{To: orders.StatusAccepted, At: base.Add(time.Minute)},
				{To: orders.StatusReady, At: base.Add(20 * time.Minute)},
			}},
		{ID: "card-unpaid", RestaurantID: "r1", CustomerID: "c2", Status: orders.StatusCreated,
			PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentAwaiting, CreatedAt: base, Items: items},
		{ID: "card-held", RestaurantID: "r1", CustomerID: "c3", Status: orders.StatusPreparing,
			PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentHeld, CreatedAt: base, Items: items},
		{ID: "other-restaurant", RestaurantID: "r2", CustomerID: "c1", Status: orders.StatusDelivered, CreatedAt: base},
		{ID: "at-range-end", RestaurantID: "r1", CustomerID: "c1", Status: orders.StatusDelivered,
			CreatedAt: base.Add(24 * time.Hour)},
	})

	rows, err := repo.StatRows(context.Background(), "r1", base, base.Add(24*time.Hour), base, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("2 ta qator kutilgan edi (naqd + to'langan karta), keldi %d: %+v", len(rows), rows)
	}
	for _, r := range rows {
		switch r.CustomerID {
		case "c1":
			if len(r.Items) != 1 || r.AcceptedAt.IsZero() || r.ReadyAt.IsZero() {
				t.Fatalf("bajarilgan buyurtma to'liq emas: %+v", r)
			}
		case "c3":
			if len(r.Items) != 0 {
				t.Fatalf("jarayondagi buyurtma taomlari kerak emas: %+v", r)
			}
		default:
			t.Fatalf("kutilmagan qator: %+v", r)
		}
	}

	// Chegara: limit+1 dan ko'p qaytmaydi.
	rows, err = repo.StatRows(context.Background(), "r1", base, base.Add(48*time.Hour), base, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("limit=1 da 2 ta (limit+1) qator kutilgan edi, keldi %d", len(rows))
	}
}

func TestMemoryFirstOrderAt(t *testing.T) {
	repo := NewMemoryOrderRepo()
	base := time.Date(2026, 1, 1, 7, 0, 0, 0, time.UTC)

	saveOrders(t, repo, []orders.Order{
		// Hammasi c1 niki — faqat oxirgisi hisobga olinishi kerak.
		{ID: "cancelled", RestaurantID: "r1", CustomerID: "c1", Status: orders.StatusCancelled, CreatedAt: base},
		{ID: "unpaid", RestaurantID: "r1", CustomerID: "c1", Status: orders.StatusCreated,
			PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentFailed, CreatedAt: base.Add(time.Hour)},
		{ID: "other", RestaurantID: "r2", CustomerID: "c1", Status: orders.StatusDelivered, CreatedAt: base.Add(2 * time.Hour)},
		{ID: "real", RestaurantID: "r1", CustomerID: "c1", Status: orders.StatusDelivered, CreatedAt: base.Add(3 * time.Hour)},
		{ID: "later", RestaurantID: "r1", CustomerID: "c1", Status: orders.StatusDelivered, CreatedAt: base.Add(9 * time.Hour)},
		{ID: "c2", RestaurantID: "r1", CustomerID: "c2", Status: orders.StatusDelivered, CreatedAt: base},
	})

	got, err := repo.FirstOrderAt(context.Background(), "r1", []string{"c1", "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("faqat so'ralgan va topilgan mijoz qaytishi kerak: %v", got)
	}
	if !got["c1"].Equal(base.Add(3 * time.Hour)) {
		t.Fatalf("birinchi haqiqiy buyurtma noto'g'ri: %v", got["c1"])
	}
}
