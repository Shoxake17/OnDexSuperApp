// Package shop — OnDex POS: do'kon, apteka va qurt do'konining kassa/ombor
// tizimi. ShoxPos Node.js backendining 1:1 o'rni (Plane ONDEXECOSY-486,
// egasi qarori 2026-11-28).
//
// ┌─ 1:1 VA NIMA O'ZGARDI ─────────────────────────────────────────────┐
// Funksiyalar ShoxPos bilan BIR XIL (kirim -> sotuvga o'tkazish, kunlik
// sotuv yig'indisi, 30 kunlik sotuv qatorlari, qarzlar, apteka chekida
// qoldiq tekshiruvi, global shtrix-kod bazasi bilan qidiruv). O'zgargani
// faqat XAVFSIZLIK va ishonchlilik:
//
//   - do'kon FAQAT sessiyadan (ShoxPos `?login=` orqali begona do'konning
//     mahsulotini ko'rsatardi — IDOR);
//   - apteka sotuvi BITTA tranzaksiya (ShoxPos'da parallel sotuv qoldiqni
//     minusga tushirardi);
//   - global dori bazasiga faqat apteka do'koni yozadi;
//   - pul tiyinda (int64), ShoxPos'dagi float so'm emas.
//
// └───────────────────────────────────────────────────────────────────┘
package shop

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("topilmadi")
	ErrInvalid      = errors.New("noto'g'ri ma'lumot")
	ErrForbidden    = errors.New("bu bo'lim sizning do'kon turingiz uchun mavjud emas")
	ErrInsufficient = errors.New("qoldiq yetarli emas")
	ErrConflict     = errors.New("bunday yozuv allaqachon bor")
)

// Do'kon kategoriyalari (OnDex admin panelda belgilanadi; users.store_category).
const (
	CategoryQurt     = "qurt"
	CategoryStore    = "store"
	CategoryPharmacy = "pharmacy"
)

// ShopType — ShoxPos `shopType` (kategoriya tavsiyalari shu bo'yicha umumiy).
func ShopType(category string) string {
	switch category {
	case CategoryQurt:
		return "qurt"
	case CategoryPharmacy:
		return "dorixona"
	default:
		return "dokon"
	}
}

// Store — so'rov egasining do'koni (sessiyadan, mijoz bera olmaydi).
type Store struct {
	ID       string
	Category string
}

// Product — do'kon mahsuloti. Quantity — sotuvda, IncomingQuantity — kirimda.
type Product struct {
	Barcode          string    `json:"barcode"`
	Name             string    `json:"name"`
	Category         string    `json:"category"`
	PriceTiyin       int64     `json:"price_tiyin"`
	BasePriceTiyin   int64     `json:"base_price_tiyin"`
	Quantity         float64   `json:"quantity"`
	IncomingQuantity float64   `json:"incoming_quantity"`
	Sold             float64   `json:"sold"`
	ImageURL         string    `json:"image_url"`
	Description      string    `json:"description"`
	Composition      string    `json:"composition"`
	WeightGrams      int       `json:"weight_grams"`
	AddedAt          time.Time `json:"added_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// BaseProduct — global shtrix-kod ma'lumotnomasidagi yozuv (yoki qidiruvda
// do'kon mahsuloti bilan birlashtirilgan natija: IsGlobal).
type BaseProduct struct {
	Barcode        string `json:"barcode"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	PriceTiyin     int64  `json:"price_tiyin"`
	BasePriceTiyin int64  `json:"base_price_tiyin"`
}

// SearchHit — qidiruv natijasi: do'kon mahsuloti yoki global bazadan.
type SearchHit struct {
	Barcode          string  `json:"barcode"`
	Name             string  `json:"name"`
	Category         string  `json:"category"`
	PriceTiyin       int64   `json:"price_tiyin"`
	BasePriceTiyin   int64   `json:"base_price_tiyin"`
	Quantity         float64 `json:"quantity"`
	IncomingQuantity float64 `json:"incoming_quantity"`
	Sold             float64 `json:"sold"`
	IsGlobal         bool    `json:"is_global"`
}

