package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/e2e/report"
	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// AUD-5: eski HS256 tokenni realm bo'yicha o'chirish (LEGACY_TOKENS_DISABLED).
// O'chirilgach: eski tokenlar o'ladi, eski ilova 426 (yangilang), yangi ilova
// markaziy sessiya bilan ishlaydi, eski yo'l merchant akkauntiga TEGMAYDI.
func TestLegacyTokenCutoff(t *testing.T) {
	w := NewWorld(t, "Eski tokenni o'chirish (AUD-5)")
	ctx := context.Background()

	post := func(t *testing.T, path string, v2 bool, body any, want int) map[string]any {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", w.Srv.URL+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if v2 {
			req.Header.Set("X-Ondex-Session", "v2")
		}
		start := time.Now()
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		Rec.AddCall(report.Call{Scenario: w.scenario, Step: w.step, Method: "POST", Path: path,
			Route: MatchRoute(Routes, "POST", path), Status: res.StatusCode,
			Ms: float64(time.Since(start).Microseconds()) / 1000, Actor: "ilova"})
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		if res.StatusCode != want {
			t.Fatalf("%s: %d, kutilgan %d: %s", path, res.StatusCode, want, raw)
		}
		return out
	}
	code := func(t *testing.T, phone string) string {
		t.Helper()
		_, c, err := w.AuthSvc.IssueCode(ctx, phone)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}

	// Kesishdan OLDIN berilgan tokenlar.
	oldOwner, oldCustomer := w.Owner.Token, w.Customer.Token
	w.Do(t, w.Owner, "GET", "/me", nil).Expect(t, 200)
	ownerCentral, err := w.Identity.Create(ctx, identity.CreateInput{UserID: w.Owner.UserID, ClientID: "merchant-desktop", AMR: []string{"otp"}})
	if err != nil {
		t.Fatal(err)
	}

	w.Step(t, "merchant o'chirildi: eski merchant tokeni o'ladi, markaziy sessiya ishlaydi", func(t *testing.T) {
		w.Tokens.DisableRealms(users.RealmMerchant)
		w.Do(t, &Actor{Name: "eski egasi", Token: oldOwner}, "GET", "/me", nil).Expect(t, 401)
		w.Do(t, &Actor{Name: "markaziy egasi", Token: ownerCentral.AccessToken}, "GET", "/me", nil).Expect(t, 200)
		w.Do(t, &Actor{Name: "mijoz", Token: oldCustomer}, "GET", "/me", nil).Expect(t, 200)
	})

	w.Step(t, "restoran raqami bilan eski kirish restoran akkauntiga TEGMAYDI", func(t *testing.T) {
		// Servis orqali: /auth/verify IP limiti butun e2e paketi uchun umumiy
		// (httptest = bitta IP) — HTTP yo'li quyida 426/v2 qadamida sinaladi.
		_, u, err := w.AuthSvc.Verify(ctx, w.Owner.Phone, code(t, w.Owner.Phone))
		if err != nil {
			t.Fatal(err)
		}
		if u.Role != users.RoleCustomer || u.ID == w.Owner.UserID {
			t.Fatalf("eski yo'l merchant akkauntini ochdi: %+v", u)
		}
		owner, _ := w.Users.GetByID(ctx, w.Owner.UserID)
		if owner.Role != users.RoleRestaurant {
			t.Fatalf("restoran akkaunti o'zgardi: %+v", owner)
		}
	})

	w.Step(t, "mijoz o'chirildi: eski ilova 426, yangi ilova markaziy sessiya", func(t *testing.T) {
		w.Tokens.DisableRealms(users.RealmCustomer)
		w.Do(t, &Actor{Name: "eski mijoz", Token: oldCustomer}, "GET", "/me", nil).Expect(t, 401)
		if out := post(t, "/auth/verify", false, map[string]string{"phone": w.Customer.Phone, "code": code(t, w.Customer.Phone)}, 426); out["upgrade_required"] != true {
			t.Fatalf("426 javobi: %v", out)
		}
		// Boshqa mijoz (kod qayta yuborish oralig'i 60 s) — yangi ilova.
		out := post(t, "/auth/verify", true, map[string]string{"phone": w.Other.Phone, "code": code(t, w.Other.Phone)}, 200)
		tok, _ := out["access_token"].(string)
		if tok == "" || out["refresh_token"] == nil {
			t.Fatalf("markaziy sessiya berilmadi: %v", out)
		}
		w.Do(t, &Actor{Name: "yangi mijoz", Token: tok}, "GET", "/me", nil).Expect(t, 200)
	})
}
