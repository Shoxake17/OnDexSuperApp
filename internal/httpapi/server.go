// Package httpapi â€” API'ning HTTP qatlami.
//
// TUZILISH (Express.js bilan taqqoslash uchun):
//
//	server.go        ~ app.js        â€” bog'lash (wiring) va router ro'yxati
//	middleware.go    ~ middleware/   â€” auth, CORS, tana chegarasi, rate limit
//	respond.go       ~ res helpers   â€” javob yozishning yagona joyi
//	authz.go         ~ policies      â€” "kim nimani ko'ra oladi" qoidalari
//	routes_*.go      ~ routes/       â€” endpointlar mavzu bo'yicha
//	internal/{orders,catalog,users,...}  ~ services/models
//
// NEGA AJRATILDI: avval hammasi `cmd/api/main.go` da edi â€” 3200 qator,
// 49 ta route, bitta funksiya (`main`) ichida. Bunday faylda (a) biror
// endpointni topish qiyin, (b) yangi modul qo'shilganda u yanada
// o'sadi, (c) handler'lar `main` ning lokal o'zgaruvchilarini
// yopilma (closure) orqali ushlagani uchun ularni alohida testlash
// IMKONSIZ edi. Endi bog'liqliklar `Server` maydonlari â€” har bir
// handler'ni soxta (fake) repo bilan chaqirib testlash mumkin.
package httpapi

import (
	"context"
	"net/http"
	"sync"
	"time"

	"chustapp/internal/agentapi"
	"chustapp/internal/alerts"
	"chustapp/internal/assistant"
	"chustapp/internal/banners"
	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/delivery"
	"chustapp/internal/favorites"
	"chustapp/internal/firebaseauth"
	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
	"chustapp/internal/images"
	"chustapp/internal/mapprovider"
	"chustapp/internal/model3d"
	"chustapp/internal/notify"
	"chustapp/internal/ondexmap"
	"chustapp/internal/orders"
	"chustapp/internal/payments"
	"chustapp/internal/payments/mcconnect"
	"chustapp/internal/payments/octo"
	"chustapp/internal/promotions"
	"chustapp/internal/qurt"
	"chustapp/internal/qurtorders"
	"chustapp/internal/ratelimit"
	"chustapp/internal/ratings"
	"chustapp/internal/revoke"
	"chustapp/internal/scenes"
	"chustapp/internal/sso"
	"chustapp/internal/staff"
	"chustapp/internal/stats"
	"chustapp/internal/support"
	"chustapp/internal/tables"
	"chustapp/internal/telegram"
	"chustapp/internal/tracking"
	"chustapp/internal/users"
	"chustapp/internal/voice"
	"chustapp/internal/wallet"
	"chustapp/internal/ws"

	"chustapp/internal/cache"
)

