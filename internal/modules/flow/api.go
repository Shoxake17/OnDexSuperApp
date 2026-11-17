// Package flow — OnDexFlow: o'z OTA (havodan) yangilanish tizimimiz
// (Plane ONDEXECOSY-472, egasi qarori 2026-11-17).
//
// ┌─ NIMA VA QANDAY ──────────────────────────────────────────────────────┐
// Ilova Shorebird dvigateli bilan yig'iladi (uning ilova ichidagi
// yangilagichi Apache-2.0/MIT, ochiq kod). `shorebird.yaml` dagi
// `base_url` BIZNING serverga qaratiladi — Shorebird bulutiga hech qanday
// murojaat yo'q. Bu modul o'sha yangilagich protokolini 1:1 bajaradi:
//
//	POST {base}/api/v1/patches/check   — "yangi yamoq bormi?"
//	POST {base}/api/v1/patches/events  — o'rnatildi/yiqildi hisobotlari
//
// Yamoq fayli = relizdagi `libapp.so` ga nisbatan bidiff+zstd farqi
// (Shorebird `patch` vositasi). Fayllar Cloudflare R2 da (cdn.ondex.uz).
// └───────────────────────────────────────────────────────────────────────┘
//
// ┌─ XAVFSIZLIK MODELI ──────────────────────────────────────────────────┐
//  1. Har ilovaning RSA kalit jufti. SHAXSIY kalit faqat egasining
//     kompyuterida; ochiq kalit ilova ichida (`patch_public_key`) va shu
//     modulda. Qurilma imzosiz/boshqa imzoli yamoqni O'RNATMAYDI — server
//     yoki R2 buzilsa ham soxta kod telefonga o'tmaydi.
//  2. Nashr (vosita -> server) ham shu kalit bilan imzolanadi va server
//     yamoq imzosini QURILMA bilan bir xil tekshiradi. Nashr faqat
//     QORALAMA yaratadi — foydalanuvchiga hech narsa ketmaydi.
//  3. Faollashtirish/pauza/orqaga qaytarish — FAQAT admin panelda (admin
//     roli + sabab + Authenticator kodi + audit, `adminAction`).
//  4. Hodisalar (events) ommaviy va soxtalashtirilishi mumkin — shuning
//     uchun ular hech qachon AVTOMATIK qaror qabul qilmaydi (masalan
//     avto-pauza hujumchiga yamoqni to'xtatish imkonini berardi); faqat
//     panelda ko'rsatiladi.
//
// └───────────────────────────────────────────────────────────────────────┘
package flow

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("topilmadi")
	ErrConflict     = errors.New("bunday yozuv allaqachon bor")
	ErrInvalid      = errors.New("noto'g'ri ma'lumot")
	ErrBadSignature = errors.New("imzo noto'g'ri")
	ErrStale        = errors.New("so'rov eskirgan yoki vaqti noto'g'ri")
	ErrTransition   = errors.New("bu holatdan bunga o'tib bo'lmaydi")
	ErrVersionLock  = errors.New("yamoq boshqa joyda o'zgartirildi — sahifani yangilang")
)

// Platformalar. Hozircha faqat Android (egasi qarori: avval Android).
const PlatformAndroid = "android"

// PatchStatus — yamoq holati.
type PatchStatus string

const (
	// Draft — nashr qilingan, foydalanuvchiga BERILMAYDI.
	PatchDraft PatchStatus = "draft"
	// Active — rollout foizidagi qurilmalarga beriladi.
	PatchActive PatchStatus = "active"
	// Paused — yangi qurilmalarga berilmaydi; o'rnatganlarda qoladi.
	PatchPaused PatchStatus = "paused"
	// RolledBack — QAYTARILMAS: qurilmalarga "o'chir" deyiladi
	// (`rolled_back_patch_numbers`), ular relizga yoki oldingi yamoqqa qaytadi.
	PatchRolledBack PatchStatus = "rolled_back"
)

// App — OnDexFlow'ga ulangan ilova. ID = `shorebird.yaml` dagi `app_id`.
type App struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Platform  string    `json:"platform"`
	PublicKey string    `json:"public_key"` // PKCS#1 DER, base64 (Shorebird formati)
	KeyID     string    `json:"key_id"`     // ochiq kalit SHA-256 ning boshi — panelda solishtirish uchun
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

// Release — do'kon/APK relizi (asosiy `libapp.so` lar).
type Release struct {
	ID        string            `json:"id"`
	AppID     string            `json:"app_id"`
	Version   string            `json:"version"` // "0.2.16+28" — qurilma yuboradigan shakl
	Platform  string            `json:"platform"`
	Artifacts []ReleaseArtifact `json:"artifacts"`
	// EntriesURL — APK ichidagi fayllar xeshi (vosita yamoqda native/asset
	// o'zgarganini aniqlash uchun ishlatadi).
	EntriesURL string    `json:"entries_url"`
	CreatedAt  time.Time `json:"created_at"`
}

