package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// `/map-picker` — Windows desktop paneli xaritani WebView2 ichida shu
// sahifa orqali ko'rsatadi (`google_maps_flutter` Windows'da ishlamaydi).
//
// Sahifa BUZILSA desktop xaritasi jimgina bo'sh qoladi — nosozlikni
// qurilmada topish qiyin. Shuning uchun eng muhim ikki xossa shu yerda
// qulflanadi: sahifa umuman berilishi va ichida KALIT BO'LMASLIGI.

func mapPickerTestServer(t *testing.T) http.Handler {
	t.Helper()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	s := New(Deps{
		UserRepo: userRepo,
		Tokens:   tokens,
		AuthSvc: users.NewService(userRepo, storage.NewMemoryCodeStore(),
			noopSms{}, tokens, NewID),
		DevMode: true,
	})
	return s.Routes(nil)
}

func TestMapPickerPageServed(t *testing.T) {
	h := mapPickerTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/map-picker", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("kutilgan 200, olindi %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type text/html bo'lishi kerak, olindi %q", ct)
	}

	body := rec.Body.String()
	// Sahifa haqiqatan xarita sahifasi ekanini tasdiqlaymiz — bo'sh
	// yoki noto'g'ri fayl embed qilinib qolmasin.
	for _, want := range []string{
		"maps.googleapis.com", // Maps JS aynan shu yerdan yuklanadi
		"/config/maps",        // kalit backenddan olinadi
		"chrome.webview",      // host bilan aloqa kanali
		// Sozlama sahifa skriptlaridan OLDIN injektsiya qilinadi.
		// Avval u `postWebMessage` bilan kelardi va qo'l berish
		// uzilganda sahifa jimgina "Xarita yuklanmoqda..." holatida
		// qotib qolardi.
		"__ondexConfig",
		// Har qanday to'xtash 5 soniyadan keyin ANIQ sabab bilan
		// ko'rsatiladi — jimgina qotib qolish qaytarilmasin.
		"Sozlama kelmadi",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sahifada %q topilmadi", want)
		}
	}
}

// ┌─ JONLI NOSOZLIK QAYTMASIN ─────────────────────────────────────────┐
// Global `writeSecurityHeaders` barcha javoblarga `default-src 'none'`
// qo'yadi. U `script-src` ni ham qamrab oladi, ya'ni bu sahifaning
// inline skripti BLOKLANARDI: xarita "Xarita yuklanmoqda…" da abadiy
// qotib qolardi va qo'yilgan diagnostikalarning HECH BIRI ko'rinmasdi
// (ular ham o'sha bloklangan skript ichida edi).
//
// Shuning uchun bu yerda uchta narsa qulflanadi: sahifa nonce oladi,
// nonce CSP bilan MOS keladi va u har so'rovda YANGI bo'ladi.
// └────────────────────────────────────────────────────────────────────┘
func TestMapPickerScriptAllowedByCSP(t *testing.T) {
	h := mapPickerTestServer(t)

	get := func() (csp, body string) {
		req := httptest.NewRequest(http.MethodGet, "/map-picker", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Header().Get("Content-Security-Policy"), rec.Body.String()
	}

	csp, body := get()

	if strings.Contains(body, cspNoncePlaceholder) {
		t.Fatalf("%s almashtirilmagan — CSP skriptni bloklaydi", cspNoncePlaceholder)
	}

	// Sahifadagi nonce'ni ajratib olamiz va CSP bilan solishtiramiz.
	const marker = `<script nonce="`
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatal("sahifada nonce'li <script> yo'q")
	}
	rest := body[i+len(marker):]
	j := strings.IndexByte(rest, '"')
	if j <= 0 {
		t.Fatal("nonce atributi yopilmagan")
	}
	nonce := rest[:j]

	if !strings.Contains(csp, "'nonce-"+nonce+"'") {
		t.Fatalf("CSP sahifadagi nonce'ga mos kelmadi.\nnonce: %q\nCSP: %s", nonce, csp)
	}

	// `'unsafe-inline'` skriptlar uchun ishlatilmasin — u nonce
	// himoyasini butunlay ma'nosiz qilardi.
	scriptSrc := csp[strings.Index(csp, "script-src"):]
	if end := strings.IndexByte(scriptSrc, ';'); end > 0 {
		scriptSrc = scriptSrc[:end]
	}
	if strings.Contains(scriptSrc, "'unsafe-inline'") {
		t.Errorf("script-src da 'unsafe-inline' bor — nonce ma'nosiz bo'lib qoladi: %s", scriptSrc)
	}

	// Nonce har so'rovda yangi bo'lishi SHART.
	_, body2 := get()
	if strings.Contains(body2, `<script nonce="`+nonce+`"`) {
		t.Error("nonce qayta ishlatildi — u har so'rovda tasodifiy bo'lishi kerak")
	}
}