// Deps â€” `main()` yig'adigan bog'liqliklar. Alohida struct sifatida:
// yangi bog'liqlik qo'shilganda `New` imzosi o'zgarmaydi va chaqiruv
// joyida qaysi qiymat qayerga borayotgani nom bilan ko'rinadi.
type Deps struct {
	OrderRepo   orders.Repository
	CourierRepo couriers.Repository
	UserRepo    users.Repository
	CatalogRepo catalog.Repository
	// BookRepo â€” kafe kutubxonasi. IXTIYORIY: faqat Mongo rejimida
	// ulanadi. `nil` bo'lsa kitob endpointlari xizmat yo'qligini
	// aytadi, qolgan API esa ishlayveradi.
	BookRepo       catalog.BookRepository
	PromotionsRepo promotions.Repository
	FavoritesRepo  favorites.Repository
	// BannerRepo — reklama/aksiya bannerlari (`internal/banners`,
	// Yandex Eats/Go uslubi, admin panelidan boshqariladi). `nil`
	// bo'lsa (Postgres ham, xotira zaxirasi ham yo'q — amalda
	// bo'lmasligi kerak) `/promo-banners` va `/admin/banners`
	// RO'YXATGA OLINMAYDI.
	BannerRepo banners.Repository
	// RatingsRepo — mijoz buyurtmadan keyin restoranga qo'ygan baho
	// (`internal/ratings`). `nil` bo'lsa `/orders/{id}/rating`
	// RO'YXATGA OLINMAYDI va `catalog.Restaurant.Rating`/`RatingCount`
	// doim 0 qaytadi (`scene_access.go`, `withRating`).
	RatingsRepo ratings.Repository
	// RouteRepo — buyurtmaga biriktirilgan yetkazish yo'li (A→B,
	// `internal/tracking`). `nil` bo'lsa kuzatuv faqat jonli qismini beradi.
	RouteRepo tracking.Repository
	// Directions — yo'l hisoblovchi. `nil` — Google Directions API
	// (`directions.go`); testlar soxtasini beradi.
	Directions DirectionsFunc
	// MapMode — GOOGLE_MAP bayrog'i (internal/mapprovider). Bo'sh qiymat —
	// Google. OnDexMap rejimi faqat OndexMap ham berilgan bo'lsa ishlaydi.
	MapMode mapprovider.Mode
	// OndexMap — OnDexMap /v1 mijozi. nil bo'lsa hamma narsa Google'da qoladi.
	OndexMap ondexmap.API
	// OndexMapBase — ilovalarga /config/maps orqali beriladigan OnDexMap manzili.
	OndexMapBase string
	// Voice — ovozli yo'l ko'rsatish (restoran nomi bor ibora, Gemini TTS).
	// `nil` bo'lsa endpoint 503, ilova umumiy iborani aytadi.
	Voice *voice.Service
	// StatsSource — restoran paneli "Statistika" sahifasi uchun buyurtma
	// qatorlari (`internal/stats`). `nil` bo'lsa `/restaurants/{id}/stats`
	// 503 qaytaradi, qolgan API ishlayveradi.
	StatsSource stats.Source
	// HistorySource — "Barcha buyurtmalar" sahifasi (restoran ochilgandan
	// beri, sahifalab). `nil` bo'lsa `/restaurants/{id}/orders/history`
	// 503 qaytaradi.
	HistorySource stats.HistorySource

	Cache      *cache.Cache
	Tokens     *users.TokenIssuer
	Revoked    *revoke.Store
	Hub        *ws.Hub
	WsTickets  *ws.TicketStore
	ImageStore images.Store

	// Scenes â€” 3D maket fayllari uchun muddatli havola beruvchi
	// (`internal/scenes`). `nil` bo'lsa maket manzili saqlanganidek
	// qaytadi, lekin FAQAT kirgan foydalanuvchiga
	// (`scene_access.go` izohiga qarang).
	Scenes *scenes.Signer
	// MediaPublicBaseURL â€” `R2_PUBLIC_URL`. Saqlangan to'liq manzildan
	// obyekt kalitini ajratish uchun kerak (`scenes.ObjectKey`).
	MediaPublicBaseURL string
	// ScenesUnavailable — `R2_SCENES_BUCKET` BERILGAN, lekin unga
	// kirib bo'lmadi (ko'pincha R2 tokenida shu bucket uchun ruxsat
	// yo'q).
	//
	// Bunda maket taklifi UMUMAN ko'rsatilmaydi. Saqlangan ommaviy
	// manzilni qaytarish YARAMAYDI: fayl allaqachon yopiq bucket'ga
	// ko'chirilgan bo'lsa u yerda 404 bo'lardi va mijoz 220 MB ni
	// yuklab bo'lgach xato ko'rardi.
	ScenesUnavailable bool

	AuthSvc    *users.Service
	OrderSvc   *orders.Service
	CatalogSvc *catalog.Service
	Dispatcher *couriers.Dispatcher

	// TableSvc â€” stol QR kodlari (dine_in buyurtmalar). `nil` bo'lsa
	// stol buyurtmalari 503 qaytaradi, qolgan hamma narsa ishlayveradi
	// â€” bu funksiyani bosqichma-bosqich yoqish uchun.
	TableSvc *tables.Service
	// TableOrders — joylar holati (band/bo'sh) uchun stol buyurtmalari.
	// `nil` bo'lsa holatlar buyurtmasiz hisoblanadi (hamma joy "bo'sh").
	TableOrders tables.OrdersSource

	// StaffSvc — "Xodimlar" bo'limi (`internal/staff`). `nil` bo'lsa
	// xodim endpointlari 503 qaytaradi.
	StaffSvc *staff.Service
	// AlertsSvc — restoran "Bildirishnomalar" markazi (`internal/alerts`).
	// `nil` bo'lsa tegishli endpointlar 503 qaytaradi.
	AlertsSvc *alerts.Service
	// SupportSvc — aloqa ma'lumotlari va restoran ↔ OnDex admin chati
	// (`internal/support`). `nil` bo'lsa tegishli endpointlar 503 qaytaradi.
	SupportSvc *support.Service

	// Payments â€” karta orqali to'lov. `nil` bo'lsa to'lov endpointlari
	// 503 qaytaradi va buyurtmalar faqat NAQD bo'ladi (tizimning
	// qolgan qismi normal ishlaydi).
	Payments *payments.Service
	// OctoClient â€” callback imzosini tekshirish uchun. `Payments`
	// bilan birga to'ldiriladi.
	OctoClient *octo.Client

	// Multicard — Multicard (Rahmat) shlyuzi: restoran Moliya
	// bo'limidagi tushum reestri. `nil` bo'lsa Moliya sahifasi
	// "shlyuz sozlanmagan" holatini ko'rsatadi va qolgan hamma narsa
	// normal ishlayveradi (Octo'dan MUSTAQIL — ikkisi bir-birini
	// bilmaydi).
	// MC — restoranning to'lov shlyuzi ulanishi
	// (`internal/payments/mcconnect`). Reyestr ichida: platforma
	// kalitlari, shlyuz manzili, sirlarni shifrlagich va klient keshi.
	//
	// `nil` bo'lsa Moliya bo'limi butunlay o'chiq va karta orqali
	// to'lov yaratilmaydi; qolgan hamma narsa ishlayveradi.
	MC *mcconnect.Registry

	// WalletSvc â€” OnDex Wallet (`internal/wallet`). `nil` bo'lsa
	// `GET /wallet` 503 qaytaradi, checkout'da "wallet" to'lov usuli
	// `orders.Service`da xuddi shunday yopiq bo'ladi.
	WalletSvc *wallet.Service

	// SSO — yagona kirish manbai (`internal/sso`): boshqa xizmatlar
	// (OnDex Merchant Panel) foydalanuvchini OnDex orqali tekshiradi.
	// `nil` bo'lsa `/sso/*` 503 qaytaradi va qolgan API o'zgarishsiz
	// ishlaydi (`SSO_MERCHANT_SECRET` sozlanmagan holat).
	SSO *sso.Service

	// Identity — MARKAZIY sessiya va token xizmati (`internal/identity`, A2):
	// Ed25519 tokenlar, JWKS, refresh rotatsiyasi. `nil` bo'lsa `/v2/auth/*`
	// va `/.well-known/jwks.json` 503 qaytaradi; eski `/auth/*` ishlayveradi.
	Identity *identity.Service

	// Authz — merchant ruxsatlari (`internal/identity/authz`, A3): tashkilot,
	// biznes, rol, a'zolik. `nil` bo'lsa `/v2/merchant/*` 503 qaytaradi.
	Authz *authz.Authorizer

	// MerchantAuth — merchant realmiga kirish (A3b: parol/kod + majburiy TOTP
	// egasi uchun). `nil` bo'lsa `/v2/auth/merchant/*`, `/v2/auth/mfa/*` 503.
	MerchantAuth *identity.MerchantAuth

	// QurtRepo — qurt do'konlari mahsulotlari (internal/qurt): POS serveri
	// yozadi, mijoz ilovasi o'qiydi. `nil` bo'lsa /qurt/* va /merchant/qurt/*
	// 503 qaytaradi, qolgan API o'zgarishsiz ishlaydi.
	QurtRepo qurt.Repository
	// QurtOrderSvc — qurt buyurtmalari va reys logistikasi (internal/qurtorders).
	QurtOrderSvc *qurtorders.Service
	// qurtLiveOnce — qurt buyurtma voqealarini jonli kanalga ulash BIR marta (registerQurtCourierRoutes).
	qurtLiveOnce sync.Once

	// DevMode â€” dev rejim (faqat aniq `APP_ENV=development`). Ba'zi
	// javoblar (masalan OTP kodi) faqat shu rejimda qaytariladi.
	DevMode bool

	// PlatformCouriers — OnDex platforma kuryerlari (o'zi ro'yxatdan
	// o'tadigan) YOQILGANMI. Hozircha `false` (2026-09-15, foydalanuvchi
	// qarori): yetkazishni restoranning O'Z kuryerlari bajaradi va
	// `POST /couriers/register` yopiq. `main.go` buni o'rnatmaydi —
	// yoqish ongli kod o'zgarishi bo'lishi kerak (dispatch'ga platforma
	// bosqichi qo'shilgan kuni).
	PlatformCouriers bool

	// EmailConfigured â€” HAQIQIY SMTP ulanganmi.
	//
	// NEGA KERAK: `dev_code` javobda qaytarilishining YAGONA sababi â€”
	// haqiqiy yetkazish kanali yo'qligi (SMS uchun Eskiz.uz hali
	// ulanmagan, shuning uchun kodni boshqa yo'l bilan olib
	// bo'lmaydi). SMTP ulangÐ°Ñ‡ email kodi HAQIQATAN pochtaga boradi,
	// ya'ni uni javobda ham qaytarish endi:
	//   * keraksiz â€” ilova uni avtomatik to'ldirib, foydalanuvchi
	//     xatni umuman ochmaydi va yetkazish muammolari YASHIRINADI;
	//   * xavfli â€” `APP_ENV` ni production'ga o'tkazish unutilsa,
	//     kod API javobida ochiq ketaveradi.
	// Shu sabab email kodi SMTP ulangan zahoti javobdan chiqariladi.
	EmailConfigured bool

	// EmailLoginEnabled â€” email orqali kirish/ro'yxatdan o'tish
	// endpointlari ochiqmi (`emailLoginReady` izohiga qarang).
	// `.env` dagi `EMAIL_LOGIN_ENABLED`, standart â€” O'CHIQ.
	EmailLoginEnabled bool

	// Firebase â€” Phone Auth ID tokenlarini tekshiruvchi. `nil` yoki
	// loyiha ID'si bo'sh bo'lsa `/auth/firebase` 503 qaytaradi.
	Firebase *firebaseauth.Verifier

	// Telegram â€” bot orqali kod yetkazish (birinchi pog'ona).
	// Sozlanmagan bo'lsa `/auth/telegram/start` 503 qaytaradi va
	// ilova keyingi pog'onaga (Firebase) o'tadi.
	Telegram *telegram.Verifier

	// TelegramBotToken â€” Mini App `initData` imzosini tekshirish uchun
	// (`POST /auth/telegram/miniapp`).
	//
	// NEGA ALOHIDA: `Verifier` tokenni ichida saqlaydi, lekin uni
	// TASHQARIGA BERMAYDI â€” bu ataylab shunday (sir bitta joyda
	// qolsin). `initData` tekshiruvi esa xuddi shu tokenni talab
	// qiladi, shuning uchun u bu yerga ALOHIDA beriladi.
	//
	// Bo'sh bo'lsa Mini App kirishi ishlamaydi va `ValidateInitData`
	// aniq xato qaytaradi â€” jimgina "hammasi joyida" demaydi.
	TelegramBotToken string

	// Devices â€” foydalanuvchi qaysi mijoz dasturidan (TMA, mobil
	// ilova, brauzer) kirgani qaydi â€” superadmin panelidagi "Qurilma"
	// ustuni uchun. `nil` bo'lsa qayd YURITILMAYDI va qolgan hamma
	// narsa o'zgarishsiz ishlaydi (`devices.go`).
	Devices users.DeviceStore

	// Model3D â€” taom rasmidan 3D model generatsiyasi. `nil` bo'lsa
	// tegishli endpointlar 503 qaytaradi va qolgan hamma narsa
	// o'zgarishsiz ishlaydi (`TRIPO_API_KEY` sozlanmagan holat).
	Model3D *model3d.Service
	// Model3DLimiter â€” restoran bo'yicha tezlik chegarasi. Har chaqiruv
	// tashqi xizmatda PUL sarflaydi, shuning uchun chegara SHART.
	Model3DLimiter *ratelimit.Limiter

	// UploadLimiter â€” media yuklash uchun xodim bo'yicha chegara.
	//
	// â”Œâ”€ NEGA YUKLASHGA HAM CHEGARA KERAK â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
	// Yuklash autentifikatsiya talab qiladi, shuning uchun uzoq vaqt
	// chegarasiz qoldirilgandi. Lekin kitob PDF i qo'shilgach hisob
	// o'zgardi: bitta so'rov 25 MB qabul qiladi, uni R2 ga yozadi
	// (JOY VA PUL) va ustiga 2000 sahifagacha PDF tahlil qiladi
	// (PROTSESSOR). Ya'ni o'g'irlangan restoran tokeni bilan bir necha
	// daqiqada bucket'ni ham, protsessorni ham band qilib bo'ladi.
	//
	// Rasm yuklash ham shu chegaraga tushadi: u yengilroq, lekin
	// alohida hisoblagich saqlashga arzimaydi.
	// â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
	UploadLimiter *ratelimit.Limiter

	// Notifications â€” saqlangan bildirishnomalar ombori
	// (`GET /notifications`). `nil` bo'lsa endpointlar 503 qaytaradi.
	Notifications notify.Store
	// PushTokens â€” qurilma push tokenlari (`POST /me/push-token`).
	PushTokens notify.TokenStore
	// Notifier â€” bildirishnomani BIR chaqiruvda saqlaydi, WebSocket
	// bilan yuboradi va ilova yopiq bo'lsa push qiladi.
	//
	// `Notifications`/`PushTokens` dan farqi: ular xom omborlar
	// (handler'lar ro'yxatni o'qish uchun ishlatadi), bu esa
	// YUBORISH yo'li. `nil` bo'lsa xabar yuborilmaydi, qolgan
	// hamma narsa ishlayveradi.
	Notifier *notify.Service

	// AgentSvc â€” tashqi AI agentlar integratsiyasi
	// (`internal/agentapi`). `nil` bo'lsa `/agent/v1/*` va
	// `/me/agent/*` 503 qaytaradi va qolgan API o'zgarishsiz
	// ishlaydi â€” bazasiz (xotira) dev rejimida aynan shunday
	// bo'ladi.
	AgentSvc *agentapi.Service

	// Assistant â€” ILOVA ICHIDAGI AI yordamchi (`internal/assistant`).
	//
	// `AgentSvc` bilan chalkashtirmang: u tashqi serverga eshik
	// ochadi, bu esa OnDex ilovasining o'z chat/ovoz oynasini
	// ta'minlaydi va buyurtma YARATA OLMAYDI â€” faqat savat taklifi.
	//
	// `SHADDIY_AI_URL` yoki `SHADDIY_API_KEY` bo'lmasa `nil` va
	// `/ai/*` 503 qaytaradi.
	Assistant *assistant.Service

	// AssistantLive â€” OVOZLI rejim sozlamasi (Gemini Live).
	//
	// Matnli chatdan MUSTAQIL: `Assistant` bor bo'lib, bu `nil`
	// bo'lishi mumkin (kalit qo'yilmagan) â€” bunda ilova chat oynasini
	// ko'rsatadi, mikrofon tugmasini esa yo'q. Teskarisi bo'lmaydi:
	// ovoz `Assistant` ning tool'lariga tayanadi.
	//
	// `GEMINI_API_KEY` bo'lmasa `nil` va `GET /ai/live` marshruti
	// UMUMAN ro'yxatga olinmaydi.
	AssistantLive *assistant.LiveConfig
}

