package telegram

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// pendingTTL — deep-link tokenining amal qilish muddati.
//
// Qisqa bo'lishi shart: token havolada ochiq boradi va u yagona narsa
// bo'lib, "qaysi ro'yxatdan o'tish urinishi" ni bildiradi. Uzoq
// yashasa, tasodifan ulashilgan havola keyinroq ishlatilishi mumkin.
const pendingTTL = 10 * time.Minute

// returnDeepLink — ilovani ochuvchi havola sxemasi.
//
// Mos keluvchi `intent-filter`:
// `apps/customer_app/android/app/src/main/AndroidManifest.xml`.
const returnDeepLink = "ondex://auth"

// returnPath — serverdagi yo'naltiruvchi endpoint.
//
// ┌─ NEGA TO'G'RIDAN-TO'G'RI `ondex://` EMAS ─────────────────────────┐
// Telegram Bot API'da inline tugmaning `url` maydoni FAQAT `http(s)://`
// va `tg://` havolalarini qabul qiladi. `ondex://` yuborilganda
// Telegram butun so'rovni `BUTTON_URL_INVALID` bilan rad etadi.
//
// Avval bu xato JIMGINA yutilardi (`SendReturnLink` tugmasiz xabarga
// qaytardi), shuning uchun "OnDex'ga qaytish" tugmasi HECH QACHON
// ko'rinmagan. Polling paytida bu sezilmasdi — ilova natijani o'zi
// olardi. Maxfiy kalit joriy qilingach esa oqim butunlay to'xtab
// qoldi: kalit faqat o'sha tugma orqali keladi.
//
// Endi tugma bizning serverimizga ishora qiladi, server esa 302 bilan
// `ondex://auth?c=...` ga yo'naltiradi. Kalit baribir FAQAT shu
// qurilmaning brauzeri orqali ilovaga tushadi, ya'ni fishingga qarshi
// himoya o'zgarmaydi (`Pending.ConfirmSecret` izohiga qarang).
// └───────────────────────────────────────────────────────────────────┘
const returnPath = "/auth/telegram/return?c="

// webReturnPath — brauzerdan boshlangan kirish uchun (`StartLoginWeb`,
// `Pending.Web`). Go serverning O'ZIGA emas — Next.js BFF'ga ishora
// qiladi (`WEB_PUBLIC_BASE_URL`), chunki sessiya cookie'si FAQAT shu
// qatlamda o'rnatiladi (`apps/web/lib/session.ts`). Go tomonda
// hech qanday cookie/HTML yo'q — u faqat JSON API.
const webReturnPath = "/api/auth/telegram-return?c="

// Pending — boshlangan, lekin hali tasdiqlanmagan urinish.
type Pending struct {
	Token string
	// Phone — ILOVADA kiritilgan raqam. Telegram yuborgan raqam SHU
	// bilan solishtiriladi (paket izohidagi hujumga qarshi).
	//
	// KIRISH rejimida BO'SH bo'ladi: foydalanuvchi hech narsa
	// yozmaydi, kimligini Telegram belgilaydi. Bunda solishtirish
	// o'rniga Telegram tasdiqlagan raqam O'ZI kimlik bo'lib xizmat
	// qiladi (`StartLogin` izohiga qarang).
	Phone     string
	ChatID    int64
	ExpiresAt time.Time

	// --- KIRISH rejimi ---

	// ConfirmSecret — natijani olish uchun MAJBURIY bo'lgan maxfiy
	// kalit. Foydalanuvchi uni HECH QACHON ko'rmaydi va yozmaydi.
	//
	// ┌─ NEGA BU MAJBURIY (fishing) ──────────────────────────────────┐
	// Busiz kirish rejimi to'liq akkaunt egallashga ochiq edi:
	//
	//	1. hujumchi ilovada "Telegram bilan kirish" ni bosadi va
	//	   deep link + kuzatish tokenini oladi;
	//	2. o'sha havolani QURBONGA yuboradi ("OnDex bonusini olish
	//	   uchun bosing" qabilida);
	//	3. qurbon Start bosadi va raqamini ulashadi — bot esa buni
	//	   HUJUMCHINING so'roviga tasdiq deb yozadi;
	//	4. hujumchi o'z tokeni bilan natijani so'raydi va QURBONNING
	//	   akkauntiga kirish tokenini oladi.
	//
	// Ildiz sabab: tasdiqlash uchun kuzatish tokenining O'ZI yetarli
	// edi, u esa hujumchining qo'lida. Bu — RFC 8628 (OAuth Device
	// Grant) dagi "remote phishing".
	//
	// YECHIM: tasdiqdan keyin bot yuboradigan "OnDex'ga qaytish"
	// tugmasi `ondex://auth?c=<ConfirmSecret>` havolasini ochadi.
	// Havola TASDIQLAGAN ODAMNING QURILMASIDA ochiladi, ya'ni kalit
	// aynan o'sha qurilmadagi ilovaga tushadi.
	//
	//	halol holat  — o'sha qurilmada so'rovni boshlagan ilova
	//	               turibdi: u kalitni oladi va kirishni yakunlaydi;
	//	fishing      — qurbon o'z telefonida tasdiqlaydi, kalit ham
	//	               O'SHA telefonga tushadi. Hujumchining qurilmasi
	//	               uni HECH QACHON ko'rmaydi va uning so'rovi
	//	               abadiy "pending" bo'lib qoladi.
	//
	// Foydalanuvchi uchun qadam QO'SHILMAYDI: u baribir "qaytish"
	// tugmasini bosadi, kalit esa ko'rinmas holda o'sha bosishda
	// keladi.
	// └───────────────────────────────────────────────────────────────┘
	ConfirmSecret string

	// Web — bu urinish MIJOZ ILOVASI emas, oddiy brauzer (`apps/web`)
	// tomonidan boshlangan (`StartLoginWeb`). "OnDex'ga qaytish"
	// tugmasi shunga qarab ikki xil manzilga ishora qiladi:
	// `ondex://auth?c=...` (ilova) yoki `<WEB_PUBLIC_BASE_URL>/api/
	// auth/telegram-return?c=...` (brauzer) — pastdagi kontakt
	// ishlov beruvchisiga qarang.
	Web bool

	// Done — Telegram tasdiqlab bo'ldi.
	Done bool
	// VerifiedPhone — Telegram tasdiqlagan raqam (kirish rejimida).
	VerifiedPhone string
}

