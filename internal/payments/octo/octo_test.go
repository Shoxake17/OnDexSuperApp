package octo

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chustapp/internal/payments"
)

// newTestClient — Octo o'rniga soxta server. Har so'rovning TANASI
// tekshiriladi: pul yo'lida "qanday so'rov ketdi" degan savol eng
// muhimi.
func newTestClient(t *testing.T, handler func(path string, body map[string]any) (int, string)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("so'rov JSON emas: %s", raw)
		}
		code, resp := handler(r.URL.Path, body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(srv.Close)

	c, err := New(Config{
		ShopID: 42911, Secret: "test-secret", Test: true,
		BaseURL: srv.URL, HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// ═══════════════════════════════════════════════════════════════════
// PUL: TIYIN <-> O'NLIK SON (float64'siz)
// ═══════════════════════════════════════════════════════════════════

func TestDecimalFromTiyin(t *testing.T) {
	cases := map[int64]string{
		0:        "0.00",
		1:        "0.01",
		99:       "0.99",
		100:      "1.00",
		1200000:  "12000.00",
		43727866: "437278.66",
		-500:     "-5.00",
	}
	for tiyin, want := range cases {
		if got := payments.DecimalFromTiyin(tiyin); got != want {
			t.Errorf("%d tiyin -> %q, kutilgan %q", tiyin, got, want)
		}
	}
}

func TestTiyinFromDecimal(t *testing.T) {
	ok := map[string]int64{
		"0":         0,
		"1":         100,
		"1.5":       150,
		"1.05":      105,
		"437278.66": 43727866,
		"12000.00":  1200000,
		"100.6600":  10066, // ortiqcha nollar zarar qilmaydi
		" 250.25 ":  25025,
		"-5.00":     -500,
		"1000":      100000,
		"0.01":      1,
	}
	for s, want := range ok {
		got, err := payments.TiyinFromDecimal(s)
		if err != nil {
			t.Errorf("%q: kutilmagan xato %v", s, err)
			continue
		}
		if got != want {
			t.Errorf("%q -> %d, kutilgan %d", s, got, want)
		}
	}

	// Tiyindan mayda qism JIMGINA yaxlitlanmaydi — pul hisobida
	// farq yaratardi.
	for _, bad := range []string{"", "abc", "100.005", "1.999"} {
		if _, err := payments.TiyinFromDecimal(bad); err == nil {
			t.Errorf("%q rad etilishi kerak edi", bad)
		}
	}
}

// Aylanma o'girish: har qanday tiyin -> matn -> tiyin, o'zgarishsiz.
func TestMoneyRoundTrip(t *testing.T) {
	for _, tiyin := range []int64{0, 1, 7, 99, 100, 12345, 1200000, 43727866, 999999999} {
		s := payments.DecimalFromTiyin(tiyin)
		back, err := payments.TiyinFromDecimal(s)
		if err != nil {
			t.Fatalf("%d -> %q: %v", tiyin, s, err)
		}
		if back != tiyin {
			t.Errorf("%d -> %q -> %d", tiyin, s, back)
		}
	}
}

// ═══════════════════════════════════════════════════════════════════
// CREATE
// ═══════════════════════════════════════════════════════════════════

const okCreateResponse = `{"error":0,"data":{
  "shop_transaction_id":"pay-1",
  "octo_payment_UUID":"e3f40dc3-4955-412a-853a-2ddd28d3201f",
  "status":"created",
  "octo_pay_url":"https://pay2.octo.uz/pay/e3f40dc3?language=uz",
  "total_sum":12000.00}}`

// ★ Ikki bosqichli to'lov: `auto_capture` FALSE bo'lishi SHART —
// aks holda pul darhol yechiladi va restoran rad etganda mijozga
// qaytarish (1-3 kun) kerak bo'lardi.
func TestCreateHoldSendsAutoCaptureFalse(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, func(path string, body map[string]any) (int, string) {
		if path != "/prepare_payment" {
			t.Errorf("yo'l: %s", path)
		}
		got = body
		return 200, okCreateResponse
	})

	res, err := c.Create(context.Background(), payments.CreateRequest{
		PaymentID: "pay-1", AmountTiyin: 1200000, Description: "Buyurtma #7",
		Hold: true, NotifyURL: "https://x/notify", ReturnURL: "https://x/ok",
		TTLMinutes: 15, Language: "uz", CustomerID: "u1", CustomerPhone: "998901234567",
	})
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}

	if got["auto_capture"] != false {
		t.Errorf("auto_capture: %v, kutilgan false (hold)", got["auto_capture"])
	}
	if got["test"] != true {
		t.Errorf("test rejimi yoqilmagan: %v", got["test"])
	}
	// Summa TIYINDA emas, o'nlik songa aylantirilgan holda ketishi kerak.
	if fmt.Sprint(got["total_sum"]) != "12000" && fmt.Sprint(got["total_sum"]) != "12000.00" {
		t.Errorf("total_sum: %v (12000.00 kutilgan)", got["total_sum"])
	}
	if got["shop_transaction_id"] != "pay-1" {
		t.Errorf("shop_transaction_id: %v", got["shop_transaction_id"])
	}
	if got["octo_shop_id"].(float64) != 42911 {
		t.Errorf("octo_shop_id: %v", got["octo_shop_id"])
	}
	if got["currency"] != "UZS" {
		t.Errorf("currency: %v", got["currency"])
	}
	// Fiskal ma'lumot to'qib yuborilmasligi kerak.
	if _, ok := got["basket"]; ok {
		t.Error("basket yuborilmasligi kerak (soxta soliq ma'lumoti)")
	}
	if res.Status != payments.StatusPending {
		t.Errorf("holat: %s", res.Status)
	}
	if res.ProviderPaymentID == "" || res.PayURL == "" {
		t.Errorf("javobdan UUID/URL olinmadi: %+v", res)
	}
}

// Bir bosqichli rejim (Hold=false) — auto_capture true.
func TestCreateWithoutHold(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, func(_ string, body map[string]any) (int, string) {
		got = body
		return 200, okCreateResponse
	})
	if _, err := c.Create(context.Background(), payments.CreateRequest{
		PaymentID: "pay-2", AmountTiyin: 100, Hold: false,
	}); err != nil {
		t.Fatal(err)
	}
	if got["auto_capture"] != true {
		t.Errorf("auto_capture: %v, kutilgan true", got["auto_capture"])
	}
}

