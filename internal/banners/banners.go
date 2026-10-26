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
	"time"
)

// ErrNotFound — berilgan ID bilan banner yo'q.
var ErrNotFound = errors.New("banner topilmadi")

// Banner — bitta reklama banneri.
type Banner struct {
	ID       string `json:"id"`
	ImageURL string `json:"image_url"`
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
