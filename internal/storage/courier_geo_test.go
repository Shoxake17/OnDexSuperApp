package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/geo"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Yaqinlik bo'yicha qidiruv (migration 0029) testlari.
//
// Xotira va Postgres implementatsiyalari BIR XIL javob berishi kerak —
// aks holda testlar (xotirada) production'ni (Postgres) ifodalamaydi.
// Shu sabab asosiy tekshiruvlar `runProximitySuite` da bir marta
// yozilgan va IKKALA repozitoriyda ham ishlatiladi.

// Chust markazi va atrofidagi nuqtalar.
var (
	center = geo.LatLng{Lat: 41.0056, Lng: 71.2378}
	// ~540 m shimoli-sharqda
	near = geo.LatLng{Lat: 41.0010, Lng: 71.2400}
	// ~12 km narida (Namangan yo'nalishi) — radiusdan tashqarida
	far = geo.LatLng{Lat: 41.1000, Lng: 71.3000}
)

func seedCouriers() []couriers.Courier {
	return []couriers.Courier{
		{ID: "yaqin", Name: "Yaqin", Lat: near.Lat, Lng: near.Lng,
			Available: true, Approved: true, VehicleType: couriers.VehicleMoped, Rating: 5},
		{ID: "uzoq", Name: "Uzoq", Lat: far.Lat, Lng: far.Lng,
			Available: true, Approved: true, VehicleType: couriers.VehicleMoped, Rating: 5},
		{ID: "markaz", Name: "Markaz", Lat: center.Lat, Lng: center.Lng,
			Available: true, Approved: true, VehicleType: couriers.VehicleMoped, Rating: 5},
		{ID: "offline", Name: "Offline", Lat: center.Lat, Lng: center.Lng,
			Available: false, Approved: true, VehicleType: couriers.VehicleMoped, Rating: 5},
		{ID: "tasdiqsiz", Name: "Tasdiqsiz", Lat: center.Lat, Lng: center.Lng,
			Available: true, Approved: false, VehicleType: couriers.VehicleMoped, Rating: 5},
		// Restoranning O'Z kuryeri — markazda, onlayn, tasdiqlangan. Platforma
		// havuzida HECH QACHON ko'rinmasligi kerak (yuqoridagi barcha
		// tekshiruvlar platforma havuzida ishlaydi va uni sanamaydi).
		{ID: "restoranniki", Name: "Restoranniki", RestaurantID: "geo-rest", Lat: center.Lat, Lng: center.Lng,
			Available: true, Approved: true, VehicleType: couriers.VehicleMoped, Rating: 5},
	}
}

func ids(list []*couriers.Courier) []string {
	out := make([]string, len(list))
	for i, c := range list {
		out[i] = c.ID
	}
	return out
}

