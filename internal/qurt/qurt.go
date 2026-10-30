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
}

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
	}
	return nil
}
