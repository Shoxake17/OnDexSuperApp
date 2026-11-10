package e2e

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"

	"chustapp/internal/e2e/report"
	"chustapp/internal/identity"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// softKey — dasturiy passkey (platforma autentifikatori o'rnida): ES256,
// "none" attestatsiya, foydalanuvchi tekshirilgan (UV). Brauzer qiladigan
// ishni qiladi: clientDataJSON'ga ORIGIN yoziladi — fishing sinovi uchun
// origin o'zgartiriladi.
type softKey struct {
	id    []byte
	priv  *ecdsa.PrivateKey
	rpID  string
	count uint32
}

func newSoftKey(t *testing.T, rpID string) *softKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 32)
	_, _ = rand.Read(id)
	return &softKey{id: id, priv: priv, rpID: rpID}
}

var b64u = base64.RawURLEncoding

func (k *softKey) authData(attested bool) []byte {
	rp := sha256.Sum256([]byte(k.rpID))
	k.count++
	flags := byte(0x01 | 0x04) // UP | UV
	if attested {
		flags |= 0x40 // AT
	}
	var b bytes.Buffer
	b.Write(rp[:])
	b.WriteByte(flags)
	_ = binary.Write(&b, binary.BigEndian, k.count)
	if attested {
		b.Write(make([]byte, 16)) // AAGUID
		_ = binary.Write(&b, binary.BigEndian, uint16(len(k.id)))
		b.Write(k.id)
		x, y := k.priv.X.FillBytes(make([]byte, 32)), k.priv.Y.FillBytes(make([]byte, 32))
		cose, _ := webauthncbor.Marshal(map[int]any{1: 2, 3: -7, -1: 1, -2: x, -3: y})
		b.Write(cose)
	}
	return b.Bytes()
}

func clientData(typ, challenge, origin string) []byte {
	raw, _ := json.Marshal(map[string]any{"type": typ, "challenge": challenge, "origin": origin, "crossOrigin": false})
	return raw
}

// create — navigator.credentials.create javobi.
func (k *softKey) create(t *testing.T, opts map[string]any, origin string) json.RawMessage {
	t.Helper()
	ch, _ := opts["challenge"].(string)
	att, err := webauthncbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": k.authData(true)})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(map[string]any{"id": b64u.EncodeToString(k.id), "rawId": b64u.EncodeToString(k.id),
		"type": "public-key", "authenticatorAttachment": "platform", "clientExtensionResults": map[string]any{},
		"response": map[string]any{"clientDataJSON": b64u.EncodeToString(clientData("webauthn.create", ch, origin)),
			"attestationObject": b64u.EncodeToString(att), "transports": []string{"internal"}}})
	return raw
}

// get — navigator.credentials.get javobi (imzo).
func (k *softKey) get(t *testing.T, opts map[string]any, origin string) json.RawMessage {
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
			"signature": b64u.EncodeToString(sig)}})
	return raw
}

