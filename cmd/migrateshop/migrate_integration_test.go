package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// Haqiqiy Mongo (ShoxPos) -> Postgres (shop) ko'chirish; ikki marta ishga
// tushiriladi — natija o'zgarmasligi (idempotentlik) tekshiriladi.
// TEST_DATABASE_URL va TEST_SHOXPOS_MONGO_URI bo'lmasa o'tkaziladi.
func TestMigrateEndToEnd(t *testing.T) {
	dsn, muri := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_SHOXPOS_MONGO_URI")
	if dsn == "" || muri == "" {
		if os.Getenv("CI") != "" && dsn != "" {
			t.Skip("CI'da Mongo yo'q")
		}
		t.Skip("TEST_DATABASE_URL / TEST_SHOXPOS_MONGO_URI berilmagan")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := storage.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(muri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(ctx) //nolint:errcheck // test
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	dbName := "shoxpos_migtest_" + suffix
	db := client.Database(dbName)
	defer db.Drop(ctx) //nolint:errcheck // test

	storeID := "st-mig-" + suffix
	phone := fmt.Sprintf("+99877%07d", time.Now().UnixNano()%10000000)
	if err := storage.NewPgUserRepo(pool).Create(ctx, &users.User{ID: "u-mig-" + suffix, Phone: phone, Name: "Mig Market",
		Role: users.RoleStore, EntityID: storeID, StoreCategory: users.StoreCategoryStore, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ins := func(coll string, docs ...any) {
		if _, err := db.Collection(coll).InsertMany(ctx, docs); err != nil {
			t.Fatal(coll, err)
		}
	}
	ins("users",
		bson.M{"login": "migmarket", "ondexEntityId": storeID, "ondexStoreCategory": "store", "shopType": "dokon",
			"shopCategories": []string{"Sut mahsulotlari", ""}},
		bson.M{"login": "yetim", "shopType": "dokon"}, // OnDex'ga bog'lanmagan
	)
	ins("products",
		bson.M{"login": "migmarket", "barcode": "4780001", "name": "Sut 1L", "category": "Sut mahsulotlari",
			"price": 12500.5, "basePrice": 10000, "quantity": 7.5, "sold": 3},
		bson.M{"login": "migmarket", "barcode": "yomon kod!", "name": "X"},
		bson.M{"login": "yetim", "barcode": "4780009", "name": "Yetim"},
	)
	ins("products_base", bson.M{"barcode": "base-" + suffix, "name": "Global choy", "price": 9000})
	ins("sales",
		bson.M{"login": "migmarket", "barcode": "4780001", "name": "Sut 1L", "quantity": 2, "price": 12500, "total": 25000,
			"paymentMethod": "naqd", "soldAt": now.Add(-2 * time.Hour)},
		bson.M{"login": "migmarket", "barcode": "4780001", "name": "Sut 1L", "quantity": 1, "price": 12500, "total": 12500,
			"paymentMethod": "naqd", "soldAt": now.Add(-time.Hour)},
	)
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	lines, _ := json.Marshal([]bucketLine{
		{Barcode: "4780001", Name: "Sut 1L", Quantity: 2, Price: 12500, Total: 25000, PaymentMethod: "naqd", SoldAt: now.Add(-2 * time.Hour)},
		{Barcode: "4780001", Name: "Sut 1L", Quantity: 1, Price: 12500, Total: 12500, PaymentMethod: "naqd", SoldAt: now.Add(-40 * 24 * time.Hour)},
	})
	_, _ = zw.Write(lines)
	_ = zw.Close()
	ins("salesbuckets", bson.M{"login": "migmarket", "periodStart": now, "periodEnd": now, "data": gz.Bytes()})
	ins("debts", bson.M{"login": "migmarket", "name": "Ali", "phone": "+998901112233", "amount": 150000, "lastPayment": now},
		bson.M{"login": "migmarket", "name": "Nol", "phone": "+998", "amount": 0})
	ins("shoptypes", bson.M{"value": "dokon", "label": "Do'kon", "categories": []string{"Ichimliklar"}})

	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("SHOXPOS_MONGO_URI", muri)
	t.Setenv("SHOXPOS_MONGO_DB", dbName)
	if err := run(false); err != nil { // quruq — hech narsa yozilmasin
		t.Fatal(err)
	}
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM shop.products WHERE store_id = $1`, storeID).Scan(&n)
	if n != 0 {
		t.Fatal("quruq ishga tushirish bazaga yozdi")
	}
	check := func() {
		t.Helper()
		var price, base int64
		var q, sold float64
		if err := pool.QueryRow(ctx, `SELECT price_tiyin, base_price_tiyin, quantity::float8, sold::float8 FROM shop.products
			WHERE store_id = $1 AND barcode = '4780001'`, storeID).Scan(&price, &base, &q, &sold); err != nil {
			t.Fatal(err)
		}
		if price != 1250050 || base != 1000000 || q != 7.5 || sold != 3 {
			t.Fatalf("mahsulot: %d %d %v %v", price, base, q, sold)
		}
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM shop.products WHERE store_id = $1`, storeID).Scan(&n)
		if n != 1 {
			t.Fatalf("mahsulotlar soni (yomon shtrix-kod o'tkazilishi kerak): %d", n)
		}
		var sq float64
		var total int64
		_ = pool.QueryRow(ctx, `SELECT count(*), COALESCE(sum(quantity),0)::float8, COALESCE(sum(total_tiyin),0) FROM shop.sales
			WHERE store_id = $1`, storeID).Scan(&n, &sq, &total)
		if n < 1 || n > 2 || sq != 3 || total != 3750000 {
			t.Fatalf("kunlik sotuvlar: %d %v %d", n, sq, total)
		}
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM shop.sale_lines WHERE store_id = $1`, storeID).Scan(&n)
		if n != 1 {
			t.Fatalf("sotilganlar (30 kundan eskisi ko'chmaydi): %d", n)
		}
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM shop.debts WHERE store_id = $1`, storeID).Scan(&n)
		if n != 1 {
			t.Fatalf("qarzlar (0 so'mlik o'tkaziladi): %d", n)
		}
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM shop.store_categories WHERE store_id = $1`, storeID).Scan(&n)
		if n != 1 {
			t.Fatalf("do'kon kategoriyalari: %d", n)
		}
		_ = pool.QueryRow(ctx, `SELECT count(*) FROM shop.product_base WHERE barcode = $1`, "base-"+suffix).Scan(&n)
		if n != 1 {
			t.Fatal("global baza")
		}
	}
	if err := run(true); err != nil {
		t.Fatal(err)
	}
	check()
	if err := run(true); err != nil { // takroriy — natija o'zgarmaydi
		t.Fatal(err)
	}
	check()
}