// Sale — kunlik sotuv yig'indisi (do'kon+mahsulot+to'lov turi+kun).
type Sale struct {
	ID             string    `json:"id"`
	Barcode        string    `json:"barcode"`
	Name           string    `json:"name"`
	Category       string    `json:"category"`
	Quantity       float64   `json:"quantity"`
	PriceTiyin     int64     `json:"price_tiyin"`
	BasePriceTiyin int64     `json:"base_price_tiyin"`
	TotalTiyin     int64     `json:"total_tiyin"`
	PaymentMethod  string    `json:"payment_method"`
	SoldAt         time.Time `json:"sold_at"`
}

// SaleLine — bitta sotuv qatori ("Sotilganlar", 30 kun).
type SaleLine struct {
	Barcode        string    `json:"barcode"`
	Name           string    `json:"name"`
	Category       string    `json:"category"`
	Quantity       float64   `json:"quantity"`
	PriceTiyin     int64     `json:"price_tiyin"`
	BasePriceTiyin int64     `json:"base_price_tiyin"`
	TotalTiyin     int64     `json:"total_tiyin"`
	PaymentMethod  string    `json:"payment_method"`
	SoldAt         time.Time `json:"sold_at"`
}

// SaleLineRetention — "Sotilganlar" qatorlari shu muddat ko'rsatiladi (ShoxPos TTL).
const SaleLineRetention = 30 * 24 * time.Hour

// CategoryStat — kategoriya bo'yicha sotuv statistikasi.
type CategoryStat struct {
	Category   string  `json:"category"`
	TotalTiyin int64   `json:"total_tiyin"`
	Quantity   float64 `json:"quantity"`
	Count      int     `json:"count"`
}

// Debt — mijoz qarzi.
type Debt struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Phone       string    `json:"phone"`
	AmountTiyin int64     `json:"amount_tiyin"`
	LastPayment time.Time `json:"last_payment"`
}

