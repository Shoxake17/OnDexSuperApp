package storage

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Bir nechta API nusxasi (yoki rolling deploy) BIR VAQTDA ishga tushsa,
// migratsiyalar bir marta qo'llanadi va hech bir nusxa yiqilmaydi
// (pg_advisory_xact_lock — PgBouncer transaction rejimida ham ishlaydi).
func TestMigrateConcurrentInstances(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL berilmagan")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	name := fmt.Sprintf("migconc_%d", time.Now().UnixNano())
	if _, err := root.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = root.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)") })
	u, _ := url.Parse(dsn)
	u.Path = "/" + name

	const n = 4
	var wg sync.WaitGroup
	errs := make([]error, n)
	pools := make([]*pgxpool.Pool, n)
	for i := range n {
		p, err := pgxpool.New(ctx, u.String())
		if err != nil {
			t.Fatal(err)
		}
		pools[i] = p
	}
	defer func() {
		for _, p := range pools {
			p.Close()
		}
	}()
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = Migrate(ctx, pools[i])
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("%d-nusxa migratsiyada yiqildi: %v", i, err)
		}
	}
	var applied, files int
	if err := pools[0].QueryRow(ctx, `SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	entries, _ := migrationFS.ReadDir("migrations")
	files = len(entries)
	if applied != files {
		t.Fatalf("qo'llangan %d, fayllar %d", applied, files)
	}
}
