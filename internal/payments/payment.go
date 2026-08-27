// Package payments — karta orqali to'lov (provayderdan MUSTAQIL qatlam).
//
// ┌─ NEGA PROVAYDERDAN MUSTAQIL ──────────────────────────────────────┐
// Hozir Octo ulanmoqda, ertaga Payme yoki Click qo'shilishi mumkin.
// Buyurtma kodi (`internal/orders`) provayderni BILMASLIGI kerak — aks
// holda har yangi provayder narxlash/holat mantig'iga tegib, pul
// yo'lidagi xatarni oshirardi.
//
// Shuning uchun bu paketda: umumiy tushunchalar (holat, summa, xato) va
// `Provider` interfeysi. Provayderga xos kod — kichik ichki paketlarda
// (`payments/octo`).
// └───────────────────────────────────────────────────────────────────┘
package payments

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Status — to'lovning BIZNING tizimimizdagi holati. Provayder statuslari
// shu ro'yxatga MOSLASHTIRILADI, ya'ni buyurtma kodi Octo'ning
// `waiting_for_capture` kabi atamalarini umuman bilmaydi.
type Status string

const (
	// StatusPending — to'lov yaratildi, mijoz hali to'lamadi.
	StatusPending Status = "pending"
	// StatusHeld — pul BLOKLANDI (hold), lekin hali yechilmadi.
	// Ikki bosqichli to'lovning asosiy holati: buyurtma restoran
	// tomonidan qabul qilinsa yechiladi, rad etilsa bo'shatiladi.
	StatusHeld Status = "held"
	// StatusPaid — pul haqiqatan yechildi.
	StatusPaid Status = "paid"
	// StatusCanceled — to'lov bekor qilindi yoki hold bo'shatildi
	// (mijozdan pul YECHILMAGAN).
	StatusCanceled Status = "canceled"
	// StatusFailed — to'lov amalga oshmadi (karta rad etdi, muddat
	// tugadi va h.k.).
	StatusFailed Status = "failed"
	// StatusRefunded — yechilgan pul mijozga qaytarildi.
	StatusRefunded Status = "refunded"
)

// IsFinal — holat o'zgarmaydigan yakuniy holatmi.
func (s Status) IsFinal() bool {
	switch s {
	case StatusPaid, StatusCanceled, StatusFailed, StatusRefunded:
		return true
	default:
		return false
	}
}

var (
	// ErrNotFound — provayderda bunday to'lov yo'q.
	ErrNotFound = errors.New("to'lov topilmadi")
	// ErrNotConfigured — provayder sozlanmagan (.env da kalitlar yo'q).
	ErrNotConfigured = errors.New("to'lov tizimi sozlanmagan")
)

// ProviderError — provayder qaytargan xato (kod + xabar). Alohida tur:
// chaqiruvchi "tarmoq uzildi" bilan "provayder rad etdi" ni ajrata
// olishi kerak — birinchisida qayta urinish mumkin, ikkinchisida yo'q.
type ProviderError struct {
	Code    int
	Message string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("to'lov provayderi xatosi %d: %s", e.Code, e.Message)
}

// CreateRequest — yangi to'lov yaratish so'rovi.
//
// Summa TIYINDA. Provayder so'm/dollar kabi o'nlik son kutsa, o'girish
// AYNAN o'sha provayder paketining ichida bo'ladi (float64 orqali EMAS —
// `DecimalFromTiyin` ga qarang).
type CreateRequest struct {
	// PaymentID — BIZNING tomondagi unikal to'lov identifikatori.
	// Provayderga `shop_transaction_id` sifatida ketadi va takroriy
	// so'rovdan himoya qiladi (idempotentlik).
	PaymentID   string
	AmountTiyin int64
	Description string

	// Hold — true bo'lsa pul darhol yechilmaydi, BLOKLANADI
	// (`auto_capture=false`). Buyurtma qabul qilinganda `Capture`,
	// rad etilganda `Cancel` chaqiriladi.
	Hold bool

	// ReturnURL — to'lovdan keyin mijoz qaytariladigan manzil.
	ReturnURL string
	// NotifyURL — provayder holat o'zgarishini yuboradigan manzil.
	NotifyURL string

	// TTLMinutes — to'lov havolasining yashash muddati (0 = provayder
	// standarti). Muddat tugagach mijoz to'lay olmaydi va buyurtma
	// avtomatik bekor qilinadi.
	TTLMinutes int

	// Language — to'lov sahifasi tili ("uz", "ru", "en").
	Language string

	// Mijoz ma'lumotlari — to'lov sahifasida ko'rsatiladi va
	// provayderning firibgarlikka qarshi tekshiruvida ishlatiladi.
	CustomerID    string
	CustomerPhone string
	CustomerEmail string
}

// CreateResult — yaratilgan to'lov.
type CreateResult struct {
	// ProviderPaymentID — provayder tomondagi ID (keyingi barcha
	// amallar shu bo'yicha: capture, cancel, refund).
	ProviderPaymentID string
	// PayURL — mijoz yo'naltiriladigan to'lov sahifasi.
	PayURL string
	Status Status
}

