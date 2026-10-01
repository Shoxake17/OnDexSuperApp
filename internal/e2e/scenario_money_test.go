package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// ssenariylar: PUL MATRITSASI — aksiya turlari va savat/to'lov/yetkazish
// kombinatsiyalari. Kutilgan qiymatlar QO'LDA (arifmetika) hisoblangan —
// production kodining o'zidan emas, shuning uchun hisob xatosini ushlaydi.

const (
	ProdSomsa      = "e2e-prod-somsa"
	ProdSomsaPrice = int64(1_000_000) // 10 000 so'm, turkum "Gazaklar"
	ProdChoy       = "e2e-prod-choy"
	ProdChoyPrice  = int64(500_000) // 5 000 so'm, turkum "Ichimliklar"
)

func seedMoneyProducts(t *testing.T, w *World) {
	t.Helper()
	for _, p := range []map[string]any{
		{"id": ProdSomsa, "name": "Somsa", "category": "Gazaklar", "price_tiyin": ProdSomsaPrice, "available": true},
		{"id": ProdChoy, "name": "Choy", "category": "Ichimliklar", "price_tiyin": ProdChoyPrice, "available": true},
	} {
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/products", p).Expect(t, 201)
	}
}

func lineItems(pairs ...any) []map[string]any {
	var out []map[string]any
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, map[string]any{"product_id": pairs[i], "qty": pairs[i+1]})
	}
	return out
}

func promoBody(name, typ, unit string, value int64, extra map[string]any) map[string]any {
	b := map[string]any{
		"name": name, "type": typ, "discount_unit": unit, "discount_value": value,
		"start_at": time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), "indefinite": true, "active": true,
	}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

