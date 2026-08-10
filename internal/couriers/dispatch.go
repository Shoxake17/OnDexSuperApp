package couriers

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"chustapp/internal/geo"
)

// Dispatch oqimi — Yandex Eats uslubidagi TO'LIQ AVTOMATLASHTIRILGAN
// matching engine (ketma-ket, ETA-asoslangan navbat modeli, CHEKSIZ
// qayta urinish bilan — restoran tomonidan qo'lda "qayta urinish"
// tugmasi UMUMAN yo'q, foydalanuvchi so'rovi bo'yicha):
//  1. Barcha ONLAYN (tasdiqlangan VA bo'sh) kuryerlar — bu nomzodlar
//     HAVUZI, masofadan qat'i nazar.
//  2. Har bir nomzod uchun Google Distance Matrix API orqali HAQIQIY yo'l
//     bo'yicha ETA hisoblanadi (transport turiga mos rejim bilan: piyoda,
//     velosiped, moped/mashina).
//  3. ScoreCandidates orqali ETA-vs-tayyorlash-vaqti oynasi + reyting +
//     tajriba asosida SARALANADI (scoring.go).
//  4. Taklif ENG YAXSHI ballga ega BITTA kuryerga yuboriladi — 20 soniya
//     (offerTTL) kutiladi.
//  5. Qabul qilsa — g'olib, dispatch tugaydi. Rad etsa YOKI javob bermasa
//     (timeout) — avtomatik ravishda NAVBATDAGI eng yaxshi nomzodga
//     o'tiladi, va hokazo, ro'yxat tugaguncha.
//  6. RO'YXAT TUGASA (hammasi rad etdi/javob bermadi) YOKI umuman onlayn
//     kuryer topilmasa — dispatch TO'XTAMAYDI. Qisqa pauzadan so'ng
//     kuryerlar ro'yxati QAYTA so'raladi (avval rad etgan kuryer ham,
//     agar hamon onlayn bo'lsa, qayta nomzod bo'ladi — offline
//     kuryerlar orasidan navbat davom etaveradi) va butun tsikl
//     TAKRORLANADI — TO BIRON KURYER QABUL QILGUNCHA. To'xtash faqat
//     ikki holatda: (a) buyurtma boshqa sabab bilan terminal holatga
//     o'tsa (`DispatchParams.IsOrderCancelled`), (b) `ctx` bekor qilinsa.
//
// Bir vaqtning o'zida FAQAT BITTA kuryerga taklif ochiq turadi (avvalgi
// "broadcast" modelidan farqli) — shuning uchun HandleResponse'da
// "g'olib"ni aniqlash uchun murakkab mutex-race himoyasi shart emas: shunchaki
// javob AYNAN hozir so'ralayotgan kuryerga tegishlimi tekshiriladi.

var (
	// ErrNoCourier — endi Dispatch() ICHIDA hech qachon qaytarilmaydi
	// (cheksiz qayta urinadi), lekin boshqa joylarda (masalan fake
	// repolarda "topilmadi" xatosi sifatida) hamon ishlatiladi.
	ErrNoCourier      = errors.New("bo'sh kuryer topilmadi")
	ErrAlreadyRunning = errors.New("bu buyurtma uchun dispatch allaqachon ishlayapti")
	// ErrOrderCancelled — dispatch davomida buyurtma boshqa sabab bilan
	// (mijoz/admin bekor qildi, restoran rad etdi) terminal holatga
	// o'tgani aniqlanganda qaytariladi — qayta urinish shu yerda to'xtaydi.
	ErrOrderCancelled = errors.New("buyurtma bekor qilingan, dispatch to'xtatildi")
)

