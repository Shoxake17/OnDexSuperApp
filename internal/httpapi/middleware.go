// HTTP middleware qatlami — Express'dagi `app.use(...)` ga mos.
//
// Tartib (tashqaridan ichkariga):
//
//	withBodyLimit -> withCORS -> mux -> auth -> rate limit -> handler
//
// INVARIANT: token qabul qiladigan HAR BIR kirish nuqtasi `Server.auth`
// dan o'tadi. Hech bir handler token parsingini o'zi yozmaydi — aks
// holda bekor qilish (revocation) tekshiruvi tushib qolishi mumkin
// (aynan shu xato `GET /ws` da bo'lgan).
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"slices"
	"strings"
	"time"

	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// ---- Tezlik cheklovchilari (suiiste'molga qarshi) ----
//
// Bularsiz PULLIK tashqi API'lar (Google/Yandex/2GIS geokodlash, Eskiz
// SMS) cheksiz chaqirilishi mumkin edi — bitta oddiy akkaunt bilan
// hisobni ko'tarish (moliyaviy DoS).
var (
	// Geokodlash: har akkauntga daqiqasiga ~30 (portlash 30) —
	// manzil tanlashda xarita surilganda bir necha so'rov normal,
	// lekin sikl bilan minglab chaqirish to'xtatiladi.
	geoUserLimiter = ratelimit.New(0.5, 30)
	// Qo'shimcha IP cheklovi — bitta hujumchi ko'p akkaunt yaratsa ham.
	geoIPLimiter = ratelimit.New(1, 60)

	// OTP/SMS: IP bo'yicha soatiga ~12 (portlash 5), global esa
	// daqiqasiga ~30 — bitta skript butun SMS byudjetini yoqa olmaydi.
	otpIPLimiter     = ratelimit.New(0.0033, 5)
	otpGlobalLimiter = ratelimit.New(0.5, 30)

	// Kuryer joylashuvi: sekundiga ~2 (portlash 10). Ilova odatda
	// 5-10 sekundda bir marta yuboradi, shuning uchun bu chegara halol
	// foydalanuvchiga umuman sezilmaydi, lekin sikl bilan yuborilgan
	// minglab yangilanish (har biri WS broadcast keltirib chiqaradi)
	// to'xtatiladi.
	courierLocLimiter = ratelimit.New(2, 10)

	// Parol bilan kirish — brute-force va credential-stuffing ning
	// asosiy nishoni. IKKI o'lchov bo'yicha cheklanadi:
	//
	//   loginIPLimiter      — bitta manbadan ommaviy urinish;
	//   loginAccountLimiter — ko'p IP'dan BITTA akkauntga hujum.
	//     Faqat IP bo'yicha cheklash yetarli emas: botnet har so'rovni
	//     boshqa IP'dan yuboradi.
	//
	// CHEGARALAR HALOL FOYDALANUVCHIGA MOSLANGAN (tuzatilgan xato):
	// avval akkaunt bo'yicha 5 ta portlash va 200 soniyada bittadan
	// tiklanish edi — ya'ni parolini ikki-uch marta xato yozgan oddiy
	// odam ~17 daqiqaga bloklanardi va "juda ko'p urinish" xatosini
	// olardi. Bu xavfsizlik emas, buzilgan mahsulot.
	//
	// Hozirgi qiymatlar: akkauntga 10 ta ketma-ket urinish, keyin
	// 20 soniyada bittadan (soatiga ~180). Bu brute-force'ni amalda
	// imkonsiz qiladi (Argon2id + 8 belgili minimal parol + ommaviy
	// parollar ro'yxati bilan birga), lekin halol foydalanuvchini
	// hech qachon bloklamaydi.
	loginIPLimiter      = ratelimit.New(0.5, 30)
	loginAccountLimiter = ratelimit.New(0.05, 10)

	// SMS kodni tasdiqlash. Kodning o'zida per-telefon 5 urinish
	// chegarasi bor, lekin uni QURBONGA QARSHI ishlatish mumkin edi:
	// begona raqamga 6 ta soxta kod yuborilsa, chegara tugab, o'sha
	// raqamning HAQIQIY kodi o'chib ketardi. Portlash 20 — halol
	// foydalanuvchi (bir necha marta xato terish + qayta yuborish)
	// bunga hech qachon yetmaydi.
	verifyIPLimiter = ratelimit.New(0.2, 20)

	// Email kodlari. SMS'dan ARZONROQ (pul ketmaydi), shuning uchun
	// chegara biroz yumshoqroq — lekin baribir qattiq: cheklovsiz
	// email yuborish domenimizni spam ro'yxatiga tushiradi va bu
	// tiklab bo'lmaydigan zarar. IP bo'yicha soatiga ~36 (portlash
	// 10), global daqiqasiga ~60.
	emailIPLimiter     = ratelimit.New(0.01, 10)
	emailGlobalLimiter = ratelimit.New(1, 60)

	// Telegram deep-link so'rovi. Pul ketmaydi (xabarni Telegram
	// bepul yetkazadi), lekin cheklovsiz qoldirilsa bot begona
	// odamlarga so'rov yuborish vositasiga aylanardi. Portlash 10,
	// tiklanish daqiqasiga ~6.
	telegramIPLimiter = ratelimit.New(0.1, 10)

	// Telegram kirish holatini so'rab turish (polling). Ilova har
	// 2 soniyada so'raydi va ~2 daqiqa kutadi, ya'ni bitta oqimda
	// ~60 so'rov. Chegara shundan kelib chiqib qo'yilgan: halol
	// foydalanuvchi yetib bormaydi, sikl bilan token qidirish esa
	// to'xtatiladi.
	telegramPollLimiter = ratelimit.New(2, 120)
)

