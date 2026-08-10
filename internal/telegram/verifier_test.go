package telegram

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

// BU FAYLDAGI ENG MUHIM TEST — `TestCodeNotIssuedWhenPhoneMismatch`.
//
// Deep link o'zi telefon raqamini TASDIQLAMAYDI: u faqat "kimdir
// botni ochdi" degan ma'noni beradi. Agar raqam mosligi tekshirilmasa,
// hujum juda oson bo'lardi:
//
//	hujumchi ilovada QURBONNING raqamini yozadi ->
//	botni O'ZINING Telegramida ochadi -> kodni oladi ->
//	qurbonning raqami bilan ro'yxatdan o'tadi.
//
// Shu sabab bu tekshiruv alohida va aniq sinaladi.

type fakeBot struct {
	mu          sync.Mutex
	sent        []string // chatga yuborilgan matnlar
	contacts    int      // "raqamni ulashing" so'rovlari soni
	returnLinks int      // "ilovaga qaytish" tugmalari soni
}

func (f *fakeBot) Configured() bool { return true }
func (f *fakeBot) BotUsername(context.Context) (string, error) {
	return "OndexTestBot", nil
}
func (f *fakeBot) SendMessage(_ context.Context, _ int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	return nil
}
func (f *fakeBot) AskContact(_ context.Context, _ int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.contacts++
	f.sent = append(f.sent, text)
	return nil
}
func (f *fakeBot) RemoveKeyboard(_ context.Context, _ int64, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	return nil
}
func (f *fakeBot) SendReturnLink(_ context.Context, _ int64, text, url, label string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.returnLinks++
	f.sent = append(f.sent, text+"|"+url+"|"+label)
	return nil
}
func (f *fakeBot) GetUpdates(context.Context, int64, int) ([]Update, error) {
	return nil, nil
}
func (f *fakeBot) lastContains(sub string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sent {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// normalize — testlar uchun sodda variant (`users.NormalizePhone` ga
// o'xshash): bo'sh joy/qavslarni olib tashlaydi va `+998...` qiladi.
func testNormalize(raw string) (string, error) {
	p := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(raw))
	if strings.HasPrefix(p, "998") && len(p) == 12 {
		p = "+" + p
	}
	if !strings.HasPrefix(p, "+998") || len(p) != 13 {
		return "", errors.New("noto'g'ri raqam")
	}
	return p, nil
}

func newTestVerifier(t *testing.T) (*Verifier, *fakeBot, *int) {
	t.Helper()
	bot := &fakeBot{}
	issued := 0
	v := NewVerifier(bot, func(context.Context, string) (string, error) {
		issued++
		return "123456", nil
	}, testNormalize, 5).
		WithPublicURL(testPublicURL).
		WithConfirmSecretRequired(true) // production rejimi
	return v, bot, &issued
}

// startFlow — deep link olib, `/start <token>` ni qayta ijro etadi.
func startFlow(t *testing.T, v *Verifier, phone string, chatID int64) string {
	t.Helper()
	link, err := v.Start(context.Background(), phone)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	i := strings.Index(link, "start=")
	if i < 0 {
		t.Fatalf("deep linkda token yo'q: %s", link)
	}
	token := link[i+len("start="):]
	v.handle(context.Background(), startUpdate(chatID, token))
	return token
}

func startUpdate(chatID int64, token string) Update {
	var u Update
	u.Message = &struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text    string `json:"text"`
		Contact *struct {
			PhoneNumber string `json:"phone_number"`
			UserID      int64  `json:"user_id"`
		} `json:"contact"`
		From *struct {
			ID int64 `json:"id"`
		} `json:"from"`
	}{}
	u.Message.Chat.ID = chatID
	u.Message.Text = "/start " + token
	return u
}

func contactUpdate(chatID int64, phone string, contactUserID, fromID int64) Update {
	u := startUpdate(chatID, "")
	u.Message.Text = ""
	u.Message.Contact = &struct {
		PhoneNumber string `json:"phone_number"`
		UserID      int64  `json:"user_id"`
	}{PhoneNumber: phone, UserID: contactUserID}
	u.Message.From = &struct {
		ID int64 `json:"id"`
	}{ID: fromID}
	return u
}

func TestCodeIssuedWhenPhoneMatches(t *testing.T) {
	v, bot, issued := newTestVerifier(t)
	startFlow(t, v, "+998901234567", 100)
	if bot.contacts != 1 {
		t.Fatal("raqamni ulashish so'ralmadi")
	}
	v.handle(context.Background(), contactUpdate(100, "998901234567", 55, 55))

	if *issued != 1 {
		t.Fatalf("kod yaratilmadi (issued=%d)", *issued)
	}
	if !bot.lastContains("123456") {
		t.Fatal("kod chatga yuborilmadi")
	}
}