// Redis kalitlari. Ikkita indeks kerak: token bo'yicha (ilova
// so'raydi) va chat bo'yicha (bot yangiligi keladi).
const (
	pendingTokenKey = "tgpending:"
	pendingChatKey  = "tgchat:"
)

// pendingStore — xotira keshi + (ixtiyoriy) Redis'da saqlash.
//
// ┌─ NEGA REDIS QO'SHILDI ─────────────────────────────────────────────┐
// Avval bu do'kon FAQAT xotirada edi. Izohda "server qayta ishga
// tushsa foydalanuvchi shunchaki qaytadan boshlaydi" deb yozilgandi —
// nazariy jihatdan to'g'ri, lekin amalda qimmatga tushardi:
//
// CI/CD har `main` ga push'da deploy qiladi va konteyner qayta
// yaratiladi. O'sha daqiqada "Telegram bilan kirish" oqimida turgan
// HAR BIR foydalanuvchining sessiyasi yo'qolardi — ular botda Start
// bosgach ilovaga qaytganda "havola eskirgan" xabarini olardi va
// sababini tushunmasdi.
//
// Endi yozuv Redis'da ham turadi (TTL bilan) va server ko'tarilganda
// `load()` uni qaytaradi. Naqsh `internal/revoke` dagi bilan bir xil:
// xotira — tez yo'l, Redis — chidamlilik. Redis YO'Q bo'lsa hammasi
// avvalgidek ishlaydi (faqat xotirada) — loyihaning umumiy qoidasi:
// Redis hech qachon ilovani to'xtatmaydi.
// └────────────────────────────────────────────────────────────────────┘
type pendingStore struct {
	mu       sync.Mutex
	byToken  map[string]*Pending
	byChatID map[int64]string // chat -> token

	rdb *redis.Client // nil bo'lishi mumkin
}

func newPendingStore() *pendingStore {
	return &pendingStore{
		byToken:  map[string]*Pending{},
		byChatID: map[int64]string{},
	}
}

// persistLocked — yozuvni Redis'ga ko'chiradi (chaqiruvchi qulf ushlaydi).
//
// Xato JIM log qilinadi va oqim davom etadi: Redis ishlamay qolgani
// uchun foydalanuvchini kirishdan to'xtatish noto'g'ri bo'lardi —
// xotiradagi nusxa baribir shu jarayon davomida ishlaydi.
func (s *pendingStore) persistLocked(p *Pending) {
	if s.rdb == nil {
		return
	}
	ttl := time.Until(p.ExpiresAt)
	if ttl <= 0 {
		return
	}
	raw, err := json.Marshal(p)
	if err != nil {
		slog.Warn("telegram: kutilayotgan sessiyani saqlab bo'lmadi", "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.rdb.Set(ctx, pendingTokenKey+p.Token, raw, ttl).Err(); err != nil {
		slog.Warn("telegram: Redis'ga yozib bo'lmadi", "err", err)
		return
	}
	if p.ChatID != 0 {
		// Chat -> token indeksi. Alohida kalit, chunki bot yangiligi
		// faqat chat ID bilan keladi.
		if err := s.rdb.Set(ctx, fmt.Sprintf("%s%d", pendingChatKey, p.ChatID), p.Token, ttl).Err(); err != nil {
			slog.Warn("telegram: chat indeksini yozib bo'lmadi", "err", err)
		}
	}
}

func (s *pendingStore) forgetLocked(p *Pending) {
	if s.rdb == nil || p == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	keys := []string{pendingTokenKey + p.Token}
	if p.ChatID != 0 {
		keys = append(keys, fmt.Sprintf("%s%d", pendingChatKey, p.ChatID))
	}
	if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
		slog.Warn("telegram: Redis'dan o'chirib bo'lmadi", "err", err)
	}
}

