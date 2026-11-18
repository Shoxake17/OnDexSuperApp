package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ┌─ ADMIN WEB -> OnDexMap PROKSI (Plane 479) ────────────────────────────────┐
// OnDexMap admin serveri (maps.ondex.uz/admin) adminni uning OnDex tokeni
// bilan taniydi (`/v2/admin/whoami`). Windows panel tokenni o'zi yuboradi;
// web panelda esa token brauzerda YO'Q (BFF). Shuning uchun web panel
// OnDexMap'ni o'z domenidagi /a/api/x/ondexmap/* orqali chaqiradi va BFF
// cookie sessiyasidan olingan access tokenni SERVER ichida qo'shadi.
//
// Xavfsizlik:
//   - manzil qat'iy (ONDEXMAP_ADMIN_URL, standart https://maps.ondex.uz/admin);
//     yo'l tozalanadi, ".." va so'rov orqali boshqa hostga chiqib bo'lmaydi;
//   - faqat admin BFF ichida (IP ro'yxati, CSRF, cookie) — bu funksiyaga
//     yetib kelgan so'rov allaqachon tekshirilgan;
//   - tana va javob hajmi cheklangan; cookie va kelgan Authorization
//     uzatilmaydi; javobdan Set-Cookie olib tashlanadi.
// └───────────────────────────────────────────────────────────────────────────┘

const (
	adminOndexMapPrefix  = "/x/ondexmap/"
	adminOndexMapBodyMax = 1 << 20
	adminOndexMapRespMax = 8 << 20
)

var adminOndexMapClient = &http.Client{
	Timeout: 30 * time.Second,
	// Yo'naltirishga ergashilmaydi: token boshqa hostga ketmasin.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func (s *Server) ondexMapAdminBase() string {
	if s.OndexMapAdminURL != "" {
		return strings.TrimRight(s.OndexMapAdminURL, "/")
	}
	return "https://maps.ondex.uz/admin"
}

// proxyOndexMap — `path` "/x/ondexmap/..." bilan boshlanadi; `token` —
// cookie sessiyasidan chiqarilgan access token (bo'sh bo'lsa 401).
func (s *Server) proxyOndexMap(w http.ResponseWriter, r *http.Request, path, token string) {
	if token == "" {
		panelWebErr(w, http.StatusUnauthorized, "sessiya tugagan — qaytadan kiring")
		return
	}
	rest := strings.TrimPrefix(path, adminOndexMapPrefix)
	if rest == "" || strings.Contains(rest, "..") || strings.Contains(rest, "//") || strings.ContainsAny(rest, "\\@") {
		panelWebErr(w, http.StatusBadRequest, "noto'g'ri yo'l")
		return
	}
	base, err := url.Parse(s.ondexMapAdminBase())
	if err != nil || base.Scheme != "https" && !s.DevMode {
		panelWebErr(w, http.StatusServiceUnavailable, "OnDexMap manzili sozlanmagan")
		return
	}
	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + "/" + rest
	target.RawQuery = r.URL.RawQuery

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var body io.Reader
	if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
		body = io.LimitReader(r.Body, adminOndexMapBodyMax)
	}
	req, err := http.NewRequestWithContext(ctx, r.Method, target.String(), body)
	if err != nil {
		panelWebErr(w, http.StatusBadRequest, "noto'g'ri so'rov")
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	req.Header.Set("Accept", "application/json")
	res, err := adminOndexMapClient.Do(req)
	if err != nil {
		panelWebErr(w, http.StatusBadGateway, "OnDexMap serveriga ulanib bo'lmadi")
		return
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.WriteHeader(res.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(res.Body, adminOndexMapRespMax))
}
