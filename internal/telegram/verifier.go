package telegram

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"
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

	// Done — Telegram tasdiqlab bo'ldi.
	Done bool
	// VerifiedPhone — Telegram tasdiqlagan raqam (kirish rejimida).
	VerifiedPhone string
}

// pendingStore — xotiradagi do'kon.
//
// NEGA XOTIRADA: yozuv 10 daqiqa yashaydi va server qayta ishga
// tushsa foydalanuvchi shunchaki qaytadan boshlaydi. Bir nechta
// server nusxasi paydo bo'lganda buni Redis'ga ko'chirish kerak —
// xuddi `internal/ratelimit` dagi kabi (u ham hozir bitta nusxa
// uchun mo'ljallangan).
type pendingStore struct {
	mu       sync.Mutex
	byToken  map[string]*Pending
	byChatID map[int64]string // chat -> token
}

func newPendingStore() *pendingStore {
	return &pendingStore{
		byToken:  map[string]*Pending{},
		byChatID: map[int64]string{},
	}
}

func (s *pendingStore) put(p *Pending) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweepLocked()
	s.byToken[p.Token] = p
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
	}
}


func (s *pendingStore) drop(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p := s.byToken[token]; p != nil {
		delete(s.byChatID, p.ChatID)
	}
	delete(s.byToken, token)
}

// sweepLocked — eskirgan yozuvlarni tozalaydi (chaqiruvchi qulf ushlaydi).
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

// WithConfirmSecretRequired — `requireSecret` izohiga qarang.
func (v *Verifier) WithConfirmSecretRequired(required bool) *Verifier {
	v.requireSecret = required
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

func (v *Verifier) Configured() bool { return v.client.Configured() }

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
			// Tarmoq uzilishi yoki Telegram tomondagi vaqtinchalik
			// muammo butun botni o'ldirmasligi kerak.
			slog.Warn("telegram: getUpdates xatosi", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
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
			_ = v.client.SendMessage(ctx, chatID,
				"Salom! Tasdiqlashni boshlash uchun OnDex ilovasidagi "+
					"\"Telegram orqali kod olish\" tugmasidan foydalaning.")
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
		p, ok := v.store.getByChat(chatID)
		if !ok {
			_ = v.client.SendMessage(ctx, chatID,
				"Faol so'rov topilmadi. Ilovada qaytadan boshlang.")
			return
		}
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
			// Avval "raqamni ulashish" klaviaturasini olib tashlaymiz,
			// keyin qaytish tugmasini yuboramiz — ikkovi bitta xabarda
			// birga bo'la olmaydi (Telegram cheklovi).
			// Kalit TALAB QILINMASA (dev, domen yo'q) tugma ham
			// yuborilmaydi: Telegram `localhost` ni baribir rad etadi
			// va foydalanuvchi hech narsa ko'rmasdi. Ilova natijani
			// o'zi so'rab oladi, ya'ni foydalanuvchi shunchaki
			// ilovaga qaytsa yetarli.
			if !v.requireSecret || v.publicURL == "" {
				_ = v.client.RemoveKeyboard(ctx, chatID,
					"Tasdiqlandi ✅\n\nOnDex ilovasiga qayting — "+
						"kirish avtomatik yakunlanadi.")
				return
			}
			// Avval "raqamni ulashish" klaviaturasini olib tashlaymiz,
			// keyin qaytish tugmasini yuboramiz — ikkovi bitta xabarda
			// birga bo'la olmaydi (Telegram cheklovi).
			_ = v.client.RemoveKeyboard(ctx, chatID, "Tasdiqlandi ✅")
			link := v.publicURL + returnPath + url.QueryEscape(p.ConfirmSecret)
			if err := v.client.SendReturnLink(ctx, chatID,
				"OnDex ilovasiga qayting — kirish avtomatik yakunlanadi.",
				link, "OnDex'ga qaytish"); err != nil {
				// Bu xato AVVAL jimgina yutilardi va tugma ko'rinmasdan
				// qolardi (`returnPath` izohiga qarang). Kalit talab
				// qilinadigan rejimda bu KIRISHNI TO'XTATADI, shuning
				// uchun foydalanuvchiga ham aytiladi.
				slog.Error("telegram: qaytish tugmasi yuborilmadi",
					"err", err, "url", link,
					"maslahat", "PUBLIC_BASE_URL ommaviy domen bo'lishi kerak")
				_ = v.client.SendMessage(ctx, chatID,
					"Kirishni yakunlab bo'lmadi (server sozlamasi). "+
						"Ilovada qaytadan urinib ko'ring.")
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
