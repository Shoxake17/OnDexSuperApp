package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"chustapp/internal/alerts"
	"chustapp/internal/notify"
)

// O'qilmaganlar sanog'i yuqoridan chegaralangan: belgi uchun aniq son
// kerak emas, cheksiz sanash esa har yangi buyurtmada qimmatlashardi
// (prod yuklama testi 2026-11-13). Chegaradan past sonlar AYNAN qaytadi.

func fillAlerts(t *testing.T, store interface {
	Insert(context.Context, *alerts.Notification) (bool, error)
}, rid string, from, to int) {
	t.Helper()
	ctx := context.Background()
	for i := from; i < to; i++ {
		id := fmt.Sprintf("%s-n%d", rid, i)
		ok, err := store.Insert(ctx, &alerts.Notification{ID: id, RestaurantID: rid, Kind: "test",
			Category: alerts.CategoryNew, Title: "t", DedupeKey: "id:" + id, CreatedAt: time.Now()})
		if err != nil || !ok {
			t.Fatalf("insert %d: ok=%v err=%v", i, ok, err)
		}
	}
}

func TestAlertUnreadCountCapped_Memory(t *testing.T) {
	s := NewMemoryAlertStore()
	fillAlerts(t, s, "r-small", 0, 3)
	fillAlerts(t, s, "r-big", 0, alerts.MaxUnreadCount+5)
	if n, _ := s.UnreadCount(context.Background(), "r-small"); n != 3 {
		t.Fatalf("kichik sanoq aniq bo'lishi kerak: %d", n)
	}
	if n, _ := s.UnreadCount(context.Background(), "r-big"); n != alerts.MaxUnreadCount {
		t.Fatalf("katta sanoq %d ga chegaralanishi kerak: %d", alerts.MaxUnreadCount, n)
	}
}

func TestAlertUnreadCountCapped_Postgres(t *testing.T) {
	pool := realmTestPool(t)
	ctx := context.Background()
	rid := fmt.Sprintf("unreadcap-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM restaurant_notifications WHERE restaurant_id = $1`, rid)
	})
	s := NewPgAlertStore(pool)
	fillAlerts(t, s, rid, 0, 4)
	if n, err := s.UnreadCount(ctx, rid); err != nil || n != 4 {
		t.Fatalf("aniq sanoq: n=%d err=%v", n, err)
	}
	if _, err := s.MarkRead(ctx, rid, rid+"-n0", time.Now()); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.UnreadCount(ctx, rid); n != 3 {
		t.Fatalf("o'qilgani sanalmasligi kerak: %d", n)
	}
	fillAlerts(t, s, rid, 4, alerts.MaxUnreadCount+10)
	if n, _ := s.UnreadCount(ctx, rid); n != alerts.MaxUnreadCount {
		t.Fatalf("chegaralanmadi: %d", n)
	}
}

func TestNotificationUnreadCountCapped_Memory(t *testing.T) {
	s := NewMemoryNotificationStore()
	ctx := context.Background()
	for i := 0; i < notify.MaxUnreadCount+5; i++ {
		if err := s.Save(ctx, &notify.Notification{ID: fmt.Sprintf("n%d", i), UserID: "u-big",
			Module: "food", Kind: "test", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Save(ctx, &notify.Notification{ID: "s1", UserID: "u-small",
		Module: "food", Kind: "test", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.UnreadCount(ctx, "u-small"); n != 1 {
		t.Fatalf("kichik sanoq aniq bo'lishi kerak: %d", n)
	}
	if n, _ := s.UnreadCount(ctx, "u-big"); n != notify.MaxUnreadCount {
		t.Fatalf("katta sanoq chegaralanmadi: %d", n)
	}
}
