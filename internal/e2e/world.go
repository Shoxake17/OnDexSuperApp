package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"chustapp/internal/alerts"
	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/e2e/report"
	"chustapp/internal/geo"
	"chustapp/internal/httpapi"
	"chustapp/internal/images"
	"chustapp/internal/notify"
	"chustapp/internal/orders"
	"chustapp/internal/payments"
	"chustapp/internal/payments/mcconnect"
	"chustapp/internal/payments/multicardpay"
	"chustapp/internal/qurtorders"
	"chustapp/internal/revoke"
	"chustapp/internal/secretstore"
	"chustapp/internal/sso"
	"chustapp/internal/staff"
	"chustapp/internal/storage"
	"chustapp/internal/support"
	"chustapp/internal/tables"
	"chustapp/internal/users"
	"chustapp/internal/wallet"
	"chustapp/internal/ws"
)

// Chust markazi — mijoz manzili va restoran joylashuvi (xizmat hududi ichida).
const (
	ChustLat = 41.0004
	ChustLng = 71.2394

	RestA = "e2e-rest-a"
	RestB = "e2e-rest-b"

	// ProdOsh — A restoranidagi taom: 35 000 so'm.
	ProdOsh      = "e2e-prod-osh"
	ProdOshPrice = int64(3_500_000)
	// ProdB — B restoranidagi taom (tenant izolyatsiyasi uchun).
	ProdB = "e2e-prod-b"

	encKey = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"

	// Yagona kirish (SSO) test klienti.
	SSOClientID     = "merchant"
	SSOClientSecret = "e2e-sso-secret-e2e-sso-secret-e2e-sso-secret-0123456789"
	SSORedirectURI  = "https://pos.ondex.uz/auth/callback"
)

// noGoogle — Google Distance Matrix o'rniga: har doim xato qaytaradi va
// dispatch to'g'ri chiziq (zaxira) ETA'siga o'tadi. Tashqi tarmoqsiz,
// deterministik.
type noGoogle struct{}

func (noGoogle) FetchETAs(context.Context, geo.LatLng, []geo.Candidate) ([]geo.Result, error) {
	return nil, errors.New("e2e: tashqi xarita xizmati o'chiq — zaxira ETA")
}

// smsLog — haqiqiy SMS o'rniga: yuborilgan deb hisoblaydi (kod dev rejimida
// javobda qaytadi).
type smsLog struct{}

func (smsLog) Send(string, string) error { return nil }

// offerLog — kuryerga yuborilgan takliflar (push/WS o'rniga) jurnali.
type offerLog struct {
	w    *World
	next couriers.OfferNotifier // haqiqiy kanal (WS + push)
}

func (o *offerLog) SendOffer(courierID string, info couriers.OfferInfo) {
	o.w.mu.Lock()
	o.w.offers = append(o.w.offers, courierID+":"+info.OrderID)
	o.w.mu.Unlock()
	if o.next != nil {
		o.next.SendOffer(courierID, info)
	}
}

func (o *offerLog) CancelOffer(courierID, orderID string) {
	if o.next != nil {
		o.next.CancelOffer(courierID, orderID)
	}
}

// OffersTo — shu kuryerga nechta taklif yuborilgani.
func (w *World) OffersTo(courierID string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := 0
	for _, o := range w.offers {
		if strings.HasPrefix(o, courierID+":") {
			n++
		}
	}
	return n
}

// Actor — tizimga kirgan ishtirokchi (token bilan).
type Actor struct {
	Name   string
	Token  string
	UserID string
	Phone  string
	Entity string // restoran/kuryer ID

	// Headers — qo'shimcha sarlavhalar (masalan POS serveri klient siri bilan,
	// Authorization'siz). Bo'sh bo'lishi mumkin.
	Headers map[string]string
}

