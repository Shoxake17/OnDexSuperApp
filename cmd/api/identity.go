package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/httpapi"
	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/secretstore"
	"chustapp/internal/storage"
	"chustapp/internal/telegram"
	"chustapp/internal/users"
)

// identityAudience — access tokenlarning `aud` qiymati (barcha OnDex API servislari).
const identityAudience = "ondex-api"

// buildIdentity — markaziy sessiya xizmati (A2, `internal/identity`).
//
// A2 BOSQICHIDA IXTIYORIY: kalit berilmasa xizmat o'chiq (`nil`, `/v2/auth/*`
// 503), eski kirish o'zgarishsiz ishlaydi. A3 dan (merchant kirishi) production'da
// MAJBURIY bo'ladi.
//
// Kalit: `AUTH_SIGNING_KEY_FILE` — Ed25519 PKCS#8 PEM (`go run ./cmd/authkey`),
// fayl ruxsati 600, `secrets/` ichida. Rotatsiya: yangi kalit faol qilinadi, eskisi
// `AUTH_PREVIOUS_KEY_FILES` (vergul bilan) ga o'tkaziladi va access token umri
// (10 daqiqa) + zaxira o'tgach olib tashlanadi.
func buildIdentity(devMode bool, pool *pgxpool.Pool, ur users.Repository) *identity.Service {
	var active *identity.SigningKey
	if path := strings.TrimSpace(os.Getenv("AUTH_SIGNING_KEY_FILE")); path != "" {
		k, err := identity.LoadPrivateKeyFile(path)
		if err != nil {
			slog.Error("markaziy sessiya O'CHIQ: imzolash kalitini o'qib bo'lmadi", "err", err)
			return nil
		}
		active = k
	} else if devMode {
		k, err := identity.GenerateKey()
		if err != nil {
			slog.Error("markaziy sessiya O'CHIQ: kalit yaratilmadi", "err", err)
			return nil
		}
		active = k
		slog.Warn("markaziy sessiya: AUTH_SIGNING_KEY_FILE yo'q — DEV uchun VAQTINCHALIK kalit (restartda barcha yangi sessiyalar yaroqsiz)")
	} else {
		slog.Warn("markaziy sessiya O'CHIQ (AUTH_SIGNING_KEY_FILE yo'q) — /v2/auth/* 503 qaytaradi")
		return nil
	}

	var previous []*identity.SigningKey
	for _, p := range strings.Split(os.Getenv("AUTH_PREVIOUS_KEY_FILES"), ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		k, err := identity.LoadPrivateKeyFile(p)
		if err != nil {
			slog.Error("eski imzolash kalitini o'qib bo'lmadi — u bilan imzolangan tokenlar rad etiladi", "file", p, "err", err)
			continue
		}
		previous = append(previous, k)
	}
	keys, err := identity.NewKeySet(active, previous...)
	if err != nil {
		slog.Error("markaziy sessiya O'CHIQ", "err", err)
		return nil
	}

	issuerURL := strings.TrimRight(strings.TrimSpace(os.Getenv("AUTH_ISSUER")), "/")
	if issuerURL == "" {
		issuerURL = strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_API_URL")), "/")
	}
	if issuerURL == "" {
		if !devMode {
			slog.Error("markaziy sessiya O'CHIQ: AUTH_ISSUER (yoki PUBLIC_API_URL) majburiy")
			return nil
		}
		issuerURL = "http://localhost:8080"
	}
	issuer, err := identity.NewIssuer(keys, issuerURL, identityAudience)
	if err != nil {
		slog.Error("markaziy sessiya O'CHIQ", "err", err)
		return nil
	}
	// A6: admin (platform) tokenlari ALOHIDA kalit bilan. Yo'q bo'lsa admin
	// markaziy kirishi o'chiq (eski admin kirishi o'zgarmaydi).
	if ps := loadPlatformKeys(devMode); ps != nil {
		issuer.WithPlatformKeys(ps)
	}

	var store identity.Store
	var audit identity.Auditor
	if pool != nil {
		store, audit = storage.NewPgAuthSessionStore(pool), storage.NewPgAuditLog(pool)
	} else {
		store, audit = identity.NewMemoryStore(), identity.LogAuditor{}
		slog.Warn("markaziy sessiyalar XOTIRADA (DATABASE_URL yo'q) — restartda yo'qoladi")
	}
	svc, err := identity.NewService(store, issuer, ur, audit, httpapi.NewID, nil)
	if err != nil {
		slog.Error("markaziy sessiya O'CHIQ", "err", err)
		return nil
	}
	slog.Info("rejim: markaziy sessiya yoqilgan", "issuer", issuerURL, "kid", active.KID, "eski_kalitlar", len(previous))
	return svc
}

