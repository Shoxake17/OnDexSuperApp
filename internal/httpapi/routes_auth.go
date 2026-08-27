package httpapi

import (
	"chustapp/internal/ratelimit"
	"chustapp/internal/telegram"
	"chustapp/internal/users"
	"encoding/json"
	"errors"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// authStatus — auth xatosi uchun HTTP holati.
//
// MUHIM: ro'yxatda YO'Q xato (pgx/mongo drayveri, tarmoq, kutilmagan
// nosozlik) 500 ga aylantiriladi va `httpError` uning matnini
// mijozdan YASHIRADI. Avval bu handlerlar har qanday xatoni 400/401
// qilib qaytarardi, ya'ni bazadagi unikal indeks buzilishi mijozga
// cheklov nomi bilan borardi.
func authStatus(err error, def int) int {
	if !users.IsUserFacing(err) {
		return http.StatusInternalServerError
	}
	switch {
	case errors.Is(err, users.ErrTooSoon), errors.Is(err, users.ErrTooManyAttempts):
		return http.StatusTooManyRequests
	case errors.Is(err, users.ErrPhoneNotVerified):
		return http.StatusForbidden
	case errors.Is(err, users.ErrServerBusy):
		// Parol tekshirish navbati to'lgan (Argon2 chegarasi). Bu
		// VAQTINCHALIK holat — 4xx emas, 503 bo'lishi kerak, aks holda
		// mijoz uni "noto'g'ri parol" deb talqin qilardi.
		return http.StatusServiceUnavailable
	}
	return def
}

// respondAuthError — auth xatosini javobga aylantiradi.
//
// Kutish vaqti MA'LUM bo'lgan xato (`users.TooSoonError`) uchun
// foydalanuvchiga ANIQ muddat ko'rsatiladi va `Retry-After` sarlavhasi
// qo'yiladi; qolganlari odatdagi yo'l bilan ketadi.
//
// HAMMA auth handleri SHU funksiyani ishlatadi. Avval ularning
// ko'pchiligi to'g'ridan-to'g'ri `httpError(w, authStatus(...))` ni
// chaqirardi va `TooSoonError` ichidagi aniq muddat YO'QOLARDI —
// foydalanuvchi "juda ko'p urinish, biroz kuting" dan boshqa hech
// narsa ko'rmasdi. Ro'yxatdan o'tishda (60 soniyalik kod pauzasi)
// bu eng ko'p uchraydigan holat edi.
func respondAuthError(w http.ResponseWriter, err error, def int) {
	var tooSoon users.TooSoonError
	if errors.As(err, &tooSoon) {
		tooManyRequests(w, tooSoon.Wait)
		return
	}
	httpError(w, authStatus(err, def), err)
}

// decodeJSON — so'rov tanasini o'qiydi.
//
// Xato bo'lsa javobni O'ZI yozadi va `false` qaytaradi. Parser matni
// ("invalid character 'l' looking for ...") mijozga hech narsa bermaydi
// va ichki tafsilot oqizadi, shuning uchun u UZATILMAYDI.
//
// Tana o'lchami `withBodyLimit` bilan 1 MB ga cheklangani uchun
// dekodlashning o'zi xavf tug'dirmaydi.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		httpError(w, http.StatusBadRequest, errors.New("so'rov formati noto'g'ri"))
		return false
	}
	return true
}

// decodeRaw — `decodeJSON` bilan bir xil, lekin XOM tanani ham
// qaytaradi.
//
// Nega kerak: to'lov provayderining callback'i nizo chiqqanda yagona
// dalil bo'ladi, shuning uchun u AYNAN kelgan holida bazaga yoziladi.
// Struct'ga dekodlangan nusxa yetarli emas — undagi noma'lum
// maydonlar yo'qoladi.
func decodeRaw(w http.ResponseWriter, r *http.Request, dst any) ([]byte, bool) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		httpError(w, http.StatusBadRequest, errors.New("so'rovni o'qib bo'lmadi"))
		return nil, false
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		httpError(w, http.StatusBadRequest, errors.New("so'rov formati noto'g'ri"))
		return nil, false
	}
	return raw, true
}

