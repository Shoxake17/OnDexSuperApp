package storage

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/orders"
	"chustapp/internal/stats"
)

// Platforma agregatsiyasi (`stats.PlatformSource`): xotira va Postgres
// implementatsiyalari BIR XIL natija berishi shart — ikkalasi bir xil
// buyurtmalar ustida solishtiriladi.

type platformFixtureRow struct {
	id, restaurant string
	status         orders.Status
	total          int64
	createdAt      time.Time
	method         orders.PaymentMethod
	state          orders.PaymentState
}

func platformFixture() (rows []platformFixtureRow, q stats.PlatformQuery) {
	now := time.Now().In(stats.Location)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, stats.Location)
	at := func(daysAgo, hour int) time.Time {
		return midnight.AddDate(0, 0, -daysAgo).Add(time.Duration(hour) * time.Hour)
	}

	rows = []platformFixtureRow{
		{"a1", "rest-a", orders.StatusDelivered, 100000, at(0, 1), "", ""},
		{"a2", "rest-a", orders.StatusServed, 50000, at(0, 23), "", ""},
		{"a3", "rest-a", orders.StatusCreated, 70000, at(0, 5), "", ""},
		{"a4", "rest-a", orders.StatusPreparing, 80000, at(0, 6), "", ""},
		{"a5", "rest-a", orders.StatusCancelled, 90000, at(0, 7), "", ""},
		{"a6", "rest-a", orders.StatusDelivered, 200000, at(10, 12), "", ""},
		{"b1", "rest-b", orders.StatusRejected, 40000, at(0, 8), "", ""},
		{"b2", "rest-b", orders.StatusPickedUp, 60000, at(3, 9), "", ""},
		// Kunning chegara vaqtlari: kecha 23:59 va bugun 00:00 — turli kunlarga tushadi.
		{"c1", "rest-c", orders.StatusDelivered, 11000, midnight.Add(-time.Minute), "", ""},
		{"c2", "rest-c", orders.StatusDelivered, 22000, midnight, "", ""},
		// To'lanmagan karta buyurtmasi hisoblanmaydi; to'langani hisoblanadi.
		{"u1", "rest-a", orders.StatusCreated, 999999, at(0, 2), orders.PaymentCard, orders.PaymentAwaiting},
		{"u2", "rest-a", orders.StatusDelivered, 33000, at(0, 3), orders.PaymentCard, orders.PaymentPaid},
		// Davrdan tashqarida (15 kun oldin): faqat "all" davrida ko'rinadi.
		{"old", "rest-a", orders.StatusDelivered, 5000, at(20, 12), "", ""},
	}
	q = stats.PlatformQuery{
		From: at(6, 0), To: midnight.AddDate(0, 0, 1),
		DailyFrom: midnight.AddDate(0, 0, -13), DailyDays: 14,
	}
	return rows, q
}

func normalizePlatform(d stats.PlatformData) stats.PlatformData {
	sort.Slice(d.Restaurants, func(i, j int) bool { return d.Restaurants[i].RestaurantID < d.Restaurants[j].RestaurantID })
	if d.Statuses == nil {
		d.Statuses = map[string]int{}
	}
	return d
}

