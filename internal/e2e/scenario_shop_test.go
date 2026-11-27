package e2e

import (
	"strings"
	"testing"
	"time"
)

// ssenariy: ONDEX POS — do'kon/qurt/dorixona (POS-1, ShoxPos o'rnida).
//
// Asosiy e'tibor: do'kon FAQAT sessiyadan (URL'da login yo'q), bir do'kon
// boshqasining ma'lumotiga tega olmaydi, kategoriya bo'limlarni cheklaydi,
// qurt mahsuloti mijoz katalogiga jarayon ichida o'tadi.

func TestShopPOSFlow(t *testing.T) {
	w := NewWorld(t, "OnDex POS: do'kon, qurt, dorixona")

	newStore := func(t *testing.T, name, phone, category string) *Actor {
		t.Helper()
		w.Do(t, w.Admin, "POST", "/admin/stores", map[string]any{
			"name": name, "phone": phone, "category": category,
		}).Expect(t, 201)
		return w.LoginAs(t, name, phone)
	}
	shopA := newStore(t, "Baraka market", "+998905552001", "store")
	shopB := newStore(t, "Chust savdo", "+998905552002", "store")
	qurtS := newStore(t, "Namangan qurti", "+998905552003", "qurt")
	pharm := newStore(t, "Shifo dorixona", "+998905552004", "pharmacy")

	w.Step(t, "kirish: faqat do'kon akkaunti", func(t *testing.T) {
		w.Do(t, nil, "GET", "/merchant/shop/me", nil).Expect(t, 401)
		w.Do(t, w.Customer, "GET", "/merchant/shop/me", nil).Expect(t, 403)
		m := w.Do(t, shopA, "GET", "/merchant/shop/me", nil).Expect(t, 200).Map(t)
		if m["category"] != "store" || m["shop_type"] != "dokon" {
			t.Fatalf("me: %v", m)
		}
		secs, _ := m["sections"].([]any)
		if len(secs) != 3 {
			t.Fatalf("oddiy do'konda 3 bo'lim (apteka yo'q): %v", secs)
		}
		pm := w.Do(t, pharm, "GET", "/merchant/shop/me", nil).Expect(t, 200).Map(t)
		if ps, _ := pm["sections"].([]any); len(ps) != 4 {
			t.Fatalf("dorixonada 4 bo'lim: %v", pm)
		}
	})

	w.Step(t, "mahsulot: qo'shish, tahrir, izolyatsiya", func(t *testing.T) {
		put := func(a *Actor, bc string, body map[string]any) *Resp {
			return w.Do(t, a, "PUT", "/merchant/shop/products/"+bc, body)
		}
		p := put(shopA, "4780001000011", map[string]any{"name": "Non", "category": "Non mahsulotlari",
			"price_tiyin": 500000, "base_price_tiyin": 400000, "quantity": 10}).Expect(t, 200).Map(t)
		if p["quantity"].(float64) != 10 {
			t.Fatalf("mahsulot: %v", p)
		}
		put(shopA, "bad%20code", map[string]any{"name": "x"}).Expect(t, 400)
		put(shopA, "4780001000028", map[string]any{"name": "x", "price_tiyin": -1}).Expect(t, 400)
		put(shopA, "4780001000035", map[string]any{"name": "Sut", "category": "Sut", "price_tiyin": 1100000, "quantity": 0}).Expect(t, 200)

		w.Do(t, shopA, "PATCH", "/merchant/shop/products/4780001000011", map[string]any{"field": "price", "value": 550000}).Expect(t, 200)
		w.Do(t, shopA, "PATCH", "/merchant/shop/products/4780001000011", map[string]any{"field": "hack", "value": 1}).Expect(t, 400)
		w.Do(t, shopA, "PATCH", "/merchant/shop/products/4780001000011", map[string]any{"field": "quantity", "value": -5}).Expect(t, 400)

		var list []map[string]any
		w.Do(t, shopA, "GET", "/merchant/shop/products", nil).Expect(t, 200).JSON(t, &list)
		if len(list) != 2 {
			t.Fatalf("A mahsulotlari: %v", list)
		}
		// B do'kon A'nikini ko'rmaydi, o'zgartirolmaydi, o'chirolmaydi.
		w.Do(t, shopB, "GET", "/merchant/shop/products", nil).Expect(t, 200).JSON(t, &list)
		if len(list) != 0 {
			t.Fatalf("B A'ning mahsulotini ko'rdi: %v", list)
		}
		w.Do(t, shopB, "PATCH", "/merchant/shop/products/4780001000011", map[string]any{"field": "price", "value": 1}).Expect(t, 404)
		w.Do(t, shopB, "DELETE", "/merchant/shop/products/4780001000011", nil).Expect(t, 404)

		var inc []map[string]any
		w.Do(t, shopA, "GET", "/merchant/shop/products/incoming", nil).Expect(t, 200).JSON(t, &inc)
		var cats []string
		w.Do(t, shopA, "GET", "/merchant/shop/categories", nil).Expect(t, 200).JSON(t, &cats)
		if strings.Join(cats, ",") != "Non mahsulotlari,Sut" {
			t.Fatalf("kategoriyalar: %v", cats)
		}
	})

	w.Step(t, "qidiruv, skaner, global baza, tashqi lookup", func(t *testing.T) {
		var hits []map[string]any
		w.Do(t, shopA, "GET", "/merchant/shop/products/search?q=choy", nil).Expect(t, 200).JSON(t, &hits)
		if len(hits) != 1 || hits[0]["is_global"] != true {
			t.Fatalf("global qidiruv: %v", hits)
		}
		s := w.Do(t, shopA, "GET", "/merchant/shop/products/scan?barcode=4780001000011", nil).Expect(t, 200).Map(t)
		if s["source"] != "user" || s["price_tiyin"].(float64) != 550000 {
			t.Fatalf("skaner (do'kon): %v", s)
		}
		g := w.Do(t, shopB, "GET", "/merchant/shop/products/scan?barcode=4600000000017", nil).Expect(t, 200).Map(t)
		if g["source"] != "global" {
			t.Fatalf("skaner (global): %v", g)
		}
		w.Do(t, shopB, "GET", "/merchant/shop/products/scan?barcode=4780001000011", nil).Expect(t, 404) // A'niki — B uchun yo'q
		c := w.Do(t, shopA, "GET", "/merchant/shop/products/base-count", nil).Expect(t, 200).Map(t)
		if c["count"].(float64) != 1 {
			t.Fatalf("global baza: %v", c)
		}
		l := w.Do(t, shopA, "GET", "/merchant/shop/products/lookup?barcode=5449000000996", nil).Expect(t, 200).Map(t)
		if l["name"] != "Coca-Cola 0.33L" {
			t.Fatalf("lookup: %v", l)
		}
		w.Do(t, shopA, "GET", "/merchant/shop/products/lookup?barcode=../../etc", nil).Expect(t, 400)
	})

	var saleID string
	w.Step(t, "sotuv: qoldiq kamayadi, kunlik yig'indi, 'Sotilganlar'", func(t *testing.T) {
		sell := func(qty float64, pay string) *Resp {
			return w.Do(t, shopA, "POST", "/merchant/shop/sales", map[string]any{"payment_method": pay, "items": []map[string]any{
				{"barcode": "4780001000011", "name": "Non", "quantity": qty, "price_tiyin": 550000, "category": "Non mahsulotlari"},
			}})
		}
		sell(3, "cash").Expect(t, 201)
		sell(2, "cash").Expect(t, 201)
		sell(1, "card").Expect(t, 201)
		w.Do(t, shopA, "POST", "/merchant/shop/sales", map[string]any{"items": []any{}}).Expect(t, 400)

		s := w.Do(t, shopA, "GET", "/merchant/shop/products/scan?barcode=4780001000011", nil).Expect(t, 200).Map(t)
		if s["quantity"].(float64) != 4 || s["sold"].(float64) != 6 {
			t.Fatalf("qoldiq/sotilgan: %v", s)
		}
		var sales []map[string]any
		today := w.now().Format("2006-01-02")
		w.Do(t, shopA, "GET", "/merchant/shop/sales?from="+today+"&to="+today, nil).Expect(t, 200).JSON(t, &sales)
		if len(sales) != 2 {
			t.Fatalf("kunlik yig'indilar (naqd+karta): %v", sales)
		}
		for _, x := range sales {
			if x["payment_method"] == "cash" {
				saleID, _ = x["id"].(string)
				if x["quantity"].(float64) != 5 || x["total_tiyin"].(float64) != 5*550000 {
					t.Fatalf("naqd yig'indi: %v", x)
				}
			}
		}
		w.Do(t, shopA, "GET", "/merchant/shop/sales?from=28.11.2026", nil).Expect(t, 400)
		var lines []map[string]any
		w.Do(t, shopA, "GET", "/merchant/shop/sold-lines", nil).Expect(t, 200).JSON(t, &lines)
		if len(lines) != 3 {
			t.Fatalf("sotuv qatorlari: %v", lines)
		}
		w.Do(t, shopB, "GET", "/merchant/shop/sold-lines", nil).Expect(t, 200).JSON(t, &lines)
		if len(lines) != 0 {
			t.Fatal("B A'ning sotuvlarini ko'rdi")
		}
		w.Do(t, shopB, "DELETE", "/merchant/shop/sales/"+saleID, nil).Expect(t, 404)
		w.Do(t, shopA, "DELETE", "/merchant/shop/sales/"+saleID, nil).Expect(t, 200)
		w.Do(t, shopA, "DELETE", "/merchant/shop/products/4780001000035", nil).Expect(t, 200)
	})

	w.Step(t, "qarzlar", func(t *testing.T) {
		d := w.Do(t, shopA, "POST", "/merchant/shop/debts", map[string]any{"name": "Ali", "phone": "+998901234567", "amount_tiyin": 2000000}).Expect(t, 201).Map(t)
		id, _ := d["id"].(string)
		w.Do(t, shopA, "POST", "/merchant/shop/debts", map[string]any{"name": "", "phone": "1", "amount_tiyin": 1}).Expect(t, 400)
		w.Do(t, shopB, "POST", "/merchant/shop/debts/"+id+"/pay", map[string]any{"paid_tiyin": 1}).Expect(t, 404)
		p := w.Do(t, shopA, "POST", "/merchant/shop/debts/"+id+"/pay", map[string]any{"paid_tiyin": 500000}).Expect(t, 200).Map(t)
		if p["fully_paid"] != false {
			t.Fatalf("qisman: %v", p)
		}
		p = w.Do(t, shopA, "POST", "/merchant/shop/debts/"+id+"/pay", map[string]any{"paid_tiyin": 1500000}).Expect(t, 200).Map(t)
		if p["fully_paid"] != true {
			t.Fatalf("to'liq: %v", p)
		}
		var list []map[string]any
		w.Do(t, shopA, "GET", "/merchant/shop/debts", nil).Expect(t, 200).JSON(t, &list)
		if len(list) != 0 {
			t.Fatalf("to'langan qarz qoldi: %v", list)
		}
	})

	w.Step(t, "qurt: gramm majburiy, mahsulot mijoz katalogiga o'tadi", func(t *testing.T) {
		w.Do(t, qurtS, "PUT", "/merchant/shop/products/q-001", map[string]any{"name": "Sho'r qurt", "category": "Sho'r",
			"price_tiyin": 1500000, "quantity": 20}).Expect(t, 400)
		w.Do(t, qurtS, "PUT", "/merchant/shop/products/q-001", map[string]any{"name": "Sho'r qurt", "category": "Sho'r",
			"price_tiyin": 1500000, "base_price_tiyin": 1000000, "quantity": 20, "weight_grams": 250,
			"image_url": "javascript:alert(1)"}).Expect(t, 400)
		w.Do(t, qurtS, "PUT", "/merchant/shop/products/q-001", map[string]any{"name": "Sho'r qurt", "category": "Sho'r",
			"price_tiyin": 1500000, "base_price_tiyin": 1000000, "quantity": 20, "weight_grams": 250}).Expect(t, 200)
		m := w.Do(t, nil, "GET", "/qurt/products", nil).Expect(t, 200)
		if !strings.Contains(string(m.Body), "Sho'r qurt") || strings.Contains(string(m.Body), "1000000") {
			t.Fatalf("mijoz katalogi (ulgurji narx chiqmasligi kerak): %s", m.Body)
		}
		w.Do(t, qurtS, "GET", "/merchant/shop/pharm", nil).Expect(t, 403) // qurt — dorixona emas
		w.Do(t, qurtS, "DELETE", "/merchant/shop/products/q-001", nil).Expect(t, 200)
		m = w.Do(t, nil, "GET", "/qurt/products", nil).Expect(t, 200)
		if strings.Contains(string(m.Body), "Sho'r qurt") {
			t.Fatalf("o'chirilgan qurt katalogda qoldi: %s", m.Body)
		}
	})

	w.Step(t, "dorixona: bo'lim faqat dorixonaga, chek atomik", func(t *testing.T) {
		w.Do(t, shopA, "GET", "/merchant/shop/pharm", nil).Expect(t, 403)
		w.Do(t, shopA, "POST", "/merchant/shop/pharm/sales", map[string]any{"items": []any{}}).Expect(t, 403)
		add := func(bc, name string, qty float64) *Resp {
			return w.Do(t, pharm, "POST", "/merchant/shop/pharm", map[string]any{"barcode": bc, "name": name,
				"category": "Og'riq qoldiruvchi", "price_tiyin": 300000, "base_price_tiyin": 200000, "quantity": qty,
				"expiry_date": "2027-06-01", "batch_number": "B-17"})
		}
		add("d-001", "Paratsetamol", 5).Expect(t, 200)
		p := add("d-001", "Paratsetamol", 3).Expect(t, 200).Map(t)
		if p["quantity"].(float64) != 8 || p["incoming_quantity"].(float64) != 8 {
			t.Fatalf("qayta qo'shish: %v", p)
		}
		add("d-002", "Ibuprofen", 1).Expect(t, 200)
		w.Do(t, pharm, "POST", "/merchant/shop/pharm", map[string]any{"barcode": "d-003", "name": "X", "category": "Y",
			"expiry_date": "01.06.2027"}).Expect(t, 400)

		w.Do(t, pharm, "POST", "/merchant/shop/pharm/sales", map[string]any{"payment_method": "naqd", "items": []map[string]any{
			{"barcode": "d-001", "quantity": 2, "price_tiyin": 300000}, {"barcode": "d-002", "quantity": 5, "price_tiyin": 300000},
		}}).Expect(t, 409)
		info := w.Do(t, pharm, "GET", "/merchant/shop/pharm/info?q=d-001", nil).Expect(t, 200).Map(t)
		if info["quantity"].(float64) != 8 {
			t.Fatalf("muvaffaqiyatsiz chek qoldiqni o'zgartirdi: %v", info)
		}
		w.Do(t, pharm, "POST", "/merchant/shop/pharm/sales", map[string]any{"payment_method": "bitcoin", "items": []map[string]any{
			{"barcode": "d-001", "quantity": 1, "price_tiyin": 300000}}}).Expect(t, 400)
		sale := w.Do(t, pharm, "POST", "/merchant/shop/pharm/sales", map[string]any{"payment_method": "plastik", "items": []map[string]any{
			{"barcode": "d-001", "quantity": 2, "price_tiyin": 300000}}}).Expect(t, 201).Map(t)
		if sale["total_tiyin"].(float64) != 600000 {
			t.Fatalf("chek: %v", sale)
		}
		var sales []map[string]any
		w.Do(t, pharm, "GET", "/merchant/shop/pharm/sales", nil).Expect(t, 200).JSON(t, &sales)
		st := w.Do(t, pharm, "GET", "/merchant/shop/pharm/stats", nil).Expect(t, 200).Map(t)
		if len(sales) != 1 || st["total_tiyin"].(float64) != 600000 {
			t.Fatalf("cheklar/statistika: %v %v", sales, st)
		}

		w.Do(t, pharm, "PATCH", "/merchant/shop/pharm/d-001", map[string]any{"price_tiyin": 350000, "shelf_number": "A3", "expiry_date": nil}).Expect(t, 200)
		up := w.Do(t, pharm, "GET", "/merchant/shop/pharm/info?q=d-001", nil).Expect(t, 200).Map(t)
		if up["price_tiyin"].(float64) != 350000 || up["shelf_number"] != "A3" || up["expiry_date"] != nil {
			t.Fatalf("tahrir: %v", up)
		}
		w.Do(t, pharm, "PATCH", "/merchant/shop/pharm/d-001", map[string]any{"name": ""}).Expect(t, 400)

		var list []map[string]any
		w.Do(t, pharm, "GET", "/merchant/shop/pharm", nil).Expect(t, 200).JSON(t, &list)
		if len(list) != 2 {
			t.Fatalf("dorilar: %v", list)
		}
		w.Do(t, pharm, "POST", "/merchant/shop/pharm/base", map[string]any{"barcode": "g-100", "name": "Analgin", "category": "Og'riq"}).Expect(t, 201)
		w.Do(t, pharm, "POST", "/merchant/shop/pharm/base", map[string]any{"barcode": "g-100", "name": "Boshqa nom", "category": "X"}).Expect(t, 409)
		w.Do(t, pharm, "GET", "/merchant/shop/pharm/base", nil).Expect(t, 200).JSON(t, &list)
		c := w.Do(t, pharm, "GET", "/merchant/shop/pharm/base-count", nil).Expect(t, 200).Map(t)
		if len(list) != 1 || c["count"].(float64) != 1 {
			t.Fatalf("global dori bazasi: %v %v", list, c)
		}
		var hits []map[string]any
		w.Do(t, pharm, "GET", "/merchant/shop/pharm/search?q=a", nil).Expect(t, 200).JSON(t, &hits)
		if len(hits) != 2 { // Paratsetamol (o'zi) + Analgin (global); Ibuprofen'da "a" yo'q
			t.Fatalf("dori qidiruvi: %v", hits)
		}
		w.Do(t, pharm, "DELETE", "/merchant/shop/pharm/d-002", nil).Expect(t, 200)
		w.Do(t, pharm, "DELETE", "/merchant/shop/pharm/d-002", nil).Expect(t, 404)
	})

	w.Step(t, "30 kundan eski 'Sotilganlar' ko'rinmaydi", func(t *testing.T) {
		w.Advance(31 * 24 * time.Hour)
		var lines []map[string]any
		w.Do(t, shopA, "GET", "/merchant/shop/sold-lines", nil).Expect(t, 200).JSON(t, &lines)
		if len(lines) != 0 {
			t.Fatalf("eski qatorlar: %v", lines)
		}
	})
}
