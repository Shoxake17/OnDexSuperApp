package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/payments/mcconnect"
	"chustapp/internal/payments/multicard"
	"chustapp/internal/secretstore"
	"chustapp/internal/storage"
	"chustapp/internal/tables"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// MOLIYA BO'LIMI TESTLARI.
//
// Bu yerda tekshirilayotgan narsa pul, shuning uchun uchta savol:
//   1. Kim ko'radi (egalik, rol);
//   2. Raqam TO'G'RI yig'iladimi (`success` / `revert` / boshqa holat);
//   3. Shlyuz yiqilganda nima bo'ladi.
//
// Multicard KLIENTI interfeys emas, konkret tur — shuning uchun
// soxta HTTP server ko'tarib, haqiqiy klientni unga qaratamiz.
// Foydasi: klientning o'zi (JWT keshi, `Tiyin` parsi, sahifalash)
// ham shu yo'lda sinaladi.

// fakeGateway — Multicard'ning `/auth` va `/payment/store/{id}/history`
// nuqtalarini taqlid qiladi.
type fakeGateway struct {
	srv *httptest.Server
	// calls — history so'rovlari soni (sahifalashni tekshirish uchun).
	calls atomic.Int32
	// lastQuery — oxirgi so'rovning query qismi.
	lastQuery atomic.Value
	// fail — true bo'lsa history xato qaytaradi.
	fail atomic.Bool
	// failStatus — xato javobining HTTP holati (0 → 200 + `success:false`,
	// shlyuz aynan shunday qiladi).
	failStatus atomic.Int32
	// failAtOffset — faqat shu `offset` dagi so'rov yiqiladi (0 — o'chiq).
	// Ko'p sahifali reestrda BITTA sahifa yiqilishini sinash uchun.
	failAtOffset atomic.Int32
	// pages — offset -> javob tanasi.
	pages map[int]string
	total int

	// authOK — bu kalitlar bilan `/auth` o'tadimi. `nil` — hammasi
	// o'tadi. Restoranning O'Z kalitlarini tekshirish uchun.
	authOK func(appID, secret string) bool
	// knownStores — `GET /payment/store/{id}` topadigan kassalar.
	// `nil` — hammasi topiladi.
	knownStores map[string]bool
	// storeCalls — kassa so'rovlari soni.
	storeCalls atomic.Int32
	// lastAuth — oxirgi `/auth` da kelgan kalitlar.
	lastAuth atomic.Value
}

func newFakeGateway(t *testing.T, total int, pages map[int]string) *fakeGateway {
	t.Helper()
	g := &fakeGateway{pages: pages, total: total}
	mux := http.NewServeMux()
	// Token muddati kelasi yilda — kesh qayta so'ramaydi.
	exp := time.Now().In(time.FixedZone("UTC+5", 5*60*60)).AddDate(0, 0, 1)
	// ⚠️ `/auth` javobi QOBIQSIZ (`{token,...}`), boshqa nuqtalar esa
	// `{"success":..,"data":..}` ichida beradi — klient aynan shunday
	// o'qiydi va jonli stendda shunday o'lchangan.
	mux.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ApplicationID string `json:"application_id"`
			Secret        string `json:"secret"`
		}
		//nolint:errcheck // soxta server
		json.NewDecoder(r.Body).Decode(&body)
		g.lastAuth.Store([2]string{body.ApplicationID, body.Secret})
		if g.authOK != nil && !g.authOK(body.ApplicationID, body.Secret) {
			// Shlyuz yaroqsiz kalitda 401 beradi.
			w.WriteHeader(http.StatusUnauthorized)
			//nolint:errcheck // soxta server
			w.Write([]byte(`{"error":"Invalid Credentials"}`))
			return
		}
		fmt.Fprintf(w, `{"token":"tok","role":"store","expiry":%q}`,
			exp.Format("2006-01-02 15:04:05"))
	})
	// Kassa ma'lumoti — ulashni tekshirish uchun.
	mux.HandleFunc("GET /payment/store/{id}", func(w http.ResponseWriter, r *http.Request) {
		g.storeCalls.Add(1)
		id := r.PathValue("id")
		if g.knownStores != nil && !g.knownStores[id] {
			w.WriteHeader(http.StatusBadRequest)
			//nolint:errcheck // soxta server
			w.Write([]byte(`{"success":false,"error":{"code":"ERROR_FIELDS",` +
				`"details":"PaymentHistoryQuery: Значение «Касса» неверно."}}`))
			return
		}
		//nolint:errcheck // soxta server
		w.Write([]byte(`{"success":true,"data":{"id":6,"uuid":"a1df872e","title":"Book Cafe kassasi",
		  "merchant":{"name":"Book Cafe MChJ","tin":"307578794"},
		  "merchant_account":{"account_no":"20208000900123456789","mfo":"00491","official_name":"BOOK CAFE MChJ"},
		  "contract":{"num":"POS/8042","fee":{"up":0,"down":1.5}}}}`))
	})
	mux.HandleFunc("GET /payment/store/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		g.calls.Add(1)
		g.lastQuery.Store(r.URL.RawQuery)
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("history Authorization sarlavhasi: %q", got)
		}
		if at := int(g.failAtOffset.Load()); at > 0 {
			if off, _ := strconv.Atoi(r.URL.Query().Get("offset")); off == at {
				w.WriteHeader(int(g.failStatus.Load()))
				//nolint:errcheck // soxta server
				w.Write([]byte(`{"success":false,"error":{"code":"ERROR_UNKNOWN","details":"sahifa yiqildi"}}`))
				return
			}
		}
		if g.fail.Load() {
			status := int(g.failStatus.Load())
			if status == 0 {
				status = http.StatusOK
			}
			w.WriteHeader(status)
			//nolint:errcheck // soxta server
			w.Write([]byte(`{"success":false,"error":{"code":"ERROR_FIELDS","details":"do'kon topilmadi"}}`))
			return
		}
		off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		body, ok := g.pages[off]
		if !ok {
			body = `{"pagination":{"total":` + strconv.Itoa(g.total) + `,"offset":0,"limit":200},"stat":[],"list":[]}`
		}
		//nolint:errcheck // soxta server
		w.Write([]byte(`{"success":true,"data":` + body + `}`))
	})
	g.srv = httptest.NewServer(mux)
	t.Cleanup(g.srv.Close)
	return g
}

