package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/orders"
)

// Band joy o'chirilmaydi; yakunlangan buyurtmasi bor joy — o'chiriladi.
func TestTablesDeleteBlockedWhileOccupied(t *testing.T) {
	h, repo, svc, jwt := tablesServer(t)
	ctx := context.Background()
	busy, _ := svc.Create(ctx, testRestA, "1")
	done, _ := svc.Create(ctx, testRestA, "2")
	for _, o := range []orders.Order{
		{ID: "o1", RestaurantID: testRestA, Type: orders.TypeDineIn, TableID: busy.ID,
			Status: orders.StatusAccepted, CreatedAt: time.Now()},
		{ID: "o2", RestaurantID: testRestA, Type: orders.TypeDineIn, TableID: done.ID,
			Status: orders.StatusServed, CreatedAt: time.Now()},
	} {
		if err := repo.Save(ctx, &o); err != nil {
			t.Fatal(err)
		}
	}

	if w := do(t, h, "DELETE", "/tables/"+busy.ID, jwt["a"], ""); w.Code != http.StatusConflict {
		t.Fatalf("band joy: %d — %s", w.Code, w.Body.String())
	}
	if _, err := svc.Get(ctx, busy.ID); err != nil {
		t.Fatal("band joy o'chirib yuborildi")
	}
	if w := do(t, h, "DELETE", "/tables/"+done.ID, jwt["a"], ""); w.Code != http.StatusOK {
		t.Fatalf("bo'sh joy: %d — %s", w.Code, w.Body.String())
	}
}