func checkPlatformData(t *testing.T, got stats.PlatformData, q stats.PlatformQuery) {
	t.Helper()
	got = normalizePlatform(got)

	// Davr = so'nggi 7 kun (bugun + 6): old (20 kun oldin) va a6 (10 kun oldin) kirmaydi.
	want := []stats.RestaurantTotals{
		{RestaurantID: "rest-a", Orders: 6, New: 1, InProgress: 1, Completed: 3, Cancelled: 1, RevenueTiyin: 183000},
		{RestaurantID: "rest-b", Orders: 2, InProgress: 1, Cancelled: 1},
		{RestaurantID: "rest-c", Orders: 2, Completed: 2, RevenueTiyin: 33000},
	}
	if !reflect.DeepEqual(got.Restaurants, want) {
		t.Fatalf("restoranlar:\n keldi %+v\n kerak %+v", got.Restaurants, want)
	}
	if got.Statuses["delivered"] != 4 || got.Statuses["served"] != 1 || got.Statuses["created"] != 1 ||
		got.Statuses["cancelled"] != 1 || got.Statuses["rejected"] != 1 || got.Statuses["picked_up"] != 1 ||
		got.Statuses["preparing"] != 1 {
		t.Fatalf("holatlar noto'g'ri: %v", got.Statuses)
	}

	if len(got.Daily) != 14 {
		t.Fatalf("14 kun kerak, keldi %d", len(got.Daily))
	}
	// Oxirgi kun — bugun: a1, a2, a3, a4, a5, b1, c2, u2 = 8 ta (u1 hisoblanmaydi);
	// bajarilgan: a1, a2, c2, u2 = 4 ta, tushum 100000+50000+22000+33000.
	today := got.Daily[13]
	if today.Orders != 8 || today.Completed != 4 || today.RevenueTiyin != 205000 {
		t.Fatalf("bugun: 8 / 4 / 205000 kerak, keldi %+v", today)
	}
	// Kecha: c1 (23:59) — chegara to'g'ri tushishi shart.
	yesterday := got.Daily[12]
	if yesterday.Orders != 1 || yesterday.RevenueTiyin != 11000 {
		t.Fatalf("kecha: 1 / 11000 kerak, keldi %+v", yesterday)
	}
	// 3 kun oldin: b2. 10 kun oldin: a6.
	if d := got.Daily[10]; d.Orders != 1 || d.Completed != 0 {
		t.Fatalf("3 kun oldin: b2 (tugallanmagan) kerak, keldi %+v", d)
	}
	if d := got.Daily[3]; d.Orders != 1 || d.RevenueTiyin != 200000 {
		t.Fatalf("10 kun oldin: a6 kerak, keldi %+v", d)
	}
	// Sana yorlig'i to'g'ri va ketma-ket.
	if want := q.DailyFrom.Format("2006-01-02"); got.Daily[0].Date != want {
		t.Fatalf("birinchi kun %s bo'lishi kerak, keldi %s", want, got.Daily[0].Date)
	}
	if want := time.Now().In(stats.Location).Format("2006-01-02"); got.Daily[13].Date != want {
		t.Fatalf("oxirgi kun bugun (%s) bo'lishi kerak, keldi %s", want, got.Daily[13].Date)
	}
}

func TestPlatformStatsMemory(t *testing.T) {
	rows, q := platformFixture()
	repo := NewMemoryOrderRepo()
	for _, r := range rows {
		o := &orders.Order{ID: r.id, RestaurantID: r.restaurant, Status: r.status, TotalTiyin: r.total,
			CreatedAt: r.createdAt, PaymentMethod: r.method, PaymentState: r.state}
		if err := repo.Save(context.Background(), o); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.PlatformStats(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	checkPlatformData(t, got, q)
}

// TestPlatformStatsPostgres — HAQIQIY Postgres'da, VAQTINCHALIK sxemada:
// mavjud jadvallarga tegilmaydi, sxema test oxirida o'chiriladi. `orders`
// jadvalining faqat hisobga kirgan ustunlari yaratiladi (ustun nomlari
// `orderColumns` bilan bir xil).
func TestPlatformStatsPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("platform_stats_it_%d", time.Now().UnixNano())
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

	if _, err := pool.Exec(ctx, `CREATE TABLE orders (
		id text PRIMARY KEY,
		restaurant_id text NOT NULL,
		status text NOT NULL,
		total_tiyin bigint NOT NULL,
		created_at timestamptz NOT NULL,
		payment_method text NOT NULL DEFAULT '',
		payment_state text NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("jadval: %v", err)
	}
	rows, q := platformFixture()
	for _, r := range rows {
		if _, err := pool.Exec(ctx, `INSERT INTO orders (id, restaurant_id, status, total_tiyin, created_at, payment_method, payment_state)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			r.id, r.restaurant, string(r.status), r.total, r.createdAt, string(r.method), string(r.state)); err != nil {
			t.Fatalf("qator %s: %v", r.id, err)
		}
	}

	got, err := NewPgOrderRepo(pool).PlatformStats(ctx, q)
	if err != nil {
		t.Fatalf("PlatformStats: %v", err)
	}
	checkPlatformData(t, got, q)
}
