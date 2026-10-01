package e2e

import (
	"testing"
)

// ssenariy: SOZLANMAGAN TASHQI XIZMATLAR — xavfsiz yopiq turadi.
//
// Google xaritasi, Gemini, Telegram, Firebase, Google OAuth, Mongo kitoblari,
// Tripo 3D, AI agent integratsiyasi (Postgres) — E2E dunyosida ulanmagan.
// Har biri 503/404 kontraktiga amal qilishi kerak: server yiqilmaydi,
// sir sizmaydi, boshqa endpointlarga ta'sir qilmaydi.

func TestUnconfiguredServicesFailClosed(t *testing.T) {
	w := NewWorld(t, "Sozlanmagan tashqi xizmatlar (fail closed)")
	c, o := w.Customer, w.Owner

	w.Step(t, "xarita va geokodlash (Google kaliti yo'q)", func(t *testing.T) {
		w.Gated(t, c, "GET", "/config/maps", nil)
		w.Gated(t, c, "GET", "/geocode/reverse?lat=41&lng=71.2", nil)
		w.Gated(t, c, "GET", "/geocode/autocomplete?input=chust", nil)
		w.Gated(t, c, "GET", "/geocode/place?place_id=x", nil)
		w.Gated(t, c, "GET", "/geocode/route?origin_lat=41&origin_lng=71.2&dest_lat=41.01&dest_lng=71.21", nil)
		// Noto'g'ri parametr — 400, 5xx emas.
		w.Gated(t, c, "GET", "/geocode/reverse?lat=abc&lng=1", nil, 400, 503)
		w.ExpectDenied(t, nil, "GET", "/geocode/reverse?lat=41&lng=71.2", nil)
		w.Do(t, nil, "GET", "/map-picker", nil).Expect(t, 200)
		// OnDexMap rejimida sahifa kutubxonalari shu serverdan beriladi.
		w.Do(t, nil, "GET", "/map-picker/lib/pmtiles.js", nil).Expect(t, 200)
		w.Do(t, nil, "GET", "/map-picker/lib/yoq.js", nil).Expect(t, 404)
		// Ilova OnDexMap xatosi tufayli Google'ga o'tganini xabar qiladi.
		w.Do(t, c, "POST", "/config/maps/fallback", map[string]any{"surface": "address", "reason": "test"}).Expect(t, 204)
		w.ExpectDenied(t, nil, "POST", "/config/maps/fallback", map[string]any{"surface": "web"})
	})

	w.Step(t, "AI yordamchi o'chiq: holat aytiladi, chat yopiq", func(t *testing.T) {
		st := w.Do(t, c, "GET", "/ai/status", nil).Expect(t, 200).Map(t)
		if st["enabled"] != false {
			t.Fatalf("sozlanmagan AI yoqiq ko'rinadi: %v", st)
		}
		w.Gated(t, c, "POST", "/ai/chat", map[string]any{"message": "salom"})
		w.Gated(t, c, "GET", "/ai/live", nil, 404, 503, 400)
		w.ExpectDenied(t, nil, "POST", "/ai/chat", map[string]any{"message": "x"})
	})

	w.Step(t, "AI asboblari: standart yoqiq, hammasini o'chirib bo'lmaydi", func(t *testing.T) {
		w.Do(t, c, "GET", "/me/ai-tools", nil).Expect(t, 200)
		w.Do(t, c, "PUT", "/me/ai-tools", map[string]any{"enabled": []string{}}).Expect(t, 400)
		w.Do(t, c, "PUT", "/me/ai-tools", map[string]any{"enabled": []string{"search_food"}}).Expect(t, 204)
		w.Do(t, c, "GET", "/me/ai-tools", nil).Expect(t, 200)
	})

	w.Step(t, "kirish usullari: Telegram, Firebase, Google, email", func(t *testing.T) {
		w.Gated(t, nil, "POST", "/auth/firebase", map[string]any{"id_token": "x"})
		w.Gated(t, nil, "POST", "/auth/google", map[string]any{"id_token": "x"})
		w.Gated(t, nil, "POST", "/auth/telegram/start", map[string]any{"phone": "+998901234567"})
		w.Gated(t, nil, "POST", "/auth/telegram/login/start", nil)
		w.Gated(t, nil, "POST", "/auth/telegram/login/start-web", nil)
		w.Gated(t, nil, "GET", "/auth/telegram/login/status?token=x", nil)
		w.Gated(t, nil, "GET", "/auth/telegram/return", nil, 400, 503)
		w.Gated(t, nil, "POST", "/auth/telegram/miniapp", map[string]any{"init_data": "soxta"})
		// Email kirishi bayroq ostida — yopiq (404).
		w.Gated(t, nil, "POST", "/auth/email/request-code", map[string]any{"email": "a@b.uz"}, 404)
		w.Gated(t, nil, "POST", "/auth/email/verify", map[string]any{"email": "a@b.uz", "code": "1"}, 404)
	})

	w.Step(t, "kafe kitoblari (Mongo) va 3D model (Tripo)", func(t *testing.T) {
		w.Gated(t, nil, "GET", "/restaurants/"+RestA+"/books", nil)
		w.Gated(t, o, "GET", "/restaurants/"+RestA+"/books/all", nil)
		w.Gated(t, o, "POST", "/restaurants/"+RestA+"/books", map[string]any{"title": "x"})
		w.Gated(t, nil, "GET", "/books/x", nil)
		w.Gated(t, o, "PATCH", "/books/x", map[string]any{})
		w.Gated(t, o, "DELETE", "/books/x", nil)
		w.Gated(t, o, "POST", "/restaurants/"+RestA+"/products/"+ProdOsh+"/model3d", nil)
		w.Gated(t, o, "DELETE", "/restaurants/"+RestA+"/products/"+ProdOsh+"/model3d", nil)
	})

	w.Step(t, "AI agent integratsiyasi (Postgres kerak) yopiq", func(t *testing.T) {
		w.Gated(t, nil, "GET", "/agent/v1/ping", nil)
		w.Gated(t, nil, "POST", "/agent/v1/link/start", map[string]any{"external_ref": "x"})
		w.Gated(t, nil, "POST", "/agent/v1/link/poll", map[string]any{"link_id": "x", "link_secret": "y"})
		w.Gated(t, nil, "GET", "/agent/v1/me", nil, 401, 503)
		w.Gated(t, nil, "GET", "/agent/v1/restaurants", nil, 401, 503)
		w.Gated(t, nil, "GET", "/agent/v1/restaurants/"+RestA+"/menu", nil, 401, 503)
		w.Gated(t, nil, "GET", "/agent/v1/products/search?q=osh", nil, 401, 503)
		w.Gated(t, nil, "POST", "/agent/v1/orders/draft", map[string]any{}, 401, 503)
		w.Gated(t, nil, "POST", "/agent/v1/orders/confirm", map[string]any{}, 401, 503)
		w.Gated(t, nil, "GET", "/agent/v1/orders", nil, 401, 503)
		w.Gated(t, nil, "GET", "/agent/v1/orders/x", nil, 401, 503)
		w.Gated(t, nil, "POST", "/agent/v1/orders/x/cancel", nil, 401, 503)
		for _, p := range [][2]string{
			{"GET", "/me/agent/grants"}, {"GET", "/me/agent/activity"}, {"GET", "/me/agent/drafts"},
			{"GET", "/me/agent/link/ABC"}, {"POST", "/me/agent/link/ABC/approve"}, {"POST", "/me/agent/link/ABC/deny"},
			{"DELETE", "/me/agent/grants/x"}, {"POST", "/me/agent/drafts/x/approve"}, {"POST", "/me/agent/drafts/x/reject"},
		} {
			w.Gated(t, c, p[0], p[1], map[string]any{}, 503, 404)
		}
	})

	w.Step(t, "Octo to'lov shlyuzi o'chirilgan — callback yopiq, buyurtmaga tegmaydi", func(t *testing.T) {
		w.Gated(t, nil, "POST", "/payments/octo/callback", map[string]any{}, 503, 400)
	})

	w.Step(t, "kuryer ovozli yo'l-yo'riq (Gemini) va kuryer qidiruvini qayta boshlash", func(t *testing.T) {
		courier, _ := addCourier(t, w, "+998901110041", "")
		w.Gated(t, courier, "GET", "/couriers/"+courier.Entity+"/voice/arrival", nil, 503, 400, 200)
		w.ExpectDenied(t, c, "GET", "/couriers/"+courier.Entity+"/voice/arrival", nil)

		// Yangi buyurtmada kuryer qidiruvini qayta boshlab bo'lmaydi (holat mos emas).
		ord := w.Do(t, c, "POST", "/orders", map[string]any{
			"items": []map[string]any{{"product_id": ProdOsh, "qty": 1}}, "payment_method": "cash",
			"idempotency_key": "e2e-retry-1",
		}).Expect(t, 201).Map(t)
		id, _ := ord["id"].(string)
		w.Do(t, o, "POST", "/orders/"+id+"/dispatch/retry", nil).Expect(t, 409)
		w.ExpectDenied(t, w.OwnerB, "POST", "/orders/"+id+"/dispatch/retry", nil)
		w.ExpectDenied(t, c, "POST", "/orders/"+id+"/dispatch/retry", nil)
	})
}