// load — server ishga tushganda Redis'dagi sessiyalarni qaytaradi.
func (s *pendingStore) load() {
	if s.rdb == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Avval TARMOQDAN o'qiladi, keyin qulf ostida qo'llanadi. Qulfni
	// Redis so'rovlari davomida ushlab turish butun Telegram oqimini
	// (bot polling + HTTP) shu vaqtga bloklardi.
	var found []*Pending
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, pendingTokenKey+"*", 200).Result()
		if err != nil {
			slog.Warn("telegram: kutilayotgan sessiyalarni o'qib bo'lmadi", "err", err)
			return
		}
		for _, k := range keys {
			raw, err := s.rdb.Get(ctx, k).Bytes()
			if err != nil {
				continue
			}
			var p Pending
			if err := json.Unmarshal(raw, &p); err != nil {
				continue
			}
			if p.Token == "" || time.Now().After(p.ExpiresAt) {
				continue
			}
			found = append(found, &p)
		}
		if next == 0 {
			break
		}
		cursor = next
	}

	s.mu.Lock()
	for _, p := range found {
		s.byToken[p.Token] = p
		if p.ChatID != 0 {
			s.byChatID[p.ChatID] = p.Token
		}
	}
	s.mu.Unlock()

	if len(found) > 0 {
		slog.Info("telegram: kutilayotgan kirish sessiyalari tiklandi", "count", len(found))
	}
}

func (s *pendingStore) put(p *Pending) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	s.byToken[p.Token] = p
	s.persistLocked(p)
}

// ┌─ NEGA KO'RSATKICH EMAS, NUSXA ─────────────────────────────────────┐
// Avval bu metodlar `*Pending` qaytarardi. Mutex esa faqat MAP'ni
// himoya qilardi — ko'rsatkich chaqiruvchiga o'tgach, u struct
// maydonlarini QULFSIZ o'qirdi:
//
//	HTTP goroutine        : LoginResult -> p.Done, p.VerifiedPhone (qulfsiz)
//	Bot polling goroutine : markLoggedIn -> p.Done, p.VerifiedPhone (qulf bilan)
//
// Bu haqiqiy ma'lumot poygasi edi (`go test -race` CI'da aniqladi).
// Oqibati faqat "detektor shikoyat qiladi" emas: Go xotira modeli
// sinxronizatsiyasiz KO'RINISHNI kafolatlamaydi, ya'ni HTTP tomoni
// `Done=true` ni ko'rib, `VerifiedPhone` ning ESKI (bo'sh) qiymatini
// o'qishi mumkin. Bunda `ConsumeLogin` dagi `p.VerifiedPhone == ""`
// sharti ishlab, kirish SABABSIZ rad etilardi — foydalanuvchi uchun
// bu "cheksiz yuklanish" bo'lib ko'rinadi.
//
// Endi nusxa QULF USHLAB TURGANDA olinadi. `Pending` faqat qiymat
// tiplaridan iborat (string, int64, time.Time), shuning uchun nusxa
// to'liq va mustaqil — chaqiruvchi uni xohlagancha o'qiy oladi.
//
// `ok=false` — yozuv yo'q yoki muddati o'tgan.
// └────────────────────────────────────────────────────────────────────┘
func (s *pendingStore) getByToken(token string) (Pending, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.byToken[token]
	if p == nil || time.Now().After(p.ExpiresAt) {
		return Pending{}, false
	}
	return *p, true
}

func (s *pendingStore) bindChat(token string, chatID int64) (Pending, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.byToken[token]
	if p == nil || time.Now().After(p.ExpiresAt) {
		return Pending{}, false
	}
	p.ChatID = chatID
	s.byChatID[chatID] = token
	s.persistLocked(p)
	return *p, true
}

func (s *pendingStore) getByChat(chatID int64) (Pending, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := s.byChatID[chatID]
	if token == "" {
		return Pending{}, false
	}
	p := s.byToken[token]
	if p == nil || time.Now().After(p.ExpiresAt) {
		return Pending{}, false
	}
	return *p, true
}

// markLoggedIn — kirish rejimida Telegram tasdig'ini yozib qo'yadi.
func (s *pendingStore) markLoggedIn(token, phone string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.byToken[token]; p != nil {
		p.Done = true
		p.VerifiedPhone = phone
		// MUHIM: aynan shu holat (tasdiqlandi, lekin ilova hali
		// natijani so'ramadi) restart paytida eng ko'p yo'qolardi —
		// foydalanuvchi botda hammasini qilib bo'lib, ilovaga
		// qaytganda "havola eskirgan" ko'rardi.
		s.persistLocked(p)
	}
}


