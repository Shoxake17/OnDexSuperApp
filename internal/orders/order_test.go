package orders

import (
	"slices"
	"testing"
)

// Terminal holatlar ro'yxatining YAGONA MANBA ekanini qulflaydigan
// testlar (bug.md 45 va 78-bandlar).
//
// ┌─ NEGA BU TEST KERAK ───────────────────────────────────────────────┐
// Bu ro'yxat loyihada olti joyda qo'lda takrorlangan edi va uchtasida
// `served` tushib qolgandi. Eng qimmatlisi Postgres so'rovidagi
// literal edi: xotiradagi (dev) repo `IsTerminal()` ni chaqirgani
// uchun xato faqat PRODUCTION'da chiqardi — restoranni o'chirish
// abadiy "faol buyurtmalari bor" deb rad etilardi.
//
// Endi SQL ham `TerminalStatusStrings()` dan oziqlanadi. Bu test
// ikkala kirish nuqtasi (`IsTerminal` va `TerminalStatusStrings`)
// bir-biridan ajralib ketmasligini qulflaydi.
// └────────────────────────────────────────────────────────────────────┘

func TestTerminalStatusStringsMatchesIsTerminal(t *testing.T) {
	all := []Status{
		StatusCreated, StatusAccepted, StatusPreparing, StatusReady,
		StatusPickedUp, StatusDelivered, StatusServed,
		StatusRejected, StatusCancelled,
	}
	list := TerminalStatusStrings()

	for _, s := range all {
		o := &Order{Status: s}
		inList := slices.Contains(list, string(s))
		if o.IsTerminal() != inList {
			t.Errorf("%q: IsTerminal()=%v, TerminalStatusStrings ichida=%v — ikki manba ajralib ketdi",
				s, o.IsTerminal(), inList)
		}
	}
}

// `served` — STOL buyurtmasining yakuniy holati. Aynan shu qiymat
// SQL ro'yxatidan tushib qolgan edi, shuning uchun alohida qulflanadi.
func TestServedIsTerminal(t *testing.T) {
	if !(&Order{Status: StatusServed}).IsTerminal() {
		t.Fatal("served terminal emas deb hisoblanmoqda")
	}
	if !slices.Contains(TerminalStatusStrings(), "served") {
		t.Fatalf("SQL ro'yxatida served yo'q: %v", TerminalStatusStrings())
	}
}

// Faol holatlar terminal deb belgilanib qolmasin — teskari xato ham
// xuddi shunday zararli (yetkazilmagan buyurtma "yakunlandi" deb
// hisoblanardi).
func TestActiveStatusesAreNotTerminal(t *testing.T) {
	for _, s := range []Status{
		StatusCreated, StatusAccepted, StatusPreparing, StatusReady, StatusPickedUp,
	} {
		if (&Order{Status: s}).IsTerminal() {
			t.Errorf("%q terminal deb belgilandi", s)
		}
	}
}

// Ro'yxat nusxa qaytarishi kerak: chaqiruvchi uni o'zgartirsa,
// paket ichidagi manba buzilmasin.
func TestTerminalStatusStringsReturnsCopy(t *testing.T) {
	first := TerminalStatusStrings()
	first[0] = "buzildi"
	if slices.Contains(TerminalStatusStrings(), "buzildi") {
		t.Fatal("TerminalStatusStrings ichki ro'yxatga havola qaytardi")
	}
}
