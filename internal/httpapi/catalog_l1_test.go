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

// l1Server — catalogServer bilan bir xil, lekin *Server ham qaytadi
// (L1 ichini tekshirish uchun).
func l1Server(t *testing.T) (*Server, http.Handler, string) {
	t.Helper()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	u := &users.User{ID: "u-a", Phone: "+998900000031", Role: users.RoleRestaurant,
		EntityID: catTestRestA, PhoneVerified: true, CreatedAt: time.Now()}
	if err := userRepo.Create(t.Context(), u); err != nil {
		t.Fatal(err)
	}
	jwt, err := tokens.Issue(u)
	if err != nil {
		t.Fatal(err)
	}
	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{
			{ID: catTestRestA, Name: "A restoran", Open: true},
			{ID: catTestRestB, Name: "B restoran", Open: true},
		},
		[]catalog.Product{
			{ID: "somsa-a", RestaurantID: catTestRestA, Name: "Somsa",
				Category: "Taomlar", PriceTiyin: 1200000, Available: true},
		},
	)
	s := New(Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      storage.NewMemoryOrderRepo(),
		Tokens:         tokens,
		CatalogSvc:     catalog.NewService(catalogRepo),
		OrderSvc: orders.NewService(storage.NewMemoryOrderRepo(), nil, NewID,
			storage.NewMemoryPromotionsRepo()),
		Hub:     ws.NewHub(nil),
		DevMode: true,
	})
	return s, s.Routes(nil), jwt
}

func menuNames(t *testing.T, h http.Handler) []string {
	t.Helper()
	w := do(t, h, "GET", "/restaurants/"+catTestRestA+"/menu", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("menyu: %d — %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("menyu Content-Type: %q", ct)
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, p := range list {
		names = append(names, p["name"].(string))
	}
	return names
}

// Panel taomni o'zgartirsa, ochiq menyu 3 s L1 TTL'ni KUTMASDAN yangilanadi:
// saqlash yo'li dropMenuCache orqali L1'ni ham tozalaydi.
func TestCatalogL1MenuInvalidatedOnSave(t *testing.T) {
	s, h, jwt := l1Server(t)

	if got := menuNames(t, h); len(got) != 1 || got[0] != "Somsa" {
		t.Fatalf("boshlang'ich menyu: %v", got)
	}
	if _, ok, _ := s.catL1.menus.Peek(catTestRestA); !ok {
		t.Fatal("menyu L1 ga tushmadi")
	}

	body := `{"id":"somsa-a","name":"Yangi somsa","category":"Taomlar",` +
		`"price_tiyin":1300000,"available":true}`
	if w := do(t, h, "POST", "/restaurants/"+catTestRestA+"/products", jwt, body); w.Code >= 300 {
		t.Fatalf("saqlash: %d — %s", w.Code, w.Body.String())
	}
	if got := menuNames(t, h); len(got) != 1 || got[0] != "Yangi somsa" {
		t.Fatalf("saqlashdan keyin eski menyu qaytdi (L1 tozalanmadi): %v", got)
	}

	if w := do(t, h, "DELETE", "/restaurants/"+catTestRestA+"/products/somsa-a", jwt, ""); w.Code >= 300 {
		t.Fatalf("o'chirish: %d — %s", w.Code, w.Body.String())
	}
	if got := menuNames(t, h); len(got) != 0 {
		t.Fatalf("o'chirilgan taom menyuda qoldi: %v", got)
	}
}

// Restoran yopilsa, ro'yxat darhol yangilanadi (L1 eskirgani ko'rinmaydi).
func TestCatalogL1RestaurantsInvalidatedOnOpenToggle(t *testing.T) {
	_, h, jwt := l1Server(t)

	openOf := func() bool {
		t.Helper()
		w := do(t, h, "GET", "/restaurants", "", "")
		if w.Code != http.StatusOK {
			t.Fatalf("ro'yxat: %d — %s", w.Code, w.Body.String())
		}
		var list []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		for _, r := range list {
			if r["id"] == catTestRestA {
				return r["open"].(bool)
			}
		}
		t.Fatal("A restoran ro'yxatda yo'q")
		return false
	}

	if !openOf() {
		t.Fatal("boshida ochiq bo'lishi kerak")
	}
	if w := do(t, h, "POST", "/restaurants/"+catTestRestA+"/open", jwt, `{"open":false}`); w.Code != http.StatusOK {
		t.Fatalf("yopish: %d — %s", w.Code, w.Body.String())
	}
	if openOf() {
		t.Fatal("yopilgan restoran ro'yxatda ochiq ko'rinmoqda (L1 tozalanmadi)")
	}
}

// L1 dagi ro'yxat bo'lishilgan — handlerlar uni O'ZGARTIRMASLIGI kerak
// (open_now, maket imzosi va h.k. faqat javob nusxasiga yoziladi).
func TestCatalogL1ListNotMutated(t *testing.T) {
	s, h, jwt := l1Server(t)

	if w := do(t, h, "GET", "/restaurants", "", ""); w.Code != http.StatusOK {
		t.Fatalf("ro'yxat: %d", w.Code)
	}
	cached, ok, _ := s.catL1.rests.Peek(allRestaurantsL1Key)
	if !ok {
		t.Fatal("ro'yxat L1 ga tushmadi")
	}
	before, err := json.Marshal(cached)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		do(t, h, "GET", "/restaurants", "", "")
		do(t, h, "GET", "/restaurants", jwt, "")
		do(t, h, "GET", "/restaurants?city=chust", "", "")
	}

	after, err := json.Marshal(cached)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("handler L1 ro'yxatini o'zgartirdi:\noldin: %s\nkeyin: %s", before, after)
	}
	for _, r := range cached {
		if r.OpenNow != nil {
			t.Fatalf("%s: open_now kesh ICHIGA yozilgan", r.ID)
		}
	}
}
