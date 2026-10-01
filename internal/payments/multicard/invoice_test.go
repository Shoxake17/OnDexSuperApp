package multicard

import (
	"context"
	"crypto/sha1" //nolint:gosec // imzo algoritmi shlyuzniki
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Javob SHAKLI haqiqiy sinov stendidan (dev-mesh.multicard.uz,
// 2026-09-30 da o'lchangan): jonli invoys yaratilib, javob ko'chirildi.

// invoiceServer — `/auth` + to'lov nuqtalari.
type invoiceServer struct {
	lastPath   string
	lastMethod string
	lastBody   map[string]any
	status     int
	body       string
}

func (f *invoiceServer) start(t *testing.T) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"tok-123","role":"dev","expiry":"` +
			time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05") + `"}`))
	})
	record := func(w http.ResponseWriter, r *http.Request) {
		f.lastPath, f.lastMethod = r.URL.Path, r.Method
		f.lastBody = nil
		//nolint:errcheck // soxta server
		json.NewDecoder(r.Body).Decode(&f.lastBody)
		if got := r.Header.Get("Authorization"); got != "Bearer tok-123" {
			t.Errorf("Authorization: %q", got)
		}
		if f.status != 0 {
			w.WriteHeader(f.status)
		}
		body := f.body
		if body == "" {
			body = `{"success":true,"data":{"uuid":"2749f775-4798-47fd-b98a-9861d01d5ecd",
			  "checkout_url":"https://dev-app.rhmt.uz/invoice/2749f775",
			  "short_link":"https://dev-app.rhmt.uz/invoice/2749f775",
			  "deeplink":"https://multicard.app/payments/checkout/6?params=%7B%7D"}}`
		}
		//nolint:errcheck // soxta server
		w.Write([]byte(body))
	}
	mux.HandleFunc("POST /payment/invoice", record)
	mux.HandleFunc("GET /payment/{uuid}", record)
	// Barcha DELETE yo'llari yozib olinadi (invoysni bekor qilish, to'lovni
	// qaytarish): sinov aynan QAYSI yo'l chaqirilganini tekshiradi.
	mux.HandleFunc("DELETE /payment/", record)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL, ApplicationID: "rhmt_test", Secret: "sir"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func okOFD(total int64) []OFDItem {
	return []OFDItem{{
		Name: "Lavash", Qty: 2, Price: total / 2, Total: total,
		MXIK: "10204001001000000", PackageCode: "1500169",
	}}
}

func okInvoice(total int64) InvoiceRequest {
	return InvoiceRequest{
		StoreID: "6", AmountTiyin: total, InvoiceID: "ord-1",
		CallbackURL: "https://ondex.uz/cb", OFD: okOFD(total),
		Lang: "uz", TTLSeconds: 900,
	}
}

func TestCreateInvoiceSendsExpectedBody(t *testing.T) {
	f := &invoiceServer{}
	c := f.start(t)

	inv, err := c.CreateInvoice(context.Background(), okInvoice(4_500_000))
	if err != nil {
		t.Fatal(err)
	}
	if f.lastPath != "/payment/invoice" || f.lastMethod != http.MethodPost {
		t.Errorf("so'rov: %s %s", f.lastMethod, f.lastPath)
	}
	if f.lastBody["store_id"] != "6" || f.lastBody["invoice_id"] != "ord-1" {
		t.Errorf("tana: %v", f.lastBody)
	}
	if f.lastBody["amount"] != float64(4_500_000) {
		t.Errorf("summa: %v", f.lastBody["amount"])
	}
	// `ofd` YUBORILISHI shart — usiz shlyuz rad etadi.
	items, _ := f.lastBody["ofd"].([]any)
	if len(items) != 1 {
		t.Fatalf("ofd: %v", f.lastBody["ofd"])
	}
	// Bo'sh ixtiyoriy maydonlar umuman yuborilmaydi.
	if _, ok := f.lastBody["return_url"]; ok {
		t.Error("bo'sh return_url yuborildi")
	}
	if inv.PayURL() != "https://dev-app.rhmt.uz/invoice/2749f775" {
		t.Errorf("PayURL: %q", inv.PayURL())
	}
}

// ┌─ FISKAL CHEK SUMMASI TO'LOV SUMMASIGA TENG BO'LISHI SHART ────────┐
// Shlyuz buni tekshirmaydi. Farq qolsa soliq idorasidagi chek haqiqiy
// to'lovdan boshqa bo'ladi va buni keyin tuzatib bo'lmaydi.
// └───────────────────────────────────────────────────────────────────┘
func TestCreateInvoiceRejectsOFDSumMismatch(t *testing.T) {
	f := &invoiceServer{}
	c := f.start(t)

	req := okInvoice(4_500_000)
	req.OFD[0].Total = 4_400_000
	_, err := c.CreateInvoice(context.Background(), req)
	if err == nil {
		t.Fatal("mos kelmagan summa qabul qilindi")
	}
	if !strings.Contains(err.Error(), "teng emas") {
		t.Errorf("xato matni: %v", err)
	}
	if f.lastPath != "" {
		t.Error("shlyuzga so'rov ketdi — tekshiruv mahalliy bo'lishi kerak")
	}
}

func TestCreateInvoiceValidatesInput(t *testing.T) {
	f := &invoiceServer{}
	c := f.start(t)

	cases := map[string]func(r *InvoiceRequest){
		"store bo'sh":    func(r *InvoiceRequest) { r.StoreID = "" },
		"summa nol":      func(r *InvoiceRequest) { r.AmountTiyin = 0 },
		"summa manfiy":   func(r *InvoiceRequest) { r.AmountTiyin = -1 },
		"invoice bo'sh":  func(r *InvoiceRequest) { r.InvoiceID = "  " },
		"callback bo'sh": func(r *InvoiceRequest) { r.CallbackURL = "" },
		"ofd bo'sh":      func(r *InvoiceRequest) { r.OFD = nil },
	}
	for name, mutate := range cases {
		req := okInvoice(4_500_000)
		mutate(&req)
		if _, err := c.CreateInvoice(context.Background(), req); err == nil {
			t.Errorf("%s: qabul qilindi", name)
		}
	}
}

func TestPaymentInfoReadsBothAmountAndInvoiceNames(t *testing.T) {
	f := &invoiceServer{}
	c := f.start(t)

	// Jonli stend `store_invoice_id` va `payment_amount` qaytardi.
	f.body = `{"success":true,"data":{"uuid":"5cab985c","status":"cancelled",
	  "store_invoice_id":"4","payment_amount":50000}}`
	info, err := c.Payment(context.Background(), "5cab985c")
	if err != nil {
		t.Fatal(err)
	}
	if info.Invoice() != "4" || info.Amount() != 50000 {
		t.Errorf("%+v", info)
	}

	// Boshqa javoblarda `invoice_id` va `amount` bo'lishi mumkin.
	f.body = `{"success":true,"data":{"uuid":"u2","status":"success",
	  "invoice_id":"ord-9","amount":120000}}`
	info, err = c.Payment(context.Background(), "u2")
	if err != nil {
		t.Fatal(err)
	}
	if info.Invoice() != "ord-9" || info.Amount() != 120000 {
		t.Errorf("%+v", info)
	}
}

func TestCancelAndRefundPaths(t *testing.T) {
	f := &invoiceServer{}
	c := f.start(t)
	ctx := context.Background()

	if err := c.CancelInvoice(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	// To'lanmagan invoys: `/payment/invoice/{uuid}` (to'langan to'lovni qaytarish `/payment/{uuid}` EMAS).
	if f.lastMethod != http.MethodDelete || f.lastPath != "/payment/invoice/u1" {
		t.Errorf("bekor qilish: %s %s", f.lastMethod, f.lastPath)
	}

	if err := c.RefundFull(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	// To'LIQ qaytarish: `/partial` EMAS (sandbox uni to'liq summada ham rad etgan).
	if f.lastMethod != http.MethodDelete || f.lastPath != "/payment/u1" {
		t.Errorf("to'liq qaytarish: %s %s", f.lastMethod, f.lastPath)
	}

	if err := c.RefundPartial(ctx, "u1", 4_500_000, okOFD(4_500_000)); err != nil {
		t.Fatal(err)
	}
	if f.lastPath != "/payment/u1/partial" {
		t.Errorf("qaytarish yo'li: %s", f.lastPath)
	}
	if f.lastBody["refund_amount"] != float64(4_500_000) {
		t.Errorf("qaytarish summasi: %v", f.lastBody["refund_amount"])
	}
	// ⚠️ Qaytarishda ham `ofd` MAJBURIY (eski chek bekor qilinib,
	// yangisi chiqariladi).
	if _, ok := f.lastBody["ofd"]; !ok {
		t.Error("qaytarishda ofd yuborilmadi")
	}
	if err := c.RefundPartial(ctx, "u1", 4_500_000, nil); !errors.Is(err, ErrOFDRequired) {
		t.Errorf("ofd'siz qaytarish: %v", err)
	}
	if err := c.RefundPartial(ctx, "u1", 0, okOFD(1)); err == nil {
		t.Error("nol summa qabul qilindi")
	}
}

// ─── Callback imzosi ───────────────────────────────────────────────────

func TestVerifyCallback(t *testing.T) {
	f := &invoiceServer{}
	c := f.start(t) // secret = "sir"

	// SHA1("u1" + "ord-1" + "4500000" + "sir")
	cb := Callback{UUID: "u1", InvoiceID: "ord-1", Amount: 4_500_000}
	cb.Sign = signFor(cb, "sir")

	if !c.VerifyCallback(cb) {
		t.Fatal("to'g'ri imzo rad etildi")
	}
	// Katta harf bilan yozilgan imzo ham qabul qilinadi.
	up := cb
	up.Sign = strings.ToUpper(cb.Sign)
	if !c.VerifyCallback(up) {
		t.Error("katta harfli imzo rad etildi")
	}

	// ⚠️ Har bir maydonning o'zgarishi imzoni buzishi SHART — aks holda
	// summani o'zgartirib yuborish mumkin bo'lardi.
	bad := map[string]Callback{
		"summa o'zgardi":   {UUID: "u1", InvoiceID: "ord-1", Amount: 1, Sign: cb.Sign},
		"uuid o'zgardi":    {UUID: "u2", InvoiceID: "ord-1", Amount: 4_500_000, Sign: cb.Sign},
		"invoice o'zgardi": {UUID: "u1", InvoiceID: "ord-2", Amount: 4_500_000, Sign: cb.Sign},
		"imzo bo'sh":       {UUID: "u1", InvoiceID: "ord-1", Amount: 4_500_000},
		"imzo axlat":       {UUID: "u1", InvoiceID: "ord-1", Amount: 4_500_000, Sign: "deadbeef"},
		"boshqa sir":       {UUID: "u1", InvoiceID: "ord-1", Amount: 4_500_000, Sign: signFor(cb, "boshqa")},
	}
	for name, c2 := range bad {
		if c.VerifyCallback(c2) {
			t.Errorf("%s: imzo qabul qilindi", name)
		}
	}
}

// signFor — hujjatdagi formula testda MUSTAQIL yozilgan:
// `SHA1(uuid + invoice_id + amount + secret)`. Kod formulani
// o'zgartirsa (masalan maydon tartibi almashsa), test yiqiladi.
func signFor(cb Callback, secret string) string {
	raw := cb.UUID + cb.InvoiceID +
		strconv.FormatInt(cb.Amount.Int64(), 10) + secret
	sum := sha1.Sum([]byte(raw)) //nolint:gosec // algoritmni shlyuz belgilaydi
	return hex.EncodeToString(sum[:])
}

// ┌─ SHLYUZ MUDDATI ILOVANIKIDAN QISQA BO'LISHI SHART ───────────────┐
// Mijoz ilovasining HTTP muddati 20 soniya
// (`packages/ondex_core/lib/src/api_client.dart`). To'lov yaratishda
// shlyuzga eng ko'pi IKKI so'rov ketadi (`/auth` + `/payment/...`),
// ya'ni eng yomon holat 2×DefaultTimeout.
//
// Agar u 20 soniyaga yetsa, ilova serverning javobini KUTMASDAN
// taslim bo'ladi va "internetni tekshiring" deydi — foydalanuvchi
// o'z internetini ayblaydi, server esa aniq sababni aytmoqchi edi.
// Jonli sinovda aynan shunday bo'ldi (ikkalasi ham 20 soniya edi).
// └───────────────────────────────────────────────────────────────────┘
func TestDefaultTimeoutLeavesRoomForMobileClient(t *testing.T) {
	const mobileClientTimeout = 20 * time.Second
	const callsPerPayment = 2

	worst := DefaultTimeout * callsPerPayment
	// Kamida 3 soniya zaxira: tarmoq va server ishlov vaqti ham bor.
	if worst > mobileClientTimeout-3*time.Second {
		t.Errorf("DefaultTimeout=%v → eng yomon holat %v, ilova muddati %v. "+
			"Server javobi ilovaga YETIB BORMAYDI va u \"internetni tekshiring\" "+
			"deydi. DefaultTimeout ni kamaytiring.",
			DefaultTimeout, worst, mobileClientTimeout)
	}
}

// ┌─ SINOV / JONLI MANZILNI AJRATISH ────────────────────────────────┐
// Multicard'da `OCTO_TEST` kabi bayroq yo'q — farq faqat manzilda.
// Noto'g'ri aniqlash ikki tomonlama zarar beradi: production sinov
// manzilida bo'lsa pul olinmaydi, dev jonli manzilda bo'lsa haqiqiy
// pul yechiladi. Ikkalasi ham jimgina sodir bo'ladi.
// └───────────────────────────────────────────────────────────────────┘
func TestIsSandbox(t *testing.T) {
	sandbox := []string{
		"https://dev-mesh.multicard.uz",
		"https://dev-mesh.multicard.uz/",
		"HTTPS://DEV-MESH.MULTICARD.UZ",
		"  https://dev-mesh.multicard.uz  ",
		"http://localhost:8099",
		"", // sozlanmagan — jonli deb hisoblanmasin
	}
	for _, u := range sandbox {
		if !IsSandbox(u) {
			t.Errorf("%q sinov deb topilmadi", u)
		}
	}
	live := []string{
		"https://mesh.multicard.uz",
		"https://mesh.multicard.uz/",
		"http://mesh.multicard.uz",
		"https://mesh.multicard.uz:443",
		"MESH.MULTICARD.UZ",
	}
	for _, u := range live {
		if IsSandbox(u) {
			t.Errorf("%q JONLI manzil, sinov deb topildi", u)
		}
	}
}

func TestNilClientVerifyIsSafe(t *testing.T) {
	var c *Client
	if c.VerifyCallback(Callback{Sign: "x"}) {
		t.Error("nil klient imzoni qabul qildi")
	}
}
