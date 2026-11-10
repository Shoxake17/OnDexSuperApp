package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

// Admin IP cheklovi (CC-0.2). `Deps.AdminAllowedNets` bo'sh — cheklov yo'q
// (standart). Berilgan bo'lsa: admin kirishi (`/v2/auth/admin/*`), admin
// v2 API va eski admin marshrutlari (`role=admin` token) FAQAT shu
// tarmoqlardan. Mijoz IP'si `clientIP` dan (TRUSTED_PROXIES/CF orqali).

var errAdminIPDenied = errors.New("admin paneli bu tarmoqdan ochilmaydi")

// ParseAdminNets — "1.2.3.4/32, 10.0.0.0/8, 2001:db8::/48" (yakka IP ham
// mumkin). Noto'g'ri qiymat — XATO: noto'g'ri yozilgan cheklov jimgina
// "hammaga ochiq" bo'lib qolmasligi kerak (chaqiruvchi ishga tushishni to'xtatadi).
func ParseAdminNets(raw string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "/") {
			a, err := netip.ParseAddr(part)
			if err != nil {
				return nil, fmt.Errorf("ADMIN_ALLOWED_CIDRS: %q noto'g'ri", part)
			}
			out = append(out, netip.PrefixFrom(a, a.BitLen()))
			continue
		}
		p, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("ADMIN_ALLOWED_CIDRS: %q noto'g'ri", part)
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// adminIPAllowed — cheklov o'chiq bo'lsa doim true.
func (s *Server) adminIPAllowed(r *http.Request) bool {
	if len(s.AdminAllowedNets) == 0 {
		return true
	}
	a, err := netip.ParseAddr(clientIP(r))
	if err != nil {
		return false
	}
	a = a.Unmap()
	for _, p := range s.AdminAllowedNets {
		if p.Contains(a) {
			return true
		}
	}
	return false
}
