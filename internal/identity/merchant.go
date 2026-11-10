package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/identity/passkey"
	"chustapp/internal/users"
)

// Merchant kirish oqimlari (A3, AUTH-PHASE-A.md 5-bo'lim; egasi qarorlari):
//
//	EGASI:  telefon + parol  ─┐
//	        telefon + kod    ─┴─> TOTP (MAJBURIY; ulanmagan bo'lsa avval ulash) -> sessiya
//	XODIM:  telefon + kod (FAQAT; parol bilan kirmaydi) -> a'zolik shu ilovaga
//	        ruxsat bersa -> sessiya
//
// Birinchi bosqichdan keyin sessiya OCHILMAYDI: qisqa muddatli `mfa_token`
// beriladi (HMAC imzoli, holatsiz). Uni faqat ikkinchi bosqich almashtiradi.
//
// Kod yetkazish — mavjud kanallar (`/auth/telegram/start`, `/auth/request-code`):
// ular faqat telefon egaligini isbotlaydi, akkaunt ochmaydi.

var (
	// ErrMFAInvalid — TOTP yoki tiklash kodi noto'g'ri / allaqachon ishlatilgan.
	ErrMFAInvalid = errors.New("tasdiqlash kodi noto'g'ri")
	// ErrMFATokenInvalid — `mfa_token` buzilgan yoki muddati o'tgan.
	ErrMFATokenInvalid = errors.New("tasdiqlash muddati tugadi — qaytadan kiring")
	// ErrMFATooMany — urinishlar ko'p (akkaunt bo'yicha).
	ErrMFATooMany = errors.New("juda ko'p urinish — birozdan keyin qayta urining")
	// ErrAppNotAllowed — akkaunt bu ilovaga kira olmaydi.
	ErrAppNotAllowed = errors.New("bu ilovaga kirish ruxsati yo'q — biznes egasiga murojaat qiling")
)

const (
	mfaPurposeVerify = "totp"
	mfaPurposeEnroll = "enroll"
	mfaPurposeFactor = "factor" // TOTP ulashdan oldin ikkinchi dalil (parol/kod)
	mfaVerifyTTL     = 5 * time.Minute
	mfaEnrollTTL     = 10 * time.Minute // ilovani o'rnatish va QR'ni skanerlashga vaqt
	mfaTokenPrefix   = "omfa_"
	mfaMaxFailures   = 5
	mfaFailWindow    = 15 * time.Minute
)

// MerchantUsers — `users.Service` ning tekshiruv funksiyalari (token bermaydi).
type MerchantUsers interface {
	AuthenticatePasswordRealm(ctx context.Context, realm users.Realm, phone, password string) (*users.User, error)
	VerifyPhoneCode(ctx context.Context, rawPhone, code string) (string, error)
	// SetPassword — `users.Service.SetPassword` (kuchlilik qoidalari, Argon2id).
	SetPassword(ctx context.Context, userID, current, password, confirm string, phoneProven bool) error
}

// Sealer — TOTP kalitini shifrlash (`secretstore.Box`).
type Sealer interface {
	Seal(plain string) (string, error)
	Open(enc string) (string, error)
}

// MerchantDeps — `NewMerchantAuth` bog'liqliklari (hammasi majburiy).
type MerchantDeps struct {
	Users MerchantUsers
	// Repo — akkauntlar (realm bo'yicha qidiruv, yangi merchant akkaunti yaratish).
	Repo  users.Repository
	Authz *authz.Authorizer
	MFA   mfa.Store
	Box   Sealer
	// TOTPIssuer — autentifikator ilovasida ko'rinadigan nom.
	TOTPIssuer string
	// Passkeys — passkey (WebAuthn) ombori; nil = passkey o'chiq (AUD-4).
	Passkeys passkey.Store
	// Passkey — RP sozlamasi (bo'sh = issuer'dan).
	Passkey PasskeyConfig
}

// MerchantAuth — merchant realmiga kirish.
type MerchantAuth struct {
	svc      *Service
	d        MerchantDeps
	tokenKey []byte
	guard    *failGuard
	// inviteHook — `OnInvite` (xodimga "sizni taklif qilishdi" xabari).
	inviteHook func(ctx context.Context, inv Invite)
	// wa, pkKey — passkey (`initPasskeys`); wa nil = o'chiq.
	wa    *webauthn.WebAuthn
	pkKey []byte
}

