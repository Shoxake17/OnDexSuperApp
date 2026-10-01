package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// Panel Payme/Click/Uzum bayroqlarini `payment_methods` ichida yuboradi.
// Server ularni "noto'g'ri shakl" deb rad ETMASLIGI kerak: noma'lum kalit
// qat'iy rad etiladi (`bitcoin`), lekin yangi to'rtta usul — taniladi.
func TestSettingsAcceptsPaymeClickUzumKeys(t *testing.T) {
	h, _, jwt := settingsServer(t)

	for _, key := range []string{"payme", "click", "uzum"} {
		// O'chirish har doim mumkin (yangi yoqilmayapti).
		w := do(t, h, "PATCH", settingsPathA, jwt["a"],
			`{"payment_methods":{"cash":true,"card_terminal":false,"card_online":false,"`+key+`":false}}`)
		if w.Code != http.StatusOK {
			t.Errorf("%s:false -> %d — %s", key, w.Code, w.Body.String())
		}
		// Yoqish: shakl taniladi. Ulanmagan bo'lsa sababi aytiladi, shakl xatosi EMAS.
		w = do(t, h, "PATCH", settingsPathA, jwt["a"],
			`{"payment_methods":{"cash":true,"card_terminal":false,"card_online":false,"`+key+`":true}}`)
		if strings.Contains(w.Body.String(), "noto'g'ri shaklda") {
			t.Errorf("%s:true shakl xatosi berdi: %s", key, w.Body.String())
		}
	}

	// Noma'lum usul baribir rad etiladi.
	w := do(t, h, "PATCH", settingsPathA, jwt["a"], `{"payment_methods":{"cash":true,"bitcoin":true}}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("bitcoin: %d", w.Code)
	}
}