func runProximitySuite(t *testing.T, repo couriers.Repository) {
	t.Helper()
	ctx := context.Background()

	// XAVFSIZLIK CHEGARASI: restoran kuryeri faqat o'z restorani havuzida,
	// platforma kuryeri faqat platforma havuzida.
	t.Run("havuzlar aralashmaydi", func(t *testing.T) {
		got, err := repo.ListAvailableNear(ctx, "geo-rest", center.Lat, center.Lng, 7000, 0, 20)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].ID != "restoranniki" {
			t.Fatalf("restoran havuzida faqat o'z kuryeri bo'lishi kerak: %v", ids(got))
		}
		got, err = repo.ListAvailableNear(ctx, "begona-rest", center.Lat, center.Lng, 7000, 0, 20)
		if err != nil || len(got) != 0 {
			t.Fatalf("begona restoran havuzi bo'sh bo'lishi kerak: %v %v", ids(got), err)
		}
		got, err = repo.ListAvailableNear(ctx, couriers.PlatformPool, center.Lat, center.Lng, 7000, 0, 20)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range ids(got) {
			if id == "restoranniki" {
				t.Fatal("restoran kuryeri platforma havuziga tushdi")
			}
		}
	})

	t.Run("radius tashqarisidagi kuryer tushmaydi", func(t *testing.T) {
		got, err := repo.ListAvailableNear(ctx, couriers.PlatformPool, center.Lat, center.Lng, 7000, 0, 20)
		if err != nil {
			t.Fatalf("ListAvailableNear: %v", err)
		}
		for _, c := range got {
			if c.ID == "uzoq" {
				t.Fatalf("12 km naridagi kuryer 7 km radiusga tushdi: %v", ids(got))
			}
		}
		if len(got) != 2 {
			t.Fatalf("2 ta nomzod kutilgan (markaz, yaqin), olindi: %v", ids(got))
		}
	})

	t.Run("eng yaqinidan tartiblanadi", func(t *testing.T) {
		got, err := repo.ListAvailableNear(ctx, couriers.PlatformPool, center.Lat, center.Lng, 7000, 0, 20)
		if err != nil {
			t.Fatalf("ListAvailableNear: %v", err)
		}
		if got[0].ID != "markaz" {
			t.Fatalf("birinchi bo'lib eng yaqin kuryer kutilgan, olindi: %v", ids(got))
		}
	})

	t.Run("offline va tasdiqlanmagan kuryer tushmaydi", func(t *testing.T) {
		got, err := repo.ListAvailableNear(ctx, couriers.PlatformPool, center.Lat, center.Lng, 7000, 0, 20)
		if err != nil {
			t.Fatalf("ListAvailableNear: %v", err)
		}
		for _, c := range got {
			if c.ID == "offline" || c.ID == "tasdiqsiz" {
				t.Fatalf("mos kelmaydigan kuryer ro'yxatda: %v", ids(got))
			}
		}
	})

	t.Run("limit hurmat qilinadi", func(t *testing.T) {
		got, err := repo.ListAvailableNear(ctx, couriers.PlatformPool, center.Lat, center.Lng, 7000, 0, 1)
		if err != nil {
			t.Fatalf("ListAvailableNear: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("limit=1 bo'lsa 1 ta kutilgan, olindi: %d", len(got))
		}
		// Limit eng yaqinini KESIB TASHLAMASLIGI kerak.
		if got[0].ID != "markaz" {
			t.Fatalf("limit eng yaqinini tashlab yubordi: %v", ids(got))
		}
	})

	t.Run("juda kichik radiusda faqat ustidagi topiladi", func(t *testing.T) {
		got, err := repo.ListAvailableNear(ctx, couriers.PlatformPool, center.Lat, center.Lng, 50, 0, 20)
		if err != nil {
			t.Fatalf("ListAvailableNear: %v", err)
		}
		if len(got) != 1 || got[0].ID != "markaz" {
			t.Fatalf("50 m radiusda faqat 'markaz' kutilgan, olindi: %v", ids(got))
		}
	})

	// ★ KOORDINATA TARTIBI
	//
	// `ST_MakePoint(lng, lat)` — X, Y tartibida. Almashtirilsa kod
	// xatosiz ishlaydi, lekin kuryerlar butunlay boshqa joyda bo'ladi.
	// Chust uchun lat≈41, lng≈71: almashtirilsa nuqta Hindiston
	// okeanига tushadi va HECH KIM topilmaydi.
	t.Run("lat va lng almashtirilmagan", func(t *testing.T) {
		// Teskari koordinata bilan qidirsak HECH KIM topilmasligi kerak.
		got, err := repo.ListAvailableNear(ctx, couriers.PlatformPool, center.Lng, center.Lat, 7000, 0, 20)
		if err != nil {
			t.Fatalf("ListAvailableNear: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("lat/lng almashtirilgan — koordinata tartibi buzuq: %v", ids(got))
		}
	})
}

func TestProximityMemory(t *testing.T) {
	repo := NewMemoryCourierRepo(seedCouriers()...)
	runProximitySuite(t, repo)
}

// Eskirgan joylashuv filtri — FAQAT xotira uchun (Postgres varianti
// `location_updated_at` ni `UpdateLocation` orqali qo'yadi va uni
// testda soxtalashtirib bo'lmaydi).
func TestStaleLocationSkipped(t *testing.T) {
	repo := NewMemoryCourierRepo(seedCouriers()...)
	ctx := context.Background()

	// Joylashuv YANGILANMAGAN kuryerlar o'tadi (vaqt noma'lum).
	got, _ := repo.ListAvailableNear(ctx, couriers.PlatformPool, center.Lat, center.Lng, 7000, time.Minute, 20)
	if len(got) != 2 {
		t.Fatalf("vaqti noma'lum kuryerlar o'tishi kerak edi: %v", ids(got))
	}

	// Endi 'markaz' joylashuvini yangilaymiz — u yangi bo'ladi.
	if err := repo.UpdateLocation(ctx, "markaz", center.Lat, center.Lng); err != nil {
		t.Fatalf("UpdateLocation: %v", err)
	}
	got, _ = repo.ListAvailableNear(ctx, couriers.PlatformPool, center.Lat, center.Lng, 7000, time.Minute, 20)
	if len(got) != 2 {
		t.Fatalf("yangi joylashuvli kuryer tushib qoldi: %v", ids(got))
	}

	// maxAge NOL bo'lsa eskilik umuman tekshirilmaydi.
	got, _ = repo.ListAvailableNear(ctx, couriers.PlatformPool, center.Lat, center.Lng, 7000, 0, 20)
	if len(got) != 2 {
		t.Fatalf("maxAge=0 da hamma o'tishi kerak: %v", ids(got))
	}
}

// TestProximityPostgres — HAQIQIY Postgres + PostGIS ustida.
//
// `TEST_DATABASE_URL` berilmasa o'tkazib yuboriladi: CI/ishlab
// chiquvchi mashinasida baza bo'lmasligi mumkin va testlar shu sababli
// qizil bo'lmasligi kerak. Baza bo'lsa — bu ENG QIMMATLI test, chunki
// PostGIS so'rovining o'zini sinaydi.
func TestProximityPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		// ┌─ CI'DA O'TKAZIB YUBORISH TAQIQLANGAN (bug.md 102-band) ───┐
		// `t.Skip` JIM: quvur yashil bo'lardi, bu esa "eng qimmatli
		// test" (PostGIS so'rovining O'ZI) hech qachon ishlamaganini
		// yashirardi. Endi CI'da Postgres+PostGIS servis sifatida
		// ko'tariladi va u yetib kelmasa test YIQILADI.
		//
		// Lokal mashinada `t.Skip` o'z joyida: ishlab chiquvchida
		// baza bo'lmasligi mumkin.
		// └───────────────────────────────────────────────────────────┘
		if os.Getenv("CI") != "" {
			t.Fatal("CI'da TEST_DATABASE_URL BO'LISHI SHART — " +
				"deploy.yml dagi `go_quality.services.postgres` ishlayaptimi?")
		}
		t.Skip("TEST_DATABASE_URL berilmagan — Postgres testi o'tkazib yuborildi")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Test kuryerlari alohida prefiks bilan — haqiqiy ma'lumotga
	// tegmaymiz va oxirida tozalaymiz.
	const prefix = "geotest_"
	cleanup := func() {
		_, _ = pool.Exec(ctx, `DELETE FROM couriers WHERE id LIKE $1`, prefix+"%")
	}
	cleanup()
	t.Cleanup(cleanup)

	repo := NewPgCourierRepo(pool)
	for _, c := range seedCouriers() {
		c.ID = prefix + c.ID
		if err := repo.Create(ctx, &c); err != nil {
			t.Fatalf("Create(%s): %v", c.ID, err)
		}
	}

	// Umumiy to'plamni prefiksli ID'lar bilan ishlatish uchun
	// natijalarni filtrlaydigan o'ram.
	runProximitySuite(t, &prefixedRepo{Repository: repo, prefix: prefix})
}

// prefixedRepo — Postgres testida FAQAT shu testning kuryerlarini
// ko'rsatadi (bazada boshqa yozuvlar ham bo'lishi mumkin) va ID'lardan
// prefiksni olib tashlaydi, shunda umumiy to'plam o'zgarishsiz ishlaydi.
type prefixedRepo struct {
	couriers.Repository
	prefix string
}

func (p *prefixedRepo) ListAvailableNear(ctx context.Context, pool string, lat, lng float64,
	radius float64, maxAge time.Duration, limit int) ([]*couriers.Courier, error) {
	// Limitni kengaytiramiz: begona yozuvlar filtrlangandan keyin ham
	// kerakli miqdor qolsin.
	all, err := p.Repository.ListAvailableNear(ctx, pool, lat, lng, radius, maxAge, 0)
	if err != nil {
		return nil, err
	}
	var out []*couriers.Courier
	for _, c := range all {
		if len(c.ID) > len(p.prefix) && c.ID[:len(p.prefix)] == p.prefix {
			cp := *c
			cp.ID = c.ID[len(p.prefix):]
			out = append(out, &cp)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
