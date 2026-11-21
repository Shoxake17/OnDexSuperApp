package users

import (
	"crypto/sha256"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Role     Role   `json:"role"`
	EntityID string `json:"entity_id,omitempty"`
	// PhoneProven — token SMS kod tasdig'i orqali berilgan (`Verify`),
	// ya'ni egasi shu telefon raqamiga HOZIRGINA ega ekanini isbotladi.
	//
	// Bu FAQAT parol o'rnatishda ishlatiladi (`POST /me/password`):
	// parolini unutgan odam joriy parolni ayta olmaydi, lekin SMS kod
	// egalikni kamida parol darajasida isbotlaydi. Boshqa hech qanday
	// imtiyoz bermaydi.
	PhoneProven bool `json:"pv,omitempty"`
	jwt.RegisteredClaims
}

// PhoneProofWindow — SMS tasdig'i "yangi" hisoblanadigan muddat.
//
// NEGA CHEKLANGAN: `PhoneProven` tokenga 30 kunga yozilib qolsa,
// o'g'irlangan token butun shu muddat davomida parolni almashtirish
// (va haqiqiy egani `Revoke` bilan tizimdan chiqarib yuborish)
// huquqini berardi. 15 daqiqa — kirgandan keyin parol qo'yish uchun
// yetarli, hujum oynasi uchun esa juda tor.
const PhoneProofWindow = 15 * time.Minute

// HasFreshPhoneProof — token SMS tasdig'i orqali va YAQINDA berilganmi.
func (c *Claims) HasFreshPhoneProof(now time.Time) bool {
	if c == nil || !c.PhoneProven || c.IssuedAt == nil {
		return false
	}
	d := now.Sub(c.IssuedAt.Time)
	return d >= 0 && d <= PhoneProofWindow
}

// ┌─ ESKI HS256 TOKEN YO'Q (Plane ONDEXECOSY-483, 2026-11-21) ─────────┐
// Bu paket endi KIRISH TOKENI CHIQARMAYDI va QABUL QILMAYDI. Barcha
// sessiyalar markaziy identity xizmatida (EdDSA, refresh rotatsiyasi,
// qurilma bo'yicha bekor qilish). ``Claims`` — middleware markaziy
// tokendan quradigan ichki ko'rinish (``httpapi.parseBearer``).
// └───────────────────────────────────────────────────────────────────┘

// Kirish usuli (AMR, RFC 8176 ruhida) — login funksiyalari birinchi qiymat
// sifatida qaytaradi; markaziy sessiyaga yoziladi.
const (
	AMROTP      = "otp"      // telefon kodi yoki Firebase telefon tasdig'i
	AMREmail    = "email"    // email kodi
	AMRPassword = "pwd"      // parol
	AMRGoogle   = "google"   // Google (Firebase) hisobi
	AMRTelegram = "telegram" // Telegram Mini App initData
)

// codePepper — OTP kodlarini hashlash uchun SERVER TOMONIDAGI kalit
// (bug.md 48-band).
//
// JWT sirining O'ZI emas, undan HKDF-ga o'xshash yo'l bilan olingan
// ALOHIDA kalit: bitta sir ikki maqsadda ishlatilsa, birining oqishi
// ikkinchisini ham ochadi. Formula O'ZGARMASIN: aks holda deploy paytida
// kutilayotgan kodlar yaroqsiz bo'lib qoladi.
func codePepper(secret string) []byte {
	sum := sha256.Sum256(append([]byte("ondex-otp-pepper-v1:"), []byte(secret)...))
	return sum[:]
}
