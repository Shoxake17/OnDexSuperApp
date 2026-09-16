package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/orders"
)

func TestCourierPublicName(t *testing.T) {
	for in, want := range map[string]string{
		"Jasur Karimov":   "Jasur",
		"  Sardor  ":      "Sardor",
		"Ali Vali o'g'li": "Ali",
		"":                "",
		"   ":             "",
	} {
		if got := courierPublicName(in); got != want {
			t.Errorf("%q: kutilgan %q, keldi %q", in, want, got)
		}
	}
}

// Mijoz kuryerni ID bilan emas, ISMI bilan ko'radi; joylashuv faqat
// o'sha buyurtma mijoziga va faqat yetkazma davomida ochiq.
func TestOrderCourierInfoVisibility(t *testing.T) {
	f := staffServer(t)
	ctx := context.Background()
	_, _, c, courierTok := staffCourier(t, f, "+998901112240")
	if err := f.couriers.UpdateLocation(ctx, c.ID, 41.0012, 71.2345); err != nil {
		t.Fatal(err)
	}
	save := func(status orders.Status) {
		t.Helper()
		if err := f.orders.Save(ctx, &orders.Order{
			ID: "ord-track", CustomerID: "u-c", RestaurantID: testRestA, CourierID: c.ID,
			Status: status, CreatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	get := func(tok string) map[string]any {
		t.Helper()
		w := do(t, f.h, "GET", "/orders/ord-track", tok, "")
		if w.Code != http.StatusOK {
			t.Fatalf("GET /orders: %d %s", w.Code, w.Body.String())
		}
		var m map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}

	save(orders.StatusPreparing)

	cust := get(f.jwt["customer"])
	if cust["courier_name"] != "Jasur" {
		t.Fatalf("mijozga faqat ism ko'rinishi kerak: %v", cust["courier_name"])
	}
	loc, ok := cust["courier_location"].(map[string]any)
	if !ok || loc["lat"] != 41.0012 || loc["lng"] != 71.2345 {
		t.Fatalf("mijozga joriy joylashuv berilmadi: %v", cust["courier_location"])
	}

	rest := get(f.jwt["a"])
	if rest["courier_name"] != "Jasur Karimov" {
		t.Fatalf("restoranga to'liq ism: %v", rest["courier_name"])
	}
	if _, has := rest["courier_location"]; has {
		t.Fatal("restoranga mijoz kuzatuvi uchun joylashuv qo'shilmasligi kerak")
	}
	if self := get(courierTok); self["courier_name"] != nil || self["courier_location"] != nil {
		t.Fatalf("kuryerning o'ziga qo'shimcha maydon kerak emas: %v", self)
	}

	// "Buyurtmalarim" ro'yxati ham ID emas, ism.
	w := do(t, f.h, "GET", "/me/orders", f.jwt["customer"], "")
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) != 1 || list[0]["courier_name"] != "Jasur" {
		t.Fatalf("/me/orders: %d %s", w.Code, w.Body.String())
	}

	// Begona mijoz buyurtmani (demak kuryerni ham) ko'rmaydi.
	if w := do(t, f.h, "GET", "/orders/ord-track", f.jwt["b"], ""); w.Code != http.StatusNotFound {
		t.Fatalf("begona restoran: 404 kutilgan, keldi %d", w.Code)
	}

	// Yetkazilgach joylashuv yopiladi, ism qoladi.
	save(orders.StatusDelivered)
	done := get(f.jwt["customer"])
	if done["courier_name"] != "Jasur" {
		t.Fatalf("yetkazilgan buyurtmada ism: %v", done["courier_name"])
	}
	if _, has := done["courier_location"]; has {
		t.Fatal("yetkazilgan buyurtmada kuryer joylashuvi hamon ochiq")
	}
}

func TestCourierLocationSharedOnlyDuringDelivery(t *testing.T) {
	for _, tc := range []struct {
		o    *orders.Order
		want bool
	}{
		{nil, false},
		{&orders.Order{Status: orders.StatusPickedUp}, false},
		{&orders.Order{CourierID: "k", Status: orders.StatusAccepted}, true},
		{&orders.Order{CourierID: "k", Status: orders.StatusPickedUp}, true},
		{&orders.Order{CourierID: "k", Status: orders.StatusDelivered}, false},
		{&orders.Order{CourierID: "k", Status: orders.StatusCancelled}, false},
		{&orders.Order{CourierID: "k", Status: orders.StatusReady, Type: orders.TypeDineIn}, false},
	} {
		if got := courierLocationShared(tc.o); got != tc.want {
			t.Errorf("%+v: kutilgan %v, keldi %v", tc.o, tc.want, got)
		}
	}
}
