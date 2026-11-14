package storage

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

// Kontekst bekor bo'lganda ulanish YOPILMAYDI: Postgres'ga cancel boradi va
// aynan o'sha ulanish (bir xil backend PID) keyingi so'rovga xizmat qiladi.
// Avval (pgx standarti) bu holatda ulanish uzilib, qayta SCRAM bilan ochilardi.
func TestPoolCancelKeepsConnection(t *testing.T) {
	realmTestPool(t) // TEST_DATABASE_URL yo'q bo'lsa skip, CI'da majburiy
	ctx := context.Background()
	pool, err := NewPool(ctx, os.Getenv("TEST_DATABASE_URL")+poolOpts("pool_max_conns=1&pool_min_conns=1"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	var pid1 int
	if err := pool.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid1); err != nil {
		t.Fatal(err)
	}
	before := pool.Stat().NewConnsCount()

	qctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	_, err = pool.Exec(qctx, `SELECT pg_sleep(5)`)
	if err == nil {
		t.Fatal("so'rov bekor qilinishi kerak edi")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("bekor qilish xatosi: %v", err)
	}

	var pid2 int
	if err := pool.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid2); err != nil {
		t.Fatalf("bekor qilishdan keyin so'rov: %v", err)
	}
	if pid2 != pid1 || pool.Stat().NewConnsCount() != before {
		t.Fatalf("ulanish qayta ochildi: pid %d -> %d, yangi ulanishlar %d -> %d",
			pid1, pid2, before, pool.Stat().NewConnsCount())
	}
}

// poolOpts — DSN'ga qo'shimcha parametr (? yoki & bilan).
func poolOpts(q string) string {
	if strings.Contains(os.Getenv("TEST_DATABASE_URL"), "?") {
		return "&" + q
	}
	return "?" + q
}
