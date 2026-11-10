package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"chustapp/internal/ttlcache"
	"chustapp/internal/users"
)

// ClientPolicy — bitta klient (ilova) uchun qoidalar. Har klient BITTA
// realmga bog'langan: merchant ilovasida mijoz sessiyasi ochilmaydi.
type ClientPolicy struct {
	Realm users.Realm
	Idle  time.Duration // harakatsiz turish chegarasi
	Abs   time.Duration // mutlaq umr
	// NoPublicRefresh — refresh FAQAT o'z yo'li orqali: konfidensial OIDC
	// klienti (klient siri bilan, /v2/oauth/token) yoki umuman yo'q (SSO cookie
	// sessiyasi). Umumiy /v2/auth/token/refresh bunday tokenni rad etadi va
	// sessiyani yopadi (RFC 9700 4.14: konfidensial klient refresh'i klient
	// autentifikatsiyasiga bog'langan bo'lishi SHART).
	NoPublicRefresh bool
}

// DefaultClients — egasi tasdiqlagan muddatlar (AUTH-PHASE-A.md 4-bo'lim).
// Yangi klient faqat shu ro'yxatga qo'shib e'lon qilinadi — noma'lum
// `client_id` bilan sessiya ochilmaydi.
var DefaultClients = map[string]ClientPolicy{
	"merchant-web":     {Realm: users.RealmMerchant, Idle: 7 * 24 * time.Hour, Abs: 30 * 24 * time.Hour, NoPublicRefresh: true},
	"merchant-desktop": {Realm: users.RealmMerchant, Idle: 7 * 24 * time.Hour, Abs: 30 * 24 * time.Hour},
	"ondex-pro":        {Realm: users.RealmMerchant, Idle: 14 * 24 * time.Hour, Abs: 60 * 24 * time.Hour},
	"ondex-go":         {Realm: users.RealmMerchant, Idle: 14 * 24 * time.Hour, Abs: 60 * 24 * time.Hour},
	"admin-web":        {Realm: users.RealmPlatform, Idle: 30 * time.Minute, Abs: 12 * time.Hour},
	// ondex-customer — mijoz ilovasi (137). Eski token 30 kun mutlaq edi;
	// endi 30 kun HARAKATSIZ (ishlatib turgan mijoz chiqib ketmaydi), 180
	// kun mutlaq — keyin bir marta qayta kirish. Qurilma alohida ko'rinadi
	// va alohida yopiladi.
	CustomerClientID: {Realm: users.RealmCustomer, Idle: 30 * 24 * time.Hour, Abs: 180 * 24 * time.Hour},
	// ondex-sso — OnDex kirish sahifasining brauzer sessiyasi (OIDC SSO
	// cookie). Refresh ishlatilmaydi (cookie faqat sid ko'rsatkichi), shuning
	// uchun idle = abs: 7 kundan keyin qayta kirish.
	SSOClientID: {Realm: users.RealmMerchant, Idle: 7 * 24 * time.Hour, Abs: 7 * 24 * time.Hour, NoPublicRefresh: true},
}

// refreshRaceGrace — HALOL poyga: klient bir refresh tokenni tarmoq xatosi
// sababli ikki marta yuboradi (birinchisi muvaffaqiyatli, javob yetmagan).
// Shu oyna ichidagi takror o'g'irlik deb hisoblanMAYDI (sessiya bekor
// qilinmaydi) — lekin yangi token ham berilmaydi (eski token bir marta
// ishlatilgan). Oynadan keyingi takror — o'g'irlik belgisi.
const refreshRaceGrace = 10 * time.Second

// activeCacheTTL — "sessiya faolmi" natijasining keshi. Boshqa nusxada
// bekor qilingan sessiya ko'pi bilan shuncha vaqt ishlaydi; shu nusxada
// bekor qilinsa kesh DARHOL tozalanadi.
const activeCacheTTL = 30 * time.Second