// newDeviceAppNames — xabarda ilova nomi.
var newDeviceAppNames = map[string]string{
	identity.SSOClientID: "OnDex ID (brauzer)",
	"merchant-web":       "OnDex Merchant (web)",
	"merchant-desktop":   "OnDex Merchant (Windows)",
	"ondex-pro":          "OnDexPro",
	"ondex-go":           "OnDexGO",
}

// wireNewDeviceAlerts — merchant akkauntiga yangi qurilmadan kirilganda
// egasining Telegram'iga xabar (Plane 135). Chat — akkauntga bog'langan
// Telegram; bo'lmasa shu raqamdagi mijoz akkauntiniki (bitta odam, bitta
// Telegram). Bog'lanmagan bo'lsa xabar yuborilmaydi (jim, log bilan).
func wireNewDeviceAlerts(svc *identity.Service, tg *telegram.Client, ur users.Repository, issuer string) {
	if svc == nil || tg == nil || !tg.Configured() {
		slog.Warn("yangi qurilma xabarlari O'CHIQ (markaziy sessiya yoki Telegram bot yo'q)")
		return
	}
	loc, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		loc = time.FixedZone("UZT", 5*3600)
	}
	accountURL := ""
	if issuer = strings.TrimRight(strings.TrimSpace(issuer), "/"); strings.HasPrefix(issuer, "https://") {
		accountURL = issuer + "/v2/oauth/account"
	}
	svc.OnNewDevice(func(ctx context.Context, d identity.NewDevice) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		chat := telegramChatFor(ctx, ur, d.UserID)
		if chat == 0 {
			slog.Info("yangi qurilma: Telegram bog'lanmagan — xabar yuborilmadi", "user", d.UserID)
			return
		}
		app := newDeviceAppNames[d.ClientID]
		if app == "" {
			app = d.ClientID
		}
		if err := tg.SendMessage(ctx, chat, identity.NewDeviceText(d, app, accountURL, loc)); err != nil {
			slog.Warn("yangi qurilma xabari yuborilmadi", "user", d.UserID, "err", err)
		}
	})
}

// telegramChatFor — akkauntga bog'langan Telegram chat; bo'lmasa shu
// raqamdagi mijoz akkauntiniki (bitta odam, bitta Telegram). 0 — yo'q.
func telegramChatFor(ctx context.Context, ur users.Repository, userID string) int64 {
	u, err := ur.GetByID(ctx, userID)
	if err != nil {
		return 0
	}
	if u.TelegramID != 0 || u.Phone == "" {
		return u.TelegramID
	}
	if c, err := ur.GetByPhoneRealm(ctx, users.RealmCustomer, u.Phone); err == nil {
		return c.TelegramID
	}
	return 0
}

// wireInviteAlerts — xodim taklifi xodimning Telegram'iga (Plane 135).
// Telegram bog'lanmagan xodimga egasi o'zi aytadi (panelda taklif holati).
func wireInviteAlerts(m *identity.MerchantAuth, tg *telegram.Client, ur users.Repository) {
	if m == nil || tg == nil || !tg.Configured() {
		return
	}
	m.OnInvite(func(ctx context.Context, inv identity.Invite) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		chat := telegramChatFor(ctx, ur, inv.UserID)
		if chat == 0 {
			slog.Info("taklif: xodim Telegram'i bog'lanmagan — xabar yuborilmadi", "user", inv.UserID)
			return
		}
		if err := tg.SendMessage(ctx, chat, identity.InviteText(inv)); err != nil {
			slog.Warn("taklif xabari yuborilmadi", "user", inv.UserID, "err", err)
		}
	})
}

