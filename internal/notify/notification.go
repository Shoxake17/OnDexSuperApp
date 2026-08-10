package notify

import (
	"context"
	"time"
)

// ── UMUMIY BILDIRISHNOMA MODELI ────────────────────────────────────
//
// ┌─ NEGA UMUMIY (avval food'ga bog'langan edi) ──────────────────────┐
// Avvalgi interfeys shunday edi:
//
//	type Notifier interface {
//	    OrderCreated(o *orders.Order)
//	    OrderStatusChanged(o *orders.Order, from Status)
//	}
//
// U `orders.Order` ga QATTIQ bog'langan. Do'kon, taxi, bandlash yoki
// e'lon moduli buni umuman ishlata olmasdi — har biri interfeysga
// yangi metod qo'shishga majbur qilardi va bildirishnoma qatlami
// modullar soniga qarab shishardi.
//
// Endi bitta `Event`: modul va tur — shunchaki SATR. Yangi modul
// qo'shilganda bu paketga TEGILMAYDI.
// └───────────────────────────────────────────────────────────────────┘

// Event — yuboriladigan bildirishnoma mazmuni.
type Event struct {
	// Module — "food" | "shop" | "ride" | ... | "platform".
	Module string
	// Kind — modul ichidagi tur: "order_status", "offer",
	// "booking_reminder", "listing_approved"...
	Kind string
	// Title/Body — foydalanuvchi ko'radigan matn (push uchun ham).
	Title string
	Body  string
	// Data — ilova ichida kerakli joyga o'tish uchun (deep link).
	// FCM `data` maydoniga ham shu ketadi, shuning uchun qiymatlar
	// SATR bo'lishi shart.
	Data map[string]string
}

// Notification — DB'ga yozilgan bildirishnoma.
type Notification struct {
	ID        string            `json:"id"`
	UserID    string            `json:"user_id"`
	Module    string            `json:"module"`
	Kind      string            `json:"kind"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Data      map[string]string `json:"data,omitempty"`
	ReadAt    *time.Time        `json:"read_at,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// Store — bildirishnomalar ombori.
//
// ┌─ NEGA SAQLANADI ──────────────────────────────────────────────────┐
// WebSocket xabari yuborilmasa IZSIZ YO'QOLADI: soket o'lik, ilova
// yopiq yoki tarmoq uzilgan bo'lishi mumkin. Avval kuryer taklifni
// ko'rmasdan qolardi, mijoz esa buyurtma holati o'zgarganini
// bilmasdi va buni aniqlashning HECH QANDAY yo'li yo'q edi.
//
// Endi bildirishnoma AVVAL yoziladi, KEYIN yuboriladi. Foydalanuvchi
// ilovani ochganda `GET /notifications` orqali o'qilmaganlarni
// baribir ko'radi.
// └───────────────────────────────────────────────────────────────────┘
type Store interface {
	Save(ctx context.Context, n *Notification) error
	// List — eng yangilaridan boshlab.
	List(ctx context.Context, userID string, limit int) ([]*Notification, error)
	UnreadCount(ctx context.Context, userID string) (int, error)
	// MarkRead — FAQAT shu foydalanuvchining yozuvini belgilaydi
	// (begona ID berilsa hech narsa o'zgarmaydi).
	MarkRead(ctx context.Context, userID, id string) error
	MarkAllRead(ctx context.Context, userID string) error
}

// Pusher — ilova YOPIQ bo'lganda xabar yetkazadi (FCM).
//
// Interfeys sifatida: push sozlanmagan bo'lsa (kalit yo'q) `nil`
// beriladi va butun oqim baribir ishlaydi — WebSocket va DB yozuvi
// o'z ishini qiladi.
type Pusher interface {
	// Push — xato qaytarsa u LOG QILINADI, lekin chaqiruvchi
	// oqimni to'xtatmaydi.
	Push(ctx context.Context, tokens []string, e Event) error
}

// TokenStore — qurilma push tokenlari.
type TokenStore interface {
	// SaveToken — bir xil token boshqa foydalanuvchida bo'lsa, u
	// YANGI egasiga o'tadi (bitta telefonda ikki hisob almashsa).
	SaveToken(ctx context.Context, userID, token, platform string) error
	DeleteToken(ctx context.Context, token string) error
	TokensFor(ctx context.Context, userID string) ([]string, error)
}
