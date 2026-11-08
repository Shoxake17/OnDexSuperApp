package e2e

import (
	"context"
	"testing"

	"chustapp/internal/couriers"
)

// ssenariy: SUPERADMIN — restoran yaratish/tahrirlash/o'chirish, kuryer
// tasdig'i, ro'yxatlar, statistika va boshqa rollarga yopiqligi.

func TestAdminFlow(t *testing.T) {
	w := NewWorld(t, "Superadmin (restoran, kuryer, akkaunt, statistika)")
	var newRest string

	w.Step(t, "ro'yxatlar va statistika", func(t *testing.T) {
		for _, p := range []string{
			"/admin/stats", "/admin/stats?period=week", "/admin/orders", "/admin/orders?restaurant_id=" + RestA,
			"/admin/accounts?role=restaurant", "/admin/accounts?role=courier", "/admin/accounts?role=waiter",
			"/admin/couriers", "/admin/customers", "/admin/waiters",
		} {
			w.Do(t, w.Admin, "GET", p, nil).Expect(t, 200)
		}
	})

	w.Step(t, "admin boshqa rollar uchun yopiq endpointlarni boshqalar ochmaydi", func(t *testing.T) {
		for _, a := range []*Actor{w.Customer, w.Owner, nil} {
			w.ExpectDenied(t, a, "GET", "/admin/stats", nil)
			w.ExpectDenied(t, a, "GET", "/admin/customers", nil)
			w.ExpectDenied(t, a, "POST", "/admin/restaurants", map[string]any{"name": "x"})
			w.ExpectDenied(t, a, "DELETE", "/admin/restaurants/"+RestA, nil)
		}
	})

	w.Step(t, "do'kon yaratish, ro'yxatda ko'rish va o'chirish", func(t *testing.T) {
		for _, a := range []*Actor{w.Customer, w.Owner, nil} {
			w.ExpectDenied(t, a, "POST", "/admin/stores", map[string]any{"name": "x", "phone": "+998905550099"})
			w.ExpectDenied(t, a, "DELETE", "/admin/stores/x", nil)
			w.ExpectDenied(t, a, "POST", "/admin/stores/x", map[string]any{"category": "qurt"})
		}
		w.Do(t, w.Admin, "POST", "/admin/stores", map[string]any{"name": ""}).Expect(t, 400)
		w.Do(t, w.Admin, "POST", "/admin/stores", map[string]any{"name": "x", "phone": "+998905550078"}).Expect(t, 400) // kategoriyasiz
		r := w.Do(t, w.Admin, "POST", "/admin/stores", map[string]any{
			"name": "Baraka do'koni", "phone": "+998905550077", "category": "qurt", "password": "Do'kon-parol-123",
		}).Expect(t, 201).Map(t)
		store, _ := r["store"].(map[string]any)
		id, _ := store["id"].(string)
		if id == "" {
			t.Fatalf("do'kon ID'si qaytmadi: %v", r)
		}
		w.Do(t, w.Admin, "GET", "/admin/accounts?role=store", nil).Expect(t, 200)
		// Kategoriyani o'zgartirish (qurt -> store) va noto'g'ri qiymat
		w.Do(t, w.Admin, "POST", "/admin/stores/"+id, map[string]any{"category": "store", "name": "Baraka yangi"}).Expect(t, 200)
		w.Do(t, w.Admin, "POST", "/admin/stores/"+id, map[string]any{"category": "supermarket"}).Expect(t, 400)
		w.Do(t, w.Admin, "POST", "/admin/stores/yoq", map[string]any{"category": "qurt"}).Expect(t, 404)
		w.Do(t, w.Admin.WithReason("e2e: tozalash"), "DELETE", "/admin/stores/"+id, nil).Expect(t, 200)
		w.Do(t, w.Admin.WithReason("e2e: tozalash"), "DELETE", "/admin/stores/"+id, nil).Expect(t, 404)
	})

	w.Step(t, "restoran yaratish (akkaunt bilan) va tahrirlash", func(t *testing.T) {
		// Noto'g'ri kirishlar rad etiladi.
		w.Do(t, w.Admin, "POST", "/admin/restaurants", map[string]any{"name": ""}).Expect(t, 400)
		r := w.Do(t, w.Admin, "POST", "/admin/restaurants", map[string]any{
			"name": "Yangi Oshxona", "address": "Chust, Mustaqillik 1", "lat": ChustLat, "lng": ChustLng,
			"phone": "+998905550001", "staff_name": "Egasi",
		}).Expect(t, 201).Map(t)
		rest, _ := r["restaurant"].(map[string]any)
		if rest == nil {
			rest = r
		}
		newRest, _ = rest["id"].(string)
		if newRest == "" {
			t.Fatalf("yangi restoran ID'si yo'q: %v", r)
		}
		w.Do(t, w.Admin, "POST", "/admin/restaurants/"+newRest, map[string]any{
			"name": "Yangi Oshxona (tahrir)", "address": "Chust, Mustaqillik 1", "lat": ChustLat, "lng": ChustLng,
			"tags": "osh,somsa", "eta_min_minutes": 20, "eta_max_minutes": 40,
		}).Expect(t, 200)
		pub := w.Do(t, w.Customer, "GET", "/restaurants/"+newRest, nil).Expect(t, 200)
		if !contains(pub.Body, "Yangi Oshxona (tahrir)") {
			t.Fatalf("tahrir saqlanmadi: %s", pub.Body)
		}
		w.Do(t, w.Admin, "POST", "/admin/restaurants/"+newRest+"/open", map[string]any{"open": false}).Expect(t, 200)
	})

	w.Step(t, "kuryer tasdig'i: tasdiqlanmagan kuryer ishga chiqa olmaydi", func(t *testing.T) {
		courier, staffID := addCourier(t, w, "+998901110021", "")
		// Restoranning o'z kuryerining kirishini ADMIN emas, restoran
		// ("Xodimlar") boshqaradi — admin tasdig'i bu yerda 409.
		w.Do(t, w.Admin, "POST", "/admin/couriers/"+courier.Entity+"/approve", map[string]any{"approved": false}).Expect(t, 409)
		w.ExpectDenied(t, w.Customer, "POST", "/admin/couriers/"+courier.Entity+"/approve", map[string]any{"approved": true})
		// Ta'tilga chiqarilgan kuryer smenaga chiqa olmaydi.
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/staff/"+staffID+"/status", map[string]any{"status": "on_leave"}).Expect(t, 200)
		if r := w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/available", map[string]any{"available": true}); r.Status != 401 && r.Status != 403 {
			w.Violate(t, "security.on_leave_courier_cannot_work", "ta'tildagi kuryer smenaga chiqdi: %d", r.Status)
		}
		w.Do(t, w.Owner, "POST", "/restaurants/"+RestA+"/staff/"+staffID+"/status", map[string]any{"status": "active"}).Expect(t, 200)
		courier = w.LoginAs(t, "kuryer", "+998901110021")
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/available", map[string]any{"available": true}).Expect(t, 200)
	})

	w.Step(t, "OnDex platforma kuryeri: admin tasdiqlaydi/bekor qiladi", func(t *testing.T) {
		// Restoran xodimi bo'lmagan (platforma) kuryer — tasdiqni ADMIN beradi.
		if err := w.Couriers.Create(context.Background(), &couriers.Courier{ID: "plat-1", Name: "Platforma kuryeri"}); err != nil {
			t.Fatal(err)
		}
		w.Do(t, w.Admin, "POST", "/admin/couriers/plat-1/approve", map[string]any{"approved": true}).Expect(t, 200)
		list := w.Do(t, w.Admin, "GET", "/admin/couriers", nil).Expect(t, 200)
		if !contains(list.Body, "plat-1") {
			t.Fatalf("platforma kuryeri ro'yxatda yo'q: %s", list.Body)
		}
		w.Do(t, w.Admin, "POST", "/admin/couriers/plat-1/approve", map[string]any{"approved": false}).Expect(t, 200)
		w.Do(t, w.Admin, "POST", "/admin/couriers/yoq-kuryer/approve", map[string]any{"approved": true}).Expect(t, 404)
	})

	w.Step(t, "kuryer o'z-o'zidan ro'yxatdan o'ta olmaydi (OnDex kuryerlari yopiq)", func(t *testing.T) {
		w.Do(t, w.Customer, "POST", "/couriers/register", map[string]any{"name": "Xohlovchi"}).Expect(t, 410)
	})

	w.Step(t, "akkauntni o'chirish va restoranni o'chirish", func(t *testing.T) {
		w.Do(t, w.Admin.WithReason("e2e: tozalash"), "DELETE", "/admin/users/"+w.Other.UserID, nil).Expect(t, 200)
		w.ExpectDenied(t, w.Customer, "DELETE", "/admin/users/"+w.Customer.UserID, nil)
		w.Do(t, w.Admin.WithReason("e2e: tozalash"), "DELETE", "/admin/restaurants/"+newRest, nil).Expect(t, 200)
		w.Do(t, w.Customer, "GET", "/restaurants/"+newRest, nil).Expect(t, 404)
	})
}

