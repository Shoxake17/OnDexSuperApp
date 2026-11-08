package e2e

import (
	"net/url"
	"testing"
)

// ssenariy: YAGONA KIRISH (SSO) — OnDex kimlik manbai.
//
// Boshqa xizmat (OnDex Merchant Panel) foydalanuvchini OnDex'ga yuboradi,
// OnDex bir martalik kod bilan qaytaradi, xizmat serveri kodni kimlikka
// almashtiradi. Bu yerda ASOSIY e'tibor "ishlamasligi kerak" holatlarda:
// begona manzil, qayta ishlatish, noto'g'ri sir — chunki aynan shular
// orqali kod o'g'irlanadi.

func TestSSOFlow(t *testing.T) {
	w := NewWorld(t, "Yagona kirish (SSO): kodni kimlikka almashtirish")
	c := w.Customer

	redirect := url.QueryEscape(SSORedirectURI)

	w.Step(t, "tasdiqlash sahifasi: ilova nomi va qaytish domeni", func(t *testing.T) {
		m := w.Do(t, nil, "GET",
			"/sso/client?client_id="+SSOClientID+"&redirect_uri="+redirect, nil).
			Expect(t, 200).Map(t)
		if m["name"] != "OnDex Merchant Panel" || m["redirect_host"] != "merchant.ondex.uz" {
			t.Fatalf("ilova ma'lumoti noto'g'ri: %v", m)
		}
		// Ro'yxatda yo'q manzil yoki klient — javob BERILMAYDI.
		w.Do(t, nil, "GET", "/sso/client?client_id="+SSOClientID+
			"&redirect_uri="+url.QueryEscape("https://evil.example/cb"), nil).Expect(t, 400)
		w.Do(t, nil, "GET", "/sso/client?client_id=yoq&redirect_uri="+redirect, nil).Expect(t, 400)
	})

	w.Step(t, "kirmagan foydalanuvchi ruxsat bera olmaydi", func(t *testing.T) {
		w.ExpectDenied(t, nil, "POST", "/sso/authorize", map[string]any{
			"client_id": SSOClientID, "redirect_uri": SSORedirectURI, "state": "x",
		})
	})

	w.Step(t, "begona manzilga kod berilmaydi", func(t *testing.T) {
		for _, bad := range []string{
			"https://evil.example/auth/callback",
			SSORedirectURI + "/", // oxirida qo'shimcha
			SSORedirectURI + "?next=https://evil.example",
		} {
			w.Do(t, c, "POST", "/sso/authorize", map[string]any{
				"client_id": SSOClientID, "redirect_uri": bad, "state": "x",
			}).Expect(t, 400)
		}
	})

	var code string
	w.Step(t, "ruxsat berilgach kod va state qaytadi", func(t *testing.T) {
		m := w.Do(t, c, "POST", "/sso/authorize", map[string]any{
			"client_id": SSOClientID, "redirect_uri": SSORedirectURI, "state": "holat-123",
		}).Expect(t, 200).Map(t)
		loc, _ := m["redirect_to"].(string)
		u, err := url.Parse(loc)
		if err != nil || u.Scheme+"://"+u.Host+u.Path != SSORedirectURI {
			t.Fatalf("qaytish manzili boshqa joyga ketdi: %q", loc)
		}
		code = u.Query().Get("code")
		if code == "" || u.Query().Get("state") != "holat-123" {
			t.Fatalf("kod yoki state yo'q: %q", loc)
		}
	})

	w.Step(t, "noto'g'ri sir bilan almashtirib bo'lmaydi (kod saqlanadi)", func(t *testing.T) {
		w.Do(t, nil, "POST", "/sso/token", map[string]any{
			"client_id": SSOClientID, "client_secret": "noto'g'ri-sir-noto'g'ri-sir-noto'g'ri",
			"code": code, "redirect_uri": SSORedirectURI,
		}).Expect(t, 401)
	})

	w.Step(t, "kod boshqa manzil bilan almashmaydi", func(t *testing.T) {
		w.Do(t, nil, "POST", "/sso/token", map[string]any{
			"client_id": SSOClientID, "client_secret": SSOClientSecret,
			"code": code, "redirect_uri": SSORedirectURI + "/",
		}).Expect(t, 400)
	})

	// Yuqoridagi muvaffaqiyatsiz urinish kodni ISTE'MOL QILDI (klient
	// tasdiqlangan edi) — xavfsizlik tomonidan to'g'ri, shuning uchun
	// asosiy almashtirish uchun yangi kod olinadi.
	w.Step(t, "to'g'ri almashtirish kimlikni beradi", func(t *testing.T) {
		m := w.Do(t, c, "POST", "/sso/authorize", map[string]any{
			"client_id": SSOClientID, "redirect_uri": SSORedirectURI, "state": "s2",
		}).Expect(t, 200).Map(t)
		u, _ := url.Parse(m["redirect_to"].(string))
		fresh := u.Query().Get("code")

		r := w.Do(t, nil, "POST", "/sso/token", map[string]any{
			"client_id": SSOClientID, "client_secret": SSOClientSecret,
			"code": fresh, "redirect_uri": SSORedirectURI,
		}).Expect(t, 200)
		if cc := r.Header.Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("kimlik javobi keshlanmasligi kerak, Cache-Control=%q", cc)
		}
		id := r.Map(t)
		if id["user_id"] != c.UserID || id["phone"] != c.Phone {
			t.Fatalf("boshqa foydalanuvchi kimligi: %v (kutilgan %s %s)", id, c.UserID, c.Phone)
		}
		// Token, rol va boshqa maxfiy narsa BERILMAYDI.
		for _, banned := range []string{"token", "role", "password", "entity_id"} {
			if _, has := id[banned]; has {
				t.Fatalf("kimlikda %q maydoni bor: %v", banned, id)
			}
		}

		// Kod bir martalik.
		w.Do(t, nil, "POST", "/sso/token", map[string]any{
			"client_id": SSOClientID, "client_secret": SSOClientSecret,
			"code": fresh, "redirect_uri": SSORedirectURI,
		}).Expect(t, 400)
	})

	// Klient serveri kimlikni KODSIZ qayta o'qiydi (kategoriya yangilanishi
	// uchun) — faqat klient siri bilan.
	w.Step(t, "kimlikni qayta o'qish: faqat klient siri bilan", func(t *testing.T) {
		w.Do(t, nil, "POST", "/sso/identity", map[string]any{
			"client_id": SSOClientID, "client_secret": "noto'g'ri-sir-noto'g'ri-sir-noto'g'ri",
			"user_id": c.UserID,
		}).Expect(t, 401)
		w.Do(t, nil, "POST", "/sso/identity", map[string]any{
			"client_id": SSOClientID, "client_secret": SSOClientSecret, "user_id": "yoq-foydalanuvchi",
		}).Expect(t, 400)
		r := w.Do(t, nil, "POST", "/sso/identity", map[string]any{
			"client_id": SSOClientID, "client_secret": SSOClientSecret, "user_id": c.UserID,
		}).Expect(t, 200)
		if cc := r.Header.Get("Cache-Control"); cc != "no-store" {
			t.Fatalf("kimlik javobi keshlanmasligi kerak, Cache-Control=%q", cc)
		}
		id := r.Map(t)
		if id["user_id"] != c.UserID || id["phone"] != c.Phone {
			t.Fatalf("boshqa foydalanuvchi kimligi: %v", id)
		}
	})
}
