package e2e

import (
	"strings"
	"testing"
)

// ssenariy: BANNERLAR — uch joylashuv (menu / restoran / do'kon), matn qatlami,
// qurt amali, xavfsiz rasm manzili va eski mijoz bilan moslik.

func TestBannerPlacements(t *testing.T) {
	w := NewWorld(t, "Bannerlar: menu / restoran / do'kon joylashuvi")

	create := func(t *testing.T, body map[string]any) string {
		t.Helper()
		r := w.Do(t, w.Admin, "POST", "/admin/banners", body).Expect(t, 201).Map(t)
		id, _ := r["id"].(string)
		if id == "" {
			t.Fatalf("banner yaratilmadi: %v", r)
		}
		return id
	}
	ids := func(t *testing.T, path string) []string {
		t.Helper()
		var list []map[string]any
		w.Do(t, w.Customer, "GET", path, nil).Expect(t, 200).JSON(t, &list)
		out := make([]string, 0, len(list))
		for _, b := range list {
			out = append(out, b["id"].(string))
		}
		return out
	}
	has := func(list []string, id string) bool {
		for _, x := range list {
			if x == id {
				return true
			}
		}
		return false
	}

	var menuID, restID, storeID string
	w.Step(t, "har joylashuvga banner yaratiladi (matn qatlami va qurt amali bilan)", func(t *testing.T) {
		menuID = create(t, map[string]any{
			"image_url": "https://cdn.example/menu.webp", "placement": "menu", "active": true,
			"title": "Qurt", "subtitle": "Tabiiy, foydali va mazali qurt endi OnDexda!",
			"button_text": "Buyurtma berish", "action": "qurt",
		})
		restID = create(t, map[string]any{
			"image_url": "https://cdn.example/rest.webp", "placement": "restaurant", "active": true,
			"restaurant_id": RestA,
		})
		storeID = create(t, map[string]any{
			"image_url": "https://cdn.example/store.webp", "placement": "store", "active": true,
		})
	})

	w.Step(t, "mijoz har sahifa uchun FAQAT o'z joylashuvidagini oladi", func(t *testing.T) {
		menu := ids(t, "/promo-banners?placement=menu")
		rest := ids(t, "/promo-banners?placement=restaurant")
		store := ids(t, "/promo-banners?placement=store")
		if !has(menu, menuID) || has(menu, restID) || has(menu, storeID) {
			t.Fatalf("menu: %v", menu)
		}
		if !has(rest, restID) || has(rest, menuID) || has(rest, storeID) {
			t.Fatalf("restoran: %v", rest)
		}
		if !has(store, storeID) || has(store, menuID) || has(store, restID) {
			t.Fatalf("do'kon: %v", store)
		}
	})

	w.Step(t, "parametrsiz so'rov (eski ilova) — restoran bannerlari, avvalgidek", func(t *testing.T) {
		legacy := ids(t, "/promo-banners")
		if !has(legacy, restID) || has(legacy, menuID) || has(legacy, storeID) {
			t.Fatalf("eski ilova: %v", legacy)
		}
		w.Do(t, w.Customer, "GET", "/promo-banners?placement=footer", nil).Expect(t, 400)
	})

	w.Step(t, "matn qatlami va amal mijozga yetadi", func(t *testing.T) {
		var list []map[string]any
		w.Do(t, w.Customer, "GET", "/promo-banners?placement=menu", nil).Expect(t, 200).JSON(t, &list)
		var got map[string]any
		for _, b := range list {
			if b["id"] == menuID {
				got = b
			}
		}
		if got == nil || got["title"] != "Qurt" || got["button_text"] != "Buyurtma berish" || got["action"] != "qurt" {
			t.Fatalf("matn/amal yetmadi: %v", got)
		}
	})

	w.Step(t, "noto'g'ri banner rad etiladi", func(t *testing.T) {
		for name, body := range map[string]map[string]any{
			"noma'lum joylashuv": {"image_url": "https://cdn.example/x.webp", "placement": "footer"},
			"noma'lum amal":      {"image_url": "https://cdn.example/x.webp", "placement": "menu", "action": "boom"},
			"tugma havolasiz":    {"image_url": "https://cdn.example/x.webp", "placement": "menu", "button_text": "Bos"},
			"menu'da restoran":   {"image_url": "https://cdn.example/x.webp", "placement": "menu", "restaurant_id": RestA},
			"restoran va amal":   {"image_url": "https://cdn.example/x.webp", "placement": "restaurant", "restaurant_id": RestA, "action": "qurt"},
			"uzun sarlavha":      {"image_url": "https://cdn.example/x.webp", "placement": "menu", "title": strings.Repeat("a", 61)},
			"xavfli rasm":        {"image_url": "javascript:alert(1)", "placement": "menu"},
			"http rasm":          {"image_url": "http://evil.example/x.png", "placement": "menu"},
			"rasmsiz":            {"image_url": "", "placement": "menu"},
		} {
			if r := w.Do(t, w.Admin, "POST", "/admin/banners", body); r.Status != 400 {
				t.Errorf("%s: %d kutilgan 400 (%s)", name, r.Status, r.Body)
			}
		}
	})

	w.Step(t, "tahrirlashda joylashuvni o'zgartirish va o'chirilgan banner yashirinishi", func(t *testing.T) {
		w.Do(t, w.Admin, "POST", "/admin/banners/"+storeID, map[string]any{
			"image_url": "https://cdn.example/store.webp", "placement": "menu", "active": true,
		}).Expect(t, 200)
		if !has(ids(t, "/promo-banners?placement=menu"), storeID) || has(ids(t, "/promo-banners?placement=store"), storeID) {
			t.Fatalf("joylashuv o'zgarmadi")
		}
		w.Do(t, w.Admin, "POST", "/admin/banners/"+storeID, map[string]any{
			"image_url": "https://cdn.example/store.webp", "placement": "menu", "active": false,
		}).Expect(t, 200)
		if has(ids(t, "/promo-banners?placement=menu"), storeID) {
			t.Fatalf("nofaol banner mijozga ko'rinyapti")
		}
		for _, id := range []string{menuID, restID, storeID} {
			w.Do(t, w.Admin, "DELETE", "/admin/banners/"+id, nil).Expect(t, 200)
		}
	})
}
