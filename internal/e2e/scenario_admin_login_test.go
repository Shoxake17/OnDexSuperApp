package e2e

import (
	"context"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// A6 / CC-0.1: OnDex egasi (admin, platform realmi) markaziy kirishi.
// Kod -> parol (yo'q bo'lsa o'rnatiladi) -> TOTP HAR kirishda. Admin tokeni
// eski admin marshrutlarida ishlaydi; TOTP'siz admin sessiyasi — hech qachon.
func TestAdminLoginV2(t *testing.T) {
	w := NewWorld(t, "Admin kirishi (A6): kod + parol + TOTP har safar")
	ctx := context.Background()
	const phone, pw = "+998900000091", "Admin-Kuchli-Parol-91"
	if err := w.Users.Create(ctx, &users.User{ID: "e2e-a6-admin", Phone: phone, Role: users.RoleAdmin,
		PhoneVerified: true, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	code := func(t *testing.T, p string) string {
		t.Helper()
		_, c, err := w.AuthSvc.IssueCode(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	type outcome struct {
		AccessToken   string   `json:"access_token"`
		MFAToken      string   `json:"mfa_token"`
		Method        string   `json:"method"`
		Secret        string   `json:"secret"`
		RecoveryCodes []string `json:"recovery_codes"`
		Tokens        struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}
	post := func(t *testing.T, path string, body map[string]any, want int) outcome {
		t.Helper()
		var o outcome
		w.Do(t, nil, "POST", path, body).Expect(t, want).JSON(t, &o)
		return o
	}

	w.Step(t, "admin bo'lmagan raqam (restoran egasi) — 401, mavjudlik oshkor emas", func(t *testing.T) {
		post(t, "/v2/auth/admin/otp/verify", map[string]any{"phone": w.Owner.Phone, "code": code(t, w.Owner.Phone)}, 401)
		post(t, "/v2/auth/admin/otp/verify", map[string]any{"phone": phone, "code": "000000"}, 401)
	})

	var secret, adminTok string
	var recovery []string
	w.Step(t, "birinchi kirish: kod -> parol o'rnatish -> TOTP ulash -> admin sessiyasi", func(t *testing.T) {
		o := post(t, "/v2/auth/admin/otp/verify", map[string]any{"phone": phone, "code": code(t, phone)}, 200)
		if o.Method != "password_set" || o.AccessToken != "" {
			t.Fatalf("kod yolg'iz: %+v", o)
		}
		o = post(t, "/v2/auth/mfa/password", map[string]any{"mfa_token": o.MFAToken, "password": pw, "password_confirm": pw}, 200)
		if o.Method != "totp_enroll" {
			t.Fatalf("parol o'rnatilgach: %+v", o)
		}
		en := post(t, "/v2/auth/mfa/totp/enroll", map[string]any{"mfa_token": o.MFAToken}, 200)
		secret = en.Secret
		c, _ := mfa.Code(secret, time.Now())
		done := post(t, "/v2/auth/mfa/totp/confirm", map[string]any{"mfa_token": o.MFAToken, "code": c}, 200)
		if done.Tokens.AccessToken == "" || len(done.RecoveryCodes) != mfa.RecoveryCount {
			t.Fatalf("confirm: %+v", done)
		}
		recovery = done.RecoveryCodes
		adm := &Actor{Name: "admin (v2)", Token: done.Tokens.AccessToken}
		w.Do(t, adm, "GET", "/admin/couriers", nil).Expect(t, 200)
		// Admin tokeni merchant API'siga kirmaydi.
		w.Do(t, adm, "GET", "/v2/merchant/me", nil).Expect(t, 401)
	})

	w.Step(t, "keyingi kirish: kod -> PAROL (har safar) -> TOTP (har safar)", func(t *testing.T) {
		o := post(t, "/v2/auth/admin/otp/verify", map[string]any{"phone": phone, "code": code(t, phone)}, 200)
		if o.Method != "password" {
			t.Fatalf("TOTP ulangan adminga ham parol so'ralishi kerak: %+v", o)
		}
		post(t, "/v2/auth/mfa/factor", map[string]any{"mfa_token": o.MFAToken, "password": "xato-parol"}, 401)
		o = post(t, "/v2/auth/mfa/factor", map[string]any{"mfa_token": o.MFAToken, "password": pw}, 200)
		if o.Method != "totp" {
			t.Fatalf("paroldan keyin TOTP: %+v", o)
		}
		time.Sleep(10 * time.Millisecond)
		c, _ := mfa.Code(secret, time.Now().Add(mfa.Period))
		tok := post(t, "/v2/auth/mfa/verify", map[string]any{"mfa_token": o.MFAToken, "code": c}, 200)
		if tok.AccessToken == "" {
			t.Fatalf("verify: %+v", tok)
		}
		w.Do(t, &Actor{Name: "admin", Token: tok.AccessToken}, "GET", "/admin/couriers", nil).Expect(t, 200)
		adminTok = tok.AccessToken
	})

	w.Step(t, "KYC: navbat, qaror faqat Authenticator kodi bilan, sababsiz rad yo'q; begona tokenlar 401", func(t *testing.T) {
		org := &authz.Organization{ID: "e2e-kyc-org", Name: "KYC Cafe MCHJ", TIN: "123456789", OwnerUserID: w.Owner.UserID}
		if err := w.Authz.Store().CreateOrganization(ctx, org); err != nil {
			t.Fatal(err)
		}
		adm := &Actor{Name: "admin", Token: adminTok}

		var q struct {
			Organizations []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"organizations"`
		}
		w.Do(t, adm, "GET", "/v2/admin/organizations?status=pending", nil).Expect(t, 200).JSON(t, &q)
		if len(q.Organizations) != 1 || q.Organizations[0].ID != org.ID {
			t.Fatalf("KYC navbati: %+v", q)
		}
		path := "/v2/admin/organizations/" + org.ID + "/verify"
		r := w.Do(t, adm, "POST", path, map[string]any{"decision": "approve"}).Expect(t, 403).Map(t)
		if r["step_up_required"] != true {
			t.Fatalf("kodsiz tasdiq: %v", r)
		}
		w.Do(t, adm, "POST", path, map[string]any{"decision": "reject"}).Expect(t, 400)
		w.Do(t, w.Owner, "POST", path, map[string]any{"decision": "approve"}).Expect(t, 401)
		w.Do(t, w.Admin, "GET", "/v2/admin/organizations", nil).Expect(t, 401) // eski HS256 admin tokeni
		w.Do(t, adm, "POST", "/v2/admin/organizations/yoq/verify", map[string]any{"decision": "approve", "stepup_code": "000000"}).Expect(t, 404)

		// Haqiqiy soatda TOTP qadamlari ishlatib bo'lingan — tiklash kodi ham step-up uchun yaraydi.
		c := recovery[0]
		var res struct {
			Status string `json:"status"`
		}
		w.Do(t, adm, "POST", path, map[string]any{"decision": "approve", "stepup_code": c}).Expect(t, 200).JSON(t, &res)
		if res.Status != authz.OrgActive {
			t.Fatalf("tasdiqlanmadi: %+v", res)
		}
		// whoami: ekotizim servislari (OnDexMap) admin tokenini shu yerda tekshiradi.
		var who struct {
			UserID string `json:"user_id"`
		}
		w.Do(t, adm, "GET", "/v2/admin/whoami", nil).Expect(t, 200).JSON(t, &who)
		if who.UserID != "e2e-a6-admin" {
			t.Fatalf("whoami: %+v", who)
		}
		w.Do(t, w.Owner, "GET", "/v2/admin/whoami", nil).Expect(t, 401)
		w.Do(t, w.Admin, "GET", "/v2/admin/whoami", nil).Expect(t, 401)
		got, _ := w.Authz.Store().GetOrganization(ctx, org.ID)
		if got.VerifiedBy != "e2e-a6-admin" || got.VerifiedAt == nil {
			t.Fatalf("kim tasdiqlagani yozilmadi: %+v", got)
		}
	})

	w.Step(t, "XAVFSIZLIK: TOTP'siz (faqat kod) admin sessiyasi eski marshrutlarda o'tmaydi", func(t *testing.T) {
		weak, err := w.Identity.Create(ctx, identity.CreateInput{UserID: "e2e-a6-admin", ClientID: identity.AdminClientID,
			AMR: []string{"otp", "pwd"}})
		if err != nil {
			t.Fatal(err)
		}
		w.Do(t, &Actor{Name: "admin (2FA'siz)", Token: weak.AccessToken}, "GET", "/admin/couriers", nil).Expect(t, 401)
	})
}
