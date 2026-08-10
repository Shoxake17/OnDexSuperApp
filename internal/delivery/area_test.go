package delivery

import "testing"

// Xizmat hududi frontend (`apps/web/lib/service-area.ts`) bilan bir xil
// natija berishi SHART — bu test ikkalasi ajralib ketmasligini ushlab
// turadi. Masofalar haqiqiy koordinatalar bo'yicha hisoblangan.
func TestCoveredOnlyChust(t *testing.T) {
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
		{"Toshkent", 41.2995, 69.2401, false},
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
	city := CityFor(41.0004, 71.2394)
	if city == nil {
		t.Fatal("Chust markazi qamrovda bo'lishi kerak edi")
	}
	if city.Name != "Chust" {
		t.Errorf("shahar nomi = %q, kutilgan \"Chust\"", city.Name)
	}
	if CityFor(41.2995, 69.2401) != nil {
		t.Error("Toshkent uchun nil kutilgan edi")
	}
}
