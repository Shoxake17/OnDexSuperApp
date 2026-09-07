// api/main.go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"chustapp/internal/agentapi"
	"chustapp/internal/appenv"
	"chustapp/internal/assistant"
	"chustapp/internal/cache"
	"chustapp/internal/catalog"
	"chustapp/internal/couriers"
	"chustapp/internal/favorites"
	"chustapp/internal/firebaseauth"
	"chustapp/internal/geo"
	"chustapp/internal/httpapi"
	"chustapp/internal/images"
	"chustapp/internal/model3d"
	"chustapp/internal/notify"
	"chustapp/internal/orders"
	"chustapp/internal/payments"
	"chustapp/internal/payments/octo"
	"chustapp/internal/promotions"
	"chustapp/internal/ratelimit"
	"chustapp/internal/revoke"
	"chustapp/internal/safego"
	"chustapp/internal/storage"
	"chustapp/internal/tables"
	"chustapp/internal/telegram"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// devMode — SMS kodlarni HTTP javobda qaytarish va zaif sozlamalarga
// ruxsat berish kabi FAQAT ishlab chiqish uchun mo'ljallangan
// yengilliklar.
//
// XAVFSIZLIK (fail-closed): bu bayroq faqat ANIQ `APP_ENV=development`
// bo'lgandagina yoqiladi. Avval teskari edi — `APP_ENV != "production"`,
// ya'ni bo'sh qiymat, `"Production"` (katta harf bilan), `"prod"` yoki
// oddiy xato yozuv ham dev rejimni YOQIB YUBORARDI. U holda:
//   - haqiqiy OTP kod HTTP javobda qaytarilardi (istalgan raqamga kirish),
//   - bo'sh JWT_SECRET qabul qilinib, hammaga ma'lum standart kalit
//     ishlatilardi (admin tokenini soxta yasash mumkin).
//
// Endi noto'g'ri/yetishmayotgan konfiguratsiya XAVFSIZ tomonga
// (production) og'adi.
var devMode bool

// devEnvValue — dev rejimni yoqadigan YAGONA qiymat.
const devEnvValue = "development"

// loadDotEnv — loyiha ildizidagi .env faylni o'qiydi (bor bo'lsa).
// Tizim muhitida allaqachon o'rnatilgan o'zgaruvchilar ustun turadi.
// Maxfiy qiymatlar (kalitlar, parollar) faqat shu faylda saqlanadi,
// kodga hech qachon yozilmaydi.
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
	slog.Info(".env yuklandi")
}