var (
	ErrUnknownClient  = errors.New("noma'lum klient")
	ErrRealmMismatch  = errors.New("akkaunt bu ilova uchun emas")
	ErrRefreshInvalid = errors.New("sessiya tugagan — qaytadan kiring")
	// ErrRefreshReuse — allaqachon almashtirilgan refresh token qayta keldi:
	// o'g'irlik belgisi. Sessiya BEKOR qilinadi.
	ErrRefreshReuse = errors.New("sessiya xavfsizlik sababli yopildi — qaytadan kiring")
	ErrNotFound     = errors.New("sessiya topilmadi")
)

// Session — markaziy sessiya yozuvi (`auth_sessions`).
type Session struct {
	ID            string
	UserID        string
	Realm         users.Realm
	ClientID      string
	OrgID         string
	AMR           []string
	MFAAt         *time.Time
	CreatedAt     time.Time
	LastUsedAt    time.Time
	IdleExpiresAt time.Time
	AbsExpiresAt  time.Time
	RevokedAt     *time.Time
	RevokeReason  string
	IPHash        []byte
	UserAgent     string
	DeviceName    string
	// ParentID — OIDC orqali ochilgan ilova sessiyasining SSO sessiyasi
	// (`ondex-sso`). Bo'sh — to'g'ridan-to'g'ri kirish. SSO yopilsa bolalari
	// ham yopiladi (`RevokeChildren`).
	ParentID string
}

// Active — sessiya hozir yaroqlimi.
func (s *Session) Active(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.IdleExpiresAt) && now.Before(s.AbsExpiresAt)
}

// RotateResult — `Store.Rotate` natijasi.
type RotateResult int

const (
	RotateOK       RotateResult = iota
	RotateNotFound              // bunday refresh umuman yo'q
	RotateExpired               // topildi, lekin muddati o'tgan yoki bekor qilingan
	RotateReused                // ESKI refresh (prev) keldi — sessiya bekor qilindi
	RotateRaced                 // ESKI refresh, lekin halol poyga oynasida
)

// Store — sessiyalar ombori. Implementatsiya `Rotate` ni ATOMIK bajarishi
// SHART (Postgres'da `SELECT ... FOR UPDATE` bitta tranzaksiyada).
type Store interface {
	Create(ctx context.Context, s *Session, refreshHash []byte) error
	// Rotate — `oldHash` ni `newHash` ga almashtiradi va idle muddatni
	// `min(now + idleFor(client_id), abs_expires_at)` ga uzaytiradi (o'sha
	// tranzaksiya ichida, sessiyaning O'Z klienti bo'yicha). Eski hash `prev`
	// ga o'tadi. `oldHash` faqat `prev` da topilsa: `last_used_at`dan `grace`
	// o'tmagan bo'lsa RotateRaced, aks holda sessiya `refresh_reuse` sababi
	// bilan bekor qilinadi (RotateReused). `idleFor` 0 qaytarsa (klient
	// o'chirilgan) — RotateExpired va sessiya bekor qilinadi.
	Rotate(ctx context.Context, oldHash, newHash []byte, now time.Time,
		idleFor func(clientID string) time.Duration, grace time.Duration) (*Session, RotateResult, error)
	Get(ctx context.Context, id string) (*Session, error)
	ListActive(ctx context.Context, userID string, now time.Time) ([]*Session, error)
	Revoke(ctx context.Context, id, reason string, now time.Time) error
	RevokeAllForUser(ctx context.Context, userID, reason string, now time.Time) (int, error)
	// RevokeChildren — `parentID` dan ochilgan faol sessiyalarni yopadi va
	// yopilganlarini qaytaradi (back-channel logout yuborish uchun).
	RevokeChildren(ctx context.Context, parentID, reason string, now time.Time) ([]*Session, error)
}

// UserReader — sessiya egasini tekshirish uchun (o'chirilgan akkaunt,
// realm o'zgarishi).
type UserReader interface {
	GetByID(ctx context.Context, id string) (*users.User, error)
}

// Meta — sessiya ochilgan/yangilangan qurilma haqida.
type Meta struct {
	IP         string
	UserAgent  string
	DeviceName string
}

// Tokens — klientga beriladigan juftlik.
type Tokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
	SessionID    string    `json:"session_id"`
}

