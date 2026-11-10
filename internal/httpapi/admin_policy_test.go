package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// doAdminAction — xavfli admin amali (CC-0.4): sabab sarlavhasi bilan.
func doAdminAction(t *testing.T, h http.Handler, method, path, jwt, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	if jwt != "" {
		r.Header.Set("Authorization", "Bearer "+jwt)
	}
	r.Header.Set(reasonHeader, url.QueryEscape("test: tozalash"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// Sababsiz xavfli amal — 428 (403 EMAS: rol rad etilishi bilan aralashmasin).
func TestAdminActionNeedsReason(t *testing.T) {
	f := adminServer(t)
	w := do(t, f.h, "DELETE", "/admin/users/"+f.customerID, f.adminJWT, "")
	if w.Code != http.StatusPreconditionRequired || !strings.Contains(w.Body.String(), "reason_required") {
		t.Fatalf("sababsiz o'chirish: %d %s", w.Code, w.Body.String())
	}
	if w := doAdminAction(t, f.h, "DELETE", "/admin/users/"+f.customerID, f.adminJWT, ""); w.Code != http.StatusOK {
		t.Fatalf("sabab bilan: %d %s", w.Code, w.Body.String())
	}
}