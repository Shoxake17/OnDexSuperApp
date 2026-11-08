// Package sso — OnDex'ni YAGONA KIRISH MANBAI (identity provider) qiladi.
//
// Boshqa xizmatlar (hozircha OnDex Merchant Panel) foydalanuvchini o'zlari
// tekshirmaydi: uni OnDex'ning kirish sahifasiga yuboradi, OnDex esa
// bir martalik KOD bilan oldindan ro'yxatdan o'tgan manzilga qaytaradi.
// Klient kodni server-server almashtirib, foydalanuvchining kimligini oladi.
//
// ┌─ NEGA OnDex TOKENI BERILMAYDI ──────────────────────────────────────┐
// Klient faqat KIM kirganini bilishi kerak (id, telefon, ism). OnDex
// tokeni (30 kunlik) esa shu foydalanuvchi nomidan OnDex'da HAR NARSA
// qilish imkonini beradi. Uni boshqa xizmatga berish — o'sha xizmatdagi
// bitta XSS butun akkauntni (buyurtma, wallet) olib ketishi demak.
// Shuning uchun almashtirish natijasi token EMAS, `Identity`.
// └─────────────────────────────────────────────────────────────────────┘
//
// ┌─ XAVFSIZLIK QOIDALARI ──────────────────────────────────────────────┐
//  1. `redirect_uri` ro'yxatdagi manzillardan biriga ANIQ teng bo'lishi
//     shart (prefiks, wildcard yo'q) — aks holda kod hujumchi saytiga
//     yo'naltirilib, o'g'irlanardi.
//  2. Kod bir martalik va 60 soniya yashaydi. Xotirada FAQAT uning
//     SHA-256 xeshi turadi.
//  3. Kod aynan o'sha klient va o'sha `redirect_uri` ga bog'langan.
//  4. Klient sirini taqqoslash doimiy vaqtda; noma'lum klient uchun ham
//     taqqoslash bajariladi (vaqt farqi klient mavjudligini bildirmasin).
//  5. Almashtirishdagi barcha rad etishlar BIR XIL xato beradi — qaysi
//     qism (kod, manzil, klient) noto'g'ri ekani oshkor bo'lmaydi.
//
// └─────────────────────────────────────────────────────────────────────┘
//
// CHEKLOV (ataylab): kodlar XOTIRADA. Server qayta ishga tushsa yo'lda
// turgan kodlar yo'qoladi va foydalanuvchi kirishni qayta boshlaydi (kod
// 60 soniya yashaydi, shuning uchun bu kamdan-kam holat). Bir nechta API
// nusxasi ishga tushirilsa kod Redis'ga ko'chirilishi kerak
// (`internal/revoke` dagi bilan bir xil cheklov).
package sso

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"chustapp/internal/users"
)

const (
	// CodeTTL — kod amal qilish muddati. Brauzer qaytishi va klient
	// serverining almashtirishi uchun yetarli, o'g'irlangan kod uchun qisqa.
	CodeTTL = 60 * time.Second

	// MaxStateLen — `state` uzunligi chegarasi (qaytish manzili cheksiz
	// uzayib ketmasligi uchun).
	MaxStateLen = 256

	// MinSecretLen — klient siri eng kam uzunligi (taxminiy tanlash
	// imkonsiz bo'lishi uchun; `openssl rand -hex 32` = 64 belgi).
	MinSecretLen = 32

	// maxPending — kutayotgan kodlar soni chegarasi. Autentifikatsiyadan
	// o'tgan odam ham cheksiz kod yaratib xotirani to'ldirmasligi kerak.
	maxPending = 10000
)

var (
	// ErrInvalidRequest — klient, manzil yoki state noto'g'ri. Qaysi biri
	// ekani ATAYLAB aytilmaydi.
	ErrInvalidRequest = errors.New("so'rov noto'g'ri")

	// ErrClientAuth — klient siri noto'g'ri yoki klient noma'lum.
	ErrClientAuth = errors.New("klient tasdiqlanmadi")

	// ErrInvalidGrant — kod yaroqsiz: noto'g'ri, ishlatilgan, muddati
	// o'tgan yoki boshqa klient/manzil uchun berilgan.
	ErrInvalidGrant = errors.New("kod yaroqsiz yoki muddati o'tgan")

	// ErrBusy — kutayotgan kodlar chegarasi to'ldi.
	ErrBusy = errors.New("xizmat band, birozdan keyin urinib ko'ring")
)

