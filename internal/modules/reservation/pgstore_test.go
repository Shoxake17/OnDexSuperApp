package reservation_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/modules/reservation"
	"chustapp/internal/storage"
)

func pgPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("CI'da TEST_DATABASE_URL BO'LISHI SHART")
		}
		t.Skip("TEST_DATABASE_URL berilmagan — Postgres testi o'tkazib yuborildi")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := storage.Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return pool
}

// Bazadagi EXCLUDE: 30 ta parallel INSERT bitta stol/vaqtga — faqat bittasi
// o'tadi; bekor qilingan bron joyni bo'shatadi; shartli Update poygasi.
func TestPGStoreExcludeAndConditionalUpdate(t *testing.T) {
	pool := pgPool(t)
	ctx := context.Background()
	st := reservation.NewPGStore(pool)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	table := "tbl-" + suffix
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM reservation.reservations WHERE table_id = $1`, table)
		_, _ = pool.Exec(context.Background(), `DELETE FROM reservation.settings WHERE restaurant_id = $1`, "rest-"+suffix)
	})
	start := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Hour)
	mk := func(i int, from time.Time) *reservation.Reservation {
		now := time.Now().UTC()
		return &reservation.Reservation{ID: fmt.Sprintf("rsv-%s-%d", suffix, i), RestaurantID: "rest-" + suffix,
			TableID: table, CustomerID: fmt.Sprintf("c%d", i), PartySize: 2, StartsAt: from,
			EndsAt: from.Add(2 * time.Hour), Status: reservation.StatusConfirmed, CreatedAt: now, UpdatedAt: now}
	}

	var ok, taken int64
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := st.Insert(ctx, mk(i, start.Add(time.Duration(i%3)*30*time.Minute)))
			switch {
			case err == nil:
				atomic.AddInt64(&ok, 1)
			case errors.Is(err, reservation.ErrSlotTaken):
				atomic.AddInt64(&taken, 1)
			default:
				t.Errorf("kutilmagan: %v", err)
			}
		}(i)
	}
	wg.Wait()
	if ok != 1 || taken != 29 {
		t.Fatalf("ok=%d taken=%d", ok, taken)
	}

	// Kesishmaydigan vaqt — o'tadi; yarim-ochiq oraliq: tugash = keyingisining boshi.
	if err := st.Insert(ctx, mk(100, start.Add(4*time.Hour))); err != nil {
		t.Fatal(err)
	}

	// Kim o'tganini topib, bekor qilamiz — joy bo'shaydi.
	list, err := st.ListByRestaurant(ctx, "rest-"+suffix, start.Add(-time.Hour), start.Add(3*time.Hour))
	if err != nil || len(list) != 1 {
		t.Fatalf("ro'yxat: %d %v", len(list), err)
	}
	winner := list[0]
	winner.Status = reservation.StatusCancelled
	if err := st.Update(ctx, winner, reservation.StatusConfirmed); err != nil {
		t.Fatal(err)
	}
	if err := st.Update(ctx, winner, reservation.StatusConfirmed); !errors.Is(err, reservation.ErrConflict) {
		t.Fatalf("shartli update: %v", err)
	}
	if err := st.Insert(ctx, mk(200, start)); err != nil {
		t.Fatalf("bo'shagan joy: %v", err)
	}
	// Bekor qilinganni qayta faollashtirish band joyga — ErrSlotTaken.
	winner.Status = reservation.StatusConfirmed
	if err := st.Update(ctx, winner, reservation.StatusCancelled); !errors.Is(err, reservation.ErrSlotTaken) {
		t.Fatalf("qayta faollashtirish: %v", err)
	}

	// Sozlamalar (JSONB zona zakladlari) va ListDue.
	if err := st.PutSettings(ctx, &reservation.Settings{RestaurantID: "rest-" + suffix, Enabled: true,
		DepositPerPersonTiyin: 5_000_000, ZoneDeposits: map[string]int64{"VIP": 10_000_000},
		SlotMinutes: 120, MinAdvanceMinutes: 60, MaxDaysAhead: 30, UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	s, err := st.GetSettings(ctx, "rest-"+suffix)
	if err != nil || s.ZoneDeposits["VIP"] != 10_000_000 || !s.Enabled {
		t.Fatalf("sozlama: %+v %v", s, err)
	}
	due, err := st.ListDue(ctx, start.Add(20*time.Minute), 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range due {
		found = found || r.ID == fmt.Sprintf("rsv-%s-200", suffix)
	}
	if !found {
		t.Fatal("kelmagan bron ListDue'da yo'q")
	}
}
