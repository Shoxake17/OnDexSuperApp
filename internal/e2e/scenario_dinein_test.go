package e2e

import (
	"testing"
)

// ssenariy: STOLDA OVQATLANISH — QR kod, mijoz va ofitsiant buyurtmasi.
//
// Qoidalar: stol buyurtmasida kuryer ham, yetkazish haqi ham YO'Q;
// holat mashinasi `served` da tugaydi (kuryer holatlari yo'q).

func TestDineInFlow(t *testing.T) {
	w := NewWorld(t, "Stolda ovqatlanish (QR + ofitsiant)")
	var tableID, token string
	var waiter *Actor
	items := []map[string]any{{"product_id": ProdOsh, "qty": 1}}

	w.Step(t, "restoran stol yaratadi va QR token oladi", func(t *testing.T) {
		w.Do(t, w.Owner, "GET", "/tables/kinds", nil).Expect(t, 200)
		tb := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/tables",
			map[string]any{"label": "5", "zone": "Zal", "kind": "table", "capacity": 4}).Expect(t, 201).Map(t)
		tableID, _ = tb["id"].(string)
		token, _ = tb["qr_token"].(string)
		if tableID == "" || token == "" {
			t.Fatalf("stol/token yo'q: %v", tb)
		}
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/tables/batch",
			map[string]any{"kind": "table", "zone": "Terrasa", "prefix": "T", "from": 1, "count": 3, "capacity": 2}).Expect(t, 201)
		list := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/tables", nil).Expect(t, 200)
		if !contains(list.Body, tableID) {
			t.Fatal("yangi stol ro'yxatda yo'q")
		}
		w.Do(t, w.Owner, "PATCH", "/tables/"+tableID, map[string]any{"capacity": 6}).Expect(t, 200)
	})

	w.Step(t, "begona restoran stolni boshqara olmaydi", func(t *testing.T) {
		w.ExpectDenied(t, w.OwnerB, "PATCH", "/tables/"+tableID, map[string]any{"capacity": 1})
		w.ExpectDenied(t, w.OwnerB, "DELETE", "/tables/"+tableID, nil)
		w.ExpectDenied(t, w.Customer, "POST", "/restaurants/"+RestA+"/tables", map[string]any{"label": "9"})
	})

	w.Step(t, "mijoz QR ni skanerlaydi (token yechiladi, soxta token — yo'q)", func(t *testing.T) {
		r := w.Do(t, w.Customer, "GET", "/tables/resolve?token="+token, nil).Expect(t, 200).Map(t)
		if r["restaurant_id"] != RestA || r["table_id"] != tableID {
			t.Fatalf("resolve: %v", r)
		}
		if _, leaked := r["qr_token"]; leaked {
			t.Fatal("resolve javobi QR tokenni qaytardi")
		}
		w.Do(t, w.Customer, "GET", "/tables/resolve?token=soxta", nil).Expect(t, 404)
		w.ExpectDenied(t, nil, "GET", "/tables/resolve?token="+token, nil)
	})

	w.Step(t, "stolda yetkazish haqi YO'Q (kuryer bor bo'lsa ham)", func(t *testing.T) {
		addCourier(t, w, "+998901110002", `,"delivery_pay_tiyin":900000`)
		q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote",
			map[string]any{"items": items, "dine_in": true}).Expect(t, 200).Map(t)
		w.CheckQuote(t, q, true)
		if num(q, "total_tiyin") != ProdOshPrice {
			t.Fatalf("stol jami %d, %d kutilgan", num(q, "total_tiyin"), ProdOshPrice)
		}
		// Yetkazishda esa haq bor — farq aynan dine_in bayrog'ida.
		d := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote", map[string]any{"items": items}).Expect(t, 200).Map(t)
		if num(d, "delivery_fee_tiyin") != 900000 {
			t.Fatalf("yetkazishda haq 900000 bo'lishi kerak: %v", d)
		}
	})

	var order map[string]any
	var orderID string
	w.Step(t, "mijoz stolga buyurtma beradi", func(t *testing.T) {
		order = w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": items, "table_token": token, "party_size": 3,
			"payment_method": "cash", "idempotency_key": "e2e-dinein-1",
		}).Expect(t, 201).Map(t)
		orderID, _ = order["id"].(string)
		w.CheckOrder(t, order)
		if order["type"] != "dine_in" || num(order, "delivery_fee_tiyin") != 0 {
			t.Fatalf("stol buyurtmasi: %v", order)
		}
		if _, leaked := order["table_token"]; leaked || contains(mustJSON(order), token) {
			t.Fatal("buyurtma javobida QR token bor")
		}
	})

	w.Step(t, "restoran qabul qiladi — kuryer chaqirilmaydi", func(t *testing.T) {
		w.Do(t, w.Owner, "POST", "/orders/"+orderID+"/transition",
			map[string]any{"to": "accepted", "preparation_minutes": 10}).Expect(t, 200)
		o := w.Do(t, w.Owner, "GET", "/orders/"+orderID, nil).Expect(t, 200).Map(t)
		if o["dispatch_state"] != nil && o["dispatch_state"] != "" {
			t.Fatalf("stol buyurtmasida kuryer qidiruvi boshlandi: %v", o["dispatch_state"])
		}
		if o["courier_id"] != nil && o["courier_id"] != "" {
			t.Fatalf("stol buyurtmasiga kuryer biriktirildi: %v", o["courier_id"])
		}
	})

	w.Step(t, "ofitsiant akkaunti Xodimlar orqali yaratiladi", func(t *testing.T) {
		body := `{"first_name":"Vali","phone":"+998901110003","position":"waiter","app_access":true}`
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/staff", body).Expect(t, 201)
		waiter = w.LoginAs(t, "ofitsiant", "+998901110003")
		me := w.Do(t, waiter, "GET", "/waiter/me", nil).Expect(t, 200)
		if len(me.Body) < 5 {
			t.Fatal("waiter/me bo'sh")
		}
		w.Do(t, waiter, "GET", "/waiter/tables", nil).Expect(t, 200)
		w.Do(t, waiter, "GET", "/waiter/orders", nil).Expect(t, 200)
	})

	w.Step(t, "ofitsiant og'zaki buyurtma kiritadi", func(t *testing.T) {
		o := w.Do(t, waiter, "POST", "/waiter/orders", map[string]any{
			"table_id": tableID, "party_size": 2, "items": items, "idempotency_key": "e2e-waiter-1",
		}).Expect(t, 201).Map(t)
		// Ofitsiant ko'rinishi qisqartirilgan (subtotal/type yo'q) — faqat
		// jami = qatorlar yig'indisi tekshiriladi.
		w.CheckItemsTotal(t, o)
		if o["table_id"] != tableID || o["status"] != "created" {
			t.Fatalf("ofitsiant buyurtmasi: %v", o)
		}
	})

	w.Step(t, "tayyorlash -> tayyor -> stolga olib borildi", func(t *testing.T) {
		step := func(a *Actor, to string) {
			t.Helper()
			o := w.Do(t, a, "POST", "/orders/"+orderID+"/transition", map[string]any{"to": to}).Expect(t, 200).Map(t)
			w.CheckOrder(t, o)
		}
		step(w.Owner, "preparing")
		step(w.Owner, "ready")
		// Kuryer holatlari stolda yo'q.
		r := w.Do(t, waiter, "POST", "/orders/"+orderID+"/transition", map[string]any{"to": "picked_up"})
		if r.Status < 400 || r.Status >= 500 {
			t.Fatalf("stol buyurtmasi `picked_up` ga o'tdi: %d", r.Status)
		}
		step(waiter, "served")
		final := w.Do(t, w.Customer, "GET", "/orders/"+orderID, nil).Expect(t, 200).Map(t)
		w.CheckOrder(t, final)
	})

	w.Step(t, "begona ofitsiant/mijoz stol buyurtmasiga tegina olmaydi", func(t *testing.T) {
		w.ExpectDenied(t, w.Other, "GET", "/orders/"+orderID, nil)
		w.ExpectDenied(t, w.OwnerB, "GET", "/orders/"+orderID, nil)
	})

	w.Step(t, "stolni o'chirish: buyurtmasi bor stol — 409, bo'sh stol o'chadi", func(t *testing.T) {
		// Buyurtma tarixi bor stol o'chirilmaydi (tarix buzilmasin).
		w.Do(t, w.Owner, "DELETE", "/tables/"+tableID, nil).Expect(t, 409)
		spare := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/tables",
			map[string]any{"label": "99", "zone": "Zal", "kind": "table", "capacity": 2}).Expect(t, 201).Map(t)
		spareID, _ := spare["id"].(string)
		w.Do(t, w.Owner, "DELETE", "/tables/"+spareID, nil).Expect(t, 200)
		list := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/tables", nil).Expect(t, 200)
		if contains(list.Body, spareID) {
			t.Fatal("o'chirilgan stol ro'yxatda qoldi")
		}
	})
}
