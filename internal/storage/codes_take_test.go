package storage

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"chustapp/internal/users"
)

// AUD-14: CodeTaker — kod faqat xeshi mos bo'lsa va FAQAT BIR MARTA olinadi
// (parallel so'rovlarda ham). Uchala ombor bir xil shartnoma bo'yicha.
func codeTakeContract(t *testing.T, store interface {
	users.CodeStore
	users.CodeTaker
}) {
	t.Helper()
	ctx := context.Background()
	const phone = "+998907770011"
	now := time.Now()
	if err := store.Save(ctx, &users.Code{Target: phone, CodeHash: "h1", ExpiresAt: now.Add(time.Minute), CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.Take(ctx, phone, "boshqa"); ok || err != nil {
		t.Fatalf("XAVFSIZLIK: noto'g'ri xesh bilan olindi: %v %v", ok, err)
	}
	var won atomic.Int32
	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, err := store.Take(ctx, phone, "h1"); ok && err == nil {
				won.Add(1)
			}
		}()
	}
	wg.Wait()
	if n := won.Load(); n != 1 {
		t.Fatalf("XAVFSIZLIK: kod %d marta olindi", n)
	}
	if _, err := store.Get(ctx, phone); err == nil {
		t.Fatal("kod olingandan keyin qoldi")
	}
	// Yo'q kod uchun urinish hisoblagichi kalit YARATMAYDI.
	if n, err := store.IncrementAttempts(ctx, phone); n != 0 || err != nil {
		t.Fatalf("yo'q kod: %d %v", n, err)
	}
	if _, err := store.Get(ctx, phone); err == nil {
		t.Fatal("IncrementAttempts yetim yozuv yaratdi")
	}
}

func TestMemoryCodeTake(t *testing.T) { codeTakeContract(t, NewMemoryCodeStore()) }

func TestPgCodeTake(t *testing.T) {
	pool := realmTestPool(t)
	_, _ = pool.Exec(context.Background(), `DELETE FROM phone_codes WHERE phone = '+998907770011'`)
	codeTakeContract(t, NewPgCodeStore(pool))
}

func TestRedisCodeTake(t *testing.T) {
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR berilmagan — Redis testi o'tkazib yuborildi")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = rdb.Close() })
	_ = rdb.Del(context.Background(), codeKey("+998907770011")).Err()
	codeTakeContract(t, NewRedisCodeStore(rdb))
}