// Invite — xodim taklifi (yangi yoki qayta) xabari uchun.
type Invite struct {
	UserID       string
	Phone        string
	BusinessName string
	RoleName     string
	Apps         []string
}

// OnInvite — taklif yuborilganda (AddStaff, ReinviteStaff). Hook alohida
// goroutine'da: xabar xatosi taklifni buzmaydi.
func (m *MerchantAuth) OnInvite(fn func(ctx context.Context, inv Invite)) { m.inviteHook = fn }

func (m *MerchantAuth) notifyInvite(ctx context.Context, mb *authz.Membership, bizName string) {
	if m.inviteHook == nil || mb == nil {
		return
	}
	u, err := m.d.Repo.GetByID(ctx, mb.UserID)
	if err != nil {
		return
	}
	inv := Invite{UserID: u.ID, Phone: u.Phone, BusinessName: bizName, Apps: slices.Clone(mb.Apps)}
	if r, err := m.d.Authz.Store().GetRole(ctx, mb.RoleID); err == nil {
		inv.RoleName = r.Name
	}
	hook := m.inviteHook
	go hook(context.WithoutCancel(ctx), inv)
}

func NewMerchantAuth(svc *Service, d MerchantDeps) (*MerchantAuth, error) {
	if svc == nil || d.Users == nil || d.Repo == nil || d.Authz == nil || d.MFA == nil || d.Box == nil {
		return nil, errors.New("identity: merchant kirishi uchun barcha bog'liqliklar majburiy")
	}
	if d.TOTPIssuer == "" {
		d.TOTPIssuer = "OnDex Merchant"
	}
	m := &MerchantAuth{
		svc: svc, d: d,
		tokenKey: svc.issuer.keys.derivedSecret("mfa-token"),
		// akkaunt bo'yicha: 15 daqiqada 5 ta XATO -> 15 daqiqa blok (10^6 kod
		// maydonini taxmin qilish amalda imkonsiz). Muvaffaqiyat sanalmaydi.
		guard: newFailGuard(mfaMaxFailures, mfaFailWindow, func() time.Time { return svc.now() }),
	}
	if err := m.initPasskeys(); err != nil {
		return nil, err
	}
	return m, nil
}

// Challenge — ikkinchi bosqich talabi.
type Challenge struct {
	Token     string    `json:"mfa_token"`
	Method    string    `json:"method"` // "totp" | "totp_enroll" | "otp" | "password"
	ExpiresAt time.Time `json:"expires_at"`
	// Passkey — method=totp bosqichida passkey ham ishlatsa bo'ladi (AUD-4).
	Passkey bool `json:"passkey,omitempty"`
}

// Outcome — birinchi bosqich natijasi: sessiya YOKI ikkinchi bosqich.
type Outcome struct {
	Tokens *Tokens
	MFA    *Challenge
}

// Enrollment — TOTP ulash ma'lumoti (QR uchun). Kalit faqat shu javobda.
type Enrollment struct {
	Secret string `json:"secret"`
	URI    string `json:"otpauth_uri"`
}

type accountKind int

const (
	kindNone accountKind = iota
	kindOwner
	kindStaff
)

// classify — merchant akkaunti egami yoki xodimmi.
//
// KO'PRIK (A5 da olib tashlanadi): a'zoliklar hali ko'chirilmagan eski
// akkauntlar roldan aniqlanadi — restaurant/store = egasi, waiter/courier =
// xodim (ofitsiant -> OnDexPro, kuryer -> OnDexGO).
func (m *MerchantAuth) classify(ctx context.Context, u *users.User, app string) (accountKind, string, error) {
	realm := u.Realm
	if realm == "" {
		realm = users.RealmForRole(u.Role)
	}
	if realm != users.RealmMerchant || u.IsDeleted() {
		return kindNone, "", nil
	}
	list, err := m.d.Authz.List(ctx, u.ID)
	if err != nil {
		return kindNone, "", err
	}
	for _, a := range list {
		if a.IsOwner {
			return kindOwner, a.Business.OrgID, nil
		}
	}
	for _, a := range list {
		if a.AllowsApp(app) {
			return kindStaff, a.Business.OrgID, nil
		}
	}
	// A'zoligi bor akkauntda FAQAT a'zolik hal qiladi — eski rol ko'prigi
	// uni chetlab o'tmasin (egasi "faqat panel" degan xodim OnDexPro'ga kirmasin).
	if len(list) > 0 {
		return kindNone, "", ErrAppNotAllowed
	}
	switch u.Role {
	case users.RoleRestaurant, users.RoleStore:
		return kindOwner, "", nil
	case users.RoleWaiter:
		if app == authz.AppPro {
			return kindStaff, "", nil
		}
		return kindNone, "", ErrAppNotAllowed
	case users.RoleCourier:
		if app == authz.AppGo {
			return kindStaff, "", nil
		}
		return kindNone, "", ErrAppNotAllowed
	}
	return kindNone, "", nil
}