// Provayder xatosi ANIQ turda qaytadi — chaqiruvchi "tarmoq uzildi"
// bilan "octo rad etdi" ni ajrata olishi kerak.
func TestCreateProviderError(t *testing.T) {
	c := newTestClient(t, func(_ string, _ map[string]any) (int, string) {
		return 200, `{"error":2,"errMessage":"Wrong secret","data":null}`
	})
	_, err := c.Create(context.Background(), payments.CreateRequest{
		PaymentID: "pay-3", AmountTiyin: 100,
	})
	var pe *payments.ProviderError
	if !errors.As(err, &pe) {
		t.Fatalf("ProviderError kutilgan edi: %v", err)
	}
	if pe.Code != 2 || !strings.Contains(pe.Message, "Wrong secret") {
		t.Errorf("xato: %+v", pe)
	}
}

func TestCreateRejectsZeroAmount(t *testing.T) {
	c := newTestClient(t, func(_ string, _ map[string]any) (int, string) {
		t.Error("nol summada so'rov UMUMAN ketmasligi kerak")
		return 200, okCreateResponse
	})
	if _, err := c.Create(context.Background(), payments.CreateRequest{PaymentID: "p", AmountTiyin: 0}); err == nil {
		t.Error("nol summa rad etilishi kerak edi")
	}
}

// ═══════════════════════════════════════════════════════════════════
// STATUS / CAPTURE / CANCEL / REFUND
// ═══════════════════════════════════════════════════════════════════

func TestStatusMapsAndParsesAmount(t *testing.T) {
	c := newTestClient(t, func(_ string, body map[string]any) (int, string) {
		// Holat so'rovida FAQAT uchta maydon ketadi.
		if _, ok := body["total_sum"]; ok {
			t.Error("holat so'rovida ortiqcha maydonlar bor")
		}
		return 200, `{"error":0,"data":{"shop_transaction_id":"pay-1",
		  "octo_payment_UUID":"uuid-1","status":"waiting_for_capture",
		  "total_sum":437278.66,"refunded_sum":0}}`
	})
	st, err := c.Status(context.Background(), "pay-1")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != payments.StatusHeld {
		t.Errorf("holat: %s, kutilgan held", st.Status)
	}
	if st.PaidAmountTiyin != 43727866 {
		t.Errorf("summa: %d tiyin, kutilgan 43727866", st.PaidAmountTiyin)
	}
	if len(st.Raw) == 0 {
		t.Error("xom javob saqlanmadi")
	}
}

