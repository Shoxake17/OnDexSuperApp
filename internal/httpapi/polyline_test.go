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

// 22-BANDNING REGRESSIYASI.
//
// ┌─ NEGA BU MUHIM ────────────────────────────────────────────────────┐
// Ichki sikllarda chegara tekshiruvi yo'q edi va KESILGAN polilinia
// (oxirgi bayt "davomi bor" deb belgilangan, davomi esa yo'q)
// `index out of range` panic'i berardi.
//
// Kirish TASHQI xizmatdan keladi (Google Directions javobi) — ya'ni
// qisman javob yoki formatning o'zgarishi serverni yiqitishi mumkin
// edi.
//
// Panic test jarayonini o'ldiradi, ya'ni testning muvaffaqiyatli
// tugashi — tuzatishning isboti. Tekshirildi: chegara tekshiruvisiz
// eski kod aynan shu kirishlarda panic beradi.
// └────────────────────────────────────────────────────────────────────┘
func TestDecodePolylineTruncatedDoesNotPanic(t *testing.T) {
	const valid = "_p~iF~ps|U_ulLnnqC_mqNvxq`@"

	cases := []struct {
		name  string
		input string
	}{
		// Har bir uzunlikda kesib ko'ramiz: kesish nuqtasi ichki
		// siklning o'rtasiga tushishi shart.
		{"bitta belgi", "_"},
		{"yarim qiymat", "_p~iF"},
		{"lat bor, lng yarim", "_p~iF~ps|U_ul"},
		// `~` (0x7E) — 63 ayirilgach 0x3F, ya'ni `>= 0x20`:
		// "davomi bor" degan belgi, davomi esa yo'q.
		{"davomi bor deb tugagan", "~~~~"},
		{"bitta davomi bor belgi", "~"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Panic bo'lsa test shu yerda o'ladi.
			got := decodePolyline(tc.input)
			// Natija bo'sh yoki qisman bo'lishi mumkin — ikkalasi ham
			// maqbul. Muhimi: qulamaydi.
			t.Logf("%d ta nuqta", len(got))
		})
	}

	// Har bir prefiks uchun ham — kesish nuqtasini o'tkazib
	// yubormaslik uchun.
	for i := 0; i <= len(valid); i++ {
		_ = decodePolyline(valid[:i])
	}
}

// Kesilgan kirishda shu paytgacha to'liq o'qilgan nuqtalar
// SAQLANISHI kerak — qisman marshrut bo'shdan yaxshiroq.
func TestDecodePolylineTruncatedKeepsCompletePoints(t *testing.T) {
	const valid = "_p~iF~ps|U_ulLnnqC_mqNvxq`@"
	// Uchinchi nuqtaning o'rtasidan kesamiz.
	truncated := valid[:len(valid)-3]

	got := decodePolyline(truncated)
	if len(got) < 2 {
		t.Fatalf("to'liq o'qilgan nuqtalar yo'qoldi: %d ta (kamida 2 kutilgan)", len(got))
	}
	if abs(got[0].Lat-38.5) > 1e-5 || abs(got[0].Lng-(-120.2)) > 1e-5 {
		t.Errorf("birinchi nuqta buzildi: %+v", got[0])
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