// loadPlatformKeys — `AUTH_PLATFORM_KEY_FILE` (Ed25519 PKCS#8 PEM, `go run
// ./cmd/authkey`, ASOSIY kalitdan BOSHQA fayl) va rotatsiya uchun
// `AUTH_PLATFORM_PREVIOUS_KEY_FILES`. Dev'da yo'q bo'lsa vaqtinchalik kalit.
func loadPlatformKeys(devMode bool) *identity.KeySet {
	var active *identity.SigningKey
	if path := strings.TrimSpace(os.Getenv("AUTH_PLATFORM_KEY_FILE")); path != "" {
		k, err := identity.LoadPrivateKeyFile(path)
		if err != nil {
			slog.Error("admin markaziy kirishi O'CHIQ: platform kalitini o'qib bo'lmadi", "err", err)
			return nil
		}
		active = k
	} else if devMode {
		k, err := identity.GenerateKey()
		if err != nil {
			return nil
		}
		active = k
		slog.Warn("admin markaziy kirishi: AUTH_PLATFORM_KEY_FILE yo'q — DEV uchun VAQTINCHALIK kalit")
	} else {
		slog.Warn("admin markaziy kirishi O'CHIQ (AUTH_PLATFORM_KEY_FILE yo'q)")
		return nil
	}
	if main := strings.TrimSpace(os.Getenv("AUTH_SIGNING_KEY_FILE")); main != "" {
		if mk, err := identity.LoadPrivateKeyFile(main); err == nil && mk.KID == active.KID {
			slog.Error("admin markaziy kirishi O'CHIQ: platform kaliti asosiy kalit bilan BIR XIL — alohida kalit yarating")
			return nil
		}
	}
	var previous []*identity.SigningKey
	for _, p := range strings.Split(os.Getenv("AUTH_PLATFORM_PREVIOUS_KEY_FILES"), ",") {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		if k, err := identity.LoadPrivateKeyFile(p); err == nil {
			previous = append(previous, k)
		} else {
			slog.Error("eski platform kalitini o'qib bo'lmadi", "file", p, "err", err)
		}
	}
	ks, err := identity.NewKeySet(active, previous...)
	if err != nil {
		slog.Error("admin markaziy kirishi O'CHIQ", "err", err)
		return nil
	}
	slog.Info("rejim: admin (platform) kaliti yoqilgan", "kid", active.KID)
	return ks
}

// buildAuthz — merchant ruxsatlari (A3, `internal/identity/authz`). Rol
// shablonlari kod katalogidan HAR ishga tushishda bazaga yoziladi (kod =
// yagona manba). Yozib bo'lmasa — xizmat o'chiq (`nil`): ruxsatsiz holatda
// merchant /v2 marshrutlari ochilmaydi, eski marshrutlar ishlayveradi.
func buildAuthz(ctx context.Context, pool *pgxpool.Pool) *authz.Authorizer {
	var store authz.Store
	if pool != nil {
		store = storage.NewPgAuthzStore(pool)
	} else {
		store = authz.NewMemStore()
		slog.Warn("merchant katalogi XOTIRADA (DATABASE_URL yo'q) — restartda yo'qoladi")
	}
	if err := authz.EnsureTemplates(ctx, store); err != nil {
		slog.Error("merchant ruxsatlari O'CHIQ: rol shablonlari yozilmadi", "err", err)
		return nil
	}
	return authz.New(store)
}

