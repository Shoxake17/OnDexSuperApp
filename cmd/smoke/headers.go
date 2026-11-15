package main

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Xavfsizlik sarlavhalari skaneri (Plane SEC-8 / ONDEXECOSY-454).
//
// AUD-6/7/8 da yopilgan sarlavhalar (HSTS, CSP, nosniff, framing) keyingi
// deploy yoki Caddy tahririda jimgina yo'qolmasin: har smoke'da tekshiriladi,
// yiqilsa Alertmanager -> Telegram. Qoida domen TURIGA qarab:
//
//	html   — sahifa: HSTS, CSP (frame-ancestors YOKI X-Frame-Options), nosniff, Referrer-Policy
//	api    — JSON: HSTS, nosniff, CSP default-src 'none' + framing taqiqlangan
//	static — CDN/plitkalar: HSTS, nosniff
type HeaderTarget struct {
	Name, URL, Kind string
}

// DefaultHeaderTargets — `-headers default` bilan tekshiriladigan ommaviy domenlar.
var DefaultHeaderTargets = []HeaderTarget{
	{"web", "https://ondex.uz/", "html"},
	{"api", "https://api.ondex.uz/restaurants", "api"},
	{"passport", "https://passport.ondex.uz/.well-known/openid-configuration", "api"},
	{"passport_login", "https://passport.ondex.uz/v2/oauth/login", "html"},
	{"merchant", "https://merchant.ondex.uz/", "html"},
	{"maps", "https://maps.ondex.uz/", "html"},
	{"maps_console", "https://console.ondex.uz/", "html"},
	{"plane", "https://plane.ondex.uz/", "html"},
	{"git", "https://git.ondex.uz/users/sign_in", "html"},
	{"cdn", "https://cdn.ondex.uz/", "static"},
	{"tiles", "https://tiles.ondex.uz/", "static"},
}

// ParseHeaderTargets — "default" yoki "nom=URL:tur,nom2=URL2:tur2".
func ParseHeaderTargets(s string) ([]HeaderTarget, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if s == "default" {
		return DefaultHeaderTargets, nil
	}
	var out []HeaderTarget
	for _, part := range strings.Split(s, ",") {
		name, rest, ok := strings.Cut(strings.TrimSpace(part), "=")
		i := strings.LastIndex(rest, ":")
		if !ok || name == "" || i <= len("https") {
			return nil, fmt.Errorf("noto'g'ri -headers bo'lagi %q (kutilgan nom=URL:tur)", part)
		}
		t := HeaderTarget{Name: name, URL: rest[:i], Kind: rest[i+1:]}
		if t.Kind != "html" && t.Kind != "api" && t.Kind != "static" {
			return nil, fmt.Errorf("%s: noma'lum tur %q", name, t.Kind)
		}
		out = append(out, t)
	}
	return out, nil
}

// minHSTS — 180 kun (Mozilla Observatory minimal talabi).
const minHSTS = 15552000

var maxAgeRe = regexp.MustCompile(`(?i)max-age\s*=\s*"?(\d+)`)

// CheckSecurityHeaders — sof funksiya (test qilinadi): yetishmayotgan yoki
// zaif sarlavhalarni bitta xato matnida qaytaradi.
func CheckSecurityHeaders(h http.Header, kind string) error {
	var miss []string
	if m := maxAgeRe.FindStringSubmatch(h.Get("Strict-Transport-Security")); m == nil {
		miss = append(miss, "HSTS yo'q")
	} else if n, _ := strconv.Atoi(m[1]); n < minHSTS {
		miss = append(miss, fmt.Sprintf("HSTS max-age=%d < %d", n, minHSTS))
	}
	if !strings.EqualFold(strings.TrimSpace(h.Get("X-Content-Type-Options")), "nosniff") {
		miss = append(miss, "nosniff yo'q")
	}
	csp := strings.ToLower(h.Get("Content-Security-Policy"))
	xfo := strings.ToUpper(strings.TrimSpace(h.Get("X-Frame-Options")))
	switch kind {
	case "html":
		if csp == "" {
			miss = append(miss, "CSP yo'q")
		}
		if !strings.Contains(csp, "frame-ancestors") && xfo != "DENY" && xfo != "SAMEORIGIN" {
			miss = append(miss, "framing himoyasi yo'q (frame-ancestors/X-Frame-Options)")
		}
		if h.Get("Referrer-Policy") == "" {
			miss = append(miss, "Referrer-Policy yo'q")
		}
	case "api":
		if !strings.Contains(csp, "default-src 'none'") {
			miss = append(miss, "CSP default-src 'none' yo'q")
		}
		if !strings.Contains(csp, "frame-ancestors 'none'") && xfo != "DENY" {
			miss = append(miss, "framing taqiqlanmagan")
		}
	}
	if len(miss) > 0 {
		return permanent{fmt.Errorf("%s", strings.Join(miss, "; "))}
	}
	return nil
}

// checkHeaders — bitta domen; redirect kuzatilmaydi (sarlavha aynan shu javobda bo'lsin).
// 5xx — xizmat o'zi yiqilgan: sarlavhadan oldin shu aytiladi.
func (r *runner) checkHeaders(t HeaderTarget) {
	r.check("hdr."+t.Name, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), r.cfg.Timeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
		if err != nil {
			return permanent{err}
		}
		req.Header.Set("User-Agent", "ondex-smoke/1")
		client := *r.http
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("tarmoq: %w", err)
		}
		res.Body.Close()
		if res.StatusCode >= 500 {
			return fmt.Errorf("HTTP %d - xizmat javob bermayapti", res.StatusCode)
		}
		return CheckSecurityHeaders(res.Header, t.Kind)
	})
}
