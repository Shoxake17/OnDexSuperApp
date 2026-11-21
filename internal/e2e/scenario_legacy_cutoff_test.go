package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"chustapp/internal/e2e/report"
	"chustapp/internal/users"
)

// ONDEXECOSY-483: eski HS256 token UMUMAN yo'q — na chiqariladi, na qabul
// qilinadi. Mijozning kirish endpointlari (sarlavha bo'lsin-bo'lmasin) HAR DOIM
// markaziy sessiya beradi; mijoz yo'li merchant akkauntiga TEGMAYDI.
func TestNoLegacyTokens(t *testing.T) {
	w := NewWorld(t, "Eski token yo'q — faqat markaziy sessiya (483)")
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

	w.Step(t, "HS256 token (hatto server siri bilan imzolangan) RAD etiladi", func(t *testing.T) {
		for _, role := range []users.Role{users.RoleCustomer, users.RoleRestaurant, users.RoleAdmin} {
			uid := map[users.Role]string{users.RoleCustomer: w.Customer.UserID,
				users.RoleRestaurant: w.Owner.UserID, users.RoleAdmin: w.Admin.UserID}[role]
			w.Do(t, forgedHS256(t, uid, role), "GET", "/me", nil).Expect(t, 401)
		}
		// Markaziy sessiyalar odatdagidek ishlaydi.
		w.Do(t, w.Customer, "GET", "/me", nil).Expect(t, 200)
		w.Do(t, w.Owner, "GET", "/me", nil).Expect(t, 200)
	})

	w.Step(t, "eski endpoint sarlavhasiz ham MARKAZIY sessiya beradi", func(t *testing.T) {
		for i, v2 := range []bool{false, true} {
			a := []*Actor{w.Customer, w.Other}[i] // kod qayta yuborish oralig'i — har biriga alohida raqam
			out := post(t, "/auth/verify", v2, map[string]string{"phone": a.Phone, "code": code(t, a.Phone)}, 200)
			tok, _ := out["access_token"].(string)
			if tok == "" || out["refresh_token"] == nil || out["token"] != tok {
				t.Fatalf("v2=%v: markaziy sessiya berilmadi: %v", v2, out)
			}
			w.Do(t, &Actor{Name: "mijoz", Token: tok}, "GET", "/me", nil).Expect(t, 200)
		}
	})

	w.Step(t, "restoran raqami bilan mijoz kirishi restoran akkauntiga TEGMAYDI", func(t *testing.T) {
		// Servis orqali: /auth/verify IP limiti butun e2e paketi uchun umumiy.
		amr, u, err := w.AuthSvc.Verify(ctx, w.Owner.Phone, code(t, w.Owner.Phone))
		if err != nil {
			t.Fatal(err)
		}
		if amr != users.AMROTP || u.Role != users.RoleCustomer || u.ID == w.Owner.UserID {
			t.Fatalf("mijoz yo'li merchant akkauntini ochdi: amr=%q %+v", amr, u)
		}
		owner, _ := w.Users.GetByID(ctx, w.Owner.UserID)
		if owner.Role != users.RoleRestaurant {
			t.Fatalf("restoran akkaunti o'zgardi: %+v", owner)
		}
	})
}

// forgedHS256 — eski formatdagi (HS256) token: server sirini bilgan hujumchi
// ham shunday yasay olardi. ONDEXECOSY-483 dan keyin HAMMA joyda 401.
func forgedHS256(t *testing.T, sub string, role users.Role) *Actor {
	t.Helper()
	claims := users.Claims{Role: role, RegisteredClaims: jwt.RegisteredClaims{
		Subject: sub, IssuedAt: jwt.NewNumericDate(time.Now()), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("e2e-secret"))
	if err != nil {
		t.Fatal(err)
	}
	return &Actor{Name: "eski HS256 " + string(role), Token: tok}
}
