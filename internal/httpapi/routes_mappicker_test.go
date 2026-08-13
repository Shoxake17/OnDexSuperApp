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