// ┌─ ENG MUHIM TEKSHIRUV ──────────────────────────────────────────────┐
// Sahifa AUTH TALAB QILMAYDI (WebView navigatsiyasiga `Authorization`
// sarlavhasini qo'shib bo'lmaydi). Bu faqat sahifada SIR BO'LMAGANDA
// xavfsiz. Kimdir kalitni "qulaylik uchun" HTML ga yozib qo'ysa,
// u ochiq internetga chiqib ketardi.
// └────────────────────────────────────────────────────────────────────┘
func TestMapPickerPageHasNoSecret(t *testing.T) {
	t.Setenv("GOOGLE_MAPS_API_KEY", "AIzaTEST_MAXFIY_KALIT")

	h := mapPickerTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/map-picker", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "AIzaTEST_MAXFIY_KALIT") {
		t.Fatal("MAPS KALITI sahifaga tushib ketdi — u faqat /config/maps " +
			"orqali, tizimga kirgan foydalanuvchiga berilishi kerak")
	}
	// Kalit nomi ham qidiruvga tushmasin.
	if strings.Contains(body, "GOOGLE_MAPS_API_KEY") {
		t.Error("sahifada .env o'zgaruvchisi nomi bor — keraksiz ma'lumot")
	}
}

// ┌─ ONDEXMAP REJIMI (GOOGLE_MAP=false) ────────────────────────────────┐
// Sahifa OnDexMap'ni MapLibre bilan chizadi; kutubxonalar shu serverdan
// beriladi. Ular yo'qolsa yoki CSP ularni bloklasa xarita jimgina bo'sh
// qoladi va Google'ga qaytish ham ishlamaydi — shuning uchun qulflanadi.
// └────────────────────────────────────────────────────────────────────┘
func TestMapPickerLibsServed(t *testing.T) {
	h := mapPickerTestServer(t)
	for name, wantCT := range map[string]string{
		"maplibre-gl.js":  "text/javascript",
		"maplibre-gl.css": "text/css",
		"pmtiles.js":      "text/javascript",
	} {
		req := httptest.NewRequest(http.MethodGet, "/map-picker/lib/"+name, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: kutilgan 200, olindi %d", name, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, wantCT) {
			t.Errorf("%s: Content-Type %q, kutilgan %q", name, ct, wantCT)
		}
		if rec.Body.Len() < 10_000 {
			t.Errorf("%s: juda kichik (%d bayt) — bo'sh fayl embed qilinganmi?", name, rec.Body.Len())
		}
		etag := rec.Header().Get("ETag")
		if etag == "" {
			t.Fatalf("%s: ETag yo'q", name)
		}

		// Shartli so'rov: o'zgarmagan fayl qayta yuborilmaydi.
		req2 := httptest.NewRequest(http.MethodGet, "/map-picker/lib/"+name, nil)
		req2.Header.Set("If-None-Match", etag)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusNotModified || rec2.Body.Len() != 0 {
			t.Errorf("%s: If-None-Match bilan 304 va bo'sh tana kutilgan: %d, %d bayt",
				name, rec2.Code, rec2.Body.Len())
		}
	}
}

func TestMapPickerLibRejectsOtherNames(t *testing.T) {
	h := mapPickerTestServer(t)
	for _, name := range []string{
		"nomalum.js",
		"..%2Fmap_picker.html",
		"%2e%2e%2fmap_picker.html",
		"THIRD_PARTY_LICENSES.txt", // embed qilinmagan, ro'yxatda yo'q
		"MAPLIBRE-GL.JS",           // registr farqi
	} {
		req := httptest.NewRequest(http.MethodGet, "/map-picker/lib/"+name, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Errorf("%q: ruxsatsiz fayl berildi", name)
		}
	}
}

func TestMapPickerCSPAllowsOndexMap(t *testing.T) {
	h := mapPickerTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/map-picker", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	csp := rec.Header().Get("Content-Security-Policy")

	directive := func(name string) string {
		for _, part := range strings.Split(csp, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, name+" ") {
				return part
			}
		}
		return ""
	}
	// Uslub, shrift va plitkalar `*.ondex.uz` dan olinadi.
	if c := directive("connect-src"); !strings.Contains(c, "https://*.ondex.uz") {
		t.Errorf("connect-src OnDexMap'ni ruxsat etmaydi: %q", c)
	}
	// maplibre-gl.css shu serverdan <link> bilan yuklanadi.
	if c := directive("style-src"); !strings.Contains(c, "'self'") {
		t.Errorf("style-src 'self' siz maplibre-gl.css bloklanadi: %q", c)
	}
	// MapLibre ishchisi blob: dan ishga tushadi.
	if c := directive("worker-src"); !strings.Contains(c, "blob:") {
		t.Errorf("worker-src blob: siz MapLibre ishlamaydi: %q", c)
	}
	// Skriptlar uchun CDN ruxsati YO'Q: kutubxonalar shu serverdan.
	if c := directive("script-src"); strings.Contains(c, "unpkg") || strings.Contains(c, "jsdelivr") {
		t.Errorf("script-src da CDN bor: %q", c)
	}

	body := rec.Body.String()
	for _, want := range []string{"/map-picker/lib/", "ondexmap", "reportFallback", "/config/maps/fallback"} {
		if !strings.Contains(body, want) {
			t.Errorf("sahifada %q yo'q — OnDexMap yo'li yoki Google'ga qaytish yo'qolgan", want)
		}
	}
}

func TestOriginOf(t *testing.T) {
	for in, want := range map[string]string{
		"https://maps.ondex.uz":       "https://maps.ondex.uz",
		"https://maps.ondex.uz/x?y=1": "https://maps.ondex.uz",
		"http://localhost:8091/":      "http://localhost:8091",
		"":                            "",
		"maps.ondex.uz":               "",
		"javascript:alert(1)":         "",
	} {
		if got := originOf(in); got != want {
			t.Errorf("originOf(%q) = %q, kutilgan %q", in, got, want)
		}
	}
}