func (m *MerchantAuth) appFor(clientID string) (string, error) {
	pol, ok := m.svc.clients[clientID]
	app := authz.AppForClient(clientID)
	if !ok || pol.Realm != users.RealmMerchant || app == "" {
		return "", ErrUnknownClient
	}
	return app, nil
}

func (m *MerchantAuth) deny(ctx context.Context, userID, clientID, reason string, meta Meta) {
	m.svc.audit.Record(ctx, Event{Action: "merchant.login", Result: ResultDenied, ActorUserID: userID,
		Realm: string(users.RealmMerchant), Target: clientID, IPHash: m.svc.ipHash(meta.IP),
		UserAgent: clip(meta.UserAgent, 300), Details: map[string]any{"reason": reason}})
}

// LoginPassword — FAQAT egasi uchun. Xodim, mijoz, yo'q akkaunt — bir xil
// `users.ErrInvalidCredentials` (kim ro'yxatda ekani oshkor bo'lmaydi).
func (m *MerchantAuth) LoginPassword(ctx context.Context, phone, password, clientID string, meta Meta) (*Outcome, error) {
	app, err := m.appFor(clientID)
	if err != nil {
		return nil, err
	}
	u, err := m.d.Users.AuthenticatePasswordRealm(ctx, users.RealmMerchant, phone, password)
	if err != nil {
		return nil, err
	}
	kind, org, err := m.classify(ctx, u, app)
	if err != nil && !errors.Is(err, ErrAppNotAllowed) {
		return nil, err
	}
	if kind != kindOwner {
		m.deny(ctx, u.ID, clientID, "password_not_owner", meta)
		return nil, users.ErrInvalidCredentials
	}
	return m.secondFactor(ctx, u, clientID, org, []string{"pwd"})
}

// LoginCode — telefon kodi bilan (xodim; egasi uchun keyin TOTP).
func (m *MerchantAuth) LoginCode(ctx context.Context, phone, code, clientID string, meta Meta) (*Outcome, error) {
	app, err := m.appFor(clientID)
	if err != nil {
		return nil, err
	}
	p, err := m.d.Users.VerifyPhoneCode(ctx, phone, code)
	if err != nil {
		return nil, err
	}
	u, err := m.d.Repo.GetByPhoneRealm(ctx, users.RealmMerchant, p)
	if err != nil {
		// Kod to'g'ri, lekin bu raqamda merchant akkaunti yo'q.
		return nil, users.ErrInvalidCredentials
	}
	// Telefon egaligi hozirgina isbotlandi — kutayotgan takliflar faollashadi.
	m.activateInvites(ctx, u, meta)
	kind, org, err := m.classify(ctx, u, app)
	switch {
	case errors.Is(err, ErrAppNotAllowed):
		m.deny(ctx, u.ID, clientID, "app_not_allowed", meta)
		return nil, ErrAppNotAllowed
	case err != nil:
		return nil, err
	}
	switch kind {
	case kindOwner:
		return m.secondFactor(ctx, u, clientID, org, []string{"otp"})
	case kindStaff:
		tok, err := m.svc.Create(ctx, CreateInput{UserID: u.ID, ClientID: clientID, OrgID: org, AMR: []string{"otp"}, Meta: meta})
		if err != nil {
			return nil, err
		}
		return &Outcome{Tokens: tok}, nil
	}
	m.deny(ctx, u.ID, clientID, "not_merchant", meta)
	return nil, users.ErrInvalidCredentials
}