// World — butun tizimning jonli nusxasi: HAQIQIY HTTP serveri, xotira
// repolari, soxta to'lov shlyuzi. Production'dagi `cmd/api` ulanishiga
// iloji boricha o'xshash.
type World struct {
	Srv     *httptest.Server
	Gateway *Gateway
	client  *http.Client

	Users    *storage.MemoryUserRepo
	Orders   *storage.MemoryOrderRepo
	Couriers *storage.MemoryCourierRepo
	Catalog  *storage.MemoryCatalogRepo
	Payments *storage.MemoryPaymentRepo
	Tokens   *users.TokenIssuer

	Admin    *Actor
	Customer *Actor
	Owner    *Actor // A restoran egasi
	OwnerB   *Actor // B restoran egasi (begona)
	Other    *Actor // ikkinchi mijoz (begona)

	scenario string
	step     string
	stopped  bool

	Hub *ws.Hub
	api *httpapi.Server
	// Wallet — OnDex Wallet xizmati (balansni to'g'ridan-to'g'ri to'ldirish uchun).
	Wallet *wallet.Service
	skew   time.Duration    // Advance bilan oldinga siljitilgan soat
	rec    *report.Recorder // nil bo'lsa umumiy Rec (meta-testlar o'zinikini beradi)
	gated  bool
	// polling — `Eventually` ichida: so'rovlar kutish uchun takrorlanadi.
	polling bool
	ctype   string // multipart so'rovlar uchun Content-Type

	mu      sync.Mutex
	offers  []string
	closers []io.Closer
}

// NewWorld — yangi, toza dunyo. Har ssenariy O'ZINING dunyosini oladi:
// bir ssenariyning yon ta'siri boshqasiga o'tmaydi.
func NewWorld(t *testing.T, scenario string) *World { return NewWorldWith(t, scenario, Options{}) }

// Options — dunyoning ixtiyoriy sozlamalari (bayroqlar).
// LiveMulticard - haqiqiy Multicard SANDBOX sozlamalari (soxta shlyuz o'rniga).
type LiveMulticard struct {
	BaseURL, AppID, Secret, StoreID string
	// NotifyURL - shlyuz callback yuboradigan HAMMA UCHUN ochiq https manzil
	// (tunnel). `payments.NewService` nisbiy yoki http manzilni rad etadi.
	NotifyURL string
}

type Options struct {
	// PlatformCouriers — OnDex platforma kuryerlari (o'zi ro'yxatdan o'tadigan) yoqilgan.
	PlatformCouriers bool
	// OfferTTL — kuryerga taklif muddati (standart 10 s).
	OfferTTL time.Duration

	// Live - soxta shlyuz O'RNIGA haqiqiy Multicard SANDBOX (qo'lda yoqiladigan
	// sinov: `sandbox_payment_test.go`). nil bo'lsa soxta shlyuz ishlatiladi.
	Live *LiveMulticard
}

