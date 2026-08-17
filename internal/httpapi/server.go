// Package httpapi — API'ning HTTP qatlami.
//
// TUZILISH (Express.js bilan taqqoslash uchun):
//
//	server.go        ~ app.js        — bog'lash (wiring) va router ro'yxati
//	middleware.go    ~ middleware/   — auth, CORS, tana chegarasi, rate limit
//	respond.go       ~ res helpers   — javob yozishning yagona joyi
//	authz.go         ~ policies      — "kim nimani ko'ra oladi" qoidalari
//	routes_*.go      ~ routes/       — endpointlar mavzu bo'yicha
//	internal/{orders,catalog,users,...}  ~ services/models
//
// NEGA AJRATILDI: avval hammasi `cmd/api/main.go` da edi — 3200 qator,
// 49 ta route, bitta funksiya (`main`) ichida. Bunday faylda (a) biror
// endpointni topish qiyin, (b) yangi modul qo'shilganda u yanada
// o'sadi, (c) handler'lar `main` ning lokal o'zgaruvchilarini
// yopilma (closure) orqali ushlagani uchun ularni alohida testlash
// IMKONSIZ edi. Endi bog'liqliklar `Server` maydonlari — har bir
// handler'ni soxta (fake) repo bilan chaqirib testlash mumkin.
package httpapi

import (
	"net/http"

	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/delivery"
	"chustapp/internal/favorites"
	"chustapp/internal/firebaseauth"
	"chustapp/internal/images"
	"chustapp/internal/notify"
	"chustapp/internal/orders"
	"chustapp/internal/promotions"
	"chustapp/internal/revoke"
	"chustapp/internal/tables"
	"chustapp/internal/telegram"
	"chustapp/internal/users"
	"chustapp/internal/ws"

	"chustapp/internal/cache"
)

