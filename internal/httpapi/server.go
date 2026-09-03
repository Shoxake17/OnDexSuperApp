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
	"context"
	"net/http"
	"time"

	"chustapp/internal/agentapi"
	"chustapp/internal/assistant"
	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/delivery"
	"chustapp/internal/favorites"
	"chustapp/internal/firebaseauth"
	"chustapp/internal/images"
	"chustapp/internal/model3d"
	"chustapp/internal/notify"
	"chustapp/internal/orders"
	"chustapp/internal/payments"
	"chustapp/internal/payments/octo"
	"chustapp/internal/promotions"
	"chustapp/internal/ratelimit"
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
	OrderRepo   orders.Repository
	CourierRepo couriers.Repository
	UserRepo    users.Repository
	CatalogRepo catalog.Repository
	// BookRepo — kafe kutubxonasi. IXTIYORIY: faqat Mongo rejimida
	// ulanadi. `nil` bo'lsa kitob endpointlari xizmat yo'qligini
	// aytadi, qolgan API esa ishlayveradi.
	BookRepo       catalog.BookRepository
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

	// Payments — karta orqali to'lov. `nil` bo'lsa to'lov endpointlari
	// 503 qaytaradi va buyurtmalar faqat NAQD bo'ladi (tizimning
	// qolgan qismi normal ishlaydi).
	Payments *payments.Service
	// OctoClient — callback imzosini tekshirish uchun. `Payments`
	// bilan birga to'ldiriladi.
	OctoClient *octo.Client

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

	// Model3D — taom rasmidan 3D model generatsiyasi. `nil` bo'lsa
	// tegishli endpointlar 503 qaytaradi va qolgan hamma narsa
	// o'zgarishsiz ishlaydi (`TRIPO_API_KEY` sozlanmagan holat).
	Model3D *model3d.Service
	// Model3DLimiter — restoran bo'yicha tezlik chegarasi. Har chaqiruv
	// tashqi xizmatda PUL sarflaydi, shuning uchun chegara SHART.
	Model3DLimiter *ratelimit.Limiter

	// UploadLimiter — media yuklash uchun xodim bo'yicha chegara.
	//
	// ┌─ NEGA YUKLASHGA HAM CHEGARA KERAK ─────────────────────────────┐
	// Yuklash autentifikatsiya talab qiladi, shuning uchun uzoq vaqt
	// chegarasiz qoldirilgandi. Lekin kitob PDF i qo'shilgach hisob
	// o'zgardi: bitta so'rov 25 MB qabul qiladi, uni R2 ga yozadi
	// (JOY VA PUL) va ustiga 2000 sahifagacha PDF tahlil qiladi
	// (PROTSESSOR). Ya'ni o'g'irlangan restoran tokeni bilan bir necha
	// daqiqada bucket'ni ham, protsessorni ham band qilib bo'ladi.
	//
	// Rasm yuklash ham shu chegaraga tushadi: u yengilroq, lekin
	// alohida hisoblagich saqlashga arzimaydi.
	// └────────────────────────────────────────────────────────────────┘
	UploadLimiter *ratelimit.Limiter

	// Notifications — saqlangan bildirishnomalar ombori
	// (`GET /notifications`). `nil` bo'lsa endpointlar 503 qaytaradi.
	Notifications notify.Store
	// PushTokens — qurilma push tokenlari (`POST /me/push-token`).
	PushTokens notify.TokenStore
	// Notifier — bildirishnomani BIR chaqiruvda saqlaydi, WebSocket
	// bilan yuboradi va ilova yopiq bo'lsa push qiladi.
	//
	// `Notifications`/`PushTokens` dan farqi: ular xom omborlar
	// (handler'lar ro'yxatni o'qish uchun ishlatadi), bu esa
	// YUBORISH yo'li. `nil` bo'lsa xabar yuborilmaydi, qolgan
	// hamma narsa ishlayveradi.
	Notifier *notify.Service

	// AgentSvc — tashqi AI agentlar integratsiyasi
	// (`internal/agentapi`). `nil` bo'lsa `/agent/v1/*` va
	// `/me/agent/*` 503 qaytaradi va qolgan API o'zgarishsiz
	// ishlaydi — bazasiz (xotira) dev rejimida aynan shunday
	// bo'ladi.
	AgentSvc *agentapi.Service

	// Assistant — ILOVA ICHIDAGI AI yordamchi (`internal/assistant`).
	//
	// `AgentSvc` bilan chalkashtirmang: u tashqi serverga eshik
	// ochadi, bu esa OnDex ilovasining o'z chat/ovoz oynasini
	// ta'minlaydi va buyurtma YARATA OLMAYDI — faqat savat taklifi.
	//
	// `SHADDIY_AI_URL` yoki `SHADDIY_API_KEY` bo'lmasa `nil` va
	// `/ai/*` 503 qaytaradi.
	Assistant *assistant.Service

	// AssistantLive — OVOZLI rejim sozlamasi (Gemini Live).
	//
	// Matnli chatdan MUSTAQIL: `Assistant` bor bo'lib, bu `nil`
	// bo'lishi mumkin (kalit qo'yilmagan) — bunda ilova chat oynasini
	// ko'rsatadi, mikrofon tugmasini esa yo'q. Teskarisi bo'lmaydi:
	// ovoz `Assistant` ning tool'lariga tayanadi.
	//
	// `GEMINI_API_KEY` bo'lmasa `nil` va `GET /ai/live` marshruti
	// UMUMAN ro'yxatga olinmaydi.
	AssistantLive *assistant.LiveConfig
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
	s := &Server{Deps: d, speedGate: delivery.NewSpeedGate()}

	// 3D model holati o'zgarganda ikki ish qilinadi. Ikkalasi ham
	// SHART:
	//   * menyu keshi tozalanadi — aks holda mijoz ilovasi 30 soniya
	//     davomida eski (modelsiz) menyuni olib turardi;
	//   * restoran kanaliga xabar — panel tugmani "tayyorlanmoqda"
	//     dan "tayyor" ga o'zi almashtiradi, sahifani yangilash
	//     kerak bo'lmaydi.
	if s.Model3D != nil {
		s.Model3D.SetOnUpdate(func(p *catalog.Product) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if s.Cache != nil {
				s.Cache.Del(ctx, menuCacheKey(p.RestaurantID))
			}
			if s.Hub != nil {
				s.Hub.Send(restaurantTopic(p.RestaurantID), map[string]any{
					"type":       "model3d_updated",
					"product_id": p.ID,
					"status":     p.Model3DStatus,
					"model_url":  p.Model3DURL,
				})
			}
		})
	}
	return s
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
	s.registerBookRoutes(mux)
	s.registerWaiterRoutes(mux)
	s.registerCourierRoutes(mux)
	s.registerPromotionRoutes(mux)
	s.registerPaymentRoutes(mux)
	s.registerAdminRoutes(mux)
	s.registerAdminUserRoutes(mux)
	s.registerGeoRoutes(mux)
	s.registerMapPickerRoutes(mux)
	s.registerUploadRoutes(mux)
	s.registerModel3DRoutes(mux)
	// Tashqi AI agentlar: sherik yuzasi (`/agent/v1/*`) va
	// foydalanuvchi nazorati (`/me/agent/*`) — ATAYLAB ikki alohida
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
