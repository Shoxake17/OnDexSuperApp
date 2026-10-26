package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/banners"
	"chustapp/internal/catalog"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// Chust va Toshkent kordinatalari — `routes_orders_city_test.go` dagi
// bilan AYNAN bir xil (shaharlararo masofa real, testlar bir-biriga mos).
const (
	bannerTestChustLat, bannerTestChustLng       = 40.99454866260851, 71.24035120010376
	bannerTestTashkentLat, bannerTestTashkentLng = 41.2995, 69.2401
)

// bannerServer — Chustda bitta restoran, unga bog'langan banner, va
// yana bitta HAVOLASIZ (umumiy) banner bilan test serveri.
func bannerServer(t *testing.T, addrLat, addrLng float64) (http.Handler, string) {
	t.Helper()
	ctx := context.Background()

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	customer := &users.User{
		ID: "cust-1", Phone: "+998901234567",
		Role: users.RoleCustomer, PhoneVerified: true,
		Address: users.AddressDetails{Lat: addrLat, Lng: addrLng, Text: "Manzil"},
	}
	if err := userRepo.Create(ctx, customer); err != nil {
		t.Fatal(err)
	}
	jwt, err := tokens.Issue(customer)
	if err != nil {
		t.Fatal(err)
	}

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{
			{ID: "rest-chust", Name: "Chust restorani",
				Lat: bannerTestChustLat, Lng: bannerTestChustLng},
		},
		nil,
	)

	bannerRepo := storage.NewMemoryBannerRepo()
	now := time.Now()
	if err := bannerRepo.Create(ctx, &banners.Banner{
		ID: "b-linked", ImageURL: "https://x/1.webp",
		RestaurantID: "rest-chust", Active: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := bannerRepo.Create(ctx, &banners.Banner{
		ID: "b-global", ImageURL: "https://x/2.webp",
		Active: true, SortOrder: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	deps := Deps{
		UserRepo:    userRepo,
		CatalogRepo: catalogRepo,
		BannerRepo:  bannerRepo,
		Tokens:      tokens,
		DevMode:     true,
	}
	return New(deps).Routes(nil), jwt
}

func bannerIDs(t *testing.T, body []byte) []string {
	t.Helper()
	var list []map[string]any
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("javobni o'qib bo'lmadi: %v — %s", err, body)
	}
	ids := make([]string, len(list))
	for i, b := range list {
		ids[i] = b["id"].(string)
	}
	return ids
}

// ★ ASOSIY: restoranga bog'langan banner faqat O'SHA restoran yetkaza
// oladigan shahardagi mijozga ko'rinadi — xuddi restoranlar ro'yxati
// kabi (`visibleRestaurants`). Havolasiz banner HAR DOIM ko'rinadi.
func TestVisibleBanners_FilteredByCity(t *testing.T) {
	t.Run("Chust mijozi — ikkalasini ham ko'radi", func(t *testing.T) {
		h, jwt := bannerServer(t, bannerTestChustLat, bannerTestChustLng)
		w := do(t, h, "GET", "/promo-banners?for_me=1", jwt, "")
		if w.Code != http.StatusOK {
			t.Fatalf("kod %d: %s", w.Code, w.Body.String())
		}
		ids := bannerIDs(t, w.Body.Bytes())
		if len(ids) != 2 {
			t.Fatalf("ikkalasi ham kutilgan, keldi: %v", ids)
		}
	})

	t.Run("Toshkent mijozi — faqat havolasiz bannerni ko'radi", func(t *testing.T) {
		h, jwt := bannerServer(t, bannerTestTashkentLat, bannerTestTashkentLng)
		w := do(t, h, "GET", "/promo-banners?for_me=1", jwt, "")
		if w.Code != http.StatusOK {
			t.Fatalf("kod %d: %s", w.Code, w.Body.String())
		}
		ids := bannerIDs(t, w.Body.Bytes())
		if len(ids) != 1 || ids[0] != "b-global" {
			t.Fatalf("faqat 'b-global' kutilgan, keldi: %v", ids)
		}
	})

	t.Run("for_me yo'q — filtrsiz, hammasi", func(t *testing.T) {
		h, jwt := bannerServer(t, bannerTestTashkentLat, bannerTestTashkentLng)
		w := do(t, h, "GET", "/promo-banners", jwt, "")
		if w.Code != http.StatusOK {
			t.Fatalf("kod %d: %s", w.Code, w.Body.String())
		}
		if ids := bannerIDs(t, w.Body.Bytes()); len(ids) != 2 {
			t.Fatalf("filtrsiz ikkalasi ham kutilgan, keldi: %v", ids)
		}
	})

	t.Run("anonim — filtrsiz, hammasi", func(t *testing.T) {
		h, _ := bannerServer(t, bannerTestTashkentLat, bannerTestTashkentLng)
		w := do(t, h, "GET", "/promo-banners?for_me=1", "", "")
		if w.Code != http.StatusOK {
			t.Fatalf("kod %d: %s", w.Code, w.Body.String())
		}
		if ids := bannerIDs(t, w.Body.Bytes()); len(ids) != 2 {
			t.Fatalf("manzilsiz foydalanuvchi uchun ikkalasi ham kutilgan, keldi: %v", ids)
		}
	})
}
