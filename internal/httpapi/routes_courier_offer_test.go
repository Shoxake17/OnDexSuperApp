package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// Taklifni tiklash: faqat kuryerning O'ZI, ochiq taklif bo'lmasa `null`.
func TestCourierPendingOfferEndpoint(t *testing.T) {
	f := staffServer(t)
	_, _, c, tok := staffCourier(t, f, "+998901112250")
	_, _, other, _ := staffCourier(t, f, "+998901112251")

	w := do(t, f.h, "GET", "/couriers/"+c.ID+"/offer", tok, "")
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != `{"offer":null}` {
		t.Fatalf("taklif yo'q: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, f.h, "GET", "/couriers/"+other.ID+"/offer", tok, ""); w.Code != http.StatusForbidden {
		t.Fatalf("begona kuryer taklifi: 403 kutilgan, keldi %d", w.Code)
	}
	if w := do(t, f.h, "GET", "/couriers/"+c.ID+"/offer", f.jwt["customer"], ""); w.Code != http.StatusForbidden {
		t.Fatalf("mijoz roli: 403 kutilgan, keldi %d", w.Code)
	}
	if w := do(t, f.h, "GET", "/couriers/"+c.ID+"/offer", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("tokensiz: 401 kutilgan, keldi %d", w.Code)
	}
}
