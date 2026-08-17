package users

import (
	"context"
	"strings"
	"time"
)

// ┌─ MIJOZ DASTURINI ANIQLASH ────────────────────────────────────────┐
// Har bir ilova so'rovda `X-Ondex-Client: <platforma>/<versiya>`
// sarlavhasini yuboradi (masalan `android/1.4.0`, `tma/1.4.0`).
//
// NEGA `User-Agent` EMAS: mobil ilovaning User-Agent'i `Dart/3.5
// (dart:io)` ko'rinishida bo'ladi va u mijoz ilovasini kuryer
// ilovasidan yoki Telegram Mini App'ni oddiy brauzerdan AJRATMAYDI.
// TMA esa brauzerdan faqat sahifa ichidagi `Telegram.WebApp` obyekti
// bilan farq qiladi — buni server hech qachon o'zi ko'ra olmaydi,
// mijoz aytishi kerak.
//
// ISHONCH DARAJASI: bu sarlavhani mijoz yozadi, ya'ni u SOXTALASHTIRISH
// MUMKIN. Shu sabab u faqat ma'lumot uchun (admin paneli statistikasi)
// ishlatiladi va HECH QANDAY huquq/qaror shu qiymatga bog'lanmaydi.
// └───────────────────────────────────────────────────────────────────┘

// Platform — qo'llab-quvvatlanadigan mijoz turlari. Ro'yxat YOPIQ:
// notanish qiymat `PlatformUnknown` ga aylantiriladi, bazaga mijoz
// yozgan ixtiyoriy satr tushmaydi (log/panel ichiga begona matn
// kiritishning oldi olinadi).
const (
	PlatformTMA     = "tma"     // Telegram Mini App (apps/web, WebApp ichida)
	PlatformAndroid = "android" // mobil ilova
	PlatformIOS     = "ios"     // mobil ilova
	PlatformWeb     = "web"     // oddiy brauzer (Telegram'siz)
	PlatformWindows = "windows" // desktop panel
	PlatformMacOS   = "macos"
	PlatformLinux   = "linux"
	PlatformUnknown = "unknown"
)

var knownPlatforms = map[string]string{
	PlatformTMA:     PlatformTMA,
	PlatformAndroid: PlatformAndroid,
	PlatformIOS:     PlatformIOS,
	PlatformWeb:     PlatformWeb,
	PlatformWindows: PlatformWindows,
	PlatformMacOS:   PlatformMacOS,
	PlatformLinux:   PlatformLinux,
}

// maxVersionLen — ilova versiyasi uchun chegara. Uzun satr kesilmaydi,
// BUTUNLAY tashlanadi: "1.4.0" dan uzun narsa yuborgan mijoz allaqachon
// shartnomani buzgan va uning qiymatini qisman saqlash foydasiz.
const maxVersionLen = 32

// ParseClientHeader — `X-Ondex-Client` sarlavhasini platformа va
// versiyaga ajratadi. Sarlavha bo'sh yoki tushunarsiz bo'lsa
// (`unknown`, "") qaytadi — xato QAYTARILMAYDI, chunki bu tahliliy
// ma'lumot va uning yo'qligi so'rovni rad etish uchun sabab emas.
func ParseClientHeader(h string) (platform, version string) {
	h = strings.TrimSpace(h)
	if h == "" {
		return PlatformUnknown, ""
	}
	rawPlatform, rawVersion, _ := strings.Cut(h, "/")
	return NormalizePlatform(rawPlatform), sanitizeVersion(rawVersion)
}

// NormalizePlatform — mijoz aytgan platformani yopiq ro'yxatga soladi.
func NormalizePlatform(p string) string {
	if known, ok := knownPlatforms[strings.ToLower(strings.TrimSpace(p))]; ok {
		return known
	}
	return PlatformUnknown
}

// sanitizeVersion — versiya satridan faqat kutilgan belgilarni
// o'tkazadi. Bu qiymat keyin admin panelida KO'RSATILADI va logga
// yoziladi, shuning uchun boshqaruv belgilari (yangi qator — log
// injection) va uzun satrlar bu yerda to'xtatiladi.
func sanitizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || len(v) > maxVersionLen {
		return ""
	}
	for _, r := range v {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r == '.', r == '-', r == '_', r == '+':
		default:
			return ""
		}
	}
	return v
}

// Device — foydalanuvchi bir platformadan kirgani haqidagi yozuv.
type Device struct {
	Platform   string    `json:"platform"`
	AppVersion string    `json:"app_version,omitempty"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
}

// DeviceStore — foydalanuvchi qaysi mijoz dasturidan kirganini qayd
// etuvchi ombor. `Repository` dan ALOHIDA: bu tahliliy ma'lumot va u
// yo'q bo'lsa ham autentifikatsiya to'liq ishlashi kerak (shuning
// uchun chaqiruv joylarida `nil` tekshiriladi).
type DeviceStore interface {
	// Touch — yozuvni yaratadi yoki `last_seen`/versiyani yangilaydi.
	Touch(ctx context.Context, userID, platform, appVersion string) error
	// ListByUsers — bir necha foydalanuvchining qurilmalarini BITTA
	// so'rovda oladi (admin ro'yxati uchun; har qator uchun alohida
	// so'rov N+1 muammosi bo'lardi).
	ListByUsers(ctx context.Context, userIDs []string) (map[string][]Device, error)
}
