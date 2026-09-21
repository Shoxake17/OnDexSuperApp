package delivery

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// Masofalar haqiqiy koordinatalar bo'yicha hisoblangan.
func TestCoveredServiceCities(t *testing.T) {
	cases := []struct {
		name    string
		lat     float64
		lng     float64
		covered bool
	}{
		{"Chust markazi", 41.0004, 71.2394, true},
		{"Chust chekkasi", 41.0300, 71.2700, true},
		// Qurilmada haqiqiy GPS shu nuqtani qaytargan — qamrovda bo'lishi shart.
		{"Chust (haqiqiy GPS)", 41.0008, 71.2208, true},
		{"Pop", 40.8742, 71.1103, false},
		{"Namangan", 40.9983, 71.6726, false},
		{"Qo'qon", 40.5286, 70.9425, false},
		{"Toshkent markazi", 41.3111, 69.2797, true},
		{"Toshkent (Chilonzor)", 41.2995, 69.2401, true},
		{"Toshkent (Sergeli, ~11 km)", 41.2270, 69.2190, true},
		{"Toshkent (Yunusobod)", 41.3650, 69.2880, true},
		// Viloyat shaharlari — xizmat radiusidan (20 km) tashqarida.
		{"Chirchiq (~31 km)", 41.4689, 69.5822, false},
		{"Nurafshon (~30 km)", 41.0417, 69.3589, false},
		{"Samarqand", 39.6542, 66.9597, false},
		// Koordinata umuman berilmagan holat — qamrovda BO'LMASLIGI kerak,
		// aks holda manzilsiz buyurtma o'tib ketardi.
		{"nol koordinata", 0, 0, false},
	}
	for _, c := range cases {
		if got := Covered(c.lat, c.lng); got != c.covered {
			t.Errorf("%s: Covered(%v,%v) = %v, kutilgan %v",
				c.name, c.lat, c.lng, got, c.covered)
		}
	}
}

func TestCityForReturnsName(t *testing.T) {
	for _, c := range []struct {
		lat, lng float64
		want     string
	}{
		{41.0004, 71.2394, "Chust"},
		{41.2995, 69.2401, "Toshkent"},
	} {
		city := CityFor(c.lat, c.lng)
		if city == nil {
			t.Fatalf("(%v,%v) qamrovda bo'lishi kerak edi", c.lat, c.lng)
		}
		if city.Name != c.want {
			t.Errorf("shahar nomi = %q, kutilgan %q", city.Name, c.want)
		}
	}
	if CityFor(39.6542, 66.9597) != nil {
		t.Error("Samarqand uchun nil kutilgan edi")
	}
}

func TestCheckServes(t *testing.T) {
	const (
		chustLat, chustLng       = 41.0004, 71.2394
		tashkentLat, tashkentLng = 41.2995, 69.2401
	)
	cases := []struct {
		name             string
		restLat, restLng float64
		addrLat, addrLng float64
		want             error
	}{
		{"Chust → Chust", chustLat, chustLng, 41.0300, 71.2700, nil},
		{"Toshkent → Toshkent", tashkentLat, tashkentLng, 41.3650, 69.2880, nil},
		{"Toshkent restorani, Chust manzili", tashkentLat, tashkentLng, chustLat, chustLng, ErrOtherCity},
		{"Chust restorani, Toshkent manzili", chustLat, chustLng, tashkentLat, tashkentLng, ErrOtherCity},
		// Restoran Chust doirasidan tashqarida (Namangan yo'lida), lekin
		// operatsion radius ichida — u Chust restorani hisoblanadi.
		{"Chust chekkasidagi restoran → Chust", 41.0011, 71.6673, chustLat, chustLng, nil},
		{"Chust chekkasidagi restoran → Toshkent", 41.0011, 71.6673, tashkentLat, tashkentLng, ErrOtherCity},
		// Manzil hech bir shaharda emas — `Covered` baribir rad etadi,
		// lekin bu funksiya ham o'zicha xavfsiz bo'lishi kerak.
		{"manzil hududdan tashqarida", chustLat, chustLng, 39.6542, 66.9597, ErrOtherCity},
		// Koordinatasi kiritilmagan restoran — avvalgi xatti-harakat.
		{"restoran koordinatasi yo'q", 0, 0, chustLat, chustLng, nil},
		{"restoran hech bir shaharga yaqin emas", 39.6542, 66.9597, chustLat, chustLng, nil},
	}
	for _, c := range cases {
		got := CheckServes(c.restLat, c.restLng, c.addrLat, c.addrLng)
		if !errors.Is(got, c.want) {
			t.Errorf("%s: CheckServes = %v, kutilgan %v", c.name, got, c.want)
		}
	}
}

// Server va web ro'yxatlari AYNAN bir xil bo'lishi SHART: web faqat
// qulaylik uchun tekshiradi (xaritadagi doira, tugma), qaror esa
// serverda. Ular ajralsa, mijoz xaritada "yetkazamiz" deb ko'rgan nuqtaga
// server buyurtma bermaydi (yoki aksincha). Avval bu faqat izohda
// yozilgan edi — endi test ushlaydi.
func TestServiceCitiesMatchWeb(t *testing.T) {
	path := filepath.Join("..", "..", "apps", "web", "lib", "service-area.ts")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s o'qilmadi: %v", path, err)
	}
	re := regexp.MustCompile(`\{\s*name:\s*"([^"]+)",\s*lat:\s*([-\d.]+),\s*lng:\s*([-\d.]+),\s*radiusKm:\s*([\d.]+)\s*\}`)
	matches := re.FindAllStringSubmatch(string(src), -1)
	if len(matches) != len(Cities) {
		t.Fatalf("web'da %d ta shahar, serverda %d ta", len(matches), len(Cities))
	}
	num := func(s string) float64 {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			t.Fatalf("raqam emas: %q", s)
		}
		return v
	}
	for i, m := range matches {
		c := Cities[i]
		if m[1] != c.Name || num(m[2]) != c.Lat || num(m[3]) != c.Lng || num(m[4]) != c.RadiusKM {
			t.Errorf("%d-shahar farq qiladi: web %v, server %+v", i, m[1:], c)
		}
	}
}
