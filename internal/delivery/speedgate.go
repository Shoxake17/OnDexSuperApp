package delivery

import (
	"sync"
	"time"
)

// OperationalRadiusKM — kuryer joylashuvi qabul qilinadigan eng katta
// masofa (xizmat shaharlaridan BIRORTASINING markazidan). Xizmat radiusidan
// ATAYLAB kengroq: kuryer chekka mahallaga chiqib qolishi yoki GPS bir
// necha yuz metr adashishi normal holat. Lekin hech bir xizmat shahriga
// yaqin bo'lmagan soxta koordinata (masalan Samarqanddan) o'tolmaydi.
//
// Shahar bo'yicha AJRATISH bu yerda emas: kuryer faqat restorandan 7 km
// ichida nomzod bo'ladi (`couriers.searchRadiusMeters`), ya'ni Toshkentdagi
// kuryer Chust buyurtmasini baribir olmaydi.
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

// confirmAfter — rad etilgan joy shuncha vaqt IZCHIL kelib tursa, xato
// oldingi ASOS nuqtada deb hisoblanadi va yangi joy qabul qilinadi.
//
// ┌─ NEGA (2026-09-15, jonli log) ────────────────────────────────────┐
// Telefon haqiqiy joy bilan aralash ~1,6 km naridagi AYNAN bir xil
// (tarmoq/kesh) nuqtani yubordi. Shu XATO nuqta asos bo'lib, o'zi qayta
// kelib asosni yangilab turdi va kuryerning HAQIQIY joylashuvi 21:30–21:55
// oralig'ida qayta-qayta "imkonsiz tezlik" deb rad etildi. Joylashuv
// eskirdi, dispatch kuryerni ko'rmay qo'ydi. Soxtalashtiruvchi uchun farq kichik: avval 5 daqiqa jim
// turib o'tardi, endi 1 daqiqa davomida bir joyni izchil yuborishi kerak;
// `InOperationalRange` baribir ishlaydi.
// └───────────────────────────────────────────────────────────────────┘
const confirmAfter = time.Minute

// confirmRadiusKM — izchillikda "o'sha joy" deb hisoblanadigan tarqoqlik.
const confirmRadiusKM = 0.15

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
	// suspect — rad etilgan joy va u BIRINCHI marta kelgan vaqt.
	suspect map[string]lastFix
}

func NewSpeedGate() *SpeedGate {
	return &SpeedGate{last: make(map[string]lastFix), suspect: make(map[string]lastFix)}
}

// Accept — nuqtani qabul qilish mumkinmi. `false` bo'lsa oxirgi ma'lum
// nuqta O'ZGARTIRILMAYDI (soxta qiymat keyingi hisob uchun asos bo'lib
// qolmasligi kerak).
func (g *SpeedGate) Accept(courierID string, lat, lng float64, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if prev, ok := g.last[courierID]; ok && !g.plausible(courierID, prev, lat, lng, now) {
		return false
	}
	g.last[courierID] = lastFix{lat: lat, lng: lng, at: now}
	if s, ok := g.suspect[courierID]; ok && distanceKM(s.lat, s.lng, lat, lng) <= confirmRadiusKM {
		delete(g.suspect, courierID)
	}

	// Oddiy tozalash: xaritalar o'smasligi uchun eskirganlarni olib tashlaymiz.
	if len(g.last) > 1000 {
		for id, f := range g.last {
			if now.Sub(f.at) > staleAfter {
				delete(g.last, id)
			}
		}
	}
	if len(g.suspect) > 1000 {
		for id, f := range g.suspect {
			if now.Sub(f.at) > staleAfter {
				delete(g.suspect, id)
			}
		}
	}
	return true
}

// plausible — mutex chaqiruvchida ushlab turiladi.
func (g *SpeedGate) plausible(courierID string, prev lastFix, lat, lng float64, now time.Time) bool {
	gap := now.Sub(prev.at)
	if gap >= staleAfter || gap <= 0 || gap < minInterval {
		// juda eski (yoki soat orqaga ketgan) yoki juda tez-tez (GPS
		// shovqini tezlikni buzadi) — tekshirmaymiz
		return true
	}
	if distanceKM(prev.lat, prev.lng, lat, lng)/gap.Hours() <= maxSpeedKMH {
		return true
	}
	s, ok := g.suspect[courierID]
	if !ok || now.Sub(s.at) >= staleAfter || distanceKM(s.lat, s.lng, lat, lng) > confirmRadiusKM {
		g.suspect[courierID] = lastFix{lat: lat, lng: lng, at: now}
		return false
	}
	return now.Sub(s.at) >= confirmAfter
}
