package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/orders"
)

// Postgres'da `delivery_fee_tiyin` ALOHIDA USTUN EMAS — invariant
// (`orders.Service`): TotalTiyin = SubtotalTiyin − DiscountTiyin +
// DeliveryFeeTiyin dan qayta hisoblanadi (`scanOrder`).
//
// ┌─ NEGA BU TEST YOZILDI ────────────────────────────────────────────┐
// 2026-10-22: `scanOrder` saqlangan ustunlarni (`total_tiyin`,
// `subtotal_tiyin`, `discount_tiyin`) o'qirdi, lekin
// `DeliveryFeeTiyin`ni ulardan QAYTA HISOBLAMASDI — natijada bu
// maydon doim Go'ning standart qiymati (0) bo'lib qolardi. Yangi
// yaratilgan buyurtmada (xotiradagi obyekt to'g'ridan-to'g'ri
// qaytarilgani uchun) haq TO'G'RI ko'rinardi, lekin keyinroq qayta
// ochilganda ("Buyurtmalarim" → buyurtma tafsiloti, Postgres'dan
// o'qilganda) mijoz chindan ham 8000 so'm to'lagan bo'lsa ham
// "Bepul" bo'lib ko'rinardi.
//
// Xotira (`MemoryOrderRepo`) bu xatoni USHLAMAYDI: u Go strukturasini
// to'g'ridan-to'g'ri saqlaydi/qaytaradi, ustun orqali round-trip
// qilmaydi — shuning uchun bu aynan Postgres integratsiya testi
// bo'lishi SHART.
// └───────────────────────────────────────────────────────────────────┘
func TestPgOrderRepo_DeliveryFeeSurvivesRoundTrip(t *testing.T) {
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
	schema := fmt.Sprintf("order_fee_it_%d", time.Now().UnixNano())
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
	// "public" ham kerak — PostGIS kengaytmasi turlari (`geography`,
	// 0029-migratsiya) o'sha sxemada yashaydi.
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migratsiya: %v", err)
	}

	repo := NewPgOrderRepo(pool)
	now := time.Now()
	o := &orders.Order{
		ID:               "o-fee-1",
		CustomerID:       "cust-1",
		RestaurantID:     "rest-1",
		Status:           orders.StatusCreated,
		SubtotalTiyin:    74_000_00,
		DeliveryFeeTiyin: 8_000_00,
		TotalTiyin:       82_000_00, // Subtotal - Discount(0) + DeliveryFee
		Items:            []orders.Item{},
		History:          []orders.StatusChange{},
		CreatedAt:        now,
		UpdatedAt:        now,
		Version:          1,
	}
	if err := repo.Save(ctx, o); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := repo.GetByID(ctx, o.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.DeliveryFeeTiyin != 8_000_00 {
		t.Fatalf("DeliveryFeeTiyin qayta o'qilganda 800000 kutilgan, keldi: %d (Postgres'dan o'qilgandan keyin "+
			"yetkazish haqi yo'qolib qoldi)", got.DeliveryFeeTiyin)
	}
	if got.TotalTiyin != got.SubtotalTiyin-got.DiscountTiyin+got.DeliveryFeeTiyin {
		t.Fatalf("invariant buzildi: jami=%d, taomlar=%d, chegirma=%d, yetkazish=%d",
			got.TotalTiyin, got.SubtotalTiyin, got.DiscountTiyin, got.DeliveryFeeTiyin)
	}
}
