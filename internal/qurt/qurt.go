// Package qurt — "Qurt" do'konlari mahsulotlari (mijoz ilovasidagi Qurt sahifasi).
//
// Qurt sotuvchisi mahsulotni POS (OnDex Merchant Panel / ShoxPos) da yozadi;
// POS uni server-server OnDex'ga yuboradi va mijoz ilovasi shu yerdan o'qiydi.
// Katalogdagi restoran mahsulotlaridan ATAYLAB ALOHIDA: qurt do'koni restoran
// emas (menyu, ochiq/yopiq, yetkazish vaqti yo'q), va ulgurji narx kabi
// do'kon ma'lumotlari restoran modeliga qo'shilmasligi kerak.
package qurt

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Chegaralar. POS ham xuddi shularni qo'llaydi (`ShoxPosProBackend/services/qurtSync.js`),
// lekin OnDex ularga TAYANMAYDI: har yuborilgan qiymat shu yerda qayta tekshiriladi.
const (
	MaxNameLen        = 120
	MaxDescriptionLen = 1000
	MaxCompositionLen = 1000
	MaxCategoryLen    = 64
	MaxWeightGrams    = 100_000 // 100 kg
	MaxPriceTiyin     = 100_000_000_00
)

// Product — mijozga ko'rinadigan qurt mahsuloti.
type Product struct {
	ID string `json:"id"`
	// StoreID — do'kon akkauntining `EntityID`si.
	StoreID string `json:"store_id"`
	// StoreName — do'kon nomi. Bazada SAQLANMAYDI: o'qishda do'kon akkauntidan
	// olinadi (nom o'zgarsa hamma mahsulotda yangilanadi).
	StoreName string `json:"store_name,omitempty"`
	// ExternalID — POS dagi mahsulot kaliti (shtrix-kod). (StoreID, ExternalID)
	// juftligi yagona: qayta yuborish mahsulotni YANGILAYDI, takrorlamaydi.
	ExternalID string `json:"-"`

	Name        string `json:"name"`
	Description string `json:"description"`
	// Composition — tarkibi.
	Composition string `json:"composition"`
	Category    string `json:"category"`
	// PriceTiyin — sotuv narxi (1 so'm = 100 tiyin).
	PriceTiyin int64 `json:"price_tiyin"`
	// WholesaleTiyin — ulgurji narx. Mijozga HECH QACHON chiqmaydi (`json:"-"`).
	WholesaleTiyin int64 `json:"-"`
	// WeightGrams — nechi grammligi.
	WeightGrams int    `json:"weight_grams"`
	ImageURL    string `json:"image_url"`
	// Available — sotuvda bor (POS qoldig'i > 0).
	Available bool      `json:"available"`
	UpdatedAt time.Time `json:"updated_at"`

	// Stock — POS qoldig'ining nusxasi (dona). Manba — POS; OnDex uni o'zi
	// o'zgartirmaydi, faqat `Reserved` ni boshqaradi.
	Stock int `json:"-"`
	// StockSet — POS `stock` yubordimi. Yubormagan (eski POS) bo'lsa mavjud
	// qoldiq O'ZGARMAYDI (bazaga yozilmaydi).
	StockSet bool `json:"-"`
	// Reserved — hali do'kon tasdiqlamagan (`new`) buyurtmalar band qilgan dona.
	Reserved int `json:"-"`
	// StockLeft — mijoz buyurtma bera oladigan dona (Stock - Reserved, 0 dan kam emas).
	// Faqat o'qishda hisoblanadi (`Product.Left`).
	StockLeft int `json:"stock_left"`
}

// Left — buyurtmaga qolgan dona.
func (p *Product) Left() int {
	if n := p.Stock - p.Reserved; n > 0 {
		return n
	}
	return 0
}

// Filter — ro'yxat sharti. `StoreIDs` BO'SH bo'lsa natija ham bo'sh (qurt
// do'koni yo'q = ko'rsatadigan narsa yo'q).
type Filter struct {
	StoreIDs []string
	Category string
	Query    string
	Limit    int
	Offset   int
}