func (s *pendingStore) drop(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.byToken[token]; p != nil {
		delete(s.byChatID, p.ChatID)
		s.forgetLocked(p)
	}
	delete(s.byToken, token)
}

// sweepLocked — eskirgan yozuvlarni tozalaydi (chaqiruvchi qulf ushlaydi).
//
// Redis'dagi nusxa TTL bilan o'zi yo'qoladi, shuning uchun bu yerda
// alohida o'chirish shart emas — xotira va Redis mustaqil ravishda,
// bir xil muddat bo'yicha tozalanadi.
func (s *pendingStore) sweepLocked() {
	now := time.Now()
	for token, p := range s.byToken {
		if now.After(p.ExpiresAt) {
			delete(s.byChatID, p.ChatID)
			delete(s.byToken, token)
		}
	}
}

// CodeIssuer — raqam TASDIQLANGANDAN keyin kod yaratib, uni odatdagi
// kod do'koniga yozadi va matnini qaytaradi.
//
// Shu tufayli Telegram yo'li ALOHIDA tasdiqlash mantiqini talab
// qilmaydi: kod mavjud `CodeStore` ga tushadi va foydalanuvchi uni
// odatdagi `POST /auth/verify` orqali kiritadi.
type CodeIssuer func(ctx context.Context, phone string) (code string, err error)

// PhoneNormalizer — raqamlarni bir ko'rinishga keltiradi. Telegram
// raqamni "+998..." yoki "998..." shaklida yuborishi mumkin, ilovadan
// esa boshqacha kelishi mumkin — solishtirishdan OLDIN ikkalasi ham
// normallashtiriladi, aks holda mos kelmay qolardi.
type PhoneNormalizer func(raw string) (string, error)

// botAPI — Verifier'ga kerak bo'lgan amallar.
//
// Interfeys ATAYLAB: `*Client` HTTP so'rov yuboradi, ya'ni tasdiqlash
// mantiqini u bilan birga sinab bo'lmasdi. Bu yerdagi eng muhim
// tekshiruv — raqam mosligi — testsiz qolmasligi kerak.
type botAPI interface {
	Configured() bool
	BotUsername(ctx context.Context) (string, error)
	SendMessage(ctx context.Context, chatID int64, text string) error
	AskContact(ctx context.Context, chatID int64, text string) error
	RemoveKeyboard(ctx context.Context, chatID int64, text string) error
	SendReturnLink(ctx context.Context, chatID int64, text, url, label string) error
	GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error)
}

// Verifier — botni tinglaydi va tasdiqlash oqimini boshqaradi.
type Verifier struct {
	client     botAPI
	store      *pendingStore
	issueCode  CodeIssuer
	normalize  PhoneNormalizer
	botName    string
	botNameMu  sync.RWMutex
	codeTTLMin int
	// publicURL — "OnDex'ga qaytish" tugmasi ishora qiladigan manzil
	// (`returnPath` izohiga qarang). Bo'sh bo'lsa tugma yuborilmaydi.
	publicURL string
	// webPublicURL — xuddi shu, lekin BRAUZERDAN kirish uchun
	// (`webReturnPath`, `Pending.Web`). Odatda `apps/web`ning o'zi
	// (masalan `https://ondex.uz`) — Go serverning manzili EMAS.
	webPublicURL string
	// onContact — kontakt ulashilganda telegram_id ↔ telefon
	// bog'lanishini saqlaydi (Mini App uchun). `WithContactHook`.
	onContact ContactHook
	// requireSecret — natijani olish uchun `ConfirmSecret` SHARTMI.
	//
	// ┌─ NEGA SOZLANADIGAN (va nega bu xavfsiz emas) ─────────────────┐
	// Kalit ilovaga FAQAT "OnDex'ga qaytish" tugmasi orqali yetadi,
	// tugma esa Telegramning cheklovi tufayli HAQIQIY OMMAVIY domen
	// talab qiladi: `localhost` va shunga o'xshash manzillarni
	// Telegram "Wrong HTTP URL" deb rad etadi (jonli tasdiqlangan).
	//
	// Ya'ni ishlab chiqish muhitida (domen yo'q) kalitni yetkazishning
	// hech qanday yo'li yo'q va uni talab qilish "Telegram bilan
	// kirish" ni BUTUNLAY ishdan chiqaradi.
	//
	// Shuning uchun:
	//   dev        — kalit talab qilinmaydi (kuzatish tokeni yetarli).
	//                Bu FISHINGGA OCHIQ: havolani qurbonga yuborgan
	//                odam uning akkauntiga kira oladi.
	//   production — kalit MAJBURIY (fail-closed). Domen bo'lgani
	//                uchun tugma ishlaydi va teshik yopiq bo'ladi.
	//
	// Bu ataylab shunday: ishlab chiqish qulayligi production
	// xavfsizligini pasaytirmasin.
	// └───────────────────────────────────────────────────────────────┘
	requireSecret bool
}

