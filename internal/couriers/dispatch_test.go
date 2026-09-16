package couriers

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"chustapp/internal/geo"
)

type fakeRepo struct {
	mu        sync.Mutex
	couriers  []*Courier
	available map[string]bool
}

func newFakeRepo(ids ...string) *fakeRepo {
	r := &fakeRepo{available: make(map[string]bool)}
	for i, id := range ids {
		r.couriers = append(r.couriers, &Courier{
			ID: id, Available: true, Approved: true, Rating: 5.0, VehicleType: VehicleMoped,
			// Nolmas (haqiqiy) koordinata — aks holda dispatch.go'dagi
			// "joylashuvi noma'lum (0,0)" filtri bularni ham chetlab qo'yardi.
			Lat: 41.0 + float64(i)*0.001, Lng: 71.0 + float64(i)*0.001,
		})
		r.available[id] = true
	}
	return r
}

func (r *fakeRepo) Create(_ context.Context, c *Courier) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.couriers = append(r.couriers, c)
	r.available[c.ID] = c.Available
	return nil
}

func (r *fakeRepo) ListAll(_ context.Context) ([]*Courier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*Courier(nil), r.couriers...), nil
}

func (r *fakeRepo) SetApproved(_ context.Context, id string, approved bool) error {
	for _, c := range r.couriers {
		if c.ID == id {
			c.Approved = approved
			return nil
		}
	}
	return ErrNoCourier
}

func (r *fakeRepo) SoftDelete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, c := range r.couriers {
		if c.ID == id {
			r.couriers = append(r.couriers[:i], r.couriers[i+1:]...)
			return nil
		}
	}
	return ErrNoCourier
}

func (r *fakeRepo) GetByID(_ context.Context, id string) (*Courier, error) {
	for _, c := range r.couriers {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, ErrNoCourier
}

func (r *fakeRepo) ListAvailable(_ context.Context) ([]*Courier, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*Courier
	for _, c := range r.couriers {
		if r.available[c.ID] {
			out = append(out, c)
		}
	}
	return out, nil
}

// ListAvailableNear — testlarda radius/eskilik CHEKLOVSIZ.
//
// NEGA: bu fayldagi testlar dispatch NAVBAT mantig'ini sinaydi
// (taklif -> rad -> keyingisi -> qabul), geografik filtrni emas.
// Filtrning o'zi `storage` paketida, haqiqiy Postgres/xotira
// implementatsiyalari ustida sinaladi (`courier_geo_test.go`).
// Bu yerda radius qo'llansa, testlar kuryerlarga soxta koordinata
// berishga majbur bo'lardi va sinalayotgan narsa xiralashardi.
//
// Havuz filtri esa SHU YERDA ham qo'llanadi: u geografiya emas,
// xavfsizlik chegarasi (restoran kuryeri begona buyurtmani ko'rmasin).
func (r *fakeRepo) ListAvailableNear(ctx context.Context, pool string, _, _ float64,
	_ float64, _ time.Duration, limit int) ([]*Courier, error) {
	all, err := r.ListAvailable(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*Courier, 0, len(all))
	for _, c := range all {
		if c.RestaurantID == pool {
			out = append(out, c)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *fakeRepo) SetName(_ context.Context, id, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range r.couriers {
		if c.ID == id {
			c.Name = name
			return nil
		}
	}
	return ErrNoCourier
}

func (r *fakeRepo) SetAvailable(_ context.Context, id string, v bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.available[id] = v
	return nil
}

func (r *fakeRepo) ClaimIfAvailable(_ context.Context, id string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.available[id] {
		return false, nil
	}
	// Haqiqiy implementatsiyalar kabi `Approved` ham tekshiriladi.
	for _, c := range r.couriers {
		if c.ID == id && !c.Approved {
			return false, nil
		}
	}
	r.available[id] = false
	return true, nil
}

func (r *fakeRepo) UpdateLocation(_ context.Context, id string, lat, lng float64) error {
	for _, c := range r.couriers {
		if c.ID == id {
			c.Lat = lat
			c.Lng = lng
			return nil
		}
	}
	return ErrNoCourier
}

func (r *fakeRepo) IncrementCompletedOrders(_ context.Context, id string) error {
	for _, c := range r.couriers {
		if c.ID == id {
			c.CompletedOrders++
			return nil
		}
	}
	return ErrNoCourier
}

// fakeNotifier — kimlarga taklif yuborilgani va kimga bekor qilingani yozib boradi.
type fakeNotifier struct {
	mu        sync.Mutex
	offers    []string
	cancelled []string
}

func (n *fakeNotifier) SendOffer(courierID string, _ OfferInfo) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.offers = append(n.offers, courierID)
}
func (n *fakeNotifier) CancelOffer(courierID, _ string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.cancelled = append(n.cancelled, courierID)
}

func (n *fakeNotifier) offered() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string(nil), n.offers...)
}