// Server â€” HTTP qatlamining holati. Barcha handler'lar shu turning
// metodlari, ya'ni bog'liqliklar oshkora va almashtiriladigan.
type Server struct {
	Deps

	// speedGate â€” kuryer koordinatasining "teleport" qilishini
	// aniqlaydi (GPS soxtalashtirishga qarshi).
	speedGate *delivery.SpeedGate

	// statsLimiter — statistika uchun xodim bo'yicha chegara (faqat kesh
	// o'tkazib yuborilganda, ya'ni haqiqiy baza ishi bo'lganda sanaladi).
	// 30 ta so'rovga bir zumda ruxsat, keyin daqiqasiga 30 ta.
	statsLimiter *ratelimit.Limiter

	// historyLimiter — buyurtmalar tarixi sahifalari uchun xodim bo'yicha
	// chegara. Sahifa arzon (indeks + LIMIT), lekin har biriga mijoz
	// telefonlari qo'shiladi: 60 ta bir zumda, keyin soniyasiga 2 ta.
	historyLimiter *ratelimit.Limiter

	// supportLimiter — chat xabarlari uchun akkaunt bo'yicha chegara:
	// 10 ta bir zumda, keyin 2 soniyada bitta. Odam yozishi uchun yetarli,
	// o'g'irlangan token bilan admin kanalini xabarga ko'mib tashlash uchun emas.
	supportLimiter *ratelimit.Limiter

	// liveRoutes — kuryer kuzatuvidagi yo'llar keshi (`order_tracking.go`):
	// Google Directions'ga buyurtma bo'yicha ko'pi bilan 30 soniyada bir.
	liveRoutes *liveRouteCache

	// ┌─ XARITA MANBASI — ADMIN PANELDAN REAL VAQTDA ──────────────────────┐
	// `Deps.MapMode` (`GOOGLE_MAP` muhit o'zgaruvchisi) — ISHGA TUSHISH
	// payti standart qiymati. Admin panel endi buni server qayta
	// ishga tushirilmasdan, JONLI o'zgartira oladi
	// (`POST /admin/config/map-mode`) — natija XOTIRADA saqlanadi
	// (keshga murojaatsiz, har so'rovda darhol), Redis'ga esa FAQAT
	// qayta ishga tushganda tiklash uchun yoziladi.
	//
	// `nil` — hali o'zgartirilmagan, `Deps.MapMode` ishlatiladi.
	// └──────────────────────────────────────────────────────────────────┘
	mapModeMu       sync.RWMutex
	mapModeOverride *mapprovider.Mode
}

