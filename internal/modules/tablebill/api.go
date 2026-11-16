// Package tablebill — stolda hisobni bo'lish (RS-5, Plane 371). ADR-003 moduli:
// o'z `tablebill` sxemasi (0078), tashqi dunyo bilan faqat portlar orqali.
//
// ┌─ EGASI QARORLARI (2026-11-16) ───────────────────────────────────────┐
// * Har mehmon tanlaydi: hisobni TENG bo'lish yoki O'Z TAOMLARI.
// * Aralash: kimdir ilovada (karta) to'laydi, qolganini ofitsiant
//   "naqd/terminal qabul qildim" deb yopadi.
// * Ixtiyoriy choy puli 0/5/10% — ofitsiantga (RS-2 da olgan ofitsiant).
// * Bron zakladi (mijoz keldi -> applied) hisobdan avtomatik ayriladi.
// └──────────────────────────────────────────────────────────────────────┘
package tablebill

import (
	"context"
	"errors"
	"time"
)

const (
	StatusOpen = "open"
	StatusPaid = "paid"

	KindEqual = "equal"
	KindItems = "items"
	KindStaff = "staff"

	MethodCard     = "card"
	MethodCash     = "cash"
	MethodTerminal = "terminal"

	SharePending = "pending"
	SharePaid    = "paid"
	ShareFailed  = "failed"

	// ShareHold — to'lanmagan karta ulushi shuncha vaqt summani "band"
	// qilib turadi (to'lov havolasi muddati bilan bir xil).
	ShareHold = 15 * time.Minute
	// OrderWindow — hisobga faqat shu oraliqda yaratilgan buyurtmalar
	// qo'shiladi (oldingi kunlarning yopilmagan eski stol buyurtmalari
	// yangi mehmonlar hisobiga tushmasin).
	OrderWindow = 12 * time.Hour
	MaxParts    = 30
)

// AllowedTips — choy puli foizlari.
var AllowedTips = map[int]bool{0: true, 5: true, 10: true}

var (
	ErrNotFound        = errors.New("hisob topilmadi")
	ErrInvalid         = errors.New("ulush ma'lumotlari noto'g'ri")
	ErrConflict        = errors.New("hisob o'zgardi — qaytadan urinib ko'ring")
	ErrNothingDue      = errors.New("hisob allaqachon to'langan")
	ErrPendingPayments = errors.New("mehmon hozir ilovada to'layapti — bir oz kuting")
	ErrPaymentDisabled = errors.New("karta bilan to'lov bu restoranda sozlanmagan")
)

// Line — buyurtma qatori (narx buyurtma chegirmalari bilan taqsimlangan).
type Line struct {
	Name        string `json:"name"`
	Qty         int    `json:"qty"`
	UnitTiyin   int64  `json:"unit_tiyin"`
	AmountTiyin int64  `json:"amount_tiyin"` // qty * unit (oxirgi qatorda yaxlitlash qoldig'i)
}

// Order — port: stol buyurtmasi.
type Order struct {
	ID         string    `json:"id"`
	Number     string    `json:"order_number"`
	Status     string    `json:"status"`
	TotalTiyin int64     `json:"total_tiyin"`
	Lines      []Line    `json:"lines"`
	WaiterID   string    `json:"-"`
	WaiterName string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}

// Orders — port: stolning faol/berilgan (bekor qilinmagan) stol buyurtmalari.
type Orders interface {
	TableOrders(ctx context.Context, restaurantID, tableID string, since time.Time) ([]Order, error)
}

// Deposits — port: shu stolga o'tirgan bronning hisobdan ayriladigan zakladi.
type Deposits interface {
	SeatedDeposit(ctx context.Context, restaurantID, tableID string) (reservationID string, amountTiyin int64, ok bool)
}

// PayLink — karta to'lovi sahifasi.
type PayLink struct {
	PaymentID string    `json:"payment_id"`
	PayURL    string    `json:"pay_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Payments — port.
type Payments interface {
	StartShare(ctx context.Context, shareID, customerID, restaurantID, channel string) (*PayLink, error)
}

// Notifier — port (restoran/ofitsiant kanali). Xato oqimni to'xtatmaydi.
type Notifier interface {
	BillChanged(ctx context.Context, b *Bill, event string)
}

// Bill — stol hisobi.
type Bill struct {
	ID                   string     `json:"id"`
	RestaurantID         string     `json:"restaurant_id"`
	TableID              string     `json:"table_id"`
	TableLabel           string     `json:"table_label"`
	Status               string     `json:"status"`
	DepositReservationID string     `json:"-"`
	DepositTiyin         int64      `json:"deposit_tiyin"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	ClosedAt             *time.Time `json:"closed_at,omitempty"`
	Version              int64      `json:"-"`
}

// ItemRef — items rejimida tanlangan qator.
type ItemRef struct {
	OrderID string `json:"order_id"`
	Line    int    `json:"line"`
	Qty     int    `json:"qty"`
}

// Share — ulush.
type Share struct {
	ID          string     `json:"id"`
	BillID      string     `json:"bill_id"`
	PayerID     string     `json:"-"`
	Kind        string     `json:"kind"`
	Method      string     `json:"method"`
	AmountTiyin int64      `json:"amount_tiyin"`
	TipTiyin    int64      `json:"tip_tiyin"`
	Items       []ItemRef  `json:"items,omitempty"`
	WaiterID    string     `json:"-"`
	WaiterName  string     `json:"waiter_name,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	PaidAt      *time.Time `json:"paid_at,omitempty"`
}

// Store — saqlash porti.
type Store interface {
	GetOpen(ctx context.Context, tableID string) (*Bill, error) // yo'q -> ErrNotFound
	// CreateOpen — stolda ochiq hisob bo'lsa ErrConflict.
	CreateOpen(ctx context.Context, b *Bill) error
	Get(ctx context.Context, id string) (*Bill, error)
	ListOpen(ctx context.Context, restaurantID string) ([]*Bill, error)
	// Attach — buyurtmalarni hisobga bog'laydi (boshqa hisobdagilari o'tkazib yuboriladi).
	Attach(ctx context.Context, billID string, orderIDs []string) error
	OrderIDs(ctx context.Context, billID string) ([]string, error)
	// Attached — qaysilari allaqachon biror hisobda.
	Attached(ctx context.Context, orderIDs []string) (map[string]bool, error)
	SetDeposit(ctx context.Context, billID, reservationID string, amount int64) error
	Shares(ctx context.Context, billID string) ([]*Share, error)
	GetShare(ctx context.Context, id string) (*Share, error)
	// AddShare — faqat hisob versiyasi `version` bo'lsa (aks holda ErrConflict); versiya +1.
	AddShare(ctx context.Context, s *Share, version int64) error
	// SetShareStatus — faqat joriy holat `from` bo'lsa.
	SetShareStatus(ctx context.Context, id, from, to string, paidAt *time.Time) error
	Close(ctx context.Context, billID string, at time.Time) error
	// Tips — [from,to) da to'langan choy pullari ofitsiant bo'yicha.
	Tips(ctx context.Context, restaurantID string, from, to time.Time) ([]WaiterTips, error)
}

// WaiterTips — ofitsiantning choy pullari (RS-5, egasi qarori: ofitsiantga).
type WaiterTips struct {
	WaiterName string `json:"waiter_name"`
	TipsTiyin  int64  `json:"tips_tiyin"`
	Count      int    `json:"count"`
}
