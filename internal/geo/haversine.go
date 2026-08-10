package geo

import "math"

// HaversineMeters — ikki koordinata orasidagi TO'G'RI CHIZIQ masofasi
// (metrda).
//
// ── QAYERDA ISHLATILADI ────────────────────────────────────────────
//   - `couriers` — Google Distance Matrix ishlamay qolganda zaxira ETA;
//   - `storage.MemoryCourierRepo` — yaqinlik bo'yicha qidiruv
//     (Postgres'da bu ish PostGIS `ST_DWithin` bilan bajariladi,
//     migration 0029).
//
// Ikkala joyda BIR XIL hisob bo'lishi shart: aks holda xotiradagi
// (test/dev) va Postgres (production) rejimlari boshqacha natija berib,
// testlar production'ni ifodalamay qolardi.
//
// ── ANIQLIK ────────────────────────────────────────────────────────
// Haversine Yerni SFERА deb hisoblaydi (PostGIS `geography` esa
// ellipsoid — WGS84). Farq ~0.5% gacha: bitta shahar ichida bu bir
// necha metr, ya'ni kuryer tanlashda ahamiyatsiz. Aniqroq kerak
// bo'lsa Vincenty formulasi kerak bo'ladi, lekin bu yerda ortiqcha.
func HaversineMeters(a, b LatLng) float64 {
	const earthRadiusM = 6371000.0
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusM * c
}
