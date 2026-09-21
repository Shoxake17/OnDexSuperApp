package delivery

import (
	"math"
	"testing"
	"time"
)

func TestValidCoordsRejectsGarbage(t *testing.T) {
	bad := []struct {
		name     string
		lat, lng float64
	}{
		{"nol oroli", 0, 0},
		{"NaN kenglik", math.NaN(), 71.2},
		{"NaN uzunlik", 41.0, math.NaN()},
		{"cheksizlik", math.Inf(1), 71.2},
		{"kenglik chegaradan tashqarida", 91, 71.2},
		{"uzunlik chegaradan tashqarida", 41, 181},
	}
	for _, c := range bad {
		if ValidCoords(c.lat, c.lng) {
			t.Errorf("%s: rad etilishi kerak edi", c.name)
		}
	}
	if !ValidCoords(41.0004, 71.2394) {
		t.Error("Chust markazi qabul qilinishi kerak edi")
	}
}

// NaN haversine hisobiga tarqalib ketmasligini alohida tekshiramiz —
// bu aynan dispatch masofa saralashini buzadigan holat edi.
func TestCoveredRejectsNaN(t *testing.T) {
	if Covered(math.NaN(), math.NaN()) {
		t.Error("NaN hech qachon qamrab olingan deb hisoblanmasligi kerak")
	}
}

func TestCheckPointErrors(t *testing.T) {
	if err := CheckPoint(0, 0); err != ErrBadCoords {
		t.Errorf("ErrBadCoords kutilgan, olindi %v", err)
	}
	// Samarqand — haqiqiy koordinata, lekin hech bir xizmat shahrida emas.
	if err := CheckPoint(39.6542, 66.9597); err != ErrOutsideArea {
		t.Errorf("ErrOutsideArea kutilgan, olindi %v", err)
	}
	if err := CheckPoint(41.0004, 71.2394); err != nil {
		t.Errorf("Chust qabul qilinishi kerak edi, olindi %v", err)
	}
	if err := CheckPoint(41.2995, 69.2401); err != nil {
		t.Errorf("Toshkent qabul qilinishi kerak edi, olindi %v", err)
	}
}

func TestInOperationalRange(t *testing.T) {
	// Namangan (~40 km) — xizmat radiusidan tashqarida, lekin kuryer
	// koordinatasi sifatida hali ishonarli.
	if !InOperationalRange(41.0011, 71.6673) {
		t.Error("Namangan operatsion mintaqada bo'lishi kerak edi")
	}
	// Toshkent — xizmat shahri: u yerdagi kuryerning joylashuvi qabul
	// qilinadi (avval 400 bilan rad etilib, `location` NULL qolardi).
	if !InOperationalRange(41.2995, 69.2401) {
		t.Error("Toshkent operatsion mintaqada bo'lishi kerak edi")
	}
	// Chirchiq (~31 km) — Toshkent doirasidan tashqarida, lekin kuryer
	// koordinatasi sifatida ishonarli.
	if !InOperationalRange(41.4689, 69.5822) {
		t.Error("Chirchiq operatsion mintaqada bo'lishi kerak edi")
	}
	// Samarqand (~250 km) — hech bir xizmat shahriga yaqin emas.
	if InOperationalRange(39.6542, 66.9597) {
		t.Error("Samarqand operatsion mintaqadan tashqarida bo'lishi kerak edi")
	}
	if InOperationalRange(math.NaN(), 71.2) {
		t.Error("NaN rad etilishi kerak edi")
	}
}

func TestSpeedGateBlocksTeleport(t *testing.T) {
	g := NewSpeedGate()
	t0 := time.Now()

	if !g.Accept("c1", 41.0004, 71.2394, t0) {
		t.Fatal("birinchi nuqta shartsiz qabul qilinishi kerak")
	}
	// 10 sekunddan keyin ~40 km narida => ~14 400 km/soat — imkonsiz.
	if g.Accept("c1", 41.0011, 71.6673, t0.Add(10*time.Second)) {
		t.Error("teleport rad etilishi kerak edi")
	}
	// Rad etilgan nuqta ESLAB QOLINMASLIGI kerak: shu asosdan davom
	// etilsa, hujumchi ikki qadamda istagan joyga "yetib" olardi.
	if !g.Accept("c1", 41.0006, 71.2400, t0.Add(20*time.Second)) {
		t.Error("haqiqiy nuqta qabul qilinishi kerak edi")
	}
}

