// Package mapprovider — xarita manbasini tanlovchi `GOOGLE_MAP` bayrog'i.
//
//	GOOGLE_MAP=true  → hamma joyda Google Maps (standart)
//	GOOGLE_MAP=false → hamma joyda OnDexMap; u ishlamasa Google'ga qaytadi
//
// Bayroq SERVERDA o'qiladi va ilovalarga `GET /config/maps` orqali yetkaziladi
// — ilovani qayta qurmasdan almashtirish mumkin.
//
// Bo'sh yoki tushunarsiz qiymat Google'ni beradi: bu hozirgi production
// xatti-harakati, ya'ni bayroq yozilmagan serverda hech narsa o'zgarmaydi.
package mapprovider

import (
	"log/slog"
	"strings"
)

// Mode — tanlangan xarita manbasi.
type Mode string

const (
	Google   Mode = "google"
	OndexMap Mode = "ondexmap"
)

// Parse — `GOOGLE_MAP` qiymatini o'qiydi. `ok=false` — qiymat bo'sh yoki
// noma'lum edi va standart (Google) tanlandi.
func Parse(raw string) (mode Mode, ok bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "on":
		return Google, true
	case "false", "0", "no", "off":
		return OndexMap, true
	}
	return Google, false
}

// FromEnv — rejimni muhitdan oladi (getenv — testlarda soxta).
func FromEnv(getenv func(string) string) Mode {
	return FromValue(getenv("GOOGLE_MAP"))
}

// FromValue — GOOGLE_MAP qiymatidan rejim. Noto'g'ri yozilgan qiymat jimgina
// o'tib ketmasin: ogohlantirish logga chiqadi.
func FromValue(raw string) Mode {
	mode, ok := Parse(raw)
	if !ok && strings.TrimSpace(raw) != "" {
		slog.Warn("GOOGLE_MAP qiymati tushunarsiz, Google ishlatiladi (true/false kutilgan)",
			"qiymat", raw)
	}
	return mode
}

// Valid — rejim ma'lum qiymatlardan biri.
func (m Mode) Valid() bool { return m == Google || m == OndexMap }
