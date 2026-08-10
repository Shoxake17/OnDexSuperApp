// HTTP javob yozishning YAGONA joyi.
//
// Handlerlar hech qachon `w.Write`/`json.NewEncoder(w)` ni to'g'ridan-
// to'g'ri chaqirmaydi — faqat shu ikki funksiya orqali. Shu sababli
// xato formati butun API bo'ylab bir xil bo'ladi va 5xx tafsilotini
// yashirish qoidasi bitta joyda kafolatlanadi.
package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// NewID — 8 baytlik kriptografik tasodifiy identifikator.
// Eksport qilingan: main() ham xizmatlarga uzatish uchun ishlatadi.
func NewID() string {
	b := make([]byte, 8)
	// XATO E'TIBORSIZ QOLDIRILMAYDI: `rand.Read` muvaffaqiyatsiz bo'lsa
	// `b` nol bo'lib qolar va ID'lar BASHORATLI/takrorlanuvchi bo'lardi
	// (buyurtma/foydalanuvchi identifikatorlari uchun jiddiy).
	// Bunday holat amalda deyarli bo'lmaydi va tiklab bo'lmaydi.
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand ishlamadi: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// tooManyRequests — tezlik cheklovi javobi, ANIQ kutish vaqti bilan.
//
// NEGA ALOHIDA FUNKSIYA: avval hamma joyda "juda ko'p urinish — biroz
// kuting" yozilardi. Foydalanuvchi uchun bu ma'lumotsiz xabar: u 5
// soniyadan keyin ham, 10 daqiqadan keyin ham qayta urinadi va o'sha
// javobni oladi — natijada ilovani buzuq deb o'ylaydi. Endi qancha
// kutish kerakligi aniq aytiladi.
//
// `Retry-After` sarlavhasi ham qo'yiladi — bu HTTP standarti
// (RFC 9110) va mijoz uni avtomatik hisoblagich uchun ishlatishi
// mumkin (matnni tahlil qilmasdan).
func tooManyRequests(w http.ResponseWriter, wait time.Duration) {
	secs := int(wait.Seconds())
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{
		"error":       "Juda ko'p urinish — " + humanDuration(secs) + " kuting",
		"retry_after": secs,
	})
}

// humanDuration — soniyalarni o'zbekcha, o'qilishi oson ko'rinishga
// keltiradi ("45 soniya", "2 daqiqa", "1 daqiqa 30 soniya").
//
// Sof soniyalarda berish ("312 soniya kuting") foydalanuvchini
// hisoblashga majbur qiladi.
func humanDuration(secs int) string {
	if secs < 60 {
		return strconv.Itoa(secs) + " soniya"
	}
	m := secs / 60
	s := secs % 60
	if m >= 60 {
		h := m / 60
		m %= 60
		if m == 0 {
			return strconv.Itoa(h) + " soat"
		}
		return strconv.Itoa(h) + " soat " + strconv.Itoa(m) + " daqiqa"
	}
	if s == 0 {
		return strconv.Itoa(m) + " daqiqa"
	}
	return strconv.Itoa(m) + " daqiqa " + strconv.Itoa(s) + " soniya"
}

// httpError — xatoni mijozga qaytaradi.
//
// MUHIM: 5xx (server ichidagi nosozlik) uchun xatoning HAQIQIY matni
// mijozga YUBORILMAYDI — u pgx/mongo drayveri matni, so'rov qismlari,
// host/port yoki cheklov (constraint) nomlarini oshkor qilishi mumkin.
// Bunday xatolar serverda to'liq log qilinadi, mijozga esa umumiy
// xabar boradi. 4xx — domen xatolari (masalan "savat bo'sh") — mijozga
// tushunarli bo'lishi kerak, shuning uchun ular o'zgarishsiz uzatiladi.
func httpError(w http.ResponseWriter, status int, err error) {
	if status >= 500 {
		slog.Error("server xatosi", "status", status, "err", err)
		writeJSON(w, status, map[string]string{"error": "server xatosi, keyinroq urinib ko'ring"})
		return
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