func (g *fakeGateway) client(t *testing.T) *multicard.Client {
	t.Helper()
	c, err := multicard.New(multicard.Config{
		BaseURL:       g.srv.URL,
		ApplicationID: "rhmt_test",
		Secret:        "sekret",
		HTTP:          g.srv.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// txnJSON — bitta reestr yozuvi. `payment_amount` ATAYLAB son:
// jonli API `list` da son, `stat` da matn beradi.
func txnJSON(uuid, status, ps, day string, amount, commission int64) string {
	return fmt.Sprintf(
		`{"uuid":%q,"status":%q,"ps":%q,"store_invoice_id":"ORD-1",`+
			`"payment_amount":%d,"commission_amount":%d,`+
			`"card_pan":"860030******5959","payment_time":"%s 12:00:00","refund_time":""}`,
		uuid, status, ps, amount, commission, day)
}

func historyJSON(total int, txns ...string) string {
	return `{"pagination":{"total":` + strconv.Itoa(total) +
		`,"offset":0,"limit":200},"stat":[],"list":[` + strings.Join(txns, ",") + `]}`
}

// testEncKey — sinovda sirlarni shifrlash kaliti (AES-256 uchun 32 bayt).
const testEncKey = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"

func testSecrets(t *testing.T) *secretstore.Box {
	t.Helper()
	b, err := secretstore.New(testEncKey)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// financeServer — restoran A (platforma kalitlari bilan ulangan),
// restoran B (ulanmagan). `g == nil` — shlyuz umuman sozlanmagan.
func financeServer(t *testing.T, g *fakeGateway, storeA string) (http.Handler, map[string]string) {
	t.Helper()
	var mc *multicard.Client
	var baseURL string
	if g != nil {
		mc, baseURL = g.client(t), g.srv.URL
	}
	h, _, jwt := financeServerFull(t, mc, baseURL, catalog.Restaurant{
		ID: testRestA, Name: "Book Cafe", Open: true, MulticardStoreID: storeA,
	})
	return h, jwt
}

// financeServerFull — ulash oqimi uchun: shlyuz manzili, katalog
// omborini qaytaradi (saqlangan qiymatlarni tekshirish uchun) va
// restoran A ni to'liq boshlang'ich holat bilan yaratadi.
func financeServerFull(t *testing.T, mc *multicard.Client, baseURL string,
	restA catalog.Restaurant) (http.Handler, catalog.Repository, map[string]string) {
	t.Helper()
	ctx := context.Background()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	jwt := map[string]string{}
	mk := func(key, id string, role users.Role, entityID, phone string) {
		u := &users.User{ID: id, Phone: phone, Role: role, EntityID: entityID,
			PhoneVerified: true, CreatedAt: time.Now()}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		tok, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		jwt[key] = tok
	}
	mk("a", "u-fa", users.RoleRestaurant, testRestA, "+998900000071")
	mk("b", "u-fb", users.RoleRestaurant, testRestB, "+998900000072")
	mk("admin", "u-fadmin", users.RoleAdmin, "", "+998900000073")
	mk("waiter", "u-fw", users.RoleWaiter, testRestA, "+998900000074")
	mk("courier", "u-fc", users.RoleCourier, "", "+998900000075")
	mk("customer", "u-fcu", users.RoleCustomer, "", "+998900000076")

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{restA, {ID: testRestB, Name: "B", Open: true}},
		nil,
	)
	orderRepo := storage.NewMemoryOrderRepo()
	h := New(Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      orderRepo,
		TableSvc:       tables.NewService(storage.NewMemoryTableRepo()),
		Tokens:         tokens,
		CatalogSvc:     catalog.NewService(catalogRepo),
		OrderSvc:       orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo()),
		Hub:            ws.NewHub(nil),
		MC:             mcconnect.NewRegistry(nil, testSecrets(t), baseURL, mc),
		DevMode:        true,
	}).Routes(nil)
	return h, catalogRepo, jwt
}

const financePathA = "/restaurants/" + testRestA + "/finance"

func getFinance(t *testing.T, h http.Handler, token, query string) financeResponse {
	t.Helper()
	w := do(t, h, "GET", financePathA+query, token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET finance: %d — %s", w.Code, w.Body.String())
	}
	var out financeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// ┌─ EGALIK VA ROL ───────────────────────────────────────────────────┐
// Tushum — moliyaviy ma'lumot. Affitsiant, kuryer, mijoz KO'RMAYDI;
// boshqa restoran xodimi ham ko'rmaydi (`routes_stats.go` qoidasi).
// └───────────────────────────────────────────────────────────────────┘
func TestFinanceAccessControl(t *testing.T) {
	g := newFakeGateway(t, 0, nil)
	h, jwt := financeServer(t, g, "6")

	cases := []struct {
		token string
		want  int
		why   string
	}{
		{"", http.StatusUnauthorized, "tokensiz"},
		{jwt["customer"], http.StatusForbidden, "mijoz"},
		{jwt["courier"], http.StatusForbidden, "kuryer"},
		{jwt["waiter"], http.StatusForbidden, "affitsiant"},
		{jwt["b"], http.StatusForbidden, "boshqa restoran"},
		{jwt["a"], http.StatusOK, "o'z restorani"},
		{jwt["admin"], http.StatusOK, "admin"},
	}
	for _, c := range cases {
		w := do(t, h, "GET", financePathA, c.token, "")
		if w.Code != c.want {
			t.Errorf("%s: %d kutildi, %d keldi — %s", c.why, c.want, w.Code, w.Body.String())
		}
	}
}

// Egalik tekshiruvi NOTO'G'RI so'rovdan ham OLDIN bo'lishi kerak:
// aks holda 400 javobi "bu restoran bor" degan ma'lumotni oshkor
// qiladi (`TestTenancyMatrix` dagi o'sha qoida).
func TestFinanceOwnershipBeforeValidation(t *testing.T) {
	g := newFakeGateway(t, 0, nil)
	h, jwt := financeServer(t, g, "6")

	w := do(t, h, "GET", financePathA+"?from=xato-sana", jwt["b"], "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("egalik tekshiruvi validatsiyadan keyin ishlagan: %d — %s", w.Code, w.Body.String())
	}
}

// Do'kon ulanmagan bo'lsa — bu XATO EMAS: panel ulash formasini
// ko'rsatishi uchun 200 va `connected: false` kerak. Shlyuzga so'rov
// ham yubormasligi kerak.
func TestFinanceNotConnectedReturnsOK(t *testing.T) {
	g := newFakeGateway(t, 0, nil)
	h, jwt := financeServer(t, g, "")

	out := getFinance(t, h, jwt["a"], "")
	if out.Connected {
		t.Error("do'kon yo'q, lekin connected=true")
	}
	if !out.Available {
		t.Error("server kalitlari bor, available=false bo'lmasligi kerak")
	}
	if out.StoreID != "" {
		t.Errorf("store_id bo'sh bo'lishi kerak: %q", out.StoreID)
	}
	if n := g.calls.Load(); n != 0 {
		t.Errorf("ulanmagan restoran uchun shlyuzga %d so'rov ketdi", n)
	}
	// Ro'yxatlar `null` emas, bo'sh massiv bo'lishi kerak — panel
	// ularni to'g'ridan-to'g'ri aylanib chiqadi.
	if out.Daily == nil || out.ByMethod == nil || out.Recent == nil {
		t.Error("bo'sh ro'yxatlar null qaytdi")
	}
}

// Shlyuz kalitlari umuman sozlanmagan (OnDex tomonidagi kamchilik):
// `available: false`, lekin sahifa baribir ochiladi.
func TestFinanceGatewayNotConfigured(t *testing.T) {
	h, jwt := financeServer(t, nil, "6")

	out := getFinance(t, h, jwt["a"], "")
	if out.Available {
		t.Error("klient yo'q, available=true")
	}
	if out.Connected {
		t.Error("klient yo'q, connected=true")
	}
}

// ┌─ ASOSIY HISOB ────────────────────────────────────────────────────┐
// Aylanma FAQAT `success` dan yig'iladi. `revert` — qaytarishga,
// qolgan holatlar (`payment_waiting`, `canceled`) HECH QAYERGA.
// Mutatsiya: `case multicard.StatusSuccess` ni olib tashlansa yoki
// `refunded` ga `+=` qo'yilsa, shu test yiqiladi.
// └───────────────────────────────────────────────────────────────────┘
func TestFinanceAggregatesOnlySuccess(t *testing.T) {
	today := time.Now().In(tashkent).Format("2006-01-02")
	body := historyJSON(4,
		txnJSON("t1", multicard.StatusSuccess, "uzcard", today, 10_000_00, 15_000),
		txnJSON("t2", multicard.StatusSuccess, "humo", today, 5_000_00, 7_500),
		txnJSON("t3", multicard.StatusRevert, "uzcard", today, 2_000_00, 0),
		txnJSON("t4", "payment_waiting", "uzcard", today, 99_000_00, 0),
	)
	g := newFakeGateway(t, 4, map[int]string{0: body})
	h, jwt := financeServer(t, g, "6")

	out := getFinance(t, h, jwt["a"], "?from="+today+"&to="+today)
	if !out.Connected {
		t.Fatal("connected=false")
	}
	if out.TurnoverTiyin != 15_000_00 {
		t.Errorf("aylanma: 1500000 kutildi, %d keldi", out.TurnoverTiyin)
	}
	if out.RefundedTiyin != 2_000_00 {
		t.Errorf("qaytarilgan: 200000 kutildi, %d keldi", out.RefundedTiyin)
	}
	if out.NetTiyin != 13_000_00 {
		t.Errorf("net: 1300000 kutildi, %d keldi", out.NetTiyin)
	}
	if out.CommissionTiyin != 22_500 {
		t.Errorf("komissiya: 22500 kutildi, %d keldi", out.CommissionTiyin)
	}
	if out.Transactions != 2 {
		t.Errorf("operatsiyalar soni: 2 kutildi, %d keldi", out.Transactions)
	}
	// So'nggi operatsiyalar — HAMMA holat, shu jumladan kutilayotgani.
	if len(out.Recent) != 4 {
		t.Errorf("recent: 4 kutildi, %d keldi", len(out.Recent))
	}
	// To'lov tizimi bo'yicha taqsimot summa kamayishi tartibida.
	if len(out.ByMethod) != 2 || out.ByMethod[0].PS != "uzcard" || out.ByMethod[1].PS != "humo" {
		t.Errorf("by_method tartibi/tarkibi xato: %+v", out.ByMethod)
	}
	if out.ByMethod[0].AmountTiyin != 10_000_00 {
		t.Errorf("uzcard summasi: %d", out.ByMethod[0].AmountTiyin)
	}
}

// Grafikdagi BO'SH kunlar ham qatorda turishi kerak: aks holda uch
// kunlik ma'lumot yetti kunlik davrda uzluksiz bo'lib ko'rinadi.
func TestFinanceDailyFillsEmptyDays(t *testing.T) {
	to := time.Now().In(tashkent)
	from := to.AddDate(0, 0, -4)
	day := from.AddDate(0, 0, 2).Format("2006-01-02")
	body := historyJSON(1, txnJSON("t1", multicard.StatusSuccess, "uzcard", day, 1_000_00, 0))

	g := newFakeGateway(t, 1, map[int]string{0: body})
	h, jwt := financeServer(t, g, "6")

	out := getFinance(t, h, jwt["a"],
		"?from="+from.Format("2006-01-02")+"&to="+to.Format("2006-01-02"))
	if len(out.Daily) != 5 {
		t.Fatalf("daily uzunligi: 5 kutildi, %d keldi (%+v)", len(out.Daily), out.Daily)
	}
	for i, d := range out.Daily {
		want := from.AddDate(0, 0, i).Format("2006-01-02")
		if d.Date != want {
			t.Errorf("daily[%d].date: %q kutildi, %q keldi", i, want, d.Date)
		}
		if d.Date == day {
			if d.AmountTiyin != 1_000_00 || d.Count != 1 {
				t.Errorf("to'lovli kun bo'sh: %+v", d)
			}
		} else if d.AmountTiyin != 0 || d.Count != 0 {
			t.Errorf("bo'sh kunda raqam bor: %+v", d)
		}
	}
}

// Davr chegaralari va noto'g'ri sana → 400.
func TestFinanceRangeValidation(t *testing.T) {
	g := newFakeGateway(t, 0, nil)
	h, jwt := financeServer(t, g, "6")

	bad := []string{
		"?from=2026-13-01",
		"?to=kecha",
		"?from=2026-09-10&to=2026-09-01", // teskari
		"?from=2020-01-01&to=2026-01-01", // bir yildan uzun
	}
	for _, q := range bad {
		w := do(t, h, "GET", financePathA+q, jwt["a"], "")
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: 400 kutildi, %d keldi — %s", q, w.Code, w.Body.String())
		}
	}
	// To'g'ri chegara (aynan bir yil) o'tishi kerak.
	w := do(t, h, "GET", financePathA+"?from=2026-01-01&to=2026-12-31", jwt["a"], "")
	if w.Code != http.StatusOK {
		t.Errorf("366 kunlik davr rad etildi: %d — %s", w.Code, w.Body.String())
	}
}

// Standart davr — 7 kun (bugun bilan birga), shlyuzga ham xuddi
// shunday oraliq yuboriladi.
func TestFinanceDefaultRangeIsSevenDays(t *testing.T) {
	g := newFakeGateway(t, 0, nil)
	h, jwt := financeServer(t, g, "6")

	out := getFinance(t, h, jwt["a"], "")
	if len(out.Daily) != 7 {
		t.Errorf("standart davr: 7 kun kutildi, %d keldi", len(out.Daily))
	}
	today := time.Now().In(tashkent)
	if out.To != today.Format("2006-01-02") {
		t.Errorf("to: %q kutildi, %q keldi", today.Format("2006-01-02"), out.To)
	}
	if out.From != today.AddDate(0, 0, -6).Format("2006-01-02") {
		t.Errorf("from: %q", out.From)
	}
	// Shlyuzga oxirgi soniyagacha yopiq oraliq ketadi.
	q, _ := g.lastQuery.Load().(string)
	if !strings.Contains(q, "23%3A59%3A59") {
		t.Errorf("end_date kun oxiri emas: %q", q)
	}
}

// Reestr bir sahifaga sig'masa, keyingi sahifalar ham o'qilishi kerak.
// Mutatsiya: `break` shartsiz qo'yilsa — ikkinchi sahifadagi pul
// yo'qoladi va aylanma kam ko'rinadi.
func TestFinancePaginatesThroughAllPages(t *testing.T) {
	today := time.Now().In(tashkent).Format("2006-01-02")
	page0 := []string{}
	for i := range 200 {
		page0 = append(page0, txnJSON(fmt.Sprintf("a%d", i), multicard.StatusSuccess, "uzcard", today, 1_00, 0))
	}
	g := newFakeGateway(t, 201, map[int]string{
		0:   historyJSON(201, page0...),
		200: historyJSON(201, txnJSON("b1", multicard.StatusSuccess, "uzcard", today, 500_00, 0)),
	})
	h, jwt := financeServer(t, g, "6")

	out := getFinance(t, h, jwt["a"], "?from="+today+"&to="+today)
	if out.Transactions != 201 {
		t.Errorf("operatsiyalar: 201 kutildi, %d keldi", out.Transactions)
	}
	if want := int64(200*1_00 + 500_00); out.TurnoverTiyin != want {
		t.Errorf("aylanma: %d kutildi, %d keldi", want, out.TurnoverTiyin)
	}
	// So'nggi operatsiyalar jadvali chegaralangan.
	if len(out.Recent) != financeRecentLimit {
		t.Errorf("recent: %d kutildi, %d keldi", financeRecentLimit, len(out.Recent))
	}
}

// ┌─ NOTO'G'RI DO'KON RAQAMI — FOYDALANUVCHI XATOSI ──────────────────┐
// Shlyuz 400 qaytarsa, u ISHLAYAPTI va so'rovimizni rad etdi. Amalda
// sabab bitta: do'kon raqami noto'g'ri. 502 qaytarilsa `httpError`
// tafsilotni yashiradi va panelda "server xatosi, keyinroq urinib
// ko'ring" chiqadi — restoran o'z xatosini tuzata olmaydi. Aynan shu
// jonli sinovda yuz berdi (`multicard_store_id` = "65").
// └───────────────────────────────────────────────────────────────────┘
func TestFinanceBadStoreIDIsUserError(t *testing.T) {
	g := newFakeGateway(t, 0, nil)
	g.fail.Store(true)
	g.failStatus.Store(http.StatusBadRequest)
	h, jwt := financeServer(t, g, "65")

	w := do(t, h, "GET", financePathA, jwt["a"], "")
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("422 kutildi, %d keldi — %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// Matn AYNAN qaysi raqam tanilmaganini aytishi kerak — aks holda
	// restoran nimani tuzatishni bilmaydi.
	if !strings.Contains(body, "65") {
		t.Errorf("javobda do'kon raqami yo'q: %s", body)
	}
	if !strings.Contains(body, "tekshiring") {
		t.Errorf("javob nima qilishni aytmaydi: %s", body)
	}
	// Shlyuzning O'Z matni (ruscha, ichki maydon nomlari bilan)
	// foydalanuvchiga chiqmaydi.
	if strings.Contains(body, "do'kon topilmadi") {
		t.Errorf("shlyuz matni javobga o'tib ketdi: %s", body)
	}
}

// ┌─ KO'P SAHIFALI REESTR: TARTIB VA PARALLELLIK ─────────────────────┐
// Sahifalar parallel olinadi, lekin natija reestr TARTIBIDA bo'lishi
// shart: "so'nggi operatsiyalar" birinchi sahifaning birinchi yozuvlari.
// Parallel yig'ishda tartib buzilsa, jadvalda eng ESKI yozuvlar
// chiqib qolardi.
// └───────────────────────────────────────────────────────────────────┘
func TestFinanceManyPagesKeepOrderAndTotals(t *testing.T) {
	today := time.Now().In(tashkent).Format("2006-01-02")

	// ⚠️ Sahifalar ATAYIN kichik (5 + 10 + 10 ...), chunki "so'nggi
	// operatsiyalar" faqat birinchi 20 ta yozuvni oladi. Agar birinchi
	// sahifa 200 ta bo'lsa, ular hammasi o'sha sahifadan keladi va
	// qolgan sahifalar TARTIBI hech qachon tekshirilmaydi. (Shu
	// kamchilik mutatsiya sinovida topildi: sahifalarni almashtirib
	// qo'yilgan kod ham testdan o'tib ketgan edi.)
	//
	// Jami son (`total`) esa 1200 deb va'da qilinadi — server aynan
	// shundan 6 sahifa hisoblaydi.
	const total = 1200
	const pages = 6
	sizes := []int{5, 10, 10, 10, 10, 10}

	pagesJSON := map[int]string{}
	var wantOrder []string
	var wantTurnover int64
	n := 0
	for p := range pages {
		var rows []string
		for i := 0; i < sizes[p]; i++ {
			id := fmt.Sprintf("tx-%03d", n)
			amt := int64(n + 1)
			wantTurnover += amt
			wantOrder = append(wantOrder, id)
			rows = append(rows, txnJSON(id, multicard.StatusSuccess, "uzcard", today, amt, 0))
			n++
		}
		pagesJSON[p*200] = historyJSON(total, rows...)
	}
	g := newFakeGateway(t, total, pagesJSON)
	h, jwt := financeServer(t, g, "6")

	out := getFinance(t, h, jwt["a"], "?from="+today+"&to="+today)
	if out.Transactions != n {
		t.Fatalf("operatsiyalar: %d, %d kutildi", out.Transactions, n)
	}
	if out.TurnoverTiyin != wantTurnover {
		t.Errorf("aylanma: %d, %d kutildi (sahifa yo'qolgan yoki ikki marta sanalgan)",
			out.TurnoverTiyin, wantTurnover)
	}
	// So'nggi operatsiyalar — reestr TARTIBIDA, bir necha sahifadan.
	if len(out.Recent) != financeRecentLimit {
		t.Fatalf("recent: %d", len(out.Recent))
	}
	for i, tx := range out.Recent {
		if tx.UUID != wantOrder[i] {
			t.Errorf("recent[%d] = %s, %s kutildi — parallel yig'ishda tartib buzildi",
				i, tx.UUID, wantOrder[i])
		}
	}
}

// Sahifalardan biri yiqilsa — butun javob xato (yarim hisobot
// "to'liq" bo'lib ko'rinmasligi kerak).
func TestFinanceFailedPageFailsWholeResponse(t *testing.T) {
	today := time.Now().In(tashkent).Format("2006-01-02")
	var rows []string
	for i := range 200 {
		rows = append(rows, txnJSON(fmt.Sprintf("a%d", i), multicard.StatusSuccess, "uzcard", today, 100, 0))
	}
	// 3 sahifa deb va'da qilinadi, lekin 2-sahifa (offset 200) yo'q.
	g := newFakeGateway(t, 600, map[int]string{0: historyJSON(600, rows...)})
	g.failAtOffset.Store(200)
	g.failStatus.Store(http.StatusInternalServerError)
	h, jwt := financeServer(t, g, "6")

	w := do(t, h, "GET", financePathA+"?from="+today+"&to="+today, jwt["a"], "")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("502 kutildi, %d keldi — %s", w.Code, w.Body.String())
	}
}

// ┌─ KESH ────────────────────────────────────────────────────────────┐
// Shlyuz ketma-ket bir xil so'rovni QAYTA olmasligi kerak: Moliya
// sahifasiga har kirganda 1–5 soniya kutish shundan edi.
//
// Redis testda yo'q (`Cache` nil — no-op), shuning uchun bu test
// KESHLASH MANTIG'INI emas, uning NATIJASINI tekshiradi: kesh yo'q
// bo'lsa ham javob to'g'ri va barqaror.
// └───────────────────────────────────────────────────────────────────┘
func TestFinanceResponseIsStableWithoutCache(t *testing.T) {
	today := time.Now().In(tashkent).Format("2006-01-02")
	g := newFakeGateway(t, 1, map[int]string{
		0: historyJSON(1, txnJSON("t1", multicard.StatusSuccess, "uzcard", today, 5_000_00, 0)),
	})
	h, jwt := financeServer(t, g, "6")

	a := getFinance(t, h, jwt["a"], "?from="+today+"&to="+today)
	b := getFinance(t, h, jwt["a"], "?from="+today+"&to="+today)
	if a.TurnoverTiyin != b.TurnoverTiyin || a.Transactions != b.Transactions {
		t.Errorf("ikki so'rov turli natija berdi: %+v / %+v", a, b)
	}
}

// Shlyuz yiqilsa — 502. 500 EMAS: nosozlik bizda emas, panel
// "keyinroq urinib ko'ring" deb ayta olishi kerak.
func TestFinanceGatewayFailureIsBadGateway(t *testing.T) {
	g := newFakeGateway(t, 0, nil)
	g.fail.Store(true)
	g.failStatus.Store(http.StatusInternalServerError)
	h, jwt := financeServer(t, g, "6")

	w := do(t, h, "GET", financePathA, jwt["a"], "")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("502 kutildi, %d keldi — %s", w.Code, w.Body.String())
	}
	// `httpError` 5xx tafsilotini yashiradi: shlyuzning matni ham,
	// bizning kalitlarimiz ham javobga CHIQMAYDI (faqat logga).
	body := w.Body.String()
	for _, leak := range []string{"do'kon topilmadi", "sekret", "rhmt_test", "multicard"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(leak)) {
			t.Errorf("javobda ichki tafsilot oshkor bo'ldi (%q): %s", leak, body)
		}
	}
}

