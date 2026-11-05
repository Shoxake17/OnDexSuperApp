package qurtorders_test

import (
	"context"
	"testing"
	"time"

	"chustapp/internal/qurtorders"
)

func TestExpireNewCancelsUnansweredAndReleasesStock(t *testing.T) {
	f := newFixture(t, 10, 10)
	ctx := context.Background()
	start := time.Date(2026, 11, 2, 10, 0, 0, 0, qurtorders.Tashkent)
	now := start
	f.svc.Now = func() time.Time { return now }

	ignored, _ := f.svc.Create(ctx, f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 3}))
	accepted, _ := f.svc.Create(ctx, f.input(qurtorders.Line{ProductID: f.a.ID, Qty: 2}))
	if _, err := f.svc.Transition(ctx, accepted[0].ID, qurtorders.StatusConfirmed, qurtorders.ActorStore, "s", "", nil); err != nil {
		t.Fatal(err)
	}
	// Tasdiqlangan buyurtma bandni allaqachon qo'yib yuborgan (POS o'zi kamaytiradi): faqat 3 ta band.
	if got := f.left(t, f.a.ID); got != 7 {
		t.Fatalf("band: %d", got)
	}

	// Muddat o'tmagan — tegilmaydi.
	now = start.Add(23 * time.Hour)
	if n, err := f.svc.ExpireNew(ctx, 24*time.Hour); err != nil || n != 0 {
		t.Fatalf("muddatdan oldin: n=%d err=%v", n, err)
	}

	now = start.Add(25 * time.Hour)
	f.events = nil
	n, err := f.svc.ExpireNew(ctx, 24*time.Hour)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got, _ := f.svc.Repo.Get(ctx, ignored[0].ID)
	if got.Status != qurtorders.StatusCancelled || got.RejectReason != qurtorders.ExpireReason {
		t.Fatalf("holat=%s sabab=%q", got.Status, got.RejectReason)
	}
	if last := got.History[len(got.History)-1]; last.By != qurtorders.ActorSystem {
		t.Fatalf("tarixda aktor: %s", last.By)
	}
	if got := f.left(t, f.a.ID); got != 10 {
		t.Fatalf("band qaytishi kerak: %d", got)
	}
	if len(f.events) != 1 || f.events[0] != "cancelled" {
		t.Fatalf("hodisa: %v", f.events)
	}
	// Tasdiqlangan buyurtmaga tegilmagan.
	kept, _ := f.svc.Repo.Get(ctx, accepted[0].ID)
	if kept.Status != qurtorders.StatusConfirmed {
		t.Fatalf("tasdiqlangan buyurtma o'zgardi: %s", kept.Status)
	}
	// Qayta chaqirish — hech narsa qilmaydi (band ikki marta qaytmaydi).
	if n, _ := f.svc.ExpireNew(ctx, 24*time.Hour); n != 0 {
		t.Fatalf("takroriy: %d", n)
	}
	if got := f.left(t, f.a.ID); got != 10 {
		t.Fatalf("takroriy chaqiruv bandni buzdi: %d", got)
	}
}

func TestSystemCannotCancelAfterStoreAccepted(t *testing.T) {
	if err := qurtorders.ValidateTransition(qurtorders.StatusConfirmed, qurtorders.StatusCancelled, qurtorders.ActorSystem); err == nil {
		t.Fatal("tizim faqat `new` ni bekor qila oladi")
	}
}
