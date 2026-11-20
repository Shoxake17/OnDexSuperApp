// Package reservation — restoran stol broni va zaklad (RS-3, Plane 369;
// ENG-1 booking dvigatelining birinchi qo'llanishi).
//
// ADR-003 modul: boshqa paketlar faqat shu fayldagi eksport qilingan API
// orqali ishlaydi; jadvallar o'z `reservation` sxemasida (0076). Stol,
// to'lov, hamyon va bildirishnoma — portlar (interfeyslar) orqali, main.go
// ulaydi.
//
// ┌─ EGASI QARORLARI (2026-11-15) ───────────────────────────────────────┐
//   - Zaklad = restoran belgilaydi, KISHI BOSHIGA (zona bo'yicha farq mumkin).
//   - Zaklad to'langach bron AVTOMATIK tasdiqlanadi.
//   - Bron vaqtidan 2+ soat oldin bekor qilinsa — zaklad to'liq qaytadi;
//     kamroq qolganda yoki kelmasa (15 daq kechikish) — restoranda qoladi.
//   - Kelsa — zaklad yakuniy hisobdan ayriladi (`DepositApplied`).
//   - Restoran bekor qilsa (sabab bilan) — zaklad doim to'liq qaytadi.
//   - To'lov: karta (Octo, restoran hisobiga) yoki OnDex Wallet.
//
// └──────────────────────────────────────────────────────────────────────┘
package reservation

import (
	"context"
	"errors"
	"time"
)

// Status — bron holati.
type Status string

const (
	StatusHeld      Status = "held"      // stol vaqtincha band, zaklad kutilmoqda
	StatusConfirmed Status = "confirmed" // tasdiqlangan
	StatusSeated    Status = "seated"    // mijoz keldi
	StatusCompleted Status = "completed"
	StatusCancelled Status = "cancelled"
	StatusNoShow    Status = "no_show"
	StatusExpired   Status = "expired" // zaklad muddatida to'lanmadi
)

// Active — stolni band qiladigan holat (bazadagi EXCLUDE bilan bir xil ro'yxat).
func (s Status) Active() bool {
	return s == StatusHeld || s == StatusConfirmed || s == StatusSeated
}

// DepositState — zaklad pulining holati.
type DepositState string

const (
	DepositNone     DepositState = ""
	DepositPending  DepositState = "pending"
	DepositPaid     DepositState = "paid"
	DepositRefunded DepositState = "refunded"
	DepositKept     DepositState = "kept"    // restoranda qoldi (kech bekor / kelmadi)
	DepositApplied  DepositState = "applied" // mijoz keldi — hisobdan ayriladi
)

// Method — zaklad to'lov usuli.
type Method string

const (
	MethodNone   Method = ""
	MethodCard   Method = "card"
	MethodWallet Method = "wallet"
)

// Cancellation actor.
const (
	ByCustomer   = "customer"
	ByRestaurant = "restaurant"
	BySystem     = "system"
)

// Qoidalar (egasi qarorlari).
const (
	FreeCancelBefore = 2 * time.Hour    // shundan oldin bekor = to'liq qaytadi
	NoShowAfter      = 15 * time.Minute // shuncha kechiksa — kelmadi
	HoldTTL          = 15 * time.Minute // zaklad to'lash muddati
	ReminderBefore   = 2 * time.Hour    // eslatma
	SlotStep         = 30 * time.Minute // bron vaqtlari qadami
	MaxActivePerUser = 3                // mijozning bir vaqtdagi faol bronlari
	MaxPartySize     = 50
)

// Settings — restoranning bron sozlamalari.
type Settings struct {
	RestaurantID          string           `json:"restaurant_id"`
	Enabled               bool             `json:"enabled"`
	DepositPerPersonTiyin int64            `json:"deposit_per_person_tiyin"`
	ZoneDeposits          map[string]int64 `json:"zone_deposits"`
	SlotMinutes           int              `json:"slot_minutes"`
	MinAdvanceMinutes     int              `json:"min_advance_minutes"`
	MaxDaysAhead          int              `json:"max_days_ahead"`
	UpdatedAt             time.Time        `json:"updated_at"`
}

// DefaultSettings — yozuv yo'q restoran uchun (bron O'CHIQ).
func DefaultSettings(restaurantID string) Settings {
	return Settings{RestaurantID: restaurantID, ZoneDeposits: map[string]int64{},
		SlotMinutes: 120, MinAdvanceMinutes: 60, MaxDaysAhead: 30}
}