// WithPublicURL — serverning TASHQARIDAN ko'rinadigan manzili
// (`PUBLIC_BASE_URL`). Telefon brauzeri shu manzilga chiqa olishi
// SHART, aks holda "qaytish" tugmasi ochilmaydi.
func (v *Verifier) WithPublicURL(base string) *Verifier {
	v.publicURL = strings.TrimRight(strings.TrimSpace(base), "/")
	return v
}

// WithWebPublicURL — `apps/web`ning TASHQARIDAN ko'rinadigan manzili
// (`WEB_PUBLIC_BASE_URL`). `WithPublicURL` bilan bir xil rol, faqat
// brauzerdan boshlangan kirish uchun (`webReturnPath` izohiga qarang).
func (v *Verifier) WithWebPublicURL(base string) *Verifier {
	v.webPublicURL = strings.TrimRight(strings.TrimSpace(base), "/")
	return v
}

// WithConfirmSecretRequired — `requireSecret` izohiga qarang.
func (v *Verifier) WithConfirmSecretRequired(required bool) *Verifier {
	v.requireSecret = required
	return v
}

// ContactHook — foydalanuvchi botda KONTAKTINI ULASHGANDA chaqiriladi.
//
// `telegramID` — kontakt EGASI tekshirilgan (`m.Contact.UserID ==
// m.From.ID`), `phone` — normallashtirilgan.
type ContactHook func(ctx context.Context, telegramID int64, phone string) error

// WithContactHook — Telegram Mini App uchun bog'lanishni saqlash.
//
// ┌─ NEGA HOOK, TO'G'RIDAN-TO'G'RI CHAQIRUV EMAS ──────────────────────┐
// `internal/telegram` paketi `internal/users` ni IMPORT QILMAYDI va
// qilmasligi ham kerak — aks holda ikki paket bir-biriga bog'lanib,
// telegram testlari uchun butun foydalanuvchi qatlamini ko'tarish
// kerak bo'lardi. Bog'lanish `cmd/api` da ulanadi.
// └────────────────────────────────────────────────────────────────────┘
func (v *Verifier) WithContactHook(h ContactHook) *Verifier {
	v.onContact = h
	return v
}

func NewVerifier(c botAPI, issue CodeIssuer, normalize PhoneNormalizer, codeTTLMinutes int) *Verifier {
	return &Verifier{
		client:     c,
		store:      newPendingStore(),
		issueCode:  issue,
		normalize:  normalize,
		codeTTLMin: codeTTLMinutes,
	}
}

// WithRedis — kutilayotgan kirish sessiyalarini Redis'da ham saqlaydi
// va mavjudlarini darhol tiklaydi.
//
// Konstruktor imzosi ATAYLAB o'zgartirilmadi: `NewVerifier` to'rtta
// testda chaqiriladi va Redis ular uchun keraksiz. `WithPush`/
// `WithContactHook` bilan bir xil naqsh — ixtiyoriy imkoniyat alohida
// metod orqali ulanadi.
//
// `rdb` nil bo'lsa hech narsa o'zgarmaydi (faqat xotirada ishlaydi).
func (v *Verifier) WithRedis(rdb *redis.Client) *Verifier {
	if rdb == nil {
		return v
	}
	v.store.mu.Lock()
	v.store.rdb = rdb
	v.store.mu.Unlock()
	v.store.load()
	return v
}

func (v *Verifier) Configured() bool { return v.client.Configured() }

// BotUsername — bot nomi (keshlanadi).
//
// Mini App kirishida raqam hali bog'lanmagan bo'lsa, klientga botning
// havolasi qaytariladi — foydalanuvchi o'sha yerda kontaktini
// ulashadi. `username` ning ochiq o'rami.
func (v *Verifier) BotUsername(ctx context.Context) (string, error) {
	return v.username(ctx)
}

// Start — yangi urinish boshlaydi va deep link qaytaradi.
func (v *Verifier) Start(ctx context.Context, phone string) (deepLink string, err error) {
	name, err := v.username(ctx)
	if err != nil {
		return "", err
	}
	token, err := randomToken()
	if err != nil {
		return "", err
	}
	v.store.put(&Pending{
		Token:     token,
		Phone:     phone,
		ExpiresAt: time.Now().Add(pendingTTL),
	})
	return DeepLink(name, token), nil
}

