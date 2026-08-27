package promotions

import (
	"sort"
	"time"
)

// CartLine — checkout'da narxlangan bitta savat qatori, aksiya mos
// kelishini tekshirish uchun yetarli minimal ma'lumot (orders.Item'dan
// mustaqil — bu paket orders'ni import qilmaydi, chaqiruvchi o'zi
// moslaydi).
type CartLine struct {
	ProductID      string
	Category       string
	UnitPriceTiyin int64
	Qty            int
}

func (l CartLine) subtotal() int64 { return l.UnitPriceTiyin * int64(l.Qty) }

// AppliedPromotion — bitta aksiyaning savatga qo'shgan HAQIQIY hissasi
// (u yutgan qatorlar yig'indisi).
type AppliedPromotion struct {
	Promotion     *Promotion
	DiscountTiyin int64
}

// Result — savat chegirmasining yakuniy hisobi.
type Result struct {
	// LineDiscounts — har qatorning YAKUNIY chegirmasi. `Apply`ga
	// berilgan `lines` bilan bir xil uzunlik va tartib; yig'indisi HAR
	// DOIM DiscountTiyin ga TENG (invariant), shuning uchun mijoz savatda
	// ko'rgan qator narxlari pastdagi jamiga aniq qo'shiladi.
	LineDiscounts []int64
	// DiscountTiyin — savatning jami chegirmasi.
	DiscountTiyin int64
	// PromotionTiyin — jamining aksiyalar bergan qismi (qolgani —
	// mahsulotlarning o'z chegirma narxlari).
	PromotionTiyin int64
	// Promotions — hissa qo'shgan aksiyalar, hissasi bo'yicha KAMAYISH
	// tartibida. Bo'sh bo'lishi mumkin (faqat mahsulot chegirmalari
	// ishlagan holat).
	Promotions []AppliedPromotion
}

// Primary — eng ko'p hissa qo'shgan aksiya (chekda ko'rsatish va
// buyurtma yozuvi uchun). Hech biri qo'llanmagan bo'lsa nil.
func (r Result) Primary() *AppliedPromotion {
	if len(r.Promotions) == 0 {
		return nil
	}
	return &r.Promotions[0]
}

