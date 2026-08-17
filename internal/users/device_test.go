package users

import "testing"

// `X-Ondex-Client` sarlavhasi MIJOZ tomonidan yoziladi, ya'ni unga
// ishonib bo'lmaydi. Bu qiymat keyin bazaga yoziladi, admin panelida
// ko'rsatiladi va logga tushadi — shuning uchun u yerga faqat kutilgan
// shakldagi matn o'tishi kerak.
func TestParseClientHeader(t *testing.T) {
	cases := []struct {
		name         string
		in           string
		wantPlatform string
		wantVersion  string
	}{
		{"oddiy", "android/1.4.0", PlatformAndroid, "1.4.0"},
		{"mini app", "tma/2.0.0-beta", PlatformTMA, "2.0.0-beta"},
		{"registr farq qilmaydi", "Android/1.0", PlatformAndroid, "1.0"},
		{"bo'shliqlar tozalanadi", "  ios / 1.2  ", PlatformIOS, "1.2"},
		{"versiyasiz", "web", PlatformWeb, ""},
		{"sarlavha yo'q", "", PlatformUnknown, ""},

		// ── Rad etiladigan kiritmalar ──────────────────────────────
		// Notanish platforma yozib qo'yilmaydi: aks holda admin
		// panelidagi ustunga mijoz istagan matnini chiqarish mumkin
		// bo'lardi.
		{"notanish platforma", "MyHackClient/1.0", PlatformUnknown, "1.0"},
		// Yangi qator — log injection: soxta log qatori yozish yo'li.
		{"yangi qator", "android/1.0\nERROR fake", PlatformAndroid, ""},
		// Uzun satr KESILMAYDI, butunlay tashlanadi.
		{"juda uzun versiya", "android/" + string(make([]byte, 40)), PlatformAndroid, ""},
		{"begona belgilar", "android/<script>", PlatformAndroid, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotPlatform, gotVersion := ParseClientHeader(c.in)
			if gotPlatform != c.wantPlatform {
				t.Errorf("platforma: kutilgan %q, olindi %q", c.wantPlatform, gotPlatform)
			}
			if gotVersion != c.wantVersion {
				t.Errorf("versiya: kutilgan %q, olindi %q", c.wantVersion, gotVersion)
			}
		})
	}
}
