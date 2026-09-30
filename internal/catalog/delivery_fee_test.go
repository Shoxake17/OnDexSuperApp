package catalog

import (
	"errors"
	"testing"
)

// RESTORANNING O'Z YETKAZIB BERISH NARXI.
//
// Restoran o'z kuryeri bilan ishlaydi va narxni o'zi belgilaydi; OnDex
// bu puldan hech narsa olmaydi. Qoidalar kichik, lekin ularning har
// biri buzilsa mijoz noto'g'ri summa to'laydi.
func TestEffectiveDeliveryFee(t *testing.T) {
	const somm = 100_000

	cases := []struct {
		name          string
		fee, freeFrom int64
		total         int64
		dineIn        bool
		want          int64
	}{
		// 0 — BEPUL, "belgilanmagan" emas. Standart holat aynan shu:
		// ko'p kafe yetkazish haqi umuman olmaydi.
		{"narx qo'yilmagan", 0, 0, 50 * somm, false, 0},

		{"oddiy narx", 8 * somm, 0, 50 * somm, false, 8 * somm},

		// Bepul chegarasi.
		{"chegaradan past", 8 * somm, 100 * somm, 99 * somm, false, 8 * somm},
		{"chegaraga TENG — bepul", 8 * somm, 100 * somm, 100 * somm, false, 0},
		{"chegaradan yuqori", 8 * somm, 100 * somm, 150 * somm, false, 0},
		{"chegara 0 — qoida ishlamaydi", 8 * somm, 0, 999 * somm, false, 8 * somm},

		// ┌─ STOL BUYURTMASI ────────────────────────────────────────┐
		// Mijoz stolda o'tiribdi — yetkaziladigan narsa yo'q. Bu
		// tekshiruv chaqiruvchida qoldirilsa, uni bir joyda (masalan
		// `/quote` da) unutib qo'yish oson edi.
		// └──────────────────────────────────────────────────────────┘
		{"stol buyurtmasi — narx bo'lsa ham 0", 8 * somm, 0, 50 * somm, true, 0},
		{"stol buyurtmasi — chegarasiz ham 0", 20 * somm, 100 * somm, 10 * somm, true, 0},

		// Manfiy narx bazaga tushib qolsa ham mijozdan pul yechilmaydi.
		{"manfiy narx — 0", -500, 0, 50 * somm, false, 0},
	}

	for _, c := range cases {
		r := &Restaurant{DeliveryFeeTiyin: c.fee, DeliveryFreeFromTiyin: c.freeFrom}
		if got := r.EffectiveDeliveryFee(c.total, c.dineIn); got != c.want {
			t.Errorf("%s: %d tiyin, kutilgan %d", c.name, got, c.want)
		}
	}
}

// nil restoran — hech qachon panic bermasin: narxlash yo'lida restoran
// topilmasligi mumkin.
func TestEffectiveDeliveryFeeNil(t *testing.T) {
	var r *Restaurant
	if got := r.EffectiveDeliveryFee(50_000_00, false); got != 0 {
		t.Errorf("nil restoran: %d, 0 kutilgan", got)
	}
}

func TestValidateDeliveryFee(t *testing.T) {
	cases := []struct {
		name          string
		fee, freeFrom int64
		want          error
	}{
		{"nol — bepul", 0, 0, nil},
		{"oddiy", 8 * 100_000, 100 * 100_000, nil},
		{"chegarada", MaxDeliveryFeeTiyin, 0, nil},
		{"manfiy narx", -1, 0, ErrDeliveryFeeNegative},
		{"manfiy chegara", 0, -1, ErrDeliveryFeeNegative},
		// ┌─ NEGA YUQORI CHEGARA ────────────────────────────────────┐
		// Panelda nol ortiqcha yozilib qolsa (6 000 so'm o'rniga
		// 6 000 000) mijoz uni checkout'da ko'radi va restoran
		// buyurtmasiz qoladi.
		// └──────────────────────────────────────────────────────────┘
		{"juda katta", MaxDeliveryFeeTiyin + 1, 0, ErrDeliveryFeeTooBig},
	}
	for _, c := range cases {
		err := ValidateDeliveryFee(c.fee, c.freeFrom)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, kutilgan %v", c.name, err, c.want)
		}
	}
}