// courierSpeedGate — kuryer koordinatasining "teleport" qilishini
// aniqlaydi (GPS soxtalashtirishga qarshi). Qarang: internal/delivery.

// trustedProxies — `X-Forwarded-For` sarlavhasiga ISHONISH mumkin
// bo'lgan manbalar (`.env` dagi `TRUSTED_PROXIES`, vergul bilan;
// CIDR yoki yakka IP). Bo'sh bo'lsa sarlavha UMUMAN o'qilmaydi.
var trustedProxies []netip.Prefix

// SetTrustedProxies — ishga tushishda `main` tomonidan chaqiriladi.
// Noto'g'ri yozilgan qiymat JIM o'tkazib yuborilmaydi (ogohlantirish
// log qilinadi), aks holda konfiguratsiyadagi bitta harf xatosi
// himoyani bildirmasdan o'chirib qo'yardi.
func SetTrustedProxies(list []string) {
	trustedProxies = nil
	for _, raw := range list {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if p, err := netip.ParsePrefix(raw); err == nil {
			trustedProxies = append(trustedProxies, p)
			continue
		}
		if a, err := netip.ParseAddr(raw); err == nil {
			trustedProxies = append(trustedProxies, netip.PrefixFrom(a, a.BitLen()))
			continue
		}
		slog.Warn("TRUSTED_PROXIES: qiymat tushunarsiz, e'tiborga olinmadi", "value", raw)
	}
	if len(trustedProxies) > 0 {
		slog.Info("ishonchli proksilar sozlandi", "count", len(trustedProxies))
	}
}

// clientIP — tezlik cheklovi uchun mijoz manzili.
//
// XAVFSIZLIK: `X-Forwarded-For` FAQAT ishonchli proksidan kelgan
// so'rovda o'qiladi. Avval u SHARTSIZ ishonilardi va bu barcha
// IP-asosidagi cheklovlarni bitta sarlavha bilan yo'q qilardi
// (jonli o'lchov: soxta XFF bilan 60/60 so'rov o'tdi, XFF'siz
// 32-so'rovda 429). Ochilib qoladigan narsalar:
//
//   - `otpIPLimiter`   -> cheksiz SMS (Eskiz.uz — haqiqiy pul);
//   - `loginIPLimiter` -> cheksiz brute-force;
//   - `geoIPLimiter`   -> cheksiz pullik geokodlash.
//
// Standart holat — sarlavhaga umuman ishonmaslik. Proksi/reverse-proxy
// (Caddy/nginx) orqasiga qo'yilganda `.env` da `TRUSTED_PROXIES`
// sozlanadi, aks holda hamma so'rov bitta (proksi) IP'dan kelgandek
// ko'rinib, cheklov barcha foydalanuvchini birga bloklab qo'yardi.
func clientIP(r *http.Request) string {
	direct := remoteHost(r)
	if len(trustedProxies) == 0 {
		return direct
	}
	addr, err := netip.ParseAddr(direct)
	if err != nil || !slices.ContainsFunc(trustedProxies,
		func(p netip.Prefix) bool { return p.Contains(addr) }) {
		return direct
	}
	// Ishonchli proksi: zanjirning BIRINCHI qiymati — asl mijoz.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := xff
		if i := strings.IndexByte(xff, ','); i > 0 {
			first = xff[:i]
		}
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}
	return direct
}

