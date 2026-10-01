package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"chustapp/internal/orders"
)

// ssenariylar: BUYURTMA HAYOT SIKLI — bekor qilish, rad etish, pul qaytishi,
// kuryer rad etishi, "kuryer topilmadi" nazoratchisi, kirish tekshiruvlari va hamyon.

// ─── Bekor qilish va rad etish ─────────────────────────────────────────

func TestOrderCancelAndReject(t *testing.T) {
	w := NewWorld(t, "Buyurtmani bekor qilish va rad etish")
	var o1, o2, o3, o4 string

	w.Step(t, "mijoz yangi buyurtmani bekor qiladi; restoran endi qabul qila olmaydi", func(t *testing.T) {
		o1, _ = w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "cash", "e2e-c-1")["id"].(string)
		w.Transition(t, w.Customer, o1, "cancelled")
		w.TransitionRejected(t, w.Owner, o1, "accepted")
		// Yakunlangan buyurtmani qayta bekor qilib bo'lmaydi.
		w.TransitionRejected(t, w.Customer, o1, "cancelled")
	})

	w.Step(t, "qabul qilingach mijoz bekor qila oladi, tayyorlanayotganda — YO'Q", func(t *testing.T) {
		o2, _ = w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "cash", "e2e-c-2")["id"].(string)
		w.Accept(t, o2)
		w.Transition(t, w.Owner, o2, "preparing")
		// Taom pishirilmoqda: mijoz endi bekor qila olmaydi (faqat admin).
		w.TransitionRejected(t, w.Customer, o2, "cancelled")
		w.ExpectDenied(t, w.Other, "POST", "/orders/"+o2+"/transition", map[string]any{"to": "cancelled"})
	})

	w.Step(t, "admin tayyorlanayotgan buyurtmani bekor qila oladi", func(t *testing.T) {
		w.Transition(t, w.Admin, o2, "cancelled")
	})

	w.Step(t, "restoran yangi buyurtmani rad etadi; mijoz holatni ko'radi", func(t *testing.T) {
		o3, _ = w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "cash", "e2e-c-3")["id"].(string)
		w.Transition(t, w.Owner, o3, "rejected")
		got := w.Do(t, w.Customer, "GET", "/orders/"+o3, nil).Expect(t, 200).Map(t)
		if got["status"] != "rejected" {
			t.Fatalf("mijoz holati: %v", got["status"])
		}
		w.TransitionRejected(t, w.Owner, o3, "accepted")
	})

	w.Step(t, "qabul qilingan buyurtmani mijoz bekor qiladi", func(t *testing.T) {
		o4, _ = w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "cash", "e2e-c-4")["id"].(string)
		w.Accept(t, o4)
		w.Transition(t, w.Customer, o4, "cancelled")
	})

	w.Step(t, "kuryerni boshqa o'tkaza olmaydi: mijoz 'delivered' qila olmaydi", func(t *testing.T) {
		o5, _ := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "cash", "e2e-c-5")["id"].(string)
		for _, to := range []string{"picked_up", "delivered", "ready", "preparing"} {
			w.TransitionRejected(t, w.Customer, o5, to)
		}
	})
}

// ─── Pul qaytishi ──────────────────────────────────────────────────────