// Nomzodlarni tanlash chegaralari (migration 0029, PostGIS).
const (
	// searchRadiusMeters — restorandan shu masofagacha bo'lgan
	// kuryerlar nomzod bo'ladi.
	//
	// 7 km — Chust shahri va yaqin atrofini to'liq qoplaydi. Kattaroq
	// qilish ma'nosiz: undan naridagi kuryer yetib kelguncha ovqat
	// sovib qoladi va `ScoreCandidates` uni baribir eng pastga
	// tushirardi (faqat PULLIK ETA so'rovi bekorga sarflanardi).
	searchRadiusMeters = 7000

	// maxCandidates — bitta tsiklda ETA so'raladigan eng ko'p kuryer.
	//
	// Google Distance Matrix narxi nomzodlar soniga TO'G'RI proporsional.
	// 20 ta eng yaqin kuryerdan mos birini topib bo'lmasa, muammo
	// masofada emas (hammasi band yoki javob bermayapti) — ro'yxatni
	// uzaytirish yordam bermaydi, tsikl baribir qaytadan boshlanadi.
	maxCandidates = 20

	// locationMaxAge — joylashuv shundan eski bo'lsa kuryer nomzod
	// BO'LMAYDI.
	//
	// NEGA: ilovasi qotib qolgan yoki tarmoqdan uzilgan kuryer hamon
	// `available` bo'lib turadi. Uning ESKI koordinatasi bo'yicha ETA
	// hisoblanardi, taklif yuborilardi va 20 soniya (offerTTL) javob
	// kutilardi — buyurtma shuncha kechikardi. Kuryer ilovasi har
	// 5-10 soniyada joylashuv yuboradi, ya'ni 3 daqiqa juda bardoshli
	// chegara (tunnel/lift kabi qisqa uzilishlar o'tib ketadi).
	locationMaxAge = 3 * time.Minute

	// waveSize — bitta to'lqinda BIR VAQTDA taklif oladigan kuryerlar.
	//
	// ┌─ NEGA 3 ──────────────────────────────────────────────────────┐
	// Bu son ikki xato o'rtasidagi muvozanat:
	//
	//	juda KICHIK (1) — eski xatti-harakat: eng yomon holatda
	//	                  20 nomzod × TTL, ya'ni buyurtma daqiqalab
	//	                  turib qoladi;
	//	juda KATTA (20) — hamma bir vaqtda taklif ko'radi, bittasi
	//	                  yutadi, qolgan 19 tasi "bosdim-u ololmadim"
	//	                  degan asabiylikni oladi. Kuryerlar bunday
	//	                  tizimga ishonchini yo'qotadi.
	//
	// 3 ta — bir vaqtda uch kishi ko'radi, ya'ni javob berish ehtimoli
	// uch barobar, "yutqazgan"lar soni esa eng ko'pi bilan 2 ta.
	// Yandex/Uber ham shu tartibdagi kichik to'lqinlarni ishlatadi.
	// └───────────────────────────────────────────────────────────────┘
	waveSize = 3
)

// OfferNotifier — kuryerga taklifni yetkazadi (WebSocket, keyinchalik FCM push).
type OfferNotifier interface {
	SendOffer(courierID string, info OfferInfo)
	CancelOffer(courierID, orderID string)
}

// OfferInfo — kuryer ilovasi taklif kartochkasini (restoran nomi, manzili,
// logotipi, ETA hisoblash uchun koordinata) darhol, qo'shimcha so'rovsiz
// ko'rsata olishi uchun kerakli to'liq kontekst. Kuryer hali buyurtmaga
// BIRIKTIRILMAGAN (CourierID bo'sh) bo'lgani uchun GET /orders/{id} orqali
// bu ma'lumotni ololmaydi — shu sabab WebSocket xabarining o'zida keladi.
type OfferInfo struct {
	OrderID           string
	RestaurantID      string
	RestaurantName    string
	RestaurantAddress string
	RestaurantLat     float64
	RestaurantLng     float64
	RestaurantLogoURL string
	ExpiresIn         time.Duration
}

// GeoClient — ETA hisoblovchi tashqi xizmat (Google Distance Matrix).
// Interfeys sifatida — testlarda soxta implementatsiya bilan almashtiriladi,
// production'da *geo.Client shu interfeysni tabiiy ravishda qanoatlantiradi.
type GeoClient interface {
	FetchETAs(ctx context.Context, dest geo.LatLng, candidates []geo.Candidate) ([]geo.Result, error)
}