// ssenariy: ADMIN — xarita manbasini JONLI almashtiradi
// (`GET/POST /admin/config/map-mode`, `mapsconfig.go`). Server qayta
// ishga tushirilmasdan: tanlov xotirada darhol qo'llanadi.
func TestAdminMapModeToggle(t *testing.T) {
	w := NewWorld(t, "Admin — xarita manbasini jonli almashtirish")

	w.Step(t, "boshqa rollar o'zgartira olmaydi", func(t *testing.T) {
		for _, a := range []*Actor{w.Customer, w.Owner, nil} {
			w.ExpectDenied(t, a, "GET", "/admin/config/map-mode", nil)
			w.ExpectDenied(t, a, "POST", "/admin/config/map-mode", map[string]any{"mode": "ondexmap"})
		}
	})

	w.Step(t, "noto'g'ri qiymat rad etiladi", func(t *testing.T) {
		w.Do(t, w.Admin, "POST", "/admin/config/map-mode", map[string]any{"mode": "yandex"}).Expect(t, 400)
	})

	w.Step(t, "admin ondexmap'ga o'tkazadi — DARHOL qo'llanadi", func(t *testing.T) {
		r := w.Do(t, w.Admin, "POST", "/admin/config/map-mode", map[string]any{"mode": "ondexmap"}).Expect(t, 200).Map(t)
		if r["mode"] != "ondexmap" {
			t.Fatalf("mode=ondexmap kutilgan, keldi: %v", r)
		}
		got := w.Do(t, w.Admin, "GET", "/admin/config/map-mode", nil).Expect(t, 200).Map(t)
		if got["mode"] != "ondexmap" {
			t.Fatalf("GET ham ondexmap qaytarishi kerak edi: %v", got)
		}
	})

	w.Step(t, "admin google'ga qaytaradi", func(t *testing.T) {
		r := w.Do(t, w.Admin, "POST", "/admin/config/map-mode", map[string]any{"mode": "google"}).Expect(t, 200).Map(t)
		if r["mode"] != "google" {
			t.Fatalf("mode=google kutilgan, keldi: %v", r)
		}
	})
}

