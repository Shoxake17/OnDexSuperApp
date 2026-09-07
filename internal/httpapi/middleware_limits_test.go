package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// CORS va tana chegarasi middleware'ining testlari
// (bug.md 23 va 28-bandlar).

// 23-BAND: CORS metodlari ro'yxati HAQIQIY marshrutlarga mos bo'lishi
// kerak.
//
// ┌─ NEGA BU JIM XATO ─────────────────────────────────────────────────┐
// Ro'yxatda yo'q metod preflight (OPTIONS) bosqichida rad etiladi va
// so'rov brauzerdan UMUMAN chiqmaydi — server hech qanday xato
// ko'rmaydi. Hozir sezilmasdi, chunki `PATCH`/`PUT` marshrutlarini
// faqat Flutter panellari (desktop, CORS'siz) chaqiradi.
// └────────────────────────────────────────────────────────────────────┘
func TestCORSAllowsEveryMethodUsedByRoutes(t *testing.T) {
	// Manba koddan haqiqiy metodlarni olamiz — ro'yxat qo'lda
	// yozilmaydi, ya'ni yangi metod qo'shilganda test o'zi ushlaydi.
	used := map[string]bool{}
	for _, r := range discoverProtectedRoutes(t) {
		used[r.method] = true
	}
	if len(used) == 0 {
		t.Fatal("birorta marshrut topilmadi")
	}

	for m := range used {
		if !strings.Contains(allowedCORSMethods, m) {
			t.Errorf("XAVFSIZLIK/UZILISH: %q metodi marshrutlarda ishlatiladi, "+
				"lekin `Access-Control-Allow-Methods` ro'yxatida YO'Q (%q) — "+
				"brauzer klientlari bu endpointlarga chiqa olmaydi",
				m, allowedCORSMethods)
		}
	}
	// OPTIONS — preflight uchun, marshrutlarda uchramaydi.
	if !strings.Contains(allowedCORSMethods, "OPTIONS") {
		t.Error("OPTIONS ro'yxatda yo'q — preflight umuman ishlamaydi")
	}
}

// Sarlavha haqiqatan javobga qo'yiladimi.
func TestCORSHeaderIsSent(t *testing.T) {
	h := withCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		[]string{"https://ondex.uz"}, false)
	req := httptest.NewRequest("OPTIONS", "/me", nil)
	req.Header.Set("Origin", "https://ondex.uz")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	got := w.Header().Get("Access-Control-Allow-Methods")
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		if !strings.Contains(got, m) {
			t.Errorf("javobda %q yo'q: %q", m, got)
		}
	}
}

// 28-BAND: tana chegarasi FAQAT haqiqiy `/uploads` endpointida
// o'chishi kerak.
//
// Avval shart `HasPrefix(path, "/uploads")` edi — `/uploadsfoo` va
// `/uploads-test` ham chegarasiz qolardi. Hozir bunday marshrut yo'q,
// lekin qo'shilgan kunda u JIMGINA chegarasiz bo'lardi.
func TestIsUploadsPath(t *testing.T) {
	cases := map[string]bool{
		"/uploads":         true,
		"/uploads/":        true,
		"/uploads/a/b.png": true,
		"/uploadsfoo":      false, // ASOSIY REGRESSIYA
		"/uploads-test":    false,
		"/uploadsx/evil":   false,
		"/me":              false,
		"/":                false,
		"":                 false,
		"/api/uploads":     false, // faqat ildizdagi endpoint
		"/UPLOADS":         false, // yo'llar registr-sezgir
	}
	for path, want := range cases {
		if got := isUploadsPath(path); got != want {
			t.Errorf("isUploadsPath(%q) = %v, kutilgan %v", path, got, want)
		}
	}
}

// Chegara HAQIQATAN qo'llanadimi: 1 MB dan katta tana rad etilishi
// kerak, `/uploads` esa o'tishi.
func TestBodyLimitApplies(t *testing.T) {
	var readErr error
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		for {
			_, err := r.Body.Read(buf)
			if err != nil {
				if err.Error() != "EOF" {
					readErr = err
				}
				return
			}
		}
	})
	h := withBodyLimit(inner)

	big := strings.NewReader(strings.Repeat("x", (1<<20)+1024))

	// Oddiy yo'l — chegara qo'llanadi.
	readErr = nil
	req := httptest.NewRequest("POST", "/auth/request-code", big)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if readErr == nil {
		t.Error("1 MB dan katta tana chegarasiz o'qildi")
	}

	// `/uploadsfoo` — bu `/uploads` EMAS, chegara qo'llanishi kerak.
	readErr = nil
	req = httptest.NewRequest("POST", "/uploadsfoo",
		strings.NewReader(strings.Repeat("x", (1<<20)+1024)))
	h.ServeHTTP(httptest.NewRecorder(), req)
	if readErr == nil {
		t.Error("XAVFSIZLIK: /uploadsfoo chegarasiz o'qildi — prefiks bo'yicha chetlab o'tildi")
	}

	// Haqiqiy `/uploads/...` — chegara o'chadi (u o'z chegarasini qo'yadi).
	readErr = nil
	req = httptest.NewRequest("POST", "/uploads/rasm.png",
		strings.NewReader(strings.Repeat("x", (1<<20)+1024)))
	h.ServeHTTP(httptest.NewRecorder(), req)
	if readErr != nil {
		t.Errorf("/uploads yo'liga chegara qo'llandi: %v", readErr)
	}
}
