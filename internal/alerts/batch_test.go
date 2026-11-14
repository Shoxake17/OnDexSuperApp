package alerts_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"chustapp/internal/alerts"
	"chustapp/internal/storage"
)

// slowStore — har yozish 20 ms (yuk ostidagi baza) va paketlar sanaladi.
type slowStore struct {
	*storage.MemoryAlertStore
	batches, rows atomic.Int64
}

func (s *slowStore) InsertBatch(ctx context.Context, ns []*alerts.Notification) ([]bool, error) {
	time.Sleep(20 * time.Millisecond)
	s.batches.Add(1)
	s.rows.Add(int64(len(ns)))
	return s.MemoryAlertStore.InsertBatch(ctx, ns)
}

// Bitta restoranga bir vaqtda 500 hodisa: avval har biri navbat bilan
// alohida yozilardi (500 x 20 ms = 10 s). Endi paketlanadi — hammasi
// yoziladi, tez tugaydi va jonli kanal tartibi buzilmaydi.
func TestPublishBurstIsBatchedAndOrdered(t *testing.T) {
	store := &slowStore{MemoryAlertStore: storage.NewMemoryAlertStore()}
	hub := &fakeHub{}
	var id atomic.Int64
	svc := alerts.NewService(store, hub, func() string { return fmt.Sprintf("n%04d", id.Add(1)) })

	const n = 500
	start := time.Now()
	var wg sync.WaitGroup
	var failed atomic.Int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if got, err := svc.Publish(ctx, "r1", alerts.Input{Kind: "k", Category: alerts.CategoryNew,
				Title: fmt.Sprintf("t%d", i), DedupeKey: fmt.Sprintf("o%d", i)}); err != nil || got == nil {
				failed.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if f := failed.Load(); f != 0 {
		t.Fatalf("%d ta bildirishnoma yozilmadi", f)
	}
	if b := store.batches.Load(); b >= n/5 {
		t.Fatalf("paketlanmadi: %d so'rov %d yozuv uchun", b, n)
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Fatalf("juda sekin: %v", el)
	}
	var last int64
	events := hub.snapshot()
	for _, s := range events {
		seq := s.event["notification"].(alerts.View).Seq
		if seq <= last {
			t.Fatalf("jonli kanalga tartib buzildi: %d keyin %d", last, seq)
		}
		last = seq
	}
	if len(events) != n || last != n {
		t.Fatalf("hodisalar: %d, oxirgi seq %d", len(events), last)
	}
	if u := events[len(events)-1].event["unread"]; u != n {
		t.Fatalf("oxirgi unread: %v", u)
	}
}

// Kutishdan voz kechgan chaqiruvchi (kontekst bekor) navbatni to'xtatmaydi.
func TestPublishCanceledCallerDoesNotBlockQueue(t *testing.T) {
	store := &slowStore{MemoryAlertStore: storage.NewMemoryAlertStore()}
	svc := alerts.NewService(store, &fakeHub{}, func() string { return fmt.Sprintf("x%d", time.Now().UnixNano()) })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.Publish(ctx, "r1", alerts.Input{Kind: "k", Category: alerts.CategoryInfo, Title: "a"}); err == nil {
		t.Fatal("bekor qilingan kontekst xato qaytarishi kerak")
	}
	n, err := svc.Publish(context.Background(), "r1", alerts.Input{Kind: "k", Category: alerts.CategoryInfo, Title: "b"})
	if err != nil || n == nil {
		t.Fatalf("keyingi yozuv: %v %v", n, err)
	}
}