// NewWorldWith — NewWorld, sozlamalar bilan.
func NewWorldWith(t *testing.T, scenario string, opts Options) *World {
	t.Helper()
	ctx := context.Background()
	w := &World{scenario: scenario, client: &http.Client{Timeout: 15 * time.Second}}

	t.Cleanup(func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		for _, c := range w.closers {
			_ = c.Close()
		}
	})
	w.Users = storage.NewMemoryUserRepo()
	w.Tokens = users.NewTokenIssuer("e2e-secret", time.Hour)
	revoked := revoke.New(nil, time.Hour)

	w.Gateway = NewGateway()
	t.Cleanup(w.Gateway.Close)

	box, err := secretstore.New(encKey)
	if err != nil {
		t.Fatal(err)
	}
	// Soxta shlyuz (standart) yoki haqiqiy sandbox (`opts.Live`).
	mcApp, mcStore, mcBase, mcNotify, mcSecret := GatewayAppID, GatewayStore, w.Gateway.Srv.URL,
		"https://api.example/payments/multicard/callback", GatewaySecret
	if opts.Live != nil {
		mcApp, mcStore, mcBase, mcNotify, mcSecret = opts.Live.AppID, opts.Live.StoreID,
			opts.Live.BaseURL, opts.Live.NotifyURL, opts.Live.Secret
	}
	enc, err := box.Seal(mcSecret)
	if err != nil {
		t.Fatal(err)
	}

	w.Catalog = storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{
			{
				ID: RestA, Name: "E2E Cafe", Kind: catalog.KindCafe, Address: "M. Fayozov ko'chasi",
				Lat: ChustLat, Lng: ChustLng, Open: true,
				MulticardAppID: mcApp, MulticardSecretEnc: enc, MulticardStoreID: mcStore,
				MulticardTaxMXIK: "10204001001000000", MulticardTaxPackageCode: "1500169",
			},
			{ID: RestB, Name: "E2E Boshqa", Kind: catalog.KindCafe, Lat: ChustLat, Lng: ChustLng, Open: true},
		},
		[]catalog.Product{
			{ID: ProdOsh, RestaurantID: RestA, Name: "Osh", Category: "Asosiy", PriceTiyin: ProdOshPrice, Available: true},
			{ID: ProdB, RestaurantID: RestB, Name: "Somsa", Category: "Asosiy", PriceTiyin: 1_000_000, Available: true},
		})
	w.Couriers = storage.NewMemoryCourierRepo()
	w.Orders = storage.NewMemoryOrderRepo()
	w.Payments = storage.NewMemoryPaymentRepo()
	// Production bilan bir xil: narxlash yo'lidagi qisqa keshlar (storage/cached_*.go).
	// E2E ssenariylari butun oqimni AYNAN shu keshlar orqali o'tkazadi: yozishda
	// bekor qilish buzilsa (restoran yopildi, narx/haq o'zgardi) ssenariy yiqiladi.
	const cacheTTL = 2 * time.Second
	promo := storage.NewCachedPromotions(storage.NewMemoryPromotionsRepo(), cacheTTL)
	staffRepo := storage.NewCachedStaff(storage.NewMemoryStaffRepo(), cacheTTL)
	cachedCatalog := storage.NewCachedCatalog(w.Catalog, cacheTTL)

	staffSvc := staff.NewService(staffRepo, &staff.UserAccounts{
		Users: w.Users, Couriers: w.Couriers, Revoked: revoked, NewID: httpapi.NewID,
		CourierBusy: func(ctx context.Context, courierID string) (bool, error) {
			_, err := w.Orders.GetActiveByCourier(ctx, courierID)
			if err != nil {
				// Faol buyurtma yo'q (topilmadi): kuryer band emas, bu xato emas.
				return false, nil //nolint:nilerr // topilmadi = band emas
			}
			return true, nil
		},
	})

	hub := ws.NewHub(nil)
	w.Hub = hub
	alertsSvc := alerts.NewService(storage.NewMemoryAlertStore(), hub, httpapi.NewID).WithOrders(w.Orders.GetByID)
	staffSvc.WithObserver(alertsSvc.StaffEvents)
	walletSvc := wallet.NewService(storage.NewMemoryWalletRepo(), httpapi.NewID)
	w.Wallet = walletSvc

	notifStore := storage.NewMemoryNotificationStore()
	tokenStore := storage.NewMemoryTokenStore()
	notifSvc := notify.NewService(notifStore, hub, httpapi.NewID)
	notifier := notify.NewLive(notifSvc).
		WithCourierUserLookup(func(ctx context.Context, courierID string) (string, error) {
			u, err := w.Users.GetByRoleEntity(ctx, users.RoleCourier, courierID)
			if err != nil {
				return "", err
			}
			return u.ID, nil
		}).
		WithWaiterLookup(func(ctx context.Context, restaurantID string) ([]string, error) {
			all, err := w.Users.ListByRole(ctx, users.RoleWaiter)
			if err != nil {
				return nil, err
			}
			var ids []string
			for _, u := range all {
				if u.EntityID == restaurantID {
					ids = append(ids, u.ID)
				}
			}
			return ids, nil
		})

	orderSvc := orders.NewService(w.Orders, alertsSvc.WrapOrders(notifier), httpapi.NewID, promo).WithClock(w.now)
	orderSvc.WithDelivery(httpapi.NewDeliveryPricer(staffRepo))
	orderSvc.WithWallet(walletSvc)

	reg := mcconnect.NewRegistry(
		func(ctx context.Context, id string) (mcconnect.Credentials, error) {
			rest, err := w.Catalog.GetRestaurant(ctx, id)
			if err != nil {
				return mcconnect.Credentials{}, err
			}
			return mcconnect.Credentials{
				AppID: rest.MulticardAppID, SecretEnc: rest.MulticardSecretEnc,
				StoreID: rest.MulticardStoreID, TaxMXIK: rest.MulticardTaxMXIK,
				TaxPackageCode: rest.MulticardTaxPackageCode,
			}, nil
		}, box, mcBase, nil)
	paySvc, err := payments.NewService(w.Payments, multicardpay.New(reg), orderSvc, httpapi.NewID,
		payments.Options{NotifyURL: mcNotify})
	if err != nil {
		t.Fatal(err)
	}
	orderSvc.WithPayments(paySvc)

	offerTTL := opts.OfferTTL
	if offerTTL == 0 {
		offerTTL = 10 * time.Second
	}
	dispatcher := couriers.NewDispatcher(w.Couriers, &offerLog{w: w, next: notifier}, noGoogle{}, offerTTL)

	mk := func(name, id string, role users.Role, entity, phone string, addr users.AddressDetails) *Actor {
		u := &users.User{ID: id, Phone: phone, Role: role, EntityID: entity,
			PhoneVerified: true, CreatedAt: time.Now(), Address: addr}
		if err := w.Users.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		tok, err := w.Tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		return &Actor{Name: name, Token: tok, UserID: id, Phone: phone, Entity: entity}
	}
	home := users.AddressDetails{Lat: ChustLat + 0.002, Lng: ChustLng + 0.002, Text: "Chust, Navoiy ko'chasi 5"}
	w.Admin = mk("admin", "e2e-admin", users.RoleAdmin, "", "+998900000001", users.AddressDetails{})
	w.Customer = mk("mijoz", "e2e-cust", users.RoleCustomer, "", "+998900000002", home)
	w.Other = mk("begona-mijoz", "e2e-cust-2", users.RoleCustomer, "", "+998900000003", home)
	w.Owner = mk("restoran-A", "e2e-owner-a", users.RoleRestaurant, RestA, "+998900000004", users.AddressDetails{})
	w.OwnerB = mk("restoran-B", "e2e-owner-b", users.RoleRestaurant, RestB, "+998900000005", users.AddressDetails{})

	ssoSvc, ssoErr := sso.New([]sso.Client{{
		ID: SSOClientID, Name: "OnDex Merchant Panel", Secret: SSOClientSecret,
		RedirectURIs: []string{SSORedirectURI},
	}}, w.Users)
	if ssoErr != nil {
		t.Fatalf("SSO sozlamasi: %v", ssoErr)
	}

	qurtProducts := storage.NewMemoryQurtRepo()
	api := httpapi.New(httpapi.Deps{
		SSO:          ssoSvc,
		QurtRepo:     qurtProducts,
		QurtOrderSvc: &qurtorders.Service{Repo: storage.NewMemoryQurtOrderRepo(), Products: qurtProducts, NewID: httpapi.NewID},
		UserRepo:     w.Users, OrderRepo: w.Orders, CourierRepo: w.Couriers,
		CatalogRepo: cachedCatalog, CatalogSvc: catalog.NewService(cachedCatalog),
		PromotionsRepo: promo, FavoritesRepo: storage.NewMemoryFavoritesRepo(),
		BannerRepo: storage.NewMemoryBannerRepo(), RatingsRepo: storage.NewMemoryRatingRepo(),
		Devices: storage.NewMemoryDeviceStore(),
		Tokens:  w.Tokens, Revoked: revoked, Hub: hub, WsTickets: ws.NewTicketStore(),
		StatsSource: w.Orders, HistorySource: w.Orders, TableOrders: w.Orders,
		AlertsSvc:     alertsSvc,
		Notifications: notifStore, PushTokens: tokenStore, Notifier: notifSvc,
		SupportSvc: support.NewService(storage.NewMemorySupportStore(), hub, httpapi.NewID),
		WalletSvc:  walletSvc,
		AuthSvc:    users.NewService(w.Users, storage.NewMemoryCodeStore(), smsLog{}, w.Tokens, httpapi.NewID),
		OrderSvc:   orderSvc, Dispatcher: dispatcher,
		StaffSvc: staffSvc, TableSvc: tables.NewService(storage.NewMemoryTableRepo()),
		Payments: paySvc, MC: reg,
		ImageStore:       images.NewLocalStore(t.TempDir()),
		DevMode:          true,
		PlatformCouriers: opts.PlatformCouriers,
	})
	w.api = api
	w.Srv = httptest.NewServer(api.Routes(nil))
	t.Cleanup(w.Srv.Close)
	return w
}