// StartLogin — "Telegram bilan kirish": foydalanuvchi HECH NARSA
// yozmaydi, kimligini Telegram belgilaydi.
//
// FARQI `Start` dan: bu yerda solishtiriladigan raqam YO'Q. Telegram
// tasdiqlagan raqamning O'ZI kimlik bo'ladi — bu xuddi SMS kodni
// o'sha raqamga yuborib, u qaytganini ko'rish bilan teng kuchda,
// chunki raqamni foydalanuvchi qo'lda kirita olmaydi (`request_contact`
// tugmasini Telegram to'ldiradi).
//
// Natijani olish uchun `ConfirmSecret` ham kerak bo'ladi — u ilovaga
// TASDIQDAN KEYIN, "OnDex'ga qaytish" havolasi orqali keladi va SHU
// YERDA qaytarilmaydi (`Pending.ConfirmSecret` izohiga qarang).
func (v *Verifier) StartLogin(ctx context.Context) (deepLink, token string, err error) {
	name, err := v.username(ctx)
	if err != nil {
		return "", "", err
	}
	token, err = randomToken()
	if err != nil {
		return "", "", err
	}
	secret, err := randomToken()
	if err != nil {
		return "", "", err
	}
	v.store.put(&Pending{
		Token: token,
		// Phone bo'sh = kirish rejimi.
		ConfirmSecret: secret,
		ExpiresAt:     time.Now().Add(pendingTTL),
	})
	return DeepLink(name, token), token, nil
}

// StartLoginWeb — `StartLogin` bilan AYNAN bir xil, faqat
// `Pending.Web = true` bilan: "OnDex'ga qaytish" tugmasi ilova
// (`ondex://`) o'rniga `apps/web`ga (`webPublicURL`) ishora qiladi.
//
// Alohida metod sifatida (branch/parametr emas): `StartLogin`ning
// mavjud chaqiruvchilari (mijoz ilovasi) O'ZGARISHSIZ qoladi — yangi
// xatti-harakat faqat ANIQ shu yo'lni tanlagan chaqiruvchiga ta'sir
// qiladi.
func (v *Verifier) StartLoginWeb(ctx context.Context) (deepLink, token string, err error) {
	name, err := v.username(ctx)
	if err != nil {
		return "", "", err
	}
	token, err = randomToken()
	if err != nil {
		return "", "", err
	}
	secret, err := randomToken()
	if err != nil {
		return "", "", err
	}
	v.store.put(&Pending{
		Token:         token,
		ConfirmSecret: secret,
		Web:           true,
		ExpiresAt:     time.Now().Add(pendingTTL),
	})
	return DeepLink(name, token), token, nil
}

// LoginResult — kirish urinishi holati.
//
// `Found=false` — token noma'lum yoki muddati o'tgan.
// `Done=false`  — foydalanuvchi hali Telegramda tasdiqlamagan.
func (v *Verifier) LoginResult(token string) (found, done bool, phone string) {
	p, ok := v.store.getByToken(token)
	if !ok {
		return false, false, ""
	}
	return true, p.Done, p.VerifiedPhone
}

// ConsumeLogin — natijani BIR MARTA oladi va yozuvni o'chiradi.
//
// `secret` — "OnDex'ga qaytish" havolasidan kelgan kalit. U MAJBURIY:
// kuzatish tokenining o'zi yetarli bo'lsa, havolani qurbonga yuborgan
// hujumchi uning akkauntiga kirib olardi
// (`Pending.ConfirmSecret` izohidagi fishing).
//
// Bir martalik: token ilova va server o'rtasida ochiq yuradi, shuning
// uchun u bilan ikkinchi marta token olib bo'lmasligi kerak.
func (v *Verifier) ConsumeLogin(token, secret string) (phone string, ok bool) {
	p, ok := v.store.getByToken(token)
	if !ok || !p.Done || p.VerifiedPhone == "" {
		return "", false
	}
	if v.requireSecret {
		// Doimiy vaqtli solishtirish — kalit uzunligi/prefiksi javob
		// vaqti orqali sizib chiqmasin.
		if subtle.ConstantTimeCompare([]byte(secret), []byte(p.ConfirmSecret)) != 1 {
			// Yozuv O'CHIRILMAYDI: halol ilova kalit bilan biroz keyin
			// kelishi mumkin. Urinishlar `telegramPollLimiter` bilan
			// cheklangan, kalit esa 16 bayt tasodifiy.
			return "", false
		}
	}
	v.store.drop(token)
	return p.VerifiedPhone, true
}

func (v *Verifier) username(ctx context.Context) (string, error) {
	v.botNameMu.RLock()
	name := v.botName
	v.botNameMu.RUnlock()
	if name != "" {
		return name, nil
	}
	name, err := v.client.BotUsername(ctx)
	if err != nil {
		return "", err
	}
	v.botNameMu.Lock()
	v.botName = name
	v.botNameMu.Unlock()
	return name, nil
}

