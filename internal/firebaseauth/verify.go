// Package firebaseauth — Firebase Phone Auth tokenini SERVERDA
// tekshiradi.
//
// ── ENG MUHIM QOIDA ────────────────────────────────────────────────
// Ilova yuborgan telefon raqamiga HECH QACHON ishonilmaydi. Mijoz
// istalgan raqamni yozib yuborishi mumkin — u shunchaki HTTP so'rov.
// Ishonch faqat Google IMZOLAGAN ID tokendan keladi: imzo tekshiriladi,
// keyin raqam TOKEN ICHIDAN olinadi.
//
// Busiz butun Firebase integratsiyasi bezak bo'lib qolardi: hujumchi
// `{"phone":"+998901234567"}` deb yuborib, istalgan akkauntga kirardi.
//
// ── NEGA FIREBASE ADMIN SDK EMAS ───────────────────────────────────
// Admin SDK xizmat akkaunti kalitini (JSON) talab qiladi — bu serverda
// saqlanadigan yana bitta maxfiy qiymat va u BUTUN Firebase loyihasiga
// to'liq huquq beradi. ID tokenni tekshirish uchun esa maxfiy narsa
// UMUMAN kerak emas: Google'ning OCHIQ sertifikatlari va loyiha ID'si
// yetarli. Kamroq maxfiylik = kamroq zarar doirasi.
package firebaseauth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// googleCertsURL — Firebase ID tokenlarni imzolaydigan kalitlarning
// ochiq sertifikatlari. Google ularni davriy almashtiradi, shuning
// uchun javobdagi `Cache-Control: max-age` hurmat qilinadi.
const googleCertsURL = "https://www.googleapis.com/robot/v1/metadata/x509/securetoken@system.gserviceaccount.com"

// clockSkew — server va Google soatlari orasidagi kichik farqga
// chidamlilik. Nolga teng bo'lsa, bir necha soniyalik farq ham
// halol tokenni rad etardi.
const clockSkew = 60 * time.Second

var (
	ErrInvalidToken = errors.New("firebase token yaroqsiz")
	ErrNoPhone      = errors.New("firebase tokenida telefon raqami yo'q")
	// ErrWrongProvider — token boshqa kirish usuli bilan berilgan.
	ErrWrongProvider = errors.New("kirish usuli mos emas")
	// ErrEmailNotVerified — email yo'q yoki tasdiqlanmagan.
	ErrEmailNotVerified = errors.New("email tasdiqlanmagan")
)

// Token — tekshirilgan va ISHONCHLI ma'lumot.
type Token struct {
	// UID — Firebase foydalanuvchi identifikatori (`sub`).
	UID string
	// Phone — TASDIQLANGAN raqam (E.164). Telefon bilan kirishda
	// to'ladi, Google bilan kirishda bo'sh bo'ladi.
	Phone string
	// Email — hisobning email manzili. O'ZI YETARLI EMAS:
	// `EmailVerified` ham tekshirilishi SHART (pastga qarang).
	Email string
	// EmailVerified — email EGALIGI isbotlanganmi.
	//
	// NEGA ALOHIDA: `email` da'vosini istalgan provayder qo'yishi
	// mumkin (masalan parol bilan ochilgan Firebase hisobi). Faqat
	// `email_verified = true` bo'lgandagina manzil egasi tasdiqlangan
	// hisoblanadi.
	EmailVerified bool
	// Provider — `firebase.sign_in_provider` ("google.com", "phone",
	// "password", ...). Endpoint kutgan provayderdan boshqasi kelsa
	// rad etiladi.
	Provider string
	// Name — Google bergan to'liq ism (bo'lishi shart emas).
	Name string
}

// Verifier — ID tokenlarni tekshiradi. Goroutine-xavfsiz.
type Verifier struct {
	projectID string
	client    *http.Client

	mu        sync.RWMutex
	keys      map[string]*rsa.PublicKey
	keysUntil time.Time

	// fetch — sertifikatlarni oluvchi. Testlarda almashtiriladi
	// (tashqi tarmoqqa chiqmasdan o'z kalitimiz bilan sinash uchun).
	fetch func(ctx context.Context) (map[string]*rsa.PublicKey, time.Duration, error)
	now   func() time.Time
}

