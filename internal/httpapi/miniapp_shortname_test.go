package httpapi

import "testing"

// Stol QR havolasidagi qisqa nom BotFather'dagi nom bilan aynan mos
// bo'lishi kerak. Bu bir marta buzilgan: kodda `app` qattiq yozilgan
// edi, BotFather'da esa ilova `ondex` nomi bilan yaratilgan — natijada
// har bir QR kod Mini App o'rniga oddiy bot profilini ochardi.
//
// Nosozlik JIMGINA sodir bo'lgan: havola sintaktik jihatdan to'g'ri,
// server ham, Telegram ham xato bermaydi. Shuning uchun standart
// qiymat testda QAT'IY qayd etiladi — kelajakda kimdir uni qaytadan
// "app" ga o'zgartirsa, test to'xtatadi.
func TestMiniAppShortNameDefaultMatchesBotFather(t *testing.T) {
	t.Setenv("TELEGRAM_MINIAPP_SHORT_NAME", "")
	if got := miniAppShortName(); got != "ondex" {
		t.Fatalf("standart qisqa nom %q, kutilgan %q — BotFather'dagi "+
			"nom bilan mos kelmasa stol QR kodlari ishlamaydi", got, "ondex")
	}
}

func TestMiniAppShortNameEnvOverride(t *testing.T) {
	t.Setenv("TELEGRAM_MINIAPP_SHORT_NAME", "boshqa_nom")
	if got := miniAppShortName(); got != "boshqa_nom" {
		t.Fatalf("muhit o'zgaruvchisi qo'llanmadi: %q", got)
	}
}

// Probellar bilan yozilgan qiymat (`.env` da tasodifan qolib ketadigan
// eng ko'p uchraydigan xato) havolani buzmasin.
func TestMiniAppShortNameTrimsSpace(t *testing.T) {
	t.Setenv("TELEGRAM_MINIAPP_SHORT_NAME", "  ondex  ")
	if got := miniAppShortName(); got != "ondex" {
		t.Fatalf("probellar tozalanmadi: %q", got)
	}
}
