package httpapi

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// AUD-8: origin HSTS'ni o'zi yuboradi (proksi sozlamasiga bog'liq emas).
func TestSecurityHeadersIncludeHSTS(t *testing.T) {
	rec := httptest.NewRecorder()
	writeSecurityHeaders(rec)
	h := rec.Header().Get("Strict-Transport-Security")
	age, _, _ := strings.Cut(strings.TrimPrefix(h, "max-age="), ";")
	if n, err := strconv.Atoi(age); err != nil || n < 31536000 || !strings.Contains(h, "includeSubDomains") {
		t.Fatalf("HSTS: %q (kamida 1 yil + includeSubDomains kerak)", h)
	}
	for k, want := range map[string]string{"X-Content-Type-Options": "nosniff", "X-Frame-Options": "DENY", "Referrer-Policy": "no-referrer"} {
		if got := rec.Header().Get(k); got != want {
			t.Fatalf("%s = %q, kutilgan %q", k, got, want)
		}
	}
}