func New(projectID string) *Verifier {
	v := &Verifier{
		projectID: projectID,
		client:    &http.Client{Timeout: 10 * time.Second},
		keys:      map[string]*rsa.PublicKey{},
		now:       time.Now,
	}
	v.fetch = v.fetchGoogleCerts
	return v
}

// ProjectID — sozlangan loyiha (bo'sh bo'lsa integratsiya o'chirilgan).
func (v *Verifier) ProjectID() string { return v.projectID }

// Verify — tokenni to'liq tekshiradi va ISHONCHLI ma'lumotni qaytaradi.
//
// Tekshiriladigan har bir narsa va NEGA:
//
//	alg = RS256  — "alg: none" yoki HMAC'ga almashtirish hujumini to'sadi
//	               (hujumchi o'z kalitida imzolab yuborardi);
//	imzo         — kalit Google'ning ochiq sertifikatidan, `kid` bo'yicha;
//	iss          — aynan securetoken.google.com/<loyiha>;
//	aud          — aynan bizning loyiha ID'imiz. BUSIZ boshqa birovning
//	               Firebase loyihasida yasalgan token ham o'tib ketardi;
//	exp / iat    — muddat (kichik soat farqiga chidamlilik bilan);
//	sub          — bo'sh bo'lmasligi kerak.
func (v *Verifier) Verify(ctx context.Context, idToken string) (*Token, error) {
	if v.projectID == "" {
		return nil, errors.New("firebase sozlanmagan (FIREBASE_PROJECT_ID yo'q)")
	}

	var claims jwt.MapClaims
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer("https://securetoken.google.com/"+v.projectID),
		jwt.WithAudience(v.projectID),
		jwt.WithLeeway(clockSkew),
		jwt.WithExpirationRequired(),
	)
	tok, err := parser.ParseWithClaims(idToken, &claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("kid yo'q")
		}
		return v.keyByID(ctx, kid)
	})
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}

	sub, _ := claims["sub"].(string)
	if strings.TrimSpace(sub) == "" {
		return nil, ErrInvalidToken
	}
	// `iat` kelajakda bo'lmasligi kerak — `WithLeeway` buni qamrab
	// oladi, lekin maydonning o'zi bor-yo'qligini ham talab qilamiz.
	if _, ok := claims["iat"]; !ok {
		return nil, ErrInvalidToken
	}

	phone, _ := claims["phone_number"].(string)
	email, _ := claims["email"].(string)
	emailVerified, _ := claims["email_verified"].(bool)
	name, _ := claims["name"].(string)

	// `firebase.sign_in_provider` — qaysi usul bilan kirilgani.
	// Endpoint buni tekshiradi: telefon endpointiga Google tokeni
	// (yoki aksincha) kelib qolmasin.
	var provider string
	if fb, ok := claims["firebase"].(map[string]any); ok {
		provider, _ = fb["sign_in_provider"].(string)
	}

	return &Token{
		UID:           sub,
		Phone:         strings.TrimSpace(phone),
		Email:         strings.ToLower(strings.TrimSpace(email)),
		EmailVerified: emailVerified,
		Provider:      provider,
		Name:          strings.TrimSpace(name),
	}, nil
}