// Apply — savatga barcha FAOL aksiyalarni qo'llaydi.
//
// ┌─ QOIDA: HAR QATOR O'ZINING ENG YAXSHISINI OLADI ──────────────────┐
// Har bir savat qatori uchun nomzodlar:
//
//   - `baseline[i]` — mahsulotning O'Z chegirma narxidan kelgan
//     chegirma (restoran panelida belgilangan, endi faqat eski
//     yozuvlarda uchraydi);
//   - har bir mos aksiyaning o'sha qatorga to'g'ri keladigan qismi.
//
// Qatorga ULARDAN ENG KATTASI qo'llanadi. Bitta qatorda ikkitasi hech
// qachon QO'SHILMAYDI ("chegirmali narxdan yana chegirma" — bir marta
// 105 850 so'mlik savatni 0 so'mga tushirgan xato), lekin TURLI
// qatorlar TURLI aksiyadan chegirma olishi mumkin.
//
// Avval butun savatga faqat BITTA aksiya qo'llanardi va mijoz menyuda
// ko'rgan narxlarning bir qismini yo'qotardi: 12 000 so'mlik Cola
// (-5 000 aksiya) va 10 000 so'mlik kokteyl (-20% boshqa aksiya)
// savatda 15 000 emas, 17 000 so'm bo'lib chiqardi.
// └───────────────────────────────────────────────────────────────────┘
//
// TypeFreeDelivery HECH QACHON qo'llanmaydi — tizimda yetkazib berish
// narxi tushunchasi umuman yo'q (hech qayerda hisoblanmaydi), ya'ni
// chegirma qiladigan haqiqiy narx yo'q. Bu — soxta chegirma
// ko'rsatmaslik uchun ataylab qilingan tanlov (ROADMAP'da qayd etilgan).
//
// `baseline` `lines` bilan bir xil uzunlikda bo'lishi kutiladi; qisqa
// yoki nil bo'lsa yetmagan qatorlar 0 deb qaraladi.
func Apply(promos []*Promotion, lines []CartLine, baseline []int64, previousOrderCount int, now time.Time) Result {
	res := Result{LineDiscounts: make([]int64, len(lines))}
	baseAt := func(i int) int64 {
		if i < len(baseline) && baseline[i] > 0 {
			return baseline[i]
		}
		return 0
	}
	for i := range lines {
		res.LineDiscounts[i] = baseAt(i)
	}

	// ---- 1. Mos keladigan aksiyalar va ularning qator taqsimoti ----
	subtotal := cartSubtotal(lines)
	type candidate struct {
		promo   *Promotion
		perLine []int64
	}
	var cands []candidate
	for _, p := range promos {
		if !isEligible(p, subtotal, previousOrderCount, now) {
			continue
		}
		perLine := lineDiscounts(p, lines)
		if sumOf(perLine) <= 0 {
			continue
		}
		cands = append(cands, candidate{promo: p, perLine: perLine})
	}

	// ---- 2. Har qatorda eng foydalisini tanlaymiz ----
	// winner[i] — shu qatorda yutgan aksiyaning indeksi (-1 = aksiya
	// yo'q, ya'ni mahsulotning o'z chegirmasi qoldi).
	winner := make([]int, len(lines))
	for i := range lines {
		winner[i] = -1
		best := res.LineDiscounts[i]
		for ci := range cands {
			if cands[ci].perLine[i] > best {
				best = cands[ci].perLine[i]
				winner[i] = ci
			}
		}
		res.LineDiscounts[i] = best
	}

	// ---- 3. "Maksimal chegirma" cheklovi ----
	//
	// Chegara aksiyaning O'ZI YUTGAN qatorlari yig'indisiga qo'llanadi va
	// oshib ketgan qism o'sha qatorlarga mutanosib ravishda
	// kichraytiriladi. Kichraytirishdan keyin qator mahsulotning o'z
	// chegirmasidan past tushib qolmaydi.
	//
	// ATAYLAB SODDA: chegara ishlaganda o'sha qatorda BOSHQA aksiya
	// foydaliroq bo'lib qolishi mumkin, lekin qayta tanlov qilinmaydi —
	// qoida bashorat qilinadigan bo'lib qolishi kerak. Natija hech qachon
	// chegaradan oshmaydi va mahsulot chegirmasidan past bo'lmaydi.
	for ci := range cands {
		limit := cands[ci].promo.MaxDiscountAmountTiyin
		if limit <= 0 {
			continue
		}
		weights := make([]int64, len(lines))
		var granted int64
		for i := range lines {
			if winner[i] == ci {
				weights[i] = res.LineDiscounts[i]
				granted += weights[i]
			}
		}
		if granted <= limit {
			continue
		}
		scaled := Spread(limit, weights)
		for i := range lines {
			if winner[i] != ci {
				continue
			}
			if base := baseAt(i); base >= scaled[i] {
				res.LineDiscounts[i] = base
				winner[i] = -1 // aksiya bu qatorda endi hech narsa bermadi
				continue
			}
			res.LineDiscounts[i] = scaled[i]
		}
	}

	// ---- 4. Yakuniy summalar va aksiyalar ro'yxati ----
	perPromo := make([]int64, len(cands))
	for i := range lines {
		res.DiscountTiyin += res.LineDiscounts[i]
		if w := winner[i]; w >= 0 {
			perPromo[w] += res.LineDiscounts[i]
			res.PromotionTiyin += res.LineDiscounts[i]
		}
	}
	for ci, amount := range perPromo {
		if amount > 0 {
			res.Promotions = append(res.Promotions,
				AppliedPromotion{Promotion: cands[ci].promo, DiscountTiyin: amount})
		}
	}
	// Hissasi bo'yicha kamayish tartibida; teng bo'lsa tartib barqaror
	// (aksiyalar ro'yxatidagi tartib) — bir xil savat har doim bir xil
	// natija berishi uchun.
	sort.SliceStable(res.Promotions, func(a, b int) bool {
		return res.Promotions[a].DiscountTiyin > res.Promotions[b].DiscountTiyin
	})
	return res
}

