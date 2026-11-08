package identity_test

import (
	"strings"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// Eskirgan, lekin IMZOSI TO'G'RI kirish so'rovi -> ilovaning kirish sahifasi
// (eski tab / uzoq TOTP ulash). Soxta yoki buzilgan so'rov -> hech qayerga.
func TestOIDC_RestartURLOnlyForExpiredGenuineRequests(t *testing.T) {
	f := newFixture(t)
	p, err := identity.NewProvider(f.svc, identity.NewMemoryCodeStore(), []identity.OIDCClient{{
		ID: rpID, SecretHash: identity.HashClientSecret(rpSecret), RedirectURIs: []string{rpRedirect},
		InitiateLoginURI: "https://pos.ondex.test/login",
	}})
	if err != nil {
		t.Fatal(err)
	}
	f.user(t, "u_owner01", users.RoleMerchant)
	req, _, err := p.ParseAuthorize(authorizeQuery(nil))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := p.SignRequest(req)

	if got := p.RestartURL(raw); got != "" {
		t.Fatalf("yangi so'rov uchun qayta boshlash berildi: %q", got)
	}
	f.clk.add(16 * time.Minute)
	if _, err := p.OpenRequest(raw); err == nil {
		t.Fatal("eskirgan so'rov qabul qilindi")
	}
	if got := p.RestartURL(raw); got != "https://pos.ondex.test/login" {
		t.Fatalf("eskirgan haqiqiy so'rov: %q", got)
	}
	payload, sig, _ := strings.Cut(raw, ".")
	for _, bad := range []string{raw + "x", payload + ".AAAA", "x" + payload[1:] + "." + sig, ""} {
		if got := p.RestartURL(bad); got != "" {
			t.Fatalf("XAVFSIZLIK: soxta so'rov qayta boshlashga yo'naltirdi: %q -> %q", bad, got)
		}
	}
	if _, err := identity.NewProvider(f.svc, identity.NewMemoryCodeStore(), []identity.OIDCClient{{
		ID: rpID, RedirectURIs: []string{rpRedirect}, InitiateLoginURI: "http://evil.test/login",
	}}); err == nil {
		t.Fatal("https bo'lmagan initiate_login_uri qabul qilindi")
	}
}
