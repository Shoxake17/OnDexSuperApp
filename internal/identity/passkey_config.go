package identity

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// ┌─ PASSKEY RP (PK-1, Plane 318) ─────────────────────────────────────────┐
// RP ID — passkey qaysi domenga bog'langani. U O'ZGARSA barcha mavjud
// passkey'lar ishlamay qoladi, shuning uchun bir marta to'g'ri tanlanadi:
// asosiy domen (eTLD+1, masalan "ondex.uz"). Shunda bitta passkey passport,
// merchant sayti, Windows va Android/iOS ilovalarida ishlaydi.
//
// Origin'lar — passkey imzosi qabul qilinadigan ANIQ manzillar:
//   - https://<rp-ichidagi host>  (brauzer sahifalari; Windows ilovasi
//     `passkeys_windows` origin'ni "https://"+RPID qilib yuboradi);
//   - android:apk-key-hash:<base64url SHA-256>  (Android ilova imzosi; OS
//     faqat assetlinks.json bilan bog'langan ilovaga beradi).
// Wildcard, http (localhost'dan tashqari) va RP'dan tashqari host — RAD.
// └────────────────────────────────────────────────────────────────────────┘

const androidOriginPrefix = "android:apk-key-hash:"

// ResolvePasskeyConfig — sozlamani issuer'dan to'ldiradi va qat'iy tekshiradi.
func ResolvePasskeyConfig(cfg PasskeyConfig, issuer string) (PasskeyConfig, error) {
	iu, err := url.Parse(issuer)
	if err != nil || iu.Hostname() == "" {
		return cfg, errors.New("passkey: issuer manzilidan RP aniqlanmadi")
	}
	out := PasskeyConfig{RPID: strings.ToLower(strings.TrimSpace(cfg.RPID))}
	if out.RPID == "" {
		out.RPID = registrableDomain(iu.Hostname())
	}
	if !isLocalHost(out.RPID) && net.ParseIP(out.RPID) == nil {
		// RP ID public suffix (`uz`, `co.uk`) bo'lsa — boshqa saytlar ham shu
		// RP ostida bo'lardi.
		if ps, _ := publicsuffix.PublicSuffix(out.RPID); ps == out.RPID {
			return cfg, fmt.Errorf("passkey: RP ID public suffix bo'lishi mumkin emas: %q", out.RPID)
		}
	}
	if !hostWithin(strings.ToLower(iu.Hostname()), out.RPID) {
		return cfg, fmt.Errorf("passkey: issuer %q RP ID %q ichida emas", iu.Hostname(), out.RPID)
	}
	raw := cfg.Origins
	if len(raw) == 0 {
		raw = []string{iu.Scheme + "://" + iu.Host}
		if !isLocalHost(out.RPID) && strings.ToLower(iu.Hostname()) != out.RPID {
			raw = append(raw, "https://"+out.RPID) // Windows ilovasi (passkeys_windows)
		}
	}
	// Android origin'lari (PASSKEY_ANDROID_APPS'dan) — o'sha tekshiruvdan o'tadi.
	raw = append(append([]string(nil), raw...), cfg.OpaqueOrigins...)
	for _, a := range cfg.AndroidApps {
		raw = append(raw, a.Origin())
	}
	out.AndroidApps, out.AppleAppIDs = cfg.AndroidApps, cfg.AppleAppIDs
	seen := map[string]bool{}
	for _, o := range raw {
		o = strings.TrimSpace(o)
		if o == "" || seen[o] {
			continue
		}
		seen[o] = true
		if strings.Contains(o, "*") {
			return cfg, fmt.Errorf("passkey: origin'da wildcard taqiqlangan: %q", o)
		}
		if strings.HasPrefix(o, androidOriginPrefix) {
			h, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(o, androidOriginPrefix))
			if err != nil || len(h) != 32 {
				return cfg, fmt.Errorf("passkey: Android origin SHA-256 (base64url, 32 bayt) emas: %q", o)
			}
			out.OpaqueOrigins = append(out.OpaqueOrigins, o)
			continue
		}
		u, err := url.Parse(o)
		if err != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.User != nil {
			return cfg, fmt.Errorf("passkey: origin noto'g'ri (faqat sxema://host[:port]): %q", o)
		}
		host := strings.ToLower(u.Hostname())
		switch {
		case u.Scheme == "https":
		case u.Scheme == "http" && isLocalHost(host):
		default:
			return cfg, fmt.Errorf("passkey: origin HTTPS bo'lishi shart: %q", o)
		}
		if !hostWithin(host, out.RPID) {
			return cfg, fmt.Errorf("passkey: origin %q RP ID %q ichida emas", o, out.RPID)
		}
		out.Origins = append(out.Origins, u.Scheme+"://"+strings.ToLower(u.Host))
	}
	if len(out.Origins) == 0 {
		return cfg, errors.New("passkey: hech bo'lmasa bitta HTTPS origin kerak")
	}
	return out, nil
}

// registrableDomain — eTLD+1 ("api.ondex.uz" -> "ondex.uz"); IP/localhost — o'zi.
func registrableDomain(host string) string {
	host = strings.ToLower(host)
	if isLocalHost(host) || net.ParseIP(host) != nil {
		return host
	}
	if d, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		return d
	}
	return host
}

func hostWithin(host, rpID string) bool {
	return host == rpID || strings.HasSuffix(host, "."+rpID)
}

func isLocalHost(h string) bool {
	return h == "localhost" || strings.HasSuffix(h, ".localhost") || h == "127.0.0.1" || h == "::1"
}
