package e2e

import (
	"context"
	"testing"
	"time"

	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// Senariy: MERCHANT KATALOGI (A3c) — haqiqiy HTTP orqali to'liq hayot sikli:
//
//	egasi ro'yxatdan o'tadi (mijoz akkaunti bor raqam bilan) -> TOTP -> /me
//	-> rollar -> xodimni taklif qiladi -> xodim kod bilan kirib taklifni qabul
//	qiladi -> rolini o'zgartiradi -> bo'shatadi (sessiya darhol o'ladi)
//	-> qayta taklif; begona egasi biznesni ko'rmaydi (404).
func TestMerchantDirectoryV2(t *testing.T) {
	w := NewWorld(t, "Merchant katalogi (A3c): ro'yxatdan o'tish, xodimlar, rollar")
	ctx := context.Background()
	code := func(t *testing.T, phone string) string {
		t.Helper()
		_, c, err := w.AuthSvc.IssueCode(ctx, phone)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	type tokens struct {
		AccessToken string `json:"access_token"`
		MFAToken    string `json:"mfa_token"`
		Method      string `json:"method"`
		Secret      string `json:"secret"`
		Tokens      struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}
	signup := func(t *testing.T, phone, org string) *Actor {
		t.Helper()
		var o tokens
		w.Do(t, nil, "POST", "/v2/merchant/signup", map[string]any{
			"phone": phone, "code": code(t, phone), "password": "Kuchli-parol-2026", "password_confirm": "Kuchli-parol-2026",
			"first_name": "Egasi", "org": map[string]any{"name": org}, "business": map[string]any{"type": "restaurant", "name": org},
			"client_id": "merchant-web",
		}).Expect(t, 200).JSON(t, &o)
		if o.Method != "totp_enroll" || o.AccessToken != "" {
			t.Fatalf("ro'yxatdan o'tgach darhol TOTP ulash kutilgan: %+v", o)
		}
		var en tokens
		w.Do(t, nil, "POST", "/v2/auth/mfa/totp/enroll", map[string]any{"mfa_token": o.MFAToken}).Expect(t, 200).JSON(t, &en)
		c, _ := mfa.Code(en.Secret, time.Now())
		var done tokens
		w.Do(t, nil, "POST", "/v2/auth/mfa/totp/confirm", map[string]any{"mfa_token": o.MFAToken, "code": c}).Expect(t, 200).JSON(t, &done)
		return &Actor{Name: "egasi " + org, Token: done.Tokens.AccessToken, Phone: phone}
	}
	type me struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
		MFAEnabled bool `json:"mfa_enabled"`
		Businesses []struct {
			ID        string   `json:"id"`
			OrgStatus string   `json:"org_status"`
			IsOwner   bool     `json:"is_owner"`
			Role      string   `json:"role"`
			Apps      []string `json:"apps"`
		} `json:"businesses"`
	}

	const ownerPhone, staffPhone = "+998900000051", "+998900000052"
	// egasi raqamida avvaldan MIJOZ akkaunti bor — u o'zgarmasligi kerak
	if err := w.Users.Create(ctx, &users.User{ID: "e2e-dir-cust", Phone: ownerPhone, Role: users.RoleCustomer,
		PhoneVerified: true, CreatedAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}

	var owner, staff *Actor
	var bizID, staffMID string

	w.Step(t, "ro'yxatdan o'tish: alohida merchant akkaunti, tashkilot pending, TOTP majburiy", func(t *testing.T) {
		owner = signup(t, ownerPhone, "Book Cafe")
		var m me
		w.Do(t, owner, "GET", "/v2/merchant/me", nil).Expect(t, 200).JSON(t, &m)
		if len(m.Businesses) != 1 || !m.Businesses[0].IsOwner || m.Businesses[0].OrgStatus != "pending" || !m.MFAEnabled {
			t.Fatalf("me: %+v", m)
		}
		if m.User.ID == "e2e-dir-cust" {
			t.Fatal("mijoz akkaunti biznes egasiga aylandi")
		}
		bizID = m.Businesses[0].ID
		if u, _ := w.Users.GetByID(ctx, "e2e-dir-cust"); u.Role != users.RoleCustomer {
			t.Fatalf("mijoz akkaunti o'zgardi: %s", u.Role)
		}
		// ikkinchi marta — 409
		w.Do(t, nil, "POST", "/v2/merchant/signup", map[string]any{"phone": ownerPhone, "code": "000000",
			"password": "Kuchli-parol-2026", "password_confirm": "Kuchli-parol-2026", "first_name": "X",
			"org": map[string]any{"name": "X"}, "business": map[string]any{"type": "store", "name": "X"},
			"client_id": "merchant-web"}).Expect(t, 401) // kod noto'g'ri: akkaunt borligi ham oshkor bo'lmaydi
	})

	w.Step(t, "rollar: shablonlar + katalog; maxsus rol yaratish", func(t *testing.T) {
		var rv struct {
			Roles   []map[string]any `json:"roles"`
			Catalog []map[string]any `json:"catalog"`
		}
		w.Do(t, owner, "GET", "/v2/merchant/businesses/"+bizID+"/roles", nil).Expect(t, 200).JSON(t, &rv)
		if len(rv.Roles) < 5 || len(rv.Catalog) < 10 {
			t.Fatalf("rollar %d, katalog %d", len(rv.Roles), len(rv.Catalog))
		}
		w.Do(t, owner, "POST", "/v2/merchant/businesses/"+bizID+"/roles", map[string]any{"key": "barmen", "name": "Barmen",
			"permissions": []string{"menu.view", "orders.view"}}).Expect(t, 201)
		w.Do(t, owner, "POST", "/v2/merchant/businesses/"+bizID+"/roles", map[string]any{"key": "yomon", "name": "Yomon",
			"permissions": []string{"payouts.steal"}}).Expect(t, 400)
	})

	w.Step(t, "xodim taklifi -> xodim kod bilan kirib qabul qiladi (faqat OnDexPro)", func(t *testing.T) {
		var sv struct {
			MembershipID string `json:"membership_id"`
			Status       string `json:"status"`
		}
		w.Do(t, owner, "POST", "/v2/merchant/businesses/"+bizID+"/staff", map[string]any{"phone": staffPhone,
			"first_name": "Ali", "role_id": authz.Template{Type: authz.TypeRestaurant, Key: "waiter"}.ID()}).Expect(t, 201).JSON(t, &sv)
		if sv.Status != "invited" {
			t.Fatalf("taklif: %+v", sv)
		}
		staffMID = sv.MembershipID
		var o tokens
		w.Do(t, nil, "POST", "/v2/auth/merchant/otp/verify", map[string]any{"phone": staffPhone, "code": code(t, staffPhone),
			"client_id": "ondex-pro"}).Expect(t, 200).JSON(t, &o)
		if o.AccessToken == "" {
			t.Fatalf("xodim kira olmadi: %+v", o)
		}
		staff = &Actor{Name: "ofitsiant (v2)", Token: o.AccessToken, Phone: staffPhone}
		var m me
		w.Do(t, staff, "GET", "/v2/merchant/me", nil).Expect(t, 200).JSON(t, &m)
		if len(m.Businesses) != 1 || m.Businesses[0].Role != "waiter" || m.Businesses[0].IsOwner {
			t.Fatalf("xodim me: %+v", m)
		}
	})

	w.Step(t, "xodim huquqi: xodimlar ro'yxati 403; egasi ko'radi", func(t *testing.T) {
		w.Do(t, staff, "GET", "/v2/merchant/businesses/"+bizID+"/staff", nil).Expect(t, 403)
		var lst struct {
			Staff []map[string]any `json:"staff"`
		}
		w.Do(t, owner, "GET", "/v2/merchant/businesses/"+bizID+"/staff", nil).Expect(t, 200).JSON(t, &lst)
		if len(lst.Staff) != 1 || lst.Staff[0]["status"] != "active" {
			t.Fatalf("ro'yxat: %+v", lst)
		}
	})

	w.Step(t, "rol va ilova o'zgarishi: OnDexPro olib tashlansa xodim sessiyasi biznes API'siga kirmaydi", func(t *testing.T) {
		w.Do(t, owner, "PATCH", "/v2/merchant/businesses/"+bizID+"/staff/"+staffMID, map[string]any{
			"role_id": authz.Template{Type: authz.TypeRestaurant, Key: "cook"}.ID(), "apps": []string{"panel"}}).Expect(t, 200)
		w.Do(t, staff, "GET", "/v2/merchant/businesses/"+bizID+"/roles", nil).Expect(t, 403)
	})

	w.Step(t, "bo'shatish: sessiya DARHOL o'ladi; qayta taklif", func(t *testing.T) {
		w.Do(t, owner, "POST", "/v2/merchant/businesses/"+bizID+"/staff/"+staffMID+"/dismiss", nil).Expect(t, 204)
		w.Do(t, staff, "GET", "/v2/auth/sessions", nil).Expect(t, 401)
		w.Do(t, owner, "POST", "/v2/merchant/businesses/"+bizID+"/staff/"+staffMID+"/invite", nil).Expect(t, 200)
	})

	w.Step(t, "begona egasi va eski HS256 token biznesni ko'rmaydi", func(t *testing.T) {
		other := signup(t, "+998900000053", "Begona Cafe")
		w.Do(t, other, "GET", "/v2/merchant/businesses/"+bizID+"/staff", nil).Expect(t, 404)
		w.Do(t, other, "POST", "/v2/merchant/businesses/"+bizID+"/staff/"+staffMID+"/dismiss", nil).Expect(t, 404)
		w.Do(t, w.Owner, "GET", "/v2/merchant/me", nil).Expect(t, 401)
	})
}