// rateLimited — cheklovni tekshiradi. Rad etilsa javobni (aniq kutish
// vaqti bilan) O'ZI yozadi va `true` qaytaradi.
//
// Chaqirish: `if rateLimited(w, otpIPLimiter, clientIP(r)) { return }`
func rateLimited(w http.ResponseWriter, l *ratelimit.Limiter, key string) bool {
	ok, wait := l.AllowWithWait(key)
	if ok {
		return false
	}
	tooManyRequests(w, wait)
	return true
}

// rateLimitedBoth — ikkita cheklov (odatda IP + global) uchun.
func rateLimitedBoth(w http.ResponseWriter,
	a *ratelimit.Limiter, ka string, b *ratelimit.Limiter, kb string) bool {
	ok, wait := allowBoth(a, ka, b, kb)
	if ok {
		return false
	}
	tooManyRequests(w, wait)
	return true
}

// telegramReady / firebaseReady — xizmat sozlanganmi. Sozlanmagan
// bo'lsa javobni o'zi yozadi.
// emailLoginReady — email orqali KIRISH/RO'YXATDAN O'TISH yoqilganmi.
//
// ┌─ NEGA BAYROQ ─────────────────────────────────────────────────────┐
// Mijoz ilovasida email tabi olib tashlandi (ROADMAP 62-band), lekin
// UI'ni yashirish hujum yuzasini YOPMAYDI: endpointlar baribir
// ommaviy va istalgan odam ularni to'g'ridan-to'g'ri chaqira oladi.
// Amalda ochiq qolgan narsalar:
//
//	* `/auth/email/request-code` — istalgan manzilga xat yuborishga
//	  majburlash (rate-limit bor, lekin domen obro'si va Resend
//	  kvotasi baribir yeyiladi);
//	* `/auth/email/verify` va email rejimidagi `/auth/register` —
//	  hech bir mijoz ilovasi ishlatmaydigan akkaunt yaratish yo'li.
//
// Ishlatilmaydigan, lekin ochiq turgan kirish yo'li — bu qarzdorlik.
// Bayroq uni yopadi, kod esa xodim panellari uchun joyida qoladi.
// └───────────────────────────────────────────────────────────────────┘
func (s *Server) emailLoginReady(w http.ResponseWriter) bool {
	if !s.EmailLoginEnabled {
		httpError(w, http.StatusNotFound,
			errors.New("email orqali kirish yoqilmagan"))
		return false
	}
	return true
}

func (s *Server) telegramReady(w http.ResponseWriter) bool {
	if s.Telegram == nil || !s.Telegram.Configured() {
		httpError(w, http.StatusServiceUnavailable,
			errors.New("Telegram bot sozlanmagan"))
		return false
	}
	return true
}

func (s *Server) firebaseReady(w http.ResponseWriter, what string) bool {
	if s.Firebase == nil || s.Firebase.ProjectID() == "" {
		httpError(w, http.StatusServiceUnavailable, errors.New(what+" sozlanmagan"))
		return false
	}
	return true
}

// devCodeVisible — kodni HTTP javobida qaytarish mumkinmi.
//
// Qoida: kod javobda faqat HAQIQIY YETKAZISH KANALI YO'Q bo'lganda
// qaytariladi. Hozir SMS provayderi (Eskiz.uz) ulanmagan — telefon
// kodini boshqa yo'l bilan olib bo'lmaydi, shuning uchun dev rejimda
// u javobda qoladi. Email esa SMTP ulangan zahoti HAQIQATAN pochtaga
// boradi va javobda takrorlash zararli bo'ladi (`Deps.EmailConfigured`
// izohiga qarang).
func (s *Server) devCodeVisible(viaEmail bool) bool {
	if !s.DevMode {
		return false
	}
	return !(viaEmail && s.EmailConfigured)
}

