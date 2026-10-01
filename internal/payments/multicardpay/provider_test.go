package multicardpay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"chustapp/internal/payments"
	"chustapp/internal/payments/mcconnect"
	"chustapp/internal/secretstore"
)

const encKey = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"

// stand — Multicard'ning kerakli nuqtalari.
type stand struct {
	srv        *httptest.Server
	lastPath   string
	lastMethod string
	lastBody   map[string]any
	paymentRes string
	fail       int
	failCode   string
}

func newStand(t *testing.T) *stand {
	t.Helper()
	s := &stand{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"tok","role":"dev","expiry":"` +
			time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05") + `"}`))
	})
	record := func(w http.ResponseWriter, r *http.Request) {
		s.lastPath, s.lastMethod = r.URL.Path, r.Method
		s.lastBody = nil
		//nolint:errcheck // soxta server
		json.NewDecoder(r.Body).Decode(&s.lastBody)
		if s.fail != 0 {
			w.WriteHeader(s.fail)
			code := s.failCode
			if code == "" {
				code = "ERROR_FIELDS"
			}
			//nolint:errcheck // soxta server
			w.Write([]byte(`{"success":false,"error":{"code":"` + code + `","details":"xato"}}`))
			return
		}
		body := `{"success":true,"data":{"uuid":"mc-uuid-1",
		  "checkout_url":"https://dev-app.rhmt.uz/invoice/mc-uuid-1"}}`
		if strings.HasPrefix(r.URL.Path, "/payment/") && r.Method == http.MethodGet {
			body = s.paymentRes
			if body == "" {
				body = `{"success":true,"data":{"uuid":"mc-uuid-1","status":"success",
				  "store_invoice_id":"pay-1","payment_amount":4500000}}`
			}
		}
		//nolint:errcheck // soxta server
		w.Write([]byte(body))
	}
	mux.HandleFunc("POST /payment/invoice", record)
	mux.HandleFunc("POST /payment", func(w http.ResponseWriter, r *http.Request) {
		s.lastPath, s.lastMethod = r.URL.Path, r.Method
		s.lastBody = nil
		//nolint:errcheck // soxta server
		json.NewDecoder(r.Body).Decode(&s.lastBody)
		if s.fail != 0 {
			w.WriteHeader(s.fail)
			//nolint:errcheck // soxta server
			w.Write([]byte(`{"success":false,"error":{"code":"ERROR_FIELDS","details":"xato"}}`))
			return
		}
		// Jonli javobda `deeplink` ilovaga olib boradi.
		//nolint:errcheck // soxta server
		w.Write([]byte(`{"success":true,"data":{"uuid":"mc-direct-1",
		  "checkout_url":"https://dev-app.rhmt.uz/invoice/mc-direct-1",
		  "deeplink":"https://payme.uz/checkout/mc-direct-1"}}`))
	})
	mux.HandleFunc("GET /payment/{uuid}", record)
	// Barcha DELETE yo'llari yozib olinadi: sinov QAYSI yo'l chaqirilganini tekshiradi.
	mux.HandleFunc("DELETE /payment/", record)
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

// provider — ulangan restoran bilan.
func (s *stand) provider(t *testing.T, creds mcconnect.Credentials) *Provider {
	t.Helper()
	box, err := secretstore.New(encKey)
	if err != nil {
		t.Fatal(err)
	}
	if creds.AppID != "" && creds.SecretEnc == "" {
		enc, serr := box.Seal("restoran-siri")
		if serr != nil {
			t.Fatal(serr)
		}
		creds.SecretEnc = enc
	}
	reg := mcconnect.NewRegistry(
		func(context.Context, string) (mcconnect.Credentials, error) { return creds, nil },
		box, s.srv.URL, nil)
	return New(reg)
}

func connected() mcconnect.Credentials {
	return mcconnect.Credentials{
		AppID: "bookcafe", StoreID: "6",
		TaxMXIK: "10204001001000000", TaxPackageCode: "1500169",
	}
}

func createReq() payments.CreateRequest {
	return payments.CreateRequest{
		PaymentID: "pay-1", AmountTiyin: 4_500_000, RestaurantID: "rest-a",
		Description: "Buyurtma 300726-1", Hold: true,
		NotifyURL: "https://ondex.uz/payments/multicard/callback",
		ReturnURL: "https://ondex.uz/ok", TTLMinutes: 15, Language: "uz",
		CustomerPhone: "+998901234567",
		Items: []payments.Item{
			{Name: "Lavash", Qty: 2, UnitPriceTiyin: 1_950_000, TotalTiyin: 3_900_000},
			{Name: "Yetkazib berish", Qty: 1, UnitPriceTiyin: 600_000, TotalTiyin: 600_000},
		},
	}
}

func TestCreateSendsInvoiceWithFiscalLines(t *testing.T) {
	s := newStand(t)
	p := s.provider(t, connected())

	res, err := p.Create(context.Background(), createReq())
	if err != nil {
		t.Fatal(err)
	}
	if s.lastPath != "/payment/invoice" {
		t.Fatalf("yo'l: %s", s.lastPath)
	}
	if s.lastBody["store_id"] != "6" || s.lastBody["invoice_id"] != "pay-1" {
		t.Errorf("tana: %v", s.lastBody)
	}
	// Telefon shlyuz formatiga o'girilgan.
	if s.lastBody["sms"] != "998901234567" {
		t.Errorf("sms: %v", s.lastBody["sms"])
	}
	// TTL daqiqadan soniyaga.
	if s.lastBody["ttl"] != float64(900) {
		t.Errorf("ttl: %v", s.lastBody["ttl"])
	}
	// Fiskal satrlar: ikkita, har birida kassaning IKPU standarti.
	items, _ := s.lastBody["ofd"].([]any)
	if len(items) != 2 {
		t.Fatalf("ofd: %v", s.lastBody["ofd"])
	}
	first, _ := items[0].(map[string]any)
	if first["name"] != "Lavash" || first["mxik"] != "10204001001000000" ||
		first["package_code"] != "1500169" {
		t.Errorf("birinchi satr: %v", first)
	}

	// ⚠️ Holat `pending`: Multicard'da hold yo'q, pul hali olinmagan.
	if res.Status != payments.StatusPending {
		t.Errorf("holat: %s", res.Status)
	}
	if res.ProviderPaymentID != "mc-uuid-1" || res.PayURL == "" {
		t.Errorf("natija: %+v", res)
	}
}

// ┌─ MIJOZ AYNAN BITTA ILOVANI TANLASA ───────────────────────────────┐
// Payme/Click/Uzum tanlanganda universal sahifa EMAS, to'g'ridan-
// to'g'ri o'sha ilovaga havola berilishi kerak — aks holda mijoz
// ikki marta tanlashga majbur bo'ladi.
// └───────────────────────────────────────────────────────────────────┘
func TestCreateWithChannelUsesDirectPayment(t *testing.T) {
	for _, ps := range []string{"payme", "click", "uzum"} {
		s := newStand(t)
		p := s.provider(t, connected())

		req := createReq()
		req.Channel = ps
		res, err := p.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("%s: %v", ps, err)
		}
		if s.lastPath != "/payment" {
			t.Errorf("%s: yo'l %s (universal invoys emas, /payment kutilgan)", ps, s.lastPath)
		}
		if s.lastBody["payment_system"] != ps {
			t.Errorf("%s: payment_system %v", ps, s.lastBody["payment_system"])
		}
		if s.lastBody["store_id"] != "6" || s.lastBody["invoice_id"] != "pay-1" {
			t.Errorf("%s: tana %v", ps, s.lastBody)
		}
		// Fiskal chek bu yo'lda ham yuboriladi.
		if items, _ := s.lastBody["ofd"].([]any); len(items) != 2 {
			t.Errorf("%s: ofd %v", ps, s.lastBody["ofd"])
		}
		// Havola — ilovaga (deeplink), umumiy sahifaga emas.
		if res.PayURL != "https://payme.uz/checkout/mc-direct-1" {
			t.Errorf("%s: PayURL %q", ps, res.PayURL)
		}
		if res.ProviderPaymentID != "mc-direct-1" {
			t.Errorf("%s: uuid %q", ps, res.ProviderPaymentID)
		}
	}
}

// Kanalsiz — universal sahifa (karta + hammasi).
func TestCreateWithoutChannelUsesInvoice(t *testing.T) {
	s := newStand(t)
	p := s.provider(t, connected())

	res, err := p.Create(context.Background(), createReq())
	if err != nil {
		t.Fatal(err)
	}
	if s.lastPath != "/payment/invoice" {
		t.Errorf("yo'l: %s", s.lastPath)
	}
	if res.PayURL != "https://dev-app.rhmt.uz/invoice/mc-uuid-1" {
		t.Errorf("PayURL: %q", res.PayURL)
	}
}

// Noma'lum xizmat JIMGINA universal sahifaga tushib ketmasligi kerak.
func TestCreateRejectsUnknownChannel(t *testing.T) {
	s := newStand(t)
	p := s.provider(t, connected())

	req := createReq()
	req.Channel = "paynet"
	if _, err := p.Create(context.Background(), req); err == nil {
		t.Fatal("noma'lum xizmat qabul qilindi")
	}
	if s.lastPath != "" {
		t.Error("shlyuzga so'rov ketdi")
	}
}

// Ulanmagan restoran — to'lov yaratilmaydi va ANIQ xato beriladi.
func TestCreateFailsWhenNotConnected(t *testing.T) {
	s := newStand(t)
	p := s.provider(t, mcconnect.Credentials{})

	_, err := p.Create(context.Background(), createReq())
	if !errors.Is(err, ErrNotConnected) {
		t.Fatalf("ErrNotConnected kutildi: %v", err)
	}
	if s.lastPath != "" {
		t.Error("shlyuzga so'rov ketdi")
	}
}

// Kassada IKPU standartlari bo'lmasa to'lov yaratilmaydi: fiskal
// cheksiz to'lov qonunga zid va shlyuz ham rad etadi.
func TestCreateFailsWithoutFiscalDefaults(t *testing.T) {
	s := newStand(t)
	creds := connected()
	creds.TaxMXIK = ""
	p := s.provider(t, creds)

	_, err := p.Create(context.Background(), createReq())
	if !errors.Is(err, ErrNoFiscalDefaults) {
		t.Fatalf("ErrNoFiscalDefaults kutildi: %v", err)
	}
	if s.lastPath != "" {
		t.Error("shlyuzga so'rov ketdi")
	}
}

func TestStatusMapsGatewayStatuses(t *testing.T) {
	cases := map[string]payments.Status{
		"success":   payments.StatusPaid,
		"revert":    payments.StatusRefunded,
		"cancelled": payments.StatusCanceled,
		"error":     payments.StatusFailed,
		// ⚠️ Pul BO'LMAGAN holatlar "to'landi" bo'lib ketmasligi kerak.
		"draft":    payments.StatusPending,
		"hold":     payments.StatusPending,
		"billing":  payments.StatusPending,
		"progress": payments.StatusPending,
		"":         payments.StatusPending,
		"nomalum":  payments.StatusPending,
	}
	for in, want := range cases {
		if got := MapStatus(in); got != want {
			t.Errorf("%q: %s kutildi, %s keldi", in, want, got)
		}
	}
}

func TestStatusReadsAmountOnlyWhenMoneyMoved(t *testing.T) {
	s := newStand(t)
	p := s.provider(t, connected())
	ctx := context.Background()

	st, err := p.Status(ctx, "rest-a", "mc-uuid-1")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != payments.StatusPaid || st.PaidAmountTiyin != 4_500_000 {
		t.Errorf("%+v", st)
	}

	// To'lanmagan holatda summa 0 bo'lishi kerak: aks holda
	// `payments.Service` uni haqiqiy to'lov deb hisoblardi.
	s.paymentRes = `{"success":true,"data":{"uuid":"mc-uuid-1","status":"draft","payment_amount":4500000}}`
	st, err = p.Status(ctx, "rest-a", "mc-uuid-1")
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != payments.StatusPending || st.PaidAmountTiyin != 0 {
		t.Errorf("%+v", st)
	}
}

func TestCaptureIsNotSupported(t *testing.T) {
	s := newStand(t)
	p := s.provider(t, connected())
	if err := p.Capture(context.Background(), "rest-a", "mc-uuid-1", 1); !errors.Is(err, payments.ErrNotSupported) {
		t.Errorf("ErrNotSupported kutildi: %v", err)
	}
	if s.lastPath != "" {
		t.Error("shlyuzga so'rov ketdi")
	}
}

func TestCancelAndRefund(t *testing.T) {
	s := newStand(t)
	p := s.provider(t, connected())
	ctx := context.Background()

	if err := p.Cancel(ctx, "rest-a", "mc-uuid-1"); err != nil {
		t.Fatal(err)
	}
	if s.lastMethod != http.MethodDelete || s.lastPath != "/payment/invoice/mc-uuid-1" {
		t.Errorf("bekor qilish: %s %s", s.lastMethod, s.lastPath)
	}

	if err := p.Refund(ctx, "rest-a", "mc-uuid-1", "ref-1", 4_500_000); err != nil {
		t.Fatal(err)
	}
	if s.lastPath != "/payment/mc-uuid-1/partial" {
		t.Errorf("qaytarish yo'li: %s", s.lastPath)
	}
	if s.lastBody["refund_amount"] != float64(4_500_000) {
		t.Errorf("summa: %v", s.lastBody["refund_amount"])
	}
	// Qaytarishda ham fiskal chek yuboriladi.
	if items, _ := s.lastBody["ofd"].([]any); len(items) != 1 {
		t.Errorf("ofd: %v", s.lastBody["ofd"])
	}
}

// Shlyuz xatosi `payments` qatlamining turlariga o'girilishi kerak:
// chaqiruvchi "qayta urinsa bo'ladi" ni ajrata olishi uchun.
func TestGatewayErrorsAreTranslated(t *testing.T) {
	s := newStand(t)
	p := s.provider(t, connected())
	ctx := context.Background()

	s.fail, s.failCode = http.StatusNotFound, "ERROR_NOT_FOUND"
	if _, err := p.Status(ctx, "rest-a", "yo-q"); !errors.Is(err, payments.ErrNotFound) {
		t.Errorf("ErrNotFound kutildi: %v", err)
	}

	s.fail, s.failCode = http.StatusBadRequest, "ERROR_FIELDS"
	_, err := p.Create(ctx, createReq())
	var provErr *payments.ProviderError
	if !errors.As(err, &provErr) {
		t.Fatalf("ProviderError kutildi: %v", err)
	}
	if provErr.Code != http.StatusBadRequest {
		t.Errorf("kod: %d", provErr.Code)
	}
}

func TestPhoneNormalisation(t *testing.T) {
	cases := map[string]string{
		"+998901234567":     "998901234567",
		"998901234567":      "998901234567",
		"+998 90 123 45 67": "998901234567",
		"901234567":         "", // kod yo'q — yuborilmaydi
		"":                  "",
		"+7 999 123 45 67":  "", // boshqa davlat
	}
	for in, want := range cases {
		if got := phone(in); got != want {
			t.Errorf("phone(%q): %q kutildi, %q keldi", in, want, got)
		}
	}
}

func TestLanguageFallsBackToUzbek(t *testing.T) {
	for in, want := range map[string]string{
		"uz": "uz", "ru": "ru", "en": "en", "": "uz", "fr": "uz", "RU": "ru",
	} {
		if got := language(in); got != want {
			t.Errorf("language(%q): %q kutildi, %q keldi", in, want, got)
		}
	}
}