// ─── So'rovlar ─────────────────────────────────────────────────────────

// now — xizmat soati: haqiqiy vaqt + Advance siljishi.
func (w *World) now() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return time.Now().Add(w.skew)
}

// Advance — xizmat soatini oldinga suradi (kutmasdan 10 daqiqalik muddatlarni sinash).
func (w *World) Advance(d time.Duration) {
	w.mu.Lock()
	w.skew += d
	w.mu.Unlock()
}

// Sweep — kuryer qidiruvi NAZORATCHISINING bitta tsiklini ishga tushiradi
// (production'da har 15 soniyada; birinchi tsikl darhol ishlaydi).
func (w *World) Sweep(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	w.api.RunDispatchWatchdog(ctx, w.Orders)
	time.Sleep(150 * time.Millisecond)
	cancel()
}

// R — natijalar yig'uvchisi (odatda umumiy Rec).
func (w *World) R() *report.Recorder {
	if w.rec != nil {
		return w.rec
	}
	return Rec
}

// Resp — bitta javob.
type Resp struct {
	Status int
	Body   []byte
	Ms     float64
	Header http.Header
}

// Do — so'rov yuboradi, vaqtini o'lchaydi, hisobotga yozadi va HAR
// javobga umumiy invariantlarni qo'llaydi (5xx yo'q, sir sizmaydi).
func (w *World) Do(t *testing.T, a *Actor, method, path string, body any) *Resp {
	t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	case []byte:
		rd = bytes.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("so'rov tanasi: %v", err)
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, w.Srv.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	if rd != nil {
		ct := "application/json"
		if w.ctype != "" {
			ct = w.ctype
		}
		req.Header.Set("Content-Type", ct)
	}
	actor := "anonim"
	if a != nil {
		actor = a.Name
		if a.Token != "" {
			req.Header.Set("Authorization", "Bearer "+a.Token)
		}
		for k, v := range a.Headers {
			req.Header.Set(k, v)
		}
	}
	start := time.Now()
	res, err := w.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	ms := float64(time.Since(start).Microseconds()) / 1000

	pathOnly, _, _ := strings.Cut(path, "?")
	Rec.AddCall(report.Call{
		Scenario: w.scenario, Step: w.step, Method: method, Path: pathOnly,
		Route: MatchRoute(Routes, method, pathOnly), Status: res.StatusCode, Ms: ms, Actor: actor,
		Poll: w.polling, Gated: w.gated,
	})
	r := &Resp{Status: res.StatusCode, Body: raw, Ms: ms, Header: res.Header}
	w.checkGeneric(t, method, path, r)

	// Kirgan foydalanuvchi javobi umumiy (CDN) keshga tushmasligi shart.
	if a != nil && a.Token != "" {
		w.R().Checked(InvNoPublicCacheAuthed)
		if cc := res.Header.Get("Cache-Control"); strings.Contains(strings.ToLower(cc), "public") {
			w.Violate(t, InvNoPublicCacheAuthed, "%s %s: tokenli so'rov javobida Cache-Control=%q", method, path, cc)
		}
	}
	return r
}