// offerCount — shu kuryerga necha marta taklif yuborilgan.
func (n *fakeNotifier) offerCount(id string) int {
	n.mu.Lock()
	defer n.mu.Unlock()
	k := 0
	for _, o := range n.offers {
		if o == id {
			k++
		}
	}
	return k
}

func (n *fakeNotifier) wasCancelled(id string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, c := range n.cancelled {
		if c == id {
			return true
		}
	}
	return false
}

// fakeGeoClient — Google Distance Matrix'ni taqlid qiladi: har bir kuryer
// ID uchun oldindan belgilangan ETA qaytaradi (haqiqiy tarmoq so'rovisiz).
type fakeGeoClient struct {
	mu   sync.Mutex
	etas map[string]time.Duration
	fail bool
	// notOK — shu ID'lar uchun element darajasida `OK=false` qaytariladi
	// (Google'ning ZERO_RESULTS javobi), lekin so'rovning O'ZI muvaffaqiyatli
	// bo'ladi. Aynan shu holat production'da yetkazishni to'xtatib qo'ygan.
	notOK map[string]bool
	calls int
}

func newFakeGeoClient(etas map[string]time.Duration) *fakeGeoClient {
	return &fakeGeoClient{etas: etas}
}

func (g *fakeGeoClient) FetchETAs(_ context.Context, _ geo.LatLng, candidates []geo.Candidate) ([]geo.Result, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls++
	if g.fail {
		return nil, errors.New("simulyatsiya qilingan Google API xatosi")
	}
	results := make([]geo.Result, len(candidates))
	for i, c := range candidates {
		if g.notOK[c.ID] {
			// Google ZERO_RESULTS qaytardi: ID bor, ETA yo'q.
			results[i] = geo.Result{ID: c.ID, OK: false}
			continue
		}
		eta, ok := g.etas[c.ID]
		if !ok {
			eta = 10 * time.Minute
		}
		results[i] = geo.Result{ID: c.ID, OK: true, Duration: eta, Distance: eta.Minutes() * 300}
	}
	return results, nil
}

// waitOffered — kuryerga taklif yetib borishini kutadi.
//
// AVVAL `lastOffered() == id` ni tekshirardi — bu KETMA-KET rejimda
// to'g'ri edi (bir vaqtda bitta taklif ochiq turardi). TO'LQINLI
// rejimda esa bir necha taklif ketma-ket jo'natiladi va "oxirgisi"
// kutilgan kuryer bo'lmasligi mumkin. Endi "shu kuryerga taklif
// KELDIMI" tekshiriladi.
func waitOffered(t *testing.T, n *fakeNotifier, id string) {
	t.Helper()
	waitOfferedAtLeast(t, n, id, 1)
}

// waitOfferedAtLeast — kuryerga KAMIDA `want` marta taklif kelishini
// kutadi (qayta urinish tsikllarini sinash uchun).
func waitOfferedAtLeast(t *testing.T, n *fakeNotifier, id string, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if n.offerCount(id) >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Errorf("kuryerga (%s) %d-taklif yetib bormadi (kelganlari: %v)",
		id, want, n.offered())
}

const defaultOfferTTL = 300 * time.Millisecond

func TestDispatchOffersInScoreOrderBestFirst(t *testing.T) {
	repo := newFakeRepo("far", "ideal", "close")
	n := &fakeNotifier{}
	// prep_time=20min -> ideal oyna 5-10 daqiqa. "ideal" ANIQ shu oynada,
	// eng yaxshi ball unga tegishli bo'lishi kerak.
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"far":   25 * time.Minute,
		"ideal": 7 * time.Minute,
		"close": 1 * time.Minute,
	})
	d := NewDispatcher(repo, n, geoClient, defaultOfferTTL)

	go func() {
		waitOffered(t, n, "ideal")
		d.HandleResponse("o1", Response{CourierID: "ideal", Accepted: true})
	}()

	got, err := d.Dispatch(context.Background(), "o1", DispatchParams{PreparationTime: 20 * time.Minute})
	if err != nil || got != "ideal" {
		t.Fatalf("kutilgan 'ideal', olindi %q, xato: %v", got, err)
	}
	if repo.available["ideal"] {
		t.Error("qabul qilgan kuryer band bo'lishi kerak edi")
	}
}

