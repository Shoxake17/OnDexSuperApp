package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// "Buyurtmalarim" ro'yxatida buyurtma raqami bo'lishi SHART — avval
// maydon yo'q edi va mijoz ilovasi kartochka tepasida "—" ko'rsatardi.
func TestMeOrdersIncludesOrderNumber(t *testing.T) {
	ctx := context.Background()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	u := &users.User{ID: "u-mo", Phone: "+998900000391", Role: users.RoleCustomer,
		PhoneVerified: true, CreatedAt: time.Now()}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	tok, err := tokens.Issue(u)
	if err != nil {
		t.Fatal(err)
	}

	orderRepo := storage.NewMemoryOrderRepo()
	svc := orders.NewService(orderRepo, nil, NewID, nil)
	created, err := svc.Create(ctx, &orders.Order{
		CustomerID: "u-mo", RestaurantID: testRestA, Type: orders.TypeDelivery,
		Items: []orders.Item{{ProductID: "p1", Name: "Osh", Qty: 1, PriceTiyin: 3_500_000}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.OrderNumber == "" {
		t.Fatal("buyurtma raqami yaratilmadi")
	}

	h := New(Deps{
		UserRepo:    userRepo,
		OrderRepo:   orderRepo,
		OrderSvc:    svc,
		CatalogRepo: storage.NewMemoryCatalogRepo(nil, nil),
		Tokens:      tokens,
		DevMode:     true,
	}).Routes(nil)

	w := do(t, h, "GET", "/me/orders", tok, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /me/orders: %d %s", w.Code, w.Body.String())
	}
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0]["order_number"] != created.OrderNumber {
		t.Fatalf("buyurtma raqami javobda yo'q: %v", list)
	}
}
