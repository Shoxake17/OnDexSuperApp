package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"chustapp/internal/e2e/report"
)

// Mijoz sessiyasi markazlashuvi (Plane 137): kirish ekranlari va
// endpointlari o'zgarmaydi; `X-Ondex-Session: v2` bilan — markaziy sessiya
// (access + refresh, qurilmalar ro'yxati, bitta qurilmani chiqarish).
// Sarlavhasiz (eski ilova) — eski token, hech kim chiqib ketmaydi.
func TestCustomerCentralSession(t *testing.T) {
	w := NewWorld(t, "Mijoz sessiyasi markazlashuvi (137)")
	ctx := context.Background()
	const phone = "+998900000077"

	type loginOut struct {
		Token        string `json:"token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		SessionID    string `json:"session_id"`
		User         struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"user"`
	}
	post := func(t *testing.T, path, bearer string, v2 bool, body any, want int) *Resp {
		t.Helper()
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", w.Srv.URL+path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Dart/3.5 (dart:io) e2e-"+path)
		if v2 {
			req.Header.Set("X-Ondex-Session", "v2")
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
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
			Ms: float64(time.Since(start).Microseconds()) / 1000, Actor: "mijoz ilovasi"})
		r := &Resp{Status: res.StatusCode, Body: raw, Header: res.Header}
		w.checkGeneric(t, "POST", path, r)
		return r.Expect(t, want)
	}
	code := func(t *testing.T) string {
		t.Helper()
		_, c, err := w.AuthSvc.IssueCode(ctx, phone)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	actor := func(tok string) *Actor { return &Actor{Name: "mijoz (v2)", Token: tok} }

	w.Step(t, "eski ilova (sarlavhasiz): eski token, o'zgarishsiz", func(t *testing.T) {
		var o loginOut
		post(t, "/auth/verify", "", false, map[string]string{"phone": phone, "code": code(t)}, 200).JSON(t, &o)
		if o.Token == "" || o.RefreshToken != "" || strings.Count(o.Token, ".") != 2 || isEdDSAForTest(o.Token) {
			t.Fatalf("eski javob o'zgardi: %+v", o)
		}
		w.Do(t, actor(o.Token), "GET", "/me", nil).Expect(t, 200)
	})

	var phoneTok, pwdTok loginOut
	w.Step(t, "yangi ilova: kod bilan kirish -> markaziy sessiya; eski marshrutlar ishlaydi", func(t *testing.T) {
		post(t, "/auth/verify", "", true, map[string]string{"phone": phone, "code": code(t)}, 200).JSON(t, &phoneTok)
		if phoneTok.RefreshToken == "" || phoneTok.AccessToken != phoneTok.Token || !isEdDSAForTest(phoneTok.Token) {
			t.Fatalf("markaziy sessiya berilmadi: %+v", phoneTok)
		}
		w.Do(t, actor(phoneTok.Token), "GET", "/me", nil).Expect(t, 200)
		w.Do(t, actor(phoneTok.Token), "GET", "/me/orders", nil).Expect(t, 200)
		// Mijoz tokeni merchant/admin API'siga kirmaydi.
		w.Do(t, actor(phoneTok.Token), "GET", "/v2/merchant/me", nil).Expect(t, 401)
		w.Do(t, actor(phoneTok.Token), "GET", "/admin/couriers", nil).Expect(t, 403)
	})

	w.Step(t, "kod bilan kirgach 15 daqiqa: joriy parolsiz parol o'rnatish; keyin hamma eski sessiyalar o'ladi", func(t *testing.T) {
		var o struct {
			PasswordSet  bool   `json:"password_set"`
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		post(t, "/me/password", phoneTok.Token, false, map[string]string{"password": "Mijoz-Parol-77",
			"password_confirm": "Mijoz-Parol-77"}, 200).JSON(t, &o)
		if !o.PasswordSet || o.RefreshToken == "" {
			t.Fatalf("parol: %+v", o)
		}
		w.Do(t, actor(phoneTok.Token), "GET", "/me", nil).Expect(t, 401)
		w.Do(t, actor(o.AccessToken), "GET", "/me", nil).Expect(t, 200)
	})

	w.Step(t, "ikkinchi qurilma (parol): qurilmalar ro'yxati, bittasini chiqarish", func(t *testing.T) {
		post(t, "/auth/login", "", true, map[string]string{"login": phone, "password": "Mijoz-Parol-77"}, 200).JSON(t, &pwdTok)
		var other loginOut
		post(t, "/auth/verify", "", true, map[string]string{"phone": phone, "code": code(t)}, 200).JSON(t, &other)
		var list struct {
			Sessions []struct {
				ID       string `json:"id"`
				ClientID string `json:"client_id"`
				Current  bool   `json:"current"`
			} `json:"sessions"`
		}
		w.Do(t, actor(pwdTok.Token), "GET", "/v2/auth/sessions", nil).Expect(t, 200).JSON(t, &list)
		if len(list.Sessions) < 2 || list.Sessions[0].ClientID != "ondex-customer" {
			t.Fatalf("qurilmalar: %+v", list)
		}
		// Parol bilan kirgan sessiya "kod bilan kirdi" imtiyozini olmaydi.
		post(t, "/me/password", pwdTok.Token, false, map[string]string{"password": "Boshqa-Parol-88",
			"password_confirm": "Boshqa-Parol-88"}, 400)
		w.Do(t, actor(pwdTok.Token), "DELETE", "/v2/auth/sessions/"+other.SessionID, nil).Expect(t, 204)
		w.Do(t, actor(other.Token), "GET", "/me", nil).Expect(t, 401)
		w.Do(t, actor(pwdTok.Token), "GET", "/me", nil).Expect(t, 200)
	})

	w.Step(t, "refresh: rotatsiya; eski refresh qayta kelsa sessiya yopiladi", func(t *testing.T) {
		var r1 loginOut
		post(t, "/v2/auth/token/refresh", "", false, map[string]string{"refresh_token": pwdTok.RefreshToken}, 200).JSON(t, &r1)
		if r1.AccessToken == "" || r1.RefreshToken == pwdTok.RefreshToken {
			t.Fatalf("rotatsiya: %+v", r1)
		}
		w.Do(t, actor(r1.AccessToken), "GET", "/me", nil).Expect(t, 200)
		pwdTok.Token, pwdTok.RefreshToken = r1.AccessToken, r1.RefreshToken
	})

	w.Step(t, "akkauntni o'chirish — barcha markaziy sessiyalar o'ladi", func(t *testing.T) {
		post(t, "/me/delete-account", pwdTok.Token, false, map[string]string{"current_password": "Mijoz-Parol-77"}, 200)
		w.Do(t, actor(pwdTok.Token), "GET", "/me", nil).Expect(t, 401)
		post(t, "/v2/auth/token/refresh", "", false, map[string]string{"refresh_token": pwdTok.RefreshToken}, 401)
	})
}

func isEdDSAForTest(tok string) bool {
	h, _, _ := strings.Cut(tok, ".")
	return strings.HasPrefix(h, "eyJhbGciOiJFZERTQSIs") // {"alg":"EdDSA",
}
