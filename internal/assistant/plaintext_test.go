package assistant

import "testing"

// Javob OVOZ bilan o'qiladi, shuning uchun markdown belgilari qolmasligi
// kerak. Model `systemPrompt` dagi taqiqni muntazam e'tiborsiz qoldiradi
// — quyidagi holatlar HAQIQIY javoblardan olingan (DeepSeek, 2026-08-31).
func TestPlainTextStripsMarkdown(t *testing.T) {
	cases := []struct{ in, want string }{
		{
			"Jami: **49 000 so'm**",
			"Jami: 49 000 so'm",
		},
		{
			"• Osh (2 dona) — 70 000 so'm\n• Chegirma: 21 000 so'm",
			"Osh (2 dona) — 70 000 so'm\nChegirma: 21 000 so'm",
		},
		{
			"## Savatingiz\n- Osh\n- Lag'mon",
			"Savatingiz\nOsh\nLag'mon",
		},
		{
			"`propose_order` chaqirildi",
			"propose_order chaqirildi",
		},
		// `_` ataylab saqlanadi — so'z ichida uchraydi va uni
		// o'chirish `propose_order` ni `proposeorder` qilardi.
		// Modellar kursiv uchun amalda `*` ishlatadi.
		{
			"_kursiv_ va **qalin**",
			"_kursiv_ va qalin",
		},
		// Oddiy matn o'zgarmasligi kerak.
		{
			"Avigo restoranidan Osh topildi. Narxi 35 000 so'm.",
			"Avigo restoranidan Osh topildi. Narxi 35 000 so'm.",
		},
		{"", ""},
	}

	for _, c := range cases {
		if got := plainText(c.in); got != c.want {
			t.Errorf("plainText(%q)\n  oldik:   %q\n  kutilgan: %q", c.in, got, c.want)
		}
	}
}

// Apostrof (o'zbek tilida HAR QADAMDA uchraydi) va tire buzilmasligi
// kerak — ular markdown emas.
func TestPlainTextKeepsUzbekPunctuation(t *testing.T) {
	const in = "Buyurtmangiz qabul qilindi — \"Buyurtmalar\" bo'limidan ko'ring."
	if got := plainText(in); got != in {
		t.Errorf("o'zbekcha tinish belgilari buzildi:\n  oldik:   %q\n  kutilgan: %q", got, in)
	}
}