func TestRefundsOnRejectAndCancel(t *testing.T) {
	w := NewWorld(t, "To'lov qaytishi (rad etish, bekor qilish)")
	var p1, p2 string
	reversals := func() int { return w.Gateway.ReversalCount() }

	w.Step(t, "to'langan buyurtmani restoran rad etsa — pul QAYTADI", func(t *testing.T) {
		o := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "card", "e2e-r-1")
		id, _ := o["id"].(string)
		p1 = w.PayCard(t, w.Customer, id)
		before := reversals()
		w.Transition(t, w.Owner, id, "rejected")
		w.Eventually(t, "to'lov qaytarildi", 3*time.Second, func() bool { return w.PaymentStatus(t, p1) == "refunded" })
		Rec2 := reversals() - before
		w.R().Checked("money.rejected_paid_order_is_refunded")
		if Rec2 != 1 {
			w.Violate(t, "money.rejected_paid_order_is_refunded", "shlyuzga %d ta qaytarish yuborildi, 1 kutilgan", Rec2)
		}
	})

	w.Step(t, "to'langan buyurtmani mijoz (qabuldan keyin) bekor qilsa — pul qaytadi, IKKI MARTA emas", func(t *testing.T) {
		o := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "card", "e2e-r-2")
		id, _ := o["id"].(string)
		p2 = w.PayCard(t, w.Customer, id)
		w.Accept(t, id)
		before := reversals()
		w.Transition(t, w.Customer, id, "cancelled")
		w.Eventually(t, "to'lov qaytarildi", 3*time.Second, func() bool { return w.PaymentStatus(t, p2) == "refunded" })
		// Qayta bekor qilish rad etiladi va pulni yana qaytarmaydi.
		w.TransitionRejected(t, w.Customer, id, "cancelled")
		w.R().Checked("money.no_double_refund")
		if got := reversals() - before; got != 1 {
			w.Violate(t, "money.no_double_refund", "qaytarishlar soni %d, 1 kutilgan", got)
		}
	})

	w.Step(t, "to'lov boshlangan, lekin to'lanmagan buyurtma bekor qilinsa — pul yechilmagan", func(t *testing.T) {
		o := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "card", "e2e-r-3")
		id, _ := o["id"].(string)
		w.Do(t, w.Customer, "POST", "/orders/"+id+"/pay", nil).Expect(t, 200)
		before, reversalsBefore := w.Gateway.InvoiceCancelCount(), w.Gateway.ReversalCount()
		w.Transition(t, w.Customer, id, "cancelled")
		// To'lanmagan invoys PROVAYDERDA ham yopilishi shart: aks holda mijoz havolani
		// keyin to'lasa pul yechiladi, buyurtma esa yo'q (Multicard sandbox'da topilgan).
		w.R().Checked("money.unpaid_cancel_closes_invoice_at_provider")
		if got := w.Gateway.InvoiceCancelCount() - before; got != 1 {
			w.Violate(t, "money.unpaid_cancel_closes_invoice_at_provider",
				"to'lanmagan invoys provayderda yopilmadi: bekor qilish so'rovlari %d, 1 kutilgan", got)
		}
		// To'lanmagan invoys QAYTARISH emas, BEKOR QILISH bilan yopiladi (pul yechilmagan edi).
		w.R().Checked("money.unpaid_cancel_is_not_a_refund")
		if got := w.Gateway.ReversalCount() - reversalsBefore; got != 0 {
			w.Violate(t, "money.unpaid_cancel_is_not_a_refund", "to'lanmagan invoys uchun %d ta QAYTARISH so'rovi ketdi", got)
		}
		w.R().Checked("money.cancel_uses_correct_endpoint")
		if n := w.Gateway.MisroutedCount(); n != 0 {
			w.Violate(t, "money.cancel_uses_correct_endpoint", "shlyuzga noto'g'ri yo'lga %d ta so'rov ketdi", n)
		}
		list, _ := w.Payments.ListByOrder(context.Background(), id)
		for _, p := range list {
			if st := string(p.Status); st == "paid" || st == "refunded" {
				w.Violate(t, "money.unpaid_cancel_has_no_charge", "to'lanmagan buyurtma to'lovi `%s`", st)
			}
		}
		// Bekor qilingan buyurtmaga endi to'lab bo'lmaydi.
		w.Do(t, w.Customer, "POST", "/orders/"+id+"/pay", nil).Expect(t, 400)
	})
}

// ─── Kuryer taklifni rad etishi ────────────────────────────────────────

