package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// `GET /products/search` testlari (bug.md 12-band).
//
// ┌─ NEGA BU ENDPOINT ALOHIDA E'TIBORGA LOYIQ ─────────────────────────┐
// U OCHIQ (autentifikatsiyasiz), har so'rov butun katalogni
// skanerlaydi va hech qanday chelakka tushmasdi — ya'ni bir qatorlik
// `curl` sikli bazani band qila olardi.
// └────────────────────────────────────────────────────────────────────┘

func searchServer(t *testing.T, products ...catalog.Product) http.Handler {
	t.Helper()
	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{{ID: "r1", Name: "Test kafe", Open: true}},
		products,
	)
	return New(Deps{
		UserRepo:       storage.NewMemoryUserRepo(),
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      storage.NewMemoryOrderRepo(),
		Tokens:         users.NewTokenIssuer("test-secret", time.Hour),
		CatalogSvc:     catalog.NewService(catalogRepo),
		OrderSvc: orders.NewService(storage.NewMemoryOrderRepo(), nil, NewID,
			storage.NewMemoryPromotionsRepo()),
		Hub:     ws.NewHub(nil),
		DevMode: true,
	}).Routes(nil)
}

// Juda uzun so'rov rad etilishi kerak — u foydali natija bermaydi,
// lekin normalizatsiya va solishtirish ishini oshiradi.
func TestSearchRejectsOverlongQuery(t *testing.T) {
	h := searchServer(t)

	long := strings.Repeat("a", maxSearchQueryLength+1)
	if w := do(t, h, "GET", "/products/search?q="+long, "", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("uzun so'rov qabul qilindi: %d", w.Code)
	}
	// Chegaradagi so'rov esa o'tishi kerak.
	ok := strings.Repeat("a", maxSearchQueryLength)
	if w := do(t, h, "GET", "/products/search?q="+ok, "", ""); w.Code != http.StatusOK {
		t.Fatalf("chegaradagi so'rov rad etildi: %d", w.Code)
	}
}

// Bo'sh so'rov bo'sh ro'yxat beradi va bazaga umuman bormaydi.
func TestSearchEmptyQuery(t *testing.T) {
	h := searchServer(t)
	w := do(t, h, "GET", "/products/search?q=", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var out []any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("bo'sh so'rov natija berdi: %v", out)
	}
}

// Endpoint ochiq qolishi kerak — u mijoz ilovasining turkum filtri
// uchun ishlatiladi va tokensiz chaqiriladi.
func TestSearchStaysPublic(t *testing.T) {
	h := searchServer(t, catalog.Product{
		ID: "p1", RestaurantID: "r1", Name: "Osh",
		Category: "Taomlar", PriceTiyin: 3000000, Available: true,
	})
	w := do(t, h, "GET", "/products/search?q=osh", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("ochiq qidiruv tokensiz ishlamadi: %d — %s", w.Code, w.Body.String())
	}
}

// Normalizatsiya semantikasi saqlanishi kerak: tinish belgilari
// e'tiborga olinmaydi. Bu Mongo tomonida regex bilan oldindan
// filtrlashdan ATAYLAB voz kechishning sababi.
func TestSearchIgnoresPunctuation(t *testing.T) {
	h := searchServer(t, catalog.Product{
		ID: "p1", RestaurantID: "r1", Name: "Coca Cola",
		Category: "Ichimliklar", PriceTiyin: 1500000, Available: true,
	})
	w := do(t, h, "GET", "/products/search?q=coca-cola", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var out []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("\"coca-cola\" so'rovi \"Coca Cola\" ni topmadi: %v", out)
	}
}

// Kesh kaliti normallashtirilgan so'rov bo'yicha: turli yozilishlar
// bitta yozuvni baham ko'radi.
func TestSearchCacheKeyIsNormalized(t *testing.T) {
	a := searchCacheKey(catalog.NormalizeForSearch("Coca-Cola"))
	b := searchCacheKey(catalog.NormalizeForSearch("  coca cola  "))
	if a != b {
		t.Fatalf("bir xil qidiruv ikki xil kalit berdi: %q va %q", a, b)
	}
	// Kalit Redis uchun xavfsiz bo'lishi kerak — faqat harf/raqam.
	if strings.ContainsAny(a, " \r\n\t{}") {
		t.Fatalf("kalitda xavfli belgi bor: %q", a)
	}
}
