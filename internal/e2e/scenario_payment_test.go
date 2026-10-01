package e2e

import (
	"testing"

	"chustapp/internal/payments/multicard"
)

// ssenariy: ONLAYN TO'LOV — restoran yoqqan usullar, kanal tanlash,
// shlyuz callback'i va fiskal chek.

func payBody(methods map[string]any) map[string]any {
	return map[string]any{"payment_methods": methods}
}

func TestOnlinePaymentChannelsFlow(t *testing.T) {
	w := NewWorld(t, "Onlayn to'lov (Payme/Click/Uzum kanallari)")
	var cardOrderID string
	var orderTotal int64

	items := []map[string]any{{"product_id": ProdOsh, "qty": 2}}

	w.Step(t, "restoran sozlamalari: usullar ko'rinadi, Moliya ulangan", func(t *testing.T) {
		s := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/settings", nil).Expect(t, 200).Map(t)
		if s["online_payments_available"] != true {
			t.Fatalf("Moliya ulangan, lekin online_payments_available=%v", s["online_payments_available"])
		}
		pm, _ := s["payment_methods"].(map[string]any)
		if pm["card_online"] != true || pm["payme"] != false || pm["uzum"] != false {
			t.Fatalf("boshlang'ich usullar: %v", pm)
		}
	})

	w.Step(t, "mijoz faqat YOQILGAN usullarni ko'radi", func(t *testing.T) {
		opts := publicOptions(t, w)
		if opts["card"] != true || opts["cash"] != true || opts["payme"] != false || opts["click"] != false || opts["uzum"] != false {
			t.Fatalf("payment_options: %v", opts)
		}
	})

	w.Step(t, "restoran Payme va Uzum'ni yoqadi", func(t *testing.T) {
		w.Do(t, w.Owner, "PATCH", "/restaurants/"+RestA+"/settings", payBody(map[string]any{
			"cash": true, "card_terminal": false, "card_online": true,
			"payme": true, "click": false, "uzum": true,
		})).Expect(t, 200)
		opts := publicOptions(t, w)
		if opts["payme"] != true || opts["uzum"] != true || opts["click"] != false {
			t.Fatalf("yoqilgandan keyin payment_options: %v", opts)
		}
	})

	w.Step(t, "karta buyurtmasi yaratiladi — restoranga hali KO'RINMAYDI", func(t *testing.T) {
		q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote", map[string]any{"items": items}).Expect(t, 200).Map(t)
		w.CheckQuote(t, q, false)
		o := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": items, "payment_method": "card", "idempotency_key": "e2e-pay-1",
			"expected_total_tiyin": num(q, "total_tiyin"),
		}).Expect(t, 201).Map(t)
		w.CheckOrder(t, o)
		cardOrderID, _ = o["id"].(string)
		orderTotal = num(o, "total_tiyin")

		list := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/orders", nil).Expect(t, 200)
		if contains(list.Body, cardOrderID) {
			t.Fatal("to'lanmagan karta buyurtmasi restoran ro'yxatida ko'rindi")
		}
		// To'lanmagan buyurtmani restoran qabul qila olmaydi.
		r := w.Do(t, w.Owner, "POST", "/orders/"+cardOrderID+"/transition",
			map[string]any{"to": "accepted", "preparation_minutes": 5})
		if r.Status < 400 || r.Status >= 500 {
			t.Fatalf("to'lanmagan buyurtma qabul qilindi: %d", r.Status)
		}
	})

	w.Step(t, "o'chirilgan Click bilan to'lov RAD ETILADI", func(t *testing.T) {
		r := w.Do(t, w.Customer, "POST", "/orders/"+cardOrderID+"/pay?ps=click", nil)
		r.Expect(t, 409)
		w.Do(t, w.Customer, "POST", "/orders/"+cardOrderID+"/pay?ps=bitcoin", nil).Expect(t, 400)
		list, _ := w.Payments.ListByOrder(t.Context(), cardOrderID)
		if len(list) != 0 {
			t.Fatalf("rad etilgan urinishlar to'lov yozuvi ochgan: %d", len(list))
		}
	})

	w.Step(t, "Uzum bilan to'lov: shlyuzga to'g'ri summa va fiskal chek", func(t *testing.T) {
		p := w.Do(t, w.Customer, "POST", "/orders/"+cardOrderID+"/pay?ps=uzum", nil).Expect(t, 200).Map(t)
		if p["pay_url"] == "" || p["pay_url"] == nil {
			t.Fatalf("to'lov havolasi yo'q: %v", p)
		}
		_, direct := w.Gateway.Last()
		if direct == nil {
			t.Fatal("Uzum kanali uchun shlyuzga to'g'ridan-to'g'ri so'rov ketmadi")
		}
		if direct["payment_system"] != "uzum" || direct["store_id"] != GatewayStore {
			t.Fatalf("shlyuz so'rovi: %v", direct)
		}
		w.CheckGatewayPayment(t, direct, orderTotal)
		w.Do(t, w.Customer, "GET", "/orders/"+cardOrderID+"/payment", nil).Expect(t, 200)
	})

	w.Step(t, "soxta imzoli callback to'lovni TASDIQLAMAYDI", func(t *testing.T) {
		_, direct := w.Gateway.Last()
		invoiceID, _ := direct["invoice_id"].(string)
		cb := multicard.Callback{UUID: "gw-uuid-1", InvoiceID: invoiceID, Amount: multicard.Tiyin(orderTotal),
			Status: "success", Sign: Sign("gw-uuid-1", invoiceID, orderTotal, "boshqa-sir")}
		w.Do(t, nil, "POST", "/payments/multicard/callback", cb).Expect(t, 200)
		o := w.Do(t, w.Customer, "GET", "/orders/"+cardOrderID, nil).Expect(t, 200).Map(t)
		if st, _ := o["payment_state"].(string); st == "paid" || st == "held" {
			t.Fatalf("begona imzo bilan to'lov tasdiqlandi: %v", st)
		}
	})

	w.Step(t, "to'g'ri imzoli callback: to'lov tasdiqlanadi, restoran ko'radi", func(t *testing.T) {
		w.Gateway.mu.Lock()
		w.Gateway.StatusFails = true // yagona dalil — imzo
		w.Gateway.mu.Unlock()
		_, direct := w.Gateway.Last()
		invoiceID, _ := direct["invoice_id"].(string)
		cb := multicard.Callback{UUID: "gw-uuid-1", InvoiceID: invoiceID, Amount: multicard.Tiyin(orderTotal),
			Status: "success", Sign: Sign("gw-uuid-1", invoiceID, orderTotal, GatewaySecret)}
		w.Do(t, nil, "POST", "/payments/multicard/callback", cb).Expect(t, 200)
		// Takroriy callback buyurtmani ikki marta o'zgartirmaydi.
		w.Do(t, nil, "POST", "/payments/multicard/callback", cb).Expect(t, 200)

		o := w.Do(t, w.Customer, "GET", "/orders/"+cardOrderID, nil).Expect(t, 200).Map(t)
		w.CheckOrder(t, o)
		if st, _ := o["payment_state"].(string); st != "paid" && st != "held" {
			t.Fatalf("to'lov holati %q", st)
		}
		list := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/orders", nil).Expect(t, 200)
		if !contains(list.Body, cardOrderID) {
			t.Fatal("to'langan buyurtma restoranga ko'rinmadi")
		}
		w.Do(t, w.Customer, "POST", "/orders/"+cardOrderID+"/pay", nil).Expect(t, 409) // allaqachon to'langan
	})

	w.Step(t, "restoran Uzum'ni o'chirdi — mijozga yashiriladi, yangi to'lov rad etiladi", func(t *testing.T) {
		w.Do(t, w.Owner, "PATCH", "/restaurants/"+RestA+"/settings", payBody(map[string]any{
			"cash": true, "card_terminal": false, "card_online": true,
			"payme": true, "click": false, "uzum": false,
		})).Expect(t, 200)
		if opts := publicOptions(t, w); opts["uzum"] != false || opts["payme"] != true {
			t.Fatalf("Uzum o'chirilgach: %v", opts)
		}
		o2 := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": items, "payment_method": "card", "idempotency_key": "e2e-pay-2",
		}).Expect(t, 201).Map(t)
		id2, _ := o2["id"].(string)
		w.Do(t, w.Customer, "POST", "/orders/"+id2+"/pay?ps=uzum", nil).Expect(t, 409)
		w.Do(t, w.Customer, "POST", "/orders/"+id2+"/pay?ps=payme", nil).Expect(t, 200)
	})

	w.Step(t, "begona mijoz boshqa birovning buyurtmasiga to'lov OCHA OLMAYDI", func(t *testing.T) {
		w.ExpectDenied(t, w.Other, "POST", "/orders/"+cardOrderID+"/pay", nil)
		w.ExpectDenied(t, w.Other, "GET", "/orders/"+cardOrderID+"/payment", nil)
	})
}

// publicOptions — mijozga OCHIQ javobdagi payment_options.
func publicOptions(t *testing.T, w *World) map[string]any {
	t.Helper()
	r := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA, nil).Expect(t, 200).Map(t)
	opts, _ := r["payment_options"].(map[string]any)
	if opts == nil {
		t.Fatalf("ochiq javobda payment_options yo'q: %v", r)
	}
	return opts
}

func contains(body []byte, s string) bool {
	return len(s) > 0 && stringsContains(string(body), s)
}
