package httpapi

import "testing"

// Bitta akkaunt — yozilish shaklidan qat'i nazar BITTA hisoblagich. (9 raqamli
// shakl NormalizePhone'dan o'tmaydi, ya'ni u bilan umuman kirib bo'lmaydi.)
func TestLoginKeyNormalizes(t *testing.T) {
	same := []string{"+998901234567", "998901234567", " +998 90 123 45 67 ", "+998-90-123-45-67"}
	want := loginKey(same[0])
	for _, v := range same {
		if got := loginKey(v); got != want {
			t.Errorf("%q -> %q, kutilgan %q (cheklovni chetlab o'tish mumkin)", v, got, want)
		}
	}
	if loginKey("Ali@Mail.UZ ") != loginKey("ali@mail.uz") {
		t.Error("email katta-kichik harf bilan boshqa hisoblagich")
	}
	if loginKey("+998901234567") == loginKey("+998901234568") {
		t.Error("turli raqamlar bitta hisoblagichda")
	}
}