// Javobda shlyuz kalitlari HECH QACHON bo'lmaydi (ulangan holatda ham).
func TestFinanceResponseLeaksNoCredentials(t *testing.T) {
	today := time.Now().In(tashkent).Format("2006-01-02")
	g := newFakeGateway(t, 1, map[int]string{
		0: historyJSON(1, txnJSON("t1", multicard.StatusSuccess, "uzcard", today, 1_000_00, 0)),
	})
	h, jwt := financeServer(t, g, "6")

	w := do(t, h, "GET", financePathA+"?from="+today+"&to="+today, jwt["a"], "")
	body := w.Body.String()
	for _, secret := range []string{"sekret", "rhmt_test", "Bearer", "tok\""} {
		if strings.Contains(body, secret) {
			t.Errorf("javobda %q bor: %s", secret, body)
		}
	}
	// Karta raqami niqoblangan holicha qoladi (to'liq PAN hech qachon).
	if strings.Contains(body, "8600305959") {
		t.Error("javobda to'liq karta raqami")
	}
}

// ┌─ OCHIQ KATALOG TO'LOV MAYDONLARINI KO'RSATMAYDI ──────────────────┐
// `GET /restaurants` va `GET /restaurants/{id}` `catalog.Restaurant`
// ni TO'G'RIDAN-TO'G'RI mijozga qaytaradi. Ya'ni strukturaga qo'shilgan
// har qanday `json` tegi OMMAGA chiqadi.
//
// Shifrlangan `secret` ochiq internetga chiqsa, uni oflayn buzishga
// urinish mumkin bo'lardi; kassa raqami va hisob niqobi esa mijozga
// umuman kerak emas. Shuning uchun hammasida `json:"-"`.
//
// Test IKKI qatlamli: (1) haqiqiy javob tanasi tekshiriladi;
// (2) `Multicard` bilan boshlanadigan YANGI maydon qo'shilsa ham
// tutiladi — kelajakdagi maydon jimgina chiqib ketmasin.
// └───────────────────────────────────────────────────────────────────┘
func TestPublicRestaurantHidesPaymentFields(t *testing.T) {
	const secret = "ENCRYPTED-SECRET-VALUE"
	h, _, _ := financeServerFull(t, nil, "", catalog.Restaurant{
		ID: testRestA, Name: "Book Cafe", Open: true,
		MulticardStoreID: "6", MulticardAppID: "bookcafe_app",
		MulticardSecretEnc:  secret,
		MulticardStoreTitle: "Book Cafe kassasi", MulticardAccountMasked: "••••6789",
	})

	for _, path := range []string{"/restaurants", "/restaurants/" + testRestA} {
		w := do(t, h, "GET", path, "", "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, w.Code)
		}
		body := w.Body.String()
		for _, leak := range []string{
			secret, "bookcafe_app", "Book Cafe kassasi", "6789",
			"multicard_secret", "multicard_app", "multicard_store", "multicard_account",
		} {
			if strings.Contains(body, leak) {
				t.Errorf("%s: javobda %q bor — %s", path, leak, body)
			}
		}
	}

	// Kelajakdagi maydonlar uchun: `Multicard*` hammasi JSON'dan
	// chiqarilgan bo'lishi SHART.
	rt := reflect.TypeOf(catalog.Restaurant{})
	for i := range rt.NumField() {
		f := rt.Field(i)
		if !strings.HasPrefix(f.Name, "Multicard") {
			continue
		}
		if f.Tag.Get("json") != "-" {
			t.Errorf("%s: `json:\"-\"` bo'lishi SHART (ochiq katalogga chiqib ketadi), "+
				"hozir: %q", f.Name, f.Tag.Get("json"))
		}
	}
}