type ReleaseArtifact struct {
	Arch string `json:"arch"`
	Hash string `json:"hash"` // libapp.so SHA-256 (hex)
	Size int64  `json:"size"`
	URL  string `json:"url"`
}

// Patch — relizga yamoq.
type Patch struct {
	ID          string          `json:"id"`
	ReleaseID   string          `json:"release_id"`
	AppID       string          `json:"app_id"`
	Version     string          `json:"release_version"`
	Number      int             `json:"number"`
	Channel     string          `json:"channel"`
	Status      PatchStatus     `json:"status"`
	Rollout     int             `json:"rollout_percent"` // 0..100
	Note        string          `json:"note"`
	Artifacts   []PatchArtifact `json:"artifacts"`
	Stats       PatchStats      `json:"stats"`
	Rev         int             `json:"rev"` // optimistik qulf
	CreatedAt   time.Time       `json:"created_at"`
	ActivatedAt *time.Time      `json:"activated_at,omitempty"`
	UpdatedAt   time.Time       `json:"updated_at"`
	UpdatedBy   string          `json:"updated_by"`
}

type PatchArtifact struct {
	Arch      string `json:"arch"`
	Hash      string `json:"hash"`      // YAMOQ QO'LLANGANDAN KEYINGI libapp.so SHA-256 (hex)
	Signature string `json:"signature"` // hash ustiga RSA PKCS1v15/SHA-256, base64
	Size      int64  `json:"size"`
	URL       string `json:"url"`
}

// PatchStats — qurilma hisobotlari (soxtalashtirilishi mumkin — faqat ma'lumot).
type PatchStats struct {
	Downloads int64 `json:"downloads"`
	Installs  int64 `json:"installs"`
	Failures  int64 `json:"failures"`
}

// ── Qurilma protokoli (Shorebird updater bilan 1:1, `network.rs`) ─────────

type CheckRequest struct {
	AppID              string `json:"app_id"`
	Channel            string `json:"channel"`
	ReleaseVersion     string `json:"release_version"`
	Platform           string `json:"platform"`
	Arch               string `json:"arch"`
	ClientID           string `json:"client_id"`
	CurrentPatchNumber *int   `json:"current_patch_number,omitempty"`
}

type CheckPatch struct {
	Number        int     `json:"number"`
	Hash          string  `json:"hash"`
	DownloadURL   string  `json:"download_url"`
	HashSignature *string `json:"hash_signature,omitempty"`
}

type CheckResponse struct {
	PatchAvailable         bool        `json:"patch_available"`
	Patch                  *CheckPatch `json:"patch,omitempty"`
	RolledBackPatchNumbers []int       `json:"rolled_back_patch_numbers"`
}

// Event — `__patch_install__` va h.k. (`events.rs`).
type Event struct {
	AppID          string `json:"app_id"`
	Arch           string `json:"arch"`
	ClientID       string `json:"client_id"`
	Type           string `json:"type"`
	PatchNumber    int    `json:"patch_number"`
	Platform       string `json:"platform"`
	ReleaseVersion string `json:"release_version"`
}

const (
	EventInstall        = "__patch_install__"
	EventInstallFailure = "__patch_install_failure__"
	EventDownload       = "__patch_download__"
	EventUpdateFailure  = "__patch_update_failure__"
)

// ── Tashqi bog'liqliklar (portlar) ────────────────────────────────────────

// Blobs — fayl ombori (prod: R2, cdn.ondex.uz). URL ommaviy qaytadi.
type Blobs interface {
	Put(ctx context.Context, key string, data []byte, contentType string) (url string, err error)
}

// Store — modulning o'z sxemasi (`flow`, migratsiya 0079).
type Store interface {
	CreateApp(ctx context.Context, a *App) error
	GetApp(ctx context.Context, id string) (*App, error)
	ListApps(ctx context.Context) ([]*App, error)

	CreateRelease(ctx context.Context, r *Release) error
	GetRelease(ctx context.Context, id string) (*Release, error)
	FindRelease(ctx context.Context, appID, version, platform string) (*Release, error)
	ListReleases(ctx context.Context, appID string) ([]*Release, error)

	CreatePatch(ctx context.Context, p *Patch) error
	GetPatch(ctx context.Context, id string) (*Patch, error)
	ListPatches(ctx context.Context, releaseID string) ([]*Patch, error)
	// UpdatePatch — p.Rev == bazadagi rev bo'lsagina yozadi va rev+1 qiladi.
	UpdatePatch(ctx context.Context, p *Patch) error
	// AddStat — hisoblagichni oshiradi (kind: downloads|installs|failures).
	AddStat(ctx context.Context, releaseID string, number int, kind string) error
}
