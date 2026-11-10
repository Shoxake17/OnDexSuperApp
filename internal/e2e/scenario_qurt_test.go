package e2e

import (
	"strings"
	"testing"
)

// ssenariy: QURT — POS serveri mahsulotni yuboradi, mijoz ilovasi ko'radi.
//
// Asosiy e'tibor "ishlamasligi kerak" holatlarda: begona sir, qurt bo'lmagan
// do'kon, noto'g'ri qiymatlar, kategoriya o'zgargach yashirinish, ulgurji
// narxning mijozga CHIQMASLIGI.

func TestQurtFlow(t *testing.T) {
	w := NewWorld(t, "Qurt: POS -> mijoz ilovasi")

	merchant := func(secret, userID string) *Actor {
		return &Actor{Name: "POS serveri", Headers: map[string]string{
			"X-Client-Id": SSOClientID, "X-Client-Secret": secret, "X-Store-User": userID,
		}}
	}
	newStore := func(t *testing.T, name, phone, category string) (userID, entity string) {
		t.Helper()
		r := w.Do(t, w.Admin, "POST", "/admin/stores", map[string]any{
			"name": name, "phone": phone, "category": category,
		}).Expect(t, 201).Map(t)
		acc, _ := r["account"].(map[string]any)
		st, _ := r["store"].(map[string]any)
		userID, _ = acc["id"].(string)
		entity, _ = st["id"].(string)
		if userID == "" || entity == "" {
			t.Fatalf("do'kon yaratilmadi: %v", r)
		}
		return userID, entity
	}

	qurtUser, qurtStore := newStore(t, "Urgut qurtlari", "+998905551001", "qurt")
	otherUser, _ := newStore(t, "Oddiy do'kon", "+998905551002", "store")
	pos := merchant(SSOClientSecret, qurtUser)

	body := func(extra map[string]any) map[string]any {
		m := map[string]any{
			"name": "Sho'r qurt", "description": "Uyda tayyorlangan", "composition": "Qatiq, tuz",
			"category": "Sho'r", "price_tiyin": 1500000, "wholesale_tiyin": 1000000,
			"weight_grams": 250, "image_url": "", "available": true,
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	w.Step(t, "boshida ro'yxat bo'sh", func(t *testing.T) {
		m := w.Do(t, nil, "GET", "/qurt/products", nil).Expect(t, 200).Map(t)
		if items, _ := m["items"].([]any); len(items) != 0 {
			t.Fatalf("bo'sh bo'lishi kerak: %v", m)
		}
		c := w.Do(t, nil, "GET", "/qurt/categories", nil).Expect(t, 200).Map(t)
		if cats, _ := c["categories"].([]any); len(cats) != 0 {
			t.Fatalf("kategoriyalar bo'sh bo'lishi kerak: %v", c)
		}
	})

	w.Step(t, "POS serveri faqat klient siri va qurt do'koni bilan kiradi", func(t *testing.T) {
		put := func(a *Actor) *Resp { return w.Do(t, a, "PUT", "/merchant/qurt/products/sku-1", body(nil)) }
		put(nil).Expect(t, 401)                                                         // sarlavhasiz
		put(merchant("noto'g'ri-sir-noto'g'ri-sir-noto'g'ri", qurtUser)).Expect(t, 401) // noto'g'ri sir
		put(merchant(SSOClientSecret, "yoq-foydalanuvchi")).Expect(t, 400)              // noma'lum foydalanuvchi
		put(merchant(SSOClientSecret, otherUser)).Expect(t, 403)                        // qurt EMAS
		put(merchant(SSOClientSecret, w.Customer.UserID)).Expect(t, 403)                // oddiy mijoz
		// Mijoz tokeni (JWT) bu yo'lda hech narsa bermaydi.
		w.Do(t, w.Customer, "PUT", "/merchant/qurt/products/sku-1", body(nil)).Expect(t, 401)
	})

	var imageURL string
	w.Step(t, "rasm yuklash", func(t *testing.T) {
		w.Upload(t, nil, "/merchant/qurt/uploads", nil, "a.png", PNG(64, 64)).Expect(t, 401)
		r := w.Upload(t, pos, "/merchant/qurt/uploads", nil, "a.png", PNG(64, 64)).Expect(t, 201).Map(t)
		imageURL, _ = r["url"].(string)
		if imageURL == "" {
			t.Fatalf("rasm manzili yo'q: %v", r)
		}
		w.Upload(t, pos, "/merchant/qurt/uploads", nil, "a.txt", []byte("rasm emas")).Expect(t, 400)
		w.Upload(t, pos, "/merchant/qurt/uploads", nil, "a.png", []byte("buzilgan")).Expect(t, 400)
	})

	w.Step(t, "noto'g'ri qiymatlar rad etiladi", func(t *testing.T) {
		for name, extra := range map[string]map[string]any{
			"nomsiz":         {"name": " "},
			"kategoriyasiz":  {"category": ""},
			"narxsiz":        {"price_tiyin": 0},
			"manfiy ulgurji": {"wholesale_tiyin": -1},
			"grammsiz":       {"weight_grams": 0},
			"juda og'ir":     {"weight_grams": 1000000},
			"xavfli rasm":    {"image_url": "javascript:alert(1)"},
			"http rasm":      {"image_url": "http://evil.example/x.png"},
			"uzun tavsif":    {"description": strings.Repeat("a", 1001)},
		} {
			if r := w.Do(t, pos, "PUT", "/merchant/qurt/products/sku-1", body(extra)); r.Status != 400 {
				t.Errorf("%s: %d kutilgan 400 (%s)", name, r.Status, r.Body)
			}
		}
		w.Do(t, pos, "PUT", "/merchant/qurt/products/bad%20key", body(nil)).Expect(t, 400)
	})

	w.Step(t, "saqlangan mahsulot mijozga ko'rinadi (ulgurji narxsiz)", func(t *testing.T) {
		w.Do(t, pos, "PUT", "/merchant/qurt/products/sku-1", body(map[string]any{"image_url": imageURL})).Expect(t, 200)
		// Qayta yuborish YANGILAYDI, takrorlamaydi.
		w.Do(t, pos, "PUT", "/merchant/qurt/products/sku-1", body(map[string]any{
			"image_url": imageURL, "price_tiyin": 1600000})).Expect(t, 200)

		r := w.Do(t, nil, "GET", "/qurt/products", nil).Expect(t, 200)
		var m struct {
			Items []map[string]any `json:"items"`
		}
		r.JSON(t, &m)
		if len(m.Items) != 1 {
			t.Fatalf("1 ta mahsulot kutilgan: %s", r.Body)
		}
		it := m.Items[0]
		if it["name"] != "Sho'r qurt" || it["store_name"] != "Urgut qurtlari" || it["composition"] != "Qatiq, tuz" ||
			it["weight_grams"] != float64(250) || it["price_tiyin"] != float64(1600000) || it["image_url"] != imageURL {
			t.Fatalf("mahsulot maydonlari noto'g'ri: %v", it)
		}
		for _, secret := range []string{"wholesale", "external", "sku-1"} {
			if strings.Contains(string(r.Body), secret) {
				t.Fatalf("mijozga %q sizdi: %s", secret, r.Body)
			}
		}
		c := w.Do(t, nil, "GET", "/qurt/categories", nil).Expect(t, 200).Map(t)
		if cats, _ := c["categories"].([]any); len(cats) != 1 || cats[0] != "Sho'r" {
			t.Fatalf("kategoriyalar: %v", c)
		}
	})

	w.Step(t, "filtr va qidiruv", func(t *testing.T) {
		count := func(path string) int {
			var m struct {
				Items []any `json:"items"`
			}
			w.Do(t, nil, "GET", path, nil).Expect(t, 200).JSON(t, &m)
			return len(m.Items)
		}
		if n := count("/qurt/products?category=" + "Sho%27r"); n != 1 {
			t.Errorf("kategoriya bo'yicha: %d", n)
		}
		if n := count("/qurt/products?category=Shirin"); n != 0 {
			t.Errorf("boshqa kategoriya: %d", n)
		}
		if n := count("/qurt/products?q=QATIQ"); n != 1 {
			t.Errorf("tarkib bo'yicha qidiruv: %d", n)
		}
		if n := count("/qurt/products?q=.*"); n != 0 {
			t.Errorf("regexp sifatida talqin qilindi: %d", n)
		}
	})

	w.Step(t, "sotuvda yo'q mahsulot va kategoriya o'zgargan do'kon yashirinadi", func(t *testing.T) {
		count := func() int {
			var m struct {
				Items []any `json:"items"`
			}
			w.Do(t, nil, "GET", "/qurt/products", nil).Expect(t, 200).JSON(t, &m)
			return len(m.Items)
		}
		w.Do(t, pos, "PUT", "/merchant/qurt/products/sku-1", body(map[string]any{"available": false})).Expect(t, 200)
		if n := count(); n != 0 {
			t.Fatalf("sotuvda yo'q mahsulot ko'rinyapti: %d", n)
		}
		w.Do(t, pos, "PUT", "/merchant/qurt/products/sku-1", body(nil)).Expect(t, 200)
		if n := count(); n != 1 {
			t.Fatalf("qaytdi: %d", n)
		}

		// Admin do'konni qurtdan chiqaradi: mahsulot yashirinadi va POS ruxsati yo'qoladi.
		w.Do(t, w.Admin, "POST", "/admin/stores/"+qurtStore, map[string]any{"category": "store"}).Expect(t, 200)
		if n := count(); n != 0 {
			t.Fatalf("qurt bo'lmagan do'kon mahsuloti ko'rinyapti: %d", n)
		}
		w.Do(t, pos, "PUT", "/merchant/qurt/products/sku-1", body(nil)).Expect(t, 403)
		w.Do(t, w.Admin, "POST", "/admin/stores/"+qurtStore, map[string]any{"category": "qurt"}).Expect(t, 200)
		if n := count(); n != 1 {
			t.Fatalf("qurtga qaytgach ko'rinishi kerak: %d", n)
		}
	})

	w.Step(t, "o'chirish va do'kon o'chirilganda mahsulotlar ketadi", func(t *testing.T) {
		w.Do(t, nil, "DELETE", "/merchant/qurt/products/sku-1", nil).Expect(t, 401)
		w.Do(t, pos, "DELETE", "/merchant/qurt/products/sku-1", nil).Expect(t, 200)
		w.Do(t, pos, "DELETE", "/merchant/qurt/products/sku-1", nil).Expect(t, 200) // takror — xato emas
		w.Do(t, pos, "DELETE", "/merchant/qurt/products/bad%20key", nil).Expect(t, 400)

		w.Do(t, pos, "PUT", "/merchant/qurt/products/sku-2", body(nil)).Expect(t, 200)
		w.Do(t, w.Admin.WithReason("e2e: tozalash"), "DELETE", "/admin/stores/"+qurtStore, nil).Expect(t, 200)
		var m struct {
			Items []any `json:"items"`
		}
		w.Do(t, nil, "GET", "/qurt/products", nil).Expect(t, 200).JSON(t, &m)
		if len(m.Items) != 0 {
			t.Fatalf("o'chirilgan do'kon mahsuloti qoldi: %v", m.Items)
		}
	})
}
