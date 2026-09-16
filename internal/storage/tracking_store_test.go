package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/tracking"
)

func TestRouteRepoMemory(t *testing.T) {
	runRouteRepoContract(t, NewMemoryRouteRepo())
}

// TestRouteRepoPostgres — HAQIQIY Postgres ustida, vaqtinchalik sxemada.
func TestRouteRepoPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("CI'da TEST_DATABASE_URL BO'LISHI SHART")
		}
		t.Skip("TEST_DATABASE_URL berilmagan — Postgres testi o'tkazib yuborildi")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer root.Close()
	schema := fmt.Sprintf("routes_it_%d", time.Now().UnixNano())
	if _, err := root.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("sxema: %v", err)
	}
	defer func() {
		if _, err := root.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("sxema o'chirilmadi: %v", err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	sqlBytes, err := migrationFS.ReadFile("migrations/0053_order_routes.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("migratsiya: %v", err)
	}
	runRouteRepoContract(t, NewPgRouteRepo(pool))
}

func runRouteRepoContract(t *testing.T, repo tracking.Repository) {
	t.Helper()
	ctx := context.Background()

	if _, err := repo.GetRoute(ctx, "o1"); !errors.Is(err, tracking.ErrNotFound) {
		t.Fatalf("saqlanmagan yo'l: ErrNotFound kutilgan, keldi %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	want := &tracking.Route{
		OrderID:     "o1",
		Origin:      tracking.Point{Lat: 41.0001, Lng: 71.2301},
		Destination: tracking.Point{Lat: 41.0202, Lng: 71.2503},
		Points: []tracking.Point{
			{Lat: 41.0001, Lng: 71.2301}, {Lat: 41.01, Lng: 71.24}, {Lat: 41.0202, Lng: 71.2503},
		},
		DistanceMeters: 2400, DurationSeconds: 540, CreatedAt: now,
	}
	if err := repo.SaveRoute(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetRoute(ctx, "o1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin != want.Origin || got.Destination != want.Destination ||
		got.DistanceMeters != 2400 || got.DurationSeconds != 540 ||
		len(got.Points) != 3 || got.Points[1] != want.Points[1] || !got.CreatedAt.Equal(now) {
		t.Fatalf("saqlangan yo'l mos emas: %+v", got)
	}

	// Birinchi yozuv qoladi — tarix o'zgarmaydi.
	if err := repo.SaveRoute(ctx, &tracking.Route{OrderID: "o1", DistanceMeters: 1, DurationSeconds: 1}); err != nil {
		t.Fatalf("takroriy saqlash xato bermasligi kerak: %v", err)
	}
	if again, _ := repo.GetRoute(ctx, "o1"); again.DistanceMeters != 2400 || len(again.Points) != 3 {
		t.Fatalf("takroriy saqlash yo'lni o'zgartirdi: %+v", again)
	}

	for name, bad := range map[string]*tracking.Route{
		"order_id bo'sh": {DistanceMeters: 1},
		"nuqtalar ko'p":  {OrderID: "o2", Points: make([]tracking.Point, tracking.MaxRoutePoints+1)},
		"manfiy masofa":  {OrderID: "o3", DistanceMeters: -1},
	} {
		if err := repo.SaveRoute(ctx, bad); err == nil {
			t.Errorf("%s: xato kutilgan", name)
		}
	}
}
