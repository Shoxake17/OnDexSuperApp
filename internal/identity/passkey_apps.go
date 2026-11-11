package identity

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ┌─ NATIVE ILOVALAR VA DOMEN BOG'LANISHI (PK-1) ─────────────────────────┐
// Android/iOS passkey'ni ilovaga FAQAT RP domeni ilovani rasman tan olsa
// beradi (soxta ilova passkey'ni ololmaydi):
//   - Android: https://<RP>/.well-known/assetlinks.json —
//     delegate_permission/common.get_login_creds + imzo SHA-256;
//   - iOS:     https://<RP>/.well-known/apple-app-site-association —
//     webcredentials.apps = ["TEAMID.bundle.id"].
// Android imzosi bitta sozlamadan (PASSKEY_ANDROID_APPS) IKKI joyga ketadi:
// assetlinks.json va server qabul qiladigan origin (android:apk-key-hash:).
// Ikki joyda alohida yozilsa ular ajralib qolishi mumkin edi.
// └────────────────────────────────────────────────────────────────────────┘

// AndroidApp — paket + imzo sertifikati SHA-256 (32 bayt).
type AndroidApp struct {
	Package     string
	Fingerprint []byte
}

var (
	androidPkgRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(\.[a-zA-Z][a-zA-Z0-9_]*)+$`)
	appleAppIDRe = regexp.MustCompile(`^[A-Z0-9]{10}\.[a-zA-Z0-9-]+(\.[a-zA-Z0-9-]+)+$`)
)

// ParseAndroidApps — "com.ondex.waiter=AA:BB:...,com.ondex.waiter=CC:..."
// (release, Play imzo kaliti, upload kaliti — bir paketga bir nechta).
func ParseAndroidApps(raw string) ([]AndroidApp, error) {
	var out []AndroidApp
	seen := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		pkg, fp, ok := strings.Cut(item, "=")
		pkg = strings.TrimSpace(pkg)
		if !ok || !androidPkgRe.MatchString(pkg) {
			return nil, fmt.Errorf("PASSKEY_ANDROID_APPS: %q — 'paket=SHA256' bo'lishi kerak", item)
		}
		b, err := hex.DecodeString(strings.ReplaceAll(strings.TrimSpace(fp), ":", ""))
		if err != nil || len(b) != 32 {
			return nil, fmt.Errorf("PASSKEY_ANDROID_APPS: %q imzosi SHA-256 (32 bayt hex) emas", pkg)
		}
		key := pkg + "=" + hex.EncodeToString(b)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, AndroidApp{Package: pkg, Fingerprint: b})
	}
	return out, nil
}

// Origin — Android ilovasi passkey javobida yuboradigan origin.
func (a AndroidApp) Origin() string {
	return androidOriginPrefix + base64.RawURLEncoding.EncodeToString(a.Fingerprint)
}

func (a AndroidApp) colonHex() string {
	h := strings.ToUpper(hex.EncodeToString(a.Fingerprint))
	parts := make([]string, 0, len(h)/2)
	for i := 0; i < len(h); i += 2 {
		parts = append(parts, h[i:i+2])
	}
	return strings.Join(parts, ":")
}

// AssetLinksJSON — Digital Asset Links. Faqat passkey/parol ulashish
// (get_login_creds); handle_all_urls ATAYLAB yo'q (havolalar ilovaga
// o'g'irlanmasin).
func AssetLinksJSON(apps []AndroidApp) ([]byte, error) {
	byPkg := map[string][]string{}
	for _, a := range apps {
		byPkg[a.Package] = append(byPkg[a.Package], a.colonHex())
	}
	pkgs := make([]string, 0, len(byPkg))
	for p := range byPkg {
		pkgs = append(pkgs, p)
	}
	sort.Strings(pkgs)
	type target struct {
		Namespace    string   `json:"namespace"`
		PackageName  string   `json:"package_name"`
		Fingerprints []string `json:"sha256_cert_fingerprints"`
	}
	type stmt struct {
		Relation []string `json:"relation"`
		Target   target   `json:"target"`
	}
	out := make([]stmt, 0, len(pkgs))
	for _, p := range pkgs {
		out = append(out, stmt{Relation: []string{"delegate_permission/common.get_login_creds"},
			Target: target{Namespace: "android_app", PackageName: p, Fingerprints: byPkg[p]}})
	}
	return json.Marshal(out)
}

// ParseAppleAppIDs — "TEAMID1234.com.ondex.waiter,...".
func ParseAppleAppIDs(raw string) ([]string, error) {
	var out []string
	for _, id := range strings.Split(raw, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if !appleAppIDRe.MatchString(id) {
			return nil, fmt.Errorf("PASSKEY_APPLE_APP_IDS: %q — 'TEAMID.bundle.id' bo'lishi kerak", id)
		}
		out = append(out, id)
	}
	return out, nil
}

// AppleAppSiteAssociationJSON — faqat webcredentials (passkey).
func AppleAppSiteAssociationJSON(ids []string) ([]byte, error) {
	return json.Marshal(map[string]any{"webcredentials": map[string]any{"apps": ids}})
}

// PasskeyAssetLinks — /.well-known/assetlinks.json (ok=false — ilova yo'q).
func (m *MerchantAuth) PasskeyAssetLinks() ([]byte, bool) {
	if m.wa == nil || len(m.pkCfg.AndroidApps) == 0 {
		return nil, false
	}
	b, err := AssetLinksJSON(m.pkCfg.AndroidApps)
	return b, err == nil
}

// PasskeyAppleAssociation — /.well-known/apple-app-site-association.
func (m *MerchantAuth) PasskeyAppleAssociation() ([]byte, bool) {
	if m.wa == nil || len(m.pkCfg.AppleAppIDs) == 0 {
		return nil, false
	}
	b, err := AppleAppSiteAssociationJSON(m.pkCfg.AppleAppIDs)
	return b, err == nil
}