// ★ ASOSIY XAVFSIZLIK TESTI
func TestCodeNotIssuedWhenPhoneMismatch(t *testing.T) {
	v, bot, issued := newTestVerifier(t)
	// Ilovada QURBONNING raqami kiritilgan.
	startFlow(t, v, "+998901234567", 100)
	// Botni HUJUMCHI o'z Telegramida ochdi va o'z raqamini ulashdi.
	v.handle(context.Background(), contactUpdate(100, "998907654321", 55, 55))

	if *issued != 0 {
		t.Fatal("MOS KELMAGAN raqamga kod yaratildi — akkaunt egallash mumkin")
	}
	if bot.lastContains("123456") {
		t.Fatal("kod chatga yuborildi")
	}
	if !bot.lastContains("mos kelmadi") {
		t.Fatal("foydalanuvchiga sabab tushuntirilmadi")
	}
}

// Telegram BOSHQA odamning kontaktini ulashishga ruxsat beradi —
// shuning uchun kontakt EGASI ham tekshiriladi.
func TestForeignContactRejected(t *testing.T) {
	v, _, issued := newTestVerifier(t)
	startFlow(t, v, "+998901234567", 100)
	// Kontakt boshqa odamniki: contact.user_id != from.id
	v.handle(context.Background(), contactUpdate(100, "998901234567", 999, 55))

	if *issued != 0 {
		t.Fatal("begona kontakt qabul qilindi")
	}
}

// Token BIR MARTALIK: kod yuborilgach yozuv o'chadi va o'sha havola
// qayta ishlatilmaydi.
func TestTokenIsSingleUse(t *testing.T) {
	v, _, issued := newTestVerifier(t)
	token := startFlow(t, v, "+998901234567", 100)
	v.handle(context.Background(), contactUpdate(100, "998901234567", 55, 55))
	if *issued != 1 {
		t.Fatalf("birinchi urinishda kod yaratilmadi")
	}
	// O'sha token bilan qayta urinish.
	v.handle(context.Background(), startUpdate(101, token))
	v.handle(context.Background(), contactUpdate(101, "998901234567", 55, 55))
	if *issued != 1 {
		t.Fatalf("token qayta ishlatildi (issued=%d)", *issued)
	}
}

func TestUnknownTokenRejected(t *testing.T) {
	v, bot, issued := newTestVerifier(t)
	v.handle(context.Background(), startUpdate(100, "yolgon-token"))
	if bot.contacts != 0 {
		t.Fatal("noma'lum token uchun raqam so'raldi")
	}
	v.handle(context.Background(), contactUpdate(100, "998901234567", 55, 55))
	if *issued != 0 {
		t.Fatal("noma'lum token bilan kod yaratildi")
	}
}

// ---------- "Telegram bilan kirish" rejimi ----------

// secretFromReturnLink — bot yuborgan "OnDex'ga qaytish" havolasidan
// maxfiy kalitni ajratib oladi (ilova uni aynan shu yo'l bilan oladi).
func secretFromReturnLink(t *testing.T, bot *fakeBot) string {
	t.Helper()
	bot.mu.Lock()
	defer bot.mu.Unlock()
	for _, s := range bot.sent {
		if i := strings.Index(s, returnPath); i >= 0 {
			rest := s[i+len(returnPath):]
			if j := strings.Index(rest, "|"); j >= 0 {
				rest = rest[:j]
			}
			return rest
		}
	}
	return ""
}

const testPublicURL = "http://localhost:8080"

func startLoginFlow(t *testing.T, v *Verifier, chatID int64) string {
	t.Helper()
	link, token, err := v.StartLogin(context.Background())
	if err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	if !strings.Contains(link, token) {
		t.Fatalf("deep linkda token yo'q: %s", link)
	}
	v.handle(context.Background(), startUpdate(chatID, token))
	return token
}

