package main

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/httpapi"
	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
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
