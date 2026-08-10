package httpapi

import "testing"

// TestDecodePolyline — Google'ning rasmiy hujjatidagi namuna bilan tekshiradi
// (https://developers.google.com/maps/documentation/utilities/polylinealgorithm).
func TestDecodePolyline(t *testing.T) {
	const encoded = "_p~iF~ps|U_ulLnnqC_mqNvxq`@"
	want := []routePoint{
		{Lat: 38.5, Lng: -120.2},
		{Lat: 40.7, Lng: -120.95},
		{Lat: 43.252, Lng: -126.453},
	}
	got := decodePolyline(encoded)
	if len(got) != len(want) {
		t.Fatalf("%d ta nuqta kutilgan edi, olindi %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		const eps = 1e-5
		if abs(got[i].Lat-want[i].Lat) > eps || abs(got[i].Lng-want[i].Lng) > eps {
			t.Errorf("nuqta %d: kutilgan %+v, olindi %+v", i, want[i], got[i])
		}
	}
}

func TestDecodePolylineEmpty(t *testing.T) {
	if got := decodePolyline(""); len(got) != 0 {
		t.Errorf("bo'sh satr uchun bo'sh natija kutilgan edi, olindi %+v", got)
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
