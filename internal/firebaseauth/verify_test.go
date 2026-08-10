package firebaseauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Bu paket AUTENTIFIKATSIYANING YAGONA ishonch nuqtasi: agar u
// yaroqsiz tokenni o'tkazib yuborsa, hujumchi istalgan telefon
// raqami bilan kira oladi. Shuning uchun har bir tekshiruv alohida
// sinaladi — "umumiy holat ishlayapti" yetarli emas.

const testProject = "ondex-test"

type testEnv struct {
	v   *Verifier
	key *rsa.PrivateKey
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("kalit: %v", err)
	}
	v := New(testProject)
	// Tarmoqqa chiqmaymiz — o'z kalitimizni Google'niki o'rniga qo'yamiz.
	v.fetch = func(context.Context) (map[string]*rsa.PublicKey, time.Duration, error) {
		return map[string]*rsa.PublicKey{"test-kid": &key.PublicKey}, time.Hour, nil
	}
	return &testEnv{v: v, key: key}
}

// sign — berilgan da'volar bilan token yasaydi.
func (e *testEnv) sign(t *testing.T, claims jwt.MapClaims, kid string, method jwt.SigningMethod) string {
	t.Helper()
	tok := jwt.NewWithClaims(method, claims)
	tok.Header["kid"] = kid
	var (
		s   string
		err error
	)
	if method == jwt.SigningMethodHS256 {
		s, err = tok.SignedString([]byte("begona-sekret"))
	} else {
		s, err = tok.SignedString(e.key)
	}
	if err != nil {
		t.Fatalf("imzolash: %v", err)
	}
	return s
}

func validClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":          "https://securetoken.google.com/" + testProject,
		"aud":          testProject,
		"sub":          "firebase-uid-1",
		"iat":          now.Add(-time.Minute).Unix(),
		"exp":          now.Add(time.Hour).Unix(),
		"phone_number": "+998901234567",
	}
}

func TestVerifyValidToken(t *testing.T) {
	e := newTestEnv(t)
	tok := e.sign(t, validClaims(), "test-kid", jwt.SigningMethodRS256)

	got, err := e.v.Verify(context.Background(), tok)
	if err != nil {
		t.Fatalf("to'g'ri token rad etildi: %v", err)
	}
	if got.Phone != "+998901234567" {
		t.Fatalf("telefon noto'g'ri: %q", got.Phone)
	}
	if got.UID != "firebase-uid-1" {
		t.Fatalf("uid noto'g'ri: %q", got.UID)
	}
}

// BOSHQA LOYIHADA yasalgan token o'tmasligi kerak. Busiz istalgan odam
// o'zining Firebase loyihasini ochib, u yerda istalgan raqamni
// "tasdiqlab", bizga token yuborardi.
func TestVerifyRejectsForeignAudience(t *testing.T) {
	e := newTestEnv(t)
	c := validClaims()
	c["aud"] = "begona-loyiha"
	if _, err := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("begona loyiha tokeni qabul qilindi: %v", err)
	}
}

func TestVerifyRejectsWrongIssuer(t *testing.T) {
	e := newTestEnv(t)
	c := validClaims()
	c["iss"] = "https://securetoken.google.com/boshqa"
	if _, err := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("noto'g'ri iss qabul qilindi: %v", err)
	}
}

// Algoritmni almashtirish hujumi: hujumchi RS256 o'rniga HS256 qo'yib,
// ochiq kalitni sekret sifatida ishlatib imzolashga urinadi.
func TestVerifyRejectsAlgorithmSwap(t *testing.T) {
	e := newTestEnv(t)
	tok := e.sign(t, validClaims(), "test-kid", jwt.SigningMethodHS256)
	if _, err := e.v.Verify(context.Background(), tok); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("HS256 token qabul qilindi: %v", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	e := newTestEnv(t)
	c := validClaims()
	// clockSkew (60s) dan ANIQ uzoqroq.
	c["exp"] = time.Now().Add(-10 * time.Minute).Unix()
	if _, err := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("muddati o'tgan token qabul qilindi: %v", err)
	}
}

func TestVerifyRejectsUnknownKid(t *testing.T) {
	e := newTestEnv(t)
	if _, err := e.v.Verify(context.Background(), e.sign(t, validClaims(), "boshqa-kid", jwt.SigningMethodRS256)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("noma'lum kalit bilan token qabul qilindi: %v", err)
	}
}

func TestVerifyRejectsEmptySubject(t *testing.T) {
	e := newTestEnv(t)
	c := validClaims()
	c["sub"] = "   "
	if _, err := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256)); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("bo'sh sub qabul qilindi: %v", err)
	}
}

// Telefonsiz token (masalan Google hisobi orqali kirish) TELEFON
// endpointi uchun yaramaydi — aks holda raqamsiz akkaunt yaratilib
// ketardi. Tekshiruv `RequirePhone` da.
func TestRequirePhoneRejectsTokenWithoutPhone(t *testing.T) {
	e := newTestEnv(t)
	c := validClaims()
	delete(c, "phone_number")
	tok, err := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256))
	if err != nil {
		t.Fatalf("token o'zi yaroqli bo'lishi kerak edi: %v", err)
	}
	if err := tok.RequirePhone(); !errors.Is(err, ErrNoPhone) {
		t.Fatalf("telefonsiz token telefon endpointida qabul qilindi: %v", err)
	}
}

