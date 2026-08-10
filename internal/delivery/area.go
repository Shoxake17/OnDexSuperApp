// Package delivery — yetkazib berish qamrovi (xizmat hududi).
//
// MUHIM: bu ro'yxat frontenddagi `apps/web/lib/service-area.ts` bilan
// BIR XIL bo'lishi kerak. Frontend faqat FOYDALANUVCHIGA qulaylik uchun
// tekshiradi (tugmani o'chiradi, xabar chiqaradi); HAQIQIY, ishonchli
// tekshiruv — SHU YERDA, serverda. Frontend chetlab o'tilsa ham
// (to'g'ridan-to'g'ri API chaqiruvi) hudud tashqarisiga buyurtma
// yaratilmaydi.
package delivery

import (
	"errors"
	"math"
)

// ErrBadCoords — koordinata umuman haqiqiy emas (Yer sharida yo'q,
// NaN/Inf, yoki aniq "nol orol" 0,0).
var ErrBadCoords = errors.New("koordinata noto'g'ri")

// ErrOutsideArea — koordinata haqiqiy, lekin xizmat hududidan tashqarida.
var ErrOutsideArea = errors.New("bu nuqta xizmat hududidan tashqarida")

// ValidCoords — koordinatani ENG ASOSIY tekshiruvdan o'tkazadi.
//
// Nima uchun kerak: kuryer joylashuvi, manzil tanlash va geokodlash
// endpoint'lari JSON'dan `float64` o'qiydi. Tekshiruvsiz `NaN` yuborilsa
// u haversine hisobida tarqalib ketadi (har qanday taqqoslash false
// bo'ladi — masofa saralash buziladi), `1e308` esa toshib ketadi, `0,0`
// esa "Nol oroli" — GPS ishlamaganda ilova yuboradigan odatiy soxta
// qiymat.
func ValidCoords(lat, lng float64) bool {
	if math.IsNaN(lat) || math.IsNaN(lng) || math.IsInf(lat, 0) || math.IsInf(lng, 0) {
		return false
	}
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return false
	}
	// (0,0) — Atlantika okeanidagi nuqta; hech qachon haqiqiy mijoz
	// yoki kuryer joylashuvi emas, ammo GPS xatosida tez-tez keladi.
	if lat == 0 && lng == 0 {
		return false
	}
	return true
}

// CheckPoint — ValidCoords + hudud tekshiruvi, bitta joyda.
// Handler'lar aynan shu funksiyani chaqiradi, shunda ikkita tekshiruvdan
// birini unutib qo'yish imkoni bo'lmaydi.
func CheckPoint(lat, lng float64) error {
	if !ValidCoords(lat, lng) {
		return ErrBadCoords
	}
	if !Covered(lat, lng) {
		return ErrOutsideArea
	}
	return nil
}

// City — xizmat ko'rsatiladigan shahar va uning radiusi.
type City struct {
	Name     string
	Lat      float64
	Lng      float64
	RadiusKM float64
}

// Cities — HOZIRCHA FAQAT CHUST. Yangi shahar qo'shish uchun shu
// ro'yxatga bitta qator qo'shiladi (va `service-area.ts` ga ham).
var Cities = []City{
	{Name: "Chust", Lat: 41.0004, Lng: 71.2394, RadiusKM: 8},
}

// distanceKM — ikki nuqta orasidagi masofa (haversine).
func distanceKM(aLat, aLng, bLat, bLng float64) float64 {
	const earthR = 6371.0
	dLat := (bLat - aLat) * math.Pi / 180
	dLng := (bLng - aLng) * math.Pi / 180
	s := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(aLat*math.Pi/180)*math.Cos(bLat*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthR * math.Asin(math.Sqrt(s))
}

// CityFor — nuqta qamrab olingan bo'lsa shaharni, aks holda nil qaytaradi.
func CityFor(lat, lng float64) *City {
	for i := range Cities {
		c := &Cities[i]
		if distanceKM(lat, lng, c.Lat, c.Lng) <= c.RadiusKM {
			return c
		}
	}
	return nil
}

// Covered — qisqa yordamchi.
func Covered(lat, lng float64) bool { return CityFor(lat, lng) != nil }
