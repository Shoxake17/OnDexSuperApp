package httpapi

import (
	"context"
	"crypto/sha1" //nolint:gosec // imzo algoritmini shlyuz belgilaydi
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/payments"
	"chustapp/internal/payments/mcconnect"
	"chustapp/internal/payments/multicard"
	"chustapp/internal/payments/multicardpay"
	"chustapp/internal/secretstore"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// MULTICARD (RAHMAT) ORQALI KARTA TO'LOVI — HTTP qatlami.
//
// ┌─ NIMA QULFLANADI ─────────────────────────────────────────────────┐
//  1. To'lov RESTORANNING O'Z kaliti bilan yaratiladi va fiskal chek
//     yuboriladi (usiz shlyuz rad etadi);
//  2. callback imzosi O'SHA restoranning siri bilan tekshiriladi —
//     boshqa restoranning siri bilan imzolangan xabar O'TMAYDI;
//  3. imzosi yo'q/soxta xabar buyurtmani o'zgartirmaydi;
//  4. takroriy xabar buyurtmani ikki marta o'zgartirmaydi;
//  5. endpoint ochiq (tokensiz) va har doim 2xx qaytaradi.
// └───────────────────────────────────────────────────────────────────┘

const (
	mcTestRestID    = "rest-mc"
	mcTestProductID = "prod-mc"
	mcTestSecret    = "restoran-siri"
	mcTestAppID     = "bookcafe_app"
	mcEncKey        = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"
)

// mcStand — Multicard shlyuzining soxta nusxasi.
type mcStand struct {
	srv *httptest.Server
	// lastInvoice — oxirgi `POST /payment/invoice` tanasi.
	lastInvoice map[string]any
	// paymentStatus — `GET /payment/{uuid}` qaytaradigan holat.
	paymentStatus string
	// statusFails — status API ishlamaydi (yagona dalil imzo bo'lsin).
	statusFails bool
}

func newMCStand(t *testing.T) *mcStand {
	t.Helper()
	s := &mcStand{paymentStatus: "success"}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"tok","role":"dev","expiry":"` +
			time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05") + `"}`))
	})
	mux.HandleFunc("POST /payment/invoice", func(w http.ResponseWriter, r *http.Request) {
		s.lastInvoice = nil
		//nolint:errcheck // soxta server
		json.NewDecoder(r.Body).Decode(&s.lastInvoice)
		_, _ = w.Write([]byte(`{"success":true,"data":{"uuid":"mc-uuid-1",
		  "checkout_url":"https://dev-app.rhmt.uz/invoice/mc-uuid-1"}}`))
	})
	// To'g'ridan-to'g'ri kanal (Payme/Click/Uzum): `POST /payment`.
	mux.HandleFunc("POST /payment", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{"uuid":"mc-uuid-1",
		  "checkout_url":"https://dev-app.rhmt.uz/invoice/mc-uuid-1"}}`))
	})
	mux.HandleFunc("GET /payment/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		if s.statusFails {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"uuid":"mc-uuid-1","status":"` +
			s.paymentStatus + `","payment_amount":1000}}`))
	})
	mux.HandleFunc("DELETE /payment/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
	})
	mux.HandleFunc("DELETE /payment/{uuid}/partial", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{}}`))
	})
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

type mcFixture struct {
	h         http.Handler
	repo      payments.Repository
	orderRepo orders.Repository
	jwt       string
	stand     *mcStand
	catalog   catalog.Repository
}

func mcPaymentsServer(t *testing.T, statusFails bool) *mcFixture {
	t.Helper()
	ctx := context.Background()

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	customer := &users.User{
		ID: "cust-mc", Phone: "+998901234567",
		Role: users.RoleCustomer, PhoneVerified: true,
		Address: users.AddressDetails{Lat: 41.0004, Lng: 71.2394, Text: "Chust markazi"},
	}
	if err := userRepo.Create(ctx, customer); err != nil {
		t.Fatal(err)
	}
	jwt, err := tokens.Issue(customer)
	if err != nil {
		t.Fatal(err)
	}

	box, err := secretstore.New(mcEncKey)
	if err != nil {
		t.Fatal(err)
	}
	enc, err := box.Seal(mcTestSecret)
	if err != nil {
		t.Fatal(err)
	}
	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{{
			ID: mcTestRestID, Name: "To'lov kafe", Open: true,
			MulticardAppID: mcTestAppID, MulticardSecretEnc: enc,
			MulticardStoreID: "6",
			MulticardTaxMXIK: "10204001001000000", MulticardTaxPackageCode: "1500169",
		}},
		[]catalog.Product{{
			ID: mcTestProductID, RestaurantID: mcTestRestID, Name: "Osh",
			PriceTiyin: 1000, Available: true,
		}},
	)
	orderRepo := storage.NewMemoryOrderRepo()
	orderSvc := orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo())

	stand := newMCStand(t)
	stand.statusFails = statusFails
	reg := mcconnect.NewRegistry(
		func(ctx context.Context, id string) (mcconnect.Credentials, error) {
			rest, rerr := catalogRepo.GetRestaurant(ctx, id)
			if rerr != nil {
				return mcconnect.Credentials{}, rerr
			}
			return mcconnect.Credentials{
				AppID: rest.MulticardAppID, SecretEnc: rest.MulticardSecretEnc,
				StoreID: rest.MulticardStoreID, TaxMXIK: rest.MulticardTaxMXIK,
				TaxPackageCode: rest.MulticardTaxPackageCode,
			}, nil
		}, box, stand.srv.URL, nil)

	payRepo := storage.NewMemoryPaymentRepo()
	paySvc, err := payments.NewService(payRepo, multicardpay.New(reg), orderSvc, NewID,
		payments.Options{NotifyURL: "https://api.example/payments/multicard/callback"})
	if err != nil {
		t.Fatal(err)
	}
	orderSvc.WithPayments(paySvc)

	h := New(Deps{
		UserRepo: userRepo, OrderRepo: orderRepo, CatalogRepo: catalogRepo,
		Tokens: tokens, CatalogSvc: catalog.NewService(catalogRepo),
		OrderSvc: orderSvc, Payments: paySvc, MC: reg, DevMode: true,
	}).Routes(nil)

	return &mcFixture{h: h, repo: payRepo, orderRepo: orderRepo, jwt: jwt, stand: stand, catalog: catalogRepo}
}

// startPayment — karta buyurtmasi yaratib, unga to'lov ochadi.
func (f *mcFixture) startPayment(t *testing.T, key string) *payments.Payment {
	t.Helper()
	body := `{"items":[{"product_id":"` + mcTestProductID + `","qty":1}],` +
		`"payment_method":"card","idempotency_key":"` + key + `"}`
	w := do(t, f.h, "POST", "/orders", f.jwt, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("buyurtma yaratilmadi: %d — %s", w.Code, w.Body.String())
	}
	var o orders.Order
	if err := json.Unmarshal(w.Body.Bytes(), &o); err != nil {
		t.Fatal(err)
	}
	w = do(t, f.h, "POST", "/orders/"+o.ID+"/pay", f.jwt, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("to'lov boshlanmadi: %d — %s", w.Code, w.Body.String())
	}
	list, err := f.repo.ListByOrder(context.Background(), o.ID)
	if err != nil || len(list) == 0 {
		t.Fatalf("to'lov yozuvi topilmadi: %v", err)
	}
	return list[len(list)-1]
}

func mcSign(uuid, invoiceID string, amount int64, secret string) string {
	raw := uuid + invoiceID + strconv.FormatInt(amount, 10) + secret
	sum := sha1.Sum([]byte(raw)) //nolint:gosec // algoritmni shlyuz belgilaydi
	return hex.EncodeToString(sum[:])
}

func (f *mcFixture) postCallback(t *testing.T, cb multicard.Callback) int {
	t.Helper()
	raw, err := json.Marshal(cb)
	if err != nil {
		t.Fatal(err)
	}
	return do(t, f.h, "POST", "/payments/multicard/callback", "", string(raw)).Code
}

func (f *mcFixture) reload(t *testing.T, id string) *payments.Payment {
	t.Helper()
	p, err := f.repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// To'lov RESTORANNING kaliti bilan va FISKAL CHEK bilan yaratiladi.
func TestMulticardPaymentUsesRestaurantStoreAndFiscalLines(t *testing.T) {
	f := mcPaymentsServer(t, true)
	p := f.startPayment(t, "mc-1")

	if p.Status != payments.StatusPending {
		t.Errorf("holat: %s (Multicard'da hold yo'q, pending kutilgan)", p.Status)
	}
	if p.ProviderPaymentID != "mc-uuid-1" || p.PayURL == "" {
		t.Errorf("to'lov: %+v", p)
	}
	inv := f.stand.lastInvoice
	if inv["store_id"] != "6" {
		t.Errorf("kassa: %v", inv["store_id"])
	}
	if inv["invoice_id"] != p.ID {
		t.Errorf("invoice_id: %v (to'lov ID'si %s)", inv["invoice_id"], p.ID)
	}
	// ⚠️ Fiskal chek YUBORILISHI shart.
	items, _ := inv["ofd"].([]any)
	if len(items) == 0 {
		t.Fatalf("ofd bo'sh: %v", inv)
	}
	first, _ := items[0].(map[string]any)
	if first["mxik"] != "10204001001000000" {
		t.Errorf("IKPU kassadan olinmadi: %v", first)
	}
	// Chek summasi to'lov summasiga teng.
	var sum float64
	for _, it := range items {
		m, _ := it.(map[string]any)
		tot, _ := m["total"].(float64)
		sum += tot
	}
	if amount, _ := inv["amount"].(float64); sum != amount {
		t.Errorf("chek yig'indisi %v, to'lov summasi %v", sum, amount)
	}
}

// ┌─ IMZO O'SHA RESTORANNING SIRI BILAN ──────────────────────────────┐
// Boshqa restoranning (yoki o'ylab topilgan) siri bilan imzolangan
// xabar buyurtmani O'ZGARTIRMASLIGI kerak — aks holda istalgan odam
// to'lovni "muvaffaqiyatli" qilib qo'ya olardi.
// └───────────────────────────────────────────────────────────────────┘
func TestMulticardCallbackRejectsForeignSecret(t *testing.T) {
	f := mcPaymentsServer(t, true) // status API o'lik: yagona dalil — imzo
	p := f.startPayment(t, "mc-2")

	cb := multicard.Callback{
		UUID: "mc-uuid-1", InvoiceID: p.ID, Amount: multicard.Tiyin(p.AmountTiyin),
		Status: "success",
		Sign:   mcSign("mc-uuid-1", p.ID, p.AmountTiyin, "boshqa-restoran-siri"),
	}
	if code := f.postCallback(t, cb); code != http.StatusOK {
		t.Fatalf("callback holati: %d", code)
	}
	if got := f.reload(t, p.ID); got.Status == payments.StatusPaid {
		t.Fatal("begona sir bilan imzolangan xabar to'lovni TASDIQLADI")
	}
}

func TestMulticardCallbackAcceptsValidSignature(t *testing.T) {
	f := mcPaymentsServer(t, true)
	p := f.startPayment(t, "mc-3")

	cb := multicard.Callback{
		UUID: "mc-uuid-1", InvoiceID: p.ID, Amount: multicard.Tiyin(p.AmountTiyin),
		Status: "success",
		Sign:   mcSign("mc-uuid-1", p.ID, p.AmountTiyin, mcTestSecret),
	}
	if code := f.postCallback(t, cb); code != http.StatusOK {
		t.Fatalf("callback holati: %d", code)
	}
	got := f.reload(t, p.ID)
	if got.Status != payments.StatusPaid {
		t.Fatalf("to'lov tasdiqlanmadi: %s (%s)", got.Status, got.ReviewReason)
	}
	// Buyurtma endi restoranga ko'rinadi.
	o, err := f.orderRepo.GetByID(context.Background(), got.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if o.PaymentState != orders.PaymentHeld && o.PaymentState != orders.PaymentPaid {
		t.Errorf("buyurtma to'lov holati: %s", o.PaymentState)
	}
}

// Takroriy xabar — IDEMPOTENT (shlyuz besh marta takrorlaydi).
func TestMulticardCallbackIsIdempotent(t *testing.T) {
	f := mcPaymentsServer(t, true)
	p := f.startPayment(t, "mc-4")
	cb := multicard.Callback{
		UUID: "mc-uuid-1", InvoiceID: p.ID, Amount: multicard.Tiyin(p.AmountTiyin),
		Status: "success",
		Sign:   mcSign("mc-uuid-1", p.ID, p.AmountTiyin, mcTestSecret),
	}
	for i := range 3 {
		if code := f.postCallback(t, cb); code != http.StatusOK {
			t.Fatalf("%d-urinish: %d", i+1, code)
		}
	}
	if got := f.reload(t, p.ID); got.Status != payments.StatusPaid {
		t.Errorf("holat: %s", got.Status)
	}
}

// Imzosiz xabar + status API o'lik = DALIL YO'Q: buyurtma
// o'zgarmaydi.
func TestMulticardCallbackWithoutProofChangesNothing(t *testing.T) {
	f := mcPaymentsServer(t, true)
	p := f.startPayment(t, "mc-5")

	cb := multicard.Callback{
		UUID: "mc-uuid-1", InvoiceID: p.ID,
		Amount: multicard.Tiyin(p.AmountTiyin), Status: "success",
	}
	if code := f.postCallback(t, cb); code != http.StatusOK {
		t.Fatalf("callback holati: %d", code)
	}
	if got := f.reload(t, p.ID); got.Status == payments.StatusPaid {
		t.Fatal("imzosiz xabar to'lovni tasdiqladi")
	}
}

// Noma'lum to'lov — zararsiz va baribir 2xx (aks holda shlyuz
// xabarni besh marta takrorlaydi).
func TestMulticardCallbackUnknownPaymentIsHarmless(t *testing.T) {
	f := mcPaymentsServer(t, true)
	cb := multicard.Callback{
		UUID: "yo-q", InvoiceID: "yo-q-to-lov", Amount: 1000, Status: "success",
	}
	if code := f.postCallback(t, cb); code != http.StatusOK {
		t.Errorf("holat: %d", code)
	}
}

// ┌─ "KEYINROQ URINIB KO'RING" FAQAT HAQIQATAN SHUNDAY BO'LSA ───────┐
// Jonli sinovda restoran kassasida IKPU yo'q edi; mijoz esa
// "to'lov tizimiga ulanib bo'lmadi, birozdan keyin urinib ko'ring"
// degan javobni oldi va besh marta qayta bosdi. Qayta urinish
// bunday xatoni hech qachon tuzatmaydi.
//
// Sozlama xatolari 5xx EMAS: shunda matn `httpError` tomonidan
// yashirilmaydi va mijoz NIMA QILISHNI biladi (naqdni tanlash).
// └───────────────────────────────────────────────────────────────────┘
func TestStartPaymentErrorsAreActionable(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		want    int
		contain string
	}{
		{"ulanmagan", multicardpay.ErrNotConnected, http.StatusConflict, "naqd"},
		{"IKPU yo'q", multicardpay.ErrNoFiscalDefaults, http.StatusConflict, "naqd"},
		{"sozlanmagan", payments.ErrNotConfigured, http.StatusConflict, "naqd"},
		{"shlyuz rad etdi", &payments.ProviderError{Code: 400, Message: "xato"},
			http.StatusUnprocessableEntity, "rad etdi"},
		{"shlyuz yiqildi", &payments.ProviderError{Code: 502, Message: "xato"},
			http.StatusBadGateway, "keyinroq"},
		{"tarmoq", errors.New("dial tcp: i/o timeout"),
			http.StatusBadGateway, "keyinroq"},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		writeStartPaymentError(w, c.err)
		if w.Code != c.want {
			t.Errorf("%s: %d kutildi, %d keldi", c.name, c.want, w.Code)
			continue
		}
		body := w.Body.String()
		// 5xx da matn yashiriladi — shuning uchun tekshiruv faqat
		// 4xx javoblar uchun.
		if c.want < 500 && !strings.Contains(body, c.contain) {
			t.Errorf("%s: javobda %q yo'q — %s", c.name, c.contain, body)
		}
	}
}

// Endpoint OCHIQ bo'lishi kerak: shlyuzda bizning tokenimiz yo'q.
func TestMulticardCallbackIsPublic(t *testing.T) {
	f := mcPaymentsServer(t, true)
	w := do(t, f.h, "POST", "/payments/multicard/callback", "", `{}`)
	if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
		t.Errorf("endpoint token talab qildi: %d", w.Code)
	}
}
