package e2e

import "testing"

// ssenariy: MIJOZ BUYURTMADAN KEYIN RESTORANNI BAHOLAYDI
// (`internal/ratings`) — marketplace ilovalaridagi "Qalay bo'ldi?" bloki.
//
// Yetkazilmagan buyurtmani baholab bo'lmaydi, begona mijoz boshqa
// birovning buyurtmasini baholay olmaydi, bitta buyurtma faqat BIR
// marta baholanadi, va restoran `GET /restaurants/{id}` javobida
// HAQIQIY o'rtacha/soni ko'rinadi (admin/restoran qo'lda kirita
// olmaydi).
func TestOrderRatingFlow(t *testing.T) {
	w := NewWorld(t, "Buyurtmadan keyin reyting")
	var orderID string
	var courier *Actor

	w.Step(t, "buyurtma yaratiladi va yetkazib berilgunga qadar olib boriladi", func(t *testing.T) {
		courier, _ = addCourier(t, w, "+998901110031", "")
		w.ShiftOn(t, courier)
		order := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "cash", "e2e-rating-1")
		orderID, _ = order["id"].(string)
		w.Accept(t, orderID)
		w.AcceptOffer(t, courier, orderID)
		w.Deliver(t, courier, orderID)
	})

	w.Step(t, "hali baho yo'q — GET 404", func(t *testing.T) {
		w.Do(t, w.Customer, "GET", "/orders/"+orderID+"/rating", nil).Expect(t, 404)
	})

	w.Step(t, "faqat buyurtma EGASI baholay oladi", func(t *testing.T) {
		w.ExpectDenied(t, w.Other, "POST", "/orders/"+orderID+"/rating", map[string]any{"stars": 5})
		w.ExpectDenied(t, w.Owner, "POST", "/orders/"+orderID+"/rating", map[string]any{"stars": 5})
		w.ExpectDenied(t, nil, "POST", "/orders/"+orderID+"/rating", map[string]any{"stars": 5})
	})

	w.Step(t, "noto'g'ri qiymat rad etiladi", func(t *testing.T) {
		w.Do(t, w.Customer, "POST", "/orders/"+orderID+"/rating", map[string]any{"stars": 0}).Expect(t, 400)
		w.Do(t, w.Customer, "POST", "/orders/"+orderID+"/rating", map[string]any{"stars": 6}).Expect(t, 400)
	})

	w.Step(t, "mijoz baho qo'yadi — 5 yulduz", func(t *testing.T) {
		r := w.Do(t, w.Customer, "POST", "/orders/"+orderID+"/rating", map[string]any{"stars": 5}).Expect(t, 201).Map(t)
		if num(r, "stars") != 5 {
			t.Fatalf("stars=5 kutilgan, keldi: %v", r)
		}
		got := w.Do(t, w.Customer, "GET", "/orders/"+orderID+"/rating", nil).Expect(t, 200).Map(t)
		if num(got, "stars") != 5 {
			t.Fatalf("GET orqali ham stars=5 kutilgan: %v", got)
		}
	})

	w.Step(t, "qayta baholash rad etiladi (409)", func(t *testing.T) {
		w.Do(t, w.Customer, "POST", "/orders/"+orderID+"/rating", map[string]any{"stars": 1}).Expect(t, 409)
	})

	w.Step(t, "restoran javobida HAQIQIY reyting ko'rinadi", func(t *testing.T) {
		rest := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA, nil).Expect(t, 200).Map(t)
		if num(rest, "rating") != 5 || num(rest, "rating_count") != 1 {
			t.Fatalf("rating=5 rating_count=1 kutilgan, keldi: %v", rest)
		}
	})

	w.Step(t, "yetkazilmagan buyurtmani baholab bo'lmaydi", func(t *testing.T) {
		second := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "cash", "e2e-rating-2")
		id2, _ := second["id"].(string)
		w.Do(t, w.Customer, "POST", "/orders/"+id2+"/rating", map[string]any{"stars": 4}).Expect(t, 409)
	})

	w.Step(t, "admin ham rating/rating_count'ni qo'lda o'zgartira olmaydi", func(t *testing.T) {
		w.Do(t, w.Admin, "POST", "/admin/restaurants/"+RestA, map[string]any{
			"name": "Restoran A", "address": "Chust", "lat": ChustLat, "lng": ChustLng,
			"rating": 1, "rating_count": 999,
		}).Expect(t, 200)
		rest := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA, nil).Expect(t, 200).Map(t)
		if num(rest, "rating") != 5 || num(rest, "rating_count") != 1 {
			t.Fatalf("admin yuborgan soxta qiymat e'tiborsiz qolishi kerak edi: %v", rest)
		}
	})
}

// ssenariy: YETKAZISH VAQTI — HAQIQIY buyurtma tarixidan hisoblanadi
// (`internal/orders.ComputeETARange`), reyting bilan BIR XIL naqsh:
// ma'lumot yo'qda 0, admin qo'lda o'zgartira olmaydi, faqat 5
// daqiqalik qadam (20/25/30/35/40) bilan chiqadi.
func TestRestaurantETAComputedFromHistory(t *testing.T) {
	w := NewWorld(t, "Yetkazish vaqti — haqiqiy tarixdan")

	w.Step(t, "hali buyurtma yo'q — eta ko'rsatilmaydi (0)", func(t *testing.T) {
		rest := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA, nil).Expect(t, 200).Map(t)
		if num(rest, "eta_min_minutes") != 0 || num(rest, "eta_max_minutes") != 0 {
			t.Fatalf("ma'lumot yo'qda 0,0 kutilgan, keldi: %v", rest)
		}
	})

	w.Step(t, "admin eta_min/eta_max'ni qo'lda o'zgartira olmaydi", func(t *testing.T) {
		w.Do(t, w.Admin, "POST", "/admin/restaurants/"+RestA, map[string]any{
			"name": "Restoran A", "address": "Chust", "lat": ChustLat, "lng": ChustLng,
			"eta_min_minutes": 1, "eta_max_minutes": 999,
		}).Expect(t, 200)
		rest := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA, nil).Expect(t, 200).Map(t)
		if num(rest, "eta_min_minutes") != 0 || num(rest, "eta_max_minutes") != 0 {
			t.Fatalf("admin yuborgan soxta qiymat e'tiborsiz qolishi kerak edi: %v", rest)
		}
	})

	w.Step(t, "buyurtma yetkazilgach — haqiqiy oraliq, 5 daqiqalik qadam bilan", func(t *testing.T) {
		courier, _ := addCourier(t, w, "+998901110032", "")
		w.ShiftOn(t, courier)
		order := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "cash", "e2e-eta-1")
		orderID, _ := order["id"].(string)
		w.Accept(t, orderID)
		w.AcceptOffer(t, courier, orderID)
		w.Deliver(t, courier, orderID)

		rest := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA, nil).Expect(t, 200).Map(t)
		lo := num(rest, "eta_min_minutes")
		hi := num(rest, "eta_max_minutes")
		// Testda buyurtma deyarli zudlik bilan yetkaziladi — real
		// davomiylik ~0 daqiqa, pastki chegara (20) ga qisiladi.
		if lo != 20 {
			t.Fatalf("eta_min=20 kutilgan (pastki chegara), keldi: %v", rest)
		}
		if hi%5 != 0 || hi < 20 || hi > 40 || hi <= lo {
			t.Fatalf("eta_max 5 daqiqalik qadam, lo dan katta, [20,40] oralig'ida bo'lishi kerak: %v", rest)
		}
	})
}
