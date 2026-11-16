package orders

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// RS-2: tayyor stol buyurtmasini bitta ofitsiant oladi — parallel 10 ta
// "Olaman"dan faqat bittasi o'tadi (versiya qulfi), qolganlari ErrWaiterTaken.
func TestClaimForWaiterParallelOnlyOneWins(t *testing.T) {
	svc, repo, _ := newDispatchFixture(t, TypeDineIn)
	ctx := context.Background()
	o := repo.data["o1"]
	o.RestaurantID = "r1"
	repo.data["o1"] = o

	if _, err := svc.ClaimForWaiter(ctx, "o1", "r1", "w0", "Ali"); !errors.Is(err, ErrNotClaimable) {
		t.Fatalf("tayyor emas: %v", err)
	}
	toReady(t, svc)
	if _, err := svc.ClaimForWaiter(ctx, "o1", "boshqa", "w0", "Ali"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("begona restoran: %v", err)
	}

	var won int64
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.ClaimForWaiter(ctx, "o1", "r1", fmt.Sprintf("w%d", i), "W")
			switch {
			case err == nil:
				atomic.AddInt64(&won, 1)
			case errors.Is(err, ErrWaiterTaken), errors.Is(err, ErrConflict):
			default:
				t.Errorf("kutilmagan: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if won != 1 {
		t.Fatalf("%d ofitsiant oldi", won)
	}
	got, _ := svc.Get(ctx, "o1")
	if got.WaiterID == "" {
		t.Fatal("ofitsiant yozilmadi")
	}
	// G'olib qayta bossa — idempotent.
	if _, err := svc.ClaimForWaiter(ctx, "o1", "r1", got.WaiterID, "W"); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
}

func TestClaimForWaiterRejectsDelivery(t *testing.T) {
	svc, repo, _ := newDispatchFixture(t, TypeDelivery)
	o := repo.data["o1"]
	o.RestaurantID = "r1"
	repo.data["o1"] = o
	toReady(t, svc)
	if _, err := svc.ClaimForWaiter(context.Background(), "o1", "r1", "w1", "A"); !errors.Is(err, ErrNotClaimable) {
		t.Fatalf("yetkazish: %v", err)
	}
}
