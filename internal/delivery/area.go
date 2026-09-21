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

// ErrOtherCity — restoran va yetkazish manzili boshqa-boshqa shaharlarda.
var ErrOtherCity = errors.New("bu restoran sizning shahringizga yetkazmaydi")

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

// Cities — xizmat ko'rsatiladigan shaharlar. Yangi shahar qo'shish uchun
// shu ro'yxatga bitta qator qo'shiladi (va `service-area.ts` ga ham —
// `area_test.go` ikkalasini solishtiradi).
//
// Toshkent markazi — Amir Temur xiyoboni. 20 km shaharning halqa yo'li
// ichidagi barcha tumanlarini (Sergeli, Yangihayot, Yunusobod, Chilonzor
// chekkalari) qamraydi, lekin viloyat shaharlariga (Chirchiq, Nurafshon,
// Yangiyo'l — 30 km+) chiqmaydi. Shaharlar bir-biridan ~170 km uzoqda,
// ya'ni doiralar ham, operatsion mintaqalar ham kesishmaydi.
var Cities = []City{
	{Name: "Chust", Lat: 41.0004, Lng: 71.2394, RadiusKM: 8},
	{Name: "Toshkent", Lat: 41.3111, Lng: 69.2797, RadiusKM: 20},
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

// nearestCity — nuqtaga ENG YAQIN shahar, agar u `maxKM` ichida bo'lsa.
func nearestCity(lat, lng, maxKM float64) *City {
	if !ValidCoords(lat, lng) {
		return nil
	}
	var best *City
	bestKM := maxKM
	for i := range Cities {
		c := &Cities[i]
		if d := distanceKM(lat, lng, c.Lat, c.Lng); d <= bestKM {
			best, bestKM = c, d
		}
	}
	return best
}

// CheckServes — restoran shu manzilga yetkazib bera oladimi (shahar
// bo'yicha). Manzilning o'zi xizmat hududida ekani (`Covered`) chaqiruvchi
// tomonidan ALOHIDA tekshiriladi.
//
// ┌─ NEGA KERAK (Toshkent qo'shilganda) ──────────────────────────────┐
// Buyurtmada restoran↔mijoz masofasi hech qachon tekshirilmagan: bitta
// shahar bo'lganda hamma restoran va hamma manzil baribir Chustda edi.
// Ikkinchi shahar bilan Chustdagi mijoz Toshkentdagi restorandan
// buyurtma bera olardi (~300 km) — kuryer esa faqat restorandan 7 km
// ichida qidiriladi, ya'ni buyurtma hech qachon yetkazilmasdi.
// └───────────────────────────────────────────────────────────────────┘
//
// Restoranning shahri xizmat radiusi bilan EMAS, operatsion radius
// (`OperationalRadiusKM`) bilan aniqlanadi: restoran shahar chekkasida,
// doiradan biroz tashqarida turishi mumkin. Koordinatasi umuman yo'q
// (0,0) yoki hech bir shaharga yaqin bo'lmagan restoran uchun cheklov
// qo'yilmaydi — avvalgi xatti-harakat saqlanadi.
func CheckServes(restLat, restLng, addrLat, addrLng float64) error {
	rc := nearestCity(restLat, restLng, OperationalRadiusKM)
	if rc == nil {
		return nil
	}
	ac := CityFor(addrLat, addrLng)
	if ac == nil || ac.Name != rc.Name {
		return ErrOtherCity
	}
	return nil
}
