package shop_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/modules/shop"
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

var seq atomic.Int64

func pgSvc(t *testing.T) (*shop.Service, *shop.PGRepo) {
	t.Helper()
	repo := shop.NewPGRepo(pgPool(t))
	tz, _ := time.LoadLocation("Asia/Tashkent")
	return shop.NewService(repo, func() string { return fmt.Sprintf("t%d-%d", time.Now().UnixNano(), seq.Add(1)) }, tz), repo
}

func uniqStore(cat string) shop.Store {
	return shop.Store{ID: fmt.Sprintf("pgtest-%d-%d", time.Now().UnixNano(), seq.Add(1)), Category: cat}
}

// Ikki kassa bir vaqtda bitta mahsulotni sotadi — "sotilgan" va kunlik
// yig'indi yo'qolmaydi (FOR UPDATE + UNIQUE kunlik kalit).
func TestPGConcurrentSell(t *testing.T) {
	s, repo := pgSvc(t)
	ctx := context.Background()
	st := uniqStore(shop.CategoryStore)
	if _, err := s.Upsert(ctx, st, shop.UpsertInput{Barcode: "c1", Name: "Cola", Category: "Ichimlik", PriceTiyin: 700000, Quantity: 100}); err != nil {
		t.Fatal(err)
	}
	const n = 20
	var wg sync.WaitGroup
	var failed atomic.Int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Sell(ctx, st, []shop.SellLine{{Barcode: "c1", Name: "Cola", Quantity: 1, PriceTiyin: 700000}}, "cash"); err != nil {
				failed.Add(1)
				t.Log(err)
			}
		}()
	}
	wg.Wait()
	if failed.Load() != 0 {
		t.Fatalf("%d ta sotuv yiqildi", failed.Load())
	}
	p, err := repo.GetProduct(ctx, st.ID, "c1")
	if err != nil || p.Quantity != 100-n || p.Sold != n {
		t.Fatalf("qoldiq/sotilgan: %+v %v", p, err)
	}
	sales, _ := s.Sales(ctx, st, nil, nil)
	if len(sales) != 1 || sales[0].Quantity != n || sales[0].TotalTiyin != n*700000 {
		t.Fatalf("kunlik yig'indi: %+v", sales)
	}
	lines, _ := s.SoldLines(ctx, st, nil, nil)
	if len(lines) != n {
		t.Fatalf("sotuv qatorlari: %d", len(lines))
	}
}

func TestPGProductsDebtsIsolation(t *testing.T) {
	s, _ := pgSvc(t)
	ctx := context.Background()
	a, b := uniqStore(shop.CategoryStore), uniqStore(shop.CategoryStore)
	if _, err := s.Upsert(ctx, a, shop.UpsertInput{Barcode: "x1", Name: "Non", Category: "Non", PriceTiyin: 300000, Quantity: 1.5}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, b, "x1"); !errors.Is(err, shop.ErrNotFound) {
		t.Fatalf("B A'ning mahsulotini o'chirdi: %v", err)
	}
	if list, _ := s.Products(ctx, b); len(list) != 0 {
		t.Fatal("B A'ning mahsulotini ko'rdi")
	}
	p, err := s.UpdateField(ctx, a, "x1", shop.FieldQuantity, 2.25)
	if err != nil || p.Quantity != 2.25 {
		t.Fatalf("miqdor: %+v %v", p, err)
	}
	d, err := s.AddDebt(ctx, a, "Vali", "+998900000000", 500000)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.PayDebt(ctx, b, d.ID, 1); !errors.Is(err, shop.ErrNotFound) {
		t.Fatalf("B A'ning qarzini to'ladi: %v", err)
	}
	full, cur, err := s.PayDebt(ctx, a, d.ID, 200000)
	if err != nil || full || cur.AmountTiyin != 300000 {
		t.Fatalf("qisman: %v %+v %v", full, cur, err)
	}
	if full, _, _ = s.PayDebt(ctx, a, d.ID, 300000); !full {
		t.Fatal("to'liq to'lov qarzni yopmadi")
	}
	if err := s.Delete(ctx, a, "x1"); err != nil {
		t.Fatal(err)
	}
}

