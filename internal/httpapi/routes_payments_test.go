package httpapi

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/payments"
	"chustapp/internal/payments/octo"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// `POST /payments/octo/callback` — HTTP DARAJASIDAGI testlar.
//
// ┌─ NEGA BU FAYL KERAK EDI (100-band) ───────────────────────────────┐
// `internal/payments` dagi sinovlar juda kuchli, LEKIN ular
// `SignatureChecked`/`SignatureValid` qiymatlarini O'ZLARI yozadi:
//
//	svc.HandleCallback(ctx, CallbackData{SignatureValid: true, ...})
//
// 32-banddagi xato esa aynan o'sha qiymatlarni HISOBLAYDIGAN qatlamda
// edi — imzo `octo_payment_UUID` ustidan tekshirilib, to'lov esa
// `shop_transaction_id` bo'yicha topilardi va ikkalasi bir to'lovga
// tegishli ekani solishtirilmasdi. Service testlari bu bo'g'inni
// MOCKLAB tashlagani uchun xatoni ko'ra olmasdi.
//
// Ustiga bu endpoint `s.auth(...)` bilan ro'yxatga OLINMAGAN (u
// to'g'ri — chaqiruvchi Octo serveri), ya'ni `authz_matrix_test.go`
// ham uni topmaydi: u `mux.HandleFunc("... ", s.auth(` naqshini
// qidiradi. Tizimdagi yagona autentifikatsiyasiz PUL endpointi ikkala
// to'plamning ham ko'rish maydonidan tashqarida qolgan edi.
// └───────────────────────────────────────────────────────────────────┘

const (
	payTestShopID    = 42911
	payTestSecret    = "test-secret"
	payTestSigKey    = "test-unique-key"
	payTestRestID    = "rest-pay"
	payTestProductID = "prod-pay"
)

// octoStub — Octo serverining o'rnini bosadi.
//
// `prepare_payment` har chaqiruvda BOSHQA UUID qaytaradi — hujum
// testida ikki to'lovning UUID'lari farq qilishi SHART.
type octoStub struct {
	n int
	// statusFails — true bo'lsa `/status` xato qaytaradi. Shunda
	// yagona dalil imzo bo'lib qoladi va bog'lanish tekshiruvi
	// haqiqatan sinaladi.
	statusFails bool
}

