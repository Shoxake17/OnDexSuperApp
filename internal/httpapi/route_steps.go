package httpapi

import (
	"math"

	"chustapp/internal/geo"
	"chustapp/internal/tracking"
	"chustapp/internal/voice"
)

// OnDexMap marshruti burilish ko'rsatmalarisiz keladi (OSRM `steps=false`),
// kuryer ilovasining ovozli yo'l ko'rsatishi esa ularga muhtoj. Shu sabab
// burilishlar chiziq shaklidan chiqariladi.
//
// Usul: har tugunda uning OLDIDAGI va KEYINGI ~30 m chiziq yo'nalishi
// solishtiriladi. Oyna tufayli uzun yumshoq egrilik (har tugunda bir necha
// daraja) burilish bo'lib sanalmaydi, aniq burchak esa sanaladi. Bir burchak
// ketma-ket bir necha tugunda ko'rinadi — shu sababli ~40 m ichidagi
// nomzodlardan eng kattasi tanlanadi.
//
// Cheklov: doira (roundabout) va ayri yo'llarni ajratib bo'lmaydi — ular
// oddiy burilish sifatida aytiladi. Google rejimidagi aniq manevrlar
// o'rniga bu — taxminiy, lekin jim qolishdan yaxshi.
const (
	turnWindowM   = 30.0 // yo'nalish o'lchanadigan oyna
	minTurnDeg    = 30.0 // shundan kichik burchak — to'g'ri yo'l
	turnClusterM  = 40.0 // shu masofa ichidagi nomzodlardan bittasi
	minWindowLegM = 8.0  // oyna shundan qisqa bo'lsa (chiziq boshi/oxiri) — o'lchanmaydi
)

type turnCandidate struct {
	start float64 // klaster boshlangan masofa (m)
	pos   float64 // burchak eng katta nuqtasining masofasi (m)
	abs   float64 // burchak moduli (daraja)
	delta float64 // ishorali burchak: + o'ngga
	at    tracking.Point
}

func synthesizeSteps(pts []tracking.Point) []RouteStep {
	n := len(pts)
	if n < 3 {
		return nil
	}
	cum := make([]float64, n)
	for i := 1; i < n; i++ {
		cum[i] = cum[i-1] + pointDist(pts[i-1], pts[i])
	}

	var found []turnCandidate
	var cand *turnCandidate
	flush := func() {
		if cand != nil {
			found = append(found, *cand)
			cand = nil
		}
	}

	back, fwd := 0, 0
	for k := 1; k < n-1; k++ {
		// back — k dan kamida turnWindowM orqada bo'lgan eng yaqin tugun.
		for back < k-1 && cum[k]-cum[back+1] >= turnWindowM {
			back++
		}
		if fwd < k+1 {
			fwd = k + 1
		}
		for fwd < n-1 && cum[fwd]-cum[k] < turnWindowM {
			fwd++
		}
		if cum[k]-cum[back] < minWindowLegM || cum[fwd]-cum[k] < minWindowLegM {
			continue
		}
		d := normDeg(bearing(pts[k], pts[fwd]) - bearing(pts[back], pts[k]))
		a := math.Abs(d)

		if cand != nil && cum[k]-cand.start > turnClusterM {
			flush()
		}
		if a < minTurnDeg {
			continue
		}
		switch {
		case cand == nil:
			cand = &turnCandidate{start: cum[k], pos: cum[k], abs: a, delta: d, at: pts[k]}
		case a > cand.abs:
			// Klaster ichida eng katta burchak — burilishning haqiqiy nuqtasi.
			cand.pos, cand.abs, cand.delta, cand.at = cum[k], a, d, pts[k]
		}
	}
	flush()

	steps := make([]RouteStep, 0, len(found))
	for i, c := range found {
		next := cum[n-1]
		if i+1 < len(found) {
			next = found[i+1].pos
		}
		steps = append(steps, RouteStep{
			Maneuver:       string(maneuverForTurn(c.delta)),
			Lat:            c.at.Lat,
			Lng:            c.at.Lng,
			DistanceMeters: int(math.Round(next - c.pos)),
		})
	}
	return steps
}

// maneuverForTurn — ishorali burchak (+ o'ngga) → ovozli manevr.
func maneuverForTurn(delta float64) voice.Maneuver {
	a, right := math.Abs(delta), delta > 0
	pick := func(r, l voice.Maneuver) voice.Maneuver {
		if right {
			return r
		}
		return l
	}
	switch {
	case a >= 170:
		return voice.UTurn
	case a >= 135:
		return pick(voice.SharpRight, voice.SharpLeft)
	case a >= 60:
		return pick(voice.Right, voice.Left)
	default:
		return pick(voice.SlightRight, voice.SlightLeft)
	}
}

func pointDist(a, b tracking.Point) float64 {
	return geo.HaversineMeters(geo.LatLng{Lat: a.Lat, Lng: a.Lng}, geo.LatLng{Lat: b.Lat, Lng: b.Lng})
}

// bearing — a dan b ga yo'nalish, darajada (0..360, shimoldan soat yo'nalishida).
func bearing(a, b tracking.Point) float64 {
	const rad = math.Pi / 180
	p1, p2 := a.Lat*rad, b.Lat*rad
	dl := (b.Lng - a.Lng) * rad
	y := math.Sin(dl) * math.Cos(p2)
	x := math.Cos(p1)*math.Sin(p2) - math.Sin(p1)*math.Cos(p2)*math.Cos(dl)
	return math.Mod(math.Atan2(y, x)/rad+360, 360)
}

// normDeg — burchakni (-180, 180] ga keltiradi.
func normDeg(d float64) float64 {
	d = math.Mod(d, 360)
	switch {
	case d > 180:
		d -= 360
	case d <= -180:
		d += 360
	}
	return d
}