func TestCourierDeclinePassesOffer(t *testing.T) {
	w := NewWorldWith(t, "Kuryer taklifni rad etadi / ikki kuryer", Options{OfferTTL: 5 * time.Second})
	var a, b *Actor
	var orderID string

	w.Step(t, "ikki kuryer smenada; buyurtma qabul qilinadi — ikkalasiga taklif", func(t *testing.T) {
		a, _ = addCourier(t, w, "+998901110051", `,"delivery_pay_tiyin":800000`)
		b, _ = addCourier(t, w, "+998901110052", `,"delivery_pay_tiyin":800000`)
		w.ShiftOn(t, a)
		w.ShiftOn(t, b)
		orderID, _ = w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "cash", "e2e-d-1")["id"].(string)
		w.Accept(t, orderID)
		for _, c := range []*Actor{a, b} {
			w.Eventually(t, c.Name+" taklif oldi", 6*time.Second, func() bool {
				return w.Do(t, c, "GET", "/couriers/"+c.Entity+"/offer", nil).Map(t)["offer"] != nil
			})
		}
	})

	w.Step(t, "A rad etadi, B qabul qiladi — buyurtma B ga biriktiriladi", func(t *testing.T) {
		w.Do(t, a, "POST", "/couriers/"+a.Entity+"/respond", map[string]any{"order_id": orderID, "accepted": false}).Expect(t, 200)
		w.AcceptOffer(t, b, orderID)
		o := w.Do(t, w.Owner, "GET", "/orders/"+orderID, nil).Map(t)
		if o["courier_id"] != b.Entity {
			t.Fatalf("buyurtma kuryeri %v, B (%s) kutilgan", o["courier_id"], b.Entity)
		}
	})

	w.Step(t, "kech qolgan javob: A endi qabul qila olmaydi (409), taklifi yo'qoladi", func(t *testing.T) {
		w.Do(t, a, "POST", "/couriers/"+a.Entity+"/respond", map[string]any{"order_id": orderID, "accepted": true}).Expect(t, 409)
		w.Eventually(t, "A ning taklifi yopildi", 3*time.Second, func() bool {
			return w.Do(t, a, "GET", "/couriers/"+a.Entity+"/offer", nil).Map(t)["offer"] == nil
		})
		// Begona buyurtmani kuryer o'tkaza olmaydi: A buyurtmani 'picked_up' qila olmaydi.
		w.TransitionRejected(t, a, orderID, "picked_up")
	})

	w.Step(t, "B yetkazadi; A hali ham smenada (bo'sh)", func(t *testing.T) {
		w.Deliver(t, b, orderID)
		ca := w.Do(t, a, "GET", "/couriers/"+a.Entity, nil).Expect(t, 200).Map(t)
		if ca["available"] != true {
			t.Fatalf("rad etgan kuryer smenadan chiqib ketdi: %v", ca)
		}
	})
}

// ─── "Kuryer topilmadi" nazoratchisi ───────────────────────────────────

func TestCourierNotFoundWatchdog(t *testing.T) {
	w := NewWorld(t, "Kuryer topilmadi: nazoratchi, qayta qidirish, avto-bekor, pul qaytishi")
	var id, paymentID string

	w.Step(t, "to'langan buyurtma tayyor, lekin kuryer YO'Q", func(t *testing.T) {
		o := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "card", "e2e-nf-1")
		id, _ = o["id"].(string)
		paymentID = w.PayCard(t, w.Customer, id)
		w.Accept(t, id)
		w.Transition(t, w.Owner, id, "preparing")
		w.Transition(t, w.Owner, id, "ready")
		got := w.Do(t, w.Owner, "GET", "/orders/"+id, nil).Map(t)
		if got["dispatch_state"] != "searching" {
			t.Fatalf("qidiruv holati %v, searching kutilgan", got["dispatch_state"])
		}
	})

	w.Step(t, "muddat o'tgach nazoratchi 'kuryer topilmadi' qiladi", func(t *testing.T) {
		// Muddat hali o'tmagan — nazoratchi tegmaydi.
		w.Sweep(t)
		if st := w.Do(t, w.Owner, "GET", "/orders/"+id, nil).Map(t)["dispatch_state"]; st != "searching" {
			t.Fatalf("muddatdan oldin holat o'zgardi: %v", st)
		}
		w.Advance(orders.CourierSearchWindow + time.Minute)
		w.Sweep(t)
		if st := w.Do(t, w.Owner, "GET", "/orders/"+id, nil).Map(t)["dispatch_state"]; st != "not_found" {
			t.Fatalf("dispatch_state %v, not_found kutilgan", st)
		}
		// "Topilmadi" holatida restoran javobini KUTADI: bekor qilish muddati o'tmaguncha
		// nazoratchi buyurtmani bekor QILMAYDI (hatto qayta-qayta tekshirsa ham).
		w.Sweep(t)
		w.Advance(orders.CourierNotFoundCancelAfter - time.Minute)
		w.Sweep(t)
		w.R().Checked("state.not_found_waits_for_restaurant_decision")
		if got := w.Do(t, w.Owner, "GET", "/orders/"+id, nil).Map(t); got["status"] != "ready" || got["dispatch_state"] != "not_found" {
			w.Violate(t, "state.not_found_waits_for_restaurant_decision", "muddatdan oldin holat: %v / %v", got["status"], got["dispatch_state"])
		}
		// Restoranga bildirishnoma tushgan.
		n := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/notifications", nil).Expect(t, 200)
		if !strings.Contains(strings.ToLower(string(n.Body)), "kuryer") {
			t.Fatalf("restoranga 'kuryer topilmadi' bildirishnomasi yo'q: %s", truncate(string(n.Body), 300))
		}
	})

	w.Step(t, "restoran 'qayta qidirish' bosadi — qidiruv YANGI muddat bilan boshlanadi", func(t *testing.T) {
		w.ExpectDenied(t, w.OwnerB, "POST", "/orders/"+id+"/dispatch/retry", nil)
		r := w.Do(t, w.Owner, "POST", "/orders/"+id+"/dispatch/retry", nil).Expect(t, 200).Map(t)
		if r["dispatch_state"] != "searching" {
			t.Fatalf("retry'dan keyin holat %v", r["dispatch_state"])
		}
		// Ikkinchi marta bosish: qidiruv allaqachon ketyapti — 409.
		w.Do(t, w.Owner, "POST", "/orders/"+id+"/dispatch/retry", nil).Expect(t, 409)
	})

	w.Step(t, "javob bo'lmasa tizim bekor qiladi va pul QAYTADI", func(t *testing.T) {
		w.Advance(orders.CourierSearchWindow + time.Minute)
		w.Sweep(t) // not_found
		before := w.Gateway.ReversalCount()
		w.Advance(orders.CourierNotFoundCancelAfter + time.Minute)
		w.Sweep(t) // avto-bekor
		w.Eventually(t, "buyurtma tizim tomonidan bekor qilindi", 3*time.Second, func() bool {
			return w.Do(t, w.Owner, "GET", "/orders/"+id, nil).Map(t)["status"] == "cancelled"
		})
		w.Eventually(t, "to'lov qaytarildi", 3*time.Second, func() bool { return w.PaymentStatus(t, paymentID) == "refunded" })
		w.R().Checked("money.auto_cancel_refunds_paid_order")
		if got := w.Gateway.ReversalCount() - before; got != 1 {
			w.Violate(t, "money.auto_cancel_refunds_paid_order", "qaytarishlar soni %d, 1 kutilgan", got)
		}
		final := w.Do(t, w.Customer, "GET", "/orders/"+id, nil).Map(t)
		w.CheckOrder(t, final)
	})
}