func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// allowBoth — ikkita cheklovni ketma-ket tekshiradi va rad etilganda
// tegishli kutish vaqtini qaytaradi.
//
// Birinchisi rad etsa ikkinchisi UMUMAN tekshirilmaydi (token bekorga
// sarflanmasin) — bu `!a.Allow() || !b.Allow()` ning xatti-harakati
// bilan bir xil.
func allowBoth(a *ratelimit.Limiter, ka string, b *ratelimit.Limiter, kb string) (bool, time.Duration) {
	if ok, wait := a.AllowWithWait(ka); !ok {
		return false, wait
	}
	if ok, wait := b.AllowWithWait(kb); !ok {
		return false, wait
	}
	return true, 0
}

// rateLimitedGeo — pullik geokodlash endpointlari uchun o'ram:
// akkaunt VA IP bo'yicha cheklaydi.
func rateLimitedGeo(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := clientIP(r)
		if c := claimsFrom(r); c != nil && c.Subject != "" {
			key = c.Subject
		}
		if ok, wait := allowBoth(geoUserLimiter, key, geoIPLimiter, clientIP(r)); !ok {
			tooManyRequests(w, wait)
			return
		}
		next(w, r)
	}
}

// Redis kesh kalitlari — GET /restaurants va GET /restaurants/{id}/menu
// uchun (eng ko'p so'raladigan, kam o'zgaradigan endpointlar). Har bir
// SaveRestaurant/DeleteRestaurant/SaveProduct/DeleteProduct chaqiruvidan
// keyin tegishli kalit(lar) darhol tozalanadi — TTL faqat zaxira sifatida.

// withBodyLimit — HAR QANDAY so'rov tanasi uchun umumiy yuqori chegara.
//
// Avval faqat `/uploads` cheklangan edi; qolgan handler'lar
// `json.NewDecoder(r.Body)`ni chegarasiz o'qirdi. Ya'ni autentifikatsiya
// TALAB QILMAYDIGAN `/auth/request-code`ga yuz megabaytlik tana yuborib,
// serverni xotira buferlashga majburlash mumkin edi.
//
// `/uploads` ISTISNO — u o'z (kattaroq, 5MB+) chegarasini qo'yadi.
func withBodyLimit(next http.Handler) http.Handler {
	const maxJSONBody = 1 << 20 // 1 MB — JSON so'rovlar uchun yetarlicha katta
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && !strings.HasPrefix(r.URL.Path, "/uploads") {
			r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
		}
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler, allowedOrigins []string, devMode bool) http.Handler {
	allowedSet := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o != "" {
			allowedSet[o] = struct{}{}
		}
	}
	allowAll := len(allowedSet) == 0
	if allowAll {
		// Fail-closed: production'da sozlanmagan CORS — ishga tushirishni
		// TO'XTATADI (JWT_SECRET bilan bir xil qoida). Avval bu faqat
		// ogohlantirish edi va prod'da ham istalgan origin aks etardi.
		if !devMode {
			slog.Error("cors: production rejimda ALLOWED_ORIGINS majburiy")
			os.Exit(1)
		}
		slog.Warn("cors: ALLOWED_ORIGINS sozlanmagan — istalgan brauzer origin'idan REST so'rovga ruxsat berilyapti (FAQAT dev)")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecurityHeaders(w)
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Vary", "Origin")
			_, ok := allowedSet[origin]
			if allowAll || ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		// `X-Ondex-Client` — mijoz dasturini bildiruvchi sarlavha
		// (`devices.go`). Bu ro'yxatda BO'LMASA brauzerdagi panellar
		// va mini-app umuman so'rov yubora olmaydi: preflight (OPTIONS)
		// javobida ruxsat etilmagan sarlavha butun so'rovni bloklaydi.
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, "+clientHeader)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeSecurityHeaders — brauzer tomonidagi standart himoyalar.
//
// API asosan JSON qaytaradi va uni Flutter ilovasi o'qiydi, LEKIN
// javoblar brauzerga ham yetib boradi (admin/restoran panellari —
// Flutter Web, mini-app'lar — WebView). Shu sabab bu sarlavhalar
// kerak:
//
//	X-Content-Type-Options: nosniff
//	  Brauzer javob turini "taxmin qilishi" (MIME sniffing) mumkin.
//	  Javob ichida foydalanuvchi kiritgan matn bo'lsa, uni HTML deb
//	  talqin qilib, XSS'ga aylantirib yuborishi mumkin edi.
//
//	X-Frame-Options: DENY
//	  Sahifani begona saytga <iframe> ichida joylashtirib bo'lmaydi
//	  (clickjacking).
//
//	Referrer-Policy: no-referrer
//	  Boshqa saytga o'tilganda URL (ichida token/ID bo'lishi mumkin)
//	  uzatilmaydi.
//
//	Content-Security-Policy
//	  JSON javob uchun eng qattiq siyosat: hech qanday resurs
//	  yuklanmaydi va skript bajarilmaydi. Bu MIME sniffing qolib
//	  ketgan holatda ham XSS'ni o'ldiradi (ikkinchi qatlam).
//	  DIQQAT: bu API javoblariga tegishli; mini-app'larning O'Z
//	  sahifalari Next.js tomonidan beriladi va o'z siyosatiga ega.
func writeSecurityHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Content-Security-Policy",
		"default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
}