func newOctoStubServer(t *testing.T, stub *octoStub) *octo.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)

		code, resp := 200, `{"error":0,"data":{}}`
		switch r.URL.Path {
		case "/prepare_payment":
			stub.n++
			id, _ := body["shop_transaction_id"].(string)
			resp = `{"error":0,"data":{
				"shop_transaction_id":"` + id + `",
				"octo_payment_UUID":"uuid-` + strconv.Itoa(stub.n) + `",
				"status":"created",
				"octo_pay_url":"https://pay.example/` + id + `",
				"total_sum":10.00}}`
		case "/status":
			if stub.statusFails {
				code, resp = 500, `{"error":1,"errMessage":"status API olik"}`
			} else {
				resp = `{"error":0,"data":{"status":"created","total_sum":10.00}}`
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(srv.Close)
	c, err := octo.New(octo.Config{
		ShopID: payTestShopID, Secret: payTestSecret, SignatureKey: payTestSigKey,
		Test: true, BaseURL: srv.URL, HTTPClient: srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// signCallback — Octo qanday imzolasa, xuddi shunday:
// sha1(unique_key + octo_payment_UUID + status).
func signCallback(uuid, status string) string {
	sum := sha1.Sum([]byte(payTestSigKey + uuid + status))
	return hex.EncodeToString(sum[:])
}

type payFixture struct {
	h         http.Handler
	svc       *payments.Service
	repo      payments.Repository
	orderRepo orders.Repository
	jwt       string
	stub      *octoStub
}

func paymentsServer(t *testing.T, statusFails bool) *payFixture {
	t.Helper()
	ctx := context.Background()

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	customer := &users.User{
		ID: "cust-pay", Phone: "+998901234567",
		Role: users.RoleCustomer, PhoneVerified: true,
		Address: users.AddressDetails{
			Lat: 41.0004, Lng: 71.2394, Text: "Chust markazi",
		},
	}
	if err := userRepo.Create(ctx, customer); err != nil {
		t.Fatal(err)
	}
	jwt, err := tokens.Issue(customer)
	if err != nil {
		t.Fatal(err)
	}

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{{ID: payTestRestID, Name: "To'lov kafe", Open: true}},
		[]catalog.Product{{
			ID: payTestProductID, RestaurantID: payTestRestID, Name: "Osh",
			PriceTiyin: 1000, Available: true,
		}},
	)
	orderRepo := storage.NewMemoryOrderRepo()
	orderSvc := orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo())

	stub := &octoStub{statusFails: statusFails}
	client := newOctoStubServer(t, stub)

	payRepo := storage.NewMemoryPaymentRepo()
	paySvc, err := payments.NewService(payRepo, client, orderSvc, NewID, payments.Options{
		NotifyURL: "https://api.example/payments/octo/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	orderSvc.WithPayments(paySvc)

	deps := Deps{
		UserRepo:    userRepo,
		OrderRepo:   orderRepo,
		CatalogRepo: catalogRepo,
		Tokens:      tokens,
		CatalogSvc:  catalog.NewService(catalogRepo),
		OrderSvc:    orderSvc,
		Payments:    paySvc,
		OctoClient:  client,
		DevMode:     true,
	}
	return &payFixture{
		h: New(deps).Routes(nil), svc: paySvc, repo: payRepo,
		orderRepo: orderRepo, jwt: jwt, stub: stub,
	}
}

// startCardPayment — karta buyurtmasi yaratib, unga to'lov ochadi.
func (f *payFixture) startCardPayment(t *testing.T, key string) *payments.Payment {
	t.Helper()
	body := `{"items":[{"product_id":"` + payTestProductID + `","qty":1}],` +
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
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("to'lov boshlanmadi: %d — %s", w.Code, w.Body.String())
	}
	list, err := f.repo.ListByOrder(context.Background(), o.ID)
	if err != nil || len(list) == 0 {
		t.Fatalf("to'lov yozuvi topilmadi: %v", err)
	}
	return list[len(list)-1]
}

// postCallback — callback endpointiga xom JSON yuboradi (auth yo'q).
func (f *payFixture) postCallback(t *testing.T, cb octo.Callback) int {
	t.Helper()
	raw, err := json.Marshal(cb)
	if err != nil {
		t.Fatal(err)
	}
	w := do(t, f.h, "POST", "/payments/octo/callback", "", string(raw))
	return w.Code
}

// ═══════════════════════════════════════════════════════════════════
// ★★ 32-BAND: IMZO BOSHQA TO'LOVGA ISHLATILMASIN
// ═══════════════════════════════════════════════════════════════════

func TestCallbackSignatureIsBoundToPayment(t *testing.T) {
	// Status API o'lik — yagona dalil imzo bo'lsin.
	f := paymentsServer(t, true)
	ctx := context.Background()

	attacker := f.startCardPayment(t, "k-attacker")
	target := f.startCardPayment(t, "k-target")

	if attacker.ProviderPaymentID == target.ProviderPaymentID {
		t.Fatalf("test sozlamasi buzuq: ikki to'lov bir xil UUID oldi (%s)",
			target.ProviderPaymentID)
	}

	// ★ HUJUM: nishonning shop_transaction_id si + hujumchining
	// UUID va YAROQLI imzosi.
	code := f.postCallback(t, octo.Callback{
		ShopTransactionID: target.ID,
		OctoPaymentUUID:   attacker.ProviderPaymentID,
		Status:            "succeeded",
		Signature:         signCallback(attacker.ProviderPaymentID, "succeeded"),
		TotalSum:          "10.00",
	})
	// Endpoint Octo'ga har doim 200 qaytaradi (5xx bo'lsa u xabarni
	// cheksiz takrorlaydi) — muhimi HOLAT o'zgarmagani.
	if code != http.StatusOK {
		t.Fatalf("kutilgan 200, olindi %d", code)
	}

	got, err := f.repo.GetByID(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status == payments.StatusPaid || got.Status == payments.StatusHeld {
		t.Fatalf("BEGONA IMZO BILAN TO'LOV TASDIQLANDI: %s — "+
			"pul to'lamasdan buyurtma berish mumkin", got.Status)
	}

	// Buyurtma ham harakatlanmasligi kerak.
	o, err := f.orderRepo.GetByID(ctx, got.OrderID)
	if err != nil {
		t.Fatal(err)
	}
	if !o.AwaitingPayment() {
		t.Fatalf("buyurtma to'lov kutish holatidan chiqdi: %s/%s",
			o.Status, o.PaymentState)
	}
}

// O'Z imzosi bilan kelgan halol callback ISHLASHDA DAVOM etishi kerak
// — tuzatish to'g'ri oqimni buzmaganini tasdiqlaydi.
func TestCallbackWithMatchingSignatureAccepted(t *testing.T) {
	f := paymentsServer(t, true)
	ctx := context.Background()

	p := f.startCardPayment(t, "k-ok")

	code := f.postCallback(t, octo.Callback{
		ShopTransactionID: p.ID,
		OctoPaymentUUID:   p.ProviderPaymentID,
		Status:            "waiting_for_capture",
		Signature:         signCallback(p.ProviderPaymentID, "waiting_for_capture"),
		TotalSum:          "10.00",
	})
	if code != http.StatusOK {
		t.Fatalf("kutilgan 200, olindi %d", code)
	}

	got, err := f.repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != payments.StatusHeld {
		t.Fatalf("halol callback qabul qilinmadi: %s", got.Status)
	}
}

// Soxta imzo — holat o'zgarmaydi.
func TestCallbackWithForgedSignatureRejected(t *testing.T) {
	f := paymentsServer(t, true)
	ctx := context.Background()

	p := f.startCardPayment(t, "k-forged")

	f.postCallback(t, octo.Callback{
		ShopTransactionID: p.ID,
		OctoPaymentUUID:   p.ProviderPaymentID,
		Status:            "succeeded",
		Signature:         "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		TotalSum:          "10.00",
	})

	got, _ := f.repo.GetByID(ctx, p.ID)
	if got.Status == payments.StatusPaid || got.Status == payments.StatusHeld {
		t.Fatalf("SOXTA imzo qabul qilindi: %s", got.Status)
	}
}

// Mavjud bo'lmagan to'lov ID'si — panika ham, 5xx ham bo'lmasin.
func TestCallbackUnknownPaymentIsHarmless(t *testing.T) {
	f := paymentsServer(t, true)

	code := f.postCallback(t, octo.Callback{
		ShopTransactionID: "yo-q-tolov",
		OctoPaymentUUID:   "yo-q-uuid",
		Status:            "succeeded",
		Signature:         signCallback("yo-q-uuid", "succeeded"),
	})
	if code != http.StatusOK {
		t.Fatalf("kutilgan 200, olindi %d", code)
	}
}

// Endpoint AUTENTIFIKATSIYASIZ ochiq bo'lishi KERAK (chaqiruvchi —
// Octo serveri). Bu testning maqsadi — kimdir uni tasodifan `s.auth`
// ostiga olib qo'ysa, Octo callback'lari jimgina yo'qolishini
// oldindan ushlash.
func TestCallbackEndpointStaysPublic(t *testing.T) {
	f := paymentsServer(t, true)
	code := f.postCallback(t, octo.Callback{
		ShopTransactionID: "x", OctoPaymentUUID: "y", Status: "created",
	})
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		t.Fatalf("callback endpointi auth talab qilyapti (%d) — "+
			"Octo xabarlari yo'qoladi", code)
	}
}