// ─── Buyurtma yaratishdagi tekshiruvlar ────────────────────────────────

func TestOrderValidation(t *testing.T) {
	w := NewWorld(t, "Buyurtma yaratish: kirish tekshiruvlari")
	bad := func(name string, body any) {
		t.Helper()
		r := w.Do(t, w.Customer, "POST", "/orders", body)
		w.R().Checked("http.invalid_order_input_is_rejected")
		if r.Status < 400 || r.Status >= 500 {
			w.Violate(t, "http.invalid_order_input_is_rejected", "%s: %d — %s", name, r.Status, truncate(string(r.Body), 120))
		}
	}
	items := func(pid string, qty any) []map[string]any { return []map[string]any{{"product_id": pid, "qty": qty}} }

	w.Step(t, "noto'g'ri savat va maydonlar rad etiladi", func(t *testing.T) {
		bad("bo'sh savat", map[string]any{"items": []any{}, "payment_method": "cash"})
		bad("items yo'q", map[string]any{"payment_method": "cash"})
		bad("miqdor 0", map[string]any{"items": items(ProdOsh, 0), "payment_method": "cash"})
		bad("miqdor manfiy", map[string]any{"items": items(ProdOsh, -3), "payment_method": "cash"})
		bad("noma'lum taom", map[string]any{"items": items("yoq-taom", 1), "payment_method": "cash"})
		bad("ikki restoran", map[string]any{"items": []map[string]any{
			{"product_id": ProdOsh, "qty": 1}, {"product_id": ProdB, "qty": 1}}, "payment_method": "cash"})
		bad("noma'lum to'lov usuli", map[string]any{"items": items(ProdOsh, 1), "payment_method": "bitcoin"})
		bad("uzun idempotency_key", map[string]any{"items": items(ProdOsh, 1), "payment_method": "cash",
			"idempotency_key": strings.Repeat("k", 129)})
		bad("JSON emas", "bu json emas")
		bad("soxta stol tokeni", map[string]any{"items": items(ProdOsh, 1), "table_token": "soxta-token"})
		bad("miqdor matn", map[string]any{"items": items(ProdOsh, "ko'p"), "payment_method": "cash"})
		bad("miqdor chegaradan katta (101)", map[string]any{"items": items(ProdOsh, 101), "payment_method": "cash"})
		// Hech biri buyurtma yaratmagan.
		mine := w.Do(t, w.Customer, "GET", "/me/orders", nil).Expect(t, 200)
		if contains(mine.Body, `"id"`) {
			t.Fatalf("rad etilgan so'rovlar buyurtma yaratgan: %s", truncate(string(mine.Body), 200))
		}
	})

	w.Step(t, "miqdor chegarasi: 100 o'tadi", func(t *testing.T) {
		o := w.PlaceOrder(t, w.Customer, items(ProdOsh, 100), "cash", "e2e-v-100")
		if num(o, "subtotal_tiyin") != 100*ProdOshPrice {
			t.Fatalf("100 dona jami %d", num(o, "subtotal_tiyin"))
		}
	})

	w.Step(t, "tokensiz buyurtma bermaydi", func(t *testing.T) {
		w.ExpectDenied(t, nil, "POST", "/orders", map[string]any{"items": items(ProdOsh, 1)})
		w.ExpectDenied(t, nil, "POST", "/restaurants/"+RestA+"/quote", map[string]any{"items": items(ProdOsh, 1)})
	})

	w.Step(t, "mavjud bo'lmagan (nofaol) taomga buyurtma berib bo'lmaydi", func(t *testing.T) {
		p := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/products", map[string]any{
			"name": "Tugagan taom", "category": "Asosiy", "price_tiyin": 1_000_000, "available": false,
		}).Expect(t, 201).Map(t)
		pid, _ := p["id"].(string)
		bad("nofaol taom", map[string]any{"items": items(pid, 1), "payment_method": "cash"})
		w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote", map[string]any{"items": items(pid, 1)}).Expect(t, 400)
	})

	w.Step(t, "manzilsiz mijoz yetkazish buyurtma bera olmaydi", func(t *testing.T) {
		if err := w.Users.UpdateAddress(context.Background(), w.Other.UserID, emptyAddress()); err != nil {
			t.Fatal(err)
		}
		r := w.Do(t, w.Other, "POST", "/orders", map[string]any{"items": items(ProdOsh, 1), "payment_method": "cash"})
		if r.Status != 400 || !strings.Contains(string(r.Body), "manzil") {
			t.Fatalf("manzilsiz buyurtma: %d %s", r.Status, r.Body)
		}
	})

	w.Step(t, "restoran ish vaqtidan tashqarida — buyurtma qabul qilinmaydi", func(t *testing.T) {
		days := make([]map[string]any, 0, 7)
		for d := 1; d <= 7; d++ {
			days = append(days, map[string]any{"day": d, "enabled": false, "open": "08:00", "close": "23:00"})
		}
		w.Do(t, w.Owner, "PATCH", "/restaurants/"+RestA+"/settings",
			map[string]any{"working_hours": map[string]any{"days": days}}).Expect(t, 200)
		bad("ish vaqtidan tashqari", map[string]any{"items": items(ProdOsh, 1), "payment_method": "cash"})
		// Cheklov olib tashlansa buyurtma yana o'tadi.
		w.Do(t, w.Owner, "PATCH", "/restaurants/"+RestA+"/settings", map[string]any{"working_hours": nil}).Expect(t, 200)
		w.PlaceOrder(t, w.Customer, items(ProdOsh, 1), "cash", "e2e-v-open")
	})

	w.Step(t, "restoran karta qabul qilmasa — karta buyurtmasi rad etiladi", func(t *testing.T) {
		w.Do(t, w.Owner, "PATCH", "/restaurants/"+RestA+"/settings", payBody(map[string]any{
			"cash": true, "card_terminal": false, "card_online": false})).Expect(t, 200)
		bad("karta o'chiq", map[string]any{"items": items(ProdOsh, 1), "payment_method": "card"})
	})
}