// ─── Restoran O'Z hisobini ulaydi ──────────────────────────────────────

const financeConnectPathA = financePathA + "/connect"

// connectServer — shlyuz + restoran A (ulanmagan) + ulash uchun tayyor
// server. `authOK`/`knownStores` orqali xato kalit/kassa sinaladi.
func connectServer(t *testing.T) (*fakeGateway, http.Handler, catalog.Repository, map[string]string) {
	t.Helper()
	g := newFakeGateway(t, 0, nil)
	g.authOK = func(appID, secret string) bool {
		return appID == "bookcafe_app" && secret == "s3cr3t-of-restaurant"
	}
	g.knownStores = map[string]bool{"6": true}
	h, repo, jwt := financeServerFull(t, nil, g.srv.URL, catalog.Restaurant{
		ID: testRestA, Name: "Book Cafe", Open: true,
	})
	return g, h, repo, jwt
}

func connectBody(app, secret, store string) string {
	return fmt.Sprintf(`{"application_id":%q,"secret":%q,"store_id":%q}`, app, secret, store)
}

// ┌─ ASOSIY OQIM ─────────────────────────────────────────────────────┐
// Restoran O'Z kalitlarini kiritadi → server ularni shlyuzda
// TEKSHIRADI → saqlaydi → javobda PUL QAYERGA tushishi ko'rinadi.
// └───────────────────────────────────────────────────────────────────┘
func TestFinanceConnectWithOwnCredentials(t *testing.T) {
	g, h, repo, jwt := connectServer(t)

	w := do(t, h, "POST", financeConnectPathA, jwt["a"],
		connectBody("bookcafe_app", "s3cr3t-of-restaurant", "6"))
	if w.Code != http.StatusOK {
		t.Fatalf("200 kutildi, %d keldi — %s", w.Code, w.Body.String())
	}
	var out financeConnectResponse
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	// Restoran nimaga ulanganini ko'rishi kerak.
	if out.StoreTitle != "Book Cafe kassasi" || out.MerchantName != "Book Cafe MChJ" {
		t.Errorf("kassa/tashkilot: %+v", out)
	}
	if out.MerchantTIN != "307578794" || out.ContractNum != "POS/8042" {
		t.Errorf("stir/shartnoma: %+v", out)
	}
	if out.FeePercent != "1.5" {
		t.Errorf("komissiya: %q", out.FeePercent)
	}
	// ⚠️ Hisob raqami FAQAT oxirgi raqamlari bilan.
	if out.AccountMasked != "••••6789" {
		t.Errorf("hisob niqobi: %q", out.AccountMasked)
	}
	if strings.Contains(w.Body.String(), "20208000900123456789") {
		t.Error("javobda TO'LIQ hisob raqami")
	}

	// Kalitlar shlyuzda tekshirilgan: `/auth` va kassa so'rovi ketgan.
	if got, _ := g.lastAuth.Load().([2]string); got != [2]string{"bookcafe_app", "s3cr3t-of-restaurant"} {
		t.Errorf("shlyuzga ketgan kalitlar: %v", got)
	}
	if g.storeCalls.Load() == 0 {
		t.Error("kassa tekshirilmadi")
	}

	// Bazada: `secret` SHIFRLANGAN, ochiq matnda emas.
	rest, err := repo.GetRestaurant(context.Background(), testRestA)
	if err != nil {
		t.Fatal(err)
	}
	if rest.MulticardAppID != "bookcafe_app" || rest.MulticardStoreID != "6" {
		t.Errorf("saqlanmadi: %+v", rest)
	}
	if rest.MulticardSecretEnc == "" || strings.Contains(rest.MulticardSecretEnc, "s3cr3t") {
		t.Errorf("secret ochiq matnda saqlandi: %q", rest.MulticardSecretEnc)
	}
	plain, err := testSecrets(t).Open(rest.MulticardSecretEnc)
	if err != nil || plain != "s3cr3t-of-restaurant" {
		t.Errorf("shifrni ochib bo'lmadi: %v / %q", err, plain)
	}
}

