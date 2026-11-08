package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// Senariy: MERCHANT KIRISHI (A3b) — egasi va xodim, haqiqiy HTTP orqali.
//
//	egasi:  parol -> telefon kodi -> TOTP ulash -> sessiya (+10 tiklash kodi)
//	        keyingi kirish: parol -> TOTP (yoki tiklash kodi)
//	xodim:  telefon kodi -> sessiya, faqat ruxsat etilgan ilovada
//
// Xavfsizlik: parol yolg'iz sessiya OCHMAYDI; faqat parol bilan TOTP ulab
// bo'lmaydi; bir TOTP kodi ikki marta o'tmaydi; mijoz va xodim parol bilan
// kira olmaydi va javob "akkaunt yo'q" bilan bir xil.
func TestMerchantLoginV2(t *testing.T) {
	w := NewWorld(t, "Merchant kirishi (A3b): parol, kod, majburiy TOTP")
	ctx := context.Background()

	mkUser := func(t *testing.T, id, phone string, role users.Role, password string) {
		t.Helper()
		u := &users.User{ID: id, Phone: phone, Role: role, PhoneVerified: true, CreatedAt: time.Now()}
		if password != "" {
			h, err := users.HashPassword(ctx, password)
			if err != nil {
				t.Fatal(err)
			}
			u.PasswordHash = h
		}
		if err := w.Users.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
	}
	// Kod yetkazish kanallari (`/auth/telegram/start`, `/auth/request-code`)
	// o'z ssenariylarida; bu yerda kod to'g'ridan-to'g'ri yaratiladi.
	code := func(t *testing.T, phone string) string {
		t.Helper()
		_, c, err := w.AuthSvc.IssueCode(ctx, phone)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	type outcome struct {
		AccessToken   string   `json:"access_token"`
		RefreshToken  string   `json:"refresh_token"`
		MFARequired   bool     `json:"mfa_required"`
		MFAToken      string   `json:"mfa_token"`
		Method        string   `json:"method"`
		Secret        string   `json:"secret"`
		URI           string   `json:"otpauth_uri"`
		RecoveryCodes []string `json:"recovery_codes"`
		Tokens        struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
		Error string `json:"error"`
	}
	post := func(t *testing.T, path string, body map[string]any, want int) outcome {
		t.Helper()
		var o outcome
		r := w.Do(t, nil, "POST", path, body).Expect(t, want)
		if want == 200 && r.Header.Get("Cache-Control") != "no-store" {
			t.Errorf("%s: Cache-Control = %q, kutilgan no-store", path, r.Header.Get("Cache-Control"))
		}
		r.JSON(t, &o)
		return o
	}

	const ownerPhone, pw = "+998900000041", "Kuchli-parol-2026"
	mkUser(t, "e2e-m-owner", ownerPhone, users.RoleRestaurant, pw)
	mkUser(t, "e2e-m-waiter", "+998900000042", users.RoleWaiter, pw)
	mkUser(t, "e2e-m-waiter2", "+998900000043", users.RoleWaiter, "")
	mkUser(t, "e2e-m-cust", "+998900000044", users.RoleCustomer, pw)

	var secret, enrollToken, confirmCode string
	var recovery []string

	w.Step(t, "parolsiz egasi: kod -> parol O'RNATISH -> TOTP ulash (boshi berk ko'cha emas)", func(t *testing.T) {
		const np = "+998900000049"
		mkUser(t, "e2e-m-nopass", np, users.RoleRestaurant, "")
		post(t, "/v2/auth/merchant/password", map[string]any{"phone": np, "password": "har-qanday-parol",
			"client_id": "merchant-web"}, 401)
		o := post(t, "/v2/auth/merchant/otp/verify", map[string]any{"phone": np, "code": code(t, np),
			"client_id": "merchant-web"}, 200)
		if o.Method != "password_set" {
			t.Fatalf("parolsiz egasiga parol o'rnatish taklif qilinmadi: %+v", o)
		}
		post(t, "/v2/auth/mfa/password", map[string]any{"mfa_token": o.MFAToken, "password": "12345678",
			"password_confirm": "12345678"}, 400)
		o = post(t, "/v2/auth/mfa/password", map[string]any{"mfa_token": o.MFAToken, "password": "Yangi-Parol-2026!",
			"password_confirm": "Yangi-Parol-2026!"}, 200)
		if o.Method != "totp_enroll" {
			t.Fatalf("parol o'rnatilgach: %+v", o)
		}
		// Endi parol bilan kirish ishlaydi.
		if o = post(t, "/v2/auth/merchant/password", map[string]any{"phone": np, "password": "Yangi-Parol-2026!",
			"client_id": "merchant-web"}, 200); o.Method != "otp" {
			t.Fatalf("o'rnatilgan parol bilan: %+v", o)
		}
	})

	w.Step(t, "egasi parol bilan: sessiya EMAS, telefon kodi so'raladi", func(t *testing.T) {
		o := post(t, "/v2/auth/merchant/password", map[string]any{"phone": ownerPhone, "password": pw,
			"client_id": "merchant-web", "device_name": "Chrome"}, 200)
		if !o.MFARequired || o.Method != "otp" || o.AccessToken != "" {
			t.Fatalf("parol yolg'iz sessiya ochdi yoki noto'g'ri bosqich: %+v", o)
		}
		o = post(t, "/v2/auth/mfa/factor", map[string]any{"mfa_token": o.MFAToken, "code": code(t, ownerPhone)}, 200)
		if o.Method != "totp_enroll" {
			t.Fatalf("kod qo'shilgach: %+v", o)
		}
		enrollToken = o.MFAToken
	})

	w.Step(t, "TOTP ulash: QR ma'lumoti, noto'g'ri kod rad, to'g'ri kod -> sessiya + tiklash kodlari", func(t *testing.T) {
		o := post(t, "/v2/auth/mfa/totp/enroll", map[string]any{"mfa_token": enrollToken}, 200)
		if o.Secret == "" || !strings.HasPrefix(o.URI, "otpauth://totp/") {
			t.Fatalf("enroll: %+v", o)
		}
		secret = o.Secret
		post(t, "/v2/auth/mfa/totp/confirm", map[string]any{"mfa_token": enrollToken, "code": "000000"}, 401)
		confirmCode, _ = mfa.Code(secret, time.Now())
		o = post(t, "/v2/auth/mfa/totp/confirm", map[string]any{"mfa_token": enrollToken, "code": confirmCode}, 200)
		if o.Tokens.AccessToken == "" || len(o.RecoveryCodes) != mfa.RecoveryCount {
			t.Fatalf("confirm: token=%v kodlar=%d", o.Tokens.AccessToken != "", len(o.RecoveryCodes))
		}
		recovery = o.RecoveryCodes
		owner := &Actor{Name: "egasi (v2)", Token: o.Tokens.AccessToken}
		w.Do(t, owner, "GET", "/v2/auth/sessions", nil).Expect(t, 200)
	})

	w.Step(t, "keyingi kirish: parol -> TOTP; ishlatilgan kod qayta O'TMAYDI; tiklash kodi bir marta", func(t *testing.T) {
		login := func() string {
			o := post(t, "/v2/auth/merchant/password", map[string]any{"phone": ownerPhone, "password": pw,
				"client_id": "merchant-desktop"}, 200)
			if o.Method != "totp" {
				t.Fatalf("ulangan egasidan TOTP so'ralmadi: %+v", o)
			}
			return o.MFAToken
		}
		tok := login()
		post(t, "/v2/auth/mfa/verify", map[string]any{"mfa_token": tok, "code": confirmCode}, 401)
		o := post(t, "/v2/auth/mfa/verify", map[string]any{"mfa_token": tok, "code": recovery[0]}, 200)
		if o.AccessToken == "" || o.RefreshToken == "" {
			t.Fatalf("tiklash kodi bilan sessiya: %+v", o)
		}
		post(t, "/v2/auth/mfa/verify", map[string]any{"mfa_token": login(), "code": recovery[0]}, 401)
	})

	w.Step(t, "akkaunt mavjudligi oshkor bo'lmaydi: noto'g'ri parol = yo'q raqam = mijoz = xodim", func(t *testing.T) {
		bodies := []map[string]any{
			{"phone": ownerPhone, "password": "notogri-parol"},
			{"phone": "+998900000049", "password": pw},
			{"phone": "+998900000044", "password": pw}, // mijoz, parol to'g'ri
			{"phone": "+998900000042", "password": pw}, // xodim, parol to'g'ri
		}
		var first string
		for i, b := range bodies {
			b["client_id"] = "merchant-web"
			o := post(t, "/v2/auth/merchant/password", b, 401)
			if i == 0 {
				first = o.Error
			} else if o.Error != first {
				t.Errorf("javob farq qiladi: %q vs %q", o.Error, first)
			}
		}
	})

	w.Step(t, "xodim: faqat kod bilan, faqat o'z ilovasiga", func(t *testing.T) {
		o := post(t, "/v2/auth/merchant/otp/verify", map[string]any{"phone": "+998900000043",
			"code": code(t, "+998900000043"), "client_id": "ondex-go"}, 403)
		if o.AccessToken != "" {
			t.Fatal("ofitsiant OnDexGO'ga kirdi")
		}
		o = post(t, "/v2/auth/merchant/otp/verify", map[string]any{"phone": "+998900000042",
			"code": code(t, "+998900000042"), "client_id": "ondex-pro", "device_name": "Galaxy A23"}, 200)
		if o.AccessToken == "" || o.MFARequired {
			t.Fatalf("ofitsiant OnDexPro: %+v", o)
		}
		w.Do(t, &Actor{Name: "ofitsiant (v2)", Token: o.AccessToken}, "GET", "/v2/auth/sessions", nil).Expect(t, 200)
	})

	w.Step(t, "mijoz kod bilan ham merchant ilovasiga kira olmaydi; noma'lum klient 400", func(t *testing.T) {
		post(t, "/v2/auth/merchant/otp/verify", map[string]any{"phone": "+998900000044",
			"code": code(t, "+998900000044"), "client_id": "merchant-web"}, 401)
		post(t, "/v2/auth/merchant/otp/verify", map[string]any{"phone": "+998900000044",
			"code": "123456", "client_id": "admin-web"}, 400)
	})

	w.Step(t, "A4a ko'prigi: eski marshrutlar yangi tokenni qabul qiladi; chiqish va kirish yopilishi darhol", func(t *testing.T) {
		const p = "+998900000045"
		mkUser(t, "e2e-m-waiter3", p, users.RoleWaiter, "")
		if err := w.Users.UpdateRole(ctx, "e2e-m-waiter3", users.RoleWaiter, RestA); err != nil {
			t.Fatal(err)
		}
		login := func() *Actor {
			o := post(t, "/v2/auth/merchant/otp/verify", map[string]any{"phone": p, "code": code(t, p), "client_id": "ondex-pro"}, 200)
			return &Actor{Name: "ofitsiant (v2 ko'prik)", Token: o.AccessToken}
		}
		a := login()
		var me struct {
			Role     string `json:"role"`
			EntityID string `json:"entity_id"`
		}
		w.Do(t, a, "GET", "/me", nil).Expect(t, 200).JSON(t, &me)
		if me.Role != "waiter" || me.EntityID != RestA {
			t.Fatalf("/me v2 token bilan: %+v", me)
		}
		w.Do(t, a, "GET", "/waiter/me", nil).Expect(t, 200)
		w.Do(t, a, "GET", "/courier/qurt/me", nil).Expect(t, 403) // rol baribir tekshiriladi
		// eski staff moduli kirishni yopsa (rol -> mijoz) — refresh'ni kutmasdan 401
		if err := w.Users.UpdateRole(ctx, "e2e-m-waiter3", users.RoleCustomer, ""); err != nil {
			t.Fatal(err)
		}
		w.Do(t, a, "GET", "/me", nil).Expect(t, 401)
		// akkaunt har so'rovda o'qiladi: kirish qayta ochilsa — yana ishlaydi
		if err := w.Users.UpdateRole(ctx, "e2e-m-waiter3", users.RoleWaiter, RestA); err != nil {
			t.Fatal(err)
		}
		w.Do(t, a, "GET", "/waiter/me", nil).Expect(t, 200)
		// markazda chiqish -> eski marshrut ham darhol 401
		w.Do(t, a, "POST", "/v2/auth/logout", nil).Expect(t, 204)
		w.Do(t, a, "GET", "/waiter/me", nil).Expect(t, 401)
	})

	w.Step(t, "buzilgan mfa_token rad", func(t *testing.T) {
		post(t, "/v2/auth/mfa/verify", map[string]any{"mfa_token": "omfa_soxta.imzo", "code": "123456"}, 401)
		post(t, "/v2/auth/mfa/totp/enroll", map[string]any{"mfa_token": enrollToken + "x"}, 401)
	})
}