// PharmProduct — aptekadagi dori.
type PharmProduct struct {
	Barcode              string     `json:"barcode"`
	Name                 string     `json:"name"`
	Category             string     `json:"category"`
	PriceTiyin           int64      `json:"price_tiyin"`
	BasePriceTiyin       int64      `json:"base_price_tiyin"`
	Quantity             float64    `json:"quantity"`
	IncomingQuantity     float64    `json:"incoming_quantity"`
	Sold                 float64    `json:"sold"`
	Dosage               string     `json:"dosage"`
	Manufacturer         string     `json:"manufacturer"`
	ExpiryDate           *time.Time `json:"expiry_date,omitempty"`
	BatchNumber          string     `json:"batch_number"`
	PrescriptionRequired bool       `json:"prescription_required"`
	StorageCondition     string     `json:"storage_condition"`
	Description          string     `json:"description"`
	DosageForm           string     `json:"dosage_form"`
	ShelfNumber          string     `json:"shelf_number"`
	RowNumber            string     `json:"row_number"`
	AddedAt              time.Time  `json:"added_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// PharmBase — global dori ma'lumotnomasi.
type PharmBase struct {
	Barcode              string `json:"barcode"`
	Name                 string `json:"name"`
	Category             string `json:"category"`
	BasePriceTiyin       int64  `json:"base_price_tiyin"`
	Dosage               string `json:"dosage"`
	Manufacturer         string `json:"manufacturer"`
	StorageCondition     string `json:"storage_condition"`
	SideEffects          string `json:"side_effects"`
	Description          string `json:"description"`
	Composition          string `json:"composition"`
	DosageForm           string `json:"dosage_form"`
	PrescriptionRequired bool   `json:"prescription_required"`
}

// PharmSaleItem — apteka chekidagi qator.
type PharmSaleItem struct {
	Barcode        string  `json:"barcode"`
	Name           string  `json:"name"`
	Quantity       float64 `json:"quantity"`
	PriceTiyin     int64   `json:"price_tiyin"`
	BasePriceTiyin int64   `json:"base_price_tiyin"`
	TotalTiyin     int64   `json:"total_tiyin"`
	Dosage         string  `json:"dosage"`
	Manufacturer   string  `json:"manufacturer"`
}

// PharmSale — apteka cheki.
type PharmSale struct {
	ID            string          `json:"id"`
	Items         []PharmSaleItem `json:"items"`
	TotalTiyin    int64           `json:"total_tiyin"`
	PaymentMethod string          `json:"payment_method"`
	SoldAt        time.Time       `json:"sold_at"`
}

// PharmPaymentMethods — ShoxPos bilan bir xil.
var PharmPaymentMethods = map[string]bool{"naqd": true, "plastik": true, "qarz": true}

// Repo — modulning ma'lumot ombori (Postgres yoki xotira). Biznes qoidalari
// `Service` da; ombor faqat oddiy amallarni bajaradi. `Tx` ichidagi amallar
// bitta tranzaksiya (Postgres) yoki bitta qulf (xotira) ostida.
type Repo interface {
	Tx(ctx context.Context, fn func(r Repo) error) error

	// Mahsulotlar
	GetProduct(ctx context.Context, storeID, barcode string) (*Product, error)
	FindProductByName(ctx context.Context, storeID, name string) (*Product, error)
	ListProducts(ctx context.Context, storeID string) ([]Product, error)
	SearchProducts(ctx context.Context, storeID, q, category string, limit int) ([]Product, error)
	SaveProduct(ctx context.Context, storeID string, p *Product) error
	DeleteProduct(ctx context.Context, storeID, barcode string) (bool, error)

	// Global baza
	GetBase(ctx context.Context, barcode string) (*BaseProduct, error)
	FindBaseByName(ctx context.Context, name string) (*BaseProduct, error)
	SearchBase(ctx context.Context, q, category string, limit int) ([]BaseProduct, error)
	CountBase(ctx context.Context) (int, error)
	ListBase(ctx context.Context, offset, limit int) ([]BaseProduct, error)

	// Kategoriyalar
	StoreCategories(ctx context.Context, storeID string) ([]string, error)
	TypeCategories(ctx context.Context, shopType string) ([]string, error)
	AddCategory(ctx context.Context, storeID, shopType, name string) error

	// Sotuvlar
	GetDailySale(ctx context.Context, storeID, barcode, payment string, day time.Time) (*Sale, error)
	SaveSale(ctx context.Context, storeID string, s *Sale, day time.Time) error
	AddSaleLine(ctx context.Context, storeID string, l SaleLine) error
	ListSales(ctx context.Context, storeID string, from, to *time.Time) ([]Sale, error)
	DeleteSale(ctx context.Context, storeID, id string) (bool, error)
	ListSaleLines(ctx context.Context, storeID string, from, to *time.Time) ([]SaleLine, error)

	// Qarzlar
	ListDebts(ctx context.Context, storeID string) ([]Debt, error)
	GetDebt(ctx context.Context, storeID, id string) (*Debt, error)
	SaveDebt(ctx context.Context, storeID string, d *Debt) error
	DeleteDebt(ctx context.Context, storeID, id string) error

	// Apteka
	GetPharm(ctx context.Context, storeID, barcode string) (*PharmProduct, error)
	ListPharm(ctx context.Context, storeID string) ([]PharmProduct, error)
	SearchPharm(ctx context.Context, storeID, q, category string, limit int) ([]PharmProduct, error)
	FindPharmInfo(ctx context.Context, storeID, q string) (*PharmProduct, error)
	SavePharm(ctx context.Context, storeID string, p *PharmProduct) error
	DeletePharm(ctx context.Context, storeID, barcode string) (bool, error)
	SearchPharmBase(ctx context.Context, q, category string, limit int) ([]PharmBase, error)
	CountPharmBase(ctx context.Context) (int, error)
	ListPharmBase(ctx context.Context) ([]PharmBase, error)
	AddPharmBase(ctx context.Context, b PharmBase, storeID string) error
	SavePharmSale(ctx context.Context, storeID string, s *PharmSale) error
	ListPharmSales(ctx context.Context, storeID string, limit int) ([]PharmSale, error)
	PharmSaleTotals(ctx context.Context, storeID string) (count int, totalTiyin int64, err error)
}