func TestDispatchMovesToNextOnReject(t *testing.T) {
	repo := newFakeRepo("first", "second")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"first":  7 * time.Minute, // ideal oynada — birinchi taklif shunga boradi
		"second": 8 * time.Minute,
	})
	d := NewDispatcher(repo, n, geoClient, defaultOfferTTL)

	go func() {
		waitOffered(t, n, "first")
		d.HandleResponse("o1", Response{CourierID: "first", Accepted: false})
		waitOffered(t, n, "second")
		d.HandleResponse("o1", Response{CourierID: "second", Accepted: true})
	}()

	got, err := d.Dispatch(context.Background(), "o1", DispatchParams{PreparationTime: 20 * time.Minute})
	if err != nil || got != "second" {
		t.Fatalf("kutilgan 'second', olindi %q, xato: %v", got, err)
	}
}

func TestDispatchMovesToNextOnTimeout(t *testing.T) {
	repo := newFakeRepo("slow", "responsive")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"slow":       7 * time.Minute,
		"responsive": 8 * time.Minute,
	})
	d := NewDispatcher(repo, n, geoClient, 60*time.Millisecond)

	go func() {
		waitOffered(t, n, "slow")
		// "slow" hech qanday javob bermaydi — timeout kutiladi.
		waitOffered(t, n, "responsive")
		d.HandleResponse("o1", Response{CourierID: "responsive", Accepted: true})
	}()

	got, err := d.Dispatch(context.Background(), "o1", DispatchParams{PreparationTime: 20 * time.Minute})
	if err != nil || got != "responsive" {
		t.Fatalf("kutilgan 'responsive', olindi %q, xato: %v", got, err)
	}
	if !n.wasCancelled("slow") {
		t.Error("muddati tugagan kuryerga offer_cancelled yuborilishi kerak edi")
	}
}

// G'olib topilgach KEYINGI TO'LQIN yuborilmasligi kerak.
//
// DIQQAT — semantika o'zgardi: to'lqinli rejimda bitta to'lqindagi
// HAMMA nomzod taklif oladi (bir vaqtda). Shuning uchun "qolganlarga
// umuman yuborilmasin" degan eski shart endi noto'g'ri. To'g'ri shart:
// g'olib aniqlangach KEYINGI to'lqinga o'tilmasin.
func TestDispatchStopsAfterAcceptDoesNotOfferNextWave(t *testing.T) {
	// waveSize+1 ta nomzod: oxirgisi ikkinchi to'lqinda bo'ladi.
	repo := newFakeRepo("w1", "w2", "w3", "next_wave")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"w1":        7 * time.Minute,
		"w2":        8 * time.Minute,
		"w3":        9 * time.Minute,
		"next_wave": 30 * time.Minute, // eng yomon ball — ikkinchi to'lqin
	})
	d := NewDispatcher(repo, n, geoClient, defaultOfferTTL)

	go func() {
		waitOffered(t, n, "w1")
		d.HandleResponse("o1", Response{CourierID: "w1", Accepted: true})
	}()

	got, err := d.Dispatch(context.Background(), "o1", DispatchParams{PreparationTime: 20 * time.Minute})
	if err != nil || got != "w1" {
		t.Fatalf("kutilgan 'w1', olindi %q, xato: %v", got, err)
	}
	time.Sleep(50 * time.Millisecond) // qo'shimcha taklif ketmasligiga ishonch
	if n.offerCount("next_wave") != 0 {
		t.Error("g'olib topilgach KEYINGI TO'LQIN yuborilmasligi kerak edi")
	}
	// Birinchi to'lqindagi qolganlarga taklif BEKOR qilinishi kerak.
	for _, id := range []string{"w2", "w3"} {
		if !n.wasCancelled(id) {
			t.Errorf("to'lqindagi %s ga bekor qilish yuborilmadi", id)
		}
	}
}