// Ulangandan keyin Moliya sahifasi RESTORANNING kalitlari bilan
// ishlashi kerak — platforma kaliti yo'q bo'lsa ham.
func TestFinanceUsesOwnCredentialsAfterConnect(t *testing.T) {
	g, h, _, jwt := connectServer(t)
	if w := do(t, h, "POST", financeConnectPathA, jwt["a"],
		connectBody("bookcafe_app", "s3cr3t-of-restaurant", "6")); w.Code != http.StatusOK {
		t.Fatalf("ulash: %d — %s", w.Code, w.Body.String())
	}
	today := time.Now().In(tashkent).Format("2006-01-02")
	g.pages = map[int]string{
		0: historyJSON(1, txnJSON("t1", multicard.StatusSuccess, "uzcard", today, 7_000_00, 0)),
	}
	g.total = 1

	out := getFinance(t, h, jwt["a"], "?from="+today+"&to="+today)
	if !out.Connected || !out.OwnCredentials {
		t.Fatalf("connected=%v own=%v", out.Connected, out.OwnCredentials)
	}
	if out.TurnoverTiyin != 7_000_00 {
		t.Errorf("aylanma: %d", out.TurnoverTiyin)
	}
	if out.StoreTitle != "Book Cafe kassasi" || out.AccountMasked != "••••6789" {
		t.Errorf("ulanish tafsiloti ko'rinmadi: %+v", out)
	}
	// ⚠️ `secret` javobda YO'Q.
	if strings.Contains(out.AppID, "s3cr3t") {
		t.Error("app_id ichida sir")
	}
}