// Gated — TASHQI XIZMATGA bog'liq endpoint (Google xaritasi, Telegram,
// Firebase, Gemini, Mongo kitoblari...). E2E dunyosida bu xizmatlar
// ATAYLAB ulanmagan, va endpoint "fail closed" qilishi shart: 503 (yoki
// aytilgan boshqa kod) qaytaradi, panika yoki 500 emas, sir sizdirmaydi.
//
// Hisobotda alohida ko'rinadi: bu "ishlaydi" emas, "xavfsiz yopiq" degani.
// Haqiqiy ishlashi jonli (live) sinovda tekshiriladi.
func (w *World) Gated(t *testing.T, a *Actor, method, path string, body any, allowed ...int) *Resp {
	t.Helper()
	if len(allowed) == 0 {
		allowed = []int{http.StatusServiceUnavailable}
	}
	w.gated = true
	defer func() { w.gated = false }()
	r := w.Do(t, a, method, path, body)
	w.R().Checked(InvGatedContract)
	for _, s := range allowed {
		if r.Status == s {
			return r
		}
	}
	w.Violate(t, InvGatedContract, "%s %s -> %d, %v kutilgan: %s", method, path, r.Status, allowed, truncate(string(r.Body), 120))
	return r
}

// JSON — javobni ochadi.
func (r *Resp) JSON(t *testing.T, into any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, into); err != nil {
		t.Fatalf("javob JSON emas: %v — %s", err, r.Body)
	}
}