func (s *Server) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/request-code", func(w http.ResponseWriter, r *http.Request) {
		// SMS — HAQIQIY pul. Per-telefon 60s pauza `users.Service`da bor,
		// lekin u skriptni turli raqamlarni aylanib chiqishdan
		// to'xtatmaydi. Shu sabab IP bo'yicha ham, global ham cheklov.
		if rateLimitedBoth(w, otpIPLimiter, clientIP(r), otpGlobalLimiter, "global") {
			return
		}
		var req struct {
			Phone string `json:"phone"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		phone, code, err := s.AuthSvc.RequestCode(r.Context(), req.Phone)
		if err != nil {
			respondAuthError(w, err, http.StatusBadRequest)
			return
		}
		resp := map[string]any{"sent": true, "phone": phone}
		if s.DevMode {
			resp["dev_code"] = code // faqat dev: SMS o'rniga kod javobda
		}
		writeJSON(w, http.StatusOK, resp)
	})

	// POST /auth/register — ro'yxatdan o'tish (image/register.png).
	//
	// TOKEN QAYTARMAYDI: akkaunt `phone_verified = false` holatida
	// yaratiladi va kirish faqat `POST /auth/verify` (SMS kod)
	// muvaffaqiyatli bo'lgandan keyin ochiladi. Busiz istalgan odam
	// BEGONA raqam bilan akkaunt ochib, raqamni band qilib qo'yardi.
	mux.HandleFunc("POST /auth/register", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Phone           string `json:"phone"`
			Email           string `json:"email"`
			FirstName       string `json:"first_name"`
			LastName        string `json:"last_name"`
			Password        string `json:"password"`
			PasswordConfirm string `json:"password_confirm"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		in := users.RegisterInput{
			Phone:           req.Phone,
			Email:           req.Email,
			FirstName:       req.FirstName,
			LastName:        req.LastName,
			Password:        req.Password,
			PasswordConfirm: req.PasswordConfirm,
		}
		// TARTIB MUHIM: forma tekshiruvi tezlik cheklovidan OLDIN.
		//
		// `otpIPLimiter` IP bo'yicha bor-yo'g'i 5 ta portlashga ruxsat
		// beradi (tiklanish ~5 daqiqada bitta). Avval cheklov eng
		// birinchi turardi, ya'ni "parollar mos kelmadi" kabi oddiy
		// forma xatosi ham bitta tokenni yoqardi va formani besh marta
		// xato to'ldirgan odam ~25 daqiqaga bloklanardi. Endi forma
		// xatosi BEPUL. Tana o'lchami `withBodyLimit` bilan 1 MB ga
		// cheklangani uchun dekodlashning o'zi xavf tug'dirmaydi.
		v, err := users.ValidateRegisterInput(in)
		if err != nil {
			httpError(w, http.StatusBadRequest, err)
			return
		}
		// EMAIL rejimi bayroq ostida (`emailLoginReady` izohi).
		// Telefon rejimida email IXTIYORIY profil ma'lumoti bo'lib
		// qolaveradi — u kimlik emas (`users.emailIsIdentity`).
		if !v.ByPhone && !s.emailLoginReady(w) {
			return
		}
		// Cheklov ASOSIY ISHDAN (Argon2 64 MB, baza, SMS) oldin
		// sarflanadi — busiz Argon2 ning o'zi DoS vositasi bo'lardi.
		if rateLimitedBoth(w, otpIPLimiter, clientIP(r), otpGlobalLimiter, "global") {
			return
		}
		code, err := s.AuthSvc.Register(r.Context(), in)
		if err != nil {
			respondAuthError(w, err, http.StatusBadRequest)
			return
		}
		// JAVOB HAR DOIM BIR XIL — band raqam/email uchun ham. Aks
		// holda bu endpoint "bu raqam ro'yxatda bormi?" degan savolga
		// bepul javob beruvchi vositaga aylanardi (`Register` izohiga
		// qarang).
		resp := map[string]any{"registered": true, "verification_required": true}
		if s.devCodeVisible(!v.ByPhone) && code != "" {
			resp["dev_code"] = code
		}
		writeJSON(w, http.StatusCreated, resp)
	})

	// POST /auth/login — telefon YOKI email + parol.
	mux.HandleFunc("POST /auth/login", func(w http.ResponseWriter, r *http.Request) {
		// Parol bilan kirish — brute-force / credential-stuffing ning
		// asosiy nishoni. IP bo'yicha VA login (telefon/email) bo'yicha
		// alohida cheklov: birinchisi bitta manbadan ommaviy urinishni,
		// ikkinchisi esa ko'p IP'dan bitta akkauntga hujumni to'sadi.
		if rateLimited(w, loginIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			Login    string `json:"login"` // telefon yoki email
			Password string `json:"password"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		// UZUNLIK CHEGARASI cheklovchidan OLDIN.
		//
		// `loginAccountLimiter` kaliti — aynan shu satr. Tana 1 MB
		// gacha bo'lishi mumkin, ya'ni har so'rov cheklovchi xaritasiga
		// ulkan kalit qo'shardi. Haqiqiy login esa hech qachon
		// `MaxEmailLength` dan uzun bo'lmaydi.
		if len(req.Login) > users.MaxEmailLength ||
			len(req.Password) > users.MaxPasswordLength {
			httpError(w, http.StatusUnauthorized, users.ErrInvalidCredentials)
			return
		}
		if rateLimited(w, loginAccountLimiter,
			strings.ToLower(strings.TrimSpace(req.Login))) {
			return
		}
		token, u, err := s.AuthSvc.LoginWithPassword(r.Context(), req.Login, req.Password)
		if err != nil {
			respondAuthError(w, err, http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
	})

	// POST /auth/verify  {"phone":"+998901234567","code":"123456"} -> {token, user}
	mux.HandleFunc("POST /auth/verify", func(w http.ResponseWriter, r *http.Request) {
		// Bu endpoint AVVAL UMUMAN cheklanmagan edi. Kodning o'zi
		// per-telefon 5 urinish bilan himoyalangan, lekin aynan SHU
		// mexanizmni hujum vositasiga aylantirish mumkin edi: begona
		// raqamga 6 ta soxta kod yuborilsa, `Verify` urinishlar
		// tugagani uchun QURBONNING HAQIQIY KODINI o'chirib yuborardi.
		// Buni takrorlab, ma'lum bir raqamni SMS bilan kirishdan
		// doimiy uzib qo'yish mumkin edi — arzon va aniq nishonli DoS.
		//
		// IP cheklovi buni bitta manbadan amalda imkonsiz qiladi.
		// Taqsimlangan (ko'p IP'li) hujum qolgan xavf bo'lib qoladi —
		// uni to'liq yopish uchun telefon bo'yicha ham cheklov kerak,
		// lekin u halol foydalanuvchini bloklamasligi uchun kodning
		// o'z `maxAttempts` mexanizmi bilan birga o'ylanishi kerak.
		if rateLimited(w, verifyIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			Phone string `json:"phone"`
			Code  string `json:"code"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		token, u, err := s.AuthSvc.Verify(r.Context(), req.Phone, req.Code)
		if err != nil {
			respondAuthError(w, err, http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
	})

	// POST /auth/telegram/start — Telegram bot orqali kod olish.
	//
	// Javob: `{deep_link: "https://t.me/<bot>?start=<token>"}`.
	// Ilova shu havolani ochadi; qolgani bot tomonida bo'ladi
	// (`internal/telegram` paketi izohiga qarang).
	//
	// KOD SHU YERDA YARATILMAYDI. U faqat foydalanuvchi botda
	// RAQAMINI ULASHGANDAN va u ilovada kiritilgan raqam bilan MOS
	// KELGANDAN keyin yaratiladi. Busiz hujumchi qurbonning raqamini
	// yozib, botni o'zining Telegramida ochib, kodni olib ketardi.
	mux.HandleFunc("POST /auth/telegram/start", func(w http.ResponseWriter, r *http.Request) {
		if !s.telegramReady(w) {
			return
		}
		var req struct {
			Phone string `json:"phone"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		phone, err := users.NormalizePhone(req.Phone)
		if err != nil {
			httpError(w, http.StatusBadRequest, err)
			return
		}
		// SMS'ga qaraganda arzon (pul ketmaydi), lekin cheklovsiz
		// qoldirilsa bot spam yuborish vositasiga aylanardi.
		if rateLimited(w, telegramIPLimiter, clientIP(r)) {
			return
		}
		link, err := s.Telegram.Start(r.Context(), phone)
		if err != nil {
			slog.Warn("telegram: start xatosi", "err", err)
			httpError(w, http.StatusBadGateway,
				errors.New("Telegram bilan bog'lanib bo'lmadi"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deep_link": link})
	})

	// POST /auth/firebase — Firebase Phone Auth orqali kirish.
	//
	// Ilova SMS kodni FIREBASE bilan tekshiradi (kodni biz yubormaymiz
	// ham, tekshirmaymiz ham) va natijada olingan ID tokenni shu yerga
	// yuboradi.
	//
	// ┌─ ENG MUHIM ──────────────────────────────────────────────────┐
	// Telefon raqami SO'ROVDAN OLINMAYDI. U faqat imzosi tekshirilgan
	// token ICHIDAN olinadi. Aks holda hujumchi
	// `{"phone":"+998901234567"}` deb yuborib istalgan akkauntga
	// kirardi va butun Firebase integratsiyasi bezak bo'lib qolardi.
	// └──────────────────────────────────────────────────────────────┘
	mux.HandleFunc("POST /auth/firebase", func(w http.ResponseWriter, r *http.Request) {
		if !s.firebaseReady(w, "Firebase") {
			return
		}
		// Token tekshiruvi RSA imzo hisoblashni talab qiladi — cheklovsiz
		// bu CPU'ni yeydigan hujum vositasi bo'lardi.
		if rateLimited(w, loginIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			IDToken string `json:"id_token"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		tok, err := s.Firebase.Verify(r.Context(), req.IDToken)
		if err == nil {
			// Bu endpoint FAQAT telefon bilan berilgan token uchun.
			// Busiz Google tokeni ham shu yerga kelib, raqamsiz
			// akkaunt yaratib yuborardi.
			err = tok.RequirePhone()
		}
		if err != nil {
			// Tafsilot berilmaydi: "kalit topilmadi" / "muddati o'tgan"
			// kabi farqlar hujumchiga tokenni moslashtirishda yordam
			// beradi. Server tomonda to'liq sabab log qilinadi.
			slog.Warn("firebase token rad etildi", "err", err, "ip", clientIP(r))
			httpError(w, http.StatusUnauthorized, errors.New("tasdiqlash amalga oshmadi"))
			return
		}
		token, u, err := s.AuthSvc.LoginWithFirebasePhone(r.Context(), tok.Phone)
		if err != nil {
			respondAuthError(w, err, http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
	})

	// POST /auth/telegram/login/start — "Telegram bilan kirish".
	//
	// `/auth/telegram/start` dan FARQI: u yerda foydalanuvchi avval
	// raqamini yozadi va bot uni SOLISHTIRADI. Bu yerda esa hech
	// narsa yozilmaydi — kimlikni Telegram belgilaydi.
	//
	// Xavfsizlik jihatidan ikkalasi teng kuchda, chunki raqamni
	// ikkala holatda ham TELEGRAM yuboradi (`request_contact`),
	// foydalanuvchi uni qo'lda kirita olmaydi.
	mux.HandleFunc("POST /auth/telegram/login/start", func(w http.ResponseWriter, r *http.Request) {
		if !s.telegramReady(w) {
			return
		}
		if rateLimited(w, telegramIPLimiter, clientIP(r)) {
			return
		}
		link, token, err := s.Telegram.StartLogin(r.Context())
		if err != nil {
			slog.Warn("telegram: login start xatosi", "err", err)
			httpError(w, http.StatusBadGateway,
				errors.New("Telegram bilan bog'lanib bo'lmadi"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"deep_link": link,
			"token":     token,
		})
	})

	// POST /auth/telegram/miniapp  {"init_data": "..."}
	//
	// ┌─ TELEGRAM MINI APP KIRISHI ───────────────────────────────────┐
	// Mini App ochilganda Telegram sahifaga `initData` beradi. U
	// KLIENT tomonida turadi, ya'ni o'zgartirilishi mumkin —
	// shuning uchun imzo bot tokeni bilan HMAC-SHA256 orqali
	// tekshiriladi (`telegram.ValidateInitData`).
	//
	// Tekshiruvsiz istalgan odam `user.id` ni almashtirib BEGONA
	// AKKAUNTGA kirardi. Bu Mini App'lardagi eng ko'p uchraydigan
	// zaiflik.
	//
	// ── TELEFON RAQAMI ──
	// `initData` da telefon YO'Q va hech qachon bo'lmaydi. Shuning
	// uchun kirish faqat botda kontakt ULASHILGAN bo'lsa ishlaydi
	// (migration 0031, `users.LinkTelegramPhone`). Bog'lanmagan
	// bo'lsa 409 va bot havolasi qaytariladi — klient foydalanuvchini
	// o'sha yerga yo'naltiradi.
	// └───────────────────────────────────────────────────────────────┘
	mux.HandleFunc("POST /auth/telegram/miniapp", func(w http.ResponseWriter, r *http.Request) {
		if !s.telegramReady(w) {
			return
		}
		// Imzo tekshiruvi arzon, lekin cheksiz urinish imzo tanlashga
		// (va CPU sarfiga) yo'l ochardi.
		if rateLimitedBoth(w, loginIPLimiter, clientIP(r),
			telegramPollLimiter, clientIP(r)) {
			return
		}
		var req struct {
			InitData string `json:"init_data"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		// Uzunlik chegarasi: haqiqiy `initData` ~500-1500 belgi.
		if len(req.InitData) > 4096 {
			httpError(w, http.StatusBadRequest, errors.New("init_data juda uzun"))
			return
		}
		tgUser, err := telegram.ValidateInitData(req.InitData, s.TelegramBotToken, time.Now())
		if err != nil {
			// Sabab OSHKOR QILINMAYDI: "imzo noto'g'ri" va "muddati
			// o'tgan" farqi hujumchiga imzo tanlashda ma'lumot berardi.
			slog.Warn("miniapp: initData rad etildi", "err", err, "ip", clientIP(r))
			httpError(w, http.StatusUnauthorized, errors.New("Telegram ma'lumoti tasdiqlanmadi"))
			return
		}

		token, u, err := s.AuthSvc.LoginWithTelegramID(r.Context(), tgUser.ID)
		if errors.Is(err, users.ErrTelegramNotLinked) {
			// 409 — "kirish rad etildi" EMAS, "yana bir qadam kerak".
			// Klient buni ko'rib botga yo'naltiradi.
			link := ""
			if name, e := s.Telegram.BotUsername(r.Context()); e == nil && name != "" {
				link = "https://t.me/" + name
			}
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":     "Raqamingiz hali bog'lanmagan",
				"need":      "share_contact",
				"bot_link":  link,
				"first_name": tgUser.FirstName,
			})
			return
		}
		if err != nil {
			respondAuthError(w, err, http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
	})

	// GET /auth/telegram/login/status?token=... — ilova shu yerni
	// so'rab turadi (polling) va tasdiq kelishi bilan token oladi.
	//
	// Natija BIR MARTALIK: token ilova va server o'rtasida ochiq
	// yuradi, shuning uchun u bilan ikkinchi marta kirish tokenini
	// olib bo'lmaydi.
	mux.HandleFunc("GET /auth/telegram/login/status", func(w http.ResponseWriter, r *http.Request) {
		if !s.telegramReady(w) {
			return
		}
		// Polling tez-tez chaqiriladi, shuning uchun cheklov ham
		// yumshoqroq — lekin cheksiz emas (token topish urinishlari).
		if rateLimited(w, telegramPollLimiter, clientIP(r)) {
			return
		}
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			httpError(w, http.StatusBadRequest, errors.New("token yo'q"))
			return
		}
		found, done, _ := s.Telegram.LoginResult(token)
		if !found {
			httpError(w, http.StatusNotFound,
				errors.New("so'rov eskirgan — qaytadan boshlang"))
			return
		}
		if !done {
			writeJSON(w, http.StatusOK, map[string]any{"pending": true})
			return
		}
		// `c` — bot yuborgan "OnDex'ga qaytish" havolasidagi maxfiy
		// kalit (`telegram.Pending.ConfirmSecret`).
		//
		// U MAJBURIYMI yoki YO'Q — buni FAQAT `ConsumeLogin` hal
		// qiladi (`Verifier.requireSecret`: production'da majburiy,
		// dev'da yo'q, chunki Telegram localhost'ga tugma yubora
		// olmaydi).
		//
		// DIQQAT: bu yerda "kalit bo'sh bo'lsa pending" degan
		// tekshiruv BO'LMASLIGI kerak. Avval shunday edi va dev
		// rejimini butunlay ishdan chiqargan edi: ilova bo'sh `c`
		// yuborardi, server esa tasdiq allaqachon kelgan bo'lsa ham
		// abadiy "pending" qaytarardi (cheksiz yuklanish).
		secret := strings.TrimSpace(r.URL.Query().Get("c"))
		phone, ok := s.Telegram.ConsumeLogin(token, secret)
		if !ok {
			// Noto'g'ri kalit ham "pending" — javobdagi farqning o'zi
			// hujumchiga kalit tanlashda ma'lumot berardi.
			writeJSON(w, http.StatusOK, map[string]any{"pending": true})
			return
		}
		// Raqam TELEGRAM tomonidan tasdiqlangan — telefon oqimi bilan
		// bir xil yo'l (akkaunt topiladi yoki yaratiladi).
		authToken, u, err := s.AuthSvc.LoginWithFirebasePhone(r.Context(), phone)
		if err != nil {
			respondAuthError(w, err, http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": authToken, "user": u})
	})

	// GET /auth/telegram/return?c=... — botdagi "OnDex'ga qaytish"
	// tugmasi shu yerga keladi va ilovaga YO'NALTIRADI.
	//
	// NEGA KERAK: Telegram inline tugmasida faqat `http(s)://` va
	// `tg://` havolalariga ruxsat beradi, `ondex://` esa rad etiladi
	// (`telegram.returnPath` izohiga qarang). Shu sabab tugma bizga
	// ishora qiladi, biz esa 302 bilan ilovani ochamiz.
	//
	// XAVFSIZLIK: bu yerda HECH NARSA tekshirilmaydi va hech narsa
	// berilmaydi — kalit shunchaki qayta uzatiladi. U qurilmadagi
	// ilovaga tushgandan keyingina `/auth/telegram/login/status` da
	// ishlaydi, va faqat SHU so'rovni boshlagan ilovada kuzatish
	// tokeni bor. Ya'ni havolani birov ushlab qolsa ham, tokensiz u
	// bilan hech narsa qila olmaydi.
	mux.HandleFunc("GET /auth/telegram/return", func(w http.ResponseWriter, r *http.Request) {
		secret := strings.TrimSpace(r.URL.Query().Get("c"))
		if secret == "" {
			httpError(w, http.StatusBadRequest, errors.New("kalit yo'q"))
			return
		}
		target := "ondex://auth?c=" + url.QueryEscape(secret)
		// Brauzer custom sxemani ochadi. Ba'zi brauzerlar 302'ni
		// bevosita ochmaydi, shuning uchun sahifada qo'lda bosiladigan
		// havola ham qoldiriladi.
		w.Header().Set("Location", target)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Bu YAGONA HTML javob. Umumiy `default-src 'none'` siyosati
		// inline `style` atributlarini ham bloklaydi va sahifa
		// bezaksiz ko'rinardi. Skript baribir TAQIQLANGAN.
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; style-src 'unsafe-inline'; "+
				"frame-ancestors 'none'; base-uri 'none'")
		w.WriteHeader(http.StatusFound)
		_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8">` +
			`<meta name="viewport" content="width=device-width,initial-scale=1">` +
			`<title>OnDex</title>` +
			`<meta http-equiv="refresh" content="0;url=` + html.EscapeString(target) + `">` +
			`<body style="font-family:sans-serif;text-align:center;padding:48px 24px">` +
			`<p>OnDex ochilmoqda…</p>` +
			`<p><a href="` + html.EscapeString(target) + `">Ilovani ochish</a></p>`))
	})

	// POST /auth/google — Google hisobi bilan kirish/ro'yxatdan o'tish.
	//
	// Ilova Google hisobini tanlaydi va Firebase ID tokenini yuboradi.
	// Bu yerda AYNAN o'sha tekshiruv ishlaydi (imzo, `aud`, `iss`,
	// `exp`, algoritm) — telefon endpointi bilan bir xil kod.
	//
	// FARQI IKKITA QO'SHIMCHA SHART:
	//   * `sign_in_provider` AYNAN "google.com" — aks holda parol
	//     bilan ochilgan Firebase hisobi istalgan email da'vosi bilan
	//     o'tib ketardi;
	//   * `email_verified = true` — manzil EGALIGI isbotlangan bo'lsin.
	//
	// Email SO'ROVDAN OLINMAYDI — faqat token ichidan.
	mux.HandleFunc("POST /auth/google", func(w http.ResponseWriter, r *http.Request) {
		if !s.firebaseReady(w, "Google kirish") {
			return
		}
		if rateLimited(w, loginIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			IDToken string `json:"id_token"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		tok, err := s.Firebase.Verify(r.Context(), req.IDToken)
		if err == nil {
			err = tok.RequireGoogleEmail()
		}
		if err != nil {
			slog.Warn("google token rad etildi", "err", err, "ip", clientIP(r))
			httpError(w, http.StatusUnauthorized, errors.New("tasdiqlash amalga oshmadi"))
			return
		}
		token, u, err := s.AuthSvc.LoginWithGoogle(r.Context(), tok.Email, tok.Name)
		if err != nil {
			respondAuthError(w, err, http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
	})

	// POST /auth/email/request-code — emailga 6 xonali kod.
	//
	// SMS oqimi bilan bir xil himoya: bir martalik kod, hash bilan
	// saqlanadi, 5 daqiqa amal qiladi, 60 soniyalik qayta yuborish
	// pauzasi, tekshirishda 5 urinish chegarasi.
	//
	// ALOHIDA CHEKLOV: email SMS'dan arzon, lekin cheklovsiz yuborish
	// domenni spam ro'yxatiga tushiradi — bu tiklab bo'lmaydigan
	// zarar, shuning uchun IP va global cheklovlar baribir bor.
	mux.HandleFunc("POST /auth/email/request-code", func(w http.ResponseWriter, r *http.Request) {
		if !s.emailLoginReady(w) {
			return
		}
		var req struct {
			Email string `json:"email"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		// Format tekshiruvi cheklovdan OLDIN — noto'g'ri yozilgan
		// manzil foydalanuvchining kvotasini yoqmasligi kerak
		// (`/auth/register` dagi bilan bir xil sabab).
		if _, err := users.NormalizeEmail(req.Email); err != nil {
			httpError(w, http.StatusBadRequest, err)
			return
		}
		if rateLimitedBoth(w, emailIPLimiter, clientIP(r), emailGlobalLimiter, "global") {
			return
		}
		email, code, err := s.AuthSvc.RequestEmailCode(r.Context(), req.Email)
		if err != nil {
			respondAuthError(w, err, http.StatusBadRequest)
			return
		}
		resp := map[string]any{"sent": true, "email": email}
		if s.devCodeVisible(true) {
			resp["dev_code"] = code
		}
		writeJSON(w, http.StatusOK, resp)
	})

	// POST /auth/email/verify — email kodi -> token.
	mux.HandleFunc("POST /auth/email/verify", func(w http.ResponseWriter, r *http.Request) {
		// `/auth/verify` bilan bir xil sabab: kodning o'z urinishlar
		// chegarasi begona manzilga qarshi qurol sifatida ishlatilishi
		// mumkin (6 ta soxta urinish qurbonning haqiqiy kodini
		// o'chiradi). IP cheklovi buni bitta manbadan imkonsiz qiladi.
		if !s.emailLoginReady(w) {
			return
		}
		if rateLimited(w, verifyIPLimiter, clientIP(r)) {
			return
		}
		var req struct {
			Email string `json:"email"`
			Code  string `json:"code"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		token, u, err := s.AuthSvc.VerifyEmail(r.Context(), req.Email, req.Code)
		if err != nil {
			respondAuthError(w, err, http.StatusUnauthorized)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": token, "user": u})
	})

	// POST /auth/logout — HAQIQIY chiqish.
	//
	// Avval "chiqish" faqat mijoz tomonidagi amal edi: ilova tokenni
	// o'chirardi, xolos. Token o'g'irlangan bo'lsa (masalan qurilma
	// yo'qolgan, backup o'qilgan) u to'liq 30 kun ishlashda davom
	// etardi va foydalanuvchida uni to'xtatishning HECH QANDAY yo'li
	// yo'q edi. Endi bu endpoint shu foydalanuvchining barcha mavjud
	// tokenlarini serverda yaroqsiz qiladi.
	mux.HandleFunc("POST /auth/logout", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			c := claimsFrom(r)
			s.Revoked.Revoke(r.Context(), c.Subject)
			slog.Info("sessiya bekor qilindi (chiqish)", "user", c.Subject)
			writeJSON(w, http.StatusOK, map[string]bool{"logged_out": true})
		}))

	// GET /config/maps — Google Maps kaliti. Frontend kodida saqlanmaydi:
	// server .env dan o'qib, faqat tizimga kirgan foydalanuvchilarga beradi.
	// Qo'shimcha himoya Google Console'da: kalit domen (referrer) va API
	// turi bo'yicha cheklanadi.
}