func TestPromotionMatrix(t *testing.T) {
	w := NewWorld(t, "Aksiya matritsasi (turlar, chegaralar, eng yaxshisi)")
	const fee = int64(800_000)

	w.Step(t, "tayyorgarlik: taomlar va 'buyurtmadan maosh'li kuryer (yetkazish haqi bor)", func(t *testing.T) {
		seedMoneyProducts(t, w)
		addCourier(t, w, "+998901110071", fmt.Sprintf(`,"delivery_pay_tiyin":%d`, fee))
	})

	type tc struct {
		name  string
		promo map[string]any
		cart  []map[string]any
		want  int64 // kutilgan JAMI chegirma (tiyin); qo'lda hisoblangan
	}
	osh, somsa, choy := ProdOshPrice, ProdSomsaPrice, ProdChoyPrice
	cases := []tc{
		{"foiz: butun buyurtmaga 10%", promoBody("p1", "percent", "percent", 10, map[string]any{"applies_to_orders": true}),
			lineItems(ProdOsh, 2), osh * 2 / 10},
		{"summa: donaga 1 000 so'm, 3 dona", promoBody("p2", "fixed_amount", "amount", 100_000,
			map[string]any{"applies_to_products": true, "target_product_ids": []string{ProdOsh}}),
			lineItems(ProdOsh, 3), 300_000},
		// Savatda boshqa taom ham bor: butun savat chegirma chegarasi (jami ≤ taomlar) bu holatni yashirmasligi uchun.
		{"summa: qator summasidan oshmaydi (5 000 so'mlik choyga 10 000)", promoBody("p3", "fixed_amount", "amount", 1_000_000,
			map[string]any{"applies_to_products": true, "target_product_ids": []string{ProdChoy}}),
			lineItems(ProdChoy, 1, ProdOsh, 1), choy},
		{"1+1: 5 dona somsadan 2 tasi bepul", promoBody("p4", "bogo", "percent", 50,
			map[string]any{"applies_to_products": true, "target_product_ids": []string{ProdSomsa}}),
			lineItems(ProdSomsa, 5), 2 * somsa},
		{"1+1: 1 dona — bepul yo'q", promoBody("p5", "bogo", "percent", 50,
			map[string]any{"applies_to_products": true, "target_product_ids": []string{ProdSomsa}}),
			lineItems(ProdSomsa, 1), 0},
		{"minimal summa: yetmasa chegirma YO'Q", promoBody("p6", "percent", "percent", 10,
			map[string]any{"applies_to_orders": true, "min_order_amount_tiyin": 5_000_000}),
			lineItems(ProdOsh, 1), 0},
		{"minimal summa: yetsa chegirma bor", promoBody("p7", "percent", "percent", 10,
			map[string]any{"applies_to_orders": true, "min_order_amount_tiyin": 5_000_000}),
			lineItems(ProdOsh, 2), osh * 2 / 10},
		{"maksimal chegirma: 50% lekin 10 000 so'mdan oshmaydi", promoBody("p8", "percent", "percent", 50,
			map[string]any{"applies_to_orders": true, "max_discount_amount_tiyin": 1_000_000}),
			lineItems(ProdOsh, 2), 1_000_000},
		{"turkum: faqat 'Ichimliklar' ga 20%", promoBody("p9", "percent", "percent", 20,
			map[string]any{"applies_to_categories": true, "target_categories": []string{"Ichimliklar"}}),
			lineItems(ProdOsh, 1, ProdChoy, 2), choy * 2 * 20 / 100},
		{"to'plam: ikkala taom bor — 15%", promoBody("p10", "bundle", "percent", 15,
			map[string]any{"applies_to_products": true, "target_product_ids": []string{ProdOsh, ProdChoy}}),
			lineItems(ProdOsh, 1, ProdChoy, 1), (osh + choy) * 15 / 100},
		{"to'plam: bittasi yo'q — chegirma YO'Q", promoBody("p11", "bundle", "percent", 15,
			map[string]any{"applies_to_products": true, "target_product_ids": []string{ProdOsh, ProdChoy}}),
			lineItems(ProdOsh, 1), 0},
		{"yetkazish aksiyasi (free_delivery) hech narsani arzonlashtirmaydi", promoBody("p12", "free_delivery", "percent", 100,
			map[string]any{"applies_to_orders": true}),
			lineItems(ProdOsh, 1), 0},
		{"sodiqlik: oldingi buyurtmasi yo'q mijozga YO'Q", promoBody("p13", "loyalty", "percent", 5,
			map[string]any{"applies_to_orders": true, "min_previous_orders": 3}),
			lineItems(ProdOsh, 1), 0},
		{"nofaol aksiya", promoBody("p14", "percent", "percent", 50,
			map[string]any{"applies_to_orders": true, "active": false}),
			lineItems(ProdOsh, 1), 0},
		{"kelajakda boshlanadigan aksiya", promoBody("p15", "percent", "percent", 50,
			map[string]any{"applies_to_orders": true, "start_at": time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)}),
			lineItems(ProdOsh, 1), 0},
		{"muddati o'tgan aksiya", promoBody("p16", "percent", "percent", 50,
			map[string]any{"applies_to_orders": true, "indefinite": false,
				"start_at": time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339),
				"end_at":   time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)}),
			lineItems(ProdOsh, 1), 0},
	}

	for _, c := range cases {
		w.Step(t, c.name, func(t *testing.T) {
			who := w.Customer
			if strings.Contains(c.name, "oldingi buyurtmasi yo'q") { // YANGI mijoz nomidan
				who = w.Other
			}
			p := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/promotions", c.promo).Expect(t, 201).Map(t)
			pid, _ := p["id"].(string)
			defer w.Do(t, w.Owner, "DELETE", "/restaurants/"+RestA+"/promotions/"+pid, nil)

			q := w.Do(t, who, "POST", "/restaurants/"+RestA+"/quote", map[string]any{"items": c.cart}).Expect(t, 200).Map(t)
			w.CheckQuote(t, q, false)
			w.R().Checked("money.promotion_discount_matches_hand_calculation")
			if got := num(q, "discount_tiyin"); got != c.want {
				w.Violate(t, "money.promotion_discount_matches_hand_calculation",
					"%s: chegirma %d, qo'lda hisoblangan %d", c.name, got, c.want)
			}
			if num(q, "delivery_fee_tiyin") != fee {
				t.Fatalf("yetkazish haqi %d, %d kutilgan (aksiya yetkazishga tegmasligi kerak)", num(q, "delivery_fee_tiyin"), fee)
			}
			// Buyurtma ham aynan shu raqamni yozadi.
			key := "e2e-promo-" + pid
			o := w.Do(t, who, "POST", "/orders", map[string]any{
				"items": c.cart, "payment_method": "cash", "idempotency_key": key,
				"expected_total_tiyin": num(q, "total_tiyin"),
			}).Expect(t, 201).Map(t)
			w.CheckOrder(t, o)
			w.CheckOrderMatchesQuote(t, q, o)
		})
	}

	w.Step(t, "ikki aksiya bir qatorda QO'SHILMAYDI — eng yaxshisi olinadi", func(t *testing.T) {
		mk := func(body map[string]any) string {
			p := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/promotions", body).Expect(t, 201).Map(t)
			id, _ := p["id"].(string)
			return id
		}
		a := mk(promoBody("kichik", "percent", "percent", 10, map[string]any{"applies_to_orders": true}))
		b := mk(promoBody("katta", "percent", "percent", 30, map[string]any{"applies_to_orders": true}))
		defer w.Do(t, w.Owner, "DELETE", "/restaurants/"+RestA+"/promotions/"+a, nil)
		defer w.Do(t, w.Owner, "DELETE", "/restaurants/"+RestA+"/promotions/"+b, nil)
		q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote", map[string]any{"items": lineItems(ProdOsh, 2)}).Expect(t, 200).Map(t)
		w.CheckQuote(t, q, false)
		w.R().Checked("money.promotions_do_not_stack_on_one_line")
		if want := osh * 2 * 30 / 100; num(q, "discount_tiyin") != want {
			w.Violate(t, "money.promotions_do_not_stack_on_one_line", "chegirma %d, eng yaxshi (30%%) = %d kutilgan", num(q, "discount_tiyin"), want)
		}
	})

	w.Step(t, "sodiqlik: yetarli oldingi buyurtmasi bor mijozga qo'llanadi", func(t *testing.T) {
		p := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/promotions", promoBody("sodiq", "loyalty", "percent", 5,
			map[string]any{"applies_to_orders": true, "min_previous_orders": 2})).Expect(t, 201).Map(t)
		pid, _ := p["id"].(string)
		defer w.Do(t, w.Owner, "DELETE", "/restaurants/"+RestA+"/promotions/"+pid, nil)
		// Mijoz allaqachon bir nechta buyurtma bergan (yuqoridagi qadamlar) — qo'llanadi.
		q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote", map[string]any{"items": lineItems(ProdOsh, 2)}).Expect(t, 200).Map(t)
		w.CheckQuote(t, q, false)
		if want := osh * 2 * 5 / 100; num(q, "discount_tiyin") != want {
			t.Fatalf("sodiqlik chegirmasi %d, %d kutilgan", num(q, "discount_tiyin"), want)
		}
		// Yangi mijozga (buyurtmasi yo'q) qo'llanmaydi.
		q2 := w.Do(t, w.Other, "POST", "/restaurants/"+RestA+"/quote", map[string]any{"items": lineItems(ProdOsh, 2)}).Expect(t, 200).Map(t)
		if num(q2, "discount_tiyin") != 0 {
			t.Fatalf("yangi mijozga sodiqlik chegirmasi berildi: %v", q2)
		}
	})
}

