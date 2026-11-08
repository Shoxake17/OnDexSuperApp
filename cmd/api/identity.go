package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/httpapi"
	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/secretstore"
	"chustapp/internal/storage"
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
//	OIDC_MERCHANT_WEB_REDIRECT_URIS   — vergul bilan, ANIQ (masalan https://pos.ondex.uz/api/auth/oidc/callback)
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
	}})
	if err != nil {
		slog.Error("OnDex OIDC O'CHIQ", "err", err)
		return nil
	}
	slog.Info("rejim: OnDex OIDC yoqilgan", "issuer", p.IssuerURL())
	return p
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