// Noto'g'ri kalit — 422 va SAQLANMAYDI.
func TestFinanceConnectRejectsBadCredentials(t *testing.T) {
	_, h, repo, jwt := connectServer(t)

	w := do(t, h, "POST", financeConnectPathA, jwt["a"],
		connectBody("bookcafe_app", "notogri", "6"))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("422 kutildi, %d — %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "secret") {
		t.Errorf("matn nimani tekshirishni aytmaydi: %s", w.Body.String())
	}
	rest, _ := repo.GetRestaurant(context.Background(), testRestA)
	if rest.MulticardAppID != "" || rest.MulticardSecretEnc != "" {
		t.Error("yaroqsiz kalitlar saqlanib qoldi")
	}
}

// Kalitlar to'g'ri, kassa raqami xato — 422 va SAQLANMAYDI.
//
// Aynan shu tekshiruv yetishmagani uchun "65" kiritilib, xato faqat
// bir kundan keyin, Moliya sahifasi ochilganda bilindi.
func TestFinanceConnectRejectsUnknownStore(t *testing.T) {
	_, h, repo, jwt := connectServer(t)

	w := do(t, h, "POST", financeConnectPathA, jwt["a"],
		connectBody("bookcafe_app", "s3cr3t-of-restaurant", "65"))
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("422 kutildi, %d — %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if !strings.Contains(body, "65") {
		t.Errorf("javobda kassa raqami yo'q: %s", body)
	}
	// Shlyuzning ruscha matni chiqmaydi.
	if strings.Contains(body, "Касса") {
		t.Errorf("shlyuz matni javobga o'tdi: %s", body)
	}
	rest, _ := repo.GetRestaurant(context.Background(), testRestA)
	if rest.MulticardStoreID != "" {
		t.Error("tekshirilmagan kassa saqlandi")
	}
}

