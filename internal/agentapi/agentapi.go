// Package agentapi — tashqi AI agentlar (integratsiya sheriklari) uchun
// foydalanuvchi nomidan ish qilish qatlami.
//
// ┌─ BU PAKET QANDAY MUAMMONI YECHADI ─────────────────────────────────┐
// Boshqa loyihadagi AI yordamchi (masalan Shaddiy) foydalanuvchining
// og'zaki buyrug'ini eshitadi va uni OnDex'da bajarishi kerak:
// "menga Osh buyur", "buyurtmam qayerda?".
//
// Sodda yechim — sherikka bitta "master" API kalit berish — MUTLAQO
// QABUL QILINMAYDI: bunday kalit butun bazaga teng bo'lardi va u
// o'g'irlansa har bir foydalanuvchining manzili, tarixi va puli
// hujumchi qo'lida bo'lardi.
//
// Shuning uchun bu yerda IKKI MUSTAQIL SIR ishlatiladi:
//
//	Partner Key  (`ondex_live_...`) — QAYSI ILOVA. Sherik serverida.
//	Grant Token  (`ondexg_...`)     — QAYSI FOYDALANUVCHI ruxsat bergan.
//
// Ikkalasi ham bo'lmasa foydalanuvchiga tegishli hech narsa
// ochilmaydi. Bu — OAuth'ning "client credentials + user consent"
// modelining soddalashtirilgan, lekin xavfsizlik xossalari saqlangan
// ko'rinishi.
// └────────────────────────────────────────────────────────────────────┘
//
// # Nima ATAYLAB yo'q
//
//   - Agent foydalanuvchi ID'sini PARAMETR sifatida bera olmaydi.
//     Kimning nomidan ish ketayotgani FAQAT grant tokenidan
//     aniqlanadi. Busiz `user_id` ni almashtirib qo'yish butun
//     modelni bir qatorda buzardi.
//   - Manzil, telefon, parol, to'lov usuli — o'zgartirib bo'lmaydi.
//     Manzilni almashtirish firibgarlikning eng qisqa yo'li:
//     buyurtmani qurbon to'laydi, taomni hujumchi oladi.
//   - Buyurtma bitta so'rov bilan yaratilmaydi (qarang: Draft).
package agentapi

import (
	"errors"
	"slices"
	"strings"
	"time"
)

// ── Xatolar ──
//
// Bular HTTP qatlamida aniq statusga aylanadi. Domen xatolari sifatida
// e'lon qilingan (satr taqqoslash EMAS), chunki ularga qarab qaror
// qabul qilinadi.
var (
	ErrNotFound      = errors.New("topilmadi")
	ErrPartnerKey    = errors.New("API kalit yaroqsiz")
	ErrPartnerOff    = errors.New("bu integratsiya vaqtincha o'chirilgan")
	ErrGrantInvalid  = errors.New("ulanish yaroqsiz — foydalanuvchi qaytadan ruxsat berishi kerak")
	ErrGrantRevoked  = errors.New("foydalanuvchi bu ulanishni uzgan")
	ErrGrantExpired  = errors.New("ulanish muddati tugagan")
	ErrScopeDenied   = errors.New("bu amalga ruxsat berilmagan")
	ErrLinkExpired   = errors.New("ulanish so'rovi muddati tugagan")
	ErrLinkDecided   = errors.New("bu so'rov allaqachon hal qilingan")
	ErrLinkPending   = errors.New("foydalanuvchi hali tasdiqlamagan")
	ErrDraftExpired  = errors.New("qoralama muddati tugagan")
	ErrDraftDecided  = errors.New("qoralama allaqachon hal qilingan")
	ErrTotalMismatch = errors.New("summa o'zgargan — qoralamani qaytadan yarating")
	ErrOverDaily     = errors.New("kunlik chegara tugagan")
	ErrTestPartner   = errors.New("sinov (test) kaliti bilan haqiqiy buyurtma berib bo'lmaydi")
)

// ── Ruxsatlar (scope) ──
//
// Ro'yxat ATAYLAB kalta. Har bir yangi scope — kengaytirilgan hujum
// yuzasi, shuning uchun u faqat haqiqiy ehtiyoj bo'lganda qo'shiladi.
const (
	// ScopeCatalogRead — restoranlar va menyu. Bu ma'lumot allaqachon
	// ochiq, lekin scope baribir kerak: sherik nima so'rayotgani
	// rozilik ekranida to'liq ko'rinsin.
	ScopeCatalogRead = "catalog:read"
	// ScopeProfileRead — ism va yetkazish manzili BOR-YO'QLIGI.
	// To'liq manzil matni QAYTARILMAYDI (`Profile` izohiga qarang).
	ScopeProfileRead = "profile:read"
	// ScopeOrdersRead — o'z buyurtmalari va ularning holati.
	ScopeOrdersRead = "orders:read"
	// ScopeOrdersCreate — qoralama yaratish va (chegara ichida)
	// tasdiqlash.
	ScopeOrdersCreate = "orders:create"
	// ScopeOrdersCancel — bekor qilish (holat mashinasi ruxsat
	// bergan oynada).
	ScopeOrdersCancel = "orders:cancel"
)

