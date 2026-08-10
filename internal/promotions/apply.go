package promotions

import "time"

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

// AppliedDiscount — ApplyBest natijasi: qaysi aksiya tanlandi va
// nechi tiyin chegirma berildi.
type AppliedDiscount struct {
	Promotion     *Promotion
	DiscountTiyin int64
}

// ApplyBest — restoranning FAOL aksiyalari orasidan berilgan savatga
// ENG KO'P chegirma beradigan bittasini tanlaydi va qo'llaydi (bir
// vaqtning o'zida faqat bitta aksiya — stacking YO'Q, oddiy va
// mijozga tushunarli xatti-harakat). Hech biri mos kelmasa yoki
// chegirma 0 bo'lsa nil qaytaradi.
//
// TypeFreeDelivery BU YERDA HECH QACHON tanlanmaydi — tizimda
// yetkazib berish narxi tushunchasi umuman yo'q (hech qayerda
// hisoblanmaydi), shuning uchun chegirma qiladigan haqiqiy narx yo'q.
// Bu — soxta/ko'rinishdagina chegirma qilishdan qochish uchun ataylab
// qilingan tanlov (ROADMAP'da qayd etilgan).
func ApplyBest(promos []*Promotion, lines []CartLine, previousOrderCount int, now time.Time) *AppliedDiscount {
	var best *AppliedDiscount
	subtotal := cartSubtotal(lines)
	for _, p := range promos {
		if p.ComputeStatus(now) != StatusActive {
			continue
		}
		if p.Type == TypeFreeDelivery {
			continue
		}
		if p.MinOrderAmountTiyin > 0 && subtotal < p.MinOrderAmountTiyin {
			continue
		}
		if p.Type == TypeLoyalty && p.MinPreviousOrders > 0 && int64(previousOrderCount) < p.MinPreviousOrders {
			continue
		}
		discount := computeDiscount(p, lines)
		if discount <= 0 {
			continue
		}
		if best == nil || discount > best.DiscountTiyin {
			best = &AppliedDiscount{Promotion: p, DiscountTiyin: discount}
		}
	}
	return best
}

func cartSubtotal(lines []CartLine) int64 {
	var total int64
	for _, l := range lines {
		total += l.subtotal()
	}
	return total
}

// eligibleLines — p.AppliesToProducts/Orders/Categories bayroqlariga
// qarab qaysi savat qatorlari shu aksiyaga mos kelishini aniqlaydi.
// AppliesToOrders=true bo'lsa BUTUN savat mos keladi (boshqa
// bayroqlardan qat'iy nazar — chunki bu "butun buyurtmaga" degani).
func eligibleLines(p *Promotion, lines []CartLine) []CartLine {
	if p.AppliesToOrders {
		return lines
	}
	productSet := make(map[string]bool, len(p.TargetProductIDs))
	for _, id := range p.TargetProductIDs {
		productSet[id] = true
	}
	categorySet := make(map[string]bool, len(p.TargetCategories))
	for _, c := range p.TargetCategories {
		categorySet[c] = true
	}
	var out []CartLine
	for _, l := range lines {
		if (p.AppliesToProducts && productSet[l.ProductID]) || (p.AppliesToCategories && categorySet[l.Category]) {
			out = append(out, l)
		}
	}
	return out
}

func capDiscount(p *Promotion, discount, eligibleSubtotal int64) int64 {
	if discount > eligibleSubtotal {
		discount = eligibleSubtotal
	}
	if p.MaxDiscountAmountTiyin > 0 && discount > p.MaxDiscountAmountTiyin {
		discount = p.MaxDiscountAmountTiyin
	}
	if discount < 0 {
		discount = 0
	}
	return discount
}

func discountByUnit(p *Promotion, eligibleSubtotal int64) int64 {
	if p.DiscountUnit == DiscountUnitPercent {
		return eligibleSubtotal * p.DiscountValue / 100
	}
	return p.DiscountValue
}

// fixedAmountDiscount — TypeFixedAmount uchun: DiscountValue (tiyin) har
// bir mos QATORNING har bir DONASIGA alohida qo'llaniladi (BOGO'dagi kabi
// miqdorga qarab o'suvchi mantiq — percentdagi kabi subtotalga nisbatan
// emas). Bitta qatorning chegirmasi o'sha qatorning o'z summasidan
// oshmaydi (masalan 3000 so'mlik taomga 5000 so'm "chegirma" — faqat
// 3000 so'm chegiriladi, taom "minus pulga" o'tib ketmaydi).
func fixedAmountDiscount(p *Promotion, lines []CartLine) int64 {
	var total int64
	for _, l := range lines {
		perLine := p.DiscountValue * int64(l.Qty)
		if lineSubtotal := l.subtotal(); perLine > lineSubtotal {
			perLine = lineSubtotal
		}
		total += perLine
	}
	return total
}

func computeDiscount(p *Promotion, lines []CartLine) int64 {
	elig := eligibleLines(p, lines)
	if len(elig) == 0 {
		return 0
	}
	eligSubtotal := cartSubtotal(elig)

	switch p.Type {
	case TypeBOGO:
		// Har bir mos qatorda: har 2 dona uchun 1 donasi BEPUL.
		var discount int64
		for _, l := range elig {
			freeUnits := int64(l.Qty / 2)
			discount += freeUnits * l.UnitPriceTiyin
		}
		return capDiscount(p, discount, eligSubtotal)

	case TypeBundle:
		// To'plamdagi BARCHA maqsadli mahsulot/turkumlar savatda
		// (kamida 1 donadan) bo'lishi shart — aks holda aksiya
		// qo'llanilmaydi (0 qaytadi).
		if p.AppliesToProducts && len(p.TargetProductIDs) > 0 {
			have := make(map[string]bool, len(elig))
			for _, l := range elig {
				have[l.ProductID] = true
			}
			for _, id := range p.TargetProductIDs {
				if !have[id] {
					return 0
				}
			}
		}
		if p.AppliesToCategories && len(p.TargetCategories) > 0 {
			have := make(map[string]bool, len(elig))
			for _, l := range elig {
				have[l.Category] = true
			}
			for _, c := range p.TargetCategories {
				if !have[c] {
					return 0
				}
			}
		}
		return capDiscount(p, discountByUnit(p, eligSubtotal), eligSubtotal)

	case TypeFixedAmount:
		// "Summa orqali chegirma" — DiscountValue HAR BIR mos qatordagi HAR
		// BIR donaga ALOHIDA qo'llaniladi (masalan "5000 so'm chegirma" 2
		// xil taomga tayinlansa va ikkalasidan 1 tadan olinsa — jami
		// 5000+5000=10000 so'm chegirma beriladi). AVVAL bu yerda ham
		// discountByUnit() ishlatilardi — u DiscountValue'ni FLAT, necha
		// xil/necha dona mos mahsulot olinishidan qat'iy nazar BIR MARTA
		// qo'llardi (masalan yuqoridagi holatda haqiqiy 10000 o'rniga
		// noto'g'ri 5000 chegirma berardi) — bu haqiqiy hisoblash xatosi
		// edi, tuzatildi.
		return capDiscount(p, fixedAmountDiscount(p, elig), eligSubtotal)

	case TypePercent, TypeLoyalty:
		return capDiscount(p, discountByUnit(p, eligSubtotal), eligSubtotal)

	default:
		return 0
	}
}