func TestFinanceConnectValidatesInput(t *testing.T) {
	_, h, _, jwt := connectServer(t)

	bad := map[string]string{
		"app bo'sh":      connectBody("", "s", "6"),
		"app xavfli":     connectBody("app id/../x", "s", "6"),
		"secret bo'sh":   connectBody("bookcafe_app", "  ", "6"),
		"secret uzun":    connectBody("bookcafe_app", strings.Repeat("x", 300), "6"),
		"kassa bo'sh":    connectBody("bookcafe_app", "s", ""),
		"kassa xavfli":   connectBody("bookcafe_app", "s", "6/../7"),
		"tana JSON emas": "{",
	}
	for name, body := range bad {
		w := do(t, h, "POST", financeConnectPathA, jwt["a"], body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: 400 kutildi, %d keldi — %s", name, w.Code, w.Body.String())
		}
	}
}

// Ulash — pul manzilini o'zgartiradi, ya'ni eng nozik amal.
func TestFinanceConnectAccessControl(t *testing.T) {
	_, h, _, jwt := connectServer(t)
	body := connectBody("bookcafe_app", "s3cr3t-of-restaurant", "6")

	cases := []struct {
		token string
		want  int
		why   string
	}{
		{"", http.StatusUnauthorized, "tokensiz"},
		{jwt["customer"], http.StatusForbidden, "mijoz"},
		{jwt["waiter"], http.StatusForbidden, "affitsiant"},
		{jwt["courier"], http.StatusForbidden, "kuryer"},
		{jwt["b"], http.StatusForbidden, "boshqa restoran"},
	}
	for _, c := range cases {
		w := do(t, h, "POST", financeConnectPathA, c.token, body)
		if w.Code != c.want {
			t.Errorf("%s: %d kutildi, %d keldi", c.why, c.want, w.Code)
		}
		// O'chirish ham xuddi shunday himoyalangan.
		if w := do(t, h, "DELETE", financeConnectPathA, c.token, ""); w.Code != c.want {
			t.Errorf("%s (DELETE): %d kutildi, %d keldi", c.why, c.want, w.Code)
		}
	}
}

