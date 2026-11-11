package e2e

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"chustapp/internal/e2e/report"
	"chustapp/internal/identity"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// getDiscoverable — telefon raqamisiz kirish javobi: userHandle (akkaunt ID)
// KALIT ichidan keladi (resident/discoverable passkey).
func (k *softKey) getDiscoverable(t *testing.T, opts map[string]any, origin, userID string) json.RawMessage {
	t.Helper()
	ch, _ := opts["challenge"].(string)
	ad := k.authData(false)
	cd := clientData("webauthn.get", ch, origin)
	cdh := sha256.Sum256(cd)
	digest := sha256.Sum256(append(append([]byte{}, ad...), cdh[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, k.priv, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"id": b64u.EncodeToString(k.id), "rawId": b64u.EncodeToString(k.id),
		"type": "public-key", "authenticatorAttachment": "platform", "clientExtensionResults": map[string]any{},
		"response": map[string]any{"clientDataJSON": b64u.EncodeToString(cd), "authenticatorData": b64u.EncodeToString(ad),
			"signature": b64u.EncodeToString(sig), "userHandle": b64u.EncodeToString([]byte(userID))}})
	return raw
}

// Plane 317: passkey — ASOSIY kirish (telefon raqamisiz), parol ixtiyoriy
// (akkaunt sahifasida Authenticator kodi bilan yoqiladi/o'chiriladi).
func TestPasswordlessPasskeyLogin(t *testing.T) {
	w := NewWorld(t, "Passkey asosiy kirish, parol ixtiyoriy (Plane 317)")
	ctx := context.Background()
	const origin, rpID = "https://api.e2e.ondex", "api.e2e.ondex"
	const phone, pwd = "+998900000095", "Parolsiz-Egasi-2026x"

	noRedirect := &http.Client{Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	send := func(t *testing.T, method, path string, hdr map[string]string, body any) *Resp {
		t.Helper()
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, w.Srv.URL+path, rd)
		req.Header.Set("Content-Type", "application/json")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		start := time.Now()
		res, err := noRedirect.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		Rec.AddCall(report.Call{Scenario: w.scenario, Step: w.step, Method: method, Path: path,
			Route: MatchRoute(Routes, method, path), Status: res.StatusCode,
			Ms: float64(time.Since(start).Microseconds()) / 1000, Actor: "brauzer"})
		r := &Resp{Status: res.StatusCode, Body: raw, Header: res.Header}
		w.checkGeneric(t, method, path, r)
		return r
	}

	// Egasi: ro'yxatdan o'tish (parol bilan -> parol bilan kirish YOQIQ) + TOTP.
	_, code, err := w.AuthSvc.IssueCode(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}
	var su struct {
		MFAToken string `json:"mfa_token"`
	}
	w.Do(t, nil, "POST", "/v2/merchant/signup", map[string]any{"phone": phone, "code": code, "password": pwd,
		"password_confirm": pwd, "first_name": "Parolsiz", "org": map[string]any{"name": "Parolsiz MChJ"},
		"business": map[string]any{"type": "restaurant", "name": "Parolsiz Oshxona"}, "client_id": "merchant-web"}).
		Expect(t, 200).JSON(t, &su)
	en, err := w.MerchantAuth.EnrollStart(ctx, su.MFAToken)
	if err != nil {
		t.Fatal(err)
	}
	c0, _ := mfa.Code(en.Secret, time.Now())
	_, recovery, err := w.MerchantAuth.EnrollConfirm(ctx, su.MFAToken, c0, identity.Meta{})
	if err != nil || len(recovery) < 4 {
		t.Fatalf("TOTP ulash: %v", err)
	}
	owner, err := w.Users.GetByPhoneRealm(ctx, users.RealmMerchant, phone)
	if err != nil {
		t.Fatal(err)
	}
	sso, err := w.Identity.Create(ctx, identity.CreateInput{UserID: owner.ID, ClientID: identity.SSOClientID, AMR: []string{"pwd", "totp"}})
	if err != nil {
		t.Fatal(err)
	}
	r := send(t, "POST", "/v2/oauth/login/complete", map[string]string{"Authorization": "Bearer " + sso.AccessToken},
		map[string]any{"account": true}).Expect(t, 200)
	cookie, _, _ := strings.Cut(r.Header.Get("Set-Cookie"), ";")
	acc := map[string]string{"Cookie": cookie, "X-OnDex-Account": "1", "Origin": origin}

	type optsResp struct {
		PublicKey map[string]any `json:"public_key"`
		State     string         `json:"state"`
	}
	key := newSoftKey(t, rpID)

	w.Step(t, "passkey ulash (resident key MAJBURIY)", func(t *testing.T) {
		var o optsResp
		send(t, "POST", "/v2/oauth/account/api/passkeys/options", acc, map[string]any{"code": recovery[0]}).Expect(t, 200).JSON(t, &o)
		sel, _ := o.PublicKey["authenticatorSelection"].(map[string]any)
		if sel["residentKey"] != "required" || sel["userVerification"] != "required" {
			t.Fatalf("discoverable/UV majburiy emas: %v", sel)
		}
		send(t, "POST", "/v2/oauth/account/api/passkeys", acc, map[string]any{"state": o.State,
			"credential": key.create(t, o.PublicKey, origin), "name": "Telefon"}).Expect(t, 201)
	})

	options := func(t *testing.T, client string, want int) optsResp {
		t.Helper()
		var o optsResp
		r := send(t, "POST", "/v2/auth/passkey/options", nil, map[string]any{"client_id": client}).Expect(t, want)
		if want == 200 {
			r.JSON(t, &o)
		}
		return o
	}

	w.Step(t, "options: telefon so'ralmaydi, allowCredentials BO'SH (akkaunt oshkor emas); noma'lum klient rad", func(t *testing.T) {
		o := options(t, "merchant-web", 200)
		if allow, ok := o.PublicKey["allowCredentials"].([]any); ok && len(allow) > 0 {
			t.Fatalf("XAVFSIZLIK: allowCredentials akkauntni oshkor qiladi: %v", allow)
		}
		if o.PublicKey["userVerification"] != "required" {
			t.Fatalf("UV majburiy emas: %v", o.PublicKey["userVerification"])
		}
		options(t, "begona-klient", 400)
	})

	w.Step(t, "kirish: FISHING domeni rad; boshqa klientga ko'chirilgan holat rad", func(t *testing.T) {
		o := options(t, "merchant-web", 200)
		send(t, "POST", "/v2/auth/passkey/login", nil, map[string]any{"client_id": "merchant-web", "state": o.State,
			"credential": key.getDiscoverable(t, o.PublicKey, "https://ondex-uz.login-secure.example", owner.ID)}).Expect(t, 401)
		o = options(t, "merchant-web", 200)
		send(t, "POST", "/v2/auth/passkey/login", nil, map[string]any{"client_id": identity.SSOClientID, "state": o.State,
			"credential": key.getDiscoverable(t, o.PublicKey, origin, owner.ID)}).Expect(t, 400)
	})

	w.Step(t, "kirish: passkey -> darhol sessiya (amr hwk+user, TOTP so'ralmaydi); replay rad", func(t *testing.T) {
		o := options(t, "merchant-web", 200)
		body := map[string]any{"client_id": "merchant-web", "state": o.State, "device_name": "Chrome",
			"credential": key.getDiscoverable(t, o.PublicKey, origin, owner.ID)}
		var tr struct {
			AccessToken string `json:"access_token"`
			MFARequired bool   `json:"mfa_required"`
		}
		send(t, "POST", "/v2/auth/passkey/login", nil, body).Expect(t, 200).JSON(t, &tr)
		if tr.MFARequired || tr.AccessToken == "" {
			t.Fatalf("passkey sessiya ochmadi: %+v", tr)
		}
		c, err := w.Identity.Authenticate(ctx, tr.AccessToken, users.RealmMerchant)
		if err != nil || strings.Join(c.AMR, ",") != "hwk,user" {
			t.Fatalf("sessiya/amr: %+v %v", c, err)
		}
		send(t, "POST", "/v2/auth/passkey/login", nil, body).Expect(t, 400)
	})

	w.Step(t, "boshqa akkaunt nomidan (userHandle almashtirilgan) imzo rad", func(t *testing.T) {
		o := options(t, "merchant-web", 200)
		send(t, "POST", "/v2/auth/passkey/login", nil, map[string]any{"client_id": "merchant-web", "state": o.State,
			"credential": key.getDiscoverable(t, o.PublicKey, origin, "begona-akkaunt-id")}).Expect(t, 401)
	})

	w.Step(t, "parol sozlamasi: kodsiz rad; o'chirish -> parol 403; yoqish -> parol ishlaydi", func(t *testing.T) {
		var me struct {
			PasswordLogin bool `json:"password_login"`
		}
		send(t, "GET", "/v2/oauth/account/api/me", map[string]string{"Cookie": cookie}, nil).Expect(t, 200).JSON(t, &me)
		if !me.PasswordLogin {
			t.Fatal("parol bilan ro'yxatdan o'tgan egada parol bilan kirish yoqilmagan")
		}
		send(t, "POST", "/v2/oauth/account/api/password-login", acc, map[string]any{"enabled": false}).Expect(t, 400)
		send(t, "POST", "/v2/oauth/account/api/password-login", map[string]string{"Cookie": cookie},
			map[string]any{"enabled": false, "code": recovery[1]}).Expect(t, 403) // CSRF
		send(t, "POST", "/v2/oauth/account/api/password-login", acc, map[string]any{"enabled": false, "code": recovery[1]}).Expect(t, 200)
		if _, err := w.MerchantAuth.LoginPassword(ctx, phone, pwd, "merchant-web", identity.Meta{}); err == nil {
			t.Fatal("XAVFSIZLIK: o'chirilgan parol bilan kirildi")
		}
		w.Do(t, nil, "POST", "/v2/auth/merchant/password", map[string]any{"phone": phone, "password": pwd,
			"client_id": "merchant-web"}).Expect(t, 403)
		send(t, "POST", "/v2/oauth/account/api/password-login", acc, map[string]any{"enabled": true, "code": recovery[2]}).Expect(t, 200)
		if out, err := w.MerchantAuth.LoginPassword(ctx, phone, pwd, "merchant-web", identity.Meta{}); err != nil || out.MFA == nil {
			t.Fatalf("yoqilgan parol bilan kirilmadi: %v", err)
		}
	})
}
