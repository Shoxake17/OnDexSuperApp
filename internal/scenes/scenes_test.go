package scenes

import "testing"

// `ObjectKey` — imzolashdan OLDINGI yagona filtr. U noto'g'ri ishlasa
// server begona domendagi obyektga imzo qo'yishga urinardi yoki
// aksincha, to'g'ri maketni topa olmasdi.
func TestObjectKey(t *testing.T) {
	const base = "https://pub-xxx.r2.dev"

	cases := []struct {
		nom    string
		stored string
		want   string
	}{
		{"to'liq URL", base + "/scenes/bookcafe.pck", "scenes/bookcafe.pck"},
		{"oxirida / bo'lgan baza", base + "/scenes/a.pck", "scenes/a.pck"},
		{"kalitning o'zi", "scenes/bookcafe.pck", "scenes/bookcafe.pck"},
		{"boshida / bilan", "/scenes/bookcafe.pck", "scenes/bookcafe.pck"},
		{"chuqur yo'l", base + "/a/b/c.pck", "a/b/c.pck"},
		{"bo'sh", "", ""},
		{"faqat probel", "   ", ""},

		// ┌─ ENG MUHIM HOLAT ──────────────────────────────────────────┐
		// Begona domendagi manzil kalit sifatida QABUL QILINMASLIGI
		// kerak. Aks holda bazaga qandaydir yo'l bilan tushgan
		// `https://evil.example/x.pck` qiymati bizning bucket'imizda
		// `https:/evil.example/x.pck` kaliti sifatida izlanardi yoki
		// javobga shundayligicha chiqib ketardi.
		// └────────────────────────────────────────────────────────────┘
		{"begona domen", "https://evil.example/x.pck", ""},
		{"http begona", "http://evil.example/x.pck", ""},
		{"boshqa r2 hisobi", "https://pub-yyy.r2.dev/scenes/a.pck", ""},
	}

	for _, c := range cases {
		t.Run(c.nom, func(t *testing.T) {
			if got := ObjectKey(c.stored, base); got != c.want {
				t.Errorf("ObjectKey(%q) = %q, kutilgan %q", c.stored, got, c.want)
			}
		})
	}
}

// Baza manzili sozlanmagan bo'lsa to'liq URL kalitga aylanmasin.
func TestObjectKeyWithoutBase(t *testing.T) {
	if got := ObjectKey("https://pub-xxx.r2.dev/scenes/a.pck", ""); got != "" {
		t.Errorf("baza yo'q bo'lsa URL rad etilishi kerak, olindi %q", got)
	}
	if got := ObjectKey("scenes/a.pck", ""); got != "scenes/a.pck" {
		t.Errorf("oddiy kalit baza yo'qligidan qat'i nazar ishlashi kerak, olindi %q", got)
	}
}

// Sozlamalar to'liq bo'lmasa imzolovchi YARATILMASIN — yarim
// sozlangan holat jimgina buzuq havola berardi.
func TestNewRejectsIncompleteConfig(t *testing.T) {
	cases := [][4]string{
		{"", "key", "secret", "bucket"},
		{"acc", "", "secret", "bucket"},
		{"acc", "key", "", "bucket"},
		{"acc", "key", "secret", ""},
	}
	for _, c := range cases {
		if _, err := New(t.Context(), c[0], c[1], c[2], c[3], 0); err == nil {
			t.Errorf("to'liq bo'lmagan sozlama qabul qilindi: %v", c)
		}
	}
}