// AllScopes — mavjud ruxsatlarning to'liq ro'yxati.
var AllScopes = []string{
	ScopeCatalogRead,
	ScopeProfileRead,
	ScopeOrdersRead,
	ScopeOrdersCreate,
	ScopeOrdersCancel,
}

// ScopeLabel — rozilik ekranida ko'rsatiladigan matn.
//
// Bu yerda turishining sababi: ruxsat qo'shilganda uni tarjima
// qilishni UNUTIB bo'lmaydi — `ValidateScopes` ro'yxatdan
// tashqaridagi qiymatni baribir rad etadi, lekin ekranда bo'sh satr
// chiqishi foydalanuvchini nimaga rozi bo'layotganidan mahrum
// qilardi.
func ScopeLabel(scope string) string {
	switch scope {
	case ScopeCatalogRead:
		return "Restoranlar va menyuni ko'rish"
	case ScopeProfileRead:
		return "Ismingiz va manzil tanlanganini bilish"
	case ScopeOrdersRead:
		return "Buyurtmalaringiz holatini ko'rish"
	case ScopeOrdersCreate:
		return "Sizning nomingizdan buyurtma berish"
	case ScopeOrdersCancel:
		return "Buyurtmani bekor qilish"
	}
	return scope
}

// ParseScopes — vergul bilan ajratilgan satrni ro'yxatga aylantiradi
// (bazada shu ko'rinishda yotadi).
func ParseScopes(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" && !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

func JoinScopes(list []string) string { return strings.Join(list, ",") }

// ValidateScopes — noma'lum ruxsatni RAD ETADI (jimgina tashlab
// yubormaydi).
//
// Farq muhim: jimgina tashlash "orders:creat" (harf xatosi) ni bo'sh
// ro'yxatga aylantirib, sherikka tushunarsiz 403 berardi. Aniq xato
// esa muammoni bir soniyada ko'rsatadi.
func ValidateScopes(list []string) error {
	for _, s := range list {
		if !slices.Contains(AllScopes, s) {
			return errors.New("noma'lum ruxsat: " + s)
		}
	}
	return nil
}

// IntersectScopes — `want` ichidan `ceiling` ruxsat berganini qoldiradi.
//
// Ikki joyda ishlatiladi va ikkalasi ham himoya:
//   - sherik o'ziga berilgan tavandan ko'p so'rasa — kesiladi;
//   - foydalanuvchi tasdiqlaganda sherik so'ramagan ruxsat
//     qo'shilmaydi.
func IntersectScopes(want, ceiling []string) []string {
	out := make([]string, 0, len(want))
	for _, s := range want {
		if slices.Contains(ceiling, s) && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// ── Modellar ──

// Environment — kalit turi.
type Environment string

const (
	EnvLive Environment = "live"
	// EnvTest — integratsiyani ishlab chiqish uchun. Katalogni
	// o'qiydi, qoralama yaratadi, lekin HAQIQIY buyurtma
	// JOYLASHTIRMAYDI: restoran hech narsa ko'rmaydi, kuryer
	// chaqirilmaydi, pul harakat qilmaydi.
	EnvTest Environment = "test"
)

// Partner — integratsiya mijozi (masalan Shaddiy AI).
type Partner struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// KeyPrefix — kalitning sir BO'LMAGAN boshi, loglar va admin
	// ro'yxati uchun.
	KeyPrefix   string      `json:"key_prefix"`
	Environment Environment `json:"environment"`
	// Scopes — sherik SO'RASHI mumkin bo'lgan tavan.
	Scopes    []string   `json:"scopes"`
	Active    bool       `json:"active"`
	Contact   string     `json:"contact,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

// LinkStatus — ulanish so'rovining holati.
type LinkStatus string

const (
	LinkPending  LinkStatus = "pending"
	LinkApproved LinkStatus = "approved"
	LinkDenied   LinkStatus = "denied"
	// LinkConsumed — sherik grant tokenini olib ketdi. Token BIR
	// MARTA beriladi: takroriy poll uni qayta qaytarmaydi, aks holda
	// bir marta yozib olingan `link_secret` bilan tokenni istagancha
	// qayta olish mumkin bo'lardi.
	LinkConsumed LinkStatus = "consumed"
	LinkExpired  LinkStatus = "expired"
)

// LinkRequest — "Shaddiy sizning OnDex akkauntingizga ulanmoqchi".
type LinkRequest struct {
	ID        string `json:"id"`
	PartnerID string `json:"partner_id"`
	// SecretHash / CodeHash — OCHIQ qiymatlar hech qayerda
	// saqlanmaydi. `Start` ularni bir marta qaytaradi va unutadi.
	SecretHash  string     `json:"-"`
	CodeHash    string     `json:"-"`
	ExternalRef string     `json:"-"`
	Scopes      []string   `json:"scopes"`
	Status      LinkStatus `json:"status"`
	UserID      string     `json:"-"`
	GrantID     string     `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	DecidedAt   *time.Time `json:"decided_at,omitempty"`
}

func (l *LinkRequest) isLive(now time.Time) bool {
	return l.Status == LinkPending && now.Before(l.ExpiresAt)
}

// GrantStatus — grant holati.
type GrantStatus string

const (
	GrantActive  GrantStatus = "active"
	GrantRevoked GrantStatus = "revoked"
)

// Grant — foydalanuvchi bergan ruxsat.
//
// DIQQAT: `TokenHash` `json:"-"` — bu struct foydalanuvchiga
// "Ulangan ilovalar" ro'yxatida qaytariladi va hash ham chiqmasligi
// kerak (u sirning o'zi emas, lekin oflayn taqqoslashga yordam
// beradi).
type Grant struct {
	ID          string      `json:"id"`
	PartnerID   string      `json:"partner_id"`
	PartnerName string      `json:"partner_name,omitempty"`
	UserID      string      `json:"-"`
	TokenHash   string      `json:"-"`
	Scopes      []string    `json:"scopes"`
	Status      GrantStatus `json:"status"`

	// PerOrderLimitTiyin — 0 bo'lsa HAR BIR buyurtma foydalanuvchi
	// tomonidan ilovada tasdiqlanadi.
	PerOrderLimitTiyin int64 `json:"per_order_limit_tiyin"`
	// DailyLimitTiyin — 0 bo'lsa kunlik chegara yo'q (lekin har
	// buyurtma baribir `PerOrderLimitTiyin` tekshiruvidan o'tadi).
	DailyLimitTiyin int64 `json:"daily_limit_tiyin"`

	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

// Allows — grantda shu ruxsat bormi.
func (g *Grant) Allows(scope string) bool { return slices.Contains(g.Scopes, scope) }

// Usable — grant hozir ishlatilishi mumkinmi va nega mumkin emasligi.
func (g *Grant) Usable(now time.Time) error {
	if g.Status != GrantActive {
		return ErrGrantRevoked
	}
	if g.ExpiresAt != nil && !now.Before(*g.ExpiresAt) {
		return ErrGrantExpired
	}
	return nil
}

// DraftStatus — qoralama holati.
type DraftStatus string

const (
	// DraftOpen — agent tasdiqlashi mumkin (chegara ichida).
	DraftOpen DraftStatus = "draft"
	// DraftAwaitingUser — chegaradan oshdi, foydalanuvchi ilovada
	// tasdiqlashi kerak.
	DraftAwaitingUser DraftStatus = "awaiting_user"
	DraftPlaced       DraftStatus = "placed"
	DraftRejected     DraftStatus = "rejected"
	DraftExpired      DraftStatus = "expired"
)

// DraftItem — qoralamadagi bitta qator (narxlangan surat).
type DraftItem struct {
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	Qty        int    `json:"qty"`
	PriceTiyin int64  `json:"price_tiyin"`
}

// Draft — tasdiqlanmagan buyurtma.
//
// ┌─ NEGA IKKI BOSQICH ────────────────────────────────────────────────┐
// Buyurtmani bitta so'rov bilan yaratish texnik jihatdan osonroq
// bo'lardi, lekin bu yerda buyruqni ODAM emas, TIL MODELI tuzadi.
// Model "ikkita osh" ni "yigirma osh" deb tushunishi, yoki
// foydalanuvchi umuman aytmagan taomni qo'shib yuborishi mumkin.
//
// Ikki bosqich buni ikki tomondan ushlaydi:
//  1. tasdiqda agent AYNAN summani qaytarishi shart — modelning
//     "esidan chiqargan" narsasi darhol nomuvofiqlik beradi;
//  2. chegaradan oshgan summa foydalanuvchining O'ZIGA boradi.
//
// └────────────────────────────────────────────────────────────────────┘
type Draft struct {
	ID            string      `json:"id"`
	GrantID       string      `json:"-"`
	UserID        string      `json:"-"`
	PartnerID     string      `json:"-"`
	PartnerName   string      `json:"partner_name,omitempty"`
	RestaurantID  string      `json:"restaurant_id"`
	Items         []DraftItem `json:"items"`
	SubtotalTiyin int64       `json:"subtotal_tiyin"`
	DiscountTiyin int64       `json:"discount_tiyin,omitempty"`
	TotalTiyin    int64       `json:"total_tiyin"`
	PaymentMethod string      `json:"payment_method,omitempty"`
	Status        DraftStatus `json:"status"`
	// RequiresUser — chegaradan oshgani uchun ilovada tasdiq kerakmi.
	RequiresUser bool       `json:"requires_user_confirmation"`
	OrderID      string     `json:"order_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	DecidedAt    *time.Time `json:"decided_at,omitempty"`
}

// AuditEntry — agent nomidan bajarilgan amal qaydi.
type AuditEntry struct {
	ID        int64     `json:"id"`
	PartnerID string    `json:"-"`
	Partner   string    `json:"partner,omitempty"`
	GrantID   string    `json:"-"`
	UserID    string    `json:"-"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail,omitempty"`
	OK        bool      `json:"ok"`
	IP        string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}