// ssenariy: ADMIN — reklama bannerlari (`internal/banners`): yaratish,
// tahrirlash, o'chirish, boshqa rollarga yopiqligi va mijozga ochiq
// ro'yxatdan o'z vaqtida yo'qolishi.
func TestAdminBanners(t *testing.T) {
	w := NewWorld(t, "Admin — reklama bannerlari")
	var bannerID string

	w.Step(t, "boshqa rollar bannerni boshqara olmaydi", func(t *testing.T) {
		for _, a := range []*Actor{w.Customer, w.Owner, nil} {
			w.ExpectDenied(t, a, "GET", "/admin/banners", nil)
			w.ExpectDenied(t, a, "POST", "/admin/banners", map[string]any{"image_url": "https://x/1.webp"})
		}
	})

	w.Step(t, "yaratish (rasm majburiy) va tahrirlash", func(t *testing.T) {
		w.Do(t, w.Admin, "POST", "/admin/banners", map[string]any{"image_url": ""}).Expect(t, 400)
		r := w.Do(t, w.Admin, "POST", "/admin/banners", map[string]any{
			"image_url": "https://x/1.webp", "restaurant_id": RestA, "active": true,
		}).Expect(t, 201).Map(t)
		bannerID, _ = r["id"].(string)
		if bannerID == "" {
			t.Fatalf("yangi banner ID'si yo'q: %v", r)
		}
		list := w.Do(t, w.Admin, "GET", "/admin/banners", nil).Expect(t, 200)
		if !contains(list.Body, bannerID) {
			t.Fatalf("yangi banner ro'yxatda yo'q: %s", list.Body)
		}
		w.Do(t, w.Admin, "POST", "/admin/banners/"+bannerID, map[string]any{
			"image_url": "https://x/2.webp", "active": true, "sort_order": 3,
		}).Expect(t, 200)
		pub := w.Do(t, w.Customer, "GET", "/promo-banners", nil).Expect(t, 200)
		if !contains(pub.Body, "https://x/2.webp") {
			t.Fatalf("tahrir ochiq ro'yxatda ko'rinmadi: %s", pub.Body)
		}
	})

	w.Step(t, "o'chirish — ochiq ro'yxatdan yo'qoladi", func(t *testing.T) {
		w.ExpectDenied(t, w.Customer, "DELETE", "/admin/banners/"+bannerID, nil)
		w.Do(t, w.Admin, "DELETE", "/admin/banners/"+bannerID, nil).Expect(t, 200)
		w.Do(t, w.Admin, "DELETE", "/admin/banners/yoq-banner", nil).Expect(t, 404)
		pub := w.Do(t, w.Customer, "GET", "/promo-banners", nil).Expect(t, 200)
		if contains(pub.Body, bannerID) {
			t.Fatalf("o'chirilgan banner hali ko'rinyapti: %s", pub.Body)
		}
	})
}

