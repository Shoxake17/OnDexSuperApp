// Package qurtorders — Qurt do'konlaridan buyurtmalar va REYS (batch) logistikasi.
//
// ┌─ NEGA RESTORAN `orders` MODULIDAN ALOHIDA ─────────────────────────────┐
// Restoran buyurtmasi bir necha daqiqada hal bo'ladi: restoran tayyorlaydi,
// OnDex kuryeri (dispatch) darrov olib ketadi, to'lov Octo orqali. Qurt esa
// Uzum Market kabi ishlaydi: buyurtmalar BIR NECHA KUN to'planadi, do'kon
// qadoqlaydi, OnDex logistikasi do'kondan yig'ib oladi va belgilangan REYS
// kunida yetkazadi. Kuryer qidiruvi (OnDex GO kuryerlari) bu yerga UMUMAN
// ulanmaydi. Ikki oqimni bitta modelga tiqish ikkalasini ham buzardi.
// └────────────────────────────────────────────────────────────────────────┘
package qurtorders

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type Status string

const (
	StatusNew            Status = "new"              // mijoz yaratdi, do'kon hali ko'rmadi (qoldiq band)
	StatusConfirmed      Status = "confirmed"        // do'kon qabul qildi
	StatusRejected       Status = "rejected"         // do'kon rad etdi (terminal)
	StatusPacked         Status = "packed"           // do'kon qadoqladi, logistika olishi mumkin
	StatusCollected      Status = "collected"        // OnDex logistikasi do'kondan oldi
	StatusOutForDelivery Status = "out_for_delivery" // reysda, mijozga yo'lda
	StatusDelivered      Status = "delivered"        // yetkazildi, naqd olindi (terminal)
	StatusFailed         Status = "failed"           // yetkazib bo'lmadi (qayta urinish yoki bekor)
	StatusCancelled      Status = "cancelled"        // bekor qilindi (terminal)
)

// CashStatus — naqd pul qayerda: kuryerda yoki OnDex'ga topshirilgan.
type CashStatus string

const (
	CashHeld    CashStatus = "held"    // kuryerda (yetkazilgan, hali topshirmagan)
	CashSettled CashStatus = "settled" // admin kuryerdan qabul qildi
)

// QurtPoolPrefix — Qurt kuryerlarining havuz belgisi (`couriers.Courier.RestaurantID` = "qurt:<do'kon ID>").
//
// Dispatch havuzi shu maydon bo'yicha QAT'IY ajratiladi: platforma qidiruvi faqat bo'sh ("")
// havuzni, restoran faqat o'z ID'sini oladi. Haqiqiy restoran ID'sida ":" bo'lmagani uchun bu
// belgi hech biriga mos kelmaydi — Qurt kuryerlari restoran/platforma buyurtmasini UMUMAN
// olmaydi va dispatch kodiga tegilmadi. Har kuryer BITTA do'konga tegishli: do'kon o'z
// kuryerlarini o'zi boshqaradi.
const QurtPoolPrefix = "qurt:"

// PoolFor — do'konning kuryer havuzi belgisi.
func PoolFor(storeID string) string { return QurtPoolPrefix + storeID }

// IsQurtPool — kuryer yozuvi Qurt do'koniga tegishlimi.
func IsQurtPool(pool string) bool {
	return strings.HasPrefix(pool, QurtPoolPrefix) && len(pool) > len(QurtPoolPrefix)
}

// Actor — holatni kim o'zgartirmoqda.
type Actor string

const (
	ActorCustomer  Actor = "customer"
	ActorStore     Actor = "store"
	ActorLogistics Actor = "logistics" // OnDex logistika xodimi (admin panel)
	ActorAdmin     Actor = "admin"
)

var transitions = map[Status]map[Status][]Actor{
	StatusNew: {
		StatusConfirmed: {ActorStore},
		StatusRejected:  {ActorStore},
		StatusCancelled: {ActorCustomer, ActorAdmin},
	},
	StatusConfirmed: {
		StatusPacked:    {ActorStore},
		StatusCancelled: {ActorAdmin},
	},
	StatusPacked: {
		StatusCollected: {ActorLogistics, ActorAdmin},
		StatusCancelled: {ActorAdmin},
	},
	StatusCollected: {
		StatusOutForDelivery: {ActorLogistics, ActorAdmin},
		StatusCancelled:      {ActorAdmin},
	},
	StatusOutForDelivery: {
		StatusDelivered: {ActorLogistics, ActorAdmin},
		StatusFailed:    {ActorLogistics, ActorAdmin},
	},
	StatusFailed: {
		StatusOutForDelivery: {ActorLogistics, ActorAdmin}, // keyingi urinish
		StatusCancelled:      {ActorAdmin},
	},
}