func TestHandleResponseRejectsStaleOrWrongCandidate(t *testing.T) {
	// waveSize+1 ta: "wave2" birinchi to'lqinga TUSHMAYDI.
	repo := newFakeRepo("c1", "c2", "c3", "wave2")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{
		"c1": 7 * time.Minute, "c2": 8 * time.Minute,
		"c3": 9 * time.Minute, "wave2": 30 * time.Minute,
	})
	d := NewDispatcher(repo, n, geoClient, 60*time.Millisecond)

	done := make(chan struct{})
	go func() {
		defer close(done)
		got, _ := d.Dispatch(context.Background(), "o1", DispatchParams{PreparationTime: 20 * time.Minute})
		if got != "wave2" {
			t.Errorf("kutilgan wave2, olindi %q", got)
		}
	}()

	waitOffered(t, n, "c1")
	// "wave2" hali BIRINCHI TO'LQINDA emas — javobi rad etilishi kerak.
	if d.HandleResponse("o1", Response{CourierID: "wave2", Accepted: true}) {
		t.Error("to'lqinda bo'lmagan kuryerning javobi qabul qilinmasligi kerak edi")
	}
	// Noma'lum buyurtma uchun javob ham rad etiladi.
	if d.HandleResponse("yoq-order", Response{CourierID: "c1", Accepted: true}) {
		t.Error("mavjud bo'lmagan buyurtmaga javob qabul bo'lmasligi kerak edi")
	}
	// Birinchi to'lqin javob bermaydi -> timeout -> ikkinchi to'lqin.
	waitOffered(t, n, "wave2")
	if !d.HandleResponse("o1", Response{CourierID: "wave2", Accepted: true}) {
		t.Error("to'lqindagi kuryerning javobi qabul qilinishi kerak edi")
	}
	<-done
}

// TestDispatchRetriesForeverUntilAccepted — YANGI xulq (foydalanuvchi
// so'rovi, 2026-07-30): restoranda "qayta urinish" tugmasi umuman yo'q —
// hammasi bir marta rad etsa/javob bermasa ham, dispatch TASLIM
// BO'LMAYDI, qisqa pauzadan so'ng xuddi shu (hamon onlayn) nomzodlarga
// YANA taklif yuboradi, to kimdir qabul qilguncha.
func TestDispatchRetriesForeverUntilAccepted(t *testing.T) {
	repo := newFakeRepo("c1", "c2")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{"c1": 7 * time.Minute, "c2": 8 * time.Minute})
	d := NewDispatcher(repo, n, geoClient, 30*time.Millisecond)

	go func() {
		// Birinchi tsikl: ikkalasi ham rad etadi.
		waitOffered(t, n, "c1")
		d.HandleResponse("o1", Response{CourierID: "c1", Accepted: false})
		waitOffered(t, n, "c2")
		d.HandleResponse("o1", Response{CourierID: "c2", Accepted: false})
		// Ikkinchi tsikl (avtomatik qayta urinish) — "c1" YANA taklif
		// oladi (qo'lda hech narsa bosilmadi!), bu safar qabul qiladi.
		// IKKINCHI taklifni kutamiz: to'lqinli rejimda birinchi taklif
		// allaqachon kelgan, shuning uchun "kelganmi?" yetarli emas.
		waitOfferedAtLeast(t, n, "c1", 2)
		d.HandleResponse("o1", Response{CourierID: "c1", Accepted: true})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	got, err := d.Dispatch(ctx, "o1", DispatchParams{PreparationTime: 20 * time.Minute})
	if err != nil || got != "c1" {
		t.Fatalf("ikkinchi tsiklda 'c1' qabul qilishi kerak edi, olindi %q, xato: %v", got, err)
	}
}

// TestDispatchReusesRankingWhenPoolUnchanged — nomzodlar havuzi
// o'zgarmagan bo'lsa, har bir qayta urinish tsiklida PULLIK Google
// Distance Matrix so'rovi TAKRORLANMASLIGI kerak.
//
// Avval takrorlanardi: hech kim qabul qilmaydigan bitta buyurtma har 5
// sekundda bir marta, ya'ni soatiga ~720 marta tashqi API'ni chaqirardi.
func TestDispatchReusesRankingWhenPoolUnchanged(t *testing.T) {
	repo := newFakeRepo("c1", "c2")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{"c1": 7 * time.Minute, "c2": 8 * time.Minute})
	d := NewDispatcher(repo, n, geoClient, 20*time.Millisecond)

	// Hech kim javob bermaydi — bir necha tsikl aylanib, keyin to'xtaydi.
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	_, _ = d.Dispatch(ctx, "o1", DispatchParams{PreparationTime: 20 * time.Minute})

	geoClient.mu.Lock()
	calls := geoClient.calls
	geoClient.mu.Unlock()
	if calls != 1 {
		t.Errorf("havuz o'zgarmaganda ETA bir marta so'ralishi kerak edi, so'raldi %d marta", calls)
	}
}