// ---------- Auth middleware ----------

type ctxKey int

const claimsKey ctxKey = 0

// auth — Bearer tokenni tekshiradi; roles bo'sh bo'lmasa rol ham talab
// qilinadi.
//
// `Server` metodi sifatida: token chiqaruvchi va bekor qilish do'koni
// endi parametr emas, bog'liqlik. Shu sababli chaqiruv joyi qisqaradi
// (`s.auth(roles, fn)`) va — muhimrog'i — hech bir handler bu
// tekshiruvni CHETLAB o'tolmaydi, chunki boshqa yo'l yo'q.
func (s *Server) auth(roles []users.Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			httpError(w, http.StatusUnauthorized, errors.New("Authorization: Bearer <token> talab qilinadi"))
			return
		}
		claims, err := s.Tokens.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			httpError(w, http.StatusUnauthorized, err)
			return
		}
		// Muddatidan oldin bekor qilingan sessiya (chiqish, akkaunt
		// o'chirilishi, kuryer tasdig'ining olib tashlanishi). Bu
		// tekshiruv RAM'dagi map orqali — DB/tarmoq murojaati yo'q.
		if s.Revoked != nil && claims.IssuedAt != nil &&
			s.Revoked.IsRevoked(claims.Subject, claims.IssuedAt.Time) {
			httpError(w, http.StatusUnauthorized, errors.New("sessiya bekor qilingan — qaytadan kiring"))
			return
		}
		if len(roles) > 0 && !slices.Contains(roles, claims.Role) {
			httpError(w, http.StatusForbidden, errors.New("bu amal sizning rolingizga ochiq emas"))
			return
		}
		s.recordDevice(r, claims.Subject)
		next(w, r.WithContext(context.WithValue(r.Context(), claimsKey, claims)))
	}
}

// claimsFrom — `auth` middleware kontekstga qo'ygan claims'ni qaytaradi.
//
// COMMA-OK bilan (panic'siz): `auth` bilan O'RALMAGAN handler — masalan
// kelajakda qo'shiladigan ochiq (public) route — buni chaqirsa, xom
// `.(*users.Claims)` assertioni nil interface'da PANIC berardi (500/DoS).
// Endi bunday holatda nil qaytadi. `rateLimitedGeo` allaqachon
// `if c := claimsFrom(r); c != nil` deb tekshiradi, ya'ni nil qaytishi
// kutilgan xatti-harakat; `auth` ostidagi handlerlar uchun esa qiymat
// har doim mavjud, shuning uchun ular o'zgarishsiz ishlaydi.
func claimsFrom(r *http.Request) *users.Claims {
	c, _ := r.Context().Value(claimsKey).(*users.Claims)
	return c
}