func TestLoginModeAcceptsTelegramPhone(t *testing.T) {
	v, bot, issued := newTestVerifier(t)
	token := startLoginFlow(t, v, 200)
	if bot.contacts != 1 {
		t.Fatal("raqamni ulashish so'ralmadi")
	}

	// Tasdiqdan OLDIN natija bo'lmasligi kerak.
	if _, done, _ := v.LoginResult(token); done {
		t.Fatal("tasdiqlashdan oldin ham tayyor deb ko'rsatildi")
	}

	// Raqamni ulashish — foydalanuvchi uchun OXIRGI qadam. Boshqa
	// hech narsa yozmaydi.
	v.handle(context.Background(), contactUpdate(200, "998901234567", 55, 55))

	found, done, phone := v.LoginResult(token)
	if !found || !done {
		t.Fatalf("tasdiq yozilmadi (found=%v done=%v)", found, done)
	}
	if phone != "+998901234567" {
		t.Fatalf("raqam noto'g'ri: %q", phone)
	}
	// Kirish rejimida KOD yuborilmaydi — ilova natijani o'zi oladi.
	if *issued != 0 {
		t.Fatal("kirish rejimida kod yaratildi (kerak emas)")
	}
	// Foydalanuvchi ilovalar orasida qidirmasligi uchun "qaytish"
	// tugmasi yuborilishi kerak (Android fondagi ilovani o'zi
	// oldinga chiqara olmaydi).
	if bot.returnLinks != 1 {
		t.Fatalf("qaytish tugmasi yuborilmadi (%d)", bot.returnLinks)
	}
	// Havolada MAXFIY KALIT bo'lishi shart — ilova natijani faqat
	// shu kalit bilan ola oladi.
	if secretFromReturnLink(t, bot) == "" {
		t.Fatal("qaytish havolasida maxfiy kalit yo'q")
	}
	// Havola HTTP bo'lishi SHART: Telegram inline tugmada `ondex://`
	// ni rad etadi va butun xabar yuborilmay qoladi
	// (`returnPath` izohiga qarang).
	if !bot.lastContains(testPublicURL + returnPath) {
		t.Fatal("qaytish havolasi HTTP emas — Telegram uni rad etadi")
	}
	if bot.lastContains("ondex://") {
		t.Fatal("botga custom sxemali havola yuborilyapti — Telegram rad etadi")
	}
}

// PRODUCTION rejimida manzil bo'lmasa oqim JIMGINA to'xtamasligi
// kerak: kalit faqat qaytish tugmasi orqali keladi.
func TestMissingPublicURLIsLoud(t *testing.T) {
	bot := &fakeBot{}
	v := NewVerifier(bot, func(context.Context, string) (string, error) {
		return "123456", nil
	}, testNormalize, 5).WithConfirmSecretRequired(true) // manzil YO'Q
	token := startLoginFlow(t, v, 200)
	v.handle(context.Background(), contactUpdate(200, "998901234567", 55, 55))

	if bot.returnLinks != 0 {
		t.Fatal("manzilsiz tugma yuborildi")
	}
	if _, ok := v.ConsumeLogin(token, "nimadir"); ok {
		t.Fatal("kalitsiz kirish ochildi")
	}
}

// ---------- DEV rejimi (kalit talab qilinmaydi) ----------

// devVerifier — `WithConfirmSecretRequired(false)`: domen yo'q,
// shuning uchun kalitni yetkazishning yo'li ham yo'q
// (`Verifier.requireSecret` izohiga qarang).
func devVerifier(t *testing.T) (*Verifier, *fakeBot) {
	t.Helper()
	bot := &fakeBot{}
	v := NewVerifier(bot, func(context.Context, string) (string, error) {
		return "123456", nil
	}, testNormalize, 5).WithConfirmSecretRequired(false)
	return v, bot
}

// Dev'da foydalanuvchi ilovaga QO'LDA qaytsa ham kirishi kerak: bot
// hech qanday tugma yubormaydi, ilova natijani o'zi so'rab oladi.
func TestDevModeCompletesWithoutSecret(t *testing.T) {
	v, bot := devVerifier(t)
	token := startLoginFlow(t, v, 200)
	v.handle(context.Background(), contactUpdate(200, "998901234567", 55, 55))

	if bot.returnLinks != 0 {
		t.Fatal("dev'da tugma yuborildi — Telegram uni baribir rad etadi")
	}
	if !bot.lastContains("ilovasiga qayting") {
		t.Fatal("foydalanuvchiga nima qilishi aytilmadi")
	}
	phone, ok := v.ConsumeLogin(token, "")
	if !ok || phone != "+998901234567" {
		t.Fatalf("kalitsiz kirish yakunlanmadi: %q ok=%v", phone, ok)
	}
}

// Dev'da ham natija BIR MARTALIK bo'lib qolishi kerak.
func TestDevModeResultIsSingleUse(t *testing.T) {
	v, _ := devVerifier(t)
	token := startLoginFlow(t, v, 200)
	v.handle(context.Background(), contactUpdate(200, "998901234567", 55, 55))

	if _, ok := v.ConsumeLogin(token, ""); !ok {
		t.Fatal("birinchi olish ishlamadi")
	}
	if _, ok := v.ConsumeLogin(token, ""); ok {
		t.Fatal("token QAYTA ishlatildi")
	}
}