func TestStatusMapping(t *testing.T) {
	cases := map[string]payments.Status{
		"created":             payments.StatusPending,
		"wait_user_action":    payments.StatusPending,
		"waiting_for_capture": payments.StatusHeld,
		"succeeded":           payments.StatusPaid,
		"canceled":            payments.StatusCanceled,
		// Noma'lum status hech qachon "to'landi" bo'lmaydi.
		"some_new_status": payments.StatusFailed,
	}
	for in, want := range cases {
		if got := MapStatus(in); got != want {
			t.Errorf("%q -> %s, kutilgan %s", in, got, want)
		}
	}
}

func TestCaptureAndCancel(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, func(path string, body map[string]any) (int, string) {
		if path != "/set_accept" {
			t.Errorf("yo'l: %s", path)
		}
		got = body
		return 200, `{"error":0,"data":{"octo_payment_UUID":"uuid-1","status":"succeeded"}}`
	})

	if err := c.Capture(context.Background(), "uuid-1", 1200000); err != nil {
		t.Fatal(err)
	}
	if got["accept_status"] != "capture" {
		t.Errorf("accept_status: %v", got["accept_status"])
	}
	if fmt.Sprint(got["final_amount"]) != "12000" && fmt.Sprint(got["final_amount"]) != "12000.00" {
		t.Errorf("final_amount: %v", got["final_amount"])
	}

	if err := c.Cancel(context.Background(), "uuid-1"); err != nil {
		t.Fatal(err)
	}
	if got["accept_status"] != "cancel" {
		t.Errorf("accept_status: %v", got["accept_status"])
	}
	// Bekor qilishda summa yuborilmaydi.
	if _, ok := got["final_amount"]; ok {
		t.Errorf("cancel'da final_amount yuborilmasligi kerak: %v", got["final_amount"])
	}
}

func TestRefund(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, func(path string, body map[string]any) (int, string) {
		if path != "/refund" {
			t.Errorf("yo'l: %s", path)
		}
		got = body
		return 200, `{"error":0,"data":{"octo_payment_UUID":"uuid-1","refund_id":"r1","status":"succeeded"}}`
	})
	if err := c.Refund(context.Background(), "uuid-1", "refund-1", 500000); err != nil {
		t.Fatal(err)
	}
	if got["shop_refund_id"] != "refund-1" {
		t.Errorf("shop_refund_id: %v", got["shop_refund_id"])
	}
	if fmt.Sprint(got["amount"]) != "5000" && fmt.Sprint(got["amount"]) != "5000.00" {
		t.Errorf("amount: %v", got["amount"])
	}
}

// ═══════════════════════════════════════════════════════════════════
// CALLBACK IMZOSI
// ═══════════════════════════════════════════════════════════════════

func TestVerifyCallbackSignature(t *testing.T) {
	c, err := New(Config{ShopID: 1, Secret: "s", SignatureKey: "uniq"})
	if err != nil {
		t.Fatal(err)
	}
	// sha1("uniq" + "uuid-1" + "succeeded")
	sum := sha1.Sum([]byte("uniq" + "uuid-1" + "succeeded"))
	valid := hex.EncodeToString(sum[:])

	ok, checked := c.VerifyCallbackSignature(Callback{
		OctoPaymentUUID: "uuid-1", Status: "succeeded", Signature: strings.ToUpper(valid),
	})
	if !checked || !ok {
		t.Errorf("to'g'ri imzo qabul qilinishi kerak edi (ok=%v checked=%v)", ok, checked)
	}

	ok, checked = c.VerifyCallbackSignature(Callback{
		OctoPaymentUUID: "uuid-1", Status: "succeeded", Signature: "deadbeef",
	})
	if !checked || ok {
		t.Errorf("soxta imzo rad etilishi kerak edi (ok=%v checked=%v)", ok, checked)
	}
}

// Kalit sozlanmagan bo'lsa — "tekshirilmadi" deb aniq aytiladi.
// Chaqiruvchi shunda holatni Octo API'sidan so'rashga TAYANADI.
func TestVerifyCallbackSignatureNotConfigured(t *testing.T) {
	c, _ := New(Config{ShopID: 1, Secret: "s"})
	ok, checked := c.VerifyCallbackSignature(Callback{
		OctoPaymentUUID: "uuid-1", Status: "succeeded", Signature: "whatever",
	})
	if checked || ok {
		t.Errorf("kalitsiz tekshiruv bo'lmasligi kerak (ok=%v checked=%v)", ok, checked)
	}
}

func TestNewRequiresCredentials(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Error("kalitsiz klient yaratilmasligi kerak")
	}
	if _, err := New(Config{ShopID: 1}); err == nil {
		t.Error("secret'siz klient yaratilmasligi kerak")
	}
}
