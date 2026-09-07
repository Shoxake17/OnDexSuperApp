package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// `POST /admin/restaurants` testlari (bug.md 24 va 25-bandlar).

// failingUserRepo — `Create` da har doim xato beradigan o'ram.
// Kompensatsiya yo'lini sinash uchun: haqiqiy bazani buzish shart
// emas.
type failingUserRepo struct {
	users.Repository
}

func (f failingUserRepo) Create(context.Context, *users.User) error {
	return errors.New("baza yozmadi (sinov)")
}

func adminRestaurantServer(t *testing.T, userRepo users.Repository) (http.Handler, string, catalog.Repository) {
	t.Helper()
	ctx := context.Background()

	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	admin := &users.User{ID: "u-admin", Phone: "+998900000001",
		Role: users.RoleAdmin, PhoneVerified: true, CreatedAt: time.Now()}
	if err := userRepo.Create(ctx, admin); err != nil {
		// `failingUserRepo` uchun kutilgan — admin tokenini baribir
		// beramiz (`auth` faqat imzoni tekshiradi).
		_ = err
	}
	jwt, err := tokens.Issue(admin)
	if err != nil {
		t.Fatal(err)
	}

	catalogRepo := storage.NewMemoryCatalogRepo(nil, nil)
	h := New(Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      storage.NewMemoryOrderRepo(),
		CourierRepo:    storage.NewMemoryCourierRepo(),
		Tokens:         tokens,
		CatalogSvc:     catalog.NewService(catalogRepo),
		OrderSvc: orders.NewService(storage.NewMemoryOrderRepo(), nil, NewID,
			storage.NewMemoryPromotionsRepo()),
		Hub:     ws.NewHub(nil),
		DevMode: true,
	}).Routes(nil)
	return h, jwt, catalogRepo
}

// 25-BAND, 1-QISM: koordinata tekshirilmasdi.
//
// `(0,0)` yoki `(999,999)` restoran kuryer taqsimotini buzadi:
// `distanceKM` ma'nosiz qiymat beradi va hech bir kuryer mos
// kelmaydi — dispatch JIMGINA ishlamay qoladi.
func TestCreateRestaurantRejectsBadCoords(t *testing.T) {
	h, jwt, _ := adminRestaurantServer(t, storage.NewMemoryUserRepo())

	bad := []string{
		`{"name":"A","phone":"+998900000011","lat":999,"lng":999}`,
		`{"name":"A","phone":"+998900000012","lat":-91,"lng":10}`,
		`{"name":"A","phone":"+998900000013","lat":41,"lng":181}`,
	}
	for _, body := range bad {
		w := do(t, h, "POST", "/admin/restaurants", jwt, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("noto'g'ri koordinata qabul qilindi (%s): %d — %s",
				body, w.Code, w.Body.String())
		}
	}
}

// To'g'ri koordinata bilan yaratish ishlashda davom etishi kerak.
func TestCreateRestaurantAcceptsValidCoords(t *testing.T) {
	h, jwt, _ := adminRestaurantServer(t, storage.NewMemoryUserRepo())

	body := `{"name":"Chust Osh","phone":"+998900000021","lat":41.003,"lng":71.236}`
	w := do(t, h, "POST", "/admin/restaurants", jwt, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("to'g'ri restoran yaratilmadi: %d — %s", w.Code, w.Body.String())
	}
	var out struct {
		Restaurant catalog.Restaurant `json:"restaurant"`
		Account    users.User         `json:"account"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Restaurant.ID == "" || out.Account.EntityID != out.Restaurant.ID {
		t.Fatalf("akkaunt restoranga bog'lanmadi: %+v", out)
	}
}

// 25-BAND, 2-QISM: amal atomik emas edi.
//
// Restoran saqlanardi, keyin akkaunt yaratilardi. Ikkinchisi xato
// bersa restoran XODIMSIZ qolib ketardi va uni faqat bazadan qo'lda
// tozalash mumkin edi.
func TestCreateRestaurantRollsBackWhenAccountFails(t *testing.T) {
	repo := failingUserRepo{Repository: storage.NewMemoryUserRepo()}
	h, jwt, catalogRepo := adminRestaurantServer(t, repo)

	body := `{"name":"Yetim restoran","phone":"+998900000031","lat":41.003,"lng":71.236}`
	w := do(t, h, "POST", "/admin/restaurants", jwt, body)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("akkaunt xatosi javobda ko'rinmadi: %d", w.Code)
	}

	list, err := catalogRepo.ListRestaurants(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("YETIM RESTORAN qoldi (%d ta) — kompensatsiya ishlamadi: %+v",
			len(list), list)
	}
}

// 24-BAND: media manzillarida sxema tekshiriladi.
func TestUpdateRestaurantRejectsUnsafeMediaURL(t *testing.T) {
	h, jwt, _ := adminRestaurantServer(t, storage.NewMemoryUserRepo())

	// Avval restoran yaratamiz.
	w := do(t, h, "POST", "/admin/restaurants", jwt,
		`{"name":"A","phone":"+998900000041","lat":41.003,"lng":71.236}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("restoran yaratilmadi: %d — %s", w.Code, w.Body.String())
	}
	var created struct {
		Restaurant catalog.Restaurant `json:"restaurant"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id := created.Restaurant.ID

	bad := []string{
		`{"name":"A","lat":41.003,"lng":71.236,"logo_url":"javascript:alert(1)"}`,
		`{"name":"A","lat":41.003,"lng":71.236,"cover_url":"data:text/html;base64,PHNjcg"}`,
		`{"name":"A","lat":41.003,"lng":71.236,"logo_url":"//evil.example/a.png"}`,
	}
	for _, body := range bad {
		w := do(t, h, "POST", "/admin/restaurants/"+id, jwt, body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("xavfli manzil qabul qilindi (%s): %d", body, w.Code)
		}
	}

	// Halol manzil o'tishi kerak.
	ok := `{"name":"A","lat":41.003,"lng":71.236,"logo_url":"https://cdn.example/logo.png"}`
	if w := do(t, h, "POST", "/admin/restaurants/"+id, jwt, ok); w.Code != http.StatusOK {
		t.Errorf("halol manzil rad etildi: %d — %s", w.Code, w.Body.String())
	}
}
