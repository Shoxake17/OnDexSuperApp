package agentapi

import (
	"context"
	"time"
)

// Repository — saqlash qatlami.
//
// Bitta katta interfeys (alohida PartnerRepo/GrantRepo emas): bu
// yozuvlar bitta hayot sikliga tegishli va har doim birga
// ishlatiladi. Ajratish faqat qo'shimcha bog'lash kodini keltirib
// chiqarardi.
//
// Implementatsiyalar: `storage.PgAgentRepo` (production) va
// `storage.MemoryAgentRepo` (testlar va DATABASE_URL'siz dev).
type Repository interface {
	// ── Sheriklar ──

	// CreatePartner — `keyHash` ALOHIDA argument: `Partner` structida
	// hash maydoni umuman yo'q, ya'ni uni tasodifan JSON javobga
	// qo'shib yuborish IMKONSIZ.
	CreatePartner(ctx context.Context, p *Partner, keyHash string) error
	PartnerByKeyHash(ctx context.Context, keyHash string) (*Partner, error)
	PartnerByID(ctx context.Context, id string) (*Partner, error)
	ListPartners(ctx context.Context) ([]*Partner, error)
	SetPartnerActive(ctx context.Context, id string, active bool) error

	// ── Ulanish so'rovlari ──

	CreateLink(ctx context.Context, l *LinkRequest) error
	LinkByID(ctx context.Context, id string) (*LinkRequest, error)
	LinkByCodeHash(ctx context.Context, codeHash string) (*LinkRequest, error)
	UpdateLink(ctx context.Context, l *LinkRequest) error

	// ── Grantlar ──

	CreateGrant(ctx context.Context, g *Grant) error
	GrantByID(ctx context.Context, id string) (*Grant, error)
	GrantByTokenHash(ctx context.Context, tokenHash string) (*Grant, error)
	// ActiveGrantFor — shu sherik + shu foydalanuvchi uchun faol
	// grant. Takroriy ulanishda eskisi bekor qilinadi (bazadagi
	// qisman unikal indeks buni majburlaydi).
	ActiveGrantFor(ctx context.Context, partnerID, userID string) (*Grant, error)
	ListGrantsByUser(ctx context.Context, userID string) ([]*Grant, error)
	UpdateGrant(ctx context.Context, g *Grant) error
	// TouchGrant — `last_used_at`. Alohida metod: har so'rovda butun
	// grantni qayta yozish keraksiz yozuv yuki berardi.
	TouchGrant(ctx context.Context, id string, at time.Time) error

	// ── Qoralamalar ──

	CreateDraft(ctx context.Context, d *Draft) error
	DraftByID(ctx context.Context, id string) (*Draft, error)
	UpdateDraft(ctx context.Context, d *Draft) error
	// ListOpenDraftsByUser — ilovada "tasdiqlashingizni kutmoqda"
	// ro'yxati uchun.
	ListOpenDraftsByUser(ctx context.Context, userID string) ([]*Draft, error)
	// SpentSince — shu grant orqali JOYLASHTIRILGAN buyurtmalar
	// summasi. Kunlik chegarani tekshirish uchun.
	SpentSince(ctx context.Context, grantID string, since time.Time) (int64, error)

	// ── Audit ──

	AppendAudit(ctx context.Context, e *AuditEntry) error
	ListAuditByUser(ctx context.Context, userID string, limit int) ([]*AuditEntry, error)
}