func TestSpeedGateAllowsRealMovement(t *testing.T) {
	g := NewSpeedGate()
	t0 := time.Now()
	g.Accept("c1", 41.0004, 71.2394, t0)
	// 60 sekundda ~0.8 km => ~48 km/soat — oddiy moped tezligi.
	if !g.Accept("c1", 41.0076, 71.2394, t0.Add(60*time.Second)) {
		t.Error("normal harakat qabul qilinishi kerak edi")
	}
}

func TestSpeedGateIgnoresStaleAndRapid(t *testing.T) {
	g := NewSpeedGate()
	t0 := time.Now()

	// Juda tez-tez kelgan yangilanish — GPS shovqini tezlikni buzadi,
	// tekshiruvdan o'tkazib yuboriladi.
	g.Accept("c1", 41.0004, 71.2394, t0)
	if !g.Accept("c1", 41.0100, 71.2394, t0.Add(time.Second)) {
		t.Error("1 sekundlik oraliq tekshiruvdan chetlab o'tilishi kerak edi")
	}

	// Uzoq tanaffusdan keyin — kuryer ilovani yopib boshqa joyga
	// borgan bo'lishi mumkin, bu firibgarlik emas.
	g2 := NewSpeedGate()
	g2.Accept("c2", 41.0004, 71.2394, t0)
	if !g2.Accept("c2", 41.0011, 71.6673, t0.Add(10*time.Minute)) {
		t.Error("eskirgan nuqtadan keyin tekshirilmasligi kerak edi")
	}
}

// Jonli holat (2026-09-15): telefon XATO tarmoq nuqtasini haqiqiy joy bilan
// navbatma-navbat yubordi. Xato nuqta asos bo'lib, o'zi qayta kelib asosni
// har 10 soniyada yangilab turdi — haqiqiy joy har safar 10 s oraliqda
// "1,6 km" sakrab, HECH QACHON o'tmasdi. Izchil kelgan joy endi bir daqiqada
// qabul qilinadi; xato nuqta keyin yana rad etiladi.
func TestSpeedGateRecoversFromBadBaseline(t *testing.T) {
	g := NewSpeedGate()
	t0 := time.Now()
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	bad := func(s int) bool { return g.Accept("c1", 41.0006327, 71.2215262, at(s)) }
	real := func(s int) bool { return g.Accept("c1", 41.0018207, 71.2029233, at(s)) }

	if !bad(0) {
		t.Fatal("birinchi nuqta shartsiz qabul qilinishi kerak")
	}
	for s := 10; s < 70; s += 20 {
		if real(s) {
			t.Fatalf("%d s: izchillik hali tasdiqlanmagan — rad etilishi kerak edi", s)
		}
		if !bad(s + 10) {
			t.Fatalf("%d s: asos bilan bir xil nuqta qabul qilinishi kerak edi", s+10)
		}
	}
	if !real(70) {
		t.Fatal("bir daqiqa izchil kelgan haqiqiy joy qabul qilinishi kerak edi")
	}
	if g.Accept("c1", 41.0006327, 71.2215262, t0.Add(80*time.Second)) {
		t.Fatal("xato nuqta yana rad etilishi kerak edi")
	}
}

// Har safar BOSHQA joyga sakrash izchillik emas — hech qachon o'tmaydi.
func TestSpeedGateScatteredJumpsNeverConfirm(t *testing.T) {
	g := NewSpeedGate()
	t0 := time.Now()
	g.Accept("c1", 41.0004, 71.2394, t0)
	for i := 1; i <= 12; i++ {
		// Ikki uzoq (10 va 18 km) nuqta navbatma-navbat — 2 daqiqada ham
		// tezlik 150 km/soatdan oshadi.
		lng := 71.36 + float64(i%2)*0.1
		if g.Accept("c1", 41.0004, lng, t0.Add(time.Duration(i*10)*time.Second)) {
			t.Fatalf("%d-sakrash qabul qilindi", i)
		}
	}
}
