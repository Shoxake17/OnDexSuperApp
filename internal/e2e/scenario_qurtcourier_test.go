package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/users"
)

// ssenariy: QURT KURYERI — do'kon (sotuvchi) logistikani O'ZI hal qiladi: o'z kuryerlarini qo'shadi,
// o'z buyurtmalarini biriktiradi, kuryer topshirgan naqd pulni qabul qiladi. Kuryer faqat O'Z
// buyurtmasini oladi/yetkazadi; mijoz ma'lumoti do'kondan olingach ochiladi; boshqa do'kon
// kuryeriga/buyurtmasiga tegib bo'lmaydi. Admin FAQAT statistika ko'radi.
// Oddiy (platforma/restoran) kuryerlar bu bo'limga KIRA OLMAYDI.

func TestQurtCourierFlow(t *testing.T) {
	w := NewWorld(t, "Qurt kuryeri: do'kon o'z logistikasini boshqaradi")

	newPOS := func(name, phone string) *Actor {
		r := w.Do(t, w.Admin, "POST", "/admin/stores", map[string]any{
			"name": name, "phone": phone, "category": "qurt",
		}).Expect(t, 201).Map(t)
		acc, _ := r["account"].(map[string]any)
		uid, _ := acc["id"].(string)
		if uid == "" {
			t.Fatalf("do'kon yaratilmadi: %v", r)
		}
		return &Actor{Name: name, Headers: map[string]string{
			"X-Client-Id": SSOClientID, "X-Client-Secret": SSOClientSecret, "X-Store-User": uid,
		}}
	}
	pos := newPOS("Kuryer sinovi qurtlari", "+998905553001")
	posB := newPOS("Boshqa do'kon", "+998905553002")
	w.Do(t, pos, "PUT", "/merchant/qurt/products/k1", map[string]any{
		"name": "Qurt k1", "category": "Sho'r", "price_tiyin": 1200000, "weight_grams": 200, "available": true, "stock": 20,
	}).Expect(t, 200)
	var prods struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	w.Do(t, nil, "GET", "/qurt/products", nil).Expect(t, 200).JSON(t, &prods)
	productID := ""
	for _, p := range prods.Items {
		if p.Name == "Qurt k1" {
			productID = p.ID
		}
	}
	if productID == "" {
		t.Fatal("mahsulot topilmadi")
	}

	newOrder := func(key string) string {
		var m struct {
			Orders []struct {
				ID string `json:"id"`
			} `json:"orders"`
		}
		w.Do(t, w.Customer, "POST", "/qurt/orders", map[string]any{
			"items": []map[string]any{{"product_id": productID, "qty": 2}}, "idempotency_key": key,
		}).Expect(t, 201).JSON(t, &m)
		if len(m.Orders) != 1 {
			t.Fatalf("1 buyurtma kutilgan: %+v", m)
		}
		id := m.Orders[0].ID
		for _, to := range []string{"confirmed", "packed"} {
			w.Do(t, pos, "POST", "/merchant/qurt/orders/"+id+"/transition", map[string]any{"to": to}).Expect(t, 200)
		}
		return id
	}
	orderA := newOrder("kc-1")

	var courier *Actor
	var courierID string
	w.Step(t, "do'kon o'z kuryerini qo'shadi (begona akkauntga tegmaydi)", func(t *testing.T) {
		body := map[string]any{"phone": "+998905553101", "name": "Qurt Kuryer"}
		w.Do(t, nil, "POST", "/merchant/qurt/couriers", body).Expect(t, 401)
		w.Do(t, w.Customer, "POST", "/merchant/qurt/couriers", body).Expect(t, 401) // JWT bu yo'lda yaramaydi
		w.Do(t, pos, "POST", "/merchant/qurt/couriers", map[string]any{"phone": "+998905553101", "name": "x"}).Expect(t, 400)
		w.Do(t, pos, "POST", "/merchant/qurt/couriers", map[string]any{"phone": "123", "name": "Qurt Kuryer"}).Expect(t, 400)
		// Restoran egasining raqami — boshqa rol: aylantirilmaydi.
		w.Do(t, pos, "POST", "/merchant/qurt/couriers", map[string]any{"phone": w.Owner.Phone, "name": "Qurt Kuryer"}).Expect(t, 409)

		r := w.Do(t, pos, "POST", "/merchant/qurt/couriers", body).Expect(t, 201).Map(t)
		courierID, _ = r["id"].(string)
		if !strings.HasPrefix(courierID, "qurt-") {
			t.Fatalf("kuryer ID: %v", r)
		}
		w.Do(t, pos, "POST", "/merchant/qurt/couriers", body).Expect(t, 201) // qayta qo'shish xavfsiz
		// Boshqa do'kon shu kuryerni o'ziniki qila olmaydi.
		w.Do(t, posB, "POST", "/merchant/qurt/couriers", body).Expect(t, 409)
		courier = w.LoginAs(t, "Qurt kuryeri", "+998905553101")

		list := w.Do(t, pos, "GET", "/merchant/qurt/couriers", nil).Expect(t, 200)
		if !strings.Contains(string(list.Body), courierID) || !strings.Contains(string(list.Body), "+998905553101") {
			t.Fatalf("ro'yxatda kuryer yo'q: %s", list.Body)
		}
		// Boshqa do'kon ro'yxatida yo'q.
		if other := w.Do(t, posB, "GET", "/merchant/qurt/couriers", nil).Expect(t, 200); strings.Contains(string(other.Body), courierID) {
			t.Fatalf("begona do'kon kuryerni ko'rdi: %s", other.Body)
		}
		// Dispatch'ga TEGILMAYDI: Qurt kuryeri alohida havzada ("qurt:<do'kon>").
		c, err := w.Couriers.GetByID(context.Background(), courierID)
		if err != nil || !strings.HasPrefix(c.RestaurantID, "qurt:") || !c.Approved {
			t.Fatalf("kuryer yozuvi: %+v %v", c, err)
		}
	})

	var plain *Actor
	w.Step(t, "oddiy kuryerga Qurt bo'limi yopiq", func(t *testing.T) {
		if err := w.Couriers.Create(context.Background(), &couriers.Courier{ID: "plain-1", Name: "Oddiy", Approved: true}); err != nil {
			t.Fatal(err)
		}
		if err := w.Users.Create(context.Background(), &users.User{
			ID: "plain-u", Phone: "+998905553102", Name: "Oddiy", Role: users.RoleCourier, EntityID: "plain-1", CreatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
		plain = w.LoginAs(t, "Oddiy kuryer", "+998905553102")
		w.ExpectDenied(t, plain, "GET", "/courier/qurt/me", nil)
		w.ExpectDenied(t, plain, "GET", "/courier/qurt/orders", nil)
		w.ExpectDenied(t, plain, "POST", "/courier/qurt/orders/"+orderA+"/transition", map[string]any{"to": "collected"})
		w.ExpectDenied(t, w.Customer, "GET", "/courier/qurt/me", nil)
		w.Do(t, nil, "GET", "/courier/qurt/orders", nil).Expect(t, 401)
	})

	w.Step(t, "biriktirilmagan buyurtma kuryerga ko'rinmaydi va uni o'zgartirib bo'lmaydi", func(t *testing.T) {
		r := w.Do(t, courier, "GET", "/courier/qurt/orders", nil).Expect(t, 200)
		if strings.Contains(string(r.Body), orderA) {
			t.Fatalf("biriktirilmagan buyurtma ko'rindi: %s", r.Body)
		}
		w.Do(t, courier, "POST", "/courier/qurt/orders/"+orderA+"/transition", map[string]any{"to": "collected"}).Expect(t, 404)
	})

	w.Step(t, "do'kon buyurtmasini o'z kuryeriga biriktiradi; begona do'kon tega olmaydi", func(t *testing.T) {
		assign := map[string]any{"courier_id": courierID, "order_ids": []string{orderA}}
		w.Do(t, nil, "POST", "/merchant/qurt/assign", assign).Expect(t, 401)
		w.Do(t, pos, "POST", "/merchant/qurt/assign", map[string]any{"courier_id": "yoq", "order_ids": []string{orderA}}).Expect(t, 400)
		w.Do(t, pos, "POST", "/merchant/qurt/assign", map[string]any{"courier_id": "plain-1", "order_ids": []string{orderA}}).Expect(t, 400) // oddiy kuryer
		w.Do(t, pos, "POST", "/merchant/qurt/assign", map[string]any{"courier_id": courierID, "date": "bad"}).Expect(t, 400)
		// Boshqa do'kon: kuryer uniki emas -> 400; buyurtma uniki emas.
		w.Do(t, posB, "POST", "/merchant/qurt/assign", assign).Expect(t, 400)
		w.Do(t, posB, "POST", "/merchant/qurt/orders/"+orderA+"/unassign", nil).Expect(t, 404)
		m := w.Do(t, pos, "POST", "/merchant/qurt/assign", assign).Expect(t, 200).Map(t)
		if m["assigned"] != float64(1) {
			t.Fatalf("biriktirish: %v", m)
		}
		// Do'kon buyurtmada kuryerni ko'radi (mijoz ma'lumotini esa YO'Q).
		r := w.Do(t, pos, "GET", "/merchant/qurt/orders/"+orderA, nil).Expect(t, 200)
		if !strings.Contains(string(r.Body), courierID) {
			t.Fatalf("do'kon kuryerni ko'rishi kerak: %s", r.Body)
		}
		for _, secret := range []string{"+99890", "Navoiy", "customer_phone"} {
			if strings.Contains(string(r.Body), secret) {
				t.Fatalf("do'konga %q sizdi: %s", secret, r.Body)
			}
		}
	})

	w.Step(t, "kuryer faqat o'z buyurtmasini ko'radi; mijoz ma'lumoti olishdan keyin ochiladi", func(t *testing.T) {
		before := w.Do(t, courier, "GET", "/courier/qurt/orders", nil).Expect(t, 200)
		if !strings.Contains(string(before.Body), orderA) {
			t.Fatalf("biriktirilgan buyurtma ko'rinmadi: %s", before.Body)
		}
		for _, secret := range []string{"+99890", "Navoiy", "customer_phone", "\"customer\""} {
			if strings.Contains(string(before.Body), secret) {
				t.Fatalf("do'kondan olishdan oldin kuryerga %q sizdi: %s", secret, before.Body)
			}
		}
		w.Do(t, courier, "POST", "/courier/qurt/orders/"+orderA+"/transition", map[string]any{"to": "confirmed"}).Expect(t, 400)
		w.Do(t, courier, "POST", "/courier/qurt/orders/"+orderA+"/transition", map[string]any{"to": "delivered"}).Expect(t, 409) // sakrab bo'lmaydi
		w.Do(t, courier, "POST", "/courier/qurt/orders/"+orderA+"/transition", map[string]any{"to": "collected"}).Expect(t, 200)
		after := w.Do(t, courier, "GET", "/courier/qurt/orders", nil).Expect(t, 200)
		if !strings.Contains(string(after.Body), "Navoiy") || !strings.Contains(string(after.Body), "+99890") {
			t.Fatalf("olingandan keyin manzil/telefon ochilishi kerak: %s", after.Body)
		}
		if strings.Contains(string(after.Body), "by_id") || strings.Contains(string(after.Body), "customer_id") {
			t.Fatalf("ichki maydonlar sizdi: %s", after.Body)
		}
	})

	w.Step(t, "boshqa do'konning kuryeri bu buyurtmaga tega olmaydi", func(t *testing.T) {
		w.Do(t, posB, "POST", "/merchant/qurt/couriers", map[string]any{"phone": "+998905553103", "name": "Ikkinchi"}).Expect(t, 201)
		other := w.LoginAs(t, "Boshqa do'kon kuryeri", "+998905553103")
		if r := w.Do(t, other, "GET", "/courier/qurt/orders", nil).Expect(t, 200); strings.Contains(string(r.Body), orderA) {
			t.Fatalf("begona buyurtma ko'rindi: %s", r.Body)
		}
		w.Do(t, other, "POST", "/courier/qurt/orders/"+orderA+"/transition", map[string]any{"to": "out_for_delivery"}).Expect(t, 404)
	})

	w.Step(t, "yo'lda -> yetkazib bo'lmadi (sabab shart) -> qayta -> yetkazildi: naqd pul kuryerda yoziladi", func(t *testing.T) {
		tr := func(to, reason string) *Resp {
			return w.Do(t, courier, "POST", "/courier/qurt/orders/"+orderA+"/transition", map[string]any{"to": to, "reason": reason})
		}
		tr("out_for_delivery", "").Expect(t, 200)
		tr("failed", "").Expect(t, 400)
		tr("failed", "Mijoz uyda yo'q").Expect(t, 200)
		tr("out_for_delivery", "").Expect(t, 200)
		tr("delivered", "").Expect(t, 200)
		tr("delivered", "").Expect(t, 409) // ikki marta emas

		me := w.Do(t, courier, "GET", "/courier/qurt/me", nil).Expect(t, 200).Map(t)
		sum, _ := me["summary"].(map[string]any)
		if sum["held_tiyin"] != float64(2400000) || sum["held_orders"] != float64(1) || sum["active_orders"] != float64(0) {
			t.Fatalf("kuryerdagi pul: %v", me)
		}
		// Do'kon hisobda ko'radi; mijozga pul/kuryer maydonlari chiqmaydi.
		list := w.Do(t, pos, "GET", "/merchant/qurt/couriers", nil).Expect(t, 200)
		if !strings.Contains(string(list.Body), `"held_tiyin":2400000`) {
			t.Fatalf("do'kon hisobi: %s", list.Body)
		}
		order := w.Do(t, pos, "GET", "/merchant/qurt/orders/"+orderA, nil).Expect(t, 200)
		if !strings.Contains(string(order.Body), `"cash_status":"held"`) {
			t.Fatalf("do'kon buyurtmada pul holatini ko'rishi kerak: %s", order.Body)
		}
		own := w.Do(t, w.Customer, "GET", "/qurt/orders/"+orderA, nil).Expect(t, 200)
		for _, secret := range []string{"cash_", "courier_id"} {
			if strings.Contains(string(own.Body), secret) {
				t.Fatalf("mijozga %q sizdi: %s", secret, own.Body)
			}
		}
	})

	w.Step(t, "do'kon pulni qabul qiladi: faqat o'z kuryeri, ikki marta sanalmaydi; pulli kuryerni o'chirib bo'lmaydi", func(t *testing.T) {
		path := "/merchant/qurt/couriers/" + courierID + "/settle"
		w.Do(t, nil, "POST", path, nil).Expect(t, 401)
		w.ExpectDenied(t, courier, "POST", path, nil)   // kuryer o'z pulini o'zi yopa olmaydi
		w.Do(t, posB, "POST", path, nil).Expect(t, 404) // begona do'kon
		w.Do(t, posB, "DELETE", "/merchant/qurt/couriers/"+courierID, nil).Expect(t, 404)
		w.Do(t, pos, "DELETE", "/merchant/qurt/couriers/"+courierID, nil).Expect(t, 409) // pul kuryerda

		m := w.Do(t, pos, "POST", path, nil).Expect(t, 200).Map(t)
		if m["orders"] != float64(1) || m["total_tiyin"] != float64(2400000) {
			t.Fatalf("qabul: %v", m)
		}
		m = w.Do(t, pos, "POST", path, nil).Expect(t, 200).Map(t)
		if m["orders"] != float64(0) || m["total_tiyin"] != float64(0) {
			t.Fatalf("takroriy qabul pulni ikkilantirdi: %v", m)
		}
		me := w.Do(t, courier, "GET", "/courier/qurt/me", nil).Expect(t, 200).Map(t)
		if sum, _ := me["summary"].(map[string]any); sum["held_tiyin"] != float64(0) {
			t.Fatalf("pul qolmasligi kerak: %v", me)
		}
	})

	w.Step(t, "admin FAQAT statistika ko'radi", func(t *testing.T) {
		w.Do(t, nil, "GET", "/admin/qurt/stats", nil).Expect(t, 401)
		w.ExpectDenied(t, w.Customer, "GET", "/admin/qurt/stats", nil)
		w.ExpectDenied(t, w.Owner, "GET", "/admin/qurt/stats", nil)
		m := w.Do(t, w.Admin, "GET", "/admin/qurt/stats", nil).Expect(t, 200).Map(t)
		if m["sold_tiyin"] != float64(2400000) || m["cash_settled_tiyin"] != float64(2400000) || m["cash_held_tiyin"] != float64(0) || m["couriers"] != float64(2) {
			t.Fatalf("statistika: %v", m)
		}
		// Admin logistikani boshqarmaydi: kuryer qo'shish/biriktirish/pul qabul qilish endpointlari yo'q.
		w.Do(t, w.Admin, "POST", "/admin/qurt/couriers", map[string]any{"phone": "+998905553199", "name": "Admin"}).Expect(t, 404)
		w.Do(t, w.Admin, "POST", "/admin/qurt/assign", map[string]any{"courier_id": courierID}).Expect(t, 404)
	})

	w.Step(t, "yo'lga chiqmagan buyurtmani kuryerdan olish; kuryerni olib tashlash", func(t *testing.T) {
		// Qadoqlangan buyurtma kuryerga O'ZI TUSHMAYDI: do'kon uni qo'lda biriktiradi.
		orderB := newOrder("kc-2")
		if r := w.Do(t, courier, "GET", "/courier/qurt/orders", nil).Expect(t, 200); strings.Contains(string(r.Body), orderB) {
			t.Fatalf("buyurtma avtomatik biriktirilib ketdi: %s", r.Body)
		}
		w.Do(t, pos, "POST", "/merchant/qurt/assign", map[string]any{"courier_id": courierID, "order_ids": []string{orderB}}).Expect(t, 200)
		if r := w.Do(t, courier, "GET", "/courier/qurt/orders", nil).Expect(t, 200); !strings.Contains(string(r.Body), orderB) {
			t.Fatalf("qo'lda biriktirilgan buyurtma kuryerda ko'rinmadi: %s", r.Body)
		}
		w.Do(t, pos, "DELETE", "/merchant/qurt/couriers/"+courierID, nil).Expect(t, 409) // faol buyurtma bor
		w.Do(t, pos, "POST", "/merchant/qurt/orders/"+orderB+"/unassign", nil).Expect(t, 204)
		if r := w.Do(t, courier, "GET", "/courier/qurt/orders", nil).Expect(t, 200); strings.Contains(string(r.Body), orderB) {
			t.Fatalf("olingan buyurtma hali kuryerda: %s", r.Body)
		}
		w.Do(t, pos, "DELETE", "/merchant/qurt/couriers/yoq", nil).Expect(t, 404)
		w.Do(t, pos, "DELETE", "/merchant/qurt/couriers/"+courierID, nil).Expect(t, 204)
		if list := w.Do(t, pos, "GET", "/merchant/qurt/couriers", nil).Expect(t, 200); strings.Contains(string(list.Body), courierID) {
			t.Fatalf("o'chirilgan kuryer ro'yxatda: %s", list.Body)
		}
	})
}