func TestPGPharmacyAtomic(t *testing.T) {
	s, repo := pgSvc(t)
	ctx := context.Background()
	st := uniqStore(shop.CategoryPharmacy)
	exp := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	if _, err := s.AddPharm(ctx, st, shop.PharmAddInput{Barcode: "d1", Name: "Analgin", Category: "Og'riq", PriceTiyin: 200000, Quantity: 4, ExpiryDate: &exp}); err != nil {
		t.Fatal(err)
	}
	_, _ = s.AddPharm(ctx, st, shop.PharmAddInput{Barcode: "d2", Name: "Yod", Category: "Antiseptik", PriceTiyin: 100000, Quantity: 1})
	_, err := s.SellPharm(ctx, st, []shop.PharmSellLine{{Barcode: "d1", Quantity: 2, PriceTiyin: 200000}, {Barcode: "d2", Quantity: 3, PriceTiyin: 100000}}, "naqd")
	if !errors.Is(err, shop.ErrInsufficient) {
		t.Fatalf("yetmagan dori: %v", err)
	}
	d1, _ := repo.GetPharm(ctx, st.ID, "d1")
	if d1.Quantity != 4 {
		t.Fatalf("rollback ishlamadi: %v", d1.Quantity)
	}
	sale, err := s.SellPharm(ctx, st, []shop.PharmSellLine{{Barcode: "d1", Quantity: 2, PriceTiyin: 200000}}, "plastik")
	if err != nil || sale.TotalTiyin != 400000 {
		t.Fatalf("chek: %+v %v", sale, err)
	}
	list, err := s.PharmSales(ctx, st)
	if err != nil || len(list) != 1 || len(list[0].Items) != 1 || list[0].Items[0].Name != "Analgin" {
		t.Fatalf("cheklar: %+v %v", list, err)
	}
	info, err := s.PharmInfo(ctx, st, "analg")
	if err != nil || info.ExpiryDate == nil || !info.ExpiryDate.Equal(exp) || info.Quantity != 2 {
		t.Fatalf("pharm-info: %+v %v", info, err)
	}
}

// Global bazada qidiruv: LIKE maxsus belgilari so'zma-so'z, katta-kichik harf
// farqsiz, trigram indeksi mavjud.
func TestPGBaseSearchEscaping(t *testing.T) {
	s, _ := pgSvc(t)
	ctx := context.Background()
	pool := pgPool(t)
	tag := fmt.Sprintf("zz%d", seq.Add(1))
	_, err := pool.Exec(ctx, `INSERT INTO shop.product_base (barcode, name, category) VALUES
		($1, $2, 'Test'), ($3, $4, 'Test') ON CONFLICT DO NOTHING`,
		tag+"a", "Sok 100% "+tag, tag+"b", "Sok 100x "+tag)
	if err != nil {
		t.Fatal(err)
	}
	st := uniqStore(shop.CategoryStore)
	hits, err := s.Search(ctx, st, "100% "+strings.ToUpper(tag), "")
	if err != nil || len(hits) != 1 || hits[0].Barcode != tag+"a" {
		t.Fatalf("'%%' so'zma-so'z bo'lishi kerak: %+v %v", hits, err)
	}
	if hits, _ = s.Search(ctx, st, tag+"b", ""); len(hits) != 1 {
		t.Fatalf("shtrix-kod bo'yicha: %+v", hits)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname='shop' AND indexname LIKE '%trgm'`).Scan(&n)
	if n != 4 {
		t.Fatalf("trigram indekslari: %d", n)
	}
}

// Oxirgi donani ikki tasdiq bir vaqtda ololmaydi; bekor qilish bir marta qaytaradi.
func TestPGReserveOrderConcurrent(t *testing.T) {
	s, repo := pgSvc(t)
	ctx := context.Background()
	st := uniqStore(shop.CategoryQurt)
	w := 200
	if _, err := s.Upsert(ctx, st, shop.UpsertInput{Barcode: "q1", Name: "Qurt", Category: "Sho'r", PriceTiyin: 100000, Quantity: 1, WeightGrams: &w}); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var okN, failN atomic.Int64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := s.ReserveOrder(ctx, st, fmt.Sprintf("%s-o%d", st.ID, i), []shop.OrderItem{{Barcode: "q1", Qty: 1}}); err == nil {
				okN.Add(1)
			} else if errors.Is(err, shop.ErrInsufficient) {
				failN.Add(1)
			} else {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	p, _ := repo.GetProduct(ctx, st.ID, "q1")
	if okN.Load() != 1 || failN.Load() != 7 || p.Quantity != 0 {
		t.Fatalf("faqat bitta tasdiq o'tishi kerak: ok=%d fail=%d qty=%v", okN.Load(), failN.Load(), p.Quantity)
	}
	for i := 0; i < 8; i++ {
		_, _ = s.RestoreForOrder(ctx, fmt.Sprintf("%s-o%d", st.ID, i))
		_, _ = s.RestoreForOrder(ctx, fmt.Sprintf("%s-o%d", st.ID, i))
	}
	p, _ = repo.GetProduct(ctx, st.ID, "q1")
	if p.Quantity != 1 {
		t.Fatalf("qaytarish bir marta bo'lishi kerak: %v", p.Quantity)
	}
}
