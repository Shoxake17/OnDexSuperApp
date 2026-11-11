package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/kms"
	"chustapp/internal/secretstore"
)

// ┌─ KMS (OpenBao / Vault Transit) ─────────────────────────────────────────┐
// `AUTH_KMS=openbao` bo'lsa imzolash kalitlari, yordamchi sirlar ildizi va
// SECRETS_ENC_KEY (o'ralgan) KMS'dan olinadi; maxfiy kalit fayli KERAK EMAS.
//
//	AUTH_KMS                 — "openbao" (bo'sh = eski fayl rejimi)
//	VAULT_ADDR, VAULT_ROLE_ID, VAULT_SECRET_ID_FILE, VAULT_CACERT,
//	VAULT_TRANSIT_MOUNT      — `kms.FromEnv` ga qarang
//	AUTH_KMS_SIGNING_KEY     — standart "ondex-identity" (ed25519)
//	AUTH_KMS_PLATFORM_KEY    — standart "ondex-platform" (ed25519, admin)
//	AUTH_KMS_DERIVE_KEY      — standart "ondex-derive"   (hmac)
//	AUTH_KMS_PUBLISH_DELAY   — yangi versiya imzoga o'tguncha (standart 15m)
//	SECRETS_ENC_KEY_WRAPPED  — "vault:v1:..." (Transit encrypt natijasi)
//	SECRETS_KMS_KEY          — standart "ondex-secrets" (aes256-gcm96)
//
// O'rnatish: deploy/openbao/README.md.
// └─────────────────────────────────────────────────────────────────────────┘

var (
	kmsOnce   sync.Once
	kmsShared *kms.Client
	kmsErr    error
)

// kmsEnabled — KMS rejimi so'ralganmi.
func kmsEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("AUTH_KMS")), "openbao")
}

// kmsClient — bitta umumiy mijoz (AppRole tokeni bir marta olinadi).
func kmsClient() (*kms.Client, error) {
	kmsOnce.Do(func() {
		cfg, err := kms.FromEnv()
		if err != nil {
			kmsErr = err
			return
		}
		kmsShared, kmsErr = kms.New(cfg)
	})
	return kmsShared, kmsErr
}

func envOr(v, def string) string {
	if v = strings.TrimSpace(v); v != "" {
		return v
	}
	return def
}

// remoteKeySet — KMS'dagi Ed25519 kalit to'plami. Yordamchi sirlar ildizi
// Transit HMAC bilan (1-versiya, ANIQ) hisoblanadi: imzolash kaliti
// rotatsiyasi SSO cookie va mfa_token'larni buzmaydi.
func remoteKeySet(keyName string) (*identity.KeySet, error) {
	kc, err := kmsClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	root, err := kc.HMAC(ctx, envOr(os.Getenv("AUTH_KMS_DERIVE_KEY"), "ondex-derive"), 1, []byte("ondex-identity-root-v1:"+keyName))
	if err != nil {
		return nil, fmt.Errorf("yordamchi sir ildizi: %w", err)
	}
	delay := identity.DefaultPublishDelay
	if raw := strings.TrimSpace(os.Getenv("AUTH_KMS_PUBLISH_DELAY")); raw != "" {
		d, err := time.ParseDuration(raw)
		if err != nil || d < 0 {
			return nil, fmt.Errorf("AUTH_KMS_PUBLISH_DELAY noto'g'ri: %q", raw)
		}
		delay = d
	}
	ks, err := identity.NewRemoteKeySet(ctx, kc.SigningKeys(keyName), root, delay)
	if err != nil {
		return nil, err
	}
	// Rotatsiya restartsiz qabul qilinadi (har daqiqada versiyalar o'qiladi).
	ks.StartRefresh(context.Background(), time.Minute)
	return ks, nil
}

// secretsEncKey — `secretstore` kaliti. KMS rejimida SECRETS_ENC_KEY_WRAPPED
// Transit bilan ochiladi (envelope: ochiq kalit diskda/env'da YO'Q, faqat
// xotirada). Aks holda eski SECRETS_ENC_KEY.
func secretsEncKey() string {
	wrapped := strings.TrimSpace(os.Getenv("SECRETS_ENC_KEY_WRAPPED"))
	if wrapped == "" {
		return os.Getenv("SECRETS_ENC_KEY")
	}
	if os.Getenv("SECRETS_ENC_KEY") != "" {
		slog.Error("SECRETS_ENC_KEY va SECRETS_ENC_KEY_WRAPPED ikkalasi berilgan — ochiq kalitni env'dan olib tashlang; o'ralgani ishlatiladi")
	}
	kc, err := kmsClient()
	if err != nil {
		slog.Error("SECRETS_ENC_KEY_WRAPPED: KMS sozlanmagan — sirlar shifrlash O'CHIQ", "err", err)
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	plain, err := kc.Decrypt(ctx, envOr(os.Getenv("SECRETS_KMS_KEY"), "ondex-secrets"), wrapped)
	if err != nil {
		slog.Error("SECRETS_ENC_KEY_WRAPPED ochilmadi — sirlar shifrlash O'CHIQ", "err", err)
		return ""
	}
	// Operator hex/base64 matnni ham, xom 32 baytni ham o'ragan bo'lishi mumkin.
	if len(plain) == secretstore.KeyLen {
		return hex.EncodeToString(plain)
	}
	return strings.TrimSpace(string(plain))
}
