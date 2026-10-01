package e2e

import (
	"context"
	"testing"
	"time"

	"chustapp/internal/payments/multicard"
)

// Qayta ishlatiladigan oqim bo'laklari: ssenariylar shularni tuzib ishlatadi.

// Cart — bitta taomdan iborat savat.
func Cart(product string, qty int) []map[string]any {
	return []map[string]any{{"product_id": product, "qty": qty}}
}

// PlaceOrder — mijoz buyurtma beradi va javobni (order) qaytaradi.
func (w *World) PlaceOrder(t *testing.T, who *Actor, items []map[string]any, method, key string) map[string]any {
	t.Helper()
	o := w.Do(t, who, "POST", "/orders", map[string]any{
		"items": items, "payment_method": method, "idempotency_key": key,
	}).Expect(t, 201).Map(t)
	w.CheckOrder(t, o)
	return o
}

// PayCard — karta buyurtmasiga to'lov ochadi va shlyuz callback'i (to'g'ri
// imzo bilan) orqali TASDIQLAYDI. To'lov ID'sini qaytaradi.
func (w *World) PayCard(t *testing.T, who *Actor, orderID string) string {
	t.Helper()
	w.Do(t, who, "POST", "/orders/"+orderID+"/pay", nil).Expect(t, 200)
	list, err := w.Payments.ListByOrder(context.Background(), orderID)
	if err != nil || len(list) == 0 {
		t.Fatalf("to'lov yozuvi yo'q: %v", err)
	}
	p := list[len(list)-1]
	w.Gateway.mu.Lock()
	w.Gateway.StatusFails = true // yagona dalil — imzo
	w.Gateway.mu.Unlock()
	cb := multicard.Callback{
		UUID: p.ProviderPaymentID, InvoiceID: p.ID, Amount: multicard.Tiyin(p.AmountTiyin), Status: "success",
		Sign: Sign(p.ProviderPaymentID, p.ID, p.AmountTiyin, GatewaySecret),
	}
	w.Do(t, nil, "POST", "/payments/multicard/callback", cb).Expect(t, 200)
	o := w.Do(t, who, "GET", "/orders/"+orderID, nil).Expect(t, 200).Map(t)
	if st, _ := o["payment_state"].(string); st != "paid" && st != "held" {
		t.Fatalf("to'lov tasdiqlanmadi: %q", st)
	}
	return p.ID
}

// PaymentStatus — to'lov holati (xotira repo'sidan).
func (w *World) PaymentStatus(t *testing.T, paymentID string) string {
	t.Helper()
	p, err := w.Payments.GetByID(context.Background(), paymentID)
	if err != nil {
		t.Fatal(err)
	}
	return string(p.Status)
}

// Transition — buyurtmani o'tkazadi va yangi holatni qaytaradi.
func (w *World) Transition(t *testing.T, who *Actor, orderID, to string) map[string]any {
	t.Helper()
	o := w.Do(t, who, "POST", "/orders/"+orderID+"/transition", map[string]any{"to": to}).Expect(t, 200).Map(t)
	if o["status"] != to {
		t.Fatalf("holat %v, %s kutilgan", o["status"], to)
	}
	w.CheckOrder(t, o)
	return o
}

// TransitionRejected — o'tish RAD ETILISHI kerak (409/400/404) va holat o'zgarmaydi.
func (w *World) TransitionRejected(t *testing.T, who *Actor, orderID, to string) {
	t.Helper()
	// Tekshiruv uchun o'qishlar mahsulot "takroriy so'rov" signali emas.
	w.polling = true
	defer func() { w.polling = false }()
	before := w.Do(t, w.Owner, "GET", "/orders/"+orderID, nil).Map(t)["status"]
	r := w.Do(t, who, "POST", "/orders/"+orderID+"/transition", map[string]any{"to": to, "preparation_minutes": 5})
	if r.Status < 400 || r.Status >= 500 {
		t.Fatalf("%s -> %s rad etilishi kerak edi: %d %s", actorName(who), to, r.Status, r.Body)
	}
	after := w.Do(t, w.Owner, "GET", "/orders/"+orderID, nil).Map(t)["status"]
	if before != after {
		w.Violate(t, InvOrderHistory, "rad etilgan o'tish holatni o'zgartirdi: %v -> %v", before, after)
	}
}

// Accept — restoran qabul qiladi (dispatch ishga tushadi).
func (w *World) Accept(t *testing.T, orderID string) {
	t.Helper()
	w.Do(t, w.Owner, "POST", "/orders/"+orderID+"/transition",
		map[string]any{"to": "accepted", "preparation_minutes": 1}).Expect(t, 200)
}

// ShiftOn — kuryer smenaga chiqadi (joylashuv + bo'sh).
func (w *World) ShiftOn(t *testing.T, c *Actor) {
	t.Helper()
	w.Do(t, c, "POST", "/couriers/"+c.Entity+"/location", map[string]any{"lat": ChustLat, "lng": ChustLng}).Expect(t, 200)
	w.Do(t, c, "POST", "/couriers/"+c.Entity+"/available", map[string]any{"available": true}).Expect(t, 200)
}

// AcceptOffer — kuryer taklifni kutadi va qabul qiladi; buyurtmaga biriktirilguncha kutadi.
func (w *World) AcceptOffer(t *testing.T, c *Actor, orderID string) {
	t.Helper()
	w.Eventually(t, c.Name+" ga taklif keldi", 8*time.Second, func() bool {
		return w.Do(t, c, "GET", "/couriers/"+c.Entity+"/offer", nil).Map(t)["offer"] != nil
	})
	w.Do(t, c, "POST", "/couriers/"+c.Entity+"/respond", map[string]any{"order_id": orderID, "accepted": true}).Expect(t, 200)
	w.Eventually(t, "buyurtmaga kuryer biriktirildi", 8*time.Second, func() bool {
		return w.Do(t, w.Owner, "GET", "/orders/"+orderID, nil).Map(t)["courier_id"] == c.Entity
	})
}

// Deliver — tayyorlash -> tayyor -> olib ketish -> yetkazish (kuryer biriktirilgan bo'lishi kerak).
func (w *World) Deliver(t *testing.T, c *Actor, orderID string) {
	t.Helper()
	w.Transition(t, w.Owner, orderID, "preparing")
	w.Transition(t, w.Owner, orderID, "ready")
	w.Transition(t, c, orderID, "picked_up")
	w.Transition(t, c, orderID, "delivered")
}