// Service — markaziy sessiya xizmati.
type Service struct {
	store   Store
	issuer  *Issuer
	users   UserReader
	audit   Auditor
	clients map[string]ClientPolicy
	newID   func() string
	now     func() time.Time
	active  *ttlcache.Cache[bool]
	ipKey   []byte
	// revokeHook — `OnRevoke` (OIDC back-channel logout).
	revokeHook func(ctx context.Context, sess *Session)
	// newDeviceHook — `OnNewDevice` (egasiga "yangi qurilmadan kirildi" xabari).
	newDeviceHook func(ctx context.Context, d NewDevice)
}

// NewDevice — merchant akkauntiga yangi qurilma (brauzer/ilova) dan kirish.
// "Yangi" — shu foydalanuvchining FAOL sessiyalari orasida bunday
// UserAgent yo'q (AUTH-PHASE-A.md: "Windows, Chust, 14:32 — sizmisiz?").
type NewDevice struct {
	UserID     string
	ClientID   string
	DeviceName string
	UserAgent  string
	IP         string
	At         time.Time
}

// OnNewDevice — yangi qurilmadan kirish xabari. Hook ALOHIDA goroutine'da
// chaqiriladi (kirish sekinlashmasin, xabar xatosi kirishni buzmasin);
// konteksti so'rovdan ajratilgan.
func (s *Service) OnNewDevice(fn func(ctx context.Context, d NewDevice)) { s.newDeviceHook = fn }

// NewService — `clients` nil bo'lsa `DefaultClients`.
func NewService(store Store, issuer *Issuer, ur UserReader, audit Auditor,
	newID func() string, clients map[string]ClientPolicy) (*Service, error) {
	if store == nil || issuer == nil || ur == nil || newID == nil {
		return nil, errors.New("identity: store, issuer, users va newID majburiy")
	}
	if audit == nil {
		audit = NopAuditor{}
	}
	if clients == nil {
		clients = DefaultClients
	}
	for id, p := range clients {
		if !p.Realm.Valid() || p.Idle <= 0 || p.Abs < p.Idle {
			return nil, errors.New("identity: klient siyosati noto'g'ri: " + id)
		}
	}
	return &Service{
		store: store, issuer: issuer, users: ur, audit: audit, clients: clients,
		newID: newID, now: time.Now,
		active: ttlcache.New[bool](activeCacheTTL, 50_000),
		ipKey:  issuer.keys.derivedSecret("ip"),
	}, nil
}

// Issuer — tokenlarni tekshirish uchun (middleware, JWKS).
func (s *Service) Issuer() *Issuer { return s.issuer }

// WithClock — soatni almashtiradi (testlar: muddat, grace oynasi). Servis va
// token chiqaruvchi BIR soatda yuradi.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	s.issuer.now = now
	return s
}

// newRefresh — 256-bit tasodifiy refresh token va uning sha256 xeshi.
// `ort_` prefiksi loglar/sir skanerlarida (gitleaks) tanib olish uchun.
func newRefresh() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	tok := "ort_" + base64.RawURLEncoding.EncodeToString(b)
	return tok, hashRefresh(tok), nil
}

func hashRefresh(tok string) []byte {
	sum := sha256.Sum256([]byte(tok))
	return sum[:]
}

func (s *Service) ipHash(ip string) []byte {
	if ip == "" {
		return nil
	}
	m := hmac.New(sha256.New, s.ipKey)
	m.Write([]byte(ip))
	return m.Sum(nil)
}

// clip — foydalanuvchi bergan matnni bazadagi CHECK chegarasiga sig'diradi
// (rune bo'yicha, yarim harf qolmasin).
func clip(v string, max int) string {
	v = strings.TrimSpace(v)
	if utf8.RuneCountInString(v) <= max {
		return v
	}
	r := []rune(v)
	return string(r[:max])
}

// CreateInput — yangi sessiya (A3 kirish oqimlari chaqiradi, kimlik
// ALLAQACHON tekshirilgan bo'lishi SHART).
type CreateInput struct {
	UserID   string
	ClientID string
	OrgID    string
	AMR      []string // pwd | otp | totp
	Meta     Meta
	// ParentID — OIDC: kod qaysi SSO sessiyasidan berilgan (bo'sh — yo'q).
	ParentID string
}