// TestBackoffGrowsAndIsCapped — ketma-ket muvaffaqiyatsiz tsikllarda
// pauza ikki baravar oshadi, lekin `maxRetryPause`dan oshmaydi.
func TestBackoffGrowsAndIsCapped(t *testing.T) {
	d := NewDispatcher(newFakeRepo(), &fakeNotifier{}, newFakeGeoClient(nil), 20*time.Second)
	base := d.retryPause() // 5s
	if got := d.backoffPause(1); got != base {
		t.Errorf("1-tsikl: %v kutilgan, olindi %v", base, got)
	}
	if got := d.backoffPause(2); got != 2*base {
		t.Errorf("2-tsikl: %v kutilgan, olindi %v", 2*base, got)
	}
	if got := d.backoffPause(3); got != 4*base {
		t.Errorf("3-tsikl: %v kutilgan, olindi %v", 4*base, got)
	}
	if got := d.backoffPause(50); got != d.maxRetryPause() {
		t.Errorf("chegara: %v kutilgan, olindi %v", d.maxRetryPause(), got)
	}
}

// TestDispatchStopsWhenContextCancelled — hech kim hech qachon qabul
// qilmasa (yoki umuman onlayn kuryer bo'lmasa), Dispatch faqat `ctx`
// bekor qilinganda to'xtaydi — o'zidan-o'zi "kuryer topilmadi" deb
// taslim bo'lmaydi.
func TestDispatchStopsWhenContextCancelled(t *testing.T) {
	repo := newFakeRepo() // onlayn kuryer umuman yo'q
	geoClient := newFakeGeoClient(nil)
	d := NewDispatcher(repo, &fakeNotifier{}, geoClient, 20*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := d.Dispatch(ctx, "o1", DispatchParams{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("context.DeadlineExceeded kutilgan edi, olindi: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("juda tez to'xtadi (%v) — qayta urinish umuman bo'lmadimi?", elapsed)
	}
}

// TestDispatchStopsWhenOrderCancelled — IsOrderCancelled true qaytarsa,
// qayta urinish tsikli darhol to'xtaydi (ctx tugashini kutmasdan).
func TestDispatchStopsWhenOrderCancelled(t *testing.T) {
	repo := newFakeRepo() // onlayn kuryer yo'q — darhol IsOrderCancelled tekshiruviga o'tadi
	geoClient := newFakeGeoClient(nil)
	d := NewDispatcher(repo, &fakeNotifier{}, geoClient, 20*time.Millisecond)

	_, err := d.Dispatch(context.Background(), "o1", DispatchParams{
		IsOrderCancelled: func(context.Context) (bool, error) { return true, nil },
	})
	if !errors.Is(err, ErrOrderCancelled) {
		t.Fatalf("ErrOrderCancelled kutilgan edi, olindi: %v", err)
	}
}

// TestDispatchFallsBackToHaversineOnGeoError — Google API xato bersa ham,
// dispatch to'xtab qolmasligi, zaxira (to'g'ri chiziq) ETA bilan davom
// etishini tekshiradi.
func TestDispatchFallsBackToHaversineOnGeoError(t *testing.T) {
	repo := newFakeRepo("c1")
	repo.couriers[0].Lat, repo.couriers[0].Lng = 41.001, 71.001
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(nil)
	geoClient.fail = true
	d := NewDispatcher(repo, n, geoClient, defaultOfferTTL)

	go func() {
		waitOffered(t, n, "c1")
		d.HandleResponse("o1", Response{CourierID: "c1", Accepted: true})
	}()

	got, err := d.Dispatch(context.Background(), "o1", DispatchParams{
		RestaurantLocation: geo.LatLng{Lat: 41.0, Lng: 71.0},
		PreparationTime:    20 * time.Minute,
	})
	if err != nil || got != "c1" {
		t.Fatalf("Google API xatosida ham zaxira ETA bilan dispatch muvaffaqiyatli bo'lishi kerak edi, olindi %q, xato: %v", got, err)
	}
}

func TestDispatchAlreadyRunning(t *testing.T) {
	repo := newFakeRepo("c1")
	n := &fakeNotifier{}
	geoClient := newFakeGeoClient(map[string]time.Duration{"c1": 7 * time.Minute})
	d := NewDispatcher(repo, n, geoClient, 200*time.Millisecond)

	go d.Dispatch(context.Background(), "o1", DispatchParams{PreparationTime: 20 * time.Minute})
	waitOffered(t, n, "c1")

	if _, err := d.Dispatch(context.Background(), "o1", DispatchParams{}); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("ErrAlreadyRunning kutilgan edi, olindi: %v", err)
	}
	d.HandleResponse("o1", Response{CourierID: "c1", Accepted: true})
}
