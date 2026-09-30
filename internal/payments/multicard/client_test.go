package multicard

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Javoblar SHAKLI haqiqiy sinov stendidan olingan
// (dev-mesh.multicard.uz, 2026-09-30 da o'lchangan) — ya'ni tahlil
// qiladigan kod haqiqiy ma'lumotga qarshi sinaladi.

// fakeServer — `/auth` va `/payment/store/{id}/history` ni taqlid
// qiladi va chaqiruvlarni sanaydi.
type fakeServer struct {
	authCalls    atomic.Int32
	historyCalls atomic.Int32
	expiry       string
	historyBody  string
	authStatus   int
}

func (f *fakeServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		f.authCalls.Add(1)
		if f.authStatus != 0 && f.authStatus != http.StatusOK {
			w.WriteHeader(f.authStatus)
			_, _ = w.Write([]byte(`{"error":"Invalid Credentials"}`))
			return
		}
		exp := f.expiry
		if exp == "" {
			exp = time.Now().Add(24 * time.Hour).Format("2006-01-02 15:04:05")
		}
		_, _ = w.Write([]byte(`{"token":"tok-123","role":"dev","expiry":"` + exp + `"}`))
	})
	mux.HandleFunc("GET /payment/store/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		f.historyCalls.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer tok-123" {
			t.Errorf("Authorization sarlavhasi: %q", got)
		}
		body := f.historyBody
		if body == "" {
			// ⚠️ SHAKL HAQIQIY STENDDAN: `stat` da summa SATR,
			// `list` da SON, `ps` esa `null` bo'lishi mumkin
			// (`Tiyin` izohiga qarang). Soxta javobni "toza"
			// qilib yozish aynan shu xatoni yashirgan edi.
			body = `{"success":true,"data":{"pagination":{"total":2,"offset":0,"limit":200},
			  "stat":[{"status":"success","ps":"uzcard","payment_amount":"137725709914"},
			          {"status":"draft","ps":null,"payment_amount":"1569969306"},
			          {"status":"revert","ps":"uzcard","payment_amount":"8764979406"}],
			  "list":[{"uuid":"u1","status":"success","ps":"uzcard","store_invoice_id":"order-1",
			           "payment_amount":40000000,"commission_amount":600000,
			           "card_pan":"860030******5959","payment_time":"2026-09-30 14:36:31",
			           "refund_time":null}]}}`
		}
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := New(Config{BaseURL: srv.URL, ApplicationID: "rhmt_test", Secret: "sir"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestNewRequiresCredentials(t *testing.T) {
	for _, c := range []Config{
		{},
		{BaseURL: "https://x"},
		{BaseURL: "https://x", ApplicationID: "a"},
		{ApplicationID: "a", Secret: "s"},
		// Bo'sh joy sir emas.
		{BaseURL: "https://x", ApplicationID: "a", Secret: "   "},
	} {
		if _, err := New(c); !errors.Is(err, ErrNotConfigured) {
			t.Errorf("%+v: %v, ErrNotConfigured kutilgan", c, err)
		}
	}
}

func TestStoreHistoryParsesRealShape(t *testing.T) {
	f := &fakeServer{}
	c := newClient(t, f.start(t))

	h, err := c.StoreHistory(context.Background(), HistoryQuery{
		StoreID: "6",
		From:    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		To:      time.Date(2026, 9, 30, 23, 59, 59, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if h.Pagination.Total != 2 || len(h.List) != 1 {
		t.Fatalf("javob o'qilmadi: %+v", h)
	}
	tr := h.List[0]
	if tr.AmountTiyin.Int64() != 40_000_000 || tr.CommissionTiyin.Int64() != 600_000 {
		t.Errorf("summa/komissiya: %+v", tr)
	}
	if tr.StoreInvoiceID != "order-1" {
		t.Errorf("store_invoice_id: %q", tr.StoreInvoiceID)
	}
}

// ┌─ ENG MUHIM TEKSHIRUV ─────────────────────────────────────────────┐
// Reestrda `draft`, `cancelled`, `error`, `hold` holatlari ham bor va
// ular yig'indisi `success` dan KATTA bo'lishi mumkin (sinov stendida
// aynan shunday edi). Ularni qo'shib yuborish restoranga bir necha
// barobar oshirilgan "daromad" ko'rsatardi.
// └───────────────────────────────────────────────────────────────────┘
func TestSuccessTiyinCountsOnlySuccess(t *testing.T) {
	f := &fakeServer{}
	c := newClient(t, f.start(t))
	h, err := c.StoreHistory(context.Background(), HistoryQuery{
		StoreID: "6", From: time.Now().Add(-24 * time.Hour), To: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := h.SuccessTiyin(); got != 137_725_709_914 {
		t.Errorf("tushum %d — faqat `success` qo'shilishi kerak", got)
	}
	if got := h.RefundedTiyin(); got != 8_764_979_406 {
		t.Errorf("qaytarilgan %d", got)
	}
}

// Token keshlanadi: har so'rov uchun qayta `POST /auth` qilish
// keraksiz tarmoq sayohati.
func TestTokenIsCached(t *testing.T) {
	f := &fakeServer{}
	c := newClient(t, f.start(t))
	for range 3 {
		if _, err := c.StoreHistory(context.Background(), HistoryQuery{
			StoreID: "6", From: time.Now().Add(-time.Hour), To: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.authCalls.Load(); got != 1 {
		t.Errorf("auth %d marta chaqirildi, 1 kutilgan", got)
	}
	if got := f.historyCalls.Load(); got != 3 {
		t.Errorf("history %d marta", got)
	}
}

// Muddati o'tgan token QAYTA olinadi.
func TestExpiredTokenIsRenewed(t *testing.T) {
	f := &fakeServer{expiry: time.Now().Add(-time.Hour).Format("2006-01-02 15:04:05")}
	c := newClient(t, f.start(t))
	for range 2 {
		_, _ = c.StoreHistory(context.Background(), HistoryQuery{
			StoreID: "6", From: time.Now().Add(-time.Hour), To: time.Now(),
		})
	}
	if got := f.authCalls.Load(); got != 2 {
		t.Errorf("auth %d marta, 2 kutilgan (token eskirgan)", got)
	}
}

func TestAuthFailure(t *testing.T) {
	f := &fakeServer{authStatus: http.StatusBadRequest}
	c := newClient(t, f.start(t))
	_, err := c.StoreHistory(context.Background(), HistoryQuery{
		StoreID: "6", From: time.Now().Add(-time.Hour), To: time.Now(),
	})
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("%v, ErrAuth kutilgan", err)
	}
	// ⚠️ Xato matnida SIR bo'lmasligi kerak — u logga tushadi.
	if strings.Contains(err.Error(), "sir") {
		t.Errorf("xato matnida sir bor: %v", err)
	}
}

// HTTP 200 bo'lsa ham `success:false` kelishi mumkin — faqat status
// kodga ishonib bo'lmaydi.
func TestSuccessFalseIsError(t *testing.T) {
	f := &fakeServer{historyBody: `{"success":false,"error":{"code":"ERROR_NOT_FOUND","details":"Object not found"}}`}
	c := newClient(t, f.start(t))
	_, err := c.StoreHistory(context.Background(), HistoryQuery{
		StoreID: "6", From: time.Now().Add(-time.Hour), To: time.Now(),
	})
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("%v, APIError kutilgan", err)
	}
	if ae.Code != "ERROR_NOT_FOUND" {
		t.Errorf("kod: %q", ae.Code)
	}
}

func TestStoreIDRequired(t *testing.T) {
	f := &fakeServer{}
	c := newClient(t, f.start(t))
	if _, err := c.StoreHistory(context.Background(), HistoryQuery{
		From: time.Now().Add(-time.Hour), To: time.Now(),
	}); err == nil {
		t.Error("bo'sh store_id xato berishi kerak")
	}
}