// ★ ASOSIY XAVFSIZLIK TESTI (Google -> telefon yo'nalishi)
//
// Firebase'da bitta hisobga bir nechta kirish usuli BOG'LANGAN bo'lishi
// mumkin (standart sozlamada Google va telefon tasdiqlangan email
// bo'yicha avtomatik birlashtiriladi). Bunday hisobga GOOGLE orqali
// kirilganda ham ID token ichida `phone_number` bo'ladi.
//
// Provayder tekshirilmasa, o'sha tokenni `/auth/google` o'rniga
// `/auth/firebase` ga yuborish mumkin edi. Farqi jiddiy:
//
//	/auth/google   -> `Issue`             (parol o'rnatish huquqi YO'Q)
//	/auth/firebase -> `IssuePhoneProven`  (15 daqiqa parolni JORIY
//	                                       parolsiz almashtirish huquqi)
//
// Ya'ni o'g'irlangan/ochiq qolgan Google sessiyasi parolni almashtirib,
// haqiqiy egani butunlay chiqarib yuborish imkonini berardi — aynan shu
// `LoginWithGoogle` da ATAYLAB to'silgan narsa.
func TestRequirePhoneRejectsGoogleToken(t *testing.T) {
	e := newTestEnv(t)
	c := validClaims() // ichida `phone_number` bor
	c["firebase"] = map[string]any{"sign_in_provider": "google.com"}
	tok, err := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if err := tok.RequirePhone(); !errors.Is(err, ErrWrongProvider) {
		t.Fatalf("IMTIYOZ OSHIRILDI: Google tokeni telefon endpointida qabul qilindi: %v", err)
	}
}

// Haqiqiy telefon oqimi buzilmasligi kerak.
func TestRequirePhoneAcceptsPhoneProvider(t *testing.T) {
	e := newTestEnv(t)
	c := validClaims()
	c["firebase"] = map[string]any{"sign_in_provider": "phone"}
	tok, err := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if err := tok.RequirePhone(); err != nil {
		t.Fatalf("halol telefon tokeni rad etildi: %v", err)
	}
}

// ---------- Google bilan kirish ----------

func googleClaims() jwt.MapClaims {
	c := validClaims()
	delete(c, "phone_number")
	c["email"] = "Shoxrux@Gmail.com"
	c["email_verified"] = true
	c["name"] = "Shoxrux Turaqulov"
	c["firebase"] = map[string]any{"sign_in_provider": "google.com"}
	return c
}

func TestGoogleTokenAccepted(t *testing.T) {
	e := newTestEnv(t)
	tok, err := e.v.Verify(context.Background(),
		e.sign(t, googleClaims(), "test-kid", jwt.SigningMethodRS256))
	if err != nil {
		t.Fatalf("Google tokeni rad etildi: %v", err)
	}
	if err := tok.RequireGoogleEmail(); err != nil {
		t.Fatalf("to'g'ri Google tokeni rad etildi: %v", err)
	}
	// Manzil kichik harfga keltirilishi kerak — akkauntlar
	// `lower(email)` bo'yicha topiladi.
	if tok.Email != "shoxrux@gmail.com" {
		t.Fatalf("email normallashtirilmadi: %q", tok.Email)
	}
	if tok.Name != "Shoxrux Turaqulov" {
		t.Fatalf("ism o'qilmadi: %q", tok.Name)
	}
}

// ★ ASOSIY XAVFSIZLIK TESTI
//
// Firebase'da PAROL bilan hisob ochgan odam ISTALGAN email da'vosini
// qo'ya oladi. Provayder tekshirilmasa, u shu da'vo bilan begona
// akkauntga kirib olardi.
func TestGoogleRejectsNonGoogleProvider(t *testing.T) {
	e := newTestEnv(t)
	c := googleClaims()
	c["firebase"] = map[string]any{"sign_in_provider": "password"}
	tok, err := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if err := tok.RequireGoogleEmail(); !errors.Is(err, ErrWrongProvider) {
		t.Fatalf("parol provayderi Google sifatida qabul qilindi: %v", err)
	}
}

// Tasdiqlanmagan email bilan begona manzilga bog'langan akkauntga
// kirib bo'lmasligi kerak.
func TestGoogleRejectsUnverifiedEmail(t *testing.T) {
	e := newTestEnv(t)
	c := googleClaims()
	c["email_verified"] = false
	tok, _ := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256))
	if err := tok.RequireGoogleEmail(); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("tasdiqlanmagan email qabul qilindi: %v", err)
	}
}

func TestGoogleRejectsMissingEmail(t *testing.T) {
	e := newTestEnv(t)
	c := googleClaims()
	delete(c, "email")
	tok, _ := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256))
	if err := tok.RequireGoogleEmail(); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("emailsiz token qabul qilindi: %v", err)
	}
}

// Telefon tokeni Google endpointiga kelib qolmasligi kerak.
func TestGoogleRejectsPhoneToken(t *testing.T) {
	e := newTestEnv(t)
	c := validClaims()
	c["firebase"] = map[string]any{"sign_in_provider": "phone"}
	tok, _ := e.v.Verify(context.Background(), e.sign(t, c, "test-kid", jwt.SigningMethodRS256))
	if err := tok.RequireGoogleEmail(); !errors.Is(err, ErrWrongProvider) {
		t.Fatalf("telefon tokeni Google endpointida qabul qilindi: %v", err)
	}
}

func TestVerifyDisabledWhenNoProject(t *testing.T) {
	v := New("")
	if _, err := v.Verify(context.Background(), "istalgan"); err == nil {
		t.Fatal("sozlanmagan holatda ham token qabul qilindi")
	}
}

func TestCacheTTL(t *testing.T) {
	if got := cacheTTL("public, max-age=19008, must-revalidate"); got != 19008*time.Second {
		t.Fatalf("max-age o'qilmadi: %v", got)
	}
	if got := cacheTTL(""); got != time.Hour {
		t.Fatalf("zaxira qiymat noto'g'ri: %v", got)
	}
	if got := cacheTTL("max-age=abc"); got != time.Hour {
		t.Fatalf("buzuq max-age da zaxira ishlatilmadi: %v", got)
	}
}