// Client — ro'yxatdan o'tgan SSO klienti.
type Client struct {
	// ID — klient identifikatori (ochiq, sir emas).
	ID string
	// Name — foydalanuvchiga tasdiqlash sahifasida KO'RSATILADI: u
	// qaysi ilovaga kirish berayotganini shundan biladi.
	Name string
	// Secret — kodni almashtirish uchun. FAQAT klient serverida turadi.
	Secret string
	// RedirectURIs — kod qaytariladigan YAGONA ruxsat etilgan manzillar.
	RedirectURIs []string
}

// Identity — klientga beriladigan kimlik. ENG KAM ma'lumot: manzil,
// to'lov ma'lumoti va token YO'Q.
//
// `Role` va `EntityID` faqat HAMKOR akkauntlari (restoran, do'kon) uchun
// to'ldiriladi: ularni faqat superadmin yaratadi, shuning uchun klient
// (POS paneli) shunga tayanib do'kon hisobini o'zi ocha oladi. Boshqa
// rollar (mijoz, kuryer, admin...) uchun bo'sh — klient ularni baribir
// qabul qilmasligi kerak.
type Identity struct {
	UserID string `json:"user_id"`
	Phone  string `json:"phone"`
	Name   string `json:"name"`
	// Role — "restaurant" yoki "store"; boshqa hollarda bo'sh.
	Role string `json:"role,omitempty"`
	// EntityID — restoran/do'kon ID'si.
	EntityID string `json:"entity_id,omitempty"`
	// EntityName — restoran/do'kon nomi (panelda ko'rsatiladi).
	EntityName string `json:"entity_name,omitempty"`
	// StoreCategory — FAQAT do'kon uchun: qurt | store | pharmacy. POS paneli
	// shu bo'yicha qaysi sahifa va API ochiq ekanligini hal qiladi.
	StoreCategory string `json:"store_category,omitempty"`
}

// EntityNamer — hamkor muassasasi nomini beradi (restoran nomi Mongo
// katalogida turadi, shuning uchun `users` dan olib bo'lmaydi). Xato yoki
// bo'sh nom — jiddiy emas: akkaunt ismi ishlatiladi.
type EntityNamer func(ctx context.Context, role users.Role, entityID string) string

// UserReader — foydalanuvchini o'qish (`users.Repository` ning bir qismi).
type UserReader interface {
	GetByID(ctx context.Context, id string) (*users.User, error)
}

type entry struct {
	userID      string
	clientID    string
	redirectURI string
	expires     time.Time
}

// Service — SSO xizmati.
type Service struct {
	clients map[string]Client
	users   UserReader
	namer   EntityNamer
	now     func() time.Time

	mu    sync.Mutex
	codes map[[sha256.Size]byte]entry // sha256(kod) -> yozuv
}

var clientIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// New — xizmatni yaratadi. Sozlama xatosi (zaif sir, xavfsiz bo'lmagan
// manzil) DARHOL qaytariladi: jimgina qabul qilingan zaif sozlama
// keyinroq o'g'irlangan kod sifatida chiqadi.
func New(clients []Client, ur UserReader) (*Service, error) {
	if ur == nil {
		return nil, errors.New("sso: foydalanuvchi ombori berilmagan")
	}
	if len(clients) == 0 {
		return nil, errors.New("sso: birorta klient berilmagan")
	}
	m := make(map[string]Client, len(clients))
	for _, c := range clients {
		if err := validateClient(c); err != nil {
			return nil, fmt.Errorf("sso: klient %q: %w", c.ID, err)
		}
		if _, dup := m[c.ID]; dup {
			return nil, fmt.Errorf("sso: klient %q ikki marta berilgan", c.ID)
		}
		m[c.ID] = c
	}
	return &Service{
		clients: m,
		users:   ur,
		now:     time.Now,
		codes:   make(map[[sha256.Size]byte]entry),
	}, nil
}