// isEligible — aksiya UMUMAN shu savatga qo'llanishi mumkinmi.
func isEligible(p *Promotion, subtotal int64, previousOrderCount int, now time.Time) bool {
	if p.ComputeStatus(now) != StatusActive {
		return false
	}
	if p.Type == TypeFreeDelivery {
		return false
	}
	if p.MinOrderAmountTiyin > 0 && subtotal < p.MinOrderAmountTiyin {
		return false
	}
	if p.Type == TypeLoyalty && p.MinPreviousOrders > 0 && int64(previousOrderCount) < p.MinPreviousOrders {
		return false
	}
	return true
}

func cartSubtotal(lines []CartLine) int64 {
	var total int64
	for _, l := range lines {
		total += l.subtotal()
	}
	return total
}

// eligibleIndexes — p.AppliesToProducts/Orders/Categories bayroqlariga
// qarab qaysi savat qatorlari shu aksiyaga mos kelishini aniqlaydi va
// ularning INDEKSLARINI qaytaradi (nusxasini emas — chegirma keyin
// aynan shu indekslar bo'yicha taqsimlanadi).
// AppliesToOrders=true bo'lsa BUTUN savat mos keladi (boshqa
// bayroqlardan qat'iy nazar — chunki bu "butun buyurtmaga" degani).
func eligibleIndexes(p *Promotion, lines []CartLine) []int {
	if p.AppliesToOrders {
		out := make([]int, len(lines))
		for i := range lines {
			out[i] = i
		}
		return out
	}
	productSet := make(map[string]bool, len(p.TargetProductIDs))
	for _, id := range p.TargetProductIDs {
		productSet[id] = true
	}
	categorySet := make(map[string]bool, len(p.TargetCategories))
	for _, c := range p.TargetCategories {
		categorySet[c] = true
	}
	var out []int
	for i, l := range lines {
		if (p.AppliesToProducts && productSet[l.ProductID]) || (p.AppliesToCategories && categorySet[l.Category]) {
			out = append(out, i)
		}
	}
	return out
}

// discountByUnit — butun mos summaga (eligibleSubtotal) nisbatan BIR
// MARTA hisoblanadigan chegirma. Birlik `p.EffectiveUnit()` dan olinadi:
// saqlangan `DiscountUnit` turga zid bo'lsa ham hisob TUR bo'yicha
// bo'ladi (klient bilan farq qilib qolmasligi uchun — promotion.go'dagi
// izohga qarang).
func discountByUnit(p *Promotion, eligibleSubtotal int64) int64 {
	if p.EffectiveUnit() == DiscountUnitPercent {
		return eligibleSubtotal * p.DiscountValue / 100
	}
	return p.DiscountValue
}

// Spread — `total` ni `weights` og'irliklari bo'yicha qatorlarga
// taqsimlaydi. Ikkita KAFOLAT beradi:
//   - natija yig'indisi ANIQ `total` ga teng (yaxlitlashda yo'qolgan
//     tiyinlar eng katta og'irlikli qatorlarga bittadan qaytariladi);
//   - hech bir qator o'z og'irligidan (ya'ni o'z summasidan) oshmaydi,
//     ya'ni "minus pulga" o'tgan qator bo'lmaydi.
//
// `total` og'irliklar yig'indisidan katta bo'lsa, natija og'irliklarning
// o'zi bo'ladi.
func Spread(total int64, weights []int64) []int64 {
	out := make([]int64, len(weights))
	var sum int64
	for _, w := range weights {
		if w > 0 {
			sum += w
		}
	}
	if sum <= 0 || total <= 0 {
		return out
	}
	if total > sum {
		total = sum
	}
	var given int64
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		out[i] = total * w / sum
		given += out[i]
	}
	for rem := total - given; rem > 0; rem-- {
		best := -1
		for i, w := range weights {
			if w <= 0 || out[i] >= w {
				continue
			}
			if best == -1 || w > weights[best] {
				best = i
			}
		}
		if best == -1 {
			break
		}
		out[best]++
	}
	return out
}

