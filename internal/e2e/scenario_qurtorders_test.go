package e2e

import (
	"strings"
	"testing"
)

// ssenariy: QURT BUYURTMASI — mijoz savat beradi, do'kon (POS) tasdiqlaydi/qadoqlaydi,
// admin (logistika) reys bo'yicha yig'ib yetkazadi. Naqd to'lov; OnDex GO kuryerlari
// bu oqimda ishtirok etmaydi.
//
// Asosiy e'tibor: qoldiq ATOMIK band qilinishi, begona do'kon/mijoz buyurtmasiga
// tegib bo'lmasligi, do'konga mijoz telefoni/manzili CHIQMASLIGI, holat tartibi.

func TestQurtOrderFlow(t *testing.T) {
	w := NewWorld(t, "Qurt buyurtmasi: mijoz -> do'kon -> logistika")

	merchant := func(userID string) *Actor {
		return &Actor{Name: "POS serveri", Headers: map[string]string{
			"X-Client-Id": SSOClientID, "X-Client-Secret": SSOClientSecret, "X-Store-User": userID,
		}}
	}
	newStore := func(name, phone string) (userID string) {
		r := w.Do(t, w.Admin, "POST", "/admin/stores", map[string]any{
			"name": name, "phone": phone, "category": "qurt",
		}).Expect(t, 201).Map(t)
		acc, _ := r["account"].(map[string]any)
		userID, _ = acc["id"].(string)
		if userID == "" {
			t.Fatalf("do'kon yaratilmadi: %v", r)
		}
		return userID
	}
	posA := merchant(newStore("Urgut qurtlari", "+998905552001"))
	posB := merchant(newStore("Chust qurtlari", "+998905552002"))

	put := func(pos *Actor, sku string, price int, stock int) {
		w.Do(t, pos, "PUT", "/merchant/qurt/products/"+sku, map[string]any{
			"name": "Qurt " + sku, "category": "Sho'r", "price_tiyin": price, "weight_grams": 200,
			"available": stock > 0, "stock": stock,
		}).Expect(t, 200)
	}
	put(posA, "a1", 1200000, 10)
	put(posB, "b1", 1500000, 3)

	type product struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		StockLeft int    `json:"stock_left"`
	}
	products := func() map[string]product {
		var m struct {
			Items []product `json:"items"`
		}
		w.Do(t, nil, "GET", "/qurt/products", nil).Expect(t, 200).JSON(t, &m)
		out := map[string]product{}
		for _, p := range m.Items {
			out[p.Name] = p
		}
		return out
	}
	pa, pb := products()["Qurt a1"], products()["Qurt b1"]
	if pa.ID == "" || pb.ID == "" || pa.StockLeft != 10 || pb.StockLeft != 3 {
		t.Fatalf("mahsulotlar/qoldiq: %+v %+v", pa, pb)
	}

	type order struct {
		ID           string `json:"id"`
		Number       string `json:"number"`
		GroupID      string `json:"group_id"`
		StoreName    string `json:"store_name"`
		Status       string `json:"status"`
		DeliveryDate string `json:"delivery_date"`
		TotalTiyin   int64  `json:"total_tiyin"`
	}
	create := func(body map[string]any) *Resp { return w.Do(t, w.Customer, "POST", "/qurt/orders", body) }
	lines := func(qa, qb int) []map[string]any {
		var l []map[string]any
		if qa > 0 {
			l = append(l, map[string]any{"product_id": pa.ID, "qty": qa})
		}
		if qb > 0 {
			l = append(l, map[string]any{"product_id": pb.ID, "qty": qb})
		}
		return l
	}

	w.Step(t, "savatni yangilash: ?ids= hozirgi narx va qoldiqni beradi", func(t *testing.T) {
		var m struct {
			Items []product `json:"items"`
		}
		w.Do(t, nil, "GET", "/qurt/products?ids="+pa.ID+","+pb.ID+",yoq-id", nil).Expect(t, 200).JSON(t, &m)
		if len(m.Items) != 2 {
			t.Fatalf("2 ta topilishi kerak (yo'q ID tushib qoladi): %+v", m.Items)
		}
		w.Do(t, nil, "GET", "/qurt/products?ids="+strings.Repeat("a,", 60)+"a", nil).Expect(t, 400)
		w.Do(t, nil, "GET", "/qurt/products?ids=bad%20id", nil).Expect(t, 400)
		// Sotuvdan chiqqan mahsulot javobda yo'q.
		w.Do(t, posB, "PUT", "/merchant/qurt/products/b1", map[string]any{
			"name": "Qurt b1", "category": "Sho'r", "price_tiyin": 1500000, "weight_grams": 200,
			"available": false, "stock": 0}).Expect(t, 200)
		w.Do(t, nil, "GET", "/qurt/products?ids="+pb.ID, nil).Expect(t, 200).JSON(t, &m)
		if len(m.Items) != 0 {
			t.Fatalf("sotuvda yo'q mahsulot qaytdi: %+v", m.Items)
		}
		put(posB, "b1", 1500000, 3) // qaytaramiz
	})

	w.Step(t, "reys ma'lumoti ochiq", func(t *testing.T) {
		m := w.Do(t, nil, "GET", "/qurt/delivery-info", nil).Expect(t, 200).Map(t)
		if d, _ := m["delivery_date"].(string); len(d) != 10 || m["payment_method"] != "cash" {
			t.Fatalf("delivery-info: %v", m)
		}
	})

	w.Step(t, "buyurtma faqat mijozga va to'g'ri qiymat bilan", func(t *testing.T) {
		w.Do(t, nil, "POST", "/qurt/orders", map[string]any{"items": lines(1, 0)}).Expect(t, 401)
		w.ExpectDenied(t, w.Owner, "POST", "/qurt/orders", map[string]any{"items": lines(1, 0)})
		w.ExpectDenied(t, w.Admin, "POST", "/qurt/orders", map[string]any{"items": lines(1, 0)})
		create(map[string]any{"items": []any{}}).Expect(t, 400)
		create(map[string]any{"items": lines(0, 0)}).Expect(t, 400)
		create(map[string]any{"items": []map[string]any{{"product_id": pa.ID, "qty": 0}}}).Expect(t, 400)
		create(map[string]any{"items": []map[string]any{{"product_id": pa.ID, "qty": 101}}}).Expect(t, 400)
		create(map[string]any{"items": lines(1, 0), "payment_method": "card"}).Expect(t, 400)
		r := create(map[string]any{"items": []map[string]any{{"product_id": "yoq", "qty": 1}}}).Expect(t, 409)
		if !strings.Contains(string(r.Body), "out_of_stock") {
			t.Fatalf("out_of_stock kodi yo'q: %s", r.Body)
		}
		// Ekrandagi jami serverdagi hisobdan farq qilsa — yaratilmaydi, yangi summa qaytadi.
		r = create(map[string]any{"items": lines(1, 0), "expected_total_tiyin": 1}).Expect(t, 409)
		if !strings.Contains(string(r.Body), "total_changed") || !strings.Contains(string(r.Body), "1200000") {
			t.Fatalf("total_changed va yangi summa kutilgan: %s", r.Body)
		}
		if p := products(); p["Qurt a1"].StockLeft != 10 {
			t.Fatalf("rad etilgan so'rov qoldiqni band qildi: %+v", p)
		}
	})

	w.Step(t, "qoldiqdan ko'p so'ralsa hech narsa yaratilmaydi", func(t *testing.T) {
		create(map[string]any{"items": lines(2, 4)}).Expect(t, 409) // b1 da faqat 3
		if p := products(); p["Qurt a1"].StockLeft != 10 || p["Qurt b1"].StockLeft != 3 {
			t.Fatalf("yarim band qoldi: %+v", p)
		}
		var m struct {
			Orders []order `json:"orders"`
		}
		w.Do(t, w.Customer, "GET", "/qurt/orders", nil).Expect(t, 200).JSON(t, &m)
		if len(m.Orders) != 0 {
			t.Fatalf("buyurtma yaratilmasligi kerak: %+v", m.Orders)
		}
	})

	var orders []order
	w.Step(t, "savat do'konlar bo'yicha bo'linadi, qoldiq band qilinadi", func(t *testing.T) {
		var m struct {
			Orders []order `json:"orders"`
		}
		create(map[string]any{"items": lines(2, 3), "idempotency_key": "k-1", "comment": "Eshik oldiga qo'ying",
			"expected_total_tiyin": 6900000}).
			Expect(t, 201).JSON(t, &m)
		orders = m.Orders
		if len(orders) != 2 || orders[0].GroupID != orders[1].GroupID || orders[0].GroupID == "" {
			t.Fatalf("2 ta buyurtma, bitta guruh kutilgan: %+v", orders)
		}
		for _, o := range orders {
			if o.Status != "new" || o.DeliveryDate == "" || o.Number == "" {
				t.Fatalf("boshlang'ich holat: %+v", o)
			}
			if o.StoreName == "Urgut qurtlari" && o.TotalTiyin != 2400000 {
				t.Errorf("A jami: %d", o.TotalTiyin)
			}
			if o.StoreName == "Chust qurtlari" && o.TotalTiyin != 4500000 {
				t.Errorf("B jami: %d", o.TotalTiyin)
			}
		}
		if p := products(); p["Qurt a1"].StockLeft != 8 || p["Qurt b1"].StockLeft != 0 {
			t.Fatalf("band qilinmadi: %+v", p)
		}
		// Takroriy so'rov (tarmoq qayta urinishi) — avvalgi natija, qayta band qilinmaydi.
		var again struct {
			Orders []order `json:"orders"`
		}
		create(map[string]any{"items": lines(2, 3), "idempotency_key": "k-1"}).Expect(t, 201).JSON(t, &again)
		if len(again.Orders) != 2 || again.Orders[0].ID != orders[0].ID {
			t.Fatalf("idempotentlik buzildi: %+v", again.Orders)
		}
		if p := products(); p["Qurt a1"].StockLeft != 8 {
			t.Fatalf("ikki marta band qilindi: %+v", p)
		}
	})

	byStore := func(name string) order {
		for _, o := range orders {
			if o.StoreName == name {
				return o
			}
		}
		t.Fatalf("%s buyurtmasi yo'q", name)
		return order{}
	}
	oa, ob := byStore("Urgut qurtlari"), byStore("Chust qurtlari")

	w.Step(t, "mijoz o'z buyurtmalarini ko'radi", func(t *testing.T) {
		var m struct {
			Orders []order `json:"orders"`
		}
		w.Do(t, w.Customer, "GET", "/qurt/orders", nil).Expect(t, 200).JSON(t, &m)
		if len(m.Orders) != 2 {
			t.Fatalf("2 ta buyurtma kutilgan: %d", len(m.Orders))
		}
		own := w.Do(t, w.Customer, "GET", "/qurt/orders/"+oa.ID, nil).Expect(t, 200)
		// POS kaliti (shtrix-kod) mijozga CHIQMAYDI.
		for _, secret := range []string{"barcode", "external"} {
			if strings.Contains(string(own.Body), secret) {
				t.Fatalf("mijozga %q sizdi: %s", secret, own.Body)
			}
		}
		w.Do(t, w.Customer, "GET", "/qurt/orders/yoq", nil).Expect(t, 404)
		w.Do(t, nil, "GET", "/qurt/orders", nil).Expect(t, 401)
		w.ExpectDenied(t, w.Owner, "GET", "/qurt/orders/"+oa.ID, nil)
	})

	w.Step(t, "do'kon faqat o'z buyurtmasini ko'radi — mijoz ma'lumotisiz", func(t *testing.T) {
		w.Do(t, nil, "GET", "/merchant/qurt/orders", nil).Expect(t, 401)
		w.Do(t, w.Customer, "GET", "/merchant/qurt/orders", nil).Expect(t, 401) // JWT bu yo'lda yaramaydi
		r := w.Do(t, posA, "GET", "/merchant/qurt/orders?status=new", nil).Expect(t, 200)
		var m struct {
			Orders []map[string]any `json:"orders"`
		}
		r.JSON(t, &m)
		if len(m.Orders) != 1 || m.Orders[0]["id"] != oa.ID {
			t.Fatalf("A faqat o'z buyurtmasini ko'rishi kerak: %s", r.Body)
		}
		// Do'kon O'Z mahsulot kalitini oladi (POS ombor qoldig'ini kamaytirish uchun).
		if !strings.Contains(string(r.Body), `"barcode":"a1"`) {
			t.Fatalf("do'kon barcode ko'rishi kerak: %s", r.Body)
		}
		for _, secret := range []string{"+99890", "Navoiy", "Eshik oldiga", "customer_phone", "address", "by_id"} {
			if strings.Contains(string(r.Body), secret) {
				t.Fatalf("do'konga %q sizdi: %s", secret, r.Body)
			}
		}
		w.Do(t, posA, "GET", "/merchant/qurt/orders?status=bogus", nil).Expect(t, 400)
		own := w.Do(t, posA, "GET", "/merchant/qurt/orders/"+oa.ID, nil).Expect(t, 200)
		for _, secret := range []string{"+99890", "Navoiy", "customer_phone"} {
			if strings.Contains(string(own.Body), secret) {
				t.Fatalf("bitta buyurtma javobida %q sizdi: %s", secret, own.Body)
			}
		}
		// Begona do'kon buyurtmasi mavjud emasdek.
		w.Do(t, posA, "GET", "/merchant/qurt/orders/"+ob.ID, nil).Expect(t, 404)
		w.Do(t, posA, "POST", "/merchant/qurt/orders/"+ob.ID+"/transition", map[string]any{"to": "confirmed"}).Expect(t, 404)
	})

	w.Step(t, "do'kon: tasdiqlash, rad etish (sabab shart), qadoqlash tartibi", func(t *testing.T) {
		tr := func(pos *Actor, id, to, reason string) *Resp {
			return w.Do(t, pos, "POST", "/merchant/qurt/orders/"+id+"/transition", map[string]any{"to": to, "reason": reason})
		}
		tr(posA, oa.ID, "packed", "").Expect(t, 409)    // new -> packed mumkin emas
		tr(posA, oa.ID, "collected", "").Expect(t, 409) // do'kon logistika qadamini bajara olmaydi
		tr(posA, oa.ID, "delivered", "").Expect(t, 409)
		tr(posA, oa.ID, "bogus", "").Expect(t, 409)
		tr(posA, oa.ID, "confirmed", "").Expect(t, 200)
		if p := products(); p["Qurt a1"].StockLeft != 10 {
			t.Fatalf("tasdiqlangach band qaytishi kerak (POS qoldiqni o'zi kamaytiradi): %+v", p)
		}
		tr(posA, oa.ID, "packed", "").Expect(t, 200)

		tr(posB, ob.ID, "rejected", "").Expect(t, 400) // sababsiz rad etib bo'lmaydi
		tr(posB, ob.ID, "rejected", "Qoldiq tugagan").Expect(t, 200)
		if p := products(); p["Qurt b1"].StockLeft != 3 {
			t.Fatalf("rad etilgach band qaytishi kerak: %+v", p)
		}
		tr(posB, ob.ID, "confirmed", "").Expect(t, 409) // terminal
	})

	w.Step(t, "mijoz tasdiqlangan buyurtmani bekor qila olmaydi, yangisini oladi", func(t *testing.T) {
		w.Do(t, w.Customer, "POST", "/qurt/orders/"+oa.ID+"/cancel", nil).Expect(t, 409)
		var m struct {
			Orders []order `json:"orders"`
		}
		create(map[string]any{"items": lines(0, 1)}).Expect(t, 201).JSON(t, &m)
		if p := products(); p["Qurt b1"].StockLeft != 2 {
			t.Fatalf("band: %+v", p)
		}
		w.Do(t, w.Customer, "POST", "/qurt/orders/"+m.Orders[0].ID+"/cancel", map[string]any{"reason": "fikrim o'zgardi"}).Expect(t, 200)
		if p := products(); p["Qurt b1"].StockLeft != 3 {
			t.Fatalf("bekor qilingach band qaytishi kerak: %+v", p)
		}
		w.Do(t, w.Customer, "POST", "/qurt/orders/"+m.Orders[0].ID+"/cancel", nil).Expect(t, 409) // ikki marta emas
		if p := products(); p["Qurt b1"].StockLeft != 3 {
			t.Fatalf("takroriy bekor bandni buzdi: %+v", p)
		}
	})

	w.Step(t, "logistika: reys bo'yicha yig'ish va yetkazish (faqat admin)", func(t *testing.T) {
		w.ExpectDenied(t, w.Customer, "GET", "/admin/qurt/batches", nil)
		w.ExpectDenied(t, w.Owner, "GET", "/admin/qurt/orders", nil)
		w.Do(t, nil, "GET", "/admin/qurt/batches", nil).Expect(t, 401)

		var b struct {
			Batches []struct {
				Date    string         `json:"date"`
				Total   int            `json:"total"`
				ByState map[string]int `json:"by_status"`
			} `json:"batches"`
		}
		w.Do(t, w.Admin, "GET", "/admin/qurt/batches", nil).Expect(t, 200).JSON(t, &b)
		if len(b.Batches) != 1 || b.Batches[0].Date != oa.DeliveryDate || b.Batches[0].ByState["packed"] != 1 {
			t.Fatalf("reyslar: %+v", b.Batches)
		}

		// Admin to'liq ma'lumotni ko'radi (manzil, telefon).
		r := w.Do(t, w.Admin, "GET", "/admin/qurt/orders?date="+oa.DeliveryDate+"&status=packed", nil).Expect(t, 200)
		if !strings.Contains(string(r.Body), "Navoiy") || !strings.Contains(string(r.Body), "customer_phone") {
			t.Fatalf("admin manzil/telefonni ko'rishi kerak: %s", r.Body)
		}

		batch := func(from, to string) *Resp {
			return w.Do(t, w.Admin, "POST", "/admin/qurt/batches/"+oa.DeliveryDate+"/transition", map[string]any{"from": from, "to": to})
		}
		w.ExpectDenied(t, w.Customer, "POST", "/admin/qurt/batches/"+oa.DeliveryDate+"/transition", map[string]any{"from": "packed", "to": "collected"})
		batch("packed", "delivered").Expect(t, 400) // sakrab bo'lmaydi
		w.Do(t, w.Admin, "POST", "/admin/qurt/batches/bad-date/transition", map[string]any{"from": "packed", "to": "collected"}).Expect(t, 400)
		for _, step := range [][2]string{{"packed", "collected"}, {"collected", "out_for_delivery"}} {
			m := batch(step[0], step[1]).Expect(t, 200).Map(t)
			if m["done"] != float64(1) {
				t.Fatalf("%v: %v", step, m)
			}
		}
		// Yetkazilmadi (sabab shart) -> qayta urinish -> yetkazildi.
		w.Do(t, w.Admin, "POST", "/admin/qurt/orders/"+oa.ID+"/transition", map[string]any{"to": "failed"}).Expect(t, 400)
		w.Do(t, w.Admin, "POST", "/admin/qurt/orders/"+oa.ID+"/transition", map[string]any{"to": "failed", "reason": "uyda yo'q"}).Expect(t, 200)
		batch("failed", "out_for_delivery").Expect(t, 200)
		batch("out_for_delivery", "delivered").Expect(t, 200)

		got := w.Do(t, w.Customer, "GET", "/qurt/orders/"+oa.ID, nil).Expect(t, 200).Map(t)
		if got["status"] != "delivered" {
			t.Fatalf("yetkazilgan bo'lishi kerak: %v", got["status"])
		}
		hist, _ := got["history"].([]any)
		if len(hist) != 8 { // new, confirmed, packed, collected, out, failed, out, delivered
			t.Fatalf("holat tarixi: %d", len(hist))
		}
		// Terminal: admin ham qaytara olmaydi.
		w.Do(t, w.Admin, "POST", "/admin/qurt/orders/"+oa.ID+"/transition", map[string]any{"to": "cancelled", "reason": "xato"}).Expect(t, 409)
	})
}
