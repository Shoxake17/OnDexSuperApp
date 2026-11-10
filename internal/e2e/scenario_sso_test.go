package e2e

import (
	"net/url"
	"testing"
)

// ssenariy: ESKI yagona kirish (OIDC'dan oldingi) — faqat `/sso/identity`.
//
// Brauzer oqimi (`/sso/client`, `/sso/authorize`, `/sso/token`) OLIB
// TASHLANDI (SSO auditi, 2026-11-10): Merchant web OnDex OIDC'ga o'tgan,
// ishlatilmaydigan kirish yo'li faqat hujum yuzasi edi. Qolgani — klient
// SERVERI siri bilan kimlikni qayta o'qish (ShoxPos).

func TestSSOFlow(t *testing.T) {
	w := NewWorld(t, "Eski SSO: faqat klient siri bilan kimlikni o'qish")
	c := w.Customer
	redirect := url.QueryEscape(SSORedirectURI)

	w.Step(t, "eski brauzer oqimi YO'Q (kod berib bo'lmaydi)", func(t *testing.T) {
		w.Do(t, nil, "GET", "/sso/client?client_id="+SSOClientID+"&redirect_uri="+redirect, nil).Expect(t, 404)
		w.Do(t, c, "POST", "/sso/authorize", map[string]any{
			"client_id": SSOClientID, "redirect_uri": SSORedirectURI, "state": "x",
		}).Expect(t, 404)
		w.Do(t, nil, "POST", "/sso/token", map[string]any{
			"client_id": SSOClientID, "client_secret": SSOClientSecret, "code": "x", "redirect_uri": SSORedirectURI,
		}).Expect(t, 404)
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
		for _, banned := range []string{"token", "role", "password", "entity_id"} {
			if _, has := id[banned]; has {
				t.Fatalf("kimlikda %q maydoni bor: %v", banned, id)
			}
		}
	})
}