type Response struct {
	CourierID string
	Accepted  bool
}

// DispatchParams — ETA/ballash uchun zarur buyurtma konteksti.
type DispatchParams struct {
	RestaurantLocation geo.LatLng
	PreparationTime    time.Duration // restoran "Qabul qilindi" bosganda kiritgan taxminiy tayyorlash vaqti

	// Taklif kartochkasida ko'rsatish uchun (OfferInfo'ga shunchaki
	// o'tkaziladi — ballashda ishtirok etmaydi).
	RestaurantID      string
	RestaurantName    string
	RestaurantAddress string
	RestaurantLogoURL string

	// IsOrderCancelled — har bir qayta urinish tsikli oldidan tekshiriladi.
	// Dispatch endi hech qachon o'zi "hech kim topilmadi" deb to'xtamaydi
	// (cheksiz qayta urinadi), shuning uchun buyurtma boshqa sabab bilan
	// (mijoz/admin bekor qilishi, restoran rad etishi) terminal holatga
	// o'tganini BU YERDA tekshirmasak, dispatch abadiy (zombie) goroutine
	// bo'lib qolar edi. nil bo'lsa tekshiruv o'tkazib yuboriladi (testlar
	// uchun qulay).
	IsOrderCancelled func(ctx context.Context) (bool, error)
}

type Dispatcher struct {
	repo      Repository
	notifier  OfferNotifier
	geoClient GeoClient
	offerTTL  time.Duration // bitta kuryerga javob berish uchun ajratilgan vaqt

	mu      sync.Mutex
	pending map[string]*waveOffer
}

// waveOffer — bitta buyurtma uchun HOZIR ochiq turgan TO'LQIN.
//
// ┌─ NEGA TO'LQIN (avval bittalab edi) ───────────────────────────────┐
// Avval taklif ketma-ket, BITTALAB yuborilardi va har biriga 20
// soniya berilardi. Eng yomon holatda 20 nomzod × 20 s ≈ 6.7 daqiqa —
// buyurtma shuncha turib qolardi.
//
// Endi taklif bir vaqtda `waveSize` ta eng yaxshi nomzodga ketadi va
// KIM BIRINCHI QABUL QILSA — o'sha oladi. Hech kim javob bermasa,
// keyingi to'lqinga o'tiladi.
//
// Bu Yandex/Uber ishlatadigan naqsh: to'liq ro'yxatga birdan yuborish
// (hamma bosadi, bittasi yutadi — qolganlari asabiylashadi) bilan
// bittalab yuborish (sifatli, lekin sekin) o'rtasidagi muvozanat.
// └───────────────────────────────────────────────────────────────────┘
type waveOffer struct {
	respCh chan Response
	// wave — HOZIR taklif ochiq turgan kuryerlar (mutex ostida
	// almashtiriladi). Javob faqat SHU to'plamdagi kuryerdan qabul
	// qilinadi — eskirgan to'lqindan kechikib kelgan javob rad etiladi.
	wave map[string]struct{}
}

// inWave — mutex chaqiruvchida ushlab turiladi.
func (o *waveOffer) inWave(courierID string) bool {
	_, ok := o.wave[courierID]
	return ok
}

func NewDispatcher(repo Repository, notifier OfferNotifier, geoClient GeoClient, offerTTL time.Duration) *Dispatcher {
	return &Dispatcher{
		repo:      repo,
		notifier:  notifier,
		geoClient: geoClient,
		offerTTL:  offerTTL,
		pending:   make(map[string]*waveOffer),
	}
}

// vehicleToMode — kuryer transportini Google Distance Matrix "mode"
// parametriga xaritalaydi. Bo'sh/noma'lum qiymat (masalan eski yozuv) —
// xavfsiz standart taxmin sifatida "driving" (moped) qabul qilinadi.
func vehicleToMode(v VehicleType) geo.Mode {
	switch v {
	case VehicleFoot:
		return geo.ModeWalking
	case VehicleBike:
		return geo.ModeBicycling
	default:
		return geo.ModeDriving
	}
}

