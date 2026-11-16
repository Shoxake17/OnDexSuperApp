package e2e

import (
	"context"
	"net/url"
	"testing"
	"time"

	"chustapp/internal/payments/multicard"
)

// RS-5: stolda hisobni bo'lish — QR token bilan hisob, o'z taomi (karta +
// choy puli), bir taomni ikki marta to'lab bo'lmaydi, teng ulush, ofitsiant
// qolganini naqd yopadi, begona restoran/ofitsiant izolyatsiyasi.
func TestTableBillFlow(t *testing.T) {
	w := NewWorld(t, "Stolda hisobni bo'lish")
	ctx := context.Background()
	var token, tableID, billID string

	payShare := func(t *testing.T, shareID string) {
		t.Helper()
		list, err := w.Payments.ListByBillShare(ctx, shareID)
		if err != nil || len(list) == 0 {
			t.Fatalf("ulush to'lovi yo'q: %v", err)
		}
		p := list[0]
		w.Gateway.mu.Lock()
		w.Gateway.StatusFails = true
		w.Gateway.mu.Unlock()
		w.Do(t, nil, "POST", "/payments/multicard/callback", multicard.Callback{
			UUID: p.ProviderPaymentID, InvoiceID: p.ID, Amount: multicard.Tiyin(p.AmountTiyin), Status: "success",
			Sign: Sign(p.ProviderPaymentID, p.ID, p.AmountTiyin, GatewaySecret),
		}).Expect(t, 200)
	}

	w.Step(t, "stol va ikki mehmon buyurtmasi", func(t *testing.T) {
		tb := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/tables",
			map[string]any{"label": "7", "zone": "Zal", "kind": "table", "capacity": 4}).Expect(t, 201).Map(t)
		tableID, _ = tb["id"].(string)
		token, _ = tb["qr_token"].(string)
		for i, who := range []*Actor{w.Customer, w.Other} {
			qty := 2 - i // mijoz 2 osh, begona 1 osh
			o := w.Do(t, who, "POST", "/orders", map[string]any{
				"items": []map[string]any{{"product_id": ProdOsh, "qty": qty}}, "table_token": token,
				"party_size": 2, "payment_method": "cash", "idempotency_key": "e2e-bill-" + who.Name,
			}).Expect(t, 201).Map(t)
			w.CheckOrder(t, o)
		}
	})

	w.Step(t, "mehmon QR orqali hisobni ko'radi", func(t *testing.T) {
		w.Do(t, w.Customer, "GET", "/tables/bill?token=soxta", nil).Expect(t, 404)
		w.ExpectDenied(t, w.Owner, "GET", "/tables/bill?token="+token, nil)
		v := w.Do(t, w.Customer, "GET", "/tables/bill?token="+token, nil).Expect(t, 200).Map(t)
		billID = v["bill"].(map[string]any)["id"].(string)
		if num(v, "total_tiyin") != 3*ProdOshPrice || num(v, "remaining_tiyin") != 3*ProdOshPrice {
			t.Fatalf("hisob: %v", v)
		}
		if contains(mustJSON(v), token) || contains(mustJSON(v), w.Customer.UserID) {
			t.Fatal("hisob javobida token yoki to'lovchi ID'si")
		}
		w.Do(t, w.Other, "GET", "/bills/"+billID, nil).Expect(t, 200)
		// Buyurtmadan hisob (ilovadagi "Hisobni to'lash") — o'sha hisob; begonaga yo'q.
		var mine []map[string]any
		found := false
		w.Do(t, w.Customer, "GET", "/me/orders", nil).Expect(t, 200).JSON(t, &mine)
		for _, o := range mine {
			if o["type"] == "dine_in" {
				ob := w.Do(t, w.Customer, "GET", "/orders/"+o["id"].(string)+"/bill", nil).Expect(t, 200).Map(t)
				if ob["bill"].(map[string]any)["id"] != billID {
					t.Fatalf("buyurtma hisobi boshqa: %v", ob)
				}
				w.ExpectDenied(t, w.Other, "GET", "/orders/"+o["id"].(string)+"/bill", nil)
				found = true
				break
			}
		}
		if !found {
			t.Fatal("mijozning stol buyurtmasi ro'yxatda yo'q")
		}
	})

	w.Step(t, "mehmon o'z 1 oshini 10% choy puli bilan to'laydi", func(t *testing.T) {
		v := w.Do(t, w.Customer, "GET", "/bills/"+billID, nil).Expect(t, 200).Map(t)
		orders := v["orders"].([]any)
		first := orders[0].(map[string]any)["id"].(string)
		w.Do(t, w.Customer, "POST", "/bills/"+billID+"/shares", map[string]any{
			"kind": "items", "items": []map[string]any{{"order_id": first, "line": 0, "qty": 9}}}).Expect(t, 400)
		w.Do(t, w.Customer, "POST", "/bills/"+billID+"/shares", map[string]any{
			"kind": "equal", "parts": 2, "tip_percent": 7}).Expect(t, 400)
		r := w.Do(t, w.Customer, "POST", "/bills/"+billID+"/shares", map[string]any{
			"kind": "items", "items": []map[string]any{{"order_id": first, "line": 0, "qty": 1}}, "tip_percent": 10,
		}).Expect(t, 201).Map(t)
		sh := r["share"].(map[string]any)
		if num(sh, "amount_tiyin") != ProdOshPrice || num(sh, "tip_tiyin") != ProdOshPrice/10 {
			t.Fatalf("ulush: %v", sh)
		}
		if p := r["pay"].(map[string]any); p["pay_url"] == "" {
			t.Fatalf("to'lov havolasi: %v", p)
		}
		// To'lanmagan paytda ofitsiant yopa olmaydi.
		w.Do(t, w.Owner, "POST", "/bills/"+billID+"/close", map[string]any{"method": "cash"}).Expect(t, 409)
		payShare(t, sh["id"].(string))
		v = w.Do(t, w.Customer, "GET", "/bills/"+billID, nil).Expect(t, 200).Map(t)
		if num(v, "paid_tiyin") != ProdOshPrice || num(v, "tips_tiyin") != ProdOshPrice/10 {
			t.Fatalf("to'langandan keyin: %v", v)
		}
	})

	w.Step(t, "restoran ochiq hisoblarni ko'radi, begona restoran — yo'q", func(t *testing.T) {
		var list []map[string]any
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/bills", nil).Expect(t, 200).JSON(t, &list)
		if len(list) != 1 {
			t.Fatalf("ochiq hisoblar: %v", list)
		}
		w.ExpectDenied(t, w.OwnerB, "GET", "/restaurants/"+RestA+"/bills", nil)
		// Choy pullari ofitsiant bo'yicha (10% x 1 osh).
		q := "?from=" + url.QueryEscape(time.Now().Add(-24*time.Hour).UTC().Format(time.RFC3339)) +
			"&to=" + url.QueryEscape(time.Now().Add(24*time.Hour).UTC().Format(time.RFC3339))
		var tips []map[string]any
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/tips"+q, nil).Expect(t, 200).JSON(t, &tips)
		if len(tips) != 1 || num(tips[0], "tips_tiyin") != ProdOshPrice/10 {
			t.Fatalf("choy pullari: %v", tips)
		}
		w.ExpectDenied(t, w.OwnerB, "GET", "/restaurants/"+RestA+"/tips"+q, nil)
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/tips?from=x&to=y", nil).Expect(t, 400)
		w.ExpectDenied(t, w.OwnerB, "POST", "/bills/"+billID+"/close", map[string]any{"method": "cash"})
	})

	w.Step(t, "ofitsiant qolganini naqd yopadi, hisob yopiladi", func(t *testing.T) {
		body := `{"first_name":"Hasan","phone":"+998901110009","position":"waiter","app_access":true}`
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/staff", body).Expect(t, 201)
		waiter := w.LoginAs(t, "ofitsiant", "+998901110009")
		var list []map[string]any
		w.Do(t, waiter, "GET", "/waiter/bills", nil).Expect(t, 200).JSON(t, &list)
		if len(list) != 1 {
			t.Fatalf("ofitsiant hisoblari: %v", list)
		}
		w.Do(t, waiter, "POST", "/bills/"+billID+"/close", map[string]any{"method": "bitcoin"}).Expect(t, 400)
		v := w.Do(t, waiter, "POST", "/bills/"+billID+"/close", map[string]any{"method": "cash"}).Expect(t, 200).Map(t)
		if v["bill"].(map[string]any)["status"] != "paid" || num(v, "remaining_tiyin") != 0 {
			t.Fatalf("yopilmadi: %v", v)
		}
		w.Do(t, w.Customer, "POST", "/bills/"+billID+"/shares", map[string]any{"kind": "equal", "parts": 2}).Expect(t, 409)
		_ = tableID
	})
}