// firebaseServiceAccount — FCM xizmat akkaunti JSON'ini ikki
// manbadan biridan oladi.
//
// ┌─ NEGA IKKI YO'L ──────────────────────────────────────────────────┐
// `.env` parseri QATORMA-QATOR ishlaydi (`loadDotEnv`). Xizmat
// akkaunti JSON'i esa Google'dan CHIROYLI (ko'p qatorli) holda
// yuklab olinadi. Uni to'g'ridan-to'g'ri `.env` ga ko'chirsangiz
// faqat BIRINCHI QATOR o'qiladi va xato "kalit PEM formatida emas"
// bo'lib chiqadi — sababi esa umuman ko'rinmaydi.
//
// Shuning uchun FAYL YO'LI afzal: JSON o'z holicha qoladi, hech
// nimani bitta qatorga siqish shart emas.
//
//	FIREBASE_SERVICE_ACCOUNT_FILE=./secrets/fcm.json   (tavsiya)
//	FIREBASE_SERVICE_ACCOUNT_JSON={"type":"service_account",...}
//
// Ikkalasi ham berilsa INLINE ustun turadi (konteynerlarda odatda
// muhit o'zgaruvchisi ishlatiladi).
// └───────────────────────────────────────────────────────────────────┘
func firebaseServiceAccount() string {
	if v := strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON")); v != "" {
		return v
	}
	path := strings.TrimSpace(os.Getenv("FIREBASE_SERVICE_ACCOUNT_FILE"))
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// Yo'l berilgan, lekin o'qib bo'lmadi — bu ANIQ konfiguratsiya
		// xatosi, jimgina "push o'chirilgan" deb o'tib ketmaymiz.
		slog.Error("FIREBASE_SERVICE_ACCOUNT_FILE o'qib bo'lmadi — push o'chirilgan holda davom etiladi",
			"path", path, "err", err)
		return ""
	}
	return string(data)
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, nil)))
	loadDotEnv()
	appEnv := strings.TrimSpace(strings.ToLower(os.Getenv("APP_ENV")))
	devMode = appEnv == devEnvValue
	if devMode {
		slog.Warn("DEV REJIM yoqilgan (APP_ENV=development) — OTP kodlar javobda qaytariladi, zaif sozlamalarga ruxsat beriladi")
	} else {
		slog.Info("production rejim", "app_env", appEnv)
	}

	// ┌─ MUHIT INVENTARIZATSIYASI (bug.md 42, 99-bandlar) ─────────────┐
	// Yo'q o'zgaruvchilarni ULAR NIMANI O'CHIRISHI bilan birga logga
	// chiqaradi. Sabab: bu loyihada bir xil xato to'rt marta
	// takrorlangan — kod o'zgaruvchini o'qiydi, prod compose'ida esa
	// u sanab chiqilmagan, va funksiya JIMGINA o'chadi (karta to'lovi,
	// AI, ovoz, email login — beshtasi bir vaqtda o'lik turgan edi).
	//
	// Ro'yxatning o'zi `internal/appenv` da va u ikki tomondan
	// test bilan qulflangan (manba kodi + prod compose).
	// └────────────────────────────────────────────────────────────────┘
	appenv.Report(devMode)

	var orderRepo orders.Repository
	var courierRepo couriers.Repository
	var userRepo users.Repository
	var codeStore users.CodeStore
	var catalogRepo catalog.Repository
	var bookRepo catalog.BookRepository
	var promotionsRepo promotions.Repository
	var favoritesRepo favorites.Repository
	var pgPool *pgxpool.Pool // katalog Mongo'da bo'lmasa zaxira sifatida ishlatiladi

	// ---------- Buyurtmalar, foydalanuvchilar, kuryerlar: PostgreSQL ----------
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			slog.Error("PostgreSQL konfiguratsiya xatosi", "err", err)
			os.Exit(1)
		}
		if err := pool.Ping(ctx); err != nil {
			slog.Error("PostgreSQL'ga ulanib bo'lmadi (docker compose up -d qilinganmi?)", "err", err)
			os.Exit(1)
		}
		if err := storage.Migrate(ctx, pool); err != nil {
			slog.Error("migratsiya xatosi", "err", err)
			os.Exit(1)
		}
		// ┌─ DEMO AKKAUNTLAR FAQAT DEV'DA ────────────────────────────┐
		// Avval bu ikki seed `DATABASE_URL` bor bo'lsa HAR DOIM,
		// ya'ni PRODUCTION'da ham ishlardi va jonli bazaga
		// `+998900000099` raqamli ADMIN hamda demo restoran/kuryer
		// akkauntlarini qo'yardi.
		//
		// NEGA XAVFLI: kirish uchun parol kerak emas — OTP yetarli.
		// Kod esa Telegram orqali keladi. Ya'ni o'sha raqam operator
		// tomonidan kimgadir berilsa yoki kimdir uni Telegramda
		// ro'yxatdan o'tkazsa, o'sha odam SUPERADMIN huquqini qo'lga
		// kiritardi. Demo ma'lumot production bazasida turishining
		// o'zi ham noto'g'ri.
		//
		// ESLATMA: bu o'zgarish MAVJUD qatorlarni O'CHIRMAYDI — u
		// faqat yangi qo'shilishini to'xtatadi. Allaqachon tushib
		// qolgan demo akkauntlar qo'lda o'chirilishi kerak.
		// └────────────────────────────────────────────────────────────┘
		if devMode {
			if err := storage.SeedDemoCouriers(ctx, pool); err != nil {
				slog.Error("seed xatosi", "err", err)
				os.Exit(1)
			}
			if err := storage.SeedDemoUsers(ctx, pool); err != nil {
				slog.Error("users seed xatosi", "err", err)
				os.Exit(1)
			}
		}

		// ┌─ SUPERADMIN TAYINLASH ────────────────────────────────────┐
		// Admin roli hech qanday endpoint orqali BERILMAYDI — bu
		// ataylab: aks holda u huquqni ko'tarish (privilege
		// escalation) yuzasi bo'lardi. Yagona yo'l — server
		// sozlamasi, ya'ni serverga kira oladigan odam.
		//
		// FAQAT MAVJUD foydalanuvchini ko'taradi. Yangi akkaunt
		// yaratmaydi: raqam egasi avval odatdagi OTP oqimi bilan
		// ro'yxatdan o'tsin, shunda raqamga egalik allaqachon
		// tasdiqlangan bo'ladi.
		//
		// Har ishga tushishda qayta qo'llanadi (idempotent), shuning
		// uchun rol tasodifan o'zgarib qolsa ham tiklanadi.
		// └────────────────────────────────────────────────────────────┘
		if raw := strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_PHONE")); raw != "" {
			phone, err := users.NormalizePhone(raw)
			if err != nil {
				slog.Error("BOOTSTRAP_ADMIN_PHONE noto'g'ri formatda", "err", err)
				os.Exit(1)
			}
			promoted, err := storage.PromoteToAdmin(ctx, pool, phone)
			switch {
			case err != nil:
				slog.Error("BOOTSTRAP_ADMIN_PHONE qo'llanmadi", "err", err)
				os.Exit(1)
			case promoted:
				// WARN darajasi ataylab: huquq berish hodisasi
				// oddiy loglar orasida ko'zdan qochmasligi kerak.
				slog.Warn("BOOTSTRAP_ADMIN_PHONE: foydalanuvchi ADMIN roliga ko'tarildi",
					"phone", phone)
			default:
				slog.Error("BOOTSTRAP_ADMIN_PHONE: bu raqamli foydalanuvchi topilmadi — "+
					"avval shu raqam bilan ilovadan ro'yxatdan o'ting, keyin serverni qayta ishga tushiring",
					"phone", phone)
			}
		}
		pgPool = pool
		orderRepo = storage.NewPgOrderRepo(pool)
		courierRepo = storage.NewPgCourierRepo(pool)
		userRepo = storage.NewPgUserRepo(pool)
		codeStore = storage.NewPgCodeStore(pool)
		favoritesRepo = storage.NewPgFavoritesRepo(pool)
		slog.Info("rejim: PostgreSQL (buyurtmalar, foydalanuvchilar, kuryerlar)")
	} else {
		courierRepo = storage.NewMemoryCourierRepo(
			couriers.Courier{ID: "c1", Name: "Aziz", Lat: 41.0056, Lng: 71.2378, Available: true, Approved: true, VehicleType: couriers.VehicleMoped, Rating: 5.0},
			couriers.Courier{ID: "c2", Name: "Bekzod", Lat: 41.0010, Lng: 71.2400, Available: true, Approved: true, VehicleType: couriers.VehicleBike, Rating: 5.0},
			couriers.Courier{ID: "c3", Name: "Doniyor", Lat: 40.9980, Lng: 71.2330, Available: true, Approved: true, VehicleType: couriers.VehicleFoot, Rating: 5.0},
		)
		orderRepo = storage.NewMemoryOrderRepo()
		userRepo = storage.NewMemoryUserRepo(storage.DemoUsers()...)
		codeStore = storage.NewMemoryCodeStore()
		favoritesRepo = storage.NewMemoryFavoritesRepo()
		slog.Warn("rejim: in-memory (DATABASE_URL berilmagan — ma'lumotlar server o'chsa yo'qoladi)")
	}

	// ---------- Katalog (restoranlar+menyu): MongoDB ----------
	// Polyglot persistence: tranzaksion, munosabatli ma'lumotlar (buyurtma,
	// to'lov, foydalanuvchi) PostgreSQL'da; hujjat-shaklidagi, tez o'zgaruvchi
	// katalog MongoDB'da.
	//
	// ┌─ YAGONA HAQIQAT MANBAI ────────────────────────────────────────┐
	// Katalog uchun FAQAT MongoDB. Ilgari bu yerda PostgreSQL zaxira
	// tarmog'i bor edi va `MONGODB_URI` berilmasa unga JIMGINA o'tardi.
	// Bu eng yomon turdagi nosozlikni tug'dirardi: server sog'lom
	// ko'tarilardi, hech qanday xato chiqmasdi, lekin ilova BUTUNLAY
	// BOSHQA (va production'da BO'SH) katalogni ko'rsatardi.
	//
	// Endi: Mongo bor -> Mongo. Yo'q va dev -> xotira (ogohlantirish
	// bilan). Yo'q va production -> DARHOL TO'XTASH. Postgres'dagi
	// katalog jadvallari 0036 migratsiyasida o'chirilgan.
	// └────────────────────────────────────────────────────────────────┘
	if mongoURI := os.Getenv("MONGODB_URI"); mongoURI != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
		if err != nil {
			slog.Error("MongoDB konfiguratsiya xatosi", "err", err)
			os.Exit(1)
		}
		if err := client.Ping(ctx, nil); err != nil {
			slog.Error("MongoDB'ga ulanib bo'lmadi (docker compose up -d qilinganmi?)", "err", err)
			os.Exit(1)
		}
		dbName := os.Getenv("MONGODB_DB")
		if dbName == "" {
			dbName = "chustapp"
		}
		mdb := client.Database(dbName)
		if err := storage.EnsureMongoIndexes(ctx, mdb); err != nil {
			slog.Error("mongo indeks xatosi", "err", err)
			os.Exit(1)
		}
		if err := storage.EnsureMongoPromotionsIndexes(ctx, mdb); err != nil {
			slog.Error("mongo indeks xatosi (aksiyalar)", "err", err)
			os.Exit(1)
		}
		// Demo katalog (r1 "Chust Osh Markazi" + p1..p3) FAQAT dev'da —
		// `SeedDemoUsers` bilan bir xil sabab: production bazasi
		// namunaviy ma'lumot bilan to'lmasligi kerak. Mavjud
		// hujjatlarni O'CHIRMAYDI (`scripts/cleanup_demo_data.sql`).
		if devMode {
			if err := storage.SeedDemoCatalogMongo(ctx, mdb); err != nil {
				slog.Error("catalog seed xatosi (mongo)", "err", err)
				os.Exit(1)
			}
		}
		catalogRepo = storage.NewMongoCatalogRepo(mdb)
		// Kitob ombori faqat Mongo rejimida: katalog ham shu yerda.
		bookRepo = storage.NewMongoCatalogRepo(mdb)
		promotionsRepo = storage.NewMongoPromotionsRepo(mdb)
		slog.Info("rejim: MongoDB (katalog)")
	} else if devMode {
		catalogRepo = storage.NewMemoryCatalogRepo(storage.DemoRestaurants(), storage.DemoProducts())
		promotionsRepo = storage.NewMemoryPromotionsRepo()
		slog.Warn("rejim: in-memory (katalog) — MONGODB_URI berilmagan; " +
			"ma'lumot server o'chsa YO'QOLADI, faqat tez sinov uchun")
	} else {
		// Fail-closed: production'da katalogsiz ishga tushish — bu
		// "ishlayotgan, lekin bo'sh do'kon" degani. Jimgina davom
		// etgandan ko'ra to'xtagan ma'qul.
		slog.Error("MONGODB_URI berilmagan — katalog manbai yo'q. " +
			"Production'da bu MAJBURIY (katalog faqat MongoDB'da saqlanadi).")
		os.Exit(1)
	}

	// ---------- Mahsulot rasmlari: Cloudflare R2 yoki lokal disk ----------
	var imageStore images.Store
	if r2Bucket := os.Getenv("R2_BUCKET"); r2Bucket != "" {
		accountID := os.Getenv("R2_ACCOUNT_ID")
		accessKey := os.Getenv("R2_ACCESS_KEY_ID")
		secretKey := os.Getenv("R2_SECRET_ACCESS_KEY")
		publicURL := os.Getenv("R2_PUBLIC_URL")
		if accountID == "" || accessKey == "" || secretKey == "" || publicURL == "" {
			slog.Error("R2_BUCKET berilgan, lekin R2_ACCOUNT_ID/R2_ACCESS_KEY_ID/R2_SECRET_ACCESS_KEY/R2_PUBLIC_URL to'liq emas")
			os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		r2, err := images.NewR2Store(ctx, accountID, accessKey, secretKey, r2Bucket, publicURL)
		cancel()
		if err != nil {
			slog.Error("R2 konfiguratsiya xatosi", "err", err)
			os.Exit(1)
		}
		imageStore = r2
		slog.Info("rejim: Cloudflare R2 (rasm saqlash)")
		// Brauzer 3D modelni (GLB) `fetch` orqali oladi va bu boshqa
		// domendan CORS sarlavhasini TALAB qiladi. Rasmlar `<img>`
		// bilan ko'rsatilgani uchun bu ilgari kerak bo'lmagan.
		// Mavjud qoida bo'lsa tegilmaydi (`EnsurePublicReadCORS`).
		//
		// ALOHIDA kontekst: yuqoridagi `ctx` R2 klientini qurish uchun
		// edi va `cancel()` allaqachon chaqirilgan.
		corsCtx, corsCancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := r2.EnsurePublicReadCORS(corsCtx); err != nil {
			// Odatiy sabab: R2 tokenida bucket SOZLAMALARI huquqi yo'q
			// (faqat obyekt o'qish/yozish) — bu to'g'ri, eng kam huquq.
			// Mobil ilova baribir ishlaydi (`model_3d_view.dart` sahifa
			// origin'ini model domeniga qo'yadi), lekin VEB/Mini App
			// uchun qoida Cloudflare panelidan qo'lda qo'yilishi kerak:
			// R2 → bucket → Settings → CORS Policy → GET/HEAD, origin *.
			slog.Warn("R2 CORS qoidasi qo'yilmadi — veb/Mini App'da 3D model yuklanmasligi mumkin "+
				"(Cloudflare panelidan qo'lda qo'ying: R2 → bucket → Settings → CORS Policy)",
				"err", err)
		}
		corsCancel()
	} else if devMode {
		imageStore = images.NewLocalStore("uploads")
		slog.Warn("rejim: lokal disk (media saqlash) — R2_BUCKET berilmagan. " +
			"FAQAT dev uchun: fayllar server diskida qoladi va deploy'da yo'qoladi")
	} else {
		// ┌─ FAIL-CLOSED: PRODUCTION'DA FAQAT R2 ─────────────────────┐
		// Barcha media (taom rasmlari, 3D modellar, kelajakdagi video
		// va hujjatlar) obyekt omborida turishi SHART.
		//
		// Lokal disk production'da jimgina ma'lumot yo'qotadi:
		// konteyner qayta yaratilganda (har deploy) `uploads/` papkasi
		// bo'shab qoladi — restoranlar menyusidagi rasmlar va 3D
		// modellar birdaniga yo'qoladi, xato esa hech qayerda
		// ko'rinmaydi. Bu MongoDB'siz ishga tushish bilan bir xil
		// toifadagi xato, shuning uchun javob ham bir xil: to'xtash.
		// └───────────────────────────────────────────────────────────┘
		slog.Error("R2_BUCKET berilmagan — media ombori yo'q. " +
			"Production'da bu MAJBURIY: rasm, 3D model va boshqa fayllar " +
			"faqat Cloudflare R2 da saqlanadi (lokal disk deploy'da yo'qoladi).")
		os.Exit(1)
	}

	// ---------- 3D model generatsiyasi (ixtiyoriy) ----------
	// TRIPO_API_KEY berilmasa xizmat O'CHIQ bo'ladi: tegishli
	// endpointlar 503 qaytaradi, qolgan hamma narsa normal ishlaydi.
	// Bu — R2/Redis/Telegram bilan bir xil falsafa: ixtiyoriy
	// komponent hech qachon ilovani to'xtatmaydi.
	//
	// KALIT FAQAT SHU YERDA O'QILADI va hech qachon mijozga
	// yuborilmaydi (`internal/httpapi/routes_model3d.go` izohiga
	// qarang).
	var model3DSvc *model3d.Service
	var model3DLimiter *ratelimit.Limiter
	if tripoKey := strings.TrimSpace(os.Getenv("TRIPO_API_KEY")); tripoKey != "" {
		gen := model3d.NewTripoClient(
			tripoKey,
			os.Getenv("TRIPO_BASE_URL"),      // bo'sh = rasmiy manzil
			os.Getenv("TRIPO_MODEL_VERSION"), // bo'sh = provayder standarti
		)
		// Kuzatuvchi (`SetOnUpdate`) `httpapi.New` ichida ulanadi.
		model3DSvc = model3d.NewService(gen, imageStore, catalogRepo, model3d.DefaultConfig(), nil)
		// Restoran uchun: soatiga ~20 ta generatsiya, qisqa muddatda
		// 5 tagacha ketma-ket. Har biri tashqi xizmatda pul sarflaydi.
		model3DLimiter = ratelimit.New(20.0/3600.0, 5)
		slog.Info("3D model generatsiyasi yoqilgan", "provayder", gen.Name())
	} else {
		slog.Info("3D model generatsiyasi o'chiq — TRIPO_API_KEY berilmagan")
	}

	// ---------- Redis: kesh + OTP kodlar (ixtiyoriy) ----------
	// Redis bo'lmasa (REDIS_ADDR berilmagan yoki ulanib bo'lmasa) — kesh
	// butunlay o'chirilgan holda ishlaydi (har so'rov to'g'ridan-to'g'ri
	// bazaga tushadi, faqat sekinroq) va OTP kodlar yuqorida tanlangan
	// asosiy bazada (Postgres yoki xotira) qolaveradi — Mongo/R2 kabi boshqa
	// ixtiyoriy komponentlar bilan bir xil falsafa: Redis hech qachon
	// ilovani to'xtatmaydi, faqat mavjud bo'lganda tezlashtiradi.
	redisClient := cache.Connect(os.Getenv("REDIS_ADDR"))
	redisCache := cache.New(redisClient)
	if redisClient != nil {
		codeStore = storage.NewRedisCodeStore(redisClient)
		slog.Info("rejim: Redis (OTP kodlar)")
	}

	// JWT_SECRET — production'da MAJBURIY va yetarlicha uzun bo'lishi
	// shart. Avval faqat bo'sh-emaslik tekshirilardi, ya'ni bir belgili
	// kalit ham o'tib ketardi (HS256 uchun bu amalda brute-force
	// qilinadigan darajada zaif).
	const minSecretLen = 32
	jwtSecret := os.Getenv("JWT_SECRET")
	if !devMode {
		if jwtSecret == "" {
			slog.Error("JWT_SECRET majburiy (production rejim)")
			os.Exit(1)
		}
		if len(jwtSecret) < minSecretLen {
			slog.Error("JWT_SECRET juda qisqa",
				"uzunligi", len(jwtSecret), "kamida", minSecretLen)
			os.Exit(1)
		}
	} else if jwtSecret == "" {
		jwtSecret = "dev-secret-almashtiring"
		slog.Warn("JWT_SECRET berilmagan — FAQAT dev uchun mo'ljallangan standart kalit ishlatilyapti")
	}
	const tokenTTL = 30 * 24 * time.Hour
	tokens := users.NewTokenIssuer(jwtSecret, tokenTTL)
	// revoked — muddatidan oldin bekor qilingan sessiyalar (chiqish,
	// akkaunt o'chirilishi, kuryer tasdig'ining bekor qilinishi).
	// Qarang: internal/revoke.
	revokedSessions := revoke.New(redisClient, tokenTTL)

	// Tezlik cheklovi mijoz IP'siga tayanadi. `X-Forwarded-For` FAQAT
	// shu ro'yxatdagi manbalardan qabul qilinadi — aks holda istalgan
	// mijoz sarlavhani o'zi yozib, barcha IP cheklovlarini (SMS, login,
	// pullik geokodlash) chetlab o'tardi. Bo'sh bo'lsa sarlavha umuman
	// o'qilmaydi; reverse-proxy orqasiga qo'yilganda sozlash SHART.
	httpapi.SetTrustedProxies(strings.Split(os.Getenv("TRUSTED_PROXIES"), ","))

	allowedOrigins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	hub := ws.NewHub(allowedOrigins)
	wsTickets := ws.NewTicketStore()
	// ---------- Bildirishnoma qatlami ----------
	//
	// Tartib: DB'ga yozish -> WebSocket -> (ilova yopiq bo'lsa) push.
	// Yuborish HTTP so'rov yo'lini BLOKLAMAYDI (`notify.Service`).
	var notifStore notify.Store
	var tokenStore notify.TokenStore
	if pgPool != nil {
		notifStore = storage.NewPgNotificationStore(pgPool)
		tokenStore = storage.NewPgTokenStore(pgPool)
	} else {
		// Postgres yo'q (dev/test) — xotirada. Server qayta ishga
		// tushganda tarix yo'qoladi, lekin oqim bir xil ishlaydi.
		notifStore = storage.NewMemoryNotificationStore()
		tokenStore = storage.NewMemoryTokenStore()
		slog.Warn("bildirishnomalar XOTIRADA saqlanadi (DATABASE_URL yo'q) — restartda yo'qoladi")
	}
	notifSvc := notify.NewService(notifStore, hub, httpapi.NewID)

	// ---------- Qaysi ilovadan kirgani (superadmin paneli) ----------
	//
	// Har bir autentifikatsiyalangan so'rovdagi `X-Ondex-Client`
	// sarlavhasidan to'ldiriladi (`internal/httpapi/devices.go`).
	var deviceStore users.DeviceStore
	if pgPool != nil {
		deviceStore = storage.NewPgDeviceStore(pgPool)
	} else {
		deviceStore = storage.NewMemoryDeviceStore()
	}

	// FCM push — `FIREBASE_SERVICE_ACCOUNT_JSON` bo'lmasa o'chirilgan
	// holda davom etadi (SMTP/Eskiz bilan bir xil naqsh).
	if fcm, err := notify.NewFCM(firebaseServiceAccount()); err != nil {
		slog.Error("FCM sozlamasi noto'g'ri — push o'chirilgan holda davom etiladi", "err", err)
	} else if fcm != nil {
		notifSvc = notifSvc.WithPush(fcm, tokenStore)
		slog.Info("rejim: FCM push yoqilgan")
	} else {
		slog.Warn("FIREBASE_SERVICE_ACCOUNT_JSON yo'q — push yuborilmaydi (ilova yopiq bo'lsa xabar yetmaydi)")
	}

	notifier := notify.NewLive(notifSvc).
		// Stol buyurtmasi tayyor bo'lganda push kimga ketishini
		// shu funksiya hal qiladi. `notify` paketi `users` ga
		// bog'lanmasligi uchun bog'liqlik shu yerda, `main` da
		// ulanadi (`Verifier.ContactHook` bilan bir xil naqsh).
		WithWaiterLookup(func(ctx context.Context, restaurantID string) ([]string, error) {
			all, err := userRepo.ListByRole(ctx, users.RoleWaiter)
			if err != nil {
				return nil, err
			}
			ids := make([]string, 0, 4)
			for _, u := range all {
				if u.EntityID == restaurantID {
					ids = append(ids, u.ID)
				}
			}
			return ids, nil
		})
	// Email yuborish — SMS bilan bir xil naqsh: `.env` da SMTP_HOST
	// bo'lsa haqiqiy yuborish, bo'lmasa dev log. Sozlanmagan bo'lsa
	// email oqimlari ANIQ xato bilan rad etiladi (`users` paketidagi
	// `ErrEmailSendUnavailable`) — jimgina "yuborildi" deyilmaydi.
	emailSender, emailConfigured := notify.NewEmailSender()
	if emailConfigured {
		slog.Info("rejim: SMTP (email tasdiqlash yoqilgan)", "host", os.Getenv("SMTP_HOST"))
	} else if devMode {
		slog.Warn("SMTP sozlanmagan — email kodlar faqat logga yoziladi (dev)")
	} else {
		slog.Warn("SMTP sozlanmagan — email orqali ro'yxatdan o'tish/tiklash ISHLAMAYDI")
	}

	// Firebase Phone Auth — SMS kodni FIREBASE yuboradi va tekshiradi;
	// biz faqat natijadagi ID tokenni tekshiramiz. Maxfiy kalit KERAK
	// EMAS: tekshiruv Google'ning ochiq sertifikatlari bilan bajariladi,
	// shuning uchun `.env` da faqat loyiha ID'si turadi.
	firebaseVerifier := firebaseauth.New(strings.TrimSpace(os.Getenv("FIREBASE_PROJECT_ID")))
	if firebaseVerifier.ProjectID() != "" {
		slog.Info("rejim: Firebase Phone Auth yoqilgan", "project", firebaseVerifier.ProjectID())
	} else {
		slog.Warn("FIREBASE_PROJECT_ID yo'q — /auth/firebase o'chirilgan")
	}

	// ---------- OTP yetkazish zanjiri ----------
	//
	// Uchta pog'ona, har biri MUSTAQIL sozlanadi va biri yo'q bo'lsa
	// ilova keyingisiga o'tadi:
	//
	//   1. Telegram bot   — bepul; foydalanuvchi botni ochishi kerak
	//   2. Firebase       — ilova tomonida (client SDK), pullik SMS
	//   3. Eskiz.uz       — mahalliy SMS provayderi, so'mda
	//
	// Uchalasi ham OXIRIDA BIR XIL yo'lga tushadi: kod `CodeStore` ga
	// yoziladi va `POST /auth/verify` bilan tekshiriladi (Firebase
	// bundan mustasno — u o'z tokenini beradi). Shu sabab yangi
	// tasdiqlash mantiqi yozilmadi.
	var smsSender users.SmsSender = notify.LogSms{}
	if eskiz, ok := notify.NewEskizFromEnv(); ok {
		smsSender = eskiz
		slog.Info("rejim: Eskiz.uz (SMS)")
	} else if devMode {
		slog.Warn("Eskiz sozlanmagan — SMS kodlar faqat logga yoziladi (FAQAT dev)")
	} else {
		// ┌─ FAIL-CLOSED: PRODUCTION'DA SMS MAJBURIY (bug.md 47-band) ─┐
		// Avval bu yerda faqat `slog.Warn` bor edi va server `LogSms`
		// bilan ishlashda davom etardi. Ikki oqibat:
		//
		//  1. KIRISH JIMGINA BUZILADI — foydalanuvchi SMS olmaydi,
		//     lekin API "sent: true" qaytaradi;
		//  2. BARCHA OTP KODLAR log faylida ochiq turadi. Logga
		//     kirish huquqi bo'lgan har kim istalgan raqamga kirish
		//     oqimini boshlab, kodni logdan o'qib oladi — parolsiz
		//     to'liq akkaunt egallash.
		//
		// `MONGODB_URI` va `R2_BUCKET` uchun bu fayl allaqachon
		// `os.Exit(1)` qiladi. SMS — KIRISH oqimining o'zagi, ya'ni
		// undan ham muhimroq; fail-open qoldirish nomuvofiq edi.
		// └────────────────────────────────────────────────────────────┘
		slog.Error("ESKIZ_EMAIL/ESKIZ_PASSWORD berilmagan — SMS yuborilmaydi. " +
			"Production'da bu MAJBURIY: aks holda kirish jimgina buziladi va " +
			"OTP kodlar log faylida ochiq qoladi.")
		os.Exit(1)
	}

	authSvc := users.NewService(userRepo, codeStore, smsSender, tokens, httpapi.NewID)
	// Dev rejimda SMTP bo'lmasa ham email oqimini SINASH mumkin bo'lsin:
	// kod logga chiqadi va `dev_code` javobda qaytadi. Production'da
	// esa haqiqiy SMTP shart.
	authSvc = authSvc.WithEmail(emailSender, emailConfigured || devMode)

	// Telegram bot — birinchi pog'ona.
	//
	// Kod bot tomonida YARATILMAYDI: raqam tasdiqlangach `IssueCode`
	// chaqiriladi va kod odatdagi do'konga tushadi. Ya'ni bot faqat
	// YETKAZISH kanali, tasdiqlash mantiqi bitta joyda qoladi.
	//
	// `RequestCode` EMAS, aynan `IssueCode`: bot kodni chatning o'zida
	// yetkazadi, SMS kerak emas. `RequestCode` bo'lsa Eskiz nosozligi
	// Telegram orqali kirishni ham o'ldirardi (`IssueCode` izohiga
	// qarang).
	tgClient := telegram.NewClient(os.Getenv("TELEGRAM_BOT_TOKEN"))
	var tgVerifier *telegram.Verifier
	if tgClient.Configured() {
		tgVerifier = telegram.NewVerifier(tgClient,
			func(ctx context.Context, phone string) (string, error) {
				_, code, err := authSvc.IssueCode(ctx, phone)
				return code, err
			},
			users.NormalizePhone,
			5, // kod amal qilish muddati (daqiqa) — users.codeTTL bilan bir xil
		// Kutilayotgan kirish sessiyalari Redis'da ham saqlanadi:
		// busiz HAR DEPLOY o'sha daqiqada Telegram orqali kirayotgan
		// foydalanuvchilarni "havola eskirgan" holatiga tushirardi.
		// Redis yo'q bo'lsa avvalgidek faqat xotirada ishlaydi.
		).WithRedis(redisClient)
		// PUBLIC_BASE_URL — "OnDex'ga qaytish" tugmasi ishora qiladigan
		// manzil. TELEFON BRAUZERI unga chiqa olishi SHART.
		//
		// Dev'da `adb reverse tcp:8080 tcp:8080` bo'lsa `http://localhost:8080`
		// ishlaydi (telefondagi localhost kompyuterga tunnellanadi).
		// Wi-Fi orqali ishlansa LAN IP yozilishi kerak, production'da esa
		// haqiqiy domen.
		publicURL := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL"))
		// WEB_PUBLIC_BASE_URL — `apps/web`ning o'zi (masalan
		// `https://ondex.uz`), `PUBLIC_BASE_URL` (Go API domeni,
		// `api.ondex.uz`) bilan ADASHTIRMASLIK KERAK. Faqat oddiy
		// brauzerdan "Telegram bilan kirish" uchun (`StartLoginWeb`,
		// `Pending.Web`) — bo'sh bo'lsa o'sha yo'l botning "qaytish"
		// tugmasisiz qoladi (ilova yo'liga ta'sir qilmaydi).
		webPublicURL := strings.TrimSpace(os.Getenv("WEB_PUBLIC_BASE_URL"))
		// ┌─ KALIT QACHON MAJBURIY ────────────────────────────────┐
		// Avval shart `!devMode` edi, ya'ni dev'da fishing teshigi
		// HAR DOIM ochiq turardi. Aslida kalitni tushirib qoldirish
		// uchun yagona uzrli sabab bor edi: uni YETKAZIB bo'lmasligi
		// (kalit faqat "OnDex'ga qaytish" tugmasi orqali boradi,
		// tugma esa ommaviy domensiz umuman yuborilmaydi).
		//
		// Lokal Cloudflare tunnel (scripts/dev_tunnel.ps1) dev'da ham
		// haqiqiy HTTPS domen beradi. Shuning uchun shart endi
		// muhitga emas, YETKAZISH IMKONIYATIGA bog'landi: domen
		// bo'lsa — kalit majburiy, muhitidan qat'i nazar.
		//
		// Production o'zgarishsiz: u yerda `PUBLIC_BASE_URL` doim
		// bor, bo'lmasa ham `!devMode` fail-closed ushlab qoladi.
		// └────────────────────────────────────────────────────────┘
		tgVerifier = tgVerifier.
			WithPublicURL(publicURL).
			WithWebPublicURL(webPublicURL).
			WithConfirmSecretRequired(!devMode || publicURL != "").
			// ┌─ MINI APP BOG'LANISHI ────────────────────────────────┐
			// Kontakt ulashilganda telegram_id ↔ telefon saqlanadi
			// (migration 0031). Busiz Mini App foydalanuvchi kimligini
			// aniqlay olmaydi: `initData` da telefon YO'Q.
			//
			// Hook orqali ulanadi, chunki `internal/telegram` paketi
			// `internal/users` ni import qilmaydi (`WithContactHook`
			// izohiga qarang).
			// └───────────────────────────────────────────────────────┘
			WithContactHook(func(ctx context.Context, tgID int64, phone string) error {
				u, err := authSvc.LinkTelegramPhone(ctx, tgID, phone)
				if err != nil {
					return err
				}
				slog.Info("telegram: raqam bog'landi",
					"telegram_id", tgID, "user_id", u.ID)
				return nil
			})
		// Log HISOBLANGAN qiymatni yozadi, muhitni emas. Avval ikkalasi
		// ham `!devMode` ga qarardi va shart `publicURL` ni hisobga
		// oladigan bo'lgach log YOLG'ON gapira boshlagan edi: dev'da
		// kalit majburiy bo'lsa ham "talab qilinmaydi" deb yozardi.
		secretRequired := !devMode || publicURL != ""
		if !secretRequired {
			slog.Warn("Telegram bilan kirish: tasdiq kaliti TALAB QILINMAYDI " +
				"(dev, PUBLIC_BASE_URL yo'q). Bu fishingga ochiq — havolani " +
				"qurbonga yuborgan odam uning akkauntiga kira oladi. Kalitni " +
				"yoqish uchun PUBLIC_BASE_URL ga OMMAVIY domen bering " +
				"(Telegram localhost'ni rad etadi) — masalan lokal tunnel: " +
				"scripts/dev_tunnel.ps1")
		} else if publicURL == "" {
			slog.Error("PUBLIC_BASE_URL yo'q — \"Telegram bilan kirish\" " +
				"YAKUNLANMAYDI (qaytish tugmasi yuborib bo'lmaydi)")
		}
		// Ilova yo'lidan MUSTAQIL: apps/web'dagi login sahifasi (`/login`)
		// SMS ulanmagan paytda ham ishlashi uchun yagona kanal shu.
		if secretRequired && webPublicURL == "" {
			slog.Error("WEB_PUBLIC_BASE_URL yo'q — brauzerdan \"Telegram bilan " +
				"kirish\" YAKUNLANMAYDI (qaytish tugmasi yuborib bo'lmaydi)")
		}
		// `safego` — recover bilan (bug.md 44-band). Bot polling
		// sikli soatlab ishlaydi va tashqi (Telegram) ma'lumot bilan
		// oziqlanadi: u yerdagi panic butun API'ni yiqitardi.
		safego.Go("telegram.verifier", func() { tgVerifier.Run(context.Background()) })
		slog.Info("rejim: Telegram bot (OTP yetkazish) yoqilgan",
			"qaytish_manzili", publicURL, "web_qaytish_manzili", webPublicURL,
			"kalit_majburiy", secretRequired)
	} else {
		slog.Warn("TELEGRAM_BOT_TOKEN yo'q — /auth/telegram/start o'chirilgan")
	}
	orderSvc := orders.NewService(orderRepo, notifier, httpapi.NewID, promotionsRepo)
	catalogSvc := catalog.NewService(catalogRepo)

	// ── Stollar (QR kod orqali buyurtma) ──
	//
	// Katalog Mongo'da bo'lishi mumkin, lekin stollar ATAYLAB
	// Postgres/xotirada: ular buyurtmalar bilan bir xil hayot
	// siklida (`orders.table_id`) va bitta ombor ichida turgani
	// ma'qul.
	var tableRepo tables.Repository
	if pgPool != nil {
		tableRepo = storage.NewPgTableRepo(pgPool)
	} else {
		tableRepo = storage.NewMemoryTableRepo()
		slog.Warn("rejim: in-memory (stollar) — QR kodlar server qayta ishga tushganda yo'qoladi")
	}
	tableSvc := tables.NewService(tableRepo)

	// ── Tashqi AI agentlar (integratsiya sheriklari) ──
	//
	// ┌─ NEGA XOTIRA REJIMIDA O'CHIQ ────────────────────────────────┐
	// Grantlar — foydalanuvchi bergan, PUL sarflashga ruxsat
	// beruvchi yozuvlar. Ular server qayta ishga tushganda
	// yo'qolsa, sherik "token yaroqsiz" xatosini olib qoladi va
	// foydalanuvchi sababini tushunmaydi. Bundan ko'ra funksiyani
	// butunlay o'chirib, ANIQ 503 qaytargan halolroq.
	//
	// Stollar/bildirishnomalardan farqi shu: ular yo'qolsa ish
	// davom etaveradi, bu esa yarim buzilgan holat yaratardi.
	// └───────────────────────────────────────────────────────────────┘
	var agentSvc *agentapi.Service
	if pgPool != nil {
		agentSvc = agentapi.NewService(
			storage.NewPgAgentRepo(pgPool), catalogSvc, orderSvc)
		slog.Info("rejim: AI agent integratsiyasi yoqilgan (/agent/v1)")
	} else {
		slog.Warn("AI agent integratsiyasi O'CHIQ (DATABASE_URL yo'q) — /agent/v1 503 qaytaradi")
	}

	// ── Ilova ichidagi AI yordamchi (chat + ovoz) ──
	//
	// ┌─ TASHQI AGENTDAN FARQI ───────────────────────────────────────┐
	// Yuqoridagi `agentSvc` — BOSHQA server (Shaddiy ilovasi)
	// foydalanuvchi nomidan ish qilishi uchun. Bu esa OnDex
	// ilovasining O'Z chat oynasi: Shaddiy bu yerda faqat TIL
	// MODELI, tool'larni OnDex o'zi bajaradi va yordamchi buyurtma
	// YARATA OLMAYDI — u savat taklifini qaytaradi, tugmani odam
	// bosadi.
	//
	// Bazaga bog'liq EMAS (holat mijozda saqlanadi), shuning uchun
	// dev rejimda ham ishlaydi.
	// └───────────────────────────────────────────────────────────────┘
	var assistantSvc *assistant.Service
	if shaddiy, ok := assistant.NewShaddiyFromEnv(); ok {
		assistantSvc = assistant.NewService(
			shaddiy, catalogRepo, catalogSvc, orderSvc, orderRepo, orderSvc)
		slog.Info("rejim: ilova ichidagi AI yordamchi yoqilgan (/ai/chat)")
	} else {
		slog.Warn("AI yordamchi O'CHIQ (SHADDIY_AI_URL/SHADDIY_API_KEY yo'q) — /ai/* 503 qaytaradi")
	}

	// ┌─ OVOZLI REJIM (Gemini Live) ───────────────────────────────────┐
	// Matnli chatdan MUSTAQIL yoqiladi. Sabab: qurilmadagi nutq
	// tanish/sintez o'zbek tilini QO'LLAMAYDI (telefonda o'lchandi:
	// tanish `ru-RU` ga tushardi, javob esa o'zbek matnini rus ovozi
	// bilan o'qirdi). Gemini Live esa o'zbekcha tabiiy ovoz beradi.
	//
	// MAXFIYLIK: bu rejimda mikrofon oqimi serverga va u yerdan
	// Google'ga ketadi — ilova buni foydalanuvchiga aytadi va
	// roziligini so'raydi. Matnli chatda audio YO'Q.
	//
	// Yordamchining o'zi bo'lmasa ovoz ham yoqilmaydi: ovozli rejim
	// AYNAN o'sha tool'lar ustida ishlaydi.
	// └────────────────────────────────────────────────────────────────┘
	var assistantLive *assistant.LiveConfig
	if assistantSvc != nil {
		if cfg, ok := assistant.LiveConfigFromEnv(); ok {
			assistantLive = &cfg
			slog.Info("rejim: ovozli yordamchi yoqilgan (/ai/live)", "model", cfg.Model)
		} else {
			slog.Warn("Ovozli rejim O'CHIQ (GEMINI_API_KEY yo'q) — ilova mikrofon tugmasini ko'rsatmaydi")
		}
	}

	// ── Karta orqali to'lov (Octo) ──
	//
	// ┌─ SOZLANMAGAN BO'LSA TIZIM NORMAL ISHLAYDI ────────────────────┐
	// Kalitlar bo'lmasa `octoClient`/`paymentSvc` nil qoladi:
	// buyurtmalar faqat NAQD bo'ladi, to'lov endpointlari 503
	// qaytaradi, qolgan hamma narsa avvalgidek. Ya'ni to'lovni
	// bosqichma-bosqich yoqish mumkin.
	// └───────────────────────────────────────────────────────────────┘
	var octoClient *octo.Client
	var paymentSvc *payments.Service
	if shopID := strings.TrimSpace(os.Getenv("OCTO_SHOP_ID")); shopID != "" {
		id, err := strconv.ParseInt(shopID, 10, 64)
		if err != nil {
			slog.Error("OCTO_SHOP_ID raqam bo'lishi kerak", "qiymat", shopID, "err", err)
			os.Exit(1)
		}
		octoClient, err = octo.New(octo.Config{
			ShopID: id,
			Secret: os.Getenv("OCTO_SECRET"),
			// Imzo kaliti (`unique_key`) Octo texnik jamoasidan
			// alohida olinadi. Bo'sh bo'lsa imzo tekshirilmaydi va
			// to'lov FAQAT provayder API'si orqali tasdiqlanadi.
			SignatureKey: os.Getenv("OCTO_SIGNATURE_KEY"),
			// Standart holda TEST rejimi: `.env` da ataylab
			// `OCTO_TEST=false` yozilmaguncha haqiqiy pul
			// harakatlanmaydi.
			Test: !strings.EqualFold(strings.TrimSpace(os.Getenv("OCTO_TEST")), "false"),
		})
		if err != nil {
			// Kalit berilgan-u, noto'g'ri bo'lsa — JIMGINA o'chirib
			// qo'yilmaydi: aks holda karta to'lovi ishlamayotganini
			// hech kim sezmasdi.
			slog.Error("Octo sozlanmadi", "err", err)
			os.Exit(1)
		}

		var paymentRepo payments.Repository
		if pgPool != nil {
			paymentRepo = storage.NewPostgresPaymentRepo(pgPool)
		} else {
			paymentRepo = storage.NewMemoryPaymentRepo()
			slog.Warn("rejim: in-memory (to'lovlar) — server qayta ishga tushganda yo'qoladi")
		}

		notifyURL := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_API_URL")), "/") +
			"/payments/octo/callback"
		paymentSvc, err = payments.NewService(paymentRepo, octoClient, orderSvc,
			httpapi.NewID, payments.Options{
				ReturnURL:  strings.TrimSpace(os.Getenv("OCTO_RETURN_URL")),
				NotifyURL:  notifyURL,
				TTLMinutes: 30,
				// Dalilsiz callback'ga ishonish — FAQAT dev.
				// `payments.NewService` uni production'da rad etadi.
				TrustCallbackWithoutProof: devMode && strings.EqualFold(
					strings.TrimSpace(os.Getenv("OCTO_TRUST_CALLBACK_DEV")), "true"),
				Production: !devMode,
			})
		if err != nil {
			slog.Error("to'lov xizmati sozlanmadi", "err", err)
			os.Exit(1)
		}
		// Buyurtma qabul qilinganda pulni yechish / rad etilganda
		// bo'shatish shu bog'lanish orqali ishlaydi.
		orderSvc.WithPayments(paymentSvc)
		// ┌─ TUZATILGAN NOSOZLIK (bug.md 43-band) ─────────────────────┐
		// `OCTO_TEST` ning standarti — `true` (kodda ham, compose'da
		// ham). Fail-safe tanlov mantiqiy: tasodifan haqiqiy pul
		// olinmasin. LEKIN production'da bu holat hech qanday
		// ogohlantirish bermasdi — log faqat `"test", true` deb
		// yozardi, xato darajasida emas.
		//
		// Sinov rejimida karta HAQIQATDA yechilmaydi, lekin oqim
		// to'liq o'tadi: `status: succeeded` keladi, buyurtma
		// "to'langan" bo'ladi va restoranga yuboriladi. Ya'ni PUL
		// OLINMASDAN buyurtma bajariladi.
		//
		// `os.Exit(1)` EMAS — ataylab: yangi do'kon Octo bilan aynan
		// sinov rejimida integratsiyani boshlaydi va serverni
		// to'xtatish o'sha ishni imkonsiz qilardi. `slog.Error` esa
		// monitoring/alertga tushadi va ko'zdan qochmaydi.
		// └────────────────────────────────────────────────────────────┘
		if !devMode && octoClient.TestMode() {
			slog.Error("DIQQAT: karta to'lovi SINOV rejimida, lekin muhit PRODUCTION. " +
				"Pul YECHILMAYDI, buyurtma esa \"to'langan\" bo'lib restoranga ketadi. " +
				"Haqiqiy pul uchun `.env` da ANIQ `OCTO_TEST=false` yozing.")
		}
		slog.Info("rejim: karta orqali to'lov (Octo) yoqilgan",
			"shop_id", id, "test", octoClient.TestMode(), "callback", notifyURL)
	} else {
		slog.Warn("OCTO_SHOP_ID yo'q — karta orqali to'lov O'CHIQ, buyurtmalar faqat naqd")
	}

	// Dispatch matching engine — Google Distance Matrix orqali HAQIQIY ETA.
	// MUHIM: bu ham xuddi geokodlash kabi SERVER-SERVER chaqiruv, shuning
	// uchun veb (HTTP referrer bilan cheklangan) yoki Android (paket+SHA-1
	// bilan cheklangan) kaliti ISHLAMAYDI — ikkalasi ham to'g'ridan-to'g'ri
	// Go serveridan kelgan so'rovni rad etadi. Shu sabab mavjud
	// GOOGLE_GEOCODING_API_KEY (allaqachon server-server uchun, cheklovsiz/
	// IP-cheklangan) qayta ishlatiladi — Cloud Console'da shu KALITGA
	// "Distance Matrix API"ni ham qo'shib yoqish kifoya, uchinchi kalit
	// yaratish shart emas.
	distanceMatrixKey := os.Getenv("GOOGLE_GEOCODING_API_KEY")
	if distanceMatrixKey == "" {
		slog.Warn("GOOGLE_GEOCODING_API_KEY berilmagan — dispatch har doim zaxira (to'g'ri chiziq masofa) ETA'ga tushadi")
	}
	geoClient := geo.NewClient(distanceMatrixKey)
	dispatcher := couriers.NewDispatcher(courierRepo, notifier, geoClient, 20*time.Second)

	// ┌─ NEGA BU YERDA TEKSHIRILADI ──────────────────────────────────┐
	// Yuqoridagi kalitdan FARQLI o'laroq (u brauzer kaliti, referrer
	// bilan cheklangan) bu faqat `/config/maps` da o'qiladi. Ya'ni
	// yo'qligi server ishga tushganda umuman bilinmasdi — nosozlik
	// kimdir admin panelda xaritani ochganda, "config/maps -> 503"
	// degan tushunarsiz xabar bo'lib chiqardi.
	//
	// Aynan shu holat jonli uchradi: kalit VPS `.env` da bor edi-yu,
	// `docker-compose.prod.yml` uni konteynerga UZATMASDI. Startup
	// logida bitta qator bo'lganida sabab bir daqiqada topilardi.
	//
	// Fatal EMAS: xaritasiz ham platformaning qolgan hammasi ishlaydi,
	// server ko'tarilmay qolishi bundan ancha yomon bo'lardi.
	// └───────────────────────────────────────────────────────────────┘
	if os.Getenv("GOOGLE_MAPS_API_KEY") == "" {
		slog.Warn("GOOGLE_MAPS_API_KEY berilmagan — /config/maps 503 qaytaradi, " +
			"admin/restoran panelida xarita ochilmaydi " +
			"(tekshiring: .env da bormi VA docker-compose.prod.yml environment ro'yxatida bormi)")
	}

	// ---------- HTTP qatlami ----------
	// Barcha endpointlar `internal/httpapi` da (routes_*.go). Bu yerda
	// faqat bog'liqliklar yig'iladi — Express'dagi `app.js` kabi.
	api := httpapi.New(httpapi.Deps{
		OrderRepo:      orderRepo,
		CourierRepo:    courierRepo,
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		BookRepo:       bookRepo,
		PromotionsRepo: promotionsRepo,
		FavoritesRepo:  favoritesRepo,
		Cache:          redisCache,
		Tokens:         tokens,
		Revoked:        revokedSessions,
		Hub:            hub,
		WsTickets:      wsTickets,
		ImageStore:     imageStore,
		AuthSvc:        authSvc,
		OrderSvc:       orderSvc,
		CatalogSvc:     catalogSvc,
		TableSvc:       tableSvc,
		Payments:       paymentSvc,
		OctoClient:     octoClient,
		Dispatcher:     dispatcher,
		DevMode:        devMode,
		// SMTP ulangan bo'lsa email kodi javobda QAYTARILMAYDI —
		// u haqiqatan pochtaga boradi (`Deps.EmailConfigured` izohi).
		EmailConfigured: emailConfigured,
		// Email orqali KIRISH — standart holda O'CHIQ. Mijoz ilovasida
		// bu yo'l olib tashlangan (ROADMAP 62-band), SMTP esa
		// chek/bildirishnoma uchun ishlashda davom etadi.
		EmailLoginEnabled: strings.EqualFold(
			strings.TrimSpace(os.Getenv("EMAIL_LOGIN_ENABLED")), "true"),
		Firebase: firebaseVerifier,
		Telegram: tgVerifier,
		// Mini App `initData` imzosini tekshirish uchun (server.go izohi).
		TelegramBotToken: strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")),
		Notifications:    notifStore,
		PushTokens:       tokenStore,
		Notifier:         notifSvc,
		AgentSvc:         agentSvc,
		Assistant:        assistantSvc,
		AssistantLive:    assistantLive,
		Devices:          deviceStore,
		Model3D:          model3DSvc,
		Model3DLimiter:   model3DLimiter,
		// Yuklash: xodim uchun daqiqasiga ~6 ta, qisqa muddatda 12
		// tagacha ketma-ket. Menyuni to'ldirish (bir necha o'nlab rasm)
		// bemalol sig'adi, 25 MB li PDF larni ketma-ket haydash esa
		// yo'q — R2 da joy ham, PDF tahlili ham pul turadi.
		UploadLimiter: ratelimit.New(6.0/60.0, 12),
	})

	// Tugallanmagan 3D vazifalarni davom ettiramiz. Server qayta ishga
	// tushganda kuzatuvchi gorutinalar yo'qoladi va mahsulot abadiy
	// "tayyorlanmoqda" holatida qolardi (`ResumePending` izohiga
	// qarang). Fon rejimida — ishga tushishni sekinlashtirmasin.
	if model3DSvc != nil {
		safego.Go("model3d.resumePending", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			model3DSvc.ResumePending(ctx)
		})
	}

	// Dispatch tiklash (crash-recovery) — dispatch holati FAQAT xotirada
	// (Dispatcher.pending map + fon goroutine) saqlanadi. Server process
	// biror sababdan (deploy, qulash, qayta ishga tushirish) o'chib-yonsa,
	// avvalgi dispatch IZSIZ yo'qoladi — "accepted" holatida qolib ketgan-u
	// hali kuryer biriktirilmagan buyurtma hech qachon qayta qidirilmay,
	// abadiy shu holatda qotib qolardi. Shuning uchun HAR server startida
	// so'nggi buyurtmalar orasidan aynan shunday holatdagilarni topib,
	// ularga dispatch qayta boshlanadi.
	{
		const recoveryWindow = 500
		recent, err := orderRepo.ListRecent(context.Background(), recoveryWindow)
		if err != nil {
			slog.Error("dispatch tiklashda buyurtmalarni o'qib bo'lmadi", "err", err)
		}
		// Oyna to'lgan bo'lsa, undan ESKIROQ osilib qolgan buyurtma
		// ko'rinmay qolgan bo'lishi mumkin. Hozirgi hajmda bu uzoq —
		// lekin jimgina o'tib ketmasligi uchun ogohlantiramiz, aks
		// holda "bitta buyurtma kuryersiz qoldi" muammosining sababi
		// hech qachon topilmasdi.
		if len(recent) >= recoveryWindow {
			slog.Warn("dispatch tiklash oynasi to'ldi — undan eski osilib qolgan buyurtmalar tekshirilmadi",
				"oyna", recoveryWindow)
		}
		for _, o := range recent {
			// Shart ATAYLAB shu yerda yozilmaydi: u `routes_orders.go`
			// dagi dispatch shartidan ajralib ketib, stol buyurtmalarini
			// kuryerlarga yuborardi. Yagona manba —
			// `orders.Order.NeedsDispatchRecovery` (izohi o'sha yerda).
			if o.NeedsDispatchRecovery() {
				slog.Warn("dispatch tiklanmoqda (server qayta ishga tushgandan keyin topilgan kuryersiz buyurtma)",
					"order", o.ID, "order_number", o.OrderNumber)
				oID, rID, prep := o.ID, o.RestaurantID, o.PreparationMinutes
				api.RecoverDispatch(oID, rID, prep)
			}
		}
	}

	addr := ":8080"
	// ANIQ timeout'lar — `http.ListenAndServe` nol-qiymatli serverdan
	// foydalanadi, ya'ni o'qish/yozish CHEKSIZ kutadi. Bir necha o'nlab
	// sekin (baytma-bayt yozadigan) ulanish barcha goroutine'larni band
	// qilib, serverni arzimas kuch bilan to'xtatishi mumkin (Slowloris).
	srv := &http.Server{
		Addr:              addr,
		Handler:           api.Routes(allowedOrigins),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		// Yozish uchun kengroq: rasm yuklash (5MB) sekin tarmoqda
		// uzoqroq davom etishi mumkin.
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown — SIGTERM/SIGINT kelganda ishlab turgan
	// so'rovlar tugatiladi, DB pool'lari yopiladi (avval jarayon
	// darhol o'lardi va so'rovlar yarim yo'lda uzilardi).
	shutdownDone := make(chan struct{})
	go func() {
		// `close(shutdownDone)` ATAYLAB `defer` da va `safego.Run` dan
		// TASHQARIDA (bug.md 44-band): shu goroutine'da panic bo'lsa
		// ham `main` `<-shutdownDone` da abadiy osilib qolmasligi
		// kerak — aks holda konteyner SIGKILL kutgan bo'lardi.
		defer close(shutdownDone)
		safego.Run("api.shutdown", func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			<-sigCh
			slog.Info("to'xtatish signali qabul qilindi — so'rovlar yakunlanmoqda")
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				slog.Error("graceful shutdown xatosi", "err", err)
			}
		})
	}()

	slog.Info("ChustApp API ishga tushdi", "addr", addr, "dev_mode", devMode)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server to'xtadi", "err", err)
		os.Exit(1)
	}
	<-shutdownDone
	slog.Info("server to'xtadi")
}
