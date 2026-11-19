package payments

import (
	"context"
	"testing"
	"time"
)

// Audit 480: to'lov yozuvi yo'q yoki faqat yopilgan urinishlari bor
// buyurtma bekor qilinadi; jonli (pending/held/paid) urinishi bor
// buyurtmaga TEGILMAYDI.
func TestCloseAbandoned(t *testing.T) {
	ord := &fakeOrders{amount: 10000}
	svc, repo := newTestService(t, &fakeProvider{}, ord, Options{})
	ctx := context.Background()
	now := time.Now()
	mk := func(id, order string, st Status) {
		if err := repo.Create(ctx, &Payment{ID: id, OrderID: order, Status: st, CreatedAt: now, UpdatedAt: now, ExpiresAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	mk("p1", "o-failed", StatusFailed)
	mk("p2", "o-failed", StatusCanceled)
	mk("p3", "o-pending", StatusPending)
	mk("p4", "o-paid", StatusFailed)
	mk("p5", "o-paid", StatusHeld)

	n, err := svc.CloseAbandoned(ctx, []string{"o-none", "o-failed", "o-pending", "o-paid"})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || ord.failed != 2 {
		t.Fatalf("yopildi=%d, OnPaymentFailed=%d; kutilgan 2/2 (o-none, o-failed)", n, ord.failed)
	}
}