// Expect — kutilgan holat kodi; aks holda javob matni bilan yiqiladi.
func (r *Resp) Expect(t *testing.T, want int) *Resp {
	t.Helper()
	if r.Status != want {
		t.Fatalf("holat %d, %d kutilgan — %s", r.Status, want, truncate(string(r.Body), 400))
	}
	return r
}

// Map — javobni obyekt sifatida.
func (r *Resp) Map(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	r.JSON(t, &m)
	return m
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ─── Qadamlar ──────────────────────────────────────────────────────────

// Step — ssenariy qadami (subtest). Qadam yiqilsa keyingilari o'tkazib
// yuboriladi: oldingi holatga tayangan qadamlar shovqinli yolg'on xato
// bermasin. Har qadam vaqti, so'rovlar soni va natijasi hisobotga tushadi.
func (w *World) Step(t *testing.T, name string, fn func(t *testing.T)) {
	t.Helper()
	if w.stopped {
		Rec.AddStep(report.Step{Scenario: w.scenario, Name: name, Skipped: true})
		t.Run(name, func(t *testing.T) { t.Skip("oldingi qadam yiqilgan") })
		return
	}
	w.step = name
	before := Rec.CallCount()
	start := time.Now()
	ok := t.Run(name, fn)
	st := report.Step{
		Scenario: w.scenario, Name: name, Passed: ok,
		Ms:    float64(time.Since(start).Microseconds()) / 1000,
		Calls: Rec.CallCount() - before,
	}
	if !ok {
		st.Error = "qadam yiqildi (tafsilot go test chiqishida)"
		w.stopped = true
	}
	Rec.AddStep(st)
	w.step = ""
}

// Violate — invariant buzilishini hisobotga yozadi va testni yiqitadi.
func (w *World) Violate(t *testing.T, invariant, format string, args ...any) {
	t.Helper()
	detail := fmt.Sprintf(format, args...)
	w.R().AddViolation(report.Violation{Scenario: w.scenario, Step: w.step, Invariant: invariant, Detail: detail})
	t.Errorf("INVARIANT %s buzildi: %s", invariant, detail)
}

// Eventually — shart bajarilguncha (fon jarayonlari: dispatch) kutadi.
func (w *World) Eventually(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	w.polling = true
	defer func() { w.polling = false }()
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("%s: %v ichida bajarilmadi", what, timeout)
}

// LoginAs — telefon bo'yicha mavjud akkaunt uchun token (xodim akkauntlari).
func (w *World) LoginAs(t *testing.T, name, phone string) *Actor {
	t.Helper()
	u, err := w.Users.GetByPhone(context.Background(), phone)
	if err != nil {
		t.Fatalf("%s akkaunti topilmadi (%s): %v", name, phone, err)
	}
	tok, err := w.Tokens.Issue(u)
	if err != nil {
		t.Fatal(err)
	}
	return &Actor{Name: name, Token: tok, UserID: u.ID, Phone: phone, Entity: u.EntityID}
}
