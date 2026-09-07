package httpapi

import "testing"

// Media manzillarining tekshiruvi (bug.md 24-band).
//
// ┌─ NEGA MUHIM ───────────────────────────────────────────────────────┐
// Bu maydonlarda FAQAT uzunlik tekshirilardi. Restoran xodimi
// (`staff` roli kifoya) mijozlarning ilovasini begona domendan
// rasm/PDF yuklashga majburlay olardi — kamida kuzatuv (IP to'plash),
// ko'pi bilan ilova ichidagi kontentni almashtirish.
// └────────────────────────────────────────────────────────────────────┘

func TestIsSafeMediaURL(t *testing.T) {
	cases := map[string]bool{
		// Ruxsat etilgan.
		"":                           true, // maydon ixtiyoriy
		"   ":                        true,
		"https://cdn.example/a.png":  true,
		"https://pub-x.r2.dev/b.glb": true,
		"/uploads/rasm.png":          true, // lokal disk rejimi
		"/uploads/a/b/c.pdf":         true,

		// XAVFLI SXEMALAR — asosiy regressiya.
		"javascript:alert(1)":          false,
		"JavaScript:alert(1)":          false,
		"data:text/html;base64,PHNjcg": false,
		"file:///etc/passwd":           false,
		"vbscript:msgbox":              false,

		// `http://` — brauzer uni aralash kontent sifatida baribir
		// bloklaydi (68-band).
		"http://example.com/a.png": false,

		// Protokolga nisbiy — brauzer buni MUTLAQ deb o'qiydi.
		"//evil.example/a.png": false,
		// Teskari chiziq hiylasi (50-band bilan bir xil sinf).
		"/\\evil.example/a.png": false,

		// Sxemasiz host — noaniq, rad etiladi.
		"evil.example/a.png": false,
		// Host'siz https.
		"https://": false,
	}
	for in, want := range cases {
		if got := isSafeMediaURL(in); got != want {
			t.Errorf("isSafeMediaURL(%q) = %v, kutilgan %v", in, got, want)
		}
	}
}

// `isOwnMediaURL` — qat'iyroq daraja: FAQAT bizning R2 domenimiz.
// Hozir media maydonlarida ishlatilmaydi (mavjud ma'lumotni buzmasin),
// lekin media butunlay R2 ga ko'chgach yoqiladi.
func TestIsOwnMediaURL(t *testing.T) {
	t.Setenv("R2_PUBLIC_URL", "https://pub-abc.r2.dev")

	if !isOwnMediaURL("https://pub-abc.r2.dev/models/a.glb") {
		t.Error("o'z domenimiz rad etildi")
	}
	if isOwnMediaURL("https://evil.example/models/a.glb") {
		t.Error("begona domen qabul qilindi")
	}
	// Prefiks hiylasi: `pub-abc.r2.dev.evil.example`.
	if isOwnMediaURL("https://pub-abc.r2.dev.evil.example/a.glb") {
		t.Error("o'xshash domen qabul qilindi — prefiks bo'yicha chetlab o'tildi")
	}
	if !isOwnMediaURL("") {
		t.Error("bo'sh qiymat rad etildi")
	}
}

// R2 sozlanmagan bo'lsa `isOwnMediaURL` HECH NARSANI qabul
// qilmasligi kerak (fail-closed) — `isAllowedSceneURL` bilan bir xil
// qoida.
func TestIsOwnMediaURLFailsClosedWithoutR2(t *testing.T) {
	t.Setenv("R2_PUBLIC_URL", "")
	if isOwnMediaURL("https://nimadir.example/a.glb") {
		t.Error("R2 sozlanmagan holda manzil qabul qilindi")
	}
	// `http://` bazasi ham rad etilishi kerak.
	t.Setenv("R2_PUBLIC_URL", "http://pub-abc.r2.dev")
	if isOwnMediaURL("http://pub-abc.r2.dev/a.glb") {
		t.Error("http bazasi qabul qilindi")
	}
}
