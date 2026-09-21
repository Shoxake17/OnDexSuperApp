package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/orders"
)

// GET /admin/orders?restaurant_id= — admin panelidagi restoran moduli
// faqat o'sha restoranning buyurtmalarini ko'radi.
//
// Filtr SERVER tomonda bo'lishi shart: umumiy oxirgi 100 ta buyurtmadan
// mijoz tomonda ajratilsa, kam faol restoran uchun ro'yxat bo'sh chiqadi.
func TestAdminOrdersFilteredByRestaurant(t *testing.T) {
	f := adminServer(t)
	ctx := context.Background()
	now := time.Now()

	seed := []*orders.Order{
		{ID: "o-a1", RestaurantID: "rest-a", CustomerID: f.customerID, Status: orders.StatusCreated, CreatedAt: now},
		{ID: "o-a2", RestaurantID: "rest-a", CustomerID: f.customerID, Status: orders.StatusCreated, CreatedAt: now.Add(-time.Minute)},
		{ID: "o-b1", RestaurantID: "rest-b", CustomerID: f.customerID, Status: orders.StatusCreated, CreatedAt: now.Add(-2 * time.Minute)},
	}
	for _, o := range seed {
		if err := f.orderRepo.Save(ctx, o); err != nil {
			t.Fatal(err)
		}
	}

	ids := func(path string) map[string]bool {
		t.Helper()
		w := do(t, f.h, "GET", path, f.adminJWT, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: kutilgan 200, keldi %d (%s)", path, w.Code, w.Body.String())
		}
		var list []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
			t.Fatalf("javobni o'qib bo'lmadi: %v (%s)", err, w.Body.String())
		}
		got := map[string]bool{}
		for _, e := range list {
			got[e["id"].(string)] = true
		}
		return got
	}

	if got := ids("/admin/orders?restaurant_id=rest-a"); len(got) != 2 || !got["o-a1"] || !got["o-a2"] {
		t.Fatalf("rest-a uchun faqat o'zining 2 ta buyurtmasi kerak, keldi %v", got)
	}
	if got := ids("/admin/orders?restaurant_id=rest-b"); len(got) != 1 || !got["o-b1"] {
		t.Fatalf("rest-b uchun faqat o'zining buyurtmasi kerak, keldi %v", got)
	}
	// Mavjud bo'lmagan restoran — bo'sh ro'yxat (xato emas).
	if got := ids("/admin/orders?restaurant_id=yoq"); len(got) != 0 {
		t.Fatalf("noma'lum restoran uchun bo'sh ro'yxat kerak, keldi %v", got)
	}
	// Filtrsiz so'rov avvalgidek HAMMA restoranlarni qaytaradi.
	if got := ids("/admin/orders"); len(got) != 3 {
		t.Fatalf("filtrsiz so'rov 3 ta buyurtmani qaytarishi kerak, keldi %v", got)
	}
}

// Filtr admin huquqini chetlab o'tmaydi: mijoz tokeni bilan 403.
func TestAdminOrdersFilterStillRequiresAdmin(t *testing.T) {
	f := adminServer(t)
	w := do(t, f.h, "GET", "/admin/orders?restaurant_id=rest-a", f.customerJWT, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("mijoz uchun 403 kutilgan, keldi %d", w.Code)
	}
}
