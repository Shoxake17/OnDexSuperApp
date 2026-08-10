package couriers

import (
	"sort"
	"time"
)

// ScoredCandidate — bitta nomzod-kuryer, Google Distance Matrix'dan olingan
// ETA va hisoblangan ball bilan.
type ScoredCandidate struct {
	Courier *Courier
	ETA     time.Duration
	Score   float64
}

// Ustuvorlik og'irliklari — sozlanuvchi konstantalar, bir joyda.
const (
	// idealWindowMinRatio/MaxRatio — ETA prep_time'ning shu foizlari
	// oralig'ida bo'lsa "ideal" hisoblanadi. Masalan 20 daqiqalik
	// tayyorlash uchun 5-10 daqiqa masofadagi kuryer ustuvor — foydalanuvchi
	// bergan aniq misolga mos (0.25 va 0.5).
	idealWindowMinRatio = 0.25
	idealWindowMaxRatio = 0.5

	maxTimingScore  = 100.0
	earlyPenaltyMax = 20.0 // juda erta kelish uchun MAKSIMAL jarima
	latePenaltyMax  = 90.0 // juda kech qolish uchun MAKSIMAL jarima (ballni 10gacha tushiradi — lekin 0'ga emas: hech kim topilmasligidan ko'ra kech bo'lsa ham BIRON kimdir topilgani afzal)

	ratingWeight     = 10.0 // (rating-3.0)*ratingWeight — 1..5 oralig'ida ±20
	experienceCap    = 50   // shuncha buyurtmadan keyin tajriba bonusi to'yinadi
	experienceWeight = 10.0 // maksimal tajriba bonusi
)

// ScoreCandidates — har bir nomzodga ball beradi va ENG YUQORIDAN ENG
// PASTGA saralaydi (natija[0] — birinchi taklif yuboriladigan kuryer).
//
// Ball tarkibi:
//   - Vaqt mosligi (0-100, ASOSIY omil): kuryerning ETA'si prep_time'ning
//     [25%, 50%] oralig'ida bo'lsa — 100 ball (masalan 20 daqiqalik
//     tayyorlashda 5-10 daqiqa masofadagi kuryer — aynan shu oraliq
//     "ovqat sovib qolmasdan, lekin kuryer bekorga kutmasdan yetib keladi"
//     degani). Bu oraliqdan tashqarida jarima qo'llaniladi: juda erta
//     kelish YENGIL jarima, juda kech qolish ANCHA OG'IRROQ (mijozga sovuq
//     ovqat yetib borishi — eng yomon holat, undan qochish ustuvor).
//   - Reyting bonusi (±20): (rating-3.0)*10. HOZIRCHA rating har doim
//     boshlang'ich 5.0 qiymatida — haqiqiy "mijoz kuryerni baholaydi"
//     tizimi hali qurilmagan (soxta raqam emas, ataylab neytral standart).
//   - Tajriba bonusi (0..+10): tugatilgan buyurtmalar soniga qarab, 50
//     tadan keyin to'yinadi — 100% haqiqiy va obyektiv mezon.
//
// MUHIM: transport turi ALOHIDA ball sifatida hisobga OLINMAYDI — u
// allaqachon Google Distance Matrix orqali olingan ETA'ning o'zida aks
// etgan (piyoda kuryerning "walking" ETA'si tabiiy ravishda moped/
// mashinaning "driving" ETA'sidan uzunroq chiqadi). Buni yana alohida ball
// sifatida qo'shish signalni ikki marta hisoblash (double-counting) bo'lar
// edi va uzoqdagi mashinali kuryerni yaqindagi piyodadan sun'iy ravishda
// ustun qo'yib yuborardi.
func ScoreCandidates(candidates []ScoredCandidate, prepTime time.Duration) []ScoredCandidate {
	idealMin := time.Duration(float64(prepTime) * idealWindowMinRatio)
	idealMax := time.Duration(float64(prepTime) * idealWindowMaxRatio)

	out := make([]ScoredCandidate, len(candidates))
	copy(out, candidates)
	for i := range out {
		out[i].Score = timingScore(out[i].ETA, idealMin, idealMax) +
			ratingBonus(out[i].Courier.Rating) +
			experienceBonus(out[i].Courier.CompletedOrders)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func timingScore(eta, idealMin, idealMax time.Duration) float64 {
	if idealMax <= 0 {
		// prep_time berilmagan/nolga teng (masalan tayyorlash vaqti
		// belgilanmagan) — ideal oyna yo'q, shunchaki ENG TEZ yetib
		// keladigan kuryerni afzal ko'ramiz (yaqin = yaxshi).
		penalty := eta.Minutes() * 5
		if penalty > latePenaltyMax {
			penalty = latePenaltyMax
		}
		return maxTimingScore - penalty
	}
	switch {
	case eta <= idealMin:
		if idealMin <= 0 {
			return maxTimingScore
		}
		frac := float64(idealMin-eta) / float64(idealMin)
		return maxTimingScore - frac*earlyPenaltyMax
	case eta <= idealMax:
		return maxTimingScore
	default:
		frac := float64(eta-idealMax) / float64(idealMax)
		penalty := frac * latePenaltyMax
		if penalty > latePenaltyMax {
			penalty = latePenaltyMax
		}
		return maxTimingScore - penalty
	}
}

func ratingBonus(rating float64) float64 {
	return (rating - 3.0) * ratingWeight
}

func experienceBonus(completed int) float64 {
	if completed > experienceCap {
		completed = experienceCap
	}
	return float64(completed) / float64(experienceCap) * experienceWeight
}