// StatusResult — provayderdan so'ralgan HAQIQIY holat.
//
// ┌─ NEGA CALLBACK YETARLI EMAS ──────────────────────────────────────┐
// Callback — ochiq internetdan keladigan HTTP so'rov. Uni istalgan
// odam yubora oladi. Shuning uchun har bir xabardan keyin holat
// provayderning O'Z API'sidan qayta so'raladi va buyurtma FAQAT shu
// javob asosida "to'landi" deb belgilanadi. Soxta callback yuborgan
// odam hech narsaga erisha olmaydi.
// └───────────────────────────────────────────────────────────────────┘
type StatusResult struct {
	ProviderPaymentID string
	Status            Status
	// PaidAmountTiyin — provayder tasdiqlagan summa (0 = noma'lum).
	// Buyurtma summasi bilan solishtiriladi: kam to'langan buyurtma
	// oshxonaga tushmasligi kerak.
	PaidAmountTiyin int64
	// RefundedTiyin — qaytarilgan summa.
	RefundedTiyin int64
	// Raw — provayderning xom javobi (diagnostika va nizolar uchun
	// bazaga yoziladi).
	Raw []byte
}

// Provider — to'lov provayderi.
type Provider interface {
	// Name — bazaga yoziladigan qisqa nom ("octo").
	Name() string
	// Create — to'lov yaratadi.
	Create(ctx context.Context, req CreateRequest) (*CreateResult, error)
	// Status — HAQIQIY holatni provayderdan so'raydi.
	Status(ctx context.Context, paymentID string) (*StatusResult, error)
	// Capture — bloklangan pulni yechadi (ikki bosqichli to'lov).
	// `amountTiyin` boshlang'ich summadan KICHIK bo'lishi mumkin.
	Capture(ctx context.Context, providerPaymentID string, amountTiyin int64) error
	// Cancel — blokni bo'shatadi (mijozdan pul yechilmaydi).
	Cancel(ctx context.Context, providerPaymentID string) error
	// Refund — yechilgan pulni qaytaradi. `refundID` — bizning
	// tomondagi unikal ID (takroriy qaytarishdan himoya).
	Refund(ctx context.Context, providerPaymentID, refundID string, amountTiyin int64) error
}

// ═══════════════════════════════════════════════════════════════════
// PUL: TIYIN <-> O'NLIK SON
// ═══════════════════════════════════════════════════════════════════
//
// ┌─ FLOAT64 PUL UCHUN ISHLATILMAYDI ─────────────────────────────────┐
// Provayder JSON'da `437278.66` kabi o'nlik son kutadi/qaytaradi,
// bizda esa hamma narsa TIYINDA (int64) — loyihaning qat'iy qoidasi.
// O'girishni `float64` orqali qilish MUMKIN EMAS: 0.1 + 0.2 = 0.3
// emasligi pulda bir tiyinlik "yo'qolish"ga olib keladi va u har
// tranzaksiyada takrorlanadi. Shuning uchun o'girish MATN orqali,
// butun sonlar bilan bajariladi.
// └───────────────────────────────────────────────────────────────────┘

// DecimalFromTiyin — 1234567 -> "12345.67". JSON'ga AYNAN shu matn
// son sifatida yoziladi (`json.Number`), qo'shtirnoqsiz.
func DecimalFromTiyin(tiyin int64) string {
	sign := ""
	if tiyin < 0 {
		sign = "-"
		tiyin = -tiyin
	}
	return fmt.Sprintf("%s%d.%02d", sign, tiyin/100, tiyin%100)
}

// TiyinFromDecimal — "12345.67" yoki "12345.6" yoki "12345" -> tiyin.
//
// Ikkitadan ortiq kasr xonasi RAD ETILADI: provayder kutilmaganda
// "100.005" yuborsa, uni jimgina yaxlitlash pul hisobida farq
// yaratardi — bunday holatda xato qaytarib, odam qarab chiqqani
// to'g'riroq.
func TiyinFromDecimal(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("summa bo'sh")
	}
	neg := false
	switch s[0] {
	case '-':
		neg, s = true, s[1:]
	case '+':
		s = s[1:]
	}
	whole, frac, hasFrac := strings.Cut(s, ".")
	if whole == "" {
		whole = "0"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("summani o'qib bo'lmadi: %q", s)
	}
	var f int64
	if hasFrac {
		switch len(frac) {
		case 0:
			// "100." — kasr qismi yo'q.
		case 1:
			if f, err = strconv.ParseInt(frac, 10, 64); err != nil {
				return 0, fmt.Errorf("summani o'qib bo'lmadi: %q", s)
			}
			f *= 10
		case 2:
			if f, err = strconv.ParseInt(frac, 10, 64); err != nil {
				return 0, fmt.Errorf("summani o'qib bo'lmadi: %q", s)
			}
		default:
			// Oxiridagi nollar zarar qilmaydi: "100.6600" == "100.66".
			if strings.Trim(frac[2:], "0") != "" {
				return 0, fmt.Errorf("summada tiyindan mayda qism bor: %q", s)
			}
			if f, err = strconv.ParseInt(frac[:2], 10, 64); err != nil {
				return 0, fmt.Errorf("summani o'qib bo'lmadi: %q", s)
			}
		}
	}
	total := w*100 + f
	if neg {
		total = -total
	}
	return total, nil
}