func validateClient(c Client) error {
	if !clientIDRe.MatchString(c.ID) {
		return errors.New("id noto'g'ri (a-z, 0-9, _ va -, 32 belgigacha)")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("nom bo'sh")
	}
	if len(c.Secret) < MinSecretLen {
		return fmt.Errorf("sir kamida %d belgi bo'lishi kerak", MinSecretLen)
	}
	if len(c.RedirectURIs) == 0 {
		return errors.New("redirect manzillar ro'yxati bo'sh")
	}
	for _, raw := range c.RedirectURIs {
		if err := validateRedirectURI(raw); err != nil {
			return fmt.Errorf("manzil %q: %w", raw, err)
		}
	}
	return nil
}

// validateRedirectURI — faqat https yoki localhost'dagi http. Fragment va
// foydalanuvchi ma'lumoti (`user:pass@`) taqiqlanadi: ular manzilni
// tekshirishda chalkashlik keltirib chiqaradi.
func validateRedirectURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return errors.New("yaroqsiz manzil")
	}
	if u.Fragment != "" || strings.Contains(raw, "#") {
		return errors.New("fragment (#) taqiqlangan")
	}
	if u.User != nil {
		return errors.New("manzilda foydalanuvchi ma'lumoti taqiqlangan")
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		switch u.Hostname() {
		case "localhost", "127.0.0.1", "::1":
			return nil
		}
		return errors.New("http faqat localhost uchun ruxsat etiladi")
	}
	return errors.New("faqat https ruxsat etiladi")
}