// IsTerminal — buyurtma yakunlanganmi.
func IsTerminal(s Status) bool {
	switch s {
	case StatusRejected, StatusDelivered, StatusCancelled:
		return true
	}
	return false
}

// ValidStatus — ma'lum holatmi (filtrlar uchun).
func ValidStatus(s Status) bool {
	switch s {
	case StatusNew, StatusConfirmed, StatusRejected, StatusPacked, StatusCollected,
		StatusOutForDelivery, StatusDelivered, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

var (
	ErrNotFound = errors.New("qurtorders: buyurtma topilmadi")
	ErrConflict = errors.New("qurtorders: buyurtma boshqa joyda o'zgargan, qayta urining")
	ErrInvalid  = errors.New("qurtorders: noto'g'ri qiymat")
)

func invalid(msg string) error { return fmt.Errorf("%w: %s", ErrInvalid, msg) }

// TransitionError — taqiqlangan o'tish.
type TransitionError struct {
	From, To Status
	By       Actor
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("holatni %s dan %s ga %s o'zgartira olmaydi", e.From, e.To, e.By)
}

// ValidateTransition — o'tish va aktor huquqini tekshiradi.
func ValidateTransition(from, to Status, by Actor) error {
	for _, a := range transitions[from][to] {
		if a == by {
			return nil
		}
	}
	return &TransitionError{From: from, To: to, By: by}
}

// Item — buyurtmadagi pozitsiya. Narx va nom buyurtma paytida NUSXALANADI:
// do'kon keyin narxni o'zgartirsa ham buyurtma o'zgarmaydi.
type Item struct {
	ProductID   string `json:"product_id"`
	Name        string `json:"name"`
	Qty         int    `json:"qty"`
	PriceTiyin  int64  `json:"price_tiyin"`
	ImageURL    string `json:"image_url,omitempty"`
	WeightGrams int    `json:"weight_grams,omitempty"`
	// ExternalID — POS dagi mahsulot kaliti (shtrix-kod). MIJOZGA CHIQMAYDI (json:"-");
	// faqat do'konga (POS ombor qoldig'ini kamaytirish uchun) beriladi.
	ExternalID string `json:"-"`
}

// Address — yetkazish manzili (buyurtma paytidagi NUSXA).
type Address struct {
	Text      string  `json:"text"`
	Entrance  string  `json:"entrance,omitempty"`
	Floor     string  `json:"floor,omitempty"`
	Apartment string  `json:"apartment,omitempty"`
	Intercom  string  `json:"intercom,omitempty"`
	Comment   string  `json:"comment,omitempty"`
	Lat       float64 `json:"lat,omitempty"`
	Lng       float64 `json:"lng,omitempty"`
}

// StatusChange — holat tarixi yozuvi.
type StatusChange struct {
	From   Status    `json:"from"`
	To     Status    `json:"to"`
	By     Actor     `json:"by"`
	ByID   string    `json:"by_id,omitempty"`
	Reason string    `json:"reason,omitempty"`
	At     time.Time `json:"at"`
}

// Order — BITTA do'kondan buyurtma. Mijoz bir nechta do'kondan savat yig'sa,
// u do'konlar bo'yicha bo'linadi va ular umumiy `GroupID` ga ega bo'ladi.
type Order struct {
	ID      string `json:"id"`
	Number  string `json:"number"`
	GroupID string `json:"group_id"`

	CustomerID string `json:"customer_id"`
	StoreID    string `json:"store_id"`
	StoreName  string `json:"store_name"`

	Items            []Item `json:"items"`
	SubtotalTiyin    int64  `json:"subtotal_tiyin"`
	DeliveryFeeTiyin int64  `json:"delivery_fee_tiyin"`
	TotalTiyin       int64  `json:"total_tiyin"`

	// PaymentMethod — hozircha faqat "cash": yetkazilganda naqd.
	PaymentMethod string `json:"payment_method"`

	Status Status `json:"status"`
	// DeliveryDate — reys kuni (YYYY-MM-DD, Toshkent vaqti). Buyurtma paytida
	// eng yaqin reysdan hisoblanadi; reys qo'lda ko'chirilishi mumkin.
	DeliveryDate string `json:"delivery_date"`

	CustomerName  string  `json:"customer_name"`
	CustomerPhone string  `json:"customer_phone"`
	Address       Address `json:"address"`
	Comment       string  `json:"comment,omitempty"`

	RejectReason string         `json:"reject_reason,omitempty"`
	History      []StatusChange `json:"history"`

	// CourierID — reysga biriktirilgan Qurt kuryeri (`couriers.Courier.ID`); bo'sh = biriktirilmagan.
	CourierID         string     `json:"courier_id,omitempty"`
	CourierAssignedAt *time.Time `json:"courier_assigned_at,omitempty"`

	// Naqd pul hisobi: yetkazilganda (kuryer biriktirilgan bo'lsa) mijozdan olingan summa
	// kuryerda "held" holatida yoziladi; admin kuryerdan pulni qabul qilganda "settled".
	CashTiyin     int64      `json:"cash_tiyin,omitempty"`
	CashStatus    CashStatus `json:"cash_status,omitempty"`
	CashSettledAt *time.Time `json:"cash_settled_at,omitempty"`
	CashSettledBy string     `json:"cash_settled_by,omitempty"`

	// Reserved — qoldiq hali band (faqat `new` holatida true).
	Reserved bool `json:"-"`

	IdempotencyKey string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	// Version — optimistik qulf: har yozuvda +1; ikki kishi bir vaqtda o'zgartirsa
	// ikkinchisi ErrConflict oladi.
	Version int `json:"version"`
}

// Reservations — band qilingan qoldiq pozitsiyalari.
func (o *Order) ReservationItems() map[string]int {
	m := map[string]int{}
	for _, it := range o.Items {
		m[it.ProductID] += it.Qty
	}
	return m
}

const (
	MaxLines        = 50
	MaxQtyPerLine   = 100
	MaxTotalTiyin   = 100_000_000_00 // 100 mln so'm
	MaxReasonLen    = 200
	MaxCommentLen   = 300
	MinReasonLen    = 3
	DefaultCutoffHr = 18
)

// NormalizeReason — rad etish/bekor qilish sababi.
func NormalizeReason(s string) (string, error) {
	s = strings.TrimSpace(s)
	if n := utf8.RuneCountInString(s); n < MinReasonLen || n > MaxReasonLen {
		return "", invalid(fmt.Sprintf("sabab %d..%d belgi bo'lsin", MinReasonLen, MaxReasonLen))
	}
	return s, nil
}

// ── Reys jadvali ───────────────────────────────────────────────────────────

// Tashkent — O'zbekiston vaqti (UTC+5, yozgi vaqt yo'q). tzdata'ga bog'liq emas.
var Tashkent = time.FixedZone("UZT", 5*3600)

// Schedule — qaysi kunlari reys bo'ladi va buyurtma qabul qilish muddati.
type Schedule struct {
	// Weekdays — reys kunlari.
	Weekdays []time.Weekday
	// CutoffHour — reys kunidan BIR KUN oldin shu soatgacha (Toshkent) berilgan
	// buyurtmalar o'sha reysga kiradi; keyin berilganlari keyingi reysga.
	CutoffHour int
}

// DefaultSchedule — haftada ikki reys: seshanba va juma, 18:00 gacha.
func DefaultSchedule() Schedule {
	return Schedule{Weekdays: []time.Weekday{time.Tuesday, time.Friday}, CutoffHour: DefaultCutoffHr}
}

func (s Schedule) has(d time.Weekday) bool {
	for _, w := range s.Weekdays {
		if w == d {
			return true
		}
	}
	return false
}

// NextDelivery — `now` dan keyingi eng yaqin reys kuni (YYYY-MM-DD) va uning
// buyurtma qabul qilish muddati. Reys kunlari bo'sh bo'lsa (noto'g'ri sozlama)
// ertangi kun qaytadi — buyurtma baribir qabul qilinadi.
func (s Schedule) NextDelivery(now time.Time) (date string, cutoff time.Time) {
	local := now.In(Tashkent)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, Tashkent)
	hour := s.CutoffHour
	if hour < 0 || hour > 23 {
		hour = DefaultCutoffHr
	}
	for i := 1; i <= 21; i++ {
		day := start.AddDate(0, 0, i)
		if !s.has(day.Weekday()) {
			continue
		}
		c := time.Date(day.Year(), day.Month(), day.Day()-1, hour, 0, 0, 0, Tashkent)
		if local.Before(c) {
			return day.Format("2006-01-02"), c
		}
	}
	day := start.AddDate(0, 0, 1)
	return day.Format("2006-01-02"), day
}
