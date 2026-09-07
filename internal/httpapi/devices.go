package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"chustapp/internal/users"
)

// ┌─ QAYSI ILOVADAN KIRGANI QANDAY QAYD ETILADI ──────────────────────┐
// Har bir autentifikatsiyalangan so'rovda mijoz `X-Ondex-Client`
// sarlavhasini yuboradi (`android/1.4.0`, `tma/1.4.0`, ...). Bu yerda
// u o'qiladi va `user_devices` jadvaliga yoziladi — superadmin
// panelidagi "Qurilma" ustuni AYNAN shu manbadan.
//
// UCH QOIDA (uchalasi ham ataylab):
//
//  1. SO'ROVNI HECH QACHON SEKINLASHTIRMAYDI. Yozuv alohida
//     goroutine'da va o'z muddati bilan ketadi. Baza sekinlashsa
//     foydalanuvchi buni sezmaydi — bu tahliliy ma'lumot, uning
//     yozilmasligi so'rovni rad etish uchun sabab emas.
//
//  2. HAR SO'ROVDA EMAS. Ilova daqiqasiga o'nlab so'rov yuboradi;
//     har biri uchun `UPDATE` qilish bazani bekorga yuklardi.
//     Shuning uchun bitta (foydalanuvchi, platforma) juftligi uchun
//     `touchInterval` da bir martadan ko'p yozilmaydi.
//
//  3. XATO JIM YUTILMAYDI, LEKIN KO'TARILMAYDI HAM: `slog.Debug` —
//     nosozlikni izlash mumkin, lekin log oqimi to'lib ketmaydi.
//
// └───────────────────────────────────────────────────────────────────┘

const (
	// clientHeader — mijoz dasturini bildiruvchi sarlavha nomi.
	clientHeader = "X-Ondex-Client"

	// touchInterval — bir xil (foydalanuvchi, platforma) uchun bazaga
	// yozishlar orasidagi eng qisqa masofa.
	touchInterval = 10 * time.Minute

	// touchCacheLimit — throttle xaritasining yuqori chegarasi.
	// Chegaraga yetganda xarita BUTUNLAY tozalanadi: bu eng yomon
	// holatda bir marta ortiqcha yozuvga olib keladi, lekin xotira
	// cheksiz o'smasligini kafolatlaydi (LRU murakkabligisiz).
	touchCacheLimit = 50_000
)

var deviceTouch = struct {
	mu   sync.Mutex
	last map[string]time.Time // "<userID>|<platform>" -> oxirgi yozuv
}{last: make(map[string]time.Time)}

// shouldTouch — shu juftlik uchun hozir yozish kerakmi.
func shouldTouch(userID, platform string) bool {
	key := userID + "|" + platform
	now := time.Now()

	deviceTouch.mu.Lock()
	defer deviceTouch.mu.Unlock()
	if t, ok := deviceTouch.last[key]; ok && now.Sub(t) < touchInterval {
		return false
	}
	if len(deviceTouch.last) >= touchCacheLimit {
		deviceTouch.last = make(map[string]time.Time)
	}
	deviceTouch.last[key] = now
	return true
}

// recordDevice — `auth()` dan chaqiriladi (token tekshirilgandan KEYIN).
//
// Sarlavha yo'q bo'lsa ham yozuv qilinadi (`unknown`): "qaysidir eski
// build" ham ma'lumot, uni ko'rinmas qilib qo'yish esa panelda
// "qurilmasi yo'q" degan noto'g'ri taassurot berardi.
func (s *Server) recordDevice(r *http.Request, userID string) {
	if s.Devices == nil || userID == "" {
		return
	}
	platform, version := users.ParseClientHeader(r.Header.Get(clientHeader))
	if !shouldTouch(userID, platform) {
		return
	}
	// `safeGo` — recover bilan (bug.md 44-band): bu goroutine
	// `net/http` ning panic tutuvchisidan tashqarida ishlaydi.
	safeGo("devices.touch", func() {
		// So'rov konteksti javob yozilishi bilan bekor qilinadi —
		// shuning uchun BOG'LIQ BO'LMAGAN kontekst va o'z muddati.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.Devices.Touch(ctx, userID, platform, version); err != nil {
			slog.Debug("qurilma yozuvini yangilab bo'lmadi",
				"user", userID, "platform", platform, "err", err)
		}
	})
}
