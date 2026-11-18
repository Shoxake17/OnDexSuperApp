package catalog

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// PickupSettings — "Olib ketish" (Borish, Plane 473/477) restoran sozlamasi.
//
// Egasi qarori 2026-11-18: to'lov FAQAT oldindan (karta/Wallet), chegirmani
// restoran o'zi qo'yadi (0..30%, standart 0), mijoz kelmasa — eslatma, 2
// soatdan keyin "olib ketilmadi" (pul qaytmaydi).
type PickupSettings struct {
	Enabled bool `json:"enabled" bson:"enabled"`
	// PrepMinutes — mijozga ko'rsatiladigan taxminiy tayyorlash vaqti.
	PrepMinutes int `json:"prep_minutes" bson:"prep_minutes"`
	// DiscountPercent — olib ketishga chegirma (0..30).
	DiscountPercent int `json:"discount_percent" bson:"discount_percent"`
	// Note — mijozga ko'rsatma ("Kassa 1-qavatda").
	Note string `json:"note" bson:"note"`
}

const (
	MaxPickupDiscountPercent = 30
	maxPickupNote            = 200
)

// Validate — normallashtirilgan nusxa yoki xato.
func (p PickupSettings) Validate() (PickupSettings, error) {
	if p.DiscountPercent < 0 || p.DiscountPercent > MaxPickupDiscountPercent {
		return p, errors.New("olib ketish chegirmasi 0..30% oralig'ida bo'lishi kerak")
	}
	if p.PrepMinutes == 0 {
		p.PrepMinutes = 15
	}
	if p.PrepMinutes < 5 || p.PrepMinutes > 180 {
		return p, errors.New("tayyorlash vaqti 5..180 daqiqa bo'lishi kerak")
	}
	p.Note = strings.TrimSpace(p.Note)
	if utf8.RuneCountInString(p.Note) > maxPickupNote {
		return p, errors.New("ko'rsatma juda uzun (200 belgigacha)")
	}
	return p, nil
}

// PickupEnabled — restoran olib ketish buyurtmasini qabul qiladimi.
func (r *Restaurant) PickupEnabled() bool { return r.Pickup != nil && r.Pickup.Enabled }

// PickupDiscount — buyurtmaga qo'llanadigan chegirma foizi (o'chiq bo'lsa 0).
func (r *Restaurant) PickupDiscount() int {
	if !r.PickupEnabled() {
		return 0
	}
	return r.Pickup.DiscountPercent
}