// secondFactor — egasi uchun keyingi bosqich.
//
// TOTP hali ULANMAGAN bo'lsa, ulashdan oldin IKKALA dalil (parol VA telefon
// kodi) talab qilinadi. Aks holda faqat parolni bilgan hujumchi o'z
// TOTP'ini ulab, akkauntni butunlay egallab olardi (egasining o'zi keyin
// kira olmasdi).
func (m *MerchantAuth) secondFactor(ctx context.Context, u *users.User, clientID, org string, amr []string) (*Outcome, error) {
	purpose, method, ttl := mfaPurposeVerify, "totp", mfaVerifyTTL
	if _, err := m.d.MFA.Confirmed(ctx, u.ID); errors.Is(err, mfa.ErrNotFound) {
		switch {
		case !slices.Contains(amr, "otp"):
			purpose, method = mfaPurposeFactor, "otp"
		case !slices.Contains(amr, "pwd") && u.PasswordHash == "":
			// Parolsiz egasi (A3'gacha admin parolsiz yaratgan, faqat kod bilan
			// kirgan akkaunt): parol SO'RALSA boshi berk ko'cha edi (2026-11-08,
			// Book Cafe). Telefon egaligi hozirgina kod bilan isbotlangan —
			// `/me/password` qoidasi bilan bir xil: shu yerda parol O'RNATILADI.
			purpose, method = mfaPurposeFactor, "password_set"
		case !slices.Contains(amr, "pwd"):
			purpose, method = mfaPurposeFactor, "password"
		default:
			purpose, method, ttl = mfaPurposeEnroll, "totp_enroll", mfaEnrollTTL
		}
	} else if err != nil {
		return nil, err
	}
	tok, exp, err := m.signMFA(mfaClaims{User: u.ID, Client: clientID, Org: org, AMR: amr, Purpose: purpose}, ttl)
	if err != nil {
		return nil, err
	}
	return &Outcome{MFA: &Challenge{Token: tok, Method: method, ExpiresAt: exp,
		Passkey: purpose == mfaPurposeVerify && m.hasPasskey(ctx, u.ID)}}, nil
}

// AddFactor — TOTP ulashdan oldingi YETISHMAYOTGAN dalil: telefon kodi
// (parol bilan kirganda) yoki parol (kod bilan kirganda). Telefon va
// akkaunt `mfa_token` dan olinadi — so'rovdan EMAS.
func (m *MerchantAuth) AddFactor(ctx context.Context, mfaToken, secret string, meta Meta) (*Outcome, error) {
	c, err := m.parseMFA(mfaToken, mfaPurposeFactor)
	if err != nil {
		return nil, err
	}
	if m.guard.Blocked(c.User) {
		return nil, ErrMFATooMany
	}
	u, err := m.svc.users.GetByID(ctx, c.User)
	if err != nil || u.Phone == "" {
		return nil, ErrMFATokenInvalid
	}
	var add string
	if !slices.Contains(c.AMR, "otp") {
		if _, err := m.d.Users.VerifyPhoneCode(ctx, u.Phone, secret); err != nil {
			m.mfaFailed(ctx, c, meta)
			return nil, err
		}
		add = "otp"
	} else {
		// Akkauntning O'Z realmi (merchant yoki admin — A6): boshqa realmdagi
		// shu raqamli akkaunt paroli o'tmasin.
		pu, err := m.d.Users.AuthenticatePasswordRealm(ctx, users.RealmForRole(u.Role), u.Phone, secret)
		if err != nil || pu.ID != u.ID {
			m.mfaFailed(ctx, c, meta)
			return nil, users.ErrInvalidCredentials
		}
		add = "pwd"
	}
	m.guard.Reset(c.User)
	return m.secondFactor(ctx, u, c.Client, c.Org, append(slices.Clone(c.AMR), add))
}