// retryPause — bitta to'liq tsikl (barcha nomzodlarga ketma-ket taklif)
// muvaffaqiyatsiz tugagach, keyingisini boshlashdan oldingi kutish vaqti.
// `offerTTL`ga nisbatan hisoblanadi (testlarda kichik offerTTL — tez
// pauza; production'da 20s offerTTL — 5s pauza) — alohida konfiguratsiya
// parametri shart emas.
func (d *Dispatcher) retryPause() time.Duration {
	p := d.offerTTL / 4
	if p < 50*time.Millisecond {
		p = 50 * time.Millisecond
	}
	return p
}

// maxRetryPause — qayta urinishlar orasidagi ENG UZUN pauza.
// `offerTTL`ga nisbatan (production'da 20s → 300s = 5 daqiqa).
func (d *Dispatcher) maxRetryPause() time.Duration { return d.offerTTL * 15 }

// backoffPause — ketma-ket muvaffaqiyatsiz tsikllar uchun pauza:
// 1-tsikldan keyin `retryPause`, keyin har safar IKKI BARAVAR, `maxRetryPause`
// gacha.
//
// NIMA UCHUN KERAK (moliyaviy bug): qayta urinish CHEKSIZ — bu ataylab
// tanlangan mahsulot xatti-harakati va o'zgarmaydi. Lekin doimiy 5
// sekundlik pauza bilan, hech kim qabul qilmayotgan bitta buyurtma
// soatiga ~720 marta tsikl aylantirardi va HAR TSIKLDA PULLIK Google
// Distance Matrix so'rovi yuborardi. Kechada bir necha "osilib qolgan"
// buyurtma butun API byudjetini yeb qo'yishi mumkin edi. Backoff
// buyurtmani baribir navbatda ushlab turadi, lekin so'rovlar sonini
// o'nlab barobar kamaytiradi. Yangi kuryer onlayn bo'lishi bilan
// backoff NOLGA qaytariladi — javob berish tezligi yo'qolmaydi.
func (d *Dispatcher) backoffPause(failedCycles int) time.Duration {
	p := d.retryPause()
	for i := 1; i < failedCycles && p < d.maxRetryPause(); i++ {
		p *= 2
	}
	if p > d.maxRetryPause() {
		p = d.maxRetryPause()
	}
	return p
}

// candidateKey — nomzodlar TO'PLAMINI (tartibsiz) bir qatorga jamlaydi.
// Ikki tsikl orasida bu kalit o'zgarmagan bo'lsa — havuz aynan o'sha,
// ya'ni ETA'ni qayta so'rashning ma'nosi yo'q (qarang: rankCache).
func candidateKey(cs []*Courier) string {
	ids := make([]string, 0, len(cs))
	for _, c := range cs {
		ids = append(ids, c.ID+":"+formatCoord(c.Lat)+","+formatCoord(c.Lng))
	}
	sort.Strings(ids)
	return strings.Join(ids, "|")
}

// formatCoord — koordinatani ~11 m aniqlikda yaxlitlaydi. Kuryer bir
// joyda turganda GPS'ning oxirgi xonalardagi shovqini keshni behuda
// buzmasligi uchun.
func formatCoord(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }

