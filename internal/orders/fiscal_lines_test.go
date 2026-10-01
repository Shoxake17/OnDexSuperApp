package orders

import (
	"testing"
)

// FISKAL CHEK SATRLARI.
//
// ┌─ NEGA ALOHIDA TEST ───────────────────────────────────────────────┐
// Chek soliq idorasiga ketadi. Undagi yig'indi to'lov summasidan bir
// tiyinga farq qilsa ham shlyuz to'lovni rad etadi — yoki undan ham
// yomoni, qabul qilib, keyin hisobot to'g'ri kelmaydi.
//
// Chegirma BUTUN savatga tegishli, chek esa QATORLAR bo'yicha
// yoziladi: bo'linish har doim butun songa to'g'ri kelmaydi va
// qoldiq yo'qolib qolishi mumkin. Shuning uchun asosiy tekshiruv
// bitta: Σ(satr) == TotalTiyin.
// └───────────────────────────────────────────────────────────────────┘

func sumLines(o *Order) int64 {
	var s int64
	for _, l := range fiscalLines(o) {
		s += l.TotalTiyin
	}
	return s
}

func TestFiscalLinesSumEqualsOrderTotal(t *testing.T) {
	cases := map[string]*Order{
		"chegirmasiz": {
			Items: []Item{
				{Name: "Lavash", Qty: 2, PriceTiyin: 2_250_000},
				{Name: "Cola", Qty: 1, PriceTiyin: 800_000},
			},
			SubtotalTiyin: 5_300_000,
			TotalTiyin:    5_300_000,
		},
		"chegirma bilan": {
			Items: []Item{
				{Name: "Lavash", Qty: 2, PriceTiyin: 2_250_000},
				{Name: "Cola", Qty: 1, PriceTiyin: 800_000},
			},
			SubtotalTiyin: 5_300_000,
			DiscountTiyin: 530_000,
			TotalTiyin:    4_770_000,
		},
		"yetkazish bilan": {
			Items:            []Item{{Name: "Osh", Qty: 1, PriceTiyin: 3_500_000}},
			SubtotalTiyin:    3_500_000,
			DeliveryFeeTiyin: 600_000,
			TotalTiyin:       4_100_000,
		},
		"chegirma + yetkazish": {
			Items: []Item{
				{Name: "Osh", Qty: 3, PriceTiyin: 3_500_000},
				{Name: "Non", Qty: 2, PriceTiyin: 300_000},
			},
			SubtotalTiyin:    11_100_000,
			DiscountTiyin:    1_110_000,
			DeliveryFeeTiyin: 600_000,
			TotalTiyin:       10_590_000,
		},
		// ⚠️ ENG MUHIM HOLAT: bo'linmaydigan chegirma. 3 ta bir xil
		// qator va 1 tiyin qoldiq — oddiy bo'lishda u yo'qolardi.
		"bo'linmaydigan qoldiq": {
			Items: []Item{
				{Name: "A", Qty: 1, PriceTiyin: 1_000},
				{Name: "B", Qty: 1, PriceTiyin: 1_000},
				{Name: "C", Qty: 1, PriceTiyin: 1_000},
			},
			SubtotalTiyin: 3_000,
			DiscountTiyin: 1,
			TotalTiyin:    2_999,
		},
		"mahsulot chegirma narxi": {
			Items: []Item{
				{Name: "Burger", Qty: 2, PriceTiyin: 4_000_000, DiscountPriceTiyin: 3_000_000},
				{Name: "Fri", Qty: 1, PriceTiyin: 1_200_000},
			},
			SubtotalTiyin: 7_200_000,
			TotalTiyin:    7_200_000,
		},
		"faqat yetkazish (taomsiz)": {
			Items:            nil,
			DeliveryFeeTiyin: 600_000,
			TotalTiyin:       600_000,
		},
	}
	for name, o := range cases {
		got := sumLines(o)
		if got != o.TotalTiyin {
			t.Errorf("%s: satrlar yig'indisi %d, buyurtma summasi %d", name, got, o.TotalTiyin)
		}
	}
}

// Har bir satrning nomi va soni saqlanishi kerak — chekda mijoz
// nimani sotib olganini ko'rishi shart.
func TestFiscalLinesKeepNamesAndQty(t *testing.T) {
	o := &Order{
		Items: []Item{
			{Name: "Lavash", Qty: 2, PriceTiyin: 2_250_000},
			{Name: "Cola", Qty: 3, PriceTiyin: 800_000},
		},
		SubtotalTiyin:    6_900_000,
		DeliveryFeeTiyin: 600_000,
		TotalTiyin:       7_500_000,
	}
	lines := fiscalLines(o)
	if len(lines) != 3 {
		t.Fatalf("satrlar soni: %d (2 taom + yetkazish kutilgan)", len(lines))
	}
	if lines[0].Name != "Lavash" || lines[0].Qty != 2 {
		t.Errorf("birinchi satr: %+v", lines[0])
	}
	if lines[1].Name != "Cola" || lines[1].Qty != 3 {
		t.Errorf("ikkinchi satr: %+v", lines[1])
	}
	// Yetkazib berish ALOHIDA satr: u taom emas.
	last := lines[len(lines)-1]
	if last.Name != "Yetkazib berish" || last.TotalTiyin != 600_000 || last.Qty != 1 {
		t.Errorf("yetkazish satri: %+v", last)
	}
}

// Stol buyurtmasida yetkazish satri BO'LMASLIGI kerak.
func TestFiscalLinesNoDeliveryLineWhenFree(t *testing.T) {
	o := &Order{
		Items:         []Item{{Name: "Osh", Qty: 1, PriceTiyin: 3_500_000}},
		SubtotalTiyin: 3_500_000,
		TotalTiyin:    3_500_000,
	}
	for _, l := range fiscalLines(o) {
		if l.Name == "Yetkazib berish" {
			t.Error("yetkazish haqi 0 bo'lsa ham satr qo'shildi")
		}
	}
}

// Soni nol bo'lgan qator chekka tushmaydi (nolga bo'lish ham
// bo'lmasligi kerak).
func TestFiscalLinesSkipsZeroQty(t *testing.T) {
	o := &Order{
		Items: []Item{
			{Name: "Yo'q", Qty: 0, PriceTiyin: 1_000_000},
			{Name: "Bor", Qty: 1, PriceTiyin: 2_000_000},
		},
		SubtotalTiyin: 2_000_000,
		TotalTiyin:    2_000_000,
	}
	lines := fiscalLines(o)
	if len(lines) != 1 || lines[0].Name != "Bor" {
		t.Fatalf("satrlar: %+v", lines)
	}
	if lines[0].TotalTiyin != 2_000_000 {
		t.Errorf("summa: %d", lines[0].TotalTiyin)
	}
}