// SetInitialPassword — parolsiz egasi kod bilan kirgach parol o'rnatadi
// (`password_set` bosqichi). Faqat: mfa_token telefon kodi bilan olingan
// (amr = otp), akkauntda parol HALI YO'Q. Parolli akkauntda bu yo'l yopiq —
// aks holda kod egasi (SIM almashtirish) parolni almashtirib qo'ya olardi.
func (m *MerchantAuth) SetInitialPassword(ctx context.Context, mfaToken, password, confirm string, meta Meta) (*Outcome, error) {
	c, err := m.parseMFA(mfaToken, mfaPurposeFactor)
	if err != nil {
		return nil, err
	}
	if m.guard.Blocked(c.User) {
		return nil, ErrMFATooMany
	}
	if !slices.Contains(c.AMR, "otp") || slices.Contains(c.AMR, "pwd") {
		return nil, ErrMFATokenInvalid
	}
	u, err := m.svc.users.GetByID(ctx, c.User)
	if err != nil || u.PasswordHash != "" || u.IsDeleted() {
		return nil, ErrMFATokenInvalid
	}
	if err := m.d.Users.SetPassword(ctx, u.ID, "", password, confirm, true); err != nil {
		return nil, err // zaif parol / mos emas — foydalanuvchiga ko'rinadigan xato
	}
	m.svc.audit.Record(ctx, Event{Action: "merchant.password_set", Result: ResultOK, ActorUserID: u.ID,
		Realm: string(users.RealmMerchant), Target: c.Client, IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
	m.guard.Reset(c.User)
	return m.secondFactor(ctx, u, c.Client, c.Org, append(slices.Clone(c.AMR), "pwd"))
}

// VerifyMFA — TOTP yoki tiklash kodi; to'g'ri bo'lsa sessiya ochiladi.
func (m *MerchantAuth) VerifyMFA(ctx context.Context, mfaToken, code string, meta Meta) (*Tokens, error) {
	c, err := m.parseMFA(mfaToken, mfaPurposeVerify)
	if err != nil {
		return nil, err
	}
	if m.guard.Blocked(c.User) {
		return nil, ErrMFATooMany
	}
	factor, err := m.checkFactorCode(ctx, c.User, code)
	switch {
	case errors.Is(err, mfa.ErrNotFound):
		return nil, ErrMFATokenInvalid // faktor shu orada o'chirilgan
	case errors.Is(err, ErrMFAInvalid):
		m.mfaFailed(ctx, c, meta)
		return nil, ErrMFAInvalid
	case err != nil:
		return nil, err
	}
	m.guard.Reset(c.User)
	return m.svc.Create(ctx, CreateInput{UserID: c.User, ClientID: c.Client, OrgID: c.Org,
		AMR: append(slices.Clone(c.AMR), factor), Meta: meta})
}

// checkFactorCode — foydalanuvchining tasdiqlangan TOTP kodi yoki tiklash
// kodi (bir martalik: qadam/kod qayta o'tmaydi). Natija — "totp" yoki
// "recovery"; noto'g'ri kod — ErrMFAInvalid; faktor yo'q — mfa.ErrNotFound.
// Urinishlar hisobi (`guard`) CHAQIRUVCHIDA.
func (m *MerchantAuth) checkFactorCode(ctx context.Context, userID, code string) (string, error) {
	f, err := m.d.MFA.Confirmed(ctx, userID)
	if err != nil {
		return "", err
	}
	if mfa.LooksLikeRecovery(code) {
		ok, err := m.d.MFA.UseRecovery(ctx, userID, mfa.HashRecovery(code), m.svc.now())
		if err != nil {
			return "", err
		}
		if !ok {
			return "", ErrMFAInvalid
		}
		left, _ := m.d.MFA.RecoveryLeft(ctx, userID)
		slog.Warn("identity: tiklash kodi ishlatildi", "user", userID, "qolgan", left)
		return "recovery", nil
	}
	secret, err := m.d.Box.Open(f.SecretEnc)
	if err != nil {
		return "", err
	}
	step, ok := mfa.Validate(secret, code, m.svc.now())
	if ok {
		if ok, err = m.d.MFA.UseStep(ctx, f.ID, step); err != nil {
			return "", err
		}
	}
	if !ok {
		return "", ErrMFAInvalid
	}
	return "totp", nil
}

func (m *MerchantAuth) mfaFailed(ctx context.Context, c *mfaClaims, meta Meta) {
	m.guard.Fail(c.User)
	m.svc.audit.Record(ctx, Event{Action: "mfa.verify", Result: ResultDenied, ActorUserID: c.User,
		Realm: string(users.RealmMerchant), Target: c.Client, IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
}

// EnrollStart — TOTP kalitini yaratadi (tasdiqlanmagan holda saqlanadi).
// Qayta chaqirilsa yangi kalit (eski tasdiqlanmagani o'chadi).
func (m *MerchantAuth) EnrollStart(ctx context.Context, mfaToken string) (*Enrollment, error) {
	c, err := m.parseMFA(mfaToken, mfaPurposeEnroll)
	if err != nil {
		return nil, err
	}
	// Autentifikator ilovasidagi yozuv nomi — telefon (bir nechta akkaunt
	// bo'lsa ajratish uchun).
	u, err := m.svc.users.GetByID(ctx, c.User)
	if err != nil {
		return nil, ErrMFATokenInvalid
	}
	account := u.Phone
	secret, err := mfa.GenerateSecret()
	if err != nil {
		return nil, err
	}
	enc, err := m.d.Box.Seal(secret)
	if err != nil {
		return nil, err
	}
	if err := m.d.MFA.SavePending(ctx, &mfa.Factor{ID: m.svc.newID(), UserID: c.User, SecretEnc: enc,
		CreatedAt: m.svc.now()}); err != nil {
		if errors.Is(err, mfa.ErrAlreadyEnrolled) {
			return nil, ErrMFATokenInvalid
		}
		return nil, err
	}
	return &Enrollment{Secret: secret, URI: mfa.URI(m.d.TOTPIssuer, account, secret)}, nil
}

// EnrollConfirm — ilovadagi birinchi kod bilan ulashni yakunlaydi, tiklash
// kodlarini (BIR MARTA ko'rsatiladi) va sessiyani qaytaradi.
func (m *MerchantAuth) EnrollConfirm(ctx context.Context, mfaToken, code string, meta Meta) (*Tokens, []string, error) {
	c, err := m.parseMFA(mfaToken, mfaPurposeEnroll)
	if err != nil {
		return nil, nil, err
	}
	if m.guard.Blocked(c.User) {
		return nil, nil, ErrMFATooMany
	}
	f, err := m.d.MFA.Pending(ctx, c.User)
	if err != nil {
		if errors.Is(err, mfa.ErrNotFound) {
			return nil, nil, ErrMFAInvalid
		}
		return nil, nil, err
	}
	secret, err := m.d.Box.Open(f.SecretEnc)
	if err != nil {
		return nil, nil, err
	}
	step, ok := mfa.Validate(secret, code, m.svc.now())
	if !ok {
		m.mfaFailed(ctx, c, meta)
		return nil, nil, ErrMFAInvalid
	}
	plain, hashes, err := mfa.NewRecoveryCodes()
	if err != nil {
		return nil, nil, err
	}
	if err := m.d.MFA.Confirm(ctx, f.ID, step, m.svc.now(), hashes); err != nil {
		if errors.Is(err, mfa.ErrNotFound) || errors.Is(err, mfa.ErrAlreadyEnrolled) {
			return nil, nil, ErrMFATokenInvalid
		}
		return nil, nil, err
	}
	m.svc.audit.Record(ctx, Event{Action: "mfa.enroll", Result: ResultOK, ActorUserID: c.User,
		Realm: string(users.RealmMerchant), Target: f.ID, IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
	m.guard.Reset(c.User)
	tok, err := m.svc.Create(ctx, CreateInput{UserID: c.User, ClientID: c.Client, OrgID: c.Org,
		AMR: append(slices.Clone(c.AMR), "totp"), Meta: meta})
	if err != nil {
		return nil, nil, err
	}
	return tok, plain, nil
}

// ── mfa_token ────────────────────────────────────────────────────────

type mfaClaims struct {
	User    string   `json:"u"`
	Client  string   `json:"c"`
	Org     string   `json:"o,omitempty"`
	AMR     []string `json:"a"`
	Purpose string   `json:"p"`
	Exp     int64    `json:"e"`
	Nonce   string   `json:"n"`
}

func (m *MerchantAuth) mac(payload string) string {
	h := hmac.New(sha256.New, m.tokenKey)
	h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func (m *MerchantAuth) signMFA(c mfaClaims, ttl time.Duration) (string, time.Time, error) {
	n := make([]byte, 12)
	if _, err := rand.Read(n); err != nil {
		return "", time.Time{}, err
	}
	exp := m.svc.now().Add(ttl)
	c.Exp, c.Nonce = exp.Unix(), base64.RawURLEncoding.EncodeToString(n)
	b, err := json.Marshal(c)
	if err != nil {
		return "", time.Time{}, err
	}
	payload := base64.RawURLEncoding.EncodeToString(b)
	return mfaTokenPrefix + payload + "." + m.mac(payload), exp, nil
}

func (m *MerchantAuth) parseMFA(tok, purpose string) (*mfaClaims, error) {
	if len(tok) > 1024 || !strings.HasPrefix(tok, mfaTokenPrefix) {
		return nil, ErrMFATokenInvalid
	}
	payload, sig, ok := strings.Cut(strings.TrimPrefix(tok, mfaTokenPrefix), ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(m.mac(payload))) {
		return nil, ErrMFATokenInvalid
	}
	b, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, ErrMFATokenInvalid
	}
	var c mfaClaims
	if err := json.Unmarshal(b, &c); err != nil || c.Purpose != purpose || c.User == "" ||
		m.svc.now().Unix() >= c.Exp {
		return nil, ErrMFATokenInvalid
	}
	if _, ok := m.svc.clients[c.Client]; !ok {
		return nil, ErrMFATokenInvalid
	}
	return &c, nil
}
