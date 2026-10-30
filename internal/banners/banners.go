// Package banners — reklama/aksiya bannerlari.
//
// ┌─ YANDEX EATS/GO USLUBI ──────────────────────────────────────────────┐
// Avval mijoz ilovasining restoran sahifasidagi reklama bloki qattiq
// yozilgan edi (matn ham, rasm ham kodda). Endi admin panelidan
// boshqariladi: rasm yuklanadi, ixtiyoriy ravishda bitta restoranga
// bog'lanadi (bosilganda o'sha restoran menyusi ochiladi), yoqish/
// o'chirish va tartib (`SortOrder`) bilan.
//
// Matn banner RASMINING O'ZIDA — xuddi Yandex Eats/Go dagidek (marketing
// jamoasi tayyor grafika yuklaydi). Kod alohida sarlavha/tavsif
// maydonlarini SAQLAMAYDI — bitta manba (rasm) ikki marta chizilmasin.
// └────────────────────────────────────────────────────────────────────────┘
package banners

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrNotFound — berilgan ID bilan banner yo'q.
var ErrNotFound = errors.New("banner topilmadi")

// Joylashuv: banner mijoz ilovasining QAYSI sahifasida chiqadi.
const (
	// PlacementMenu — bosh sahifa ("Menu Banner").
	PlacementMenu = "menu"
	// PlacementRestaurant — restoranlar sahifasi ("Restoran Banner"). Bannerlar
	// shu joylashuvgacha faqat shu yerda ko'rinardi, shuning uchun u standart.
	PlacementRestaurant = "restaurant"
	// PlacementStore — do'kon / Qurt sahifasi ("Do'kon Banner").
	PlacementStore = "store"
)

// Bosilganda bajariladigan amal (restoranga havola alohida: `RestaurantID`).
const (
	ActionNone = ""
	// ActionQurt — Qurt sahifasini ochadi.
	ActionQurt = "qurt"
)

// Matn qatlami chegaralari (belgi, bayt EMAS).
const (
	MaxTitleLen      = 60
	MaxSubtitleLen   = 140
	MaxButtonTextLen = 24
)

// ErrInvalid — banner maydonlari noto'g'ri (xabar adminga ko'rsatiladi).
var ErrInvalid = errors.New("banner maydonlari noto'g'ri")

// Placements — ruxsat etilgan joylashuvlar.
func Placements() []string {
	return []string{PlacementMenu, PlacementRestaurant, PlacementStore}
}

// ValidPlacement — ro'yxatdagi qiymatmi.
func ValidPlacement(p string) bool {
	return p == PlacementMenu || p == PlacementRestaurant || p == PlacementStore
}

// Banner — bitta reklama banneri.
type Banner struct {
	ID       string `json:"id"`
	ImageURL string `json:"image_url"`
	// Placement — qaysi sahifada chiqadi (`PlacementMenu|Restaurant|Store`).
	Placement string `json:"placement"`
	// Title/Subtitle/ButtonText — rasm USTIGA chiziladigan ixtiyoriy matn. Rasmning
	// o'zida matn bo'lsa bo'sh qoldiriladi.
	Title      string `json:"title,omitempty"`
	Subtitle   string `json:"subtitle,omitempty"`
	ButtonText string `json:"button_text,omitempty"`
	// Action — bosilganda amal (`ActionQurt`); bo'sh bo'lsa restoranga havola yoki
	// umuman havolasiz.
	Action string `json:"action,omitempty"`
	// RestaurantID — bosilganda ochiladigan restoran. Bo'sh — banner
	// faqat ko'rsatish uchun, bosilmaydi (`internal/assistant`dagi
	// "ishlaydigandek tugma qo'ymang" qoidasi bilan bir xil: mijoz
	// ilovasi bu maydon bo'sh bo'lsa bannerni NOINTERACTIVE chizadi).
	RestaurantID string    `json:"restaurant_id,omitempty"`
	Active       bool      `json:"active"`
	SortOrder    int       `json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Normalize — maydonlarni tozalaydi va tekshiradi (joyida o'zgartiradi). Rasm
// manzilini bu yerda tekshirib BO'LMAYDI (saqlash sozlamasiga bog'liq) — uni HTTP
// qatlami tekshiradi.
//
// Qoidalar:
//   - joylashuv bo'sh bo'lsa `restaurant` (eski mijozlar va eski bannerlar);
//   - tugma matni faqat HAVOLA bo'lsa mumkin: ishlamaydigan tugma chizilmasin;
//   - restoranga havola va amal bir vaqtda bo'lmaydi (qaysi biri ochilishi noaniq);
//   - restoranga havola faqat restoran sahifasida mantiqli, Qurt amali esa
//     restoran sahifasida emas.
func (b *Banner) Normalize() error {
	b.Placement = strings.TrimSpace(b.Placement)
	if b.Placement == "" {
		b.Placement = PlacementRestaurant
	}
	b.Title = strings.TrimSpace(b.Title)
	b.Subtitle = strings.TrimSpace(b.Subtitle)
	b.ButtonText = strings.TrimSpace(b.ButtonText)
	b.Action = strings.TrimSpace(b.Action)
	b.RestaurantID = strings.TrimSpace(b.RestaurantID)

	bad := func(msg string) error { return errors.Join(ErrInvalid, errors.New(msg)) }
	switch {
	case !ValidPlacement(b.Placement):
		return bad("joylashuv: menu, restaurant yoki store")
	case utf8.RuneCountInString(b.Title) > MaxTitleLen:
		return bad("sarlavha juda uzun")
	case utf8.RuneCountInString(b.Subtitle) > MaxSubtitleLen:
		return bad("tavsif juda uzun")
	case utf8.RuneCountInString(b.ButtonText) > MaxButtonTextLen:
		return bad("tugma matni juda uzun")
	case b.Action != ActionNone && b.Action != ActionQurt:
		return bad("amal: bo'sh yoki qurt")
	case b.Action != ActionNone && b.RestaurantID != "":
		return bad("restoranga havola va amal bir vaqtda bo'lmaydi")
	case b.RestaurantID != "" && b.Placement != PlacementRestaurant:
		return bad("restoranga havola faqat restoran bannerida mumkin")
	case b.ButtonText != "" && b.Action == ActionNone && b.RestaurantID == "":
		return bad("tugma uchun havola (amal yoki restoran) tanlang")
	}
	return nil
}

// Repository — bannerlar ombori.
type Repository interface {
	// List — HAMMASI (faol va nofaol), tartib bo'yicha — admin panel
	// uchun.
	List(ctx context.Context) ([]*Banner, error)
	// ListActive — faqat FAOL bannerlar, tartib bo'yicha — mijoz
	// ilovasi uchun.
	ListActive(ctx context.Context) ([]*Banner, error)
	Get(ctx context.Context, id string) (*Banner, error)
	Create(ctx context.Context, b *Banner) error
	Update(ctx context.Context, b *Banner) error
	Delete(ctx context.Context, id string) error
}
