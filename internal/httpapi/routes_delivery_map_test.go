package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// RS-1: yetkazish xaritasi sahifasi — nonce'li CSP, faqat panel manbasi
// freymlaydi (global X-Frame-Options: DENY olib tashlangan), sir yo'q,
// kutubxonalar nisbiy yo'ldan (web'da /r/api/ orqali).
func TestCourierMapPage(t *testing.T) {
	h := mapPickerTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/courier-map", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "frame-ancestors 'self' https://merchant.ondex.uz") || !strings.Contains(csp, "'nonce-") {
		t.Fatalf("CSP: %s", csp)
	}
	if strings.Contains(csp, "https:;") || strings.Contains(csp, "unsafe-eval") {
		t.Fatalf("CSP juda keng: %s", csp)
	}
	if xfo := rec.Header().Get("X-Frame-Options"); xfo != "" {
		t.Fatalf("X-Frame-Options qoldi: %q", xfo)
	}
	body := rec.Body.String()
	if strings.Contains(body, "__CSP_NONCE__") || strings.Contains(body, "__STYLE_URL__") {
		t.Fatal("belgilar almashtirilmadi")
	}
	for _, want := range []string{"/tiles/style.json", "'map-picker/lib/'", "chrome.webview", "e.origin === window.location.origin"} {
		if !strings.Contains(body, want) {
			t.Errorf("sahifada %q yo'q", want)
		}
	}
	for _, bad := range []string{"Authorization", "Bearer", "localStorage"} {
		if strings.Contains(body, bad) {
			t.Errorf("sahifada %q bo'lmasligi kerak", bad)
		}
	}
}
