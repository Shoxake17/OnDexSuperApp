package identity

import (
	"crypto/sha256"
	"encoding/base64"
	"slices"
	"testing"
)

func TestResolvePasskeyConfigDefaults(t *testing.T) {
	cfg, err := ResolvePasskeyConfig(PasskeyConfig{}, "https://api.ondex.uz")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RPID != "ondex.uz" {
		t.Fatalf("RP ID eTLD+1 emas: %q", cfg.RPID)
	}
	want := []string{"https://api.ondex.uz", "https://ondex.uz"}
	if !slices.Equal(cfg.Origins, want) {
		t.Fatalf("origins: %v", cfg.Origins)
	}
	if cfg, _ := ResolvePasskeyConfig(PasskeyConfig{}, "http://localhost:8080"); cfg.RPID != "localhost" ||
		!slices.Equal(cfg.Origins, []string{"http://localhost:8080"}) {
		t.Fatalf("localhost: %+v", cfg)
	}
}

func TestResolvePasskeyConfigStrict(t *testing.T) {
	sum := sha256.Sum256([]byte("imzo"))
	android := androidOriginPrefix + base64.RawURLEncoding.EncodeToString(sum[:])
	ok, err := ResolvePasskeyConfig(PasskeyConfig{RPID: "ondex.uz",
		Origins: []string{"https://passport.ondex.uz", "https://ondex.uz", android}}, "https://api.ondex.uz")
	if err != nil || !slices.Equal(ok.OpaqueOrigins, []string{android}) || len(ok.Origins) != 2 {
		t.Fatalf("to'g'ri sozlama rad etildi: %+v %v", ok, err)
	}
	bad := []struct {
		name string
		cfg  PasskeyConfig
	}{
		{"wildcard", PasskeyConfig{RPID: "ondex.uz", Origins: []string{"https://*.ondex.uz"}}},
		{"http", PasskeyConfig{RPID: "ondex.uz", Origins: []string{"http://passport.ondex.uz"}}},
		{"begona domen", PasskeyConfig{RPID: "ondex.uz", Origins: []string{"https://ondex.uz.evil.com"}}},
		{"o'xshash domen", PasskeyConfig{RPID: "ondex.uz", Origins: []string{"https://myondex.uz"}}},
		{"yo'lli origin", PasskeyConfig{RPID: "ondex.uz", Origins: []string{"https://ondex.uz/login"}}},
		{"buzuq android", PasskeyConfig{RPID: "ondex.uz", Origins: []string{"https://ondex.uz", androidOriginPrefix + "abc"}}},
		{"faqat android", PasskeyConfig{RPID: "ondex.uz", Origins: []string{android}}},
		{"issuer RP tashqarisida", PasskeyConfig{RPID: "boshqa.uz"}},
		{"TLD RP", PasskeyConfig{RPID: "uz", Origins: []string{"https://ondex.uz"}}},
	}
	for _, b := range bad {
		if _, err := ResolvePasskeyConfig(b.cfg, "https://api.ondex.uz"); err == nil {
			t.Errorf("XAVFSIZLIK: %s qabul qilindi", b.name)
		}
	}
}

// Prod 2026-11-19: WEBAUTHN_ORIGINS'da admin.ondex.uz yo'q edi — admin web'da
// passkey "Error validating origin". Web panel origin'lari har doim qo'shiladi,
// RP'dan tashqaridagisi (dev) esa passkey'ni O'CHIRMAYDI.
func TestResolvePasskeyConfigWebPanelOrigins(t *testing.T) {
	cfg, err := ResolvePasskeyConfig(PasskeyConfig{
		Origins:         []string{"https://passport.ondex.uz", "https://merchant.ondex.uz"},
		WebPanelOrigins: []string{"https://merchant.ondex.uz", "https://admin.ondex.uz", "https://evil.example.com", "http://admin.ondex.uz"},
	}, "https://passport.ondex.uz")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"https://passport.ondex.uz": true, "https://merchant.ondex.uz": true, "https://admin.ondex.uz": true}
	if len(cfg.Origins) != len(want) {
		t.Fatalf("origins = %v", cfg.Origins)
	}
	for _, o := range cfg.Origins {
		if !want[o] {
			t.Fatalf("kutilmagan origin %q (%v)", o, cfg.Origins)
		}
	}
	dev, err := ResolvePasskeyConfig(PasskeyConfig{WebPanelOrigins: []string{"https://admin.ondex.uz"}}, "http://localhost:8080")
	if err != nil || len(dev.Origins) != 1 || dev.Origins[0] != "http://localhost:8080" {
		t.Fatalf("dev: %v %v", dev.Origins, err)
	}
}