// ParseRedirectURIs — vergul bilan ajratilgan ro'yxatni bo'laklaydi.
func ParseRedirectURIs(csv string) []string {
	var out []string
	for _, p := range strings.Split(csv, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// resolve — (klient, manzil) juftligini tekshiradi. Manzil ro'yxatdagi
// biriga ANIQ teng bo'lishi shart.
func (s *Service) resolve(clientID, redirectURI string) (Client, error) {
	c, ok := s.clients[clientID]
	if !ok {
		return Client{}, ErrInvalidRequest
	}
	for _, allowed := range c.RedirectURIs {
		if allowed == redirectURI {
			return c, nil
		}
	}
	return Client{}, ErrInvalidRequest
}

// ClientInfo — tasdiqlash sahifasi uchun: ilova nomi va qaytish manzili
// domeni. Juftlik noto'g'ri bo'lsa `ErrInvalidRequest`.
func (s *Service) ClientInfo(clientID, redirectURI string) (name, host string, err error) {
	c, err := s.resolve(clientID, redirectURI)
	if err != nil {
		return "", "", err
	}
	u, _ := url.Parse(redirectURI)
	return c.Name, u.Host, nil
}

// Authorize — kirgan foydalanuvchi ruxsat berganda chaqiriladi. Klient
// qaytariladigan to'liq manzilni (`code` va `state` bilan) qaytaradi.
func (s *Service) Authorize(userID, clientID, redirectURI, state string) (string, error) {
	if userID == "" || len(state) > MaxStateLen {
		return "", ErrInvalidRequest
	}
	if _, err := s.resolve(clientID, redirectURI); err != nil {
		return "", err
	}
	code, err := newCode()
	if err != nil {
		return "", err
	}

	now := s.now()
	s.mu.Lock()
	s.sweepLocked(now)
	if len(s.codes) >= maxPending {
		s.mu.Unlock()
		return "", ErrBusy
	}
	s.codes[hashCode(code)] = entry{
		userID: userID, clientID: clientID, redirectURI: redirectURI,
		expires: now.Add(CodeTTL),
	}
	s.mu.Unlock()

	u, err := url.Parse(redirectURI)
	if err != nil {
		return "", ErrInvalidRequest
	}
	q := u.Query()
	q.Set("code", code)
	if state != "" {
		q.Set("state", state)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Exchange — klient serveri kodni kimlikka almashtiradi.
//
// Tartib muhim: avval KLIENT tasdiqlanadi. Sirni bilmaydigan odam kodni
// "yoqib yubora" olmaydi (xizmatni to'xtatish hujumi). Klient tasdiqlangach
// kod har qanday holatda ISTE'MOL qilinadi — muvaffaqiyatsiz urinishdan
// keyin ham qayta ishlatib bo'lmaydi.
func (s *Service) Exchange(ctx context.Context, clientID, secret, code, redirectURI string) (*Identity, error) {
	c, known := s.clients[clientID]
	// Noma'lum klient uchun ham taqqoslash bajariladi (bo'sh sirga qarshi).
	match := subtle.ConstantTimeCompare([]byte(secret), []byte(c.Secret)) == 1
	if !known || !match {
		return nil, ErrClientAuth
	}

	key := hashCode(code)
	now := s.now()
	s.mu.Lock()
	e, found := s.codes[key]
	if found {
		delete(s.codes, key)
	}
	s.mu.Unlock()

	if !found || now.After(e.expires) || e.clientID != clientID ||
		subtle.ConstantTimeCompare([]byte(e.redirectURI), []byte(redirectURI)) != 1 {
		return nil, ErrInvalidGrant
	}

	return s.identityOf(ctx, e.userID)
}

// Lookup — klient SERVERI foydalanuvchi kimligini (kodsiz) qayta o'qiydi.
//
// Nega kerak: kategoriya/nom OnDex'da o'zgarishi mumkin (admin do'konni qurtdan
// oddiy do'konga o'tkazdi), POS esa uni FAQAT kirishda oladi. Lookup POS'ga
// sessiya davomida ham yangilab turish imkonini beradi.
//
// Xavfsizlik: klient SIRI bilan tasdiqlanadi (Exchange kabi, doimiy vaqtli
// taqqoslash); noma'lum/o'chirilgan foydalanuvchi ham, noma'lum klient ham bir
// xil xato beradi. Kimlik Exchange bergan bilan AYNAN bir xil maydonlar.
func (s *Service) Lookup(ctx context.Context, clientID, secret, userID string) (*Identity, error) {
	c, known := s.clients[clientID]
	match := subtle.ConstantTimeCompare([]byte(secret), []byte(c.Secret)) == 1
	if !known || !match {
		return nil, ErrClientAuth
	}
	if userID == "" || len(userID) > 128 {
		return nil, ErrInvalidGrant
	}
	return s.identityOf(ctx, userID)
}

// identityOf — foydalanuvchidan klientga beriladigan kimlikni quradi.
func (s *Service) identityOf(ctx context.Context, userID string) (*Identity, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, users.ErrUserNotFound) {
			return nil, ErrInvalidGrant
		}
		return nil, err
	}
	// Kod berilgandan keyin akkaunt o'chirilgan bo'lishi mumkin.
	if u.DeletedAt != nil {
		return nil, ErrInvalidGrant
	}
	id := &Identity{UserID: u.ID, Phone: u.Phone, Name: u.Name}
	if (u.Role == users.RoleRestaurant || u.Role == users.RoleStore) && u.EntityID != "" {
		id.Role = string(u.Role)
		id.EntityID = u.EntityID
		id.EntityName = u.Name
		if u.Role == users.RoleStore {
			// Kategoriya yozilmagan (eski) do'kon — oddiy do'kon. Cheklangan
			// panel faqat ANIQ "qurt" qiymatida beriladi.
			id.StoreCategory = string(u.StoreCategory)
			if id.StoreCategory == "" {
				id.StoreCategory = string(users.StoreCategoryStore)
			}
		}
		if u.Role == users.RoleRestaurant {
			id.EntityName = ""
			if s.namer != nil {
				id.EntityName = s.namer(ctx, u.Role, u.EntityID)
			}
			if id.EntityName == "" {
				id.EntityName = u.Name
			}
		}
	}
	return id, nil
}

// IdentityOf — foydalanuvchining hamkor kimligi (OIDC userinfo uchun; kod
// almashuvisiz, chaqiruvchi tokenni o'zi tekshirgan bo'lishi SHART).
func (s *Service) IdentityOf(ctx context.Context, userID string) (*Identity, error) {
	return s.identityOf(ctx, userID)
}

// WithEntityNamer — restoran nomini katalogdan olish uchun (ixtiyoriy).
func (s *Service) WithEntityNamer(n EntityNamer) *Service {
	s.namer = n
	return s
}

// sweepLocked — muddati o'tgan kodlarni olib tashlaydi (`s.mu` ushlangan).
func (s *Service) sweepLocked(now time.Time) {
	for k, e := range s.codes {
		if now.After(e.expires) {
			delete(s.codes, k)
		}
	}
}

func newCode() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashCode(code string) [sha256.Size]byte {
	return sha256.Sum256([]byte(code))
}