// RequirePhone — telefon endpointi uchun.
//
// IKKALA shart ham majburiy (`RequireGoogleEmail` bilan simmetrik):
//   - provayder AYNAN "phone";
//   - raqam mavjud.
//
// ┌─ NEGA PROVAYDER HAM TEKSHIRILADI ─────────────────────────────────┐
// Firebase'da bitta hisobga bir nechta kirish usuli BOG'LANGAN
// bo'lishi mumkin (standart sozlamada Google va telefon tasdiqlangan
// email bo'yicha avtomatik birlashtiriladi). Bunday hisobga GOOGLE
// orqali kirilganda ham ID token ichida `phone_number` bo'ladi.
//
// Avval faqat raqam borligi tekshirilardi, ya'ni o'sha tokenni
// `/auth/google` o'rniga `/auth/firebase` ga yuborish mumkin edi.
// Farqi jiddiy:
//
//	/auth/google   -> `Issue`             (parol o'rnatish huquqi YO'Q)
//	/auth/firebase -> `IssuePhoneProven`  (15 daqiqa parolni JORIY
//	                                       parolsiz almashtirish huquqi)
//
// Ya'ni ochiq qolgan Google sessiyasi parolni almashtirib, keyin
// `Revoke` orqali haqiqiy egani butunlay chiqarib yuborardi — aynan
// shu `LoginWithGoogle` da ataylab to'silgan narsa
// (`users.Service.LoginWithGoogle` izohiga qarang).
// └───────────────────────────────────────────────────────────────────┘
func (t *Token) RequirePhone() error {
	// Tartib MUHIM: telefonsiz Google tokeni uchun ham `ErrNoPhone`
	// emas, `ErrWrongProvider` qaytishi mantiqan to'g'ri bo'lardi,
	// lekin mavjud xatti-harakat (telefonsiz token -> `ErrNoPhone`)
	// saqlanadi — u aniqroq va testlar bilan qoplangan.
	if t.Phone == "" {
		return ErrNoPhone
	}
	if t.Provider != "phone" {
		return ErrWrongProvider
	}
	return nil
}

// RequireGoogleEmail — Google endpointi uchun.
//
// IKKALA shart ham majburiy:
//   - provayder AYNAN "google.com" — aks holda parol bilan ochilgan
//     Firebase hisobi istalgan email da'vosi bilan o'tib ketardi;
//   - `email_verified = true` — manzil EGALIGI isbotlangan bo'lsin.
//
// Busiz hujumchi begona email bilan hisob ochib, o'sha manzilga
// bog'langan akkauntimizga kirib olardi.
func (t *Token) RequireGoogleEmail() error {
	if t.Provider != "google.com" {
		return ErrWrongProvider
	}
	if t.Email == "" || !t.EmailVerified {
		return ErrEmailNotVerified
	}
	return nil
}

// keyByID — `kid` bo'yicha ochiq kalit; kerak bo'lsa yangilaydi.
func (v *Verifier) keyByID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	k, ok := v.keys[kid]
	fresh := v.now().Before(v.keysUntil)
	v.mu.RUnlock()
	if ok && fresh {
		return k, nil
	}

	keys, ttl, err := v.fetch(ctx)
	if err != nil {
		// Yangilab bo'lmasa, eski kalit bo'lsa u bilan davom etamiz:
		// Google'ga vaqtinchalik yetib bo'lmagani butun kirishni
		// to'xtatib qo'ymasligi kerak.
		if ok {
			return k, nil
		}
		return nil, err
	}
	v.mu.Lock()
	v.keys = keys
	v.keysUntil = v.now().Add(ttl)
	v.mu.Unlock()

	if k, ok := keys[kid]; ok {
		return k, nil
	}
	return nil, errors.New("imzo kaliti topilmadi")
}

func (v *Verifier) fetchGoogleCerts(ctx context.Context) (map[string]*rsa.PublicKey, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleCertsURL, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("google sertifikatlari: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, 0, err
	}
	var raw map[string]string
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, 0, err
	}
	keys := make(map[string]*rsa.PublicKey, len(raw))
	for kid, certPEM := range raw {
		block, _ := pem.Decode([]byte(certPEM))
		if block == nil {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		if pk, ok := cert.PublicKey.(*rsa.PublicKey); ok {
			keys[kid] = pk
		}
	}
	if len(keys) == 0 {
		return nil, 0, errors.New("google sertifikatlari bo'sh")
	}
	return keys, cacheTTL(resp.Header.Get("Cache-Control")), nil
}

// cacheTTL — `Cache-Control: public, max-age=NNN` dan muddatni oladi.
// Topilmasa ehtiyotkor standart (1 soat).
func cacheTTL(header string) time.Duration {
	const fallback = time.Hour
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if after, ok := strings.CutPrefix(part, "max-age="); ok {
			if n, err := strconv.Atoi(after); err == nil && n > 0 {
				return time.Duration(n) * time.Second
			}
		}
	}
	return fallback
}
