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
		w.Do(t, w.Admin, "DELETE", "/admin/users/"+w.Other.UserID, nil).Expect(t, 200)
		w.ExpectDenied(t, w.Customer, "DELETE", "/admin/users/"+w.Customer.UserID, nil)
		w.Do(t, w.Admin, "DELETE", "/admin/restaurants/"+newRest, nil).Expect(t, 200)
		w.Do(t, w.Customer, "GET", "/restaurants/"+newRest, nil).Expect(t, 404)
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
		tok, _ := r["token"].(string)
		id, _ := c["id"].(string)
		if id == "" || tok == "" {
			t.Fatalf("kuryer/token yo'q: %v", r)
		}
		courier = &Actor{Name: "platforma-kuryer", Token: tok, Entity: id}
		// Faqat mijoz ro'yxatdan o'ta oladi.
		w.ExpectDenied(t, w.Owner, "POST", "/couriers/register", map[string]any{"name": "x"})
	})

	w.Step(t, "tasdiqlanmaguncha ishga chiqa olmaydi; tasdiqlangach chiqadi", func(t *testing.T) {
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/available", map[string]any{"available": true}).Expect(t, 403)
		w.Do(t, w.Admin, "POST", "/admin/couriers/"+courier.Entity+"/approve", map[string]any{"approved": true}).Expect(t, 200)
		w.Do(t, courier, "POST", "/couriers/"+courier.Entity+"/available", map[string]any{"available": true}).Expect(t, 200)
	})
}
