package delivery

import (
	"errors"
	"testing"
)

// Kelishilgan jadval (2026-09-30): baza 1.5 km — 6 000 so'm, keyin har
// QO'SHIMCHA kilometr +2 000 so'm, 7 km dan narida yetkazilmaydi.
//
// Chegaradagi qiymatlar ATAYLAB ko'p: aynan ular buziladi. 1500 va 1501,
// 2500 va 2501 — bir metr farq narxni bir pog'ona ko'taradi va formula
// «<=» ni «<» ga almashtirsa test darhol yiqiladi.
func TestDeliveryFee(t *testing.T) {
	const somm = 100_000 // 1 000 so'm tiyinda — jadvalni o'qish uchun

	cases := []struct {
		meters int
		want   int64
	}{
		{0, 6 * somm},
		{1, 6 * somm},
		{1499, 6 * somm},
		{1500, 6 * somm}, // baza — hali qo'shimcha yo'q
		{1501, 8 * somm}, // bir metr o'tdi — to'liq kilometr hisoblanadi
		{2000, 8 * somm},
		{2500, 8 * somm},  // 1000 m qo'shimcha — aynan 1 km
		{2501, 10 * somm}, // 1001 m — ikkinchi kilometr boshlandi
		{3500, 10 * somm},
		{3501, 12 * somm},
		{4500, 12 * somm},
		{5500, 14 * somm},
		{6500, 16 * somm},
		{6501, 18 * somm},
		{7000, 18 * somm}, // eng chekka nuqta — hali yetkaziladi
	}
	for _, c := range cases {
		got, err := DeliveryFee(c.meters)
		if err != nil {
			t.Errorf("%d m: kutilmagan xato %v", c.meters, err)
			continue
		}
		if got != c.want {
			t.Errorf("%d m: %d tiyin, kutilgan %d", c.meters, got, c.want)
		}
	}
}

func TestDeliveryFeeLimits(t *testing.T) {
	// ┌─ NEGA XATO, NOL EMAS ────────────────────────────────────────┐
	// Nol qaytarilsa chaqiruvchi uni «bepul» deb tushunib, mijozga
	// bajarib bo'lmaydigan buyurtmani ko'rsatardi.
	// └──────────────────────────────────────────────────────────────┘
	if _, err := DeliveryFee(FeeMaxMeters + 1); !errors.Is(err, ErrTooFar) {
		t.Errorf("7001 m: %v, ErrTooFar kutilgan", err)
	}
	if _, err := DeliveryFee(-1); !errors.Is(err, ErrBadDistance) {
		t.Errorf("-1 m: %v, ErrBadDistance kutilgan", err)
	}
}

func TestDeliverable(t *testing.T) {
	for _, c := range []struct {
		meters int
		want   bool
	}{
		{0, true},
		{FeeMaxMeters, true},
		{FeeMaxMeters + 1, false},
		{-1, false},
	} {
		if got := Deliverable(c.meters); got != c.want {
			t.Errorf("%d m: Deliverable = %v, kutilgan %v", c.meters, got, c.want)
		}
	}
}

// Narx masofa bilan KAMAYMASLIGI kerak — formula o'zgarganda buzilishi
// oson va jadval hamma qiymatni qamramaydi.
func TestDeliveryFeeMonotonic(t *testing.T) {
	var oldin int64
	for m := 0; m <= FeeMaxMeters; m++ {
		got, err := DeliveryFee(m)
		if err != nil {
			t.Fatalf("%d m: kutilmagan xato %v", m, err)
		}
		if got < oldin {
			t.Fatalf("%d m: narx kamaydi (%d -> %d)", m, oldin, got)
		}
		oldin = got
	}
}

func TestDistanceMeters(t *testing.T) {
	// Chust markazi va undan ~1 km shimoldagi nuqta (0.009° kenglik).
	const chustLat, chustLng = 41.0004, 71.2394
	got := DistanceMeters(chustLat, chustLng, chustLat+0.009, chustLng)
	if got < 950 || got > 1050 {
		t.Errorf("~1 km kutilgan edi, %d m chiqdi", got)
	}
	// Bir xil nuqta — nol.
	if got := DistanceMeters(chustLat, chustLng, chustLat, chustLng); got != 0 {
		t.Errorf("bir xil nuqta: %d m, 0 kutilgan", got)
	}
	// Simmetrik.
	a := DistanceMeters(chustLat, chustLng, 41.0100, 71.2500)
	b := DistanceMeters(41.0100, 71.2500, chustLat, chustLng)
	if a != b {
		t.Errorf("simmetrik emas: %d != %d", a, b)
	}
}
