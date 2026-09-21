package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/delivery"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// Toshkent qo'shilgandan keyin: restoran va yetkazish manzili BITTA
// shaharda bo'lishi shart. Busiz Chustdagi mijoz Toshkentdagi
// restorandan buyurtma bera olardi, kuryer esa restorandan 7 km ichida
// qidiriladi — buyurtma hech qachon yetkazilmasdi.

func cityOrderServer(t *testing.T, restLat, restLng, addrLat, addrLng float64) (http.Handler, string) {
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
		[]catalog.Restaurant{{ID: "rest-1", Name: "Avigo", Open: true, Lat: restLat, Lng: restLng}},
		[]catalog.Product{{
			ID: "prod-1", RestaurantID: "rest-1", Name: "Osh",
			PriceTiyin: 3000000, Available: true,
		}},
	)
	orderRepo := storage.NewMemoryOrderRepo()
	deps := Deps{
		UserRepo:    userRepo,
		OrderRepo:   orderRepo,
		CatalogRepo: catalogRepo,
		Tokens:      tokens,
		CatalogSvc:  catalog.NewService(catalogRepo),
		OrderSvc:    orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo()),
		DevMode:     true,
	}
	return New(deps).Routes(nil), jwt
}

func TestCreateOrder_RestaurantAndAddressCity(t *testing.T) {
	const (
		chustLat, chustLng       = 40.99454866260851, 71.24035120010376
		tashkentLat, tashkentLng = 41.2995, 69.2401
	)
	cases := []struct {
		name             string
		restLat, restLng float64
		addrLat, addrLng float64
		wantCode         int
	}{
		{"Toshkent restorani, Chust mijozi", tashkentLat, tashkentLng, chustLat, chustLng, http.StatusBadRequest},
		{"Chust restorani, Toshkent mijozi", chustLat, chustLng, tashkentLat, tashkentLng, http.StatusBadRequest},
		{"Toshkent restorani, Toshkent mijozi", tashkentLat, tashkentLng, 41.3650, 69.2880, http.StatusCreated},
		{"Chust restorani, Chust mijozi", chustLat, chustLng, chustLat, chustLng, http.StatusCreated},
		// Koordinatasi kiritilmagan restoran — avvalgi xatti-harakat.
		{"restoran koordinatasi yo'q", 0, 0, chustLat, chustLng, http.StatusCreated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, jwt := cityOrderServer(t, c.restLat, c.restLng, c.addrLat, c.addrLng)
			w := do(t, h, "POST", "/orders", jwt,
				`{"items":[{"product_id":"prod-1","qty":1}],"idempotency_key":"k1"}`)
			if w.Code != c.wantCode {
				t.Fatalf("kod %d, kutilgan %d — %s", w.Code, c.wantCode, w.Body.String())
			}
			if c.wantCode == http.StatusBadRequest &&
				!strings.Contains(w.Body.String(), delivery.ErrOtherCity.Error()) {
				t.Errorf("javobda sabab yo'q: %s", w.Body.String())
			}
		})
	}
}
