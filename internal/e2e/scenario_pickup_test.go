package e2e

import (
	"testing"
	"time"
)

// Olib ketish (Borish, Plane 473/474/477): restoran sozlamasi, chegirmali
// narxlash, faqat oldindan to'lov, kuryersiz hayot sikli, olish kodi bilan
// berish, 30 daqiqalik eslatma va 2 soatlik "olib ketilmadi" (pul qaytmaydi).
func TestPickupFlow(t *testing.T) {
	w := NewWorld(t, "Olib ketish (Borish)")

	// ── Restoran olib ketishni yoqadi (10% chegirma) ───────────────────
	w.Do(t, w.Customer, "PATCH", "/restaurants/"+RestA+"/settings",
		map[string]any{"pickup": map[string]any{"enabled": true}}).Expect(t, 403)
	w.Do(t, w.Owner, "PATCH", "/restaurants/"+RestA+"/settings",
		map[string]any{"pickup": map[string]any{"enabled": true, "discount_percent": 45}}).Expect(t, 400)
	st := w.Do(t, w.Owner, "PATCH", "/restaurants/"+RestA+"/settings", map[string]any{"pickup": map[string]any{
		"enabled": true, "prep_minutes": 20, "discount_percent": 10, "note": "Kassa 1-qavatda",
	}}).Expect(t, 200).Map(t)
	if p, _ := st["pickup"].(map[string]any); p["enabled"] != true || num(p, "discount_percent") != 10 {
		t.Fatalf("sozlama saqlanmadi: %v", st["pickup"])
	}
	rest := w.Do(t, nil, "GET", "/restaurants/"+RestA, nil).Expect(t, 200).Map(t)
	if p, _ := rest["pickup"].(map[string]any); p["enabled"] != true {
		t.Fatalf("mijozga olib ketish ko'rinmadi: %v", rest["pickup"])
	}

	// ── Narxlash: yetkazish haqi yo'q, 10% chegirma ───────────────────
	cart := Cart(ProdOsh, 2)
	q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote",
		map[string]any{"items": cart, "pickup": true}).Expect(t, 200).Map(t)
	wantOff := ProdOshPrice * 2 * 10 / 100
	if num(q, "delivery_fee_tiyin") != 0 || num(q, "pickup_discount_tiyin") != wantOff ||
		num(q, "total_tiyin") != ProdOshPrice*2-wantOff {
		t.Fatalf("olib ketish narxi: %v", q)
	}
	w.Do(t, w.Customer, "POST", "/restaurants/"+RestB+"/quote",
		map[string]any{"items": Cart(ProdB, 1), "pickup": true}).Expect(t, 400)

	// ── Yaratish: naqd rad, o'chiq restoran rad, karta — OK ────────────
	place := func(method, key string, items []map[string]any) *Resp {
		return w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": items, "payment_method": method, "idempotency_key": key, "fulfillment": "pickup",
		})
	}
	place("cash", "p-cash", cart).Expect(t, 400)
	place("card", "p-b", Cart(ProdB, 1)).Expect(t, 400)
	w.Do(t, w.Customer, "POST", "/orders", map[string]any{"items": cart, "payment_method": "card",
		"fulfillment": "uchar"}).Expect(t, 400)
	o := place("card", "p-1", cart).Expect(t, 201).Map(t)
	w.CheckOrder(t, o)
	id := o["id"].(string)
	if o["type"] != "pickup" || num(o, "delivery_fee_tiyin") != 0 || num(o, "pickup_discount_tiyin") != wantOff {
		t.Fatalf("buyurtma: %v", o)
	}
	w.PayCard(t, w.Customer, id)

	// ── Olish kodi: faqat egasi ───────────────────────────────────────
	code, _ := w.Do(t, w.Customer, "GET", "/orders/"+id+"/pickup-code", nil).Expect(t, 200).Map(t)["code"].(string)
	if len(code) != 4 {
		t.Fatalf("kod: %q", code)
	}
	w.Do(t, w.Other, "GET", "/orders/"+id+"/pickup-code", nil).Expect(t, 404)
	w.Do(t, w.Owner, "GET", "/orders/"+id+"/pickup-code", nil).Expect(t, 403)
	if body := string(w.Do(t, w.Owner, "GET", "/orders/"+id, nil).Expect(t, 200).Body); stringsContains(body, `"`+code+`"`) {
		t.Fatal("olish kodi restoranga ko'rinadigan buyurtma javobida sizdi")
	}

	// ── Hayot sikli: kuryersiz, kod bilan berish ──────────────────────
	w.Accept(t, id)
	w.Transition(t, w.Owner, id, "preparing")
	w.Transition(t, w.Owner, id, "ready")
	w.Sweep(t) // kuryer nazoratchisi olib ketishga tegmasligi kerak
	if got := w.Do(t, w.Owner, "GET", "/orders/"+id, nil).Map(t); got["courier_id"] != nil && got["courier_id"] != "" {
		t.Fatalf("olib ketishga kuryer biriktirildi: %v", got["courier_id"])
	}
	deliver := func(c string) *Resp {
		return w.Do(t, w.Owner, "POST", "/orders/"+id+"/transition", map[string]any{"to": "delivered", "pickup_code": c})
	}
	deliver("").Expect(t, 422)
	bad := "0000"
	if code == bad {
		bad = "1111"
	}
	deliver(bad).Expect(t, 422)
	w.TransitionRejected(t, w.Customer, id, "delivered")
	done := deliver(code).Expect(t, 200).Map(t)
	w.CheckOrder(t, done)
	if done["status"] != "delivered" {
		t.Fatalf("berilmadi: %v", done["status"])
	}
	w.Do(t, w.Customer, "GET", "/orders/"+id+"/pickup-code", nil).Expect(t, 409)

	// ── Kelmadi: 30 daq eslatma, 2 soatda yopiladi, pul qaytmaydi ─────
	o2 := place("card", "p-2", Cart(ProdOsh, 1)).Expect(t, 201).Map(t)
	id2 := o2["id"].(string)
	w.PayCard(t, w.Customer, id2)
	w.Accept(t, id2)
	w.Transition(t, w.Owner, id2, "preparing")
	w.Transition(t, w.Owner, id2, "ready")
	w.TransitionRejected(t, w.Owner, id2, "cancelled") // restoran tayyor buyurtmani bekor qila olmaydi

	w.Advance(31 * time.Minute)
	w.SweepPickups(t)
	r2 := w.Do(t, w.Customer, "GET", "/orders/"+id2, nil).Expect(t, 200).Map(t)
	if r2["status"] != "ready" || r2["pickup_reminded_at"] == nil {
		t.Fatalf("eslatma belgilanmadi: %v / %v", r2["status"], r2["pickup_reminded_at"])
	}
	w.Advance(90 * time.Minute)
	w.SweepPickups(t)
	r2 = w.Do(t, w.Customer, "GET", "/orders/"+id2, nil).Expect(t, 200).Map(t)
	w.CheckOrder(t, r2)
	if r2["status"] != "not_picked_up" {
		t.Fatalf("2 soatdan keyin yopilmadi: %v", r2["status"])
	}
	if ps := r2["payment_state"]; ps != "paid" {
		t.Fatalf("olinmagan buyurtma puli qaytmasligi kerak edi: %v", ps)
	}
}
