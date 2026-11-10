package e2e

import (
	"context"
	"strings"
	"testing"

	"chustapp/internal/identity"
)

// Senariy: MARKAZIY SESSIYA (A2) — JWKS, refresh rotatsiyasi, qurilmalar ro'yxati,
// boshqa qurilmani yopish, chiqish.
//
// Kirish oqimlari (A3) hali yo'q: sessiya `Identity.Create` bilan ochiladi
// (kimlik tekshirilgan deb), QOLGAN HAMMASI haqiqiy HTTP orqali.
//
// Xavfsizlik tekshiruvlari: eski HS256 token yangi marshrutlarda O'TMAYDI;
// begona sessiyani yopib bo'lmaydi (404, mavjudligi oshkor bo'lmaydi);
// yopilgan sessiyaning access VA refresh tokeni darhol o'ladi.
func TestCentralSessionsV2(t *testing.T) {
	w := NewWorld(t, "Markaziy sessiya (A2): JWKS, refresh, qurilmalar, chiqish")
	ctx := context.Background()

	open := func(t *testing.T, a *Actor, client string) (*Actor, *identity.Tokens) {
		t.Helper()
		tok, err := w.Identity.Create(ctx, identity.CreateInput{UserID: a.UserID, ClientID: client,
			AMR: []string{"otp"}, Meta: identity.Meta{DeviceName: client}})
		if err != nil {
			t.Fatalf("sessiya ochilmadi: %v", err)
		}
		return &Actor{Name: a.Name + " (" + client + ")", Token: tok.AccessToken, UserID: a.UserID, Entity: a.Entity}, tok
	}

	w.Step(t, "JWKS ochiq, faqat ochiq kalit", func(t *testing.T) {
		r := w.Do(t, nil, "GET", "/.well-known/jwks.json", nil).Expect(t, 200)
		if cc := r.Header.Get("Cache-Control"); !strings.HasPrefix(cc, "public, max-age=") {
			t.Errorf("Cache-Control = %q", cc)
		}
		m := r.Map(t)
		keys, _ := m["keys"].([]any)
		if len(keys) != 1 {
			t.Fatalf("kalitlar: %v", m)
		}
		k := keys[0].(map[string]any)
		if k["kty"] != "OKP" || k["alg"] != "EdDSA" || k["kid"] == "" {
			t.Errorf("JWK: %v", k)
		}
		if _, has := k["d"]; has {
			t.Fatal("JWKS maxfiy kalitni oshkor qildi")
		}
	})

	webOwner, webTok := open(t, w.Owner, "ondex-pro")
	deskOwner, _ := open(t, w.Owner, "merchant-desktop")
	ownerB, _ := open(t, w.OwnerB, "merchant-web")

	w.Step(t, "mijoz akkaunti merchant ilovasida sessiya OCHOLMAYDI", func(t *testing.T) {
		_, err := w.Identity.Create(ctx, identity.CreateInput{UserID: w.Customer.UserID, ClientID: "merchant-web"})
		if err == nil {
			t.Fatal("mijoz merchant sessiyasini ochdi")
		}
	})

	w.Step(t, "qurilmalar ro'yxati: faqat o'ziniki, joriysi belgilangan", func(t *testing.T) {
		var out struct {
			Sessions []struct {
				ID       string `json:"id"`
				ClientID string `json:"client_id"`
				Current  bool   `json:"current"`
			} `json:"sessions"`
		}
		w.Do(t, webOwner, "GET", "/v2/auth/sessions", nil).Expect(t, 200).JSON(t, &out)
		if len(out.Sessions) != 2 {
			t.Fatalf("2 ta sessiya kutilgan, keldi %d", len(out.Sessions))
		}
		current := 0
		for _, s := range out.Sessions {
			if s.Current {
				current++
				if s.ID != webTok.SessionID {
					t.Error("joriy sessiya noto'g'ri belgilangan")
				}
			}
		}
		if current != 1 {
			t.Errorf("joriy sessiyalar soni %d", current)
		}
	})

	w.Step(t, "eski HS256 token va tokensiz so'rov — 401", func(t *testing.T) {
		w.Do(t, w.Owner, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
		w.Do(t, nil, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
	})

	w.Step(t, "refresh: rotatsiya, yangi token ishlaydi; yaroqsizi 401", func(t *testing.T) {
		var tok identity.Tokens
		r := w.Do(t, nil, "POST", "/v2/auth/token/refresh", map[string]string{"refresh_token": webTok.RefreshToken}).Expect(t, 200)
		if cc := r.Header.Get("Cache-Control"); cc != "no-store" {
			t.Errorf("token javobi keshlanishi mumkin: %q", cc)
		}
		r.JSON(t, &tok)
		if tok.RefreshToken == "" || tok.RefreshToken == webTok.RefreshToken || tok.SessionID != webTok.SessionID {
			t.Fatalf("rotatsiya noto'g'ri: %+v", tok)
		}
		webOwner.Token = tok.AccessToken
		webTok = &tok
		w.Do(t, webOwner, "GET", "/v2/auth/sessions", nil).Expect(t, 200)
		w.Do(t, nil, "POST", "/v2/auth/token/refresh", map[string]string{"refresh_token": "ort_soxta"}).Expect(t, 401)
	})

	w.Step(t, "RFC 9700: konfidensial klient (merchant-web) refresh'i umumiy yo'ldan - rad, sessiya yopiladi", func(t *testing.T) {
		bff, bffTok := open(t, w.Owner, "merchant-web")
		w.Do(t, nil, "POST", "/v2/auth/token/refresh", map[string]string{"refresh_token": bffTok.RefreshToken}).Expect(t, 401)
		w.Do(t, bff, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
		sso, ssoTok := open(t, w.Owner, identity.SSOClientID)
		w.Do(t, nil, "POST", "/v2/auth/token/refresh", map[string]string{"refresh_token": ssoTok.RefreshToken}).Expect(t, 401)
		w.Do(t, sso, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
	})

	w.Step(t, "begona sessiyani yopib bo'lmaydi (404), egasi ishlayveradi", func(t *testing.T) {		list, _ := w.Identity.ListSessions(ctx, w.OwnerB.UserID)
		if len(list) != 1 {
			t.Fatal("B egasining sessiyasi yo'q")
		}
		w.Do(t, webOwner, "DELETE", "/v2/auth/sessions/"+list[0].ID, nil).Expect(t, 404)
		w.Do(t, ownerB, "GET", "/v2/auth/sessions", nil).Expect(t, 200)
	})

	w.Step(t, "boshqa qurilmani yopish — uning tokeni darhol o'ladi", func(t *testing.T) {
		list, _ := w.Identity.ListSessions(ctx, w.Owner.UserID)
		var deskID string
		for _, s := range list {
			if s.ClientID == "merchant-desktop" {
				deskID = s.ID
			}
		}
		w.Do(t, webOwner, "DELETE", "/v2/auth/sessions/"+deskID, nil).Expect(t, 204)
		w.Do(t, deskOwner, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
	})

	w.Step(t, "chiqish — access ham, refresh ham o'ladi", func(t *testing.T) {
		w.Do(t, webOwner, "POST", "/v2/auth/logout", nil).Expect(t, 204)
		w.Do(t, webOwner, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
		w.Do(t, nil, "POST", "/v2/auth/token/refresh", map[string]string{"refresh_token": webTok.RefreshToken}).Expect(t, 401)
	})
}
