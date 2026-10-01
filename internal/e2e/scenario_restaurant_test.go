package e2e

import (
	"fmt"
	"testing"
	"time"
)

// ssenariy: RESTORAN BOSHQARUVI — menyu, aksiyalar, ochiq/yopiq, Moliya,
// xodimlar, bildirishnomalar va admin bilan yordam chati.

func TestRestaurantManagementFlow(t *testing.T) {
	w := NewWorld(t, "Restoran boshqaruvi (menyu, aksiya, Moliya, xodim, chat)")
	var newProd, promoID string

	w.Step(t, "menyu: yangi taom qo'shish, tahrirlash, chegirma narxi", func(t *testing.T) {
		p := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/products", map[string]any{
			"name": "Lag'mon", "category": "Asosiy", "price_tiyin": 2_500_000, "available": true,
		}).Expect(t, 201).Map(t)
		newProd, _ = p["id"].(string)
		if newProd == "" {
			t.Fatalf("taom ID'si yo'q: %v", p)
		}
		// Nomaqbul: manfiy narx, chegirma narxi asosiydan katta.
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/products", map[string]any{"name": "X", "price_tiyin": -1}).Expect(t, 400)
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/products", map[string]any{
			"name": "X", "price_tiyin": 1000, "discount_price_tiyin": 2000}).Expect(t, 400)
		// Tahrirlash (id bilan) va ro'yxatda ko'rinishi.
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/products", map[string]any{
			"id": newProd, "name": "Lag'mon (katta)", "category": "Asosiy", "price_tiyin": 2_800_000, "available": true,
		}).Expect(t, 201)
		list := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/products", nil).Expect(t, 200)
		if !contains(list.Body, "Lag'mon (katta)") {
			t.Fatal("tahrirlangan taom ro'yxatda yo'q")
		}
		menu := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA+"/menu", nil).Expect(t, 200)
		if !contains(menu.Body, newProd) {
			t.Fatal("yangi taom mijoz menyusida ko'rinmadi")
		}
	})

	w.Step(t, "begona restoran menyuga TEGA OLMAYDI", func(t *testing.T) {
		w.ExpectDenied(t, w.OwnerB, "POST", "/restaurants/"+RestA+"/products", map[string]any{"name": "Y", "price_tiyin": 1000})
		w.ExpectDenied(t, w.OwnerB, "DELETE", "/restaurants/"+RestA+"/products/"+newProd, nil)
		// B restorani A ning taom ID'sini o'z yo'lida bosib yoza olmaydi.
		w.ExpectDenied(t, w.OwnerB, "POST", "/restaurants/"+RestB+"/products", map[string]any{
			"id": newProd, "name": "Hujum", "price_tiyin": 1})
		w.ExpectDenied(t, w.Customer, "GET", "/restaurants/"+RestA+"/products", nil)
	})

	w.Step(t, "taomni o'chirish: menyudan yo'qoladi, eski buyurtmalar saqlanadi", func(t *testing.T) {
		// Buyurtma bergan taomni o'chirish eski buyurtmani buzmasligi kerak.
		ord := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": []map[string]any{{"product_id": newProd, "qty": 1}}, "payment_method": "cash",
			"idempotency_key": "e2e-del-prod-1",
		}).Expect(t, 201).Map(t)
		oid, _ := ord["id"].(string)
		w.Do(t, w.Owner, "DELETE", "/restaurants/"+RestA+"/products/"+newProd, nil).Expect(t, 200)
		menu := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA+"/menu", nil).Expect(t, 200)
		if contains(menu.Body, newProd) {
			t.Fatal("o'chirilgan taom menyuda qoldi")
		}
		old := w.Do(t, w.Customer, "GET", "/orders/"+oid, nil).Expect(t, 200).Map(t)
		w.CheckOrder(t, old)
		// O'chirilgan taomga yangi buyurtma berib bo'lmaydi.
		w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": []map[string]any{{"product_id": newProd, "qty": 1}}, "payment_method": "cash",
			"idempotency_key": "e2e-del-prod-2",
		}).Expect(t, 400)
	})

	w.Step(t, "aksiya yaratish: buyurtma narxiga chegirma qo'llanadi (jami = qismlar)", func(t *testing.T) {
		start := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
		p := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/promotions", map[string]any{
			"name": "10% chegirma", "type": "percent", "discount_unit": "percent", "discount_value": 10,
			"start_at": start, "indefinite": true, "active": true, "applies_to_orders": true,
		}).Expect(t, 201).Map(t)
		promoID, _ = p["id"].(string)
		// Noto'g'ri: 150%, turi-birlik zidligi, qo'llanish joyi yo'q.
		bad := func(m map[string]any) {
			t.Helper()
			m["name"], m["start_at"], m["indefinite"], m["active"] = "x", start, true, true
			w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/promotions", m).Expect(t, 400)
		}
		bad(map[string]any{"type": "percent", "discount_unit": "percent", "discount_value": 150, "applies_to_orders": true})
		bad(map[string]any{"type": "percent", "discount_unit": "amount", "discount_value": 10, "applies_to_orders": true})
		bad(map[string]any{"type": "percent", "discount_unit": "percent", "discount_value": 10})

		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/promotions", nil).Expect(t, 200)
		act := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA+"/active-promotions", nil).Expect(t, 200)
		if !contains(act.Body, "10% chegirma") {
			t.Fatalf("faol aksiya ko'rinmadi: %s", act.Body)
		}
		q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote",
			map[string]any{"items": []map[string]any{{"product_id": ProdOsh, "qty": 2}}}).Expect(t, 200).Map(t)
		w.CheckQuote(t, q, false)
		if num(q, "discount_tiyin") != ProdOshPrice*2/10 {
			t.Fatalf("10%% chegirma %d, %d kutilgan: %v", num(q, "discount_tiyin"), ProdOshPrice*2/10, q)
		}
		// Buyurtma ham xuddi shu raqamni yozadi.
		o := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": []map[string]any{{"product_id": ProdOsh, "qty": 2}}, "payment_method": "cash",
			"idempotency_key": "e2e-promo-1", "expected_total_tiyin": num(q, "total_tiyin"),
		}).Expect(t, 201).Map(t)
		w.CheckOrder(t, o)
		w.CheckOrderMatchesQuote(t, q, o)
	})

	w.Step(t, "aksiyani o'chirgach chegirma yo'qoladi; begona o'chira olmaydi", func(t *testing.T) {
		w.ExpectDenied(t, w.OwnerB, "DELETE", "/restaurants/"+RestA+"/promotions/"+promoID, nil)
		w.Do(t, w.Owner, "DELETE", "/restaurants/"+RestA+"/promotions/"+promoID, nil).Expect(t, 200)
		q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote",
			map[string]any{"items": []map[string]any{{"product_id": ProdOsh, "qty": 2}}}).Expect(t, 200).Map(t)
		if num(q, "discount_tiyin") != 0 {
			t.Fatalf("o'chirilgan aksiya hali ishlayapti: %v", q)
		}
	})

	w.Step(t, "restoran yopiladi — buyurtma berib bo'lmaydi; ochilsa — bo'ladi", func(t *testing.T) {
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/open", map[string]any{"open": false}).Expect(t, 200)
		r := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": []map[string]any{{"product_id": ProdOsh, "qty": 1}}, "payment_method": "cash",
			"idempotency_key": "e2e-closed-1",
		})
		if r.Status < 400 || r.Status >= 500 {
			t.Fatalf("yopiq restoranga buyurtma qabul qilindi: %d %s", r.Status, r.Body)
		}
		pub := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA, nil).Expect(t, 200).Map(t)
		if pub["open_now"] != false {
			t.Fatalf("yopiq restoran open_now=%v", pub["open_now"])
		}
		w.ExpectDenied(t, w.OwnerB, "POST", "/restaurants/"+RestA+"/open", map[string]any{"open": true})
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/open", map[string]any{"open": true}).Expect(t, 200)
		w.Do(t, w.Customer, "POST", "/orders", map[string]any{
			"items": []map[string]any{{"product_id": ProdOsh, "qty": 1}}, "payment_method": "cash",
			"idempotency_key": "e2e-open-1",
		}).Expect(t, 201)
	})

	w.Step(t, "Moliya: ulanish, reestr, uzish", func(t *testing.T) {
		f := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/finance", nil).Expect(t, 200).Map(t)
		if f["connected"] != true {
			t.Fatalf("Moliya ulangan bo'lishi kerak: %v", f)
		}
		// Daromad: faqat `success` — draft yig'indisi (99 000 000) QO'SHILMASLIGI kerak.
		if num(f, "turnover_tiyin") != 7_000_000 {
			w.Violate(t, "money.finance_counts_only_successful_payments",
				"aylanma %d, 7000000 kutilgan (draft/cancelled qo'shilgan bo'lishi mumkin)", num(f, "turnover_tiyin"))
		}
		// Noto'g'ri kassa — pul begona hisobga ketmasin.
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/finance/connect", map[string]any{
			"application_id": GatewayAppID, "secret": "x", "store_id": "999"}).Expect(t, 422)
		w.ExpectDenied(t, w.OwnerB, "GET", "/restaurants/"+RestA+"/finance", nil)
		w.ExpectDenied(t, w.Customer, "GET", "/restaurants/"+RestA+"/finance", nil)
		w.Do(t, w.Owner, "DELETE", "/restaurants/"+RestA+"/finance/connect", nil).Expect(t, 204)
		// Uzilgach onlayn usullar mijozdan yashiriladi va to'lov ochilmaydi.
		if opts := publicOptions(t, w); opts["card"] != false {
			t.Fatalf("uzilgan restoranda karta ko'rinadi: %v", opts)
		}
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/finance/connect", map[string]any{
			"application_id": GatewayAppID, "secret": GatewaySecret, "store_id": GatewayStore}).Expect(t, 200)
		if opts := publicOptions(t, w); opts["card"] != true {
			t.Fatalf("qayta ulangach karta ko'rinmadi: %v", opts)
		}
	})

	w.Step(t, "xodimlar: tahrirlash, ta'til, bo'shatish, qayta ishga olish", func(t *testing.T) {
		_, id := addCourier(t, w, "+998901110011", `,"monthly_salary_tiyin":350000000`)
		path := "/restaurants/" + RestA + "/staff/" + id
		w.Do(t, w.Owner, "PATCH", path, map[string]any{"note": "yaxshi kuryer"}).Expect(t, 200)
		w.ExpectDenied(t, w.OwnerB, "PATCH", path, map[string]any{"note": "hujum"})
		for _, st := range []string{"on_leave", "active", "dismissed", "active"} {
			w.Do(t, w.Owner, "POST", path+"/status", map[string]any{"status": st}).Expect(t, 200)
		}
		w.Do(t, w.Owner, "POST", path+"/status", map[string]any{"status": "yoq-holat"}).Expect(t, 400)
	})

	w.Step(t, "ta'tildagi/bo'shatilgan kuryerning haqi mijozga QO'SHILMAYDI", func(t *testing.T) {
		_, id := addCourier(t, w, "+998901110012", `,"delivery_pay_tiyin":900000`)
		fee := func() int64 {
			q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote",
				map[string]any{"items": []map[string]any{{"product_id": ProdOsh, "qty": 1}}}).Expect(t, 200).Map(t)
			w.CheckQuote(t, q, false)
			return num(q, "delivery_fee_tiyin")
		}
		path := "/restaurants/" + RestA + "/staff/" + id + "/status"
		if got := fee(); got != 900_000 {
			t.Fatalf("faol kuryer haqi %d, 900000 kutilgan", got)
		}
		for _, st := range []string{"on_leave", "dismissed"} {
			w.Do(t, w.Owner, "POST", path, map[string]any{"status": st}).Expect(t, 200)
			if got := fee(); got != 0 {
				w.Violate(t, "money.inactive_courier_fee_not_charged", "%s kuryer haqi %d, 0 kutilgan", st, got)
			}
		}
		w.Do(t, w.Owner, "POST", path, map[string]any{"status": "active"}).Expect(t, 200)
		if got := fee(); got != 900_000 {
			t.Fatalf("qaytgan kuryer haqi %d, 900000 kutilgan", got)
		}
	})

	w.Step(t, "bildirishnomalar markazi", func(t *testing.T) {
		w.Do(t, w.Admin, "POST", "/admin/restaurant-notifications", map[string]any{
			"restaurant_id": RestA, "title": "Yangilik", "body": "Platforma yangilandi"}).Expect(t, 201)
		n := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/notifications", nil).Expect(t, 200)
		if len(n.Body) < 5 {
			t.Fatal("bildirishnomalar bo'sh")
		}
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/notifications/summary", nil).Expect(t, 200)
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/notifications/read-all", map[string]any{"up_to_seq": 1000}).Expect(t, 200)
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/notifications/yoq/read", nil)
		w.ExpectDenied(t, w.OwnerB, "GET", "/restaurants/"+RestA+"/notifications", nil)
	})

	w.Step(t, "yordam markazi: restoran <-> admin chati", func(t *testing.T) {
		w.Do(t, w.Customer, "GET", "/support/contacts", nil).Expect(t, 200)
		w.Do(t, w.Admin, "PUT", "/admin/support/contacts", map[string]any{
			"phone": "+998712000000", "telegram": "@ondex_support"})
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/support/messages", map[string]any{
			"body": "Salom, yordam kerak", "client_id": "e2e-msg-1"}).Expect(t, 201)
		msgs := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/support/messages", nil).Expect(t, 200)
		if !contains(msgs.Body, "yordam kerak") {
			t.Fatalf("xabar ko'rinmadi: %s", msgs.Body)
		}
		w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/support/summary", nil).Expect(t, 200)
		w.Do(t, w.Admin, "GET", "/admin/support/summary", nil).Expect(t, 200)
		w.Do(t, w.Admin, "GET", "/admin/support/threads", nil).Expect(t, 200)
		thread := "/admin/support/threads/" + RestA
		w.Do(t, w.Admin, "GET", thread+"/messages", nil).Expect(t, 200)
		w.Do(t, w.Admin, "POST", thread+"/messages", map[string]any{"body": "Yordam beramiz", "client_id": "e2e-adm-1"}).Expect(t, 201)
		w.Do(t, w.Admin, "POST", thread+"/read", map[string]any{"up_to_seq": 100}).Expect(t, 200)
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/support/read", map[string]any{"up_to_seq": 100}).Expect(t, 200)
		// Begona restoran bu suhbatni o'qiy olmaydi.
		w.ExpectDenied(t, w.OwnerB, "GET", "/restaurants/"+RestA+"/support/messages", nil)
		w.ExpectDenied(t, w.Owner, "GET", "/admin/support/threads", nil)
		_ = fmt.Sprint()
	})
}
