package storage

import (
	"context"
	"testing"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/stats"
)

var historyBase = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func seedHistory(t *testing.T) *MemoryOrderRepo {
	t.Helper()
	repo := NewMemoryOrderRepo()
	base := historyBase
	list := []orders.Order{
		{ID: "a1", RestaurantID: "r1", Status: orders.StatusServed, TotalTiyin: 30_000, CreatedAt: base},
		{ID: "a2", RestaurantID: "r1", Status: orders.StatusCancelled, TotalTiyin: 5_000, CreatedAt: base.Add(time.Hour)},
		// a3 va a4 — bir xil vaqt (nanosekundi farqli): tartib id bo'yicha.
		{ID: "a3", RestaurantID: "r1", Status: orders.StatusDelivered, TotalTiyin: 50_000, CreatedAt: base.Add(2 * time.Hour)},
		{ID: "a4", RestaurantID: "r1", Status: orders.StatusReady, TotalTiyin: 14_000, CreatedAt: base.Add(2*time.Hour + 300)},
		{ID: "a5", RestaurantID: "r1", Status: orders.StatusCreated, TotalTiyin: 9_000, CreatedAt: base.Add(3 * time.Hour)},
		// Ko'rinmasligi kerak: to'lanmagan karta va begona restoran.
		{ID: "x1", RestaurantID: "r1", Status: orders.StatusCreated, TotalTiyin: 70_000, CreatedAt: base.Add(4 * time.Hour),
			PaymentMethod: orders.PaymentCard, PaymentState: orders.PaymentAwaiting},
		{ID: "x2", RestaurantID: "r2", Status: orders.StatusDelivered, TotalTiyin: 80_000, CreatedAt: base.Add(5 * time.Hour)},
	}
	for i := range list {
		if err := repo.Save(context.Background(), &list[i]); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func collectHistory(t *testing.T, repo *MemoryOrderRepo, q stats.HistoryQuery, pageSize int) []string {
	t.Helper()
	var got []string
	for page := 0; page < 20; page++ {
		list, err := repo.OrderHistory(context.Background(), "r1", q, pageSize)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range list {
			got = append(got, o.ID)
		}
		if len(list) < pageSize {
			return got
		}
		// Kursor klientga satr sifatida borib qaytadi — xuddi shunday sinaymiz.
		c, err := stats.ParseCursor(stats.CursorOf(list[len(list)-1]).Encode())
		if err != nil {
			t.Fatal(err)
		}
		q.After = c
	}
	t.Fatal("sahifalash tugamadi")
	return nil
}

func sameIDs(a []string, b ...string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestMemoryOrderHistoryPagesWithoutGapsOrDuplicates(t *testing.T) {
	repo := seedHistory(t)
	got := collectHistory(t, repo, stats.HistoryQuery{Status: stats.HistoryAll}, 2)
	if !sameIDs(got, "a5", "a4", "a3", "a2", "a1") {
		t.Fatalf("tartib: %v", got)
	}
}

func TestMemoryOrderHistoryFilters(t *testing.T) {
	repo := seedHistory(t)
	ctx := context.Background()
	for status, want := range map[stats.HistoryStatus]int{
		stats.HistoryAll: 5, stats.HistoryCompleted: 2, stats.HistoryCancelled: 1, stats.HistoryInProgress: 2,
	} {
		list, err := repo.OrderHistory(ctx, "r1", stats.HistoryQuery{Status: status}, 50)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != want {
			t.Errorf("%s: %d ta, kutilgan %d", status, len(list), want)
		}
	}
}

func TestMemoryOrderHistoryPeriod(t *testing.T) {
	repo := seedHistory(t)
	// [1:30, 2:30) — faqat a3 va a4; chegaradagi a2 (1:00) va a5 (3:00) kirmaydi.
	p := stats.Period{From: historyBase.Add(90 * time.Minute), To: historyBase.Add(150 * time.Minute)}

	got := collectHistory(t, repo, stats.HistoryQuery{Status: stats.HistoryAll, Period: p}, 1)
	if !sameIDs(got, "a4", "a3") {
		t.Fatalf("davr: %v", got)
	}
	got = collectHistory(t, repo, stats.HistoryQuery{Status: stats.HistoryCompleted, Period: p}, 5)
	if !sameIDs(got, "a3") {
		t.Fatalf("davr + holat: %v", got)
	}

	l, err := repo.Lifetime(context.Background(), "r1", p)
	if err != nil {
		t.Fatal(err)
	}
	if l.Orders != 2 || l.Completed != 1 || l.InProgress != 1 || l.RevenueTiyin != 50_000 {
		t.Fatalf("davr xulosasi: %+v", l)
	}

	// Chegaraning o'zi: From kiradi, To kirmaydi.
	exact := stats.Period{From: historyBase, To: historyBase.Add(time.Hour)}
	if got := collectHistory(t, repo, stats.HistoryQuery{Period: exact}, 10); !sameIDs(got, "a1") {
		t.Fatalf("chegara: %v", got)
	}
}

func TestMemoryLifetime(t *testing.T) {
	repo := seedHistory(t)
	l, err := repo.Lifetime(context.Background(), "r1", stats.Period{})
	if err != nil {
		t.Fatal(err)
	}
	if l.Orders != 5 || l.Completed != 2 || l.Cancelled != 1 || l.InProgress != 2 {
		t.Fatalf("sonlar (to'lanmagan yoki begona buyurtma kirib qolgan?): %+v", l)
	}
	if l.RevenueTiyin != 80_000 || l.InProgressTiyin != 23_000 {
		t.Fatalf("summalar: %+v", l)
	}
	if l.AvgOrderTiyin == nil || *l.AvgOrderTiyin != 40_000 {
		t.Fatalf("o'rtacha: %v", l.AvgOrderTiyin)
	}
	if l.FirstOrderAt == nil || l.FirstOrderAt.Hour() != 12 || l.LastOrderAt == nil || l.LastOrderAt.Hour() != 15 {
		t.Fatalf("vaqtlar: %v %v", l.FirstOrderAt, l.LastOrderAt)
	}

	empty, err := repo.Lifetime(context.Background(), "yoq", stats.Period{})
	if err != nil {
		t.Fatal(err)
	}
	if empty.Orders != 0 || empty.AvgOrderTiyin != nil || empty.FirstOrderAt != nil {
		t.Fatalf("bo'sh restoran: %+v", empty)
	}
}