// ssenariy: OnDex PLATFORMA KURYERLARI (bayroq yoqilganda) — o'zi ro'yxatdan
// o'tadi, admin tasdig'ini kutadi, tasdiqlangach ishga chiqadi.
func TestPlatformCourierRegistration(t *testing.T) {
	w := NewWorldWith(t, "Platforma kuryeri ro'yxatdan o'tishi (bayroq yoqiq)", Options{PlatformCouriers: true})
	var courier *Actor

	w.Step(t, "mijoz kuryer bo'lib ro'yxatdan o'tadi (ism majburiy)", func(t *testing.T) {
		w.Do(t, w.Customer, "POST", "/couriers/register", map[string]any{"name": ""}).Expect(t, 400)
		r := w.Do(t, w.Customer, "POST", "/couriers/register", map[string]any{"name": "Platforma Jasur", "vehicle_type": "moped"}).Expect(t, 201).Map(t)
		c, _ := r["courier"].(map[string]any)
		id, _ := c["id"].(string)
		if id == "" || r["token"] != nil || r["relogin_required"] != true {
			t.Fatalf("A5: kuryer bor, eski token YO'Q, qayta kirish kerak: %v", r)
		}
		// OnDexGO'ga markaziy kirish (telefon kodi -> sessiya).
		courier = w.LoginAs(t, "platforma-kuryer", w.Customer.Phone)
		if courier.Entity != id {
			t.Fatalf("kuryer yozuvi: %q != %q", courier.Entity, id)
		}
		// Faqat mijoz ro'yxatdan o'ta oladi.
		w.ExpectDenied(t, w.Owner, "POST", "/couriers/register", map[string]any{"name": "x"})
	})

	w.Step(t, "tasdiqlanmaguncha ishga chiqa olmaydi; tasdiqlangach chiqadi", func(t *testing.T) {
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/available", map[string]any{"available": true}).Expect(t, 403)
		w.Do(t, w.Admin, "POST", "/admin/couriers/"+courier.Entity+"/approve", map[string]any{"approved": true}).Expect(t, 200)
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/available", map[string]any{"available": true}).Expect(t, 200)
	})
}
