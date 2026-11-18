package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"fmt"
	"strings"

	"chustapp/internal/ratelimit"
)

// ┌─ OLIB KETISH KODI (Plane 473/474) ─────────────────────────────────────┐
// Mijoz restoranga kelib 4 xonali kodni aytadi, xodim uni panelga kiritadi
// va FAQAT shunda buyurtma "berildi" bo'ladi.
//
// Kod BAZADA SAQLANMAYDI — `HMAC(PickupCodeKey, order_id)` dan hisoblanadi.
// Sabab: buyurtma JSON'i ko'p joyga ketadi (restoran paneli, WebSocket,
// statistika). Kod maydon bo'lganda uning restoranga TASODIFAN sizib
// chiqishi bitta unutilgan redaksiya bilan bo'lardi; hisoblanadigan kod esa
// faqat `GET /orders/{id}/pickup-code` (faqat buyurtma egasi) orqali beriladi.
//
// Kalit JWT sirining o'zi emas, undan HOSIL QILINGAN alohida kalit.
// Noto'g'ri kod urinishlari buyurtma bo'yicha cheklangan (taxmin qilish
// 10 000 variant — 5 urinish/5 daq).
// └───────────────────────────────────────────────────────────────────────┘

var pickupCodeLimiter = ratelimit.New(1.0/60, 5).Shared("pickupCodeLimiter")

// DerivePickupCodeKey — main.go: JWT sirdan alohida kalit.
func DerivePickupCodeKey(secret string) []byte {
	sum := sha256.Sum256([]byte("ondex-pickup-code:v1:" + secret))
	return sum[:]
}

func (s *Server) pickupCode(orderID string) string {
	m := hmac.New(sha256.New, s.PickupCodeKey)
	m.Write([]byte(orderID))
	return fmt.Sprintf("%04d", binary.BigEndian.Uint32(m.Sum(nil)[:4])%10000)
}

// checkPickupCode — kodni doimiy vaqtda solishtiradi.
func (s *Server) checkPickupCode(orderID, code string) bool {
	code = strings.TrimSpace(code)
	if len(s.PickupCodeKey) == 0 || len(code) != 4 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(s.pickupCode(orderID)), []byte(code)) == 1
}
