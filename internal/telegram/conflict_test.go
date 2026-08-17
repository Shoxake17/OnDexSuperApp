package telegram

import (
	"errors"
	"strings"
	"testing"
)

// 409 boshqa xatolardan AJRATILISHI kerak: uning sababi (ikkita server
// bitta botni polling qilyapti) va yechimi (dev uchun alohida bot)
// butunlay boshqacha. Ilgari u oddiy tarmoq xatosi bilan bir xil
// loglanardi va sababini topish soatlab vaqt olgan edi.
func TestClassifyErrorDetectsConflict(t *testing.T) {
	// Telegram'ning haqiqiy javobi.
	real := "Conflict: terminated by other getUpdates request; " +
		"make sure that only one bot instance is running"

	err := classifyError(real)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("409 aniqlanmadi: %v", err)
	}
	// Asl matn ham saqlanishi kerak — logda to'liq sabab ko'rinsin.
	if !strings.Contains(err.Error(), "getUpdates") {
		t.Errorf("asl tavsif yo'qolgan: %v", err)
	}
}

func TestClassifyErrorOtherErrorsAreNotConflict(t *testing.T) {
	for _, d := range []string{
		"Unauthorized",
		"Bad Request: chat not found",
		"Too Many Requests: retry after 5",
		"",
	} {
		err := classifyError(d)
		if errors.Is(err, ErrConflict) {
			t.Errorf("%q noto'g'ri ravishda 409 deb belgilandi", d)
		}
		if err == nil {
			t.Errorf("%q uchun xato qaytmadi", d)
		}
	}
}