// Run — long-polling sikli. `ctx` bekor qilinganda to'xtaydi.
func (v *Verifier) Run(ctx context.Context) {
	var offset int64
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := v.client.GetUpdates(ctx, offset, 50)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			wait := 5 * time.Second
			if errors.Is(err, ErrConflict) {
				// Bu tarmoq nosozligi EMAS — sozlama xatosi. Qayta
				// urinish uni hech qachon tuzatmaydi, shuning uchun
				// xabar ERROR darajasida va yechim bilan beriladi.
				slog.Error("telegram: BOSHQA SERVER ham shu botni polling qilyapti — "+
					"foydalanuvchilarning yarmi kirishni yakunlay olmaydi ('havola eskirgan'). "+
					"YECHIM: dev uchun @BotFather'da ALOHIDA bot yarating va lokal .env dagi "+
					"TELEGRAM_BOT_TOKEN ni o'shanikiga almashtiring (production tokeni faqat serverda qolsin)",
					"err", err)
				// Konflikt holatida tez-tez urinish log'ni to'ldiradi
				// va ikkinchi serverni ham uzib turadi.
				wait = 30 * time.Second
			} else {
				// Tarmoq uzilishi yoki Telegram tomondagi vaqtinchalik
				// muammo butun botni o'ldirmasligi kerak.
				slog.Warn("telegram: getUpdates xatosi", "err", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			continue
		}
		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			v.handle(ctx, u)
		}
	}
}