// Deps — `main()` yig'adigan bog'liqliklar. Alohida struct sifatida:
// yangi bog'liqlik qo'shilganda `New` imzosi o'zgarmaydi va chaqiruv
// joyida qaysi qiymat qayerga borayotgani nom bilan ko'rinadi.
type Deps struct {
	OrderRepo      orders.Repository
	CourierRepo    couriers.Repository
	UserRepo       users.Repository
	CatalogRepo    catalog.Repository
	PromotionsRepo promotions.Repository
	FavoritesRepo  favorites.Repository

	Cache      *cache.Cache
	Tokens     *users.TokenIssuer
	Revoked    *revoke.Store
	Hub        *ws.Hub
	WsTickets  *ws.TicketStore
	ImageStore images.Store

	AuthSvc    *users.Service
	OrderSvc   *orders.Service
	CatalogSvc *catalog.Service
	Dispatcher *couriers.Dispatcher

	// TableSvc — stol QR kodlari (dine_in buyurtmalar). `nil` bo'lsa
	// stol buyurtmalari 503 qaytaradi, qolgan hamma narsa ishlayveradi
	// — bu funksiyani bosqichma-bosqich yoqish uchun.
	TableSvc *tables.Service

	// DevMode — dev rejim (faqat aniq `APP_ENV=development`). Ba'zi
	// javoblar (masalan OTP kodi) faqat shu rejimda qaytariladi.
	DevMode bool

	// EmailConfigured — HAQIQIY SMTP ulanganmi.
	//
	// NEGA KERAK: `dev_code` javobda qaytarilishining YAGONA sababi —
	// haqiqiy yetkazish kanali yo'qligi (SMS uchun Eskiz.uz hali
	// ulanmagan, shuning uchun kodni boshqa yo'l bilan olib
	// bo'lmaydi). SMTP ulangач email kodi HAQIQATAN pochtaga boradi,
	// ya'ni uni javobda ham qaytarish endi:
	//   * keraksiz — ilova uni avtomatik to'ldirib, foydalanuvchi
	//     xatni umuman ochmaydi va yetkazish muammolari YASHIRINADI;
	//   * xavfli — `APP_ENV` ni production'ga o'tkazish unutilsa,
	//     kod API javobida ochiq ketaveradi.
	// Shu sabab email kodi SMTP ulangan zahoti javobdan chiqariladi.
	EmailConfigured bool

	// EmailLoginEnabled — email orqali kirish/ro'yxatdan o'tish
	// endpointlari ochiqmi (`emailLoginReady` izohiga qarang).
	// `.env` dagi `EMAIL_LOGIN_ENABLED`, standart — O'CHIQ.
	EmailLoginEnabled bool

	// Firebase — Phone Auth ID tokenlarini tekshiruvchi. `nil` yoki
	// loyiha ID'si bo'sh bo'lsa `/auth/firebase` 503 qaytaradi.
	Firebase *firebaseauth.Verifier

	// Telegram — bot orqali kod yetkazish (birinchi pog'ona).
	// Sozlanmagan bo'lsa `/auth/telegram/start` 503 qaytaradi va
	// ilova keyingi pog'onaga (Firebase) o'tadi.
	Telegram *telegram.Verifier

	// TelegramBotToken — Mini App `initData` imzosini tekshirish uchun
	// (`POST /auth/telegram/miniapp`).
	//
	// NEGA ALOHIDA: `Verifier` tokenni ichida saqlaydi, lekin uni
	// TASHQARIGA BERMAYDI — bu ataylab shunday (sir bitta joyda
	// qolsin). `initData` tekshiruvi esa xuddi shu tokenni talab
	// qiladi, shuning uchun u bu yerga ALOHIDA beriladi.
	//
	// Bo'sh bo'lsa Mini App kirishi ishlamaydi va `ValidateInitData`
	// aniq xato qaytaradi — jimgina "hammasi joyida" demaydi.
	TelegramBotToken string

	// Devices — foydalanuvchi qaysi mijoz dasturidan (TMA, mobil
	// ilova, brauzer) kirgani qaydi — superadmin panelidagi "Qurilma"
	// ustuni uchun. `nil` bo'lsa qayd YURITILMAYDI va qolgan hamma
	// narsa o'zgarishsiz ishlaydi (`devices.go`).
	Devices users.DeviceStore

	// Notifications — saqlangan bildirishnomalar ombori
	// (`GET /notifications`). `nil` bo'lsa endpointlar 503 qaytaradi.
	Notifications notify.Store
	// PushTokens — qurilma push tokenlari (`POST /me/push-token`).
	PushTokens notify.TokenStore
}

// Server — HTTP qatlamining holati. Barcha handler'lar shu turning
// metodlari, ya'ni bog'liqliklar oshkora va almashtiriladigan.
type Server struct {
	Deps

	// speedGate — kuryer koordinatasining "teleport" qilishini
	// aniqlaydi (GPS soxtalashtirishga qarshi).
	speedGate *delivery.SpeedGate
}

func New(d Deps) *Server {
	return &Server{Deps: d, speedGate: delivery.NewSpeedGate()}
}

// Routes — barcha endpointlarni ro'yxatga oladi va tayyor handler
// qaytaradi (Express'dagi `app.use(router)` zanjiriga mos).
//
// Har bir `register*` metodi alohida faylda — yangi modul qo'shilganda
// shu ro'yxatga BITTA qator qo'shiladi, mavjud fayllar tegilmaydi.
func (s *Server) Routes(allowedOrigins []string) http.Handler {
	mux := http.NewServeMux()

	s.registerWsRoutes(mux)
	s.registerAuthRoutes(mux)
	s.registerMeRoutes(mux)
	s.registerNotificationRoutes(mux)
	s.registerCatalogRoutes(mux)
	s.registerFavoriteRoutes(mux)
	s.registerOrderRoutes(mux)
	s.registerTableRoutes(mux)
	s.registerWaiterRoutes(mux)
	s.registerCourierRoutes(mux)
	s.registerPromotionRoutes(mux)
	s.registerAdminRoutes(mux)
	s.registerAdminUserRoutes(mux)
	s.registerGeoRoutes(mux)
	s.registerMapPickerRoutes(mux)
	s.registerUploadRoutes(mux)

	return withBodyLimit(withCORS(mux, allowedOrigins, s.DevMode))
}
