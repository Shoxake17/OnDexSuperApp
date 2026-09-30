package delivery

import (
	"errors"
	"math"
)

// YETKAZIB BERISH NARXI — OnDex kuryeri uchun.
//
// ┌─ QAYERDA ISHLATILADI ─────────────────────────────────────────────┐
// Bu narx FAQAT `delivery_mode = ondex` bo'lgan restoranlarga tegishli.
// Restoran o'z kuryeri bilan ishlasa narxni O'ZI belgilaydi (0 = bepul)
// va bu formulaning unga aloqasi yo'q.
//
// Hisob BITTA joyda: savat summasi (`/quote`), buyurtma jami, fiskal
// chek qatori va (keyinchalik) split oyog'i — hammasi shu funksiyadan
// o'qiydi. Ikki joyda ikki xil hisob bo'lsa mijoz ko'rgan narx bilan
// yechilgan summa farq qilardi.
// └───────────────────────────────────────────────────────────────────┘
//
// ┌─ NEGA METRDA VA BUTUN SONDA ──────────────────────────────────────┐
// Pul loyihada HAR DOIM tiyinda, `int64` (`payments/payment.go`
// izohiga qarang). Masofa ham metrda, `int`: kilometrni `float64` da
// yuqoriga yaxlitlash chegaradagi qiymatlarda beqaror — 2.5 km ba'zan
// 1, ba'zan 2 qo'shimcha kilometr bo'lib chiqardi va bir xil manzil
// har safar boshqa narx ko'rsatardi.
// └───────────────────────────────────────────────────────────────────┘
const (
	// FeeBaseMeters — shu masofagacha narx o'zgarmaydi.
	FeeBaseMeters = 1500

	// FeeBaseTiyin — bazaviy narx (6 000 so'm).
	FeeBaseTiyin int64 = 600_000

	// FeePerKMTiyin — har QO'SHIMCHA kilometr (2 000 so'm). To'liq
	// bo'lmagan kilometr ham to'liq hisoblanadi.
	FeePerKMTiyin int64 = 200_000

	// FeeMaxMeters — undan narida yetkazib berilmaydi.
	//
	// 7 km — operatsion zona: kuryer nomzodlari restorandan aynan shu
	// masofada qidiriladi (`couriers.searchRadiusMeters`). Bu ikkisi
	// bir xil tushuncha EMAS (biri restoran↔kuryer, ikkinchisi
	// restoran↔mijoz), lekin bir xil shahar o'lchamidan kelib
	// chiqqan. Kattaroq qilinsa mijozga narx ko'rsatiladi-yu, ovqat
	// sovib yetib boradi.
	FeeMaxMeters = 7000
)

// ErrTooFar — manzil yetkazib berish zonasidan tashqarida.
//
// `ErrOutsideArea` dan FARQ QILADI: u nuqta umuman xizmat shahrida
// emasligini bildiradi, bu esa shahar ichida bo'lsa ham restorandan
// juda uzoqligini.
var ErrTooFar = errors.New("bu manzil restorandan juda uzoq — yetkazib berilmaydi")

// ErrBadDistance — masofa manfiy (dasturda xato).
var ErrBadDistance = errors.New("masofa manfiy bo'lishi mumkin emas")

// DeliveryFee — `meters` masofa uchun narx (tiyin).
//
// Xato qaytarish ATAYLAB: chaqiruvchi "juda uzoq" holatini e'tiborsiz
// qoldira olmaydi. Nol qaytarilsa u "bepul" deb tushunilib, mijozga
// bajarib bo'lmaydigan buyurtma ko'rsatilardi.
func DeliveryFee(meters int) (int64, error) {
	switch {
	case meters < 0:
		return 0, ErrBadDistance
	case meters > FeeMaxMeters:
		return 0, ErrTooFar
	case meters <= FeeBaseMeters:
		return FeeBaseTiyin, nil
	}
	// Butun sonlarda yuqoriga yaxlitlash: 1501 m -> 1 km, 2500 m -> 1 km,
	// 2501 m -> 2 km.
	extraKM := int64((meters - FeeBaseMeters + 999) / 1000)
	return FeeBaseTiyin + extraKM*FeePerKMTiyin, nil
}

// Deliverable — shu masofaga yetkazib beramizmi.
//
// Narx kerak bo'lmagan joylar uchun (masalan restoran ro'yxatida
// "yetkazib beradi/bermaydi" belgisi) — `DeliveryFee` xatosini
// tekshirishdan ko'ra o'qilishi oson.
func Deliverable(meters int) bool { return meters >= 0 && meters <= FeeMaxMeters }

// DistanceMeters — ikki nuqta orasidagi masofa, metrda (yaxlitlangan).
//
// ┌─ TO'G'RI CHIZIQ, YO'L EMAS ───────────────────────────────────────┐
// Haqiqiy yo'l masofasi (OSRM) adolatliroq bo'lardi, lekin u
// ChustApp'ga YANGI tashqi bog'liqlik qo'shadi: OnDexMap alohida
// mikroservis va u javob bermasa savat summasi umuman hisoblanmay
// qolardi. Savat esa eng ko'p chaqiriladigan yo'l.
//
// Shuning uchun hozir haversine. Yo'l masofasiga o'tilsa, o'zgarish
// FAQAT shu funksiyada bo'ladi va zaxira sifatida haversine qolishi
// kerak.
// └───────────────────────────────────────────────────────────────────┘
func DistanceMeters(aLat, aLng, bLat, bLng float64) int {
	return int(math.Round(distanceKM(aLat, aLng, bLat, bLng) * 1000))
}
