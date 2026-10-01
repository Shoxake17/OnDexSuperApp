package httpapi

import (
	"math"
	"testing"

	"chustapp/internal/tracking"
	"chustapp/internal/voice"
)

// shift — p dan shimolga/sharqqa metr bo'yicha siljitilgan nuqta.
func shift(p tracking.Point, northM, eastM float64) tracking.Point {
	const mPerDegLat = 111320.0
	return tracking.Point{
		Lat: p.Lat + northM/mPerDegLat,
		Lng: p.Lng + eastM/(mPerDegLat*math.Cos(p.Lat*math.Pi/180)),
	}
}

// leg — `from` dan (dn,de) yo'nalishda `lengthM` metr, stepM qadam bilan.
// Bosh nuqta kiritilmaydi.
func leg(from tracking.Point, dn, de, lengthM, stepM float64) []tracking.Point {
	norm := math.Hypot(dn, de)
	var out []tracking.Point
	for d := stepM; d <= lengthM+0.001; d += stepM {
		out = append(out, shift(from, dn/norm*d, de/norm*d))
	}
	return out
}

var stepsOrigin = tracking.Point{Lat: 41.0, Lng: 71.2}

func TestSynthesizeStepsStraightLineHasNone(t *testing.T) {
	pts := append([]tracking.Point{stepsOrigin}, leg(stepsOrigin, 1, 0, 600, 20)...)
	if got := synthesizeSteps(pts); len(got) != 0 {
		t.Fatalf("to'g'ri yo'lda burilish bo'lmasligi kerak: %+v", got)
	}
}

func TestSynthesizeStepsRightAndLeftCorner(t *testing.T) {
	cases := []struct {
		name   string
		dn, de float64 // ikkinchi bo'lak yo'nalishi (birinchisi — shimol)
		want   voice.Maneuver
	}{
		{"o'ngga 90", 0, 1, voice.Right},
		{"chapga 90", 0, -1, voice.Left},
		{"o'ngga yengil ~45", 1, 1, voice.SlightRight},
		{"chapga yengil ~45", 1, -1, voice.SlightLeft},
		{"o'ngga keskin ~150", -1.7, 1, voice.SharpRight},
		{"chapga keskin ~150", -1.7, -1, voice.SharpLeft},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			first := leg(stepsOrigin, 1, 0, 200, 20)
			corner := first[len(first)-1]
			pts := append([]tracking.Point{stepsOrigin}, first...)
			pts = append(pts, leg(corner, c.dn, c.de, 200, 20)...)

			got := synthesizeSteps(pts)
			if len(got) != 1 {
				t.Fatalf("bitta burilish kutilgan: %+v", got)
			}
			if voice.Maneuver(got[0].Maneuver) != c.want {
				t.Fatalf("manevr %q, kutilgan %q", got[0].Maneuver, c.want)
			}
			// Burilish burchakda: ~200 m dan keyin, oxirigacha ~200 m qoladi.
			if got[0].DistanceMeters < 150 || got[0].DistanceMeters > 250 {
				t.Fatalf("burilishdan oxirigacha masofa: %d", got[0].DistanceMeters)
			}
			if d := pointDist(tracking.Point{Lat: got[0].Lat, Lng: got[0].Lng}, corner); d > 40 {
				t.Fatalf("burilish nuqtasi burchakdan %.0f m uzoqda", d)
			}
		})
	}
}

func TestSynthesizeStepsGentleCurveHasNone(t *testing.T) {
	// 300 m radiusli yoy bo'ylab 90° egrilik: har 20 m ga ~3.8° — burilish emas.
	const r, stepDeg = 300.0, 3.8
	pts := []tracking.Point{stepsOrigin}
	for a := stepDeg; a <= 90; a += stepDeg {
		rad := a * math.Pi / 180
		pts = append(pts, shift(stepsOrigin, r*math.Sin(rad), r*(1-math.Cos(rad))))
	}
	if got := synthesizeSteps(pts); len(got) != 0 {
		t.Fatalf("yumshoq egrilikda burilish bo'lmasligi kerak: %+v", got)
	}
}

func TestSynthesizeStepsMergesNearbyVertices(t *testing.T) {
	// Burchak ikki yaqin tugunda bo'linib (45° + 45°) keladi — bitta burilish.
	first := leg(stepsOrigin, 1, 0, 200, 20)
	c1 := first[len(first)-1]
	mid := leg(c1, 1, 1, 14, 14) // 14 m diagonal
	c2 := mid[len(mid)-1]
	pts := append([]tracking.Point{stepsOrigin}, first...)
	pts = append(pts, mid...)
	pts = append(pts, leg(c2, 0, 1, 200, 20)...)

	got := synthesizeSteps(pts)
	if len(got) != 1 || voice.Maneuver(got[0].Maneuver) != voice.Right {
		t.Fatalf("bitta o'ng burilish kutilgan: %+v", got)
	}
}

func TestSynthesizeStepsTwoTurnsDistances(t *testing.T) {
	a := leg(stepsOrigin, 1, 0, 300, 20)
	ca := a[len(a)-1]
	b := leg(ca, 0, 1, 400, 20)
	cb := b[len(b)-1]
	c := leg(cb, 1, 0, 150, 20)
	pts := append(append(append([]tracking.Point{stepsOrigin}, a...), b...), c...)

	got := synthesizeSteps(pts)
	if len(got) != 2 {
		t.Fatalf("ikkita burilish kutilgan: %+v", got)
	}
	if voice.Maneuver(got[0].Maneuver) != voice.Right || voice.Maneuver(got[1].Maneuver) != voice.Left {
		t.Fatalf("manevrlar: %+v", got)
	}
	// Birinchi qadam uzunligi — ikkinchi burilishgacha (~400 m), ikkinchisi — oxirigacha (~150 m).
	if got[0].DistanceMeters < 350 || got[0].DistanceMeters > 450 {
		t.Fatalf("1-qadam masofasi: %d", got[0].DistanceMeters)
	}
	if got[1].DistanceMeters < 100 || got[1].DistanceMeters > 200 {
		t.Fatalf("2-qadam masofasi: %d", got[1].DistanceMeters)
	}
}

func TestSynthesizeStepsDegenerateInputs(t *testing.T) {
	if got := synthesizeSteps(nil); got != nil {
		t.Fatalf("nil: %+v", got)
	}
	if got := synthesizeSteps([]tracking.Point{stepsOrigin, shift(stepsOrigin, 50, 0)}); got != nil {
		t.Fatalf("2 nuqta: %+v", got)
	}
	// Bir xil nuqtalar (NaN bo'lishi mumkin) — panic yoki burilish bo'lmasin.
	same := []tracking.Point{stepsOrigin, stepsOrigin, stepsOrigin, stepsOrigin}
	if got := synthesizeSteps(same); len(got) != 0 {
		t.Fatalf("bir xil nuqtalar: %+v", got)
	}
}
