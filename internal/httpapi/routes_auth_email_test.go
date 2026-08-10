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

// Email orqali kirish `EMAIL_LOGIN_ENABLED` bayrog'i ostida
// (`emailLoginReady` izohiga qarang). UI'ni yashirish yetarli emas edi:
// endpointlar ommaviy va to'g'ridan-to'g'ri chaqirilardi.

func emailTestServer(t *testing.T, enabled bool) http.Handler {
	t.Helper()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	s := New(Deps{
		UserRepo: userRepo,
		Tokens:   tokens,
		AuthSvc: users.NewService(userRepo, storage.NewMemoryCodeStore(),
			noopSms{}, tokens, NewID),
		DevMode:           true,
		EmailLoginEnabled: enabled,
	})
	return s.Routes(nil)
}

func postJSON(t *testing.T, h http.Handler, path, body string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// O'chirilgan holatda uchala yo'l ham YOPIQ bo'lishi kerak.
func TestEmailEndpointsClosedByDefault(t *testing.T) {
	h := emailTestServer(t, false)

	cases := []struct {
		name, path, body string
	}{
		{"kod so'rash", "/auth/email/request-code", `{"email":"a@b.com"}`},
		{"tasdiqlash", "/auth/email/verify", `{"email":"a@b.com","code":"123456"}`},
		{"email bilan register", "/auth/register",
			`{"email":"a@b.com","first_name":"A","password":"juda-kuchli-parol-7","password_confirm":"juda-kuchli-parol-7"}`},
	}
	for _, c := range cases {
		if got := postJSON(t, h, c.path, c.body); got != http.StatusNotFound {
			t.Fatalf("%s: 404 kutilgan, olindi %d", c.name, got)
		}
	}
}

// TELEFON bilan ro'yxatdan o'tish bayroqdan MUSTAQIL ishlashi kerak —
// email u yerda ixtiyoriy profil ma'lumoti, kimlik emas.
func TestPhoneRegisterUnaffectedByEmailFlag(t *testing.T) {
	h := emailTestServer(t, false)
	code := postJSON(t, h, "/auth/register",
		`{"phone":"+998901234567","email":"a@b.com","first_name":"A",`+
			`"password":"juda-kuchli-parol-7","password_confirm":"juda-kuchli-parol-7"}`)
	if code != http.StatusCreated {
		t.Fatalf("telefon bilan register bloklandi: %d", code)
	}
}

// Yoqilgan holatda yo'l ochiladi (xodim panellari uchun kerak bo'lganda).
func TestEmailEndpointsOpenWhenEnabled(t *testing.T) {
	h := emailTestServer(t, true)
	// SMTP ulanmagani uchun 404 EMAS, boshqa xato kutamiz — muhimi
	// bayroq to'sig'idan o'tgani.
	if got := postJSON(t, h, "/auth/email/request-code",
		`{"email":"a@b.com"}`); got == http.StatusNotFound {
		t.Fatal("bayroq yoqilgan bo'lsa ham endpoint yopiq qoldi")
	}
}