func sumOf(values []int64) int64 {
	var total int64
	for _, v := range values {
		total += v
	}
	return total
}

// lineDiscounts — bitta aksiyaning savat qatorlari bo'yicha chegirmasi
// (maksimal chegirma cheklovisiz — u `Apply` da, aksiya HAQIQATDA
// yutgan qatorlar bo'yicha qo'llanadi).
//
// Taqsimot mexanikaga qarab ikki xil:
//   - BOGO va "summa orqali" chegirma HAR QATORDA alohida ma'noga ega
//     (bepul donalar / donaga qat'iy summa) — o'z qiymati saqlanadi;
//   - foiz/to'plam/sodiqlik esa BUTUN mos summaga nisbatan hisoblanadi,
//     shuning uchun qatorlarga ularning summasiga MUTANOSIB tarqatiladi.
//
// Hech bir qator o'z summasidan ko'p chegirma olmaydi.
func lineDiscounts(p *Promotion, lines []CartLine) []int64 {
	empty := make([]int64, len(lines))
	elig := eligibleIndexes(p, lines)
	if len(elig) == 0 {
		return empty
	}
	var eligSubtotal int64
	weights := make([]int64, len(lines))
	for _, i := range elig {
		s := lines[i].subtotal()
		weights[i] = s
		eligSubtotal += s
	}

	switch p.Type {
	case TypeBOGO:
		// Har bir mos qatorda: har 2 dona uchun 1 donasi BEPUL.
		perLine := make([]int64, len(lines))
		for _, i := range elig {
			perLine[i] = int64(lines[i].Qty/2) * lines[i].UnitPriceTiyin
		}
		return perLine

	case TypeBundle:
		// To'plamdagi BARCHA maqsadli mahsulot/turkumlar savatda
		// (kamida 1 donadan) bo'lishi shart — aks holda aksiya
		// qo'llanilmaydi.
		if p.AppliesToProducts && len(p.TargetProductIDs) > 0 {
			have := make(map[string]bool, len(elig))
			for _, i := range elig {
				have[lines[i].ProductID] = true
			}
			for _, id := range p.TargetProductIDs {
				if !have[id] {
					return empty
				}
			}
		}
		if p.AppliesToCategories && len(p.TargetCategories) > 0 {
			have := make(map[string]bool, len(elig))
			for _, i := range elig {
				have[lines[i].Category] = true
			}
			for _, c := range p.TargetCategories {
				if !have[c] {
					return empty
				}
			}
		}
		return Spread(discountByUnit(p, eligSubtotal), weights)

	case TypeFixedAmount:
		// "Summa orqali chegirma" — DiscountValue HAR BIR mos qatordagi HAR
		// BIR donaga ALOHIDA qo'llaniladi (masalan "5000 so'm chegirma" 2
		// xil taomga tayinlansa va ikkalasidan 1 tadan olinsa — jami
		// 5000+5000=10000 so'm chegirma beriladi). AVVAL bu yerda ham
		// discountByUnit() ishlatilardi — u DiscountValue'ni FLAT, necha
		// xil/necha dona mos mahsulot olinishidan qat'iy nazar BIR MARTA
		// qo'llardi — bu haqiqiy hisoblash xatosi edi, tuzatildi.
		//
		// Bitta qatorning chegirmasi o'sha qatorning o'z summasidan
		// oshmaydi (3000 so'mlik taomga 5000 so'm "chegirma" — faqat
		// 3000 so'm chegiriladi, taom "minus pulga" o'tmaydi).
		perLine := make([]int64, len(lines))
		for _, i := range elig {
			d := p.DiscountValue * int64(lines[i].Qty)
			if s := lines[i].subtotal(); d > s {
				d = s
			}
			perLine[i] = d
		}
		return perLine

	case TypePercent, TypeLoyalty:
		return Spread(discountByUnit(p, eligSubtotal), weights)

	default:
		return empty
	}
}
