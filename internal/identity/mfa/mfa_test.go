package mfa_test

import (
	"encoding/base32"
	"net/url"
	"strings"
	"testing"
	"time"

	"chustapp/internal/identity/mfa"
	"chustapp/internal/identity/mfa/mfatest"
)

// RFC 6238 B ilova: SHA-1 kaliti "12345678901234567890", 8 xonali qiymatlarning
// oxirgi 6 raqami.
func TestTOTP_RFC6238Vectors(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	for unix, want := range map[int64]string{
		59:          "287082",
		1111111109:  "081804",
		1111111111:  "050471",
		1234567890:  "005924",
		2000000000:  "279037",
		20000000000: "353130",
	} {
		got, err := mfa.Code(secret, time.Unix(unix, 0))
		if err != nil || got != want {
			t.Errorf("t=%d: %q %v, kutilgan %q", unix, got, err, want)
		}
	}
}

func TestTOTP_ValidateWindow(t *testing.T) {
	secret, err := mfa.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 7, 12, 0, 15, 0, time.UTC)
	cur, _ := mfa.Code(secret, now)
	prev, _ := mfa.Code(secret, now.Add(-mfa.Period))
	next, _ := mfa.Code(secret, now.Add(mfa.Period))
	old, _ := mfa.Code(secret, now.Add(-3*mfa.Period))
	for name, c := range map[string]struct {
		code string
		ok   bool
		step int64
	}{
		"joriy":          {cur, true, mfa.Step(now)},
		"oldingi qadam":  {prev, true, mfa.Step(now) - 1},
		"keyingi qadam":  {next, true, mfa.Step(now) + 1},
		"eski (3 qadam)": {old, false, 0},
		"bo'sh":          {"", false, 0},
		"5 raqam":        {cur[:5], false, 0},
		"harf":           {"12a456", false, 0},
	} {
		step, ok := mfa.Validate(secret, c.code, now)
		if ok != c.ok || (ok && step != c.step) {
			t.Errorf("%s: ok=%v step=%d, kutilgan %v %d", name, ok, step, c.ok, c.step)
		}
	}
	if _, ok := mfa.Validate("buzuq!", cur, now); ok {
		t.Error("yaroqsiz kalit bilan o'tdi")
	}
}

func TestURI(t *testing.T) {
	u, err := url.Parse(mfa.URI("OnDex Merchant", "+998901234567", "ABCDEFGHIJKLMNOP"))
	if err != nil || u.Scheme != "otpauth" || u.Host != "totp" {
		t.Fatalf("URI: %v %v", u, err)
	}
	q := u.Query()
	if q.Get("secret") != "ABCDEFGHIJKLMNOP" || q.Get("issuer") != "OnDex Merchant" || q.Get("digits") != "6" {
		t.Fatalf("parametrlar: %v", q)
	}
}

func TestRecoveryCodes(t *testing.T) {
	plain, hashes, err := mfa.NewRecoveryCodes()
	if err != nil || len(plain) != mfa.RecoveryCount || len(hashes) != mfa.RecoveryCount {
		t.Fatalf("%d %d %v", len(plain), len(hashes), err)
	}
	seen := map[string]bool{}
	for i, c := range plain {
		if len(c) != 11 || c[5] != '-' || seen[c] || strings.ContainsAny(c, "01OIL") {
			t.Errorf("kod formati: %q", c)
		}
		seen[c] = true
		if string(mfa.HashRecovery(strings.ToLower(strings.ReplaceAll(c, "-", " ")))) != string(hashes[i]) {
			t.Errorf("normallashtirish: %q", c)
		}
		if !mfa.LooksLikeRecovery(c) || mfa.LooksLikeRecovery("123456") {
			t.Error("LooksLikeRecovery")
		}
	}
}

func TestMemStoreContract(t *testing.T) {
	mfatest.RunStoreContract(t, func(*testing.T) mfa.Store { return mfa.NewMemStore() }, "u1")
}