// Repository — qurt mahsulotlari ombori.
type Repository interface {
	// Upsert — (StoreID, ExternalID) bo'yicha yaratadi yoki yangilaydi.
	// Qaytgan mahsulotda ID to'ldirilgan.
	Upsert(ctx context.Context, p *Product) (*Product, error)
	// Delete — mahsulotni olib tashlaydi; yo'q bo'lsa ham xato EMAS.
	Delete(ctx context.Context, storeID, externalID string) error
	// DeleteByStore — do'kon o'chirilganda hamma mahsulotini.
	DeleteByStore(ctx context.Context, storeID string) error
	// List — mijozga ko'rinadigan (Available) mahsulotlar.
	List(ctx context.Context, f Filter) ([]*Product, error)
	// Categories — shu do'konlarda sotuvdagi mahsulotlar kategoriyalari.
	Categories(ctx context.Context, storeIDs []string) ([]string, error)
	// GetMany — ID bo'yicha mahsulotlar (topilmaganlari natijada yo'q).
	GetMany(ctx context.Context, ids []string) ([]*Product, error)
	// Reserve — barcha pozitsiyalarni BIR YO'LA band qiladi (atomik, hammasi yoki
	// hech biri). Qoldiq yetmasa `ErrInsufficientStock`.
	Reserve(ctx context.Context, items []Reservation) error
	// Release — bandni qaytaradi (tasdiqlangan/rad etilgan/bekor qilingan buyurtma).
	// Band 0 dan pastga tushmaydi.
	Release(ctx context.Context, items []Reservation) error
}

// Reservation — bitta mahsulotdan band qilinadigan miqdor.
type Reservation struct {
	ProductID string
	Qty       int
}

// ErrInsufficientStock — so'ralgan miqdor qoldiqdan ko'p.
var ErrInsufficientStock = errors.New("qurt: qoldiq yetarli emas")

// MaxStock — POS yuborishi mumkin bo'lgan eng katta qoldiq.
const MaxStock = 1_000_000

var externalIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// ErrInvalid — kiritish noto'g'ri (xabar foydalanuvchiga ko'rsatilishi mumkin).
var ErrInvalid = errors.New("qurt: noto'g'ri qiymat")

func invalid(msg string) error { return errors.Join(ErrInvalid, errors.New(msg)) }

// ValidExternalID — POS kaliti: harf, raqam, `-`, `_`; 1..64 belgi.
func ValidExternalID(id string) bool { return externalIDRe.MatchString(id) }

// Normalize — maydonlarni tozalaydi va tekshiradi (`Product` ni joyida o'zgartiradi).
// Rasm manzilini bu yerda tekshirib BO'LMAYDI (saqlash sozlamasiga bog'liq) —
// uni chaqiruvchi (HTTP qatlami) tekshiradi.
func (p *Product) Normalize() error {
	p.Name = strings.TrimSpace(p.Name)
	p.Description = strings.TrimSpace(p.Description)
	p.Composition = strings.TrimSpace(p.Composition)
	p.Category = strings.TrimSpace(p.Category)
	p.ImageURL = strings.TrimSpace(p.ImageURL)

	switch {
	case !ValidExternalID(p.ExternalID):
		return invalid("mahsulot kaliti noto'g'ri")
	case p.Name == "" || utf8.RuneCountInString(p.Name) > MaxNameLen:
		return invalid("nom bo'sh yoki juda uzun")
	case utf8.RuneCountInString(p.Description) > MaxDescriptionLen:
		return invalid("tavsif juda uzun")
	case utf8.RuneCountInString(p.Composition) > MaxCompositionLen:
		return invalid("tarkib juda uzun")
	case p.Category == "" || utf8.RuneCountInString(p.Category) > MaxCategoryLen:
		return invalid("kategoriya bo'sh yoki juda uzun")
	case p.PriceTiyin <= 0 || p.PriceTiyin > MaxPriceTiyin:
		return invalid("sotuv narxi noto'g'ri")
	case p.WholesaleTiyin < 0 || p.WholesaleTiyin > MaxPriceTiyin:
		return invalid("ulgurji narx noto'g'ri")
	case p.WeightGrams <= 0 || p.WeightGrams > MaxWeightGrams:
		return invalid("gramm 1 dan 100000 gacha bo'lishi kerak")
	case p.Stock < 0 || p.Stock > MaxStock:
		return invalid("qoldiq noto'g'ri")
	}
	return nil
}