// ─── Hamyon ────────────────────────────────────────────────────────────

func TestWalletFlow(t *testing.T) {
	w := NewWorld(t, "OnDex Wallet (keshbek, sarflash, qaytarish)")
	ctx := context.Background()
	var courier *Actor
	balance := func() int64 {
		w.polling = true // tekshiruv o'qishi
		defer func() { w.polling = false }()
		m := w.Do(t, w.Customer, "GET", "/wallet", nil).Expect(t, 200).Map(t)
		return num(m, "balance_tiyin")
	}

	w.Step(t, "yangi hamyon bo'sh; sarflash chegarasidan past — wallet bilan to'lab bo'lmaydi", func(t *testing.T) {
		if got := balance(); got != 0 {
			t.Fatalf("yangi hamyon balansi %d", got)
		}
		r := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": Cart(ProdOsh, 1), "payment_method": "wallet", "idempotency_key": "e2e-w-0"})
		w.R().Checked("money.wallet_below_threshold_rejected")
		if r.Status < 400 || r.Status >= 500 {
			w.Violate(t, "money.wallet_below_threshold_rejected", "bo'sh hamyon bilan buyurtma: %d", r.Status)
		}
		if balance() != 0 {
			w.Violate(t, "money.wallet_below_threshold_rejected", "rad etilgan buyurtma balansni o'zgartirdi")
		}
	})

	w.Step(t, "yetkazilgan buyurtma uchun keshbek AYNAN 1%", func(t *testing.T) {
		courier, _ = addCourier(t, w, "+998901110061", `,"delivery_pay_tiyin":800000`)
		w.ShiftOn(t, courier)
		o := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 2), "cash", "e2e-w-1") // 70 000 so'm + yetkazish
		id, _ := o["id"].(string)
		total := num(o, "total_tiyin")
		w.Accept(t, id)
		w.AcceptOffer(t, courier, id)
		w.Deliver(t, courier, id)
		w.Eventually(t, "keshbek tushdi", 3*time.Second, func() bool { return balance() > 0 })
		w.R().Checked("money.cashback_is_one_percent")
		if want := total * 1 / 100; balance() != want {
			w.Violate(t, "money.cashback_is_one_percent", "keshbek %d, jami %d ning 1%% i (%d) kutilgan", balance(), total, want)
		}
	})

	w.Step(t, "50 000 so'mdan kam buyurtmaga keshbek YO'Q", func(t *testing.T) {
		before := balance()
		o := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "cash", "e2e-w-2") // 35 000 + haq = 43 000 < 50 000
		id, _ := o["id"].(string)
		w.Accept(t, id)
		w.AcceptOffer(t, courier, id)
		w.Deliver(t, courier, id)
		time.Sleep(100 * time.Millisecond)
		if balance() != before {
			w.Violate(t, "money.cashback_is_one_percent", "kichik buyurtmaga keshbek berildi: %d -> %d", before, balance())
		}
	})

	var spent int64
	w.Step(t, "hamyon bilan to'lov: balansdan AYNAN buyurtma jami yechiladi", func(t *testing.T) {
		// 100 000 so'm qo'shamiz (keshbek mexanizmi bilan: 1% => 10 mln so'm buyurtma).
		if err := w.Wallet.CreditCashback(ctx, "seed-topup", w.Customer.UserID, 1_000_000_000); err != nil {
			t.Fatal(err)
		}
		before := balance()
		o := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "wallet", "e2e-w-3")
		spent = num(o, "total_tiyin")
		w.R().Checked("money.wallet_spend_equals_order_total")
		if got := before - balance(); got != spent {
			w.Violate(t, "money.wallet_spend_equals_order_total", "balansdan %d yechildi, buyurtma jami %d", got, spent)
		}
		id, _ := o["id"].(string)

		// Restoran rad etsa — to'liq qaytadi.
		mid := balance()
		w.Transition(t, w.Owner, id, "rejected")
		w.R().Checked("money.wallet_refund_restores_balance")
		if got := balance() - mid; got != spent {
			w.Violate(t, "money.wallet_refund_restores_balance", "qaytgan %d, yechilgan %d", got, spent)
		}
	})

	w.Step(t, "balans buyurtmani qoplamasa — rad etiladi, balans o'zgarmaydi", func(t *testing.T) {
		before := balance()
		r := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": Cart(ProdOsh, 40), "payment_method": "wallet", "idempotency_key": "e2e-w-4"}) // 1.4 mln so'm
		w.R().Checked("money.wallet_insufficient_rejected")
		if r.Status < 400 || r.Status >= 500 {
			w.Violate(t, "money.wallet_insufficient_rejected", "yetarsiz balans bilan buyurtma: %d", r.Status)
		}
		if balance() != before {
			w.Violate(t, "money.wallet_insufficient_rejected", "balans o'zgardi: %d -> %d", before, balance())
		}
	})

	w.Step(t, "begona mijozning hamyoni alohida", func(t *testing.T) {
		m := w.Do(t, w.Other, "GET", "/wallet", nil).Expect(t, 200).Map(t)
		if num(m, "balance_tiyin") != 0 {
			t.Fatalf("begona hamyon balansi %v", m)
		}
	})
}