// Natija BIR MARTALIK: token ochiq yuradi, shuning uchun u bilan
// ikkinchi marta kirish tokenini olib bo'lmasligi kerak.
func TestLoginResultIsSingleUse(t *testing.T) {
	v, bot, _ := newTestVerifier(t)
	token := startLoginFlow(t, v, 200)
	v.handle(context.Background(), contactUpdate(200, "998901234567", 55, 55))
	secret := secretFromReturnLink(t, bot)

	phone, ok := v.ConsumeLogin(token, secret)
	if !ok || phone != "+998901234567" {
		t.Fatalf("birinchi olishda natija noto'g'ri: %q ok=%v", phone, ok)
	}
	if _, ok := v.ConsumeLogin(token, secret); ok {
		t.Fatal("token QAYTA ishlatildi")
	}
	if found, _, _ := v.LoginResult(token); found {
		t.Fatal("ishlatilgan token hali ham topilyapti")
	}
}

// Kirish rejimida ham kontakt EGASI tekshirilishi kerak — aks holda
// begona odamning kontaktini ulashib, uning akkauntiga kirish mumkin
// bo'lardi.
func TestLoginModeRejectsForeignContact(t *testing.T) {
	v, _, _ := newTestVerifier(t)
	token := startLoginFlow(t, v, 200)
	v.handle(context.Background(), contactUpdate(200, "998901234567", 999, 55))

	if _, done, _ := v.LoginResult(token); done {
		t.Fatal("begona kontakt bilan kirish tasdiqlandi")
	}
}

// ★ FISHINGGA QARSHI ASOSIY TEST
//
// Hujum (`Pending.ConfirmSecret` izohida to'liq yozilgan):
//
//	hujumchi ilovada oqimni boshlaydi -> deep linkni QURBONGA
//	yuboradi -> qurbon Start bosib raqamini ulashadi -> hujumchi o'z
//	KUZATISH TOKENI bilan natijani so'raydi.
//
// Hujumchida bor narsa — kuzatish tokeni. Yo'q narsa — maxfiy kalit:
// u "OnDex'ga qaytish" havolasida va u QURBONNING qurilmasida
// ochiladi. Shu sabab uning so'rovi hech qachon yakunlanmaydi.
func TestLoginNotCompletedWithoutConfirmSecret(t *testing.T) {
	v, bot, _ := newTestVerifier(t)
	token := startLoginFlow(t, v, 200)
	v.handle(context.Background(), contactUpdate(200, "998901234567", 55, 55))

	// Hujumchi kalitsiz so'raydi.
	if _, ok := v.ConsumeLogin(token, ""); ok {
		t.Fatal("FISHING: kalitsiz kirish tokeni berildi")
	}
	// Taxmin qilib ko'radi.
	for _, guess := range []string{"0", "deadbeef", strings.Repeat("a", 32)} {
		if _, ok := v.ConsumeLogin(token, guess); ok {
			t.Fatalf("FISHING: soxta kalit qabul qilindi: %q", guess)
		}
	}
	// Yozuv o'chib ketmagan bo'lishi kerak — halol ilova kalit bilan
	// biroz keyin keladi va kira olishi shart.
	secret := secretFromReturnLink(t, bot)
	if _, ok := v.ConsumeLogin(token, secret); !ok {
		t.Fatal("halol ilova to'g'ri kalit bilan ham kira olmadi")
	}
}

// Kalit deep linkda (`t.me/...?start=`) BO'LMASLIGI shart — aks holda
// hujumchi uni havola bilan birga tarqatib yuborardi.
func TestConfirmSecretNotInStartLink(t *testing.T) {
	v, bot, _ := newTestVerifier(t)
	link, token, err := v.StartLogin(context.Background())
	if err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	v.handle(context.Background(), startUpdate(200, token))
	v.handle(context.Background(), contactUpdate(200, "998901234567", 55, 55))

	secret := secretFromReturnLink(t, bot)
	if secret == "" {
		t.Fatal("kalit umuman yuborilmadi")
	}
	if strings.Contains(link, secret) {
		t.Fatalf("KALIT DEEP LINKKA TUSHGAN: %s", link)
	}
}

func TestLoginResultUnknownToken(t *testing.T) {
	v, _, _ := newTestVerifier(t)
	if found, _, _ := v.LoginResult("yolgon"); found {
		t.Fatal("noma'lum token topildi")
	}
	if _, ok := v.ConsumeLogin("yolgon", "yolgon-kalit"); ok {
		t.Fatal("noma'lum token bilan natija olindi")
	}
}

// Botga to'g'ridan-to'g'ri yozgan odam (deep linksiz) tasdiqlay
// olmasligi kerak.
func TestContactWithoutStartRejected(t *testing.T) {
	v, _, issued := newTestVerifier(t)
	v.handle(context.Background(), contactUpdate(100, "998901234567", 55, 55))
	if *issued != 0 {
		t.Fatal("faol so'rovsiz kod yaratildi")
	}
}