func New(d Deps) *Server {
	s := &Server{
		Deps:           d,
		speedGate:      delivery.NewSpeedGate(),
		statsLimiter:   ratelimit.New(0.5, 30),
		historyLimiter: ratelimit.New(2, 60),
		supportLimiter: ratelimit.New(0.5, 10),
		liveRoutes:     newLiveRouteCache(),
	}
	// Admin oldin o'zgartirgan xarita rejimini tiklaydi (server qayta
	// ishga tushgan bo'lsa) — topilmasa `Deps.MapMode` standart qoladi.
	s.loadMapModeOverride(context.Background())

	// 3D model holati o'zgarganda ikki ish qilinadi. Ikkalasi ham
	// SHART:
	//   * menyu keshi tozalanadi â€” aks holda mijoz ilovasi 30 soniya
	//     davomida eski (modelsiz) menyuni olib turardi;
	//   * restoran kanaliga xabar â€” panel tugmani "tayyorlanmoqda"
	//     dan "tayyor" ga o'zi almashtiradi, sahifani yangilash
	//     kerak bo'lmaydi.
	if s.Model3D != nil {
		s.Model3D.SetOnUpdate(func(p *catalog.Product) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if s.Cache != nil {
				s.Cache.Del(ctx, menuCacheKey(p.RestaurantID))
			}
			s.sendPublicRestaurantEvent(p.RestaurantID, map[string]any{
				"type":       "model3d_updated",
				"product_id": p.ID,
				"status":     p.Model3DStatus,
				"model_url":  p.Model3DURL,
			})
		})
	}
	return s
}