func (v *Verifier) handle(ctx context.Context, u Update) {
	m := u.Message
	if m == nil {
		return
	}
	chatID := m.Chat.ID

	// 1-qadam: deep link orqali kelgan `/start <token>`.
	if strings.HasPrefix(m.Text, "/start") {
		token := strings.TrimSpace(strings.TrimPrefix(m.Text, "/start"))
		if token == "" {
			// ┌─ TOKENSIZ `/start` — MINI APP KIRISH NUQTASI ─────────┐
			// Avval bu yerda faqat "ilovadagi tugmadan foydalaning"
			// deb yozilardi va oqim tugardi.
			//
			// Endi bu Mini App uchun ASOSIY yo'l: foydalanuvchi botni
			// ochadi, raqamini ulashadi va shundan keyin mini ilova
			// uni tanib oladi. Raqamsiz Mini App kimligini
			// aniqlay OLMAYDI — `initData` da telefon yo'q.
			// └───────────────────────────────────────────────────────┘
			_ = v.client.AskContact(ctx, chatID,
				"Salom! OnDex mini ilovasidan foydalanish uchun "+
					"raqamingizni tasdiqlang.\n\n"+
					"Pastdagi tugma Telegram tomonidan tasdiqlangan raqamni "+
					"yuboradi — shu sabab uni qo'lda yozib bo'lmaydi.")
			return
		}
		if _, ok := v.store.bindChat(token, chatID); !ok {
			_ = v.client.SendMessage(ctx, chatID,
				"Havola eskirgan. Ilovada qaytadan urinib ko'ring.")
			return
		}
		_ = v.client.AskContact(ctx, chatID,
			"Raqamingizni tasdiqlash uchun pastdagi tugmani bosing.\n\n"+
				"Tugma Telegram tomonidan tasdiqlangan raqamni yuboradi — "+
				"shu sabab uni qo'lda yozish mumkin emas.")
		return
	}

	// 2-qadam: ulashilgan kontakt.
	if m.Contact != nil {
		// ┌─ TARTIB O'ZGARTIRILDI ────────────────────────────────────┐
		// Avval BIRINCHI navbatda faol kirish so'rovi qidirilardi va
		// topilmasa xabar berib chiqib ketilardi. Endi egalik
		// tekshiruvi va raqamni o'qish OLDINGA olindi, chunki kontakt
		// FAOL SO'ROVSIZ ham keladi — Telegram Mini App uchun
		// foydalanuvchi shunchaki raqamini bog'lamoqchi bo'lganda.
		// └───────────────────────────────────────────────────────────┘

		// BOSHQA ODAMNING kontaktini ulashish mumkin — Telegram bunga
		// ruxsat beradi. Shuning uchun kontakt EGASI ham tekshiriladi.
		if m.From == nil || m.Contact.UserID != m.From.ID {
			_ = v.client.SendMessage(ctx, chatID,
				"Faqat O'ZINGIZNING raqamingizni ulashishingiz mumkin.")
			return
		}
		shared, err := v.normalize(m.Contact.PhoneNumber)
		if err != nil {
			_ = v.client.SendMessage(ctx, chatID, "Raqam formati tanilmadi.")
			return
		}

		// ┌─ MINI APP UCHUN BOG'LANISH ───────────────────────────────┐
		// telegram_id ↔ telefon bog'lanishi HAR DOIM saqlanadi —
		// kirish oqimi bo'lsa ham, bo'lmasa ham.
		//
		// NEGA: Mini App `initData` da telefon raqami YO'Q, faqat
		// Telegram ID bor. Bog'lanish shu yerda yozilmasa, Mini App
		// foydalanuvchi kimligini HECH QACHON bila olmasdi.
		//
		// Xato yutiladi (faqat log): baza vaqtincha ishlamasa ham
		// kirish oqimi to'xtamasligi kerak — u mustaqil ishlaydi.
		// └───────────────────────────────────────────────────────────┘
		if v.onContact != nil {
			if err := v.onContact(ctx, m.From.ID, shared); err != nil {
				slog.Error("telegram: raqamni bog'lab bo'lmadi",
					"telegram_id", m.From.ID, "err", err)
			}
		}

		p, ok := v.store.getByChat(chatID)
		if !ok {
			// Faol kirish so'rovi yo'q — demak foydalanuvchi raqamini
			// Mini App uchun bog'lash maqsadida yubordi.
			_ = v.client.RemoveKeyboard(ctx, chatID,
				"Raqamingiz saqlandi ✅\n\nEndi OnDex mini ilovasini "+
					"ochsangiz avtomatik kirasiz.")
			return
		}

		// KIRISH rejimi (`StartLogin`): solishtiriladigan raqam yo'q —
		// Telegram tasdiqlagan raqamning o'zi kimlik bo'ladi.
		//
		// Foydalanuvchi uchun bu OXIRGI qadam: kod so'ralmaydi, u
		// shunchaki "OnDex'ga qaytish" ni bosadi.
		//
		// Fishingga qarshi himoya SHU tugmaning ichida: havolada
		// maxfiy kalit bor va u tasdiqlagan odamning QURILMASIDAGI
		// ilovaga tushadi, hujumchinikiga emas
		// (`Pending.ConfirmSecret` izohiga qarang).
		if p.Phone == "" {
			v.store.markLoggedIn(p.Token, shared)

			// Web va ilova ikki xil manzilga qaytadi (`Pending.Web`
			// izohiga qarang) — qaysi biri ISHLATILISHI shu yerda
			// hal qilinadi, qolgani ikkalasi uchun bir xil.
			base, path, target := v.publicURL, returnPath, "ilova"
			if p.Web {
				base, path, target = v.webPublicURL, webReturnPath, "veb"
			}

			// Avval "raqamni ulashish" klaviaturasini olib tashlaymiz,
			// keyin qaytish tugmasini yuboramiz — ikkovi bitta xabarda
			// birga bo'la olmaydi (Telegram cheklovi).
			// Kalit TALAB QILINMASA (dev, domen yo'q) tugma ham
			// yuborilmaydi: Telegram `localhost` ni baribir rad etadi
			// va foydalanuvchi hech narsa ko'rmasdi. Ilova natijani
			// o'zi so'rab oladi, ya'ni foydalanuvchi shunchaki
			// ilovaga qaytsa yetarli.
			if !v.requireSecret || base == "" {
				_ = v.client.RemoveKeyboard(ctx, chatID,
					"Tasdiqlandi ✅\n\nOnDex ilovasiga qayting — "+
						"kirish avtomatik yakunlanadi.")
				return
			}
			_ = v.client.RemoveKeyboard(ctx, chatID, "Tasdiqlandi ✅")
			link := base + path + url.QueryEscape(p.ConfirmSecret)
			if err := v.client.SendReturnLink(ctx, chatID,
				"OnDex'ga qayting — kirish avtomatik yakunlanadi.",
				link, "OnDex'ga qaytish"); err != nil {
				// Bu xato AVVAL jimgina yutilardi va tugma ko'rinmasdan
				// qolardi (`returnPath` izohiga qarang). Kalit talab
				// qilinadigan rejimda bu KIRISHNI TO'XTATADI, shuning
				// uchun foydalanuvchiga ham aytiladi.
				slog.Error("telegram: qaytish tugmasi yuborilmadi",
					"err", err, "url", link, "manba", target,
					"maslahat", "PUBLIC_BASE_URL/WEB_PUBLIC_BASE_URL ommaviy domen bo'lishi kerak")
				_ = v.client.SendMessage(ctx, chatID,
					"Kirishni yakunlab bo'lmadi (server sozlamasi). "+
						"Qaytadan urinib ko'ring.")
			}
			return
		}

		if shared != p.Phone {
			// Ilovada boshqa raqam kiritilgan — kod YUBORILMAYDI.
			// Aynan shu tekshiruv butun oqimni SMS bilan teng kuchga
			// keltiradi (paket izohiga qarang).
			_ = v.client.RemoveKeyboard(ctx, chatID,
				"Bu raqam ilovada kiritilgan raqamga mos kelmadi.\n"+
					"Ilovada o'z raqamingizni kiriting va qaytadan urinib ko'ring.")
			v.store.drop(p.Token)
			return
		}

		code, err := v.issueCode(ctx, p.Phone)
		if err != nil {
			slog.Warn("telegram: kod yaratib bo'lmadi", "err", err)
			_ = v.client.RemoveKeyboard(ctx, chatID,
				"Hozir kod yuborib bo'lmadi. Birozdan keyin urinib ko'ring.")
			return
		}
		// Kod YUBORILDI — token bir martalik, darhol o'chiriladi.
		v.store.drop(p.Token)
		_ = v.client.RemoveKeyboard(ctx, chatID, fmt.Sprintf(
			"Tasdiqlash kodi: %s\n\nKod %d daqiqa amal qiladi.\n"+
				"Kodni hech kim bilan ulashmang.", code, v.codeTTLMin))
		return
	}

}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