// Reservation — bitta bron.
type Reservation struct {
	ID            string       `json:"id"`
	RestaurantID  string       `json:"restaurant_id"`
	TableID       string       `json:"table_id"`
	TableLabel    string       `json:"table_label"`
	CustomerID    string       `json:"customer_id"`
	PartySize     int          `json:"party_size"`
	StartsAt      time.Time    `json:"starts_at"`
	EndsAt        time.Time    `json:"ends_at"`
	Status        Status       `json:"status"`
	DepositTiyin  int64        `json:"deposit_tiyin"`
	DepositMethod Method       `json:"deposit_method"`
	DepositState  DepositState `json:"deposit_state"`
	Comment       string       `json:"comment,omitempty"`
	HoldExpiresAt *time.Time   `json:"hold_expires_at,omitempty"`
	CancelReason  string       `json:"cancel_reason,omitempty"`
	CancelledBy   string       `json:"cancelled_by,omitempty"`
	RemindedAt    *time.Time   `json:"reminded_at,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

// Xatolar (HTTP qatlami status kodiga aylantiradi).
var (
	ErrNotFound        = errors.New("bron topilmadi")
	ErrDisabled        = errors.New("bu restoranda stol broni yoqilmagan")
	ErrInvalid         = errors.New("bron ma'lumotlari noto'g'ri")
	ErrNoTable         = errors.New("bu vaqtda mos bo'sh stol yo'q")
	ErrSlotTaken       = errors.New("stol shu vaqtda band")
	ErrConflict        = errors.New("bron holati o'zgargan — sahifani yangilang")
	ErrTooMany         = errors.New("faol bronlaringiz soni chegarada")
	ErrClosed          = errors.New("restoran bu vaqtda yopiq")
	ErrWalletFunds     = errors.New("OnDex Wallet balansi yetarli emas")
	ErrPaymentDisabled = errors.New("karta bilan to'lov bu restoranda sozlanmagan")
)

// Table — port: restoran stoli (tables moduli adapteri beradi).
type Table struct {
	ID       string
	Label    string
	Zone     string
	Capacity int // 0 = noma'lum (bronga berilmaydi)
	Active   bool
}

// Tables — port.
type Tables interface {
	ListTables(ctx context.Context, restaurantID string) ([]Table, error)
}

// Hours — port: restoran [start, end) oralig'ida ochiqmi (nil — tekshirilmaydi).
type Hours interface {
	OpenBetween(ctx context.Context, restaurantID string, start, end time.Time) (bool, error)
}

// PayLink — karta to'lovi sahifasi.
type PayLink struct {
	PaymentID string    `json:"payment_id"`
	PayURL    string    `json:"pay_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Payments — port: karta zakladi (payments.Service adapteri).
type Payments interface {
	StartDeposit(ctx context.Context, reservationID, customerID, restaurantID, description, channel string) (*PayLink, error)
	RefundDeposit(ctx context.Context, reservationID string) error
}

// Wallet — port: OnDex Wallet (ref — bron ID'siga bog'langan idempotent kalit).
type Wallet interface {
	Debit(ctx context.Context, ref, userID string, amountTiyin int64) error
	Refund(ctx context.Context, ref, userID string, amountTiyin int64) error
}

// Event — bron hodisasi (restoran paneli jonli kanali + mijozga push).
type Event struct {
	Type        string // reservation_new | reservation_confirmed | reservation_cancelled | reservation_seated | reservation_no_show | reservation_reminder | reservation_expired
	Reservation Reservation
}

// Notifier — port; xato bron oqimini to'xtatmaydi.
type Notifier interface {
	Notify(ctx context.Context, e Event)
}

// Store — saqlash porti. `Insert` va holat o'zgarishi stol band bo'lsa
// ErrSlotTaken qaytaradi (bazadagi EXCLUDE).
type Store interface {
	GetSettings(ctx context.Context, restaurantID string) (*Settings, error) // yo'q -> ErrNotFound
	PutSettings(ctx context.Context, s *Settings) error
	Insert(ctx context.Context, r *Reservation) error
	Get(ctx context.Context, id string) (*Reservation, error)
	// Update — faqat bazadagi holat `from` bo'lsa yozadi (aks holda ErrConflict).
	Update(ctx context.Context, r *Reservation, from Status) error
	ListByRestaurant(ctx context.Context, restaurantID string, from, to time.Time) ([]*Reservation, error)
	ListByCustomer(ctx context.Context, customerID string, limit int) ([]*Reservation, error)
	// ListDue — fon nazoratchisi uchun: muddati o'tgan held, kelmagan
	// confirmed, eslatma vaqti kelgan confirmed, tugagan seated.
	ListDue(ctx context.Context, now time.Time, limit int) ([]*Reservation, error)
}