// Routes â€” barcha endpointlarni ro'yxatga oladi va tayyor handler
// qaytaradi (Express'dagi `app.use(router)` zanjiriga mos).
//
// Har bir `register*` metodi alohida faylda â€” yangi modul qo'shilganda
// shu ro'yxatga BITTA qator qo'shiladi, mavjud fayllar tegilmaydi.
func (s *Server) Routes(allowedOrigins []string) http.Handler {
	mux := http.NewServeMux()

	s.registerWsRoutes(mux)
	s.registerAuthRoutes(mux)
	s.registerAuthV2Routes(mux)
	s.registerMerchantAuthRoutes(mux)
	s.registerMeRoutes(mux)
	s.registerNotificationRoutes(mux)
	s.registerCatalogRoutes(mux)
	s.registerBannerRoutes(mux)
	s.registerFavoriteRoutes(mux)
	s.registerOrderRoutes(mux)
	s.registerRatingRoutes(mux)
	s.registerTrackingRoutes(mux)
	s.registerVoiceRoutes(mux)
	s.registerStatsRoutes(mux)
	s.registerFinanceRoutes(mux)
	s.registerRestaurantSettingsRoutes(mux)
	s.registerStaffRoutes(mux)
	s.registerAlertRoutes(mux)
	s.registerSupportRoutes(mux)
	s.registerTableRoutes(mux)
	s.registerBookRoutes(mux)
	s.registerWaiterRoutes(mux)
	s.registerCourierRoutes(mux)
	s.registerPromotionRoutes(mux)
	s.registerPaymentRoutes(mux)
	s.registerWalletRoutes(mux)
	s.registerSSORoutes(mux)
	s.registerAdminRoutes(mux)
	s.registerAdminStoreRoutes(mux)
	s.registerQurtRoutes(mux)
	s.registerQurtOrderRoutes(mux)
	s.registerQurtCourierRoutes(mux)
	s.registerAdminUserRoutes(mux)
	s.registerGeoRoutes(mux)
	s.registerMapPickerRoutes(mux)
	s.registerUploadRoutes(mux)
	s.registerModel3DRoutes(mux)
	// Tashqi AI agentlar: sherik yuzasi (`/agent/v1/*`) va
	// foydalanuvchi nazorati (`/me/agent/*`) â€” ATAYLAB ikki alohida
	// ro'yxat, chunki ular butunlay boshqa autentifikatsiyaga
	// tayanadi (sherik kaliti + grant / oddiy foydalanuvchi JWT).
	s.registerAgentRoutes(mux)
	s.registerMeAgentRoutes(mux)
	// Ilova ichidagi AI yordamchi (chat + ovoz). Tashqi agent
	// yo'lidan MUSTAQIL: oddiy foydalanuvchi JWT'si bilan ishlaydi.
	s.registerAssistantRoutes(mux)
	s.registerAssistantLiveRoutes(mux, allowedOrigins)
	s.registerMeAIToolRoutes(mux)

	return withBodyLimit(withCORS(mux, allowedOrigins, s.DevMode))
}