// Uzish — kalitlar bazadan butunlay o'chadi.
func TestFinanceDisconnectClearsCredentials(t *testing.T) {
	_, h, repo, jwt := connectServer(t)
	if w := do(t, h, "POST", financeConnectPathA, jwt["a"],
		connectBody("bookcafe_app", "s3cr3t-of-restaurant", "6")); w.Code != http.StatusOK {
		t.Fatalf("ulash: %d", w.Code)
	}
	if w := do(t, h, "DELETE", financeConnectPathA, jwt["a"], ""); w.Code != http.StatusNoContent {
		t.Fatalf("uzish: %d — %s", w.Code, w.Body.String())
	}
	rest, _ := repo.GetRestaurant(context.Background(), testRestA)
	if rest.MulticardAppID != "" || rest.MulticardSecretEnc != "" ||
		rest.MulticardStoreID != "" || rest.MulticardStoreTitle != "" {
		t.Errorf("uzilgandan keyin qoldi: %+v", rest)
	}
	out := getFinance(t, h, jwt["a"], "")
	if out.Connected {
		t.Error("uzilgandan keyin ham connected=true")
	}
}

// Kalit almashtirilsa keshdagi ESKI klient ishlatilmasligi kerak.
func TestFinanceConnectRefreshesCachedClient(t *testing.T) {
	g, h, _, jwt := connectServer(t)
	if w := do(t, h, "POST", financeConnectPathA, jwt["a"],
		connectBody("bookcafe_app", "s3cr3t-of-restaurant", "6")); w.Code != http.StatusOK {
		t.Fatalf("ulash: %d", w.Code)
	}
	// Bir marta ishlatamiz — klient keshga tushadi.
	getFinance(t, h, jwt["a"], "")

	// Restoran shlyuzda sirini almashtirdi va panelga yangisini kiritdi.
	g.authOK = func(appID, secret string) bool {
		return appID == "bookcafe_app" && secret == "yangi-sir"
	}
	if w := do(t, h, "POST", financeConnectPathA, jwt["a"],
		connectBody("bookcafe_app", "yangi-sir", "6")); w.Code != http.StatusOK {
		t.Fatalf("qayta ulash: %d — %s", w.Code, w.Body.String())
	}
	// Eski sir keshda qolgan bo'lsa shu so'rov 502 bilan yiqilardi.
	out := getFinance(t, h, jwt["a"], "")
	if !out.Connected {
		t.Error("qayta ulangandan keyin connected=false")
	}
	if got, _ := g.lastAuth.Load().([2]string); got[1] != "yangi-sir" {
		t.Errorf("shlyuzga eski sir ketdi: %v", got)
	}
}

// Shifrlash kaliti yo'q bo'lsa ulash TAKLIF QILINMAYDI (aks holda
// `secret` ochiq matnda yotardi).
func TestFinanceConnectNeedsEncryptionKey(t *testing.T) {
	g := newFakeGateway(t, 0, nil)
	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{{ID: testRestA, Name: "Book Cafe", Open: true}}, nil)
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	u := &users.User{ID: "u-x", Phone: "+998900000099", Role: users.RoleRestaurant,
		EntityID: testRestA, PhoneVerified: true, CreatedAt: time.Now()}
	if err := userRepo.Create(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	tok, err := tokens.Issue(u)
	if err != nil {
		t.Fatal(err)
	}
	orderRepo := storage.NewMemoryOrderRepo()
	h := New(Deps{
		UserRepo: userRepo, CatalogRepo: catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(), OrderRepo: orderRepo,
		TableSvc: tables.NewService(storage.NewMemoryTableRepo()), Tokens: tokens,
		CatalogSvc: catalog.NewService(catalogRepo),
		OrderSvc:   orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo()),
		Hub:        ws.NewHub(nil),
		MC:         mcconnect.NewRegistry(nil, nil, g.srv.URL, nil),
		DevMode:    true,
	}).Routes(nil)

	w := do(t, h, "POST", financeConnectPathA, tok,
		connectBody("bookcafe_app", "s3cr3t-of-restaurant", "6"))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("503 kutildi, %d — %s", w.Code, w.Body.String())
	}
	// Sahifa ham "ulash mumkin emas" deb ko'rsatadi.
	var out financeResponse
	wr := do(t, h, "GET", financePathA, tok, "")
	if err := json.Unmarshal(wr.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Available {
		t.Error("shifrlash kaliti yo'q, lekin available=true")
	}
}

// Oldingi davr xatosi butun sahifani yiqitmasligi kerak: u faqat
// foiz ko'rsatkichi uchun. Bu yerda joriy davr muvaffaqiyatli,
// oldingisi uchun shlyuz bo'sh javob beradi.
func TestFinancePreviousPeriodIsRequested(t *testing.T) {
	today := time.Now().In(tashkent).Format("2006-01-02")
	g := newFakeGateway(t, 1, map[int]string{
		0: historyJSON(1, txnJSON("t1", multicard.StatusSuccess, "uzcard", today, 1_000_00, 0)),
	})
	h, jwt := financeServer(t, g, "6")

	out := getFinance(t, h, jwt["a"], "?from="+today+"&to="+today)
	if out.TurnoverTiyin != 1_000_00 {
		t.Fatalf("joriy davr: %d", out.TurnoverTiyin)
	}
	// Ikki so'rov: joriy davr + oldingi davr.
	if n := g.calls.Load(); n < 2 {
		t.Errorf("oldingi davr so'rovi yuborilmadi (jami %d so'rov)", n)
	}
}
