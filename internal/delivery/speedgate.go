package delivery

import (
	"sync"
	"time"
)

// OperationalRadiusKM — kuryer joylashuvi qabul qilinadigan eng katta
// masofa (xizmat shahri markazidan). Xizmat radiusi (8 km) dan ATAYLAB
// kengroq: kuryer chekka mahallaga chiqib qolishi yoki GPS bir necha yuz
// metr adashishi normal holat. Lekin "Toshkentdan turib Chustga buyurtma
// olaman" degan soxta koordinata bu chegaradan o'tolmaydi.
const OperationalRadiusKM = 50

// InOperationalRange — nuqta xizmat shaharlaridan birortasining
// operatsion radiusi ichidami.
func InOperationalRange(lat, lng float64) bool {
	if !ValidCoords(lat, lng) {
		return false
	}
	for i := range Cities {
		c := &Cities[i]
		if distanceKM(lat, lng, c.Lat, c.Lng) <= OperationalRadiusKM {
			return true
		}
	}
	return false
}

// maxSpeedKMH — jismonan imkonsiz deb hisoblanadigan tezlik. Moped/mashina
// uchun 150 km/soat allaqachon juda saxiy chegara; undan yuqorisi — GPS
// soxtalashtirish yoki jiddiy xato.
const maxSpeedKMH = 150

// minInterval — bundan tezroq kelgan yangilanishlar tezlik hisobiga
// kiritilmaydi: kichik vaqt oralig'ida GPS shovqini (bir necha o'nlab
// metr) juda katta "tezlik" berib, halol kuryerni noto'g'ri bloklaydi.
const minInterval = 3 * time.Second

// staleAfter — bundan eski oxirgi nuqta unutiladi (kuryer ilovani yopib,
// mashinada boshqa joyga borgan bo'lishi mumkin — bu firibgarlik emas).
const staleAfter = 5 * time.Minute

type lastFix struct {
	lat, lng float64
	at       time.Time
}

// SpeedGate — kuryer koordinatasining "teleport" qilishini aniqlaydi.
//
// Nima uchun xotirada, bazada emas: bu tekshiruv uchun bazaga ustun
// qo'shish (migratsiya) shart emas — u faqat KETMA-KET ikki yangilanish
// orasidagi farqqa qaraydi, tarixiy ma'lumot kerak emas. Server qayta
// ishga tushsa xotira tozalanadi va birinchi nuqta shartsiz qabul
// qilinadi — bu xavfsiz, chunki `InOperationalRange` baribir ishlaydi.
type SpeedGate struct {
	mu   sync.Mutex
	last map[string]lastFix
}

func NewSpeedGate() *SpeedGate { return &SpeedGate{last: make(map[string]lastFix)} }

// Accept — nuqtani qabul qilish mumkinmi. `false` bo'lsa oxirgi ma'lum
// nuqta O'ZGARTIRILMAYDI (soxta qiymat keyingi hisob uchun asos bo'lib
// qolmasligi kerak).
func (g *SpeedGate) Accept(courierID string, lat, lng float64, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	prev, ok := g.last[courierID]
	if ok {
		gap := now.Sub(prev.at)
		switch {
		case gap >= staleAfter || gap <= 0:
			// juda eski (yoki soat orqaga ketgan) — tekshirmaymiz
		case gap < minInterval:
			// juda tez-tez — GPS shovqini tezlikni buzadi, o'tkazamiz
		default:
			km := distanceKM(prev.lat, prev.lng, lat, lng)
			if km/gap.Hours() > maxSpeedKMH {
				return false
			}
		}
	}
	g.last[courierID] = lastFix{lat: lat, lng: lng, at: now}

	// Oddiy tozalash: xarita o'smasligi uchun eskirganlarni olib tashlaymiz.
	if len(g.last) > 1000 {
		for id, f := range g.last {
			if now.Sub(f.at) > staleAfter {
				delete(g.last, id)
			}
		}
	}
	return true
}