// PUL MATRITSASI: savat × to'lov usuli × yetkazish usuli. Har kombinatsiyada
// quote = buyurtma = (karta bo'lsa) shlyuz summasi = fiskal chek yig'indisi.
func TestMoneyMatrix(t *testing.T) {
	w := NewWorld(t, "Pul matritsasi (savat × to'lov × yetkazish)")
	const fee = int64(800_000)
	var token string

	w.Step(t, "tayyorgarlik: taomlar, kuryer, 10% aksiya, stol", func(t *testing.T) {
		seedMoneyProducts(t, w)
		addCourier(t, w, "+998901110072", fmt.Sprintf(`,"delivery_pay_tiyin":%d`, fee))
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/promotions", promoBody("10%", "percent", "percent", 10,
			map[string]any{"applies_to_orders": true, "min_order_amount_tiyin": 5_000_000})).Expect(t, 201)
		tb := w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/tables",
			map[string]any{"label": "1", "zone": "Zal", "kind": "table", "capacity": 4}).Expect(t, 201).Map(t)
		token, _ = tb["qr_token"].(string)
	})

	carts := []struct {
		name  string
		items []map[string]any
	}{
		{"1 osh", lineItems(ProdOsh, 1)},
		{"3 osh", lineItems(ProdOsh, 3)},
		{"osh + somsa + choy", lineItems(ProdOsh, 1, ProdSomsa, 2, ProdChoy, 3)},
		{"7 somsa", lineItems(ProdSomsa, 7)},
	}
	n := 0
	for _, cart := range carts {
		for _, method := range []string{"cash", "card"} {
			n++
			key := fmt.Sprintf("e2e-matrix-%d", n)
			w.Step(t, fmt.Sprintf("%s · %s · yetkazish", cart.name, method), func(t *testing.T) {
				q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote", map[string]any{"items": cart.items}).Expect(t, 200).Map(t)
				w.CheckQuote(t, q, false)
				if num(q, "delivery_fee_tiyin") != fee {
					t.Fatalf("yetkazish haqi %d, %d kutilgan", num(q, "delivery_fee_tiyin"), fee)
				}
				o := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
					"items": cart.items, "payment_method": method, "idempotency_key": key,
					"expected_total_tiyin": num(q, "total_tiyin"),
				}).Expect(t, 201).Map(t)
				w.CheckOrder(t, o)
				w.CheckOrderMatchesQuote(t, q, o)
				if method == "card" {
					id, _ := o["id"].(string)
					w.PayCard(t, w.Customer, id)
					invoice, _ := w.Gateway.Last()
					w.CheckGatewayPayment(t, invoice, num(o, "total_tiyin"))
				}
			})
		}
		// Stol: yetkazish haqi yo'q, aksiya baribir qo'llanadi.
		n++
		key := fmt.Sprintf("e2e-matrix-%d", n)
		w.Step(t, fmt.Sprintf("%s · stol (yetkazishsiz)", cart.name), func(t *testing.T) {
			q := w.Do(t, w.Customer, "POST", "/restaurants/"+RestA+"/quote", map[string]any{"items": cart.items, "dine_in": true}).Expect(t, 200).Map(t)
			w.CheckQuote(t, q, true)
			o := w.Do(t, w.Customer, "POST", "/orders", map[string]any{
				"items": cart.items, "table_token": token, "payment_method": "cash", "idempotency_key": key,
				"expected_total_tiyin": num(q, "total_tiyin"),
			}).Expect(t, 201).Map(t)
			w.CheckOrder(t, o)
			w.CheckOrderMatchesQuote(t, q, o)
			if num(o, "delivery_fee_tiyin") != 0 {
				t.Fatalf("stol buyurtmasida yetkazish haqi: %v", o["delivery_fee_tiyin"])
			}
		})
	}
}
