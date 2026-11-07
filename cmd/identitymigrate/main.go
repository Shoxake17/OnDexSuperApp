// identitymigrate — A1b: eski restoran/do'kon/xodim akkauntlaridan
// tashkilot, biznes va a'zoliklarni to'ldirish (AUTH-PHASE-A.md 8-bo'lim).
//
//	go run ./cmd/identitymigrate            # QURUQ: faqat hisobot, bazaga yozmaydi
//	go run ./cmd/identitymigrate --apply    # bitta tranzaksiyada yozadi
//
// Muhit: DATABASE_URL (majburiy), MONGODB_URI + MONGODB_DB (restoran nomlari;
// bo'lmasa akkaunt nomi olinadi va hisobotda ogohlantirish chiqadi).
//
// Production tartibi: 1) to'liq zaxira (backup-others.sh) va tiklash sinovi;
// 2) quruq ishga tushirish, hisobotni egasi ko'radi; 3) --apply.
// Takroriy ishga tushirish xavfsiz: allaqachon ko'chirilganlar o'tkazib yuboriladi.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"chustapp/internal/httpapi"
	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/legacymigrate"
	"chustapp/internal/storage"
)

func main() {
	apply := flag.Bool("apply", false, "rejani bazaga yozish (busiz faqat hisobot)")
	flag.Parse()
	if err := run(*apply); err != nil {
		slog.Error("identitymigrate", "err", err)
		os.Exit(1)
	}
}

func run(apply bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL majburiy")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return err
	}
	defer pool.Close()
	// Migratsiya QILINMAYDI: quruq rejim bazaga hech narsa yozmasligi shart.
	// Sxemani API ishga tushganda o'zi yangilaydi (0063–0070).
	if err := storage.LegacySchemaReady(ctx, pool); err != nil {
		return err
	}

	in, err := storage.LoadLegacyMerchants(ctx, pool)
	if err != nil {
		return fmt.Errorf("eski holatni o'qish: %w", err)
	}
	if err := loadRestaurantNames(ctx, in); err != nil {
		return err
	}
	in.Now = time.Now().UTC()
	in.NewID = httpapi.NewID

	plan := legacymigrate.Build(*in)
	fmt.Printf("Eski merchant akkauntlari: %d\n\n%s", len(in.Users), plan.Summary())
	if !apply {
		fmt.Println("\nQURUQ ishga tushirish — bazaga HECH NARSA yozilmadi. Yozish uchun: --apply")
		return nil
	}
	if err := authz.EnsureTemplates(ctx, storage.NewPgAuthzStore(pool)); err != nil {
		return fmt.Errorf("rol shablonlari: %w", err)
	}
	res, err := storage.ApplyLegacyPlan(ctx, pool, plan, in.Now)
	if err != nil {
		return fmt.Errorf("qo'llash (hech narsa yozilmadi): %w", err)
	}
	fmt.Printf("\nYOZILDI: tashkilot %d, biznes %d, a'zolik %d\n", res.Orgs, res.Businesses, res.Memberships)
	return nil
}

// loadRestaurantNames — katalogdan (Mongo) restoran nomlari.
func loadRestaurantNames(ctx context.Context, in *legacymigrate.Input) error {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		fmt.Println("DIQQAT: MONGODB_URI yo'q — restoran nomlari o'rniga akkaunt nomlari olinadi")
		return nil
	}
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return err
	}
	defer client.Disconnect(context.Background()) //nolint:errcheck // chiqishda
	db := os.Getenv("MONGODB_DB")
	if db == "" {
		db = "chustapp"
	}
	list, err := storage.NewMongoCatalogRepo(client.Database(db)).ListRestaurants(ctx)
	if err != nil {
		return fmt.Errorf("katalog: %w", err)
	}
	for _, r := range list {
		in.RestaurantNames[r.ID] = r.Name
	}
	return nil
}