// buildOIDC — OnDex OIDC provayderi (A4c). Klient: Merchant web (ShoxPos
// BFF, `merchant-web`). Sozlamalar:
//
//	OIDC_MERCHANT_WEB_SECRET          — klient siri (>= 32 belgi; BFF'da ham shu)
//	OIDC_MERCHANT_WEB_REDIRECT_URIS   — vergul bilan, ANIQ (masalan https://merchant.ondex.uz/api/auth/oidc/callback)
//	OIDC_MERCHANT_WEB_POST_LOGOUT_URIS — chiqishdan keyin qaytish (ixtiyoriy)
//	OIDC_MERCHANT_WEB_BACKCHANNEL_URI — back-channel logout (ixtiyoriy)
//
// Sir yo'q bo'lsa provayder o'chiq (`nil`) — /v2/oauth/* 503.
func buildOIDC(svc *identity.Service, pool *pgxpool.Pool) *identity.Provider {
	if svc == nil {
		return nil
	}
	secret := strings.TrimSpace(os.Getenv("OIDC_MERCHANT_WEB_SECRET"))
	if secret == "" {
		slog.Warn("OnDex OIDC O'CHIQ (OIDC_MERCHANT_WEB_SECRET yo'q) — /v2/oauth/* 503")
		return nil
	}
	if len(secret) < 32 {
		slog.Error("OnDex OIDC O'CHIQ: OIDC_MERCHANT_WEB_SECRET kamida 32 belgi bo'lishi shart")
		return nil
	}
	csv := func(raw string) []string {
		var out []string
		for _, v := range strings.Split(raw, ",") {
			if v = strings.TrimSpace(v); v != "" {
				out = append(out, v)
			}
		}
		return out
	}
	var codes identity.CodeStore
	if pool != nil {
		codes = storage.NewPgOAuthCodeStore(pool)
	} else {
		codes = identity.NewMemoryCodeStore()
		slog.Warn("OIDC kodlari XOTIRADA (DATABASE_URL yo'q)")
	}
	p, err := identity.NewProvider(svc, codes, []identity.OIDCClient{{
		ID: "merchant-web", Name: "OnDex Merchant (web)", SecretHash: identity.HashClientSecret(secret),
		RedirectURIs:           csv(os.Getenv("OIDC_MERCHANT_WEB_REDIRECT_URIS")),
		PostLogoutRedirectURIs: csv(os.Getenv("OIDC_MERCHANT_WEB_POST_LOGOUT_URIS")),
		BackchannelLogoutURI:   strings.TrimSpace(os.Getenv("OIDC_MERCHANT_WEB_BACKCHANNEL_URI")),
		// Eskirgan kirish so'rovidan qaytish: birinchi redirect_uri'ning origin'i + /login.
		InitiateLoginURI: initiateLoginFrom(os.Getenv("OIDC_MERCHANT_WEB_REDIRECT_URIS")),
	}})
	if err != nil {
		slog.Error("OnDex OIDC O'CHIQ", "err", err)
		return nil
	}
	slog.Info("rejim: OnDex OIDC yoqilgan", "issuer", p.IssuerURL())
	return p
}

// initiateLoginFrom — redirect_uri ro'yxatining birinchisidan ilovaning kirish
// sahifasi (origin + /login). Alohida sozlama shart emas: kirish sahifasi har
// doim ilovaning o'z domenida.
func initiateLoginFrom(csv string) string {
	first, _, _ := strings.Cut(csv, ",")
	u, err := url.Parse(strings.TrimSpace(first))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/login"
}

// buildMerchantAuth — merchant kirish oqimlari (A3b). Egasi uchun TOTP
// MAJBURIY, kalit `secretstore` bilan shifrlanadi: `SECRETS_ENC_KEY` yo'q
// bo'lsa production'da xizmat o'chiq (TOTP kalitini ochiq saqlashdan ko'ra
// kirishni bermaslik to'g'ri); dev'da vaqtinchalik kalit.
func buildMerchantAuth(devMode bool, svc *identity.Service, az *authz.Authorizer, pool *pgxpool.Pool,
	box *secretstore.Box, us *users.Service, ur users.Repository) *identity.MerchantAuth {
	if svc == nil || az == nil {
		slog.Warn("merchant kirishi O'CHIQ: markaziy sessiya yoki ruxsatlar xizmati yo'q")
		return nil
	}
	if box == nil {
		if !devMode {
			slog.Error("merchant kirishi O'CHIQ: SECRETS_ENC_KEY yo'q (TOTP kalitlari shifrlanishi SHART)")
			return nil
		}
		raw := make([]byte, secretstore.KeyLen)
		if _, err := rand.Read(raw); err != nil {
			return nil
		}
		b, err := secretstore.New(hex.EncodeToString(raw))
		if err != nil {
			return nil
		}
		box = b
		slog.Warn("merchant kirishi: SECRETS_ENC_KEY yo'q — DEV uchun VAQTINCHALIK kalit (restartdan keyin TOTP qayta ulanadi)")
	}
	var store mfa.Store
	if pool != nil {
		store = storage.NewPgMFAStore(pool)
	} else {
		store = mfa.NewMemStore()
	}
	m, err := identity.NewMerchantAuth(svc, identity.MerchantDeps{Users: us, Repo: ur, Authz: az, MFA: store, Box: box})
	if err != nil {
		slog.Error("merchant kirishi O'CHIQ", "err", err)
		return nil
	}
	return m
}