// Create — sessiya ochadi va token juftligini qaytaradi.
//
// Realm akkauntdan O'QILADI (chaqiruvchidan olinmaydi) va klient realmi
// bilan mos kelishi SHART: mijoz akkaunti merchant ilovasida sessiya
// ocholmaydi.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Tokens, error) {
	pol, ok := s.clients[in.ClientID]
	if !ok {
		return nil, ErrUnknownClient
	}
	u, err := s.users.GetByID(ctx, in.UserID)
	if err != nil {
		return nil, err
	}
	if u.IsDeleted() {
		return nil, users.ErrAccountDeleted
	}
	realm := u.Realm
	if realm == "" {
		realm = users.RealmForRole(u.Role)
	}
	if realm != pol.Realm {
		s.audit.Record(ctx, Event{Action: "session.create", Result: ResultDenied,
			ActorUserID: u.ID, Realm: string(realm), Target: in.ClientID, Details: map[string]any{"reason": "realm_mismatch"}})
		return nil, ErrRealmMismatch
	}

	if realm == users.RealmPlatform && s.issuer.platform == nil {
		return nil, ErrPlatformKeyMissing // sessiya yozuvi yetim qolmasin
	}
	refresh, hash, err := newRefresh()
	if err != nil {
		return nil, err
	}
	now := s.now()
	sess := &Session{
		ID: s.newID(), UserID: u.ID, Realm: realm, ClientID: in.ClientID, OrgID: in.OrgID,
		AMR: in.AMR, CreatedAt: now, LastUsedAt: now,
		IdleExpiresAt: now.Add(pol.Idle), AbsExpiresAt: now.Add(pol.Abs),
		IPHash: s.ipHash(in.Meta.IP), UserAgent: clip(in.Meta.UserAgent, 300), DeviceName: clip(in.Meta.DeviceName, 100),
		ParentID: in.ParentID,
	}
	for _, a := range in.AMR {
		// tiklash kodi — TOTP o'rnini bosuvchi; hwk — passkey (AUD-4) ham ikkinchi bosqich.
		if a == "totp" || a == "recovery" || a == "hwk" {
			t := now
			sess.MFAAt = &t
		}
	}
	// Yangi qurilma: faqat merchant realmi, faqat to'g'ridan-to'g'ri kirish
	// (OIDC bola sessiyasi — ota SSO sessiyasi allaqachon xabar bergan).
	newDevice := false
	if s.newDeviceHook != nil && (realm == users.RealmMerchant || realm == users.RealmPlatform) && in.ParentID == "" {
		newDevice = true
		if list, lerr := s.store.ListActive(ctx, u.ID, now); lerr == nil {
			for _, x := range list {
				if x.UserAgent == sess.UserAgent {
					newDevice = false
					break
				}
			}
		}
	}
	if err := s.store.Create(ctx, sess, hash); err != nil {
		return nil, err
	}
	access, exp, err := s.issuer.issue(sess, sess.AbsExpiresAt)
	if err != nil {
		return nil, err
	}
	if newDevice {
		d := NewDevice{UserID: u.ID, ClientID: in.ClientID, DeviceName: sess.DeviceName,
			UserAgent: sess.UserAgent, IP: in.Meta.IP, At: now}
		hook := s.newDeviceHook
		go hook(context.WithoutCancel(ctx), d)
	}
	s.audit.Record(ctx, Event{Action: "session.create", Result: ResultOK, ActorUserID: u.ID,
		Realm: string(realm), OrgID: in.OrgID, Target: sess.ID, IPHash: sess.IPHash, UserAgent: sess.UserAgent,
		Details: map[string]any{"client_id": in.ClientID, "amr": in.AMR}})
	return &Tokens{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresAt: exp, SessionID: sess.ID}, nil
}

