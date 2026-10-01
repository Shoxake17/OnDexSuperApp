package e2e

import (
	"fmt"
	"testing"
	"time"
)

// ssenariy: YETKAZISH — naqd to'lov, kuryer bilan, boshidan oxirigacha.
//
// Mijoz menyuni ko'radi, narxlaydi, buyurtma beradi; restoran qabul qiladi;
// kuryer (restoran xodimi sifatida qo'shilgan) taklifni qabul qiladi,
// olib ketadi va yetkazadi. Har qadamda pul va holat invariantlari
// tekshiriladi.

// addCourier — restoran egasi "Xodimlar" orqali yetkazib beruvchi qo'shadi
// va kuryer akkauntiga kiradi.
func addCourier(t *testing.T, w *World, phone, payJSON string) (*Actor, string) {
	t.Helper()
	body := fmt.Sprintf(`{"first_name":"Jasur","phone":%q,"position":"courier","app_access":true%s}`, phone, payJSON)
	r := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/staff", body).Expect(t, 201)
	m := r.Map(t)
	id, _ := m["id"].(string)
	return w.LoginAs(t, "kuryer", phone), id
}

func TestDeliveryCashFlow(t *testing.T) {
	w := NewWorld(t, "Yetkazish (naqd, kuryer bilan)")
	const fee = int64(800_000)
	var courier *Actor
	var staffID string
	var quote, order map[string]any
	var orderID string

	w.Step(t, "mijoz restoranlar va menyuni ko'radi", func(t *testing.T) {
		w.Do(t, w.Customer, "GET", "/restaurants", nil).Expect(t, 200)
		w.Do(t, w.Customer, "GET", "/restaurants/"+RestA, nil).Expect(t, 200)
		menu := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA+"/menu", nil).Expect(t, 200)
		if len(menu.Body) < 10 {
			t.Fatalf("menyu bo'sh: %s", menu.Body)
		}
		w.Do(t, w.Customer, "GET", "/categories", nil).Expect(t, 200)
		w.Do(t, w.Customer, "GET", "/products/search?q=osh", nil).Expect(t, 200)
		w.Do(t, w.Customer, "GET", "/restaurants/"+RestA+"/active-promotions", nil).Expect(t, 200)
	})

	w.Step(t, "kuryersiz: yetkazish BEPUL", func(t *testing.T) {
		q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote",
			map[string]any{"items": []map[string]any{{"product_id": ProdOsh, "qty": 1}}}).Expect(t, 200).Map(t)
		w.CheckQuote(t, q, false)
		if num(q, "delivery_fee_tiyin") != 0 || num(q, "total_tiyin") != ProdOshPrice {
			t.Fatalf("kuryersiz haq 0 bo'lishi kerak: %v", q)
		}
	})

	w.Step(t, "restoran 'buyurtmadan maosh'li kuryer qo'shadi", func(t *testing.T) {
		courier, staffID = addCourier(t, w, "+998901110001", fmt.Sprintf(`,"delivery_pay_tiyin":%d`, fee))
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/staff", nil).Expect(t, 200)
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/staff/"+staffID+"/activity", nil).Expect(t, 200)
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/staff/report?month="+time.Now().Format("2006-01"), nil).Expect(t, 200)
	})

	w.Step(t, "mijoz narxlaydi: yetkazish haqi qo'shiladi", func(t *testing.T) {
		quote = w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote",
			map[string]any{"items": []map[string]any{{"product_id": ProdOsh, "qty": 1}}}).Expect(t, 200).Map(t)
		w.CheckQuote(t, quote, false)
		if num(quote, "delivery_fee_tiyin") != fee || num(quote, "total_tiyin") != ProdOshPrice+fee {
			t.Fatalf("haq %d, jami %d kutilgan: %v", fee, ProdOshPrice+fee, quote)
		}
	})

	w.Step(t, "buyurtma beriladi (ekrandagi jami bilan)", func(t *testing.T) {
		order = w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items":                []map[string]any{{"product_id": ProdOsh, "qty": 1}},
			"payment_method":       "cash",
			"idempotency_key":      "e2e-delivery-1",
			"expected_total_tiyin": num(quote, "total_tiyin"),
		}).Expect(t, 201).Map(t)
		orderID, _ = order["id"].(string)
		w.CheckOrder(t, order)
		w.CheckOrderMatchesQuote(t, quote, order)
	})

	w.Step(t, "takroriy so'rov dublikat buyurtma YARATMAYDI (idempotentlik)", func(t *testing.T) {
		again := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items":           []map[string]any{{"product_id": ProdOsh, "qty": 1}},
			"payment_method":  "cash",
			"idempotency_key": "e2e-delivery-1",
		}).Map(t)
		if again["id"] != orderID {
			t.Fatalf("ikkinchi buyurtma yaratildi: %v != %v", again["id"], orderID)
		}
	})

	w.Step(t, "ekrandagi jami o'zgarsa buyurtma 409 bilan to'xtatiladi", func(t *testing.T) {
		r := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items":                []map[string]any{{"product_id": ProdOsh, "qty": 1}},
			"payment_method":       "cash",
			"idempotency_key":      "e2e-delivery-stale",
			"expected_total_tiyin": ProdOshPrice, // yetkazish haqisiz — eskirgan
		})
		r.Expect(t, 409)
	})

	w.Step(t, "begona foydalanuvchilar buyurtmani KO'RA OLMAYDI", func(t *testing.T) {
		w.ExpectDenied(t, w.Other, "GET", "/orders/"+orderID, nil)
		w.ExpectDenied(t, w.OwnerB, "GET", "/orders/"+orderID, nil)
		w.ExpectDenied(t, w.OwnerB, "POST", "/orders/"+orderID+"/transition",
			map[string]any{"to": "accepted", "preparation_minutes": 5})
		w.ExpectDenied(t, nil, "GET", "/orders/"+orderID, nil)
	})

	w.Step(t, "kuryer ishga chiqadi", func(t *testing.T) {
		w.Do(t, courier, "GET", "/couriers/"+courier.Entity, nil).Expect(t, 200)
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/location",
			map[string]any{"lat": ChustLat, "lng": ChustLng}).Expect(t, 200)
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/available",
			map[string]any{"available": true}).Expect(t, 200)
	})

	w.Step(t, "restoran qabul qiladi — kuryerga taklif ketadi", func(t *testing.T) {
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/orders", nil).Expect(t, 200)
		w.Do(t, w.Owner, "POST", "/orders/"+orderID+"/transition",
			map[string]any{"to": "accepted", "preparation_minutes": 1}).Expect(t, 200)

		w.Eventually(t, "kuryerga taklif keldi", 8*time.Second, func() bool {
			m := w.Do(t, courier, "GET", "/couriers/"+courier.Entity+"/offer", nil).Map(t)
			return m["offer"] != nil
		})
		if w.OffersTo(courier.Entity) == 0 {
			t.Fatal("taklif kuryerga yuborilmadi (bildirishnoma kanali jim)")
		}
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/respond",
			map[string]any{"order_id": orderID, "accepted": true}).Expect(t, 200)

		w.Eventually(t, "kuryer buyurtmaga biriktirildi", 8*time.Second, func() bool {
			o := w.Do(t, w.Customer, "GET", "/orders/"+orderID, nil).Map(t)
			return o["courier_id"] == courier.Entity
		})
		active := w.Do(t, courier, "GET", "/couriers/"+courier.Entity+"/active-order", nil).Expect(t, 200).Map(t)
		if active["order_id"] != orderID {
			t.Fatalf("kuryerning faol buyurtmasi %v, %s kutilgan", active["order_id"], orderID)
		}
	})

	w.Step(t, "tayyorlash, olib ketish, yetkazish", func(t *testing.T) {
		step := func(a *Actor, to string) {
			t.Helper()
			o := w.Do(t, a, "POST", "/orders/"+orderID+"/transition", map[string]any{"to": to}).Expect(t, 200).Map(t)
			if o["status"] != to {
				t.Fatalf("holat %v, %s kutilgan", o["status"], to)
			}
			w.CheckOrder(t, o)
		}
		step(w.Owner, "preparing")
		step(w.Owner, "ready")
		step(courier, "picked_up")
		w.Do(t, w.Customer, "GET", "/orders/"+orderID+"/tracking", nil)
		step(courier, "delivered")
	})

	w.Step(t, "tugagach: tarix, kuryer bo'sh, qayta o'tish yo'q", func(t *testing.T) {
		mine := w.Do(t, w.Customer, "GET", "/me/orders", nil).Expect(t, 200)
		if len(mine.Body) < 10 {
			t.Fatalf("buyurtmalarim bo'sh: %s", mine.Body)
		}
		final := w.Do(t, w.Customer, "GET", "/orders/"+orderID, nil).Expect(t, 200).Map(t)
		w.CheckOrder(t, final)
		if final["status"] != "delivered" {
			t.Fatalf("yakuniy holat %v", final["status"])
		}
		// Terminal holatdan chiqib bo'lmaydi.
		r := w.Do(t, w.Owner, "POST", "/orders/"+orderID+"/transition", map[string]any{"to": "preparing"})
		if r.Status < 400 || r.Status >= 500 {
			t.Fatalf("yakunlangan buyurtma qayta ochildi: %d", r.Status)
		}
		c := w.Do(t, courier, "GET", "/couriers/"+courier.Entity, nil).Expect(t, 200).Map(t)
		if c["available"] != true {
			t.Fatalf("kuryer yetkazgandan keyin bo'sh bo'lmadi: %v", c)
		}
		// Statistika: bugungi yetkazilgan buyurtma hisobga olinadi.
		today := time.Now().Format("2006-01-02")
		st := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/stats?from="+today+"&to="+today+"&granularity=day", nil).Expect(t, 200)
		if len(st.Body) < 20 {
			t.Fatalf("statistika bo'sh: %s", st.Body)
		}
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/stats?from=2026-13-40&to="+today, nil).Expect(t, 400)
		w.ExpectDenied(t, w.OwnerB, "GET", "/restaurants/"+RestA+"/stats?from="+today+"&to="+today, nil)
		hist := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/orders/history", nil).Expect(t, 200)
		if !contains(hist.Body, orderID) {
			t.Fatalf("yetkazilgan buyurtma tarixda yo'q: %s", truncate(string(hist.Body), 200))
		}
	})
}