// AUD-4: passkey — ulash (akkaunt sahifasi, Authenticator kodi bilan) va
// kirishning ikkinchi bosqichi; fishing domeni, replay, boshqa tokenga
// ko'chirish, klon (hisoblagich orqaga) — hammasi rad.
func TestPasskeySecondFactor(t *testing.T) {
	w := NewWorld(t, "Passkey (WebAuthn) — fishingga chidamli ikkinchi bosqich (AUD-4)")
	ctx := context.Background()
	const origin, rpID = "https://api.e2e.ondex", "api.e2e.ondex"
	const phone, pwd = "+998900000091", "Passkey-Egasi-2026x"

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

	// Egasi: ro'yxatdan o'tish + TOTP (servis orqali — umumiy MFA limiti sarflanmaydi).
	_, code, err := w.AuthSvc.IssueCode(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}
	var su struct {
		MFAToken string `json:"mfa_token"`
	}
	w.Do(t, nil, "POST", "/v2/merchant/signup", map[string]any{"phone": phone, "code": code, "password": pwd,
		"password_confirm": pwd, "first_name": "Passkey", "org": map[string]any{"name": "Passkey MChJ"},
		"business": map[string]any{"type": "restaurant", "name": "Passkey Oshxona"}, "client_id": "merchant-web"}).
		Expect(t, 200).JSON(t, &su)
	en, err := w.MerchantAuth.EnrollStart(ctx, su.MFAToken)
	if err != nil {
		t.Fatal(err)
	}
	c0, _ := mfa.Code(en.Secret, time.Now())
	_, recovery, err := w.MerchantAuth.EnrollConfirm(ctx, su.MFAToken, c0, identity.Meta{})
	if err != nil || len(recovery) < 2 {
		t.Fatalf("TOTP ulash: %v", err)
	}
	owner, err := w.Users.GetByPhoneRealm(ctx, users.RealmMerchant, phone)
	if err != nil {
		t.Fatal(err)
	}

	// Akkaunt sahifasi sessiyasi (SSO cookie).
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

	w.Step(t, "ulash: Authenticator kodisiz rad; CSRF'siz rad; kod bilan -> passkey saqlanadi; replay rad", func(t *testing.T) {
		send(t, "POST", "/v2/oauth/account/api/passkeys/options", acc, map[string]any{}).Expect(t, 400)
		send(t, "POST", "/v2/oauth/account/api/passkeys/options", map[string]string{"Cookie": cookie}, map[string]any{"code": recovery[0]}).Expect(t, 403)
		var o optsResp
		send(t, "POST", "/v2/oauth/account/api/passkeys/options", acc, map[string]any{"code": recovery[0]}).Expect(t, 200).JSON(t, &o)
		if sel, _ := o.PublicKey["authenticatorSelection"].(map[string]any); sel["userVerification"] != "required" {
			t.Fatalf("UV majburiy emas: %v", o.PublicKey["authenticatorSelection"])
		}
		body := map[string]any{"state": o.State, "credential": key.create(t, o.PublicKey, origin), "name": "Noutbuk (Windows Hello)"}
		send(t, "POST", "/v2/oauth/account/api/passkeys", acc, body).Expect(t, 201)
		send(t, "POST", "/v2/oauth/account/api/passkeys", acc, body).Expect(t, 400) // challenge bir martalik

		var list struct {
			Passkeys []identity.PasskeyView `json:"passkeys"`
		}
		send(t, "GET", "/v2/oauth/account/api/passkeys", map[string]string{"Cookie": cookie}, nil).Expect(t, 200).JSON(t, &list)
		if len(list.Passkeys) != 1 || list.Passkeys[0].Name != "Noutbuk (Windows Hello)" {
			t.Fatalf("ro'yxat: %+v", list.Passkeys)
		}
		if strings.Contains(string(send(t, "GET", "/v2/oauth/account/api/passkeys", map[string]string{"Cookie": cookie}, nil).Body), "publicKey") {
			t.Fatal("ro'yxatda kalit ma'lumoti chiqdi")
		}
	})

	login := func(t *testing.T) string {
		t.Helper()
		out, err := w.MerchantAuth.LoginPassword(ctx, phone, pwd, "merchant-web", identity.Meta{})
		if err != nil || out.MFA == nil || out.MFA.Method != "totp" || !out.MFA.Passkey {
			t.Fatalf("ikkinchi bosqichda passkey taklif qilinmadi: %+v %v", out, err)
		}
		return out.MFA.Token
	}
	options := func(t *testing.T, mfaToken string) optsResp {
		t.Helper()
		var o optsResp
		send(t, "POST", "/v2/auth/mfa/passkey/options", nil, map[string]any{"mfa_token": mfaToken}).Expect(t, 200).JSON(t, &o)
		return o
	}

	w.Step(t, "kirish: FISHING domenidagi imzo rad; boshqa mfa_token'ga ko'chirilgan holat rad", func(t *testing.T) {
		tok := login(t)
		o := options(t, tok)
		if allow, _ := o.PublicKey["allowCredentials"].([]any); len(allow) != 1 {
			t.Fatalf("allowCredentials: %v", o.PublicKey["allowCredentials"])
		}
		// Foydalanuvchi soxta saytda (proksi) passkey'ni tasdiqladi — imzo baribir yaroqsiz.
		send(t, "POST", "/v2/auth/mfa/passkey", nil, map[string]any{"mfa_token": tok, "state": o.State,
			"credential": key.get(t, o.PublicKey, "https://ondex-uz.login-secure.example")}).Expect(t, 401)

		o = options(t, tok)
		other := login(t)
		send(t, "POST", "/v2/auth/mfa/passkey", nil, map[string]any{"mfa_token": other, "state": o.State,
			"credential": key.get(t, o.PublicKey, origin)}).Expect(t, 400)
	})

	w.Step(t, "kirish: haqiqiy saytda passkey -> sessiya (amr hwk); javobni qayta yuborish rad", func(t *testing.T) {
		tok := login(t)
		o := options(t, tok)
		body := map[string]any{"mfa_token": tok, "state": o.State, "credential": key.get(t, o.PublicKey, origin)}
		var tr struct {
			AccessToken string `json:"access_token"`
		}
		send(t, "POST", "/v2/auth/mfa/passkey", nil, body).Expect(t, 200).JSON(t, &tr)
		c, err := w.Identity.Authenticate(ctx, tr.AccessToken, users.RealmMerchant)
		if err != nil || !strings.Contains(strings.Join(c.AMR, ","), "hwk") {
			t.Fatalf("sessiya/amr: %+v %v", c, err)
		}
		send(t, "POST", "/v2/auth/mfa/passkey", nil, body).Expect(t, 400)
	})

	w.Step(t, "klonlangan kalit (hisoblagich orqaga) rad", func(t *testing.T) {
		tok := login(t)
		o := options(t, tok)
		key.count = 0 // nusxa: hisoblagich eski qiymatdan
		send(t, "POST", "/v2/auth/mfa/passkey", nil, map[string]any{"mfa_token": tok, "state": o.State,
			"credential": key.get(t, o.PublicKey, origin)}).Expect(t, 401)
	})

	w.Step(t, "o'chirish: kodsiz rad, kod bilan -> passkey yo'q, kirishda taklif qilinmaydi", func(t *testing.T) {
		id := b64u.EncodeToString(key.id)
		send(t, "POST", "/v2/oauth/account/api/passkeys/delete", acc, map[string]any{"id": id}).Expect(t, 400)
		send(t, "POST", "/v2/oauth/account/api/passkeys/delete", acc, map[string]any{"id": id, "code": recovery[1]}).Expect(t, 200)
		out, err := w.MerchantAuth.LoginPassword(ctx, phone, pwd, "merchant-web", identity.Meta{})
		if err != nil || out.MFA == nil || out.MFA.Passkey {
			t.Fatalf("o'chirilgan passkey hali taklif qilinmoqda: %+v %v", out, err)
		}
		send(t, "POST", "/v2/auth/mfa/passkey/options", nil, map[string]any{"mfa_token": out.MFA.Token}).Expect(t, 400)
	})
}