// Refresh — refresh tokenni almashtiradi (rotatsiya).
//
// Har muvaffaqiyatli chaqiriqda ESKI refresh yaroqsiz bo'ladi. Eski refresh
// qayta kelsa (o'g'irlangan nusxa) — sessiya bekor qilinadi.
// Akkaunt o'chirilgan yoki realmi o'zgargan bo'lsa ham sessiya yopiladi.
func (s *Service) Refresh(ctx context.Context, refresh string, meta Meta) (*Tokens, error) {
	if !strings.HasPrefix(refresh, "ort_") || len(refresh) > 128 {
		return nil, ErrRefreshInvalid
	}
	now := s.now()
	newTok, newHash, err := newRefresh()
	if err != nil {
		return nil, err
	}
	sess, res, err := s.store.Rotate(ctx, hashRefresh(refresh), newHash, now, s.idleFor, refreshRaceGrace)
	if err != nil {
		return nil, err
	}
	switch res {
	case RotateOK:
	case RotateReused:
		s.active.Invalidate(sess.ID)
		s.notifyRevoked(ctx, sess)
		s.audit.Record(ctx, Event{Action: "session.refresh_reuse", Result: ResultDenied, ActorUserID: sess.UserID,
			Realm: string(sess.Realm), Target: sess.ID, IPHash: s.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
		slog.Warn("identity: refresh qayta ishlatildi — sessiya bekor qilindi", "session", sess.ID, "user", sess.UserID)
		return nil, ErrRefreshReuse
	default:
		return nil, ErrRefreshInvalid
	}

	u, err := s.users.GetByID(ctx, sess.UserID)
	if err != nil || u.IsDeleted() {
		_ = s.revoke(ctx, sess, "account_gone")
		return nil, ErrRefreshInvalid
	}
	realm := u.Realm
	if realm == "" {
		realm = users.RealmForRole(u.Role)
	}
	if realm != sess.Realm {
		_ = s.revoke(ctx, sess, "realm_changed")
		return nil, ErrRefreshInvalid
	}

	access, exp, err := s.issuer.issue(sess, sess.AbsExpiresAt)
	if err != nil {
		return nil, err
	}
	return &Tokens{AccessToken: access, RefreshToken: newTok, TokenType: "Bearer", ExpiresAt: exp, SessionID: sess.ID}, nil
}

// RefreshForClient — `Refresh` + sessiya SHU klientga tegishliligi (OIDC
// /token). Boshqa klientning refresh tokeni kelsa — o'g'irlik belgisi:
// sessiya yopiladi (rotatsiya allaqachon bo'lgan, egasi baribir qayta kiradi).
func (s *Service) RefreshForClient(ctx context.Context, refresh, clientID string, meta Meta) (*Tokens, error) {
	tok, err := s.Refresh(ctx, refresh, meta)
	if err != nil {
		return nil, err
	}
	sess, err := s.store.Get(ctx, tok.SessionID)
	if err != nil {
		return nil, ErrRefreshInvalid
	}
	if sess.ClientID != clientID {
		_ = s.revoke(ctx, sess, "refresh_wrong_client")
		return nil, ErrRefreshInvalid
	}
	return tok, nil
}

// RefreshPublic — umumiy `/v2/auth/token/refresh` (mobil/desktop ilovalar,
// mijoz BFF). `NoPublicRefresh` klient sessiyasining tokeni bu yo'ldan kelsa
// — o'g'irlik belgisi (halol klient uni hech qachon bu yerga yubormaydi):
// sessiya yopiladi.
func (s *Service) RefreshPublic(ctx context.Context, refresh string, meta Meta) (*Tokens, error) {
	tok, err := s.Refresh(ctx, refresh, meta)
	if err != nil {
		return nil, err
	}
	sess, err := s.store.Get(ctx, tok.SessionID)
	if err != nil {
		return nil, ErrRefreshInvalid
	}
	if s.clients[sess.ClientID].NoPublicRefresh {
		_, _ = s.revokeTree(ctx, sess, "refresh_public_forbidden")
		return nil, ErrRefreshInvalid
	}
	return tok, nil
}
// idleFor — klientning harakatsizlik chegarasi; noma'lum klient — 0
// (Store sessiyani yopadi).
func (s *Service) idleFor(clientID string) time.Duration {
	return s.clients[clientID].Idle
}

// OnRevoke — sessiya markazda yopilganda chaqiriladi (OIDC back-channel
// logout). Bitta kuzatuvchi; `nil` — o'chiq. Chaqiruv bloklamasligi SHART.
func (s *Service) OnRevoke(fn func(ctx context.Context, sess *Session)) { s.revokeHook = fn }

func (s *Service) notifyRevoked(ctx context.Context, sess *Session) {
	if s.revokeHook != nil && sess != nil {
		s.revokeHook(ctx, sess)
	}
}

func (s *Service) revoke(ctx context.Context, sess *Session, reason string) error {
	err := s.store.Revoke(ctx, sess.ID, reason, s.now())
	s.active.Invalidate(sess.ID)
	s.notifyRevoked(ctx, sess)
	s.audit.Record(ctx, Event{Action: "session.revoke", Result: ResultOK, ActorUserID: sess.UserID,
		Realm: string(sess.Realm), Target: sess.ID, Details: map[string]any{"reason": reason}})
	return err
}

// revokeTree — sessiyani va uning BOLALARINI (OIDC: SSO'dan ochilgan ilova
// sessiyalari) yopadi. Yopilgan bolalar qaytariladi.
func (s *Service) revokeTree(ctx context.Context, sess *Session, reason string) ([]*Session, error) {
	err := s.revoke(ctx, sess, reason)
	kids, kerr := s.store.RevokeChildren(ctx, sess.ID, reason, s.now())
	for _, k := range kids {
		s.active.Invalidate(k.ID)
		s.notifyRevoked(ctx, k)
		s.audit.Record(ctx, Event{Action: "session.revoke", Result: ResultOK, ActorUserID: k.UserID,
			Realm: string(k.Realm), Target: k.ID, Details: map[string]any{"reason": reason, "parent": sess.ID}})
	}
	if err == nil {
		err = kerr
	}
	return kids, err
}

// Authenticate — access tokenni tekshiradi VA sessiya markazda hali
// faolligini (30 s kesh bilan) tasdiqlaydi.
func (s *Service) Authenticate(ctx context.Context, raw string, realms ...users.Realm) (*AccessClaims, error) {
	c, err := s.issuer.Verify(raw, realms...)
	if err != nil {
		return nil, err
	}
	ok, err := s.active.Get(c.SessionID, func() (bool, error) {
		sess, err := s.store.Get(ctx, c.SessionID)
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return sess.Active(s.now()) && sess.UserID == c.Subject && sess.Realm == c.Realm, nil
	})
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrTokenInvalid
	}
	return c, nil
}

// Logout — joriy sessiyani yopadi.
func (s *Service) Logout(ctx context.Context, c *AccessClaims) error {
	sess, err := s.store.Get(ctx, c.SessionID)
	if err != nil {
		return err
	}
	if sess.UserID != c.Subject {
		return ErrNotFound
	}
	return s.revoke(ctx, sess, "logout")
}

// ListSessions — foydalanuvchining faol sessiyalari ("Qurilmalar" sahifasi).
func (s *Service) ListSessions(ctx context.Context, userID string) ([]*Session, error) {
	return s.store.ListActive(ctx, userID, s.now())
}

// RevokeOwn — foydalanuvchi o'z sessiyasini (boshqa qurilma) yopadi.
// Begona sessiya — `ErrNotFound` (mavjudligi oshkor bo'lmaydi).
func (s *Service) RevokeOwn(ctx context.Context, userID, sessionID string) error {
	sess, err := s.store.Get(ctx, sessionID)
	if err != nil || sess.UserID != userID || sess.RevokedAt != nil {
		return ErrNotFound
	}
	return s.revoke(ctx, sess, "user_revoked")
}

// RevokeAllForUser — parol almashtirilganda, xodim bo'shatilganda, akkaunt
// bloklanganda: foydalanuvchining BARCHA sessiyalari.
func (s *Service) RevokeAllForUser(ctx context.Context, userID, reason string) (int, error) {
	list, _ := s.store.ListActive(ctx, userID, s.now())
	n, err := s.store.RevokeAllForUser(ctx, userID, reason, s.now())
	for _, x := range list {
		s.active.Invalidate(x.ID)
		s.notifyRevoked(ctx, x)
	}
	s.audit.Record(ctx, Event{Action: "session.revoke_all", Result: ResultOK, ActorUserID: userID,
		Target: userID, Details: map[string]any{"reason": reason, "count": n}})
	return n, err
}