// Dispatch — bloklanuvchi chaqiruv: kimdir qabul qilguncha CHEKSIZ qayta
// urinadi (pastga qarang), faqat `ctx` bekor qilinsa yoki
// `params.IsOrderCancelled` true qaytarsa to'xtaydi. Chaqiruvchi buni
// goroutine'da ishga tushiradi.
func (d *Dispatcher) Dispatch(ctx context.Context, orderID string, params DispatchParams) (string, error) {
	d.mu.Lock()
	if _, exists := d.pending[orderID]; exists {
		d.mu.Unlock()
		return "", ErrAlreadyRunning
	}
	// Bufer — to'lqin hajmicha. Busiz bir vaqtda bosgan kuryerlarning
	// javobi TASHLAB YUBORILARDI (`HandleResponse` dagi `default`) va
	// ular ilovada "xatolik" ko'rardi.
	offer := &waveOffer{
		respCh: make(chan Response, waveSize),
		wave:   make(map[string]struct{}, waveSize),
	}
	d.pending[orderID] = offer
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(d.pending, orderID)
		d.mu.Unlock()
	}()

	// failedCycles — ketma-ket muvaffaqiyatsiz tugagan tsikllar soni
	// (backoff uchun). Nomzodlar havuzi o'zgarishi bilan nolga qaytadi.
	failedCycles := 0
	// rankCache — oxirgi tsiklda hisoblangan saralash va u qaysi nomzodlar
	// to'plamiga tegishli ekani. Havuz o'zgarmagan bo'lsa qayta
	// ishlatiladi — PULLIK Google Distance Matrix so'rovi takrorlanmaydi.
	var cachedKey string
	var cachedRank []ScoredCandidate

	for {
		if params.IsOrderCancelled != nil {
			cancelled, err := params.IsOrderCancelled(ctx)
			if err != nil {
				return "", err
			}
			if cancelled {
				return "", ErrOrderCancelled
			}
		}

		// Nomzodlar RESTORAN atrofidan, DB darajasida tanlanadi
		// (PostGIS `ST_DWithin` + GiST indeks, migration 0029).
		//
		// ┌─ NEGA RADIUS ─────────────────────────────────────────────┐
		// Avval `ListAvailable()` masofadan qat'i nazar HAMMA onlayn
		// kuryerni qaytarardi va har biriga PULLIK Google Distance
		// Matrix so'rovi ketardi. 20 km naridagi kuryer uchun ham ETA
		// hisoblanib, natija baribir tashlab yuborilardi.
		//
		// Jonli o'lchov (50 000 kuryer): eski usul 50 002 qator,
		// yangisi 20 qator — 2500 barobar kam nomzod.
		// └───────────────────────────────────────────────────────────┘
		candidates, err := d.repo.ListAvailableNear(ctx,
			params.RestaurantLocation.Lat, params.RestaurantLocation.Lng,
			searchRadiusMeters, locationMaxAge, maxCandidates)
		if err != nil {
			return "", err
		}

		key := candidateKey(candidates)
		if key != cachedKey {
			// Havuz o'zgardi (kimdir onlayn bo'ldi/offline bo'ldi/joyi
			// o'zgardi) — qayta saralaymiz va backoff'ni nolga
			// qaytaramiz: yangi kuryerga taklif DARHOL yuborilsin.
			cachedRank = d.rankCandidates(ctx, candidates, params)
			cachedKey = key
			failedCycles = 0
		}
		ranked := cachedRank

		// Nomzodlar TO'LQINLARGA bo'linadi: har to'lqinda `waveSize` ta
		// eng yaxshi nomzodga BIR VAQTDA taklif ketadi.
		for start := 0; start < len(ranked); start += waveSize {
			end := start + waveSize
			if end > len(ranked) {
				end = len(ranked)
			}
			batch := ranked[start:end]

			// To'lqinni ro'yxatga olamiz — `HandleResponse` faqat shu
			// to'plamdagi kuryerdan javob qabul qiladi.
			ids := make([]string, 0, len(batch))
			d.mu.Lock()
			offer.wave = make(map[string]struct{}, len(batch))
			for _, cand := range batch {
				offer.wave[cand.Courier.ID] = struct{}{}
				ids = append(ids, cand.Courier.ID)
			}
			// Eski to'lqindan qolib ketgan javoblarni tozalaymiz —
			// aks holda ular yangi to'lqinda "qabul qilindi" bo'lib
			// hisoblanardi.
			drainResponses(offer.respCh)
			d.mu.Unlock()

			for _, cand := range batch {
				d.notifier.SendOffer(cand.Courier.ID, OfferInfo{
					OrderID:           orderID,
					RestaurantID:      params.RestaurantID,
					RestaurantName:    params.RestaurantName,
					RestaurantAddress: params.RestaurantAddress,
					RestaurantLat:     params.RestaurantLocation.Lat,
					RestaurantLng:     params.RestaurantLocation.Lng,
					RestaurantLogoURL: params.RestaurantLogoURL,
					ExpiresIn:         d.offerTTL,
				})
			}
			slog.Info("dispatch: to'lqin yuborildi",
				"order", orderID, "couriers", ids, "size", len(batch))

			// cancelRest — g'olib aniqlanganda yoki muddat tugaganda
			// QOLGANLARIGA taklifni yopadi.
			cancelRest := func(winner string) {
				for _, id := range ids {
					if id != winner {
						d.notifier.CancelOffer(id, orderID)
					}
				}
			}

			winner, err := d.awaitWave(ctx, orderID, offer, ids, cancelRest)
			if err != nil {
				return "", err
			}
			if winner != "" {
				return winner, nil
			}
			// Bu to'lqinda hech kim olmadi — keyingisiga o'tamiz.
			continue
		}

		// Ushbu tsiklda hech kim topilmadi (ro'yxat bo'sh edi yoki hammasi
		// rad etdi/javob bermadi) — TASLIM BO'LMAYMIZ. Qisqa pauzadan
		// so'ng ro'yxat qayta so'raladi (avval rad etgan kuryer hamon
		// onlayn bo'lsa, yana nomzod bo'ladi; yangi onlayn bo'lgan kuryer
		// ham shu safar qo'shiladi) va tsikl takrorlanadi.
		failedCycles++
		pause := d.backoffPause(failedCycles)
		if len(ranked) == 0 {
			// Havuz BO'SH bo'lsa backoff qo'llanmaydi: bu holatda tsikl
			// hech qanday tashqi (pullik) so'rov yubormaydi —
			// `rankCandidates` nomzod yo'qligini ko'rib darhol qaytadi —
			// shuning uchun tez-tez tekshirish arzon, va birinchi onlayn
			// bo'lgan kuryerga taklif kechikmasdan yetib boradi.
			pause = d.retryPause()
			slog.Info("dispatch: hozircha bo'sh onlayn kuryer yo'q, kutilmoqda...",
				"order", orderID, "cycle", failedCycles)
		} else {
			slog.Info("dispatch: bu tsiklda hech kim qabul qilmadi, qayta urinish davom etadi",
				"order", orderID, "cycle", failedCycles, "pause", pause)
		}
		select {
		case <-time.After(pause):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}

// rankCandidates — ETA'larni oladi (Google Distance Matrix orqali) va
// ScoreCandidates bilan saralaydi. Google API vaqtincha ishlamay qolsa
// (tarmoq xatosi, kvota va h.k.) — butun dispatch TO'XTAB QOLMAYDI: to'g'ri
// chiziq masofa + taxminiy o'rtacha tezlikka asoslangan ZAXIRA ETA
// ishlatiladi. Bu — asosiy yo'nalish HAR DOIM Google API ekanini
// o'zgartirmaydi, faqat uning vaqtinchalik nosozligida tizim ishlashda
// davom etishini ta'minlaydi (aks holda haqiqiy bo'sh kuryerlar bo'lsa ham,
// tashqi API bir lahzalik uzilishi butun buyurtmani "kuryer topilmadi"
// holatiga tushirib qo'yishi mumkin edi).
func (d *Dispatcher) rankCandidates(ctx context.Context, candidates []*Courier, params DispatchParams) []ScoredCandidate {
	// MUHIM (jonli sinovda topilgan haqiqiy holat): kuryer ilovasi
	// ro'yxatdan o'tgan-u, lekin hali BIRON marta ham joylashuvini
	// yubormagan bo'lsa (masalan GPS ruxsati hali berilmagan, yoki ilova
	// joylashuvni yuborishdan oldin yopilgan), uning bazadagi Lat/Lng
	// standart qiymati 0,0 (Gvineya ko'rfazi) bo'lib qoladi — bu haqiqiy
	// joylashuv EMAS. Bunday "joylashuvi noma'lum" kuryer dispatch
	// nomzodlaridan chetlashtiriladi (aks holda mantiqsiz o'n kunlab
	// ETA bilan bitta taklif siklini behuda isrof qiladi va HAR DOIM eng
	// past ballga tushib, real kuryerlarga yetib borishni kechiktiradi).
	realCandidates := make([]*Courier, 0, len(candidates))
	for _, c := range candidates {
		if c.Lat == 0 && c.Lng == 0 {
			slog.Warn("dispatch: kuryer joylashuvi noma'lum (0,0) — nomzodlardan chetlashtirildi", "courier", c.ID)
			continue
		}
		realCandidates = append(realCandidates, c)
	}
	candidates = realCandidates

	geoCandidates := make([]geo.Candidate, len(candidates))
	byID := make(map[string]*Courier, len(candidates))
	for i, c := range candidates {
		geoCandidates[i] = geo.Candidate{
			ID:       c.ID,
			Location: geo.LatLng{Lat: c.Lat, Lng: c.Lng},
			Mode:     vehicleToMode(c.VehicleType),
		}
		byID[c.ID] = c
	}
	if len(candidates) == 0 {
		return nil
	}

	results, err := d.geoClient.FetchETAs(ctx, params.RestaurantLocation, geoCandidates)
	if err != nil {
		slog.Warn("dispatch: Google Distance Matrix ishlamadi — zaxira (to'g'ri chiziq) ETA'ga o'tildi", "err", err)
		scored := make([]ScoredCandidate, len(candidates))
		for i, c := range candidates {
			eta := haversineFallbackETA(params.RestaurantLocation, geo.LatLng{Lat: c.Lat, Lng: c.Lng}, vehicleToMode(c.VehicleType))
			scored[i] = ScoredCandidate{Courier: c, ETA: eta}
		}
		return ScoreCandidates(scored, params.PreparationTime)
	}

	scored := make([]ScoredCandidate, 0, len(results))
	for _, res := range results {
		if !res.OK {
			continue // Google yo'l topmadi (ZERO_RESULTS) — bu nomzod inobatga olinmaydi
		}
		scored = append(scored, ScoredCandidate{Courier: byID[res.ID], ETA: res.Duration})
	}
	return ScoreCandidates(scored, params.PreparationTime)
}

// haversineFallbackETA — FAQAT Google Distance Matrix ishlamay qolganda
// ishlatiladigan zaxira: to'g'ri chiziq masofa + transport turiga qarab
// taxminiy o'rtacha tezlik (shahar ichi).
func haversineFallbackETA(a, b geo.LatLng, mode geo.Mode) time.Duration {
	distanceMeters := haversineMeters(a, b)
	var speedKmh float64
	switch mode {
	case geo.ModeWalking:
		speedKmh = 4
	case geo.ModeBicycling:
		speedKmh = 15
	default:
		speedKmh = 25
	}
	hours := (distanceMeters / 1000) / speedKmh
	return time.Duration(hours * float64(time.Hour))
}

// haversineMeters — endi `geo.HaversineMeters` da (xotiradagi kuryer
// repozitoriysi ham shu hisobdan foydalanadi, ikki nusxa bo'lmasin).
func haversineMeters(a, b geo.LatLng) float64 { return geo.HaversineMeters(a, b) }

// awaitWave — bitta to'lqin natijasini kutadi.
//
// Qaytaradi: g'olib kuryer ID (topilsa) yoki bo'sh satr (to'lqin
// natijasiz tugadi — chaqiruvchi keyingisiga o'tadi).
//
// ┌─ POYGA XAVFSIZLIGI (eng muhim joyi) ──────────────────────────────┐
// To'lqindagi bir necha kuryer BIR VAQTDA "qabul qilaman" bosishi
// mumkin. Ikkalasi ham g'olib bo'lib qolsa, bitta buyurtmaga ikki
// kuryer biriktirilardi.
//
// Buni ikki narsa to'sadi:
//
//  1. Javoblarni FAQAT shu goroutine o'qiydi (`respCh`), ya'ni ular
//     KETMA-KET qayta ishlanadi — ikki "qabul" bir vaqtda g'olib
//     bo'la olmaydi.
//  2. G'olib `ClaimIfAvailable` (atomik UPDATE) bilan tasdiqlanadi.
//     Agar kuryer shu lahzada BOSHQA buyurtmani olib ulgurgan bo'lsa,
//     claim `false` qaytaradi va biz to'lqindagi KEYINGI javobni
//     kutishda davom etamiz — taklif behuda yo'qolmaydi.
//
// Birinchi muvaffaqiyatli claim'dan keyin darhol qaytamiz, qolganlarga
// esa `cancelRest` taklifni yopadi.
// └───────────────────────────────────────────────────────────────────┘
func (d *Dispatcher) awaitWave(ctx context.Context, orderID string,
	offer *waveOffer, ids []string, cancelRest func(winner string)) (string, error) {

	timer := time.NewTimer(d.offerTTL)
	defer timer.Stop()

	// rejected — nechta kuryer ANIQ rad etdi. Hammasi rad etsa,
	// muddat tugashini kutish ma'nosiz — darhol keyingi to'lqinga.
	rejected := 0

	for {
		select {
		case resp := <-offer.respCh:
			if !resp.Accepted {
				rejected++
				slog.Info("dispatch: rad etildi",
					"order", orderID, "courier", resp.CourierID,
					"rad_etganlar", rejected, "to_lqin", len(ids))
				if rejected >= len(ids) {
					// Hammasi rad etdi — kutmaymiz.
					return "", nil
				}
				continue
			}

			claimed, err := d.repo.ClaimIfAvailable(ctx, resp.CourierID)
			if err != nil {
				return "", err
			}
			if !claimed {
				// Kuryer shu lahzada boshqa buyurtmani olgan.
				// To'lqindagi qolganlarni kutishda davom etamiz.
				slog.Info("dispatch: kuryer allaqachon band — to'lqin davom etadi",
					"order", orderID, "courier", resp.CourierID)
				rejected++
				if rejected >= len(ids) {
					return "", nil
				}
				continue
			}

			slog.Info("dispatch: kuryer topildi",
				"order", orderID, "courier", resp.CourierID)
			cancelRest(resp.CourierID)
			return resp.CourierID, nil

		case <-timer.C:
			slog.Info("dispatch: to'lqin muddati tugadi",
				"order", orderID, "couriers", ids)
			cancelRest("")
			return "", nil

		case <-ctx.Done():
			cancelRest("")
			return "", ctx.Err()
		}
	}
}

// drainResponses — kanalda qolib ketgan eski javoblarni tashlaydi.
// Chaqiruvchi mutexni ushlab turadi.
func drainResponses(ch chan Response) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// HandleResponse — kuryer ilovadan "qabul qilaman / rad etaman" bosganda
// HTTP handler shu metodni chaqiradi.
//
// Javob FAQAT HOZIRGI TO'LQINDAGI kuryerdan qabul qilinadi. Muddati
// tugagan yoki keyingi to'lqinga o'tib ulgurgan kuryerdan kechikib
// kelgan javob rad etiladi (`false` qaytadi — ilova "kech qoldingiz"
// deb ko'rsatadi).
//
// G'olibni aniqlash BU YERDA emas: bu funksiya javobni shunchaki
// dispatch goroutine'iga uzatadi, u esa ketma-ket qayta ishlaydi va
// `ClaimIfAvailable` bilan tasdiqlaydi (`awaitWave` izohiga qarang).
func (d *Dispatcher) HandleResponse(orderID string, resp Response) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	offer, ok := d.pending[orderID]
	if !ok {
		return false // taklif allaqachon yopilgan
	}
	if !offer.inWave(resp.CourierID) {
		return false // eskirgan to'lqin yoki noto'g'ri nomzoddan javob
	}
	select {
	case offer.respCh <- resp:
		return true
	default:
		return false
	}
}
