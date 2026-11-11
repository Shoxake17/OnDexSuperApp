package e2e

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// PK-1/PK-2 (Plane 318/319): passkey ilova ICHIDA (Android/iOS/Windows) va
// barcha realmlarda — xodim, mijoz, admin. Domen bog'lanishi (.well-known),
// Android imzosi (soxta ilova rad), step-up (TOTP yoki Telegram kodi),
// realm/ilova chegarasi, admin IP ro'yxati.
func TestPasskeyNativeApps(t *testing.T) {
	w := NewWorld(t, "Passkey native ilovalarda va barcha realmlarda (PK-1/PK-2)")
	ctx := context.Background()
	const rpID = "e2e.ondex"
	windowsOrigin := "https://" + rpID // passkeys_windows: "https://"+RPID
	androidOrigin := E2EAndroidApp.Origin()
	fakeApp := identity.AndroidApp{Package: "com.ondex.waiter", Fingerprint: make([]byte, 32)}.Origin()

	code := func(t *testing.T, p string) string {
		t.Helper()
		_, c, err := w.AuthSvc.IssueCode(ctx, p)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	mkUser := func(t *testing.T, id, phone string, role users.Role) *users.User {
		t.Helper()
		u := &users.User{ID: id, Phone: phone, Role: role, PhoneVerified: true, CreatedAt: time.Now()}
		if err := w.Users.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	session := func(t *testing.T, userID, client string, amr ...string) *Actor {
		t.Helper()
		tok, err := w.Identity.Create(ctx, identity.CreateInput{UserID: userID, ClientID: client, AMR: amr})
		if err != nil {
			t.Fatal(err)
		}
		return &Actor{Name: client, Token: tok.AccessToken}
	}
	type opts struct {
		PublicKey map[string]any `json:"public_key"`
		State     string         `json:"state"`
	}
	register := func(t *testing.T, a *Actor, key *softKey, codeV, origin string) {
		t.Helper()
		var o opts
		w.Do(t, a, "POST", "/v2/me/passkeys/options", map[string]any{"code": codeV}).Expect(t, 200).JSON(t, &o)
		if sel, _ := o.PublicKey["authenticatorSelection"].(map[string]any); sel["residentKey"] != "required" {
			t.Fatalf("resident key majburiy emas: %v", sel)
		}
		if rp, _ := o.PublicKey["rp"].(map[string]any); rp["id"] != rpID {
			t.Fatalf("RP ID eTLD+1 emas: %v", rp)
		}
		w.Do(t, a, "POST", "/v2/me/passkeys", map[string]any{"state": o.State,
			"credential": key.create(t, o.PublicKey, origin), "name": "Telefon"}).Expect(t, 201)
	}
	login := func(t *testing.T, client string, key *softKey, origin, userID string, want int) string {
		t.Helper()
		var o opts
		w.Do(t, nil, "POST", "/v2/auth/passkey/options", map[string]any{"client_id": client}).Expect(t, 200).JSON(t, &o)
		var tr struct {
			AccessToken string `json:"access_token"`
		}
		r := w.Do(t, nil, "POST", "/v2/auth/passkey/login", map[string]any{"client_id": client, "state": o.State,
			"credential": key.getDiscoverable(t, o.PublicKey, origin, userID), "device_name": "e2e"}).Expect(t, want)
		if want == 200 {
			r.JSON(t, &tr)
		}
		return tr.AccessToken
	}

	w.Step(t, ".well-known: assetlinks (faqat get_login_creds) va AASA (webcredentials)", func(t *testing.T) {
		var links []struct {
			Relation []string `json:"relation"`
			Target   struct {
				Package string   `json:"package_name"`
				SHA     []string `json:"sha256_cert_fingerprints"`
			} `json:"target"`
		}
		r := w.Do(t, nil, "GET", "/.well-known/assetlinks.json", nil).Expect(t, 200)
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("Content-Type: %q", ct)
		}
		r.JSON(t, &links)
		if len(links) != 1 || links[0].Target.Package != "com.ondex.waiter" || len(links[0].Target.SHA) != 1 ||
			strings.Join(links[0].Relation, ",") != "delegate_permission/common.get_login_creds" {
			t.Fatalf("assetlinks: %+v", links)
		}
		var aasa map[string]map[string][]string
		w.Do(t, nil, "GET", "/.well-known/apple-app-site-association", nil).Expect(t, 200).JSON(t, &aasa)
		if apps := aasa["webcredentials"]["apps"]; len(apps) != 1 || apps[0] != E2EAppleAppID {
			t.Fatalf("aasa: %v", aasa)
		}
	})

	waiter := mkUser(t, "e2e-pk-waiter", "+998900000071", users.RoleWaiter)
	wkey := newSoftKey(t, rpID)

	w.Step(t, "xodim (TOTP'siz): passkey Telegram kodi bilan qo'shiladi; kodsiz/xato kod rad", func(t *testing.T) {
		a := session(t, waiter.ID, "ondex-pro", "otp")
		w.Do(t, a, "POST", "/v2/me/passkeys/options", map[string]any{}).Expect(t, 400)
		c := code(t, waiter.Phone) // kod yuborildi, lekin avval boshqasi terildi
		w.Do(t, a, "POST", "/v2/me/passkeys/options", map[string]any{"code": "000000"}).Expect(t, 400)
		register(t, a, wkey, c, androidOrigin)
		var list struct {
			Passkeys []identity.PasskeyView `json:"passkeys"`
			RPID     string                 `json:"rp_id"`
		}
		w.Do(t, a, "GET", "/v2/me/passkeys", nil).Expect(t, 200).JSON(t, &list)
		if len(list.Passkeys) != 1 || list.RPID != rpID {
			t.Fatalf("ro'yxat: %+v", list)
		}
		w.Do(t, nil, "GET", "/v2/me/passkeys", nil).Expect(t, 401)
	})

	w.Step(t, "xodim: Android ilovadan kirish; SOXTA ilova imzosi rad; boshqa ilova (OnDexGO) 403", func(t *testing.T) {
		tok := login(t, "ondex-pro", wkey, androidOrigin, waiter.ID, 200)
		c, err := w.Identity.Authenticate(ctx, tok, users.RealmMerchant)
		if err != nil || c.ClientID != "ondex-pro" || strings.Join(c.AMR, ",") != "hwk,user" {
			t.Fatalf("sessiya: %+v %v", c, err)
		}
		login(t, "ondex-pro", wkey, fakeApp, waiter.ID, 401)
		login(t, "ondex-go", wkey, androidOrigin, waiter.ID, 403)
	})

	customer := mkUser(t, "e2e-pk-cust", "+998900000072", users.RoleCustomer)
	ckey := newSoftKey(t, rpID)

	w.Step(t, "mijoz: passkey (Telegram kodi), Windows/brauzer origin bilan kirish; biznes ilovasiga kira olmaydi", func(t *testing.T) {
		a := session(t, customer.ID, identity.CustomerClientID, "otp")
		register(t, a, ckey, code(t, customer.Phone), windowsOrigin)
		tok := login(t, identity.CustomerClientID, ckey, windowsOrigin, customer.ID, 200)
		if _, err := w.Identity.Authenticate(ctx, tok, users.RealmCustomer); err != nil {
			t.Fatal(err)
		}
		login(t, "ondex-pro", ckey, windowsOrigin, customer.ID, 401)
		// Va aksincha: biznes xodimining passkey'i mijoz klientida — rad.
		login(t, identity.CustomerClientID, wkey, androidOrigin, waiter.ID, 401)
	})

	w.Step(t, "o'chirish: Telegram kodi bilan; o'chirilgan passkey bilan kirish rad", func(t *testing.T) {
		a := session(t, customer.ID, identity.CustomerClientID, "otp")
		w.Do(t, a, "POST", "/v2/me/passkeys/delete", map[string]any{"id": b64u.EncodeToString(ckey.id)}).Expect(t, 400)
		w.Do(t, a, "POST", "/v2/me/passkeys/delete", map[string]any{"id": b64u.EncodeToString(ckey.id),
			"code": code(t, customer.Phone)}).Expect(t, 200)
		login(t, identity.CustomerClientID, ckey, windowsOrigin, customer.ID, 401)
	})

	// Admin: birinchi kirish (kod -> parol -> TOTP), so'ng passkey.
	const aPhone, aPw = "+998900000073", "Admin-Passkey-Parol-73"
	admin := mkUser(t, "e2e-pk-admin", aPhone, users.RoleAdmin)
	akey := newSoftKey(t, rpID)
	var secret string
	type outcome struct {
		MFAToken string `json:"mfa_token"`
		Secret   string `json:"secret"`
		Tokens   struct {
			AccessToken string `json:"access_token"`
		} `json:"tokens"`
	}
	post := func(t *testing.T, path string, body map[string]any) outcome {
		t.Helper()
		var o outcome
		w.Do(t, nil, "POST", path, body).Expect(t, 200).JSON(t, &o)
		return o
	}

	w.Step(t, "admin: TOTP bilan passkey qo'shadi; Telegram kodi YETMAYDI (TOTP majburiy)", func(t *testing.T) {
		o := post(t, "/v2/auth/admin/otp/verify", map[string]any{"phone": aPhone, "code": code(t, aPhone)})
		o = post(t, "/v2/auth/mfa/password", map[string]any{"mfa_token": o.MFAToken, "password": aPw, "password_confirm": aPw})
		en := post(t, "/v2/auth/mfa/totp/enroll", map[string]any{"mfa_token": o.MFAToken})
		secret = en.Secret
		c, _ := mfa.Code(secret, time.Now())
		done := post(t, "/v2/auth/mfa/totp/confirm", map[string]any{"mfa_token": o.MFAToken, "code": c})
		a := &Actor{Name: "admin", Token: done.Tokens.AccessToken}
		w.Do(t, a, "POST", "/v2/me/passkeys/options", map[string]any{"code": code(t, aPhone)}).Expect(t, 400)
		c2, _ := mfa.Code(secret, time.Now().Add(mfa.Period))
		register(t, a, akey, c2, windowsOrigin)
	})

	w.Step(t, "admin: passkey bilan kirish -> admin API ochiq (kuchli AMR); boshqa IP'dan 403", func(t *testing.T) {
		ratelimit.ResetAll() // bitta ssenariy passkey IP budjetini (30) tugatadi
		tok := login(t, identity.AdminClientID, akey, windowsOrigin, admin.ID, 200)
		a := &Actor{Name: "admin (passkey)", Token: tok}
		w.Do(t, a, "GET", "/admin/couriers", nil).Expect(t, 200)
		// Passkey'siz zaif admin sessiyasi (faqat kod) — rad.
		weak := session(t, admin.ID, identity.AdminClientID, "otp", "pwd")
		w.Do(t, weak, "GET", "/admin/couriers", nil).Expect(t, 401)
		// Mijoz passkey'i admin klientida — rad.
		login(t, identity.AdminClientID, wkey, androidOrigin, waiter.ID, 401)

		w.api.AdminAllowedNets = adminNetsDeny()
		defer func() { w.api.AdminAllowedNets = nil }()
		w.Do(t, nil, "POST", "/v2/auth/passkey/options", map[string]any{"client_id": identity.AdminClientID}).Expect(t, 403)
		w.Do(t, a, "GET", "/v2/me/passkeys", nil).Expect(t, 403)
	})
}

// adminNetsDeny — test klienti (127.0.0.1) ruxsat etilmagan ro'yxat.
func adminNetsDeny() []netip.Prefix { return []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")} }
