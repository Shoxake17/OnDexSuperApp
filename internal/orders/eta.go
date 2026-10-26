package orders

import (
	"math"
	"sort"
)

// ┌─ YETKAZISH VAQTI ENDI HAQIQIY TARIXDAN ────────────────────────────┐
// `catalog.Restaurant.ETAMinMinutes`/`ETAMaxMinutes` avval admin qo'lda
// kiritardi. Endi — reyting kabi (`internal/ratings`) — HAQIQIY
// buyurtmalar tarixidan hisoblanadi: mijoz buyurtma bergan ondan
// (`Order.CreatedAt`) yetkazilgan/stolga berilgan ongacha (`History`
// dagi "delivered"/"served" yozuvi).
//
// Aniq daqiqa (masalan "23 daqiqa") KO'RSATILMAYDI — har yetkazma
// bir-ikki daqiqa farq qiladi, aniq raqam mijozni "nega doim bir xil
// emas" deb chalg'itadi. Shuning uchun FAQAT 20/25/30/35/40 (5
// daqiqalik qadam, 20–40 oralig'ida) — marketplace ilovalaridagi
// "siljituvchi oyna" uslubi.
// └───────────────────────────────────────────────────────────────────┘

const (
	etaFloorMinutes     = 20
	etaCeilMinutes      = 40
	etaRoundStepMinutes = 5
	// ETASampleSize — hisobga oladigan ENG SO'NGGI yakunlangan
	// buyurtmalar soni. Butun tarix emas: eski buyurtmalar (masalan
	// restoran endi ko'proq xodim yollagan bo'lsa) joriy tezlikni
	// aks ettirmay qoladi.
	ETASampleSize = 30
)

// ComputeETARange — restoranning so'nggi yakunlangan buyurtmalaridan
// yetkazish vaqti oralig'ini hisoblaydi. Ma'lumot yo'q bo'lsa (hali
// birorta ham yetkazilmagan) (0, 0) qaytadi — "ma'lumot yo'q" holati,
// soxta "20–40 daqiqa" HAR DOIM ko'rsatilmaydi.
func ComputeETARange(list []*Order) (minMinutes, maxMinutes int) {
	durations := make([]int, 0, len(list))
	for _, o := range list {
		if o == nil || o.CreatedAt.IsZero() {
			continue
		}
		for _, h := range o.History {
			if h.To != StatusDelivered && h.To != StatusServed {
				continue
			}
			m := int(h.At.Sub(o.CreatedAt).Minutes())
			// Salbiy (soat/sana qo'lda o'zgartirilgan) yoki mantiqsiz
			// uzun qiymatlar chetga chiqariladi — aks holda ular
			// o'rtachani buzib yuborardi. 0 esa RUXSAT ETILADI: juda
			// tez yetkazilgan (yoki sinovda zudlik bilan o'tkazilgan)
			// buyurtma — pastki chegaraga (`etaFloorMinutes`) baribir
			// qisiladi.
			if m >= 0 && m <= 180 {
				durations = append(durations, m)
			}
			break
		}
	}
	if len(durations) == 0 {
		return 0, 0
	}
	sort.Ints(durations)

	lo := roundAndClampETA(durations[0])
	hi := roundAndClampETA(durations[len(durations)-1])
	// Bitta buyurtma (yoki hammasi bir xil 5 daqiqalik "quti"ga
	// tushgan) bo'lsa — oraliq "30–30" ko'rinmasin, kamida bitta
	// qadam kengaytiriladi.
	if hi <= lo {
		if hi < etaCeilMinutes {
			hi = lo + etaRoundStepMinutes
		} else {
			lo = hi - etaRoundStepMinutes
		}
	}
	return lo, hi
}

// roundAndClampETA — eng yaqin 5 daqiqaga yaxlitlaydi, so'ng
// [etaFloorMinutes, etaCeilMinutes] oralig'iga qisadi.
func roundAndClampETA(m int) int {
	rounded := int(math.Round(float64(m)/etaRoundStepMinutes)) * etaRoundStepMinutes
	if rounded < etaFloorMinutes {
		rounded = etaFloorMinutes
	}
	if rounded > etaCeilMinutes {
		rounded = etaCeilMinutes
	}
	return rounded
}
