package users

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"regexp"
	"strings"
	"time"
)

const (
	codeTTL        = 5 * time.Minute
	resendCooldown = 60 * time.Second
	maxAttempts    = 5
)

var phoneRe = regexp.MustCompile(`^\+998\d{9}$`)

type Service struct {
	users  Repository
	codes  CodeStore
	sms    SmsSender
	email  EmailSender
	tokens *TokenIssuer
	idgen  func() string
	now    func() time.Time
	// emailConfigured — haqiqiy SMTP ulanganmi. Ulanmagan bo'lsa email
	// oqimlari ANIQ xato bilan rad etiladi (`ErrEmailSendUnavailable`),
	// jimgina "yuborildi" deyilmaydi.
	emailConfigured bool
}

func NewService(users Repository, codes CodeStore, sms SmsSender, tokens *TokenIssuer, idgen func() string) *Service {
	return &Service{users: users, codes: codes, sms: sms, tokens: tokens, idgen: idgen, now: time.Now}
}

// WithEmail — email yuboruvchini ulaydi. `configured=false` bo'lsa
// (SMTP sozlanmagan) email oqimlari ochilmaydi.
func (s *Service) WithEmail(sender EmailSender, configured bool) *Service {
	s.email = sender
	s.emailConfigured = configured
	return s
}

// NormalizePhone — "+998 90 123-45-67" yoki "998901234567" ni "+998901234567" ga keltiradi.
func NormalizePhone(raw string) (string, error) {
	p := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(raw))
	if strings.HasPrefix(p, "998") && len(p) == 12 {
		p = "+" + p
	}
	if !phoneRe.MatchString(p) {
		return "", ErrInvalidPhone
	}
	return p, nil
}

// IssueCode — 6 xonali kodni yaratadi va saqlaydi, LEKIN HECH QAYERGA
// YUBORMAYDI. Yetkazish chaqiruvchining zimmasida.
//
// ┌─ NEGA YARATISH VA YETKAZISH AJRATILGAN ───────────────────────────┐
// Telegram boti kodni CHATNING O'ZIDA yetkazadi (`telegram.Verifier`),
// ya'ni unga SMS umuman kerak emas. Ilgari bot ham `RequestCode` ni
// chaqirardi, u esa majburan SMS yuborardi. Ikki oqibati bor edi:
//
//  1. Eskiz yiqilsa (masalan jo'natuvchi nomi ro'yxatdan o'tmagan
//     bo'lsa) `RequestCode` xato qaytarardi va BOT ham "Hozir kod
//     yuborib bo'lmadi" deb javob berardi — kod allaqachon saqlangan
//     bo'lsa ham. Mustaqil kanal begona kanalning nosozligidan o'lardi.
//  2. Har bir Telegram kirishi ustiga keraksiz SMS yozilardi (pul).
//
// Endi bitta kanalning nosozligi ikkinchisini to'xtatmaydi.
// └───────────────────────────────────────────────────────────────────┘
func (s *Service) IssueCode(ctx context.Context, rawPhone string) (phone, code string, err error) {
	phone, err = NormalizePhone(rawPhone)
	if err != nil {
		return "", "", err
	}
	if existing, err := s.codes.Get(ctx, phone); err == nil {
		if elapsed := s.now().Sub(existing.CreatedAt); elapsed < resendCooldown {
			return "", "", TooSoonError{Wait: resendCooldown - elapsed}
		}
	}
	code, err = randomCode()
	if err != nil {
		return "", "", err
	}
	if err := s.codes.Save(ctx, &Code{
		Target:    phone,
		CodeHash:  s.hashCode(phone, code),
		ExpiresAt: s.now().Add(codeTTL),
		CreatedAt: s.now(),
	}); err != nil {
		return "", "", err
	}
	return phone, code, nil
}

// RequestCode — kod yaratadi va uni SMS orqali yetkazadi. Kodni
// qaytaradi, lekin handler uni faqat dev rejimda javobga qo'shadi
// (production'da faqat SMS orqali boradi).
//
// SMS KERAK BO'LMAGAN kanallar (Telegram boti) `IssueCode` ni
// chaqirsin — izohiga qarang.
func (s *Service) RequestCode(ctx context.Context, rawPhone string) (phone, code string, err error) {
	phone, code, err = s.IssueCode(ctx, rawPhone)
	if err != nil {
		return "", "", err
	}
	// DIQQAT: bu matn Eskiz kabinetida MODERATSIYADAN o'tgan shablon
	// bilan AYNAN bir xil bo'lishi shart. Tasdiqlanmagan matnni Eskiz
	// rad etadi va foydalanuvchi kodni umuman olmaydi. Matnni
	// o'zgartirsangiz — avval Eskiz'da yangi shablonni tasdiqlating.
	if err := s.sms.Send(phone, fmt.Sprintf("OnDex tasdiqlash kodi: %s", code)); err != nil {
		return "", "", err
	}
	return phone, code, nil
}

// Verify — kod to'g'ri bo'lsa foydalanuvchini topadi (yo'q bo'lsa mijoz sifatida
// yaratadi) va JWT token qaytaradi.
func (s *Service) Verify(ctx context.Context, rawPhone, code string) (string, *User, error) {
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return "", nil, err
	}
	c, err := s.codes.Get(ctx, phone)
	if err != nil {
		return "", nil, ErrInvalidCode
	}
	if s.now().After(c.ExpiresAt) {
		s.codes.Delete(ctx, phone)
		return "", nil, ErrInvalidCode
	}
	// Urinish AVVAL atomik hisoblanadi, KEYIN chegara tekshiriladi —
	// aks holda parallel so'rovlar bir xil eski qiymatni o'qib,
	// chegarani chetlab o'tardi (interfeys izohiga qarang).
	attempts, err := s.codes.IncrementAttempts(ctx, phone)
	if err != nil {
		return "", nil, ErrInvalidCode
	}
	if attempts > maxAttempts {
		s.codes.Delete(ctx, phone)
		return "", nil, ErrTooManyAttempts
	}
	if !s.sameCode(phone, code, c.CodeHash) {
		return "", nil, ErrInvalidCode
	}
	s.codes.Delete(ctx, phone)

	return s.finishPhoneLogin(ctx, phone)
}

// LoginWithFirebasePhone — raqam Firebase tomonidan tasdiqlangandan
// keyingi kirish.
//
// XAVFSIZLIK: `phone` FAQAT tekshirilgan Firebase ID tokenidan
// olinishi shart (`internal/firebaseauth`). Bu funksiya raqamning
// qayerdan kelganini bila olmaydi — chaqiruvchi mijoz yuborgan xom
// qiymatni bu yerga UZATMASLIGI kerak, aks holda istalgan odam
// istalgan akkauntga kirardi.
func (s *Service) LoginWithFirebasePhone(ctx context.Context, verifiedPhone string) (string, *User, error) {
	phone, err := NormalizePhone(verifiedPhone)
	if err != nil {
		return "", nil, err
	}
	return s.finishPhoneLogin(ctx, phone)
}

// ErrTelegramNotLinked — Telegram akkaunti hali biror raqamga
// bog'lanmagan. Mini App buni ko'rib foydalanuvchini kontakt
// ulashishga taklif qiladi.
var ErrTelegramNotLinked = errors.New("telegram akkaunti raqamga bog'lanmagan")

// LinkTelegramPhone — botda KONTAKT ULASHILGANDA chaqiriladi.
//
// ┌─ NEGA AYNAN SHU YERDA ────────────────────────────────────────────┐
// Telegram Mini App `initData` da telefon raqami YO'Q — u faqat
// Telegram ID beradi. Raqamni olishning YAGONA yo'li — botdagi
// "kontaktni ulashish" tugmasi (`AskContact`).
//
// Shu sabab bog'lanish aynan kontakt kelgan paytda yoziladi. Undan
// keyin Mini App faqat ID bilan kelsa ham server raqamni biladi.
// └───────────────────────────────────────────────────────────────────┘
//
// XAVFSIZLIK: `verifiedPhone` FAQAT Telegram yuborgan kontaktdan
// olinishi shart va kontakt EGASI tekshirilgan bo'lishi kerak
// (`m.Contact.UserID == m.From.ID` — `internal/telegram/verifier.go`).
// Busiz istalgan odam BEGONA raqamni ulashib, o'sha akkauntni o'ziga
// bog'lab olardi.
//
// Akkaunt topilmasa YARATILADI — telefon oqimi bilan bir xil yo'l,
// ya'ni keyin SMS bilan kirgan odam AYNAN SHU akkauntga tushadi.
func (s *Service) LinkTelegramPhone(ctx context.Context, telegramID int64, verifiedPhone string) (*User, error) {
	if telegramID == 0 {
		return nil, errors.New("telegram ID bo'sh")
	}
	phone, err := NormalizePhone(verifiedPhone)
	if err != nil {
		return nil, err
	}
	u, err := s.users.GetByPhone(ctx, phone)
	if errors.Is(err, ErrUserNotFound) {
		u = &User{
			ID:            s.idgen(),
			Phone:         phone,
			Role:          RoleCustomer,
			PhoneVerified: true, // raqamni Telegram tasdiqladi
			CreatedAt:     s.now(),
		}
		if err := s.users.Create(ctx, u); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if err := s.users.LinkTelegram(ctx, u.ID, telegramID); err != nil {
		return nil, err
	}
	u.TelegramID = telegramID
	return u, nil
}

// LoginWithTelegramID — Mini App kirishi.
//
// XAVFSIZLIK: `telegramID` FAQAT imzosi tekshirilgan `initData` dan
// olinishi shart (`telegram.ValidateInitData`). Xom, klient yuborgan
// ID bu yerga UZATILMASLIGI kerak — aks holda istalgan odam raqamni
// almashtirib begona akkauntga kirardi.
//
// Bog'lanish topilmasa `ErrTelegramNotLinked` — chaqiruvchi
// foydalanuvchini kontakt ulashishga yo'naltiradi.
func (s *Service) LoginWithTelegramID(ctx context.Context, telegramID int64) (string, *User, error) {
	if telegramID == 0 {
		return "", nil, ErrTelegramNotLinked
	}
	u, err := s.users.GetByTelegramID(ctx, telegramID)
	if errors.Is(err, ErrUserNotFound) {
		return "", nil, ErrTelegramNotLinked
	}
	if err != nil {
		return "", nil, err
	}
	// ┌─ NEGA `PhoneProven` EMAS ─────────────────────────────────────┐
	// Bu token bilan parol o'zgartirishga JORIY PAROL so'raladi.
	// `initData` — Telegram seansining isboti, foydalanuvchi ayni
	// damda raqamiga ega ekanining isboti EMAS (telefon o'g'irlangan
	// yoki Telegram seansi ochiq qolgan bo'lishi mumkin).
	// └───────────────────────────────────────────────────────────────┘
	token, err := s.tokens.Issue(u)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

// finishPhoneLogin — raqam TASDIQLANGANDAN keyingi umumiy qism:
// foydalanuvchini topish/yaratish va token berish.
//
// SMS kod (`Verify`) va Firebase (`LoginWithFirebasePhone`) yo'llari
// shu yerda birlashadi — ikkalasi ham "raqam egaligi isbotlandi"
// degan bir xil holatga keladi, shuning uchun mantiq BITTA joyda.
func (s *Service) finishPhoneLogin(ctx context.Context, phone string) (string, *User, error) {
	u, err := s.users.GetByPhone(ctx, phone)
	if errors.Is(err, ErrUserNotFound) {
		u = &User{
			ID:            s.idgen(),
			Phone:         phone,
			Role:          RoleCustomer,
			PhoneVerified: true, // raqam endigina tasdiqlandi
			CreatedAt:     s.now(),
		}
		if err := s.users.Create(ctx, u); err != nil {
			return "", nil, err
		}
	} else if err != nil {
		return "", nil, err
	} else if !u.PhoneVerified {
		// Ro'yxatdan o'tishda yaratilgan, lekin hali tasdiqlanmagan
		// akkaunt — kod to'g'ri kelgani uchun endi tasdiqlanadi.
		//
		// IKKINCHI HIMOYA QATLAMI: tasdiqlanmagan yozuvdagi parol
		// hash'i TOZALANADI. `Register` endi telefon rejimida parol
		// yozmaydi (o'sha izohdagi akkaunt egallash zaifligi), lekin
		// bu tuzatishdan OLDIN yaratilgan yozuvlarda begona hash
		// qolgan bo'lishi mumkin — ular tasdiqlanganda tirilib
		// ketmasligi kerak. Foydalanuvchi kirgandan keyin parolni
		// `POST /me/password` orqali o'zi qo'yadi.
		// ATAYLAB `SetPasswordHash`, `UpdateProfile` EMAS: ikkinchisi
		// yon ta'sir sifatida `name` ni first/last dan qayta hisoblaydi
		// va admin yaratgan (nomi bor, first/last si bo'sh) restoran/
		// kuryer akkauntlarining nomini o'chirib yuborardi.
		if err := s.users.SetPasswordHash(ctx, u.ID, ""); err != nil {
			return "", nil, err
		}
		if err := s.users.MarkPhoneVerified(ctx, u.ID); err != nil {
			return "", nil, err
		}
		u.PhoneVerified = true
		u.PasswordHash = ""
	}

	// Raqam tasdiqlandi — token "telefon egaligi isbotlangan" deb
	// belgilanadi. Bu FAQAT parolni joriy parolsiz o'rnatishga ruxsat
	// beradi (parolini unutgan foydalanuvchi uchun) va 15 daqiqadan
	// keyin kuchini yo'qotadi. Qarang: `Claims.PhoneProven`.
	token, err := s.tokens.IssuePhoneProven(u)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

// emailIsIdentity — topilgan yozuvda email KIMLIK sifatida ishlay
// oladimi (ya'ni "bu manzil egasi = bu akkaunt egasi" deb hisoblash
// mumkinmi).
//
// ┌─ NEGA BU QOIDA KERAK ─────────────────────────────────────────────┐
// `users.email` ustuni ikki xil ma'noda ishlatiladi:
//
//	(a) KIMLIK — email bilan ro'yxatdan o'tgan akkaunt uchun;
//	(b) profildagi oddiy bog'lanish ma'lumoti — telefon bilan
//	    ro'yxatdan o'tgan odam formada ixtiyoriy ravishda yozgan manzil.
//
// (b) HECH QACHON tasdiqlanmaydi. Uni kimlik deb qabul qilish quyidagi
// hujumni ochib qo'yardi (jonli isbotlangan, `email_identity_test.go`):
//
//	hujumchi O'Z RAQAMI bilan register qiladi, `email` maydoniga
//	QURBONNING manzilini yozadi -> o'z SMS kodini kiritib yozuvni
//	tasdiqlaydi -> qurbon o'sha manzil bilan Google orqali kirganda
//	HUJUMCHINING akkauntiga tushardi (hujumchi esa unga o'z telefoni
//	orqali kirib turaverardi).
//
// Qoida: manzil kimlik bo'lishi uchun YO tasdiqlangan bo'lsin, YO
// yozuvda boshqa kimlik (telefon) umuman bo'lmasin — ya'ni yozuv
// aynan email oqimida yaratilgan bo'lsin.
// └───────────────────────────────────────────────────────────────────┘
func emailIsIdentity(u *User) bool { return u.EmailVerified || u.Phone == "" }

// releaseUnprovenEmail — begona yozuvga yopishtirilgan, HECH QACHON
// tasdiqlanmagan manzilni bo'shatadi.
//
// Bu ma'lumot yo'qotish emas: yozuv bu manzilga egalikni hech qachon
// isbotlamagan, egaligini isbotlagan odam esa hozir shu yerda turibdi.
// Bo'shatilgandan keyin manzil haqiqiy egasiga ochiladi (`email <> ''`
// qisman unikal indeksi bo'sh qiymatlarni to'qnashtirmaydi —
// migration 0026).
func (s *Service) releaseUnprovenEmail(ctx context.Context, u *User) error {
	empty := ""
	if err := s.users.UpdateProfile(ctx, u.ID, ProfileUpdate{Email: &empty}); err != nil {
		return err
	}
	slog.Warn("tasdiqlanmagan email boshqa yozuvdan bo'shatildi",
		"user", u.ID, "sabab", "manzil egaligi boshqa odam tomonidan isbotlandi")
	return nil
}

// LoginWithGoogle — Google hisobi bilan kirish/ro'yxatdan o'tish.
//
// XAVFSIZLIK: `verifiedEmail` FAQAT tekshirilgan Firebase ID
// tokenidan olinishi va u yerda `sign_in_provider == "google.com"`
// hamda `email_verified == true` bo'lishi SHART
// (`firebaseauth.Token.RequireGoogleEmail`). Bu funksiya manzilning
// qayerdan kelganini bila olmaydi — mijoz yuborgan xom qiymatni bu
// yerga UZATMASLIK kerak.
//
// Akkaunt topilmasa YARATILADI: Google manzil egaligini isbotlagan,
// ya'ni bu "tasdiqlangan ro'yxatdan o'tish" bilan teng.
func (s *Service) LoginWithGoogle(ctx context.Context, verifiedEmail, fullName string) (string, *User, error) {
	email, err := NormalizeEmail(verifiedEmail)
	if err != nil {
		return "", nil, err
	}

	u, err := s.users.GetByEmail(ctx, email)
	switch {
	case errors.Is(err, ErrUserNotFound):
		u = nil
	case err != nil:
		return "", nil, err
	case !emailIsIdentity(u):
		// Manzil BEGONA yozuvga (telefon bilan ochilgan akkauntga)
		// tasdiqlanmagan holda yopishtirilgan. Google endi egalikni
		// isbotladi — u yozuvga KIRISH huquqini bermaydi, faqat
		// manzilni bo'shatadi va Google egasiga o'z akkaunti ochiladi.
		// `emailIsIdentity` izohidagi hujumga qarang.
		if err := s.releaseUnprovenEmail(ctx, u); err != nil {
			return "", nil, err
		}
		u = nil
	}

	if u == nil {
		first, last := splitName(fullName)
		u = &User{
			ID: s.idgen(), Role: RoleCustomer,
			Email: email, EmailVerified: true,
			FirstName: first, LastName: last,
			Name:      strings.TrimSpace(fullName),
			CreatedAt: s.now(),
		}
		if err := s.users.Create(ctx, u); err != nil {
			return "", nil, err
		}
	} else if !u.EmailVerified {
		// Manzil avval TASDIQLANMAGAN holda band qilingan (kimdir
		// email bilan ro'yxatdan o'tishni boshlagan, lekin kodni
		// kiritmagan). Yozuvda telefon YO'Q (yuqoridagi tekshiruvdan
		// o'tdi), ya'ni u aynan shu manzil uchun ochilgan. Google
		// egalikni isbotladi — yozuv shu odamga o'tadi.
		//
		// PAROL TOZALANADI — telefon oqimidagi bilan bir xil sabab:
		// tasdiqlanmagan yozuvga begona odam parol qo'yib qo'ygan
		// bo'lishi mumkin va u tasdiqlangandan keyin tirilib
		// ketmasligi kerak.
		if err := s.users.SetPasswordHash(ctx, u.ID, ""); err != nil {
			return "", nil, err
		}
		if err := s.users.MarkEmailVerified(ctx, u.ID); err != nil {
			return "", nil, err
		}
		u.EmailVerified = true
		u.PasswordHash = ""
	}

	// Google kirish PAROL O'RNATISH huquqini bermaydi: `Issue`
	// (`IssuePhoneProven` emas). Aks holda Google hisobiga ega
	// bo'lgan odam parolni joriy parolsiz almashtira olardi — bu
	// imtiyoz faqat bir martalik kod bilan tasdiqlanganda beriladi.
	token, err := s.tokens.Issue(u)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

// splitName — "Shoxrux Turaqulov" -> ("Shoxrux", "Turaqulov").
// Google to'liq ismni bitta maydonda beradi.
func splitName(full string) (first, last string) {
	parts := strings.Fields(strings.TrimSpace(full))
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

// ---------- Email orqali tasdiqlash ----------

// NormalizeEmail — kichik harf + bo'sh joylarni olib tashlash +
// qat'iy format tekshiruvi.
//
// XAVFSIZLIK: CR/LF belgilari ALOHIDA rad etiladi — manzil SMTP
// sarlavhasiga tushadi va u yerda yangi qator hujumchiga o'z
// sarlavhasini (masalan `Bcc:`) qo'shish imkonini berardi. Yuborish
// qatlamida ham tekshiruv bor (`notify.guardHeader`) — ikki qatlam
// ataylab.
func NormalizeEmail(raw string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if e == "" || len(e) > MaxEmailLength || !emailRe.MatchString(e) {
		return "", ErrInvalidEmail
	}
	if strings.ContainsAny(e, "\r\n") {
		return "", ErrInvalidEmail
	}
	return e, nil
}

// RequestEmailCode — emailga 6 xonali kod yuboradi.
//
// SMS oqimi bilan BIR XIL himoya: bir martalik kod, hash bilan
// saqlanadi, 5 daqiqa amal qiladi, 60 soniyalik qayta yuborish
// pauzasi, 5 urinish chegarasi (`Verify`/`VerifyEmail` da).
//
// FOYDALANUVCHINI SANAB OLISH: bu funksiya akkaunt bor-yo'qligini
// TEKSHIRMAYDI va javob har doim bir xil bo'ladi.
func (s *Service) RequestEmailCode(ctx context.Context, rawEmail string) (email, code string, err error) {
	if !s.emailConfigured || s.email == nil {
		return "", "", ErrEmailSendUnavailable
	}
	email, err = NormalizeEmail(rawEmail)
	if err != nil {
		return "", "", err
	}
	if existing, err := s.codes.Get(ctx, email); err == nil {
		if elapsed := s.now().Sub(existing.CreatedAt); elapsed < resendCooldown {
			return "", "", TooSoonError{Wait: resendCooldown - elapsed}
		}
	}
	code, err = randomCode()
	if err != nil {
		return "", "", err
	}
	if err := s.codes.Save(ctx, &Code{
		Target:    email,
		CodeHash:  s.hashCode(email, code),
		ExpiresAt: s.now().Add(codeTTL),
		CreatedAt: s.now(),
	}); err != nil {
		return "", "", err
	}
	subject, text, htmlBody := VerificationEmail(code, int(codeTTL.Minutes()))
	if err := s.email.Send(email, subject, text, htmlBody); err != nil {
		return "", "", err
	}
	return email, code, nil
}

// VerifyEmail — email kodini tekshiradi va tokenni qaytaradi.
//
// `Verify` (telefon) bilan bir xil mantiq, shu jumladan urinishlarni
// ATOMIK sanash. FARQI: bu yerda akkaunt YARATILMAYDI — email oqimi
// har doim ro'yxatdan o'tishdan boshlanadi, ya'ni yozuv allaqachon
// mavjud bo'lishi kerak. Aks holda begona email uchun kod so'rab,
// tasdiqlab, akkaunt ochib olish mumkin bo'lardi.
func (s *Service) VerifyEmail(ctx context.Context, rawEmail, code string) (string, *User, error) {
	if !s.emailConfigured || s.email == nil {
		return "", nil, ErrEmailSendUnavailable
	}
	email, err := NormalizeEmail(rawEmail)
	if err != nil {
		return "", nil, err
	}
	c, err := s.codes.Get(ctx, email)
	if err != nil {
		return "", nil, ErrInvalidCode
	}
	if s.now().After(c.ExpiresAt) {
		s.codes.Delete(ctx, email)
		return "", nil, ErrInvalidCode
	}
	attempts, err := s.codes.IncrementAttempts(ctx, email)
	if err != nil {
		return "", nil, ErrInvalidCode
	}
	if attempts > maxAttempts {
		s.codes.Delete(ctx, email)
		return "", nil, ErrTooManyAttempts
	}
	if !s.sameCode(email, code, c.CodeHash) {
		return "", nil, ErrInvalidCode
	}
	s.codes.Delete(ctx, email)

	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		// Akkaunt yo'q — kod to'g'ri bo'lsa ham yaratmaymiz (yuqoridagi
		// izoh). Xato mijozga "kod noto'g'ri" ko'rinishida qaytadi,
		// ya'ni manzil ro'yxatda bor-yo'qligi OSHKOR BO'LMAYDI.
		return "", nil, ErrInvalidCode
	}
	if !emailIsIdentity(u) {
		// Manzil BEGONA yozuvga tasdiqlanmagan holda yopishtirilgan
		// (`emailIsIdentity` izohidagi hujum). Kod manzil egaligini
		// isbotlaydi, LEKIN o'sha yozuvga kirish huquqini bermaydi.
		// Manzilni bo'shatamiz va "akkaunt yo'q" bilan BIR XIL javob
		// qaytaramiz — aks holda javobdagi farqning o'zi hujumchiga
		// nishon topilganini aytib qo'yardi.
		if err := s.releaseUnprovenEmail(ctx, u); err != nil {
			return "", nil, err
		}
		return "", nil, ErrInvalidCode
	}
	if !u.EmailVerified {
		// Tasdiqlanmagan yozuvdagi parol hash'i TOZALANADI — telefon
		// oqimidagi akkaunt egallash zaifligining aynan email varianti:
		// begona odam sizning emailingiz bilan ro'yxatdan o'tib, o'z
		// parolini qo'yib qo'yishi mumkin edi.
		if err := s.users.SetPasswordHash(ctx, u.ID, ""); err != nil {
			return "", nil, err
		}
		if err := s.users.MarkEmailVerified(ctx, u.ID); err != nil {
			return "", nil, err
		}
		u.EmailVerified = true
		u.PasswordHash = ""
	}

	// Telefon oqimi bilan bir xil: kod tasdiqlangani parolni joriy
	// parolsiz o'rnatishga 15 daqiqalik ruxsat beradi.
	token, err := s.tokens.IssuePhoneProven(u)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// hashCode — OTP kodining saqlanadigan shakli.
//
// ┌─ TUZATILGAN NOSOZLIK (bug.md 48-band) ─────────────────────────────┐
// Avval bu oddiy, TUZSIZ SHA-256 edi:
//
//	sum := sha256.Sum256([]byte(code))
//
// Kod maydoni — 10⁶ (`%06d`). Tuz yo'q, pepper yo'q, kalit cho'zish
// yo'q: bir million qiymatning oldindan hisoblangan jadvali (bir
// necha soniyalik ish) BARCHA hashlarni bir zumda ochardi. Ya'ni
// kodlar ombori (Redis yoki Postgres) o'qilgan holatda hujumchi
// istalgan raqamning kodini bilardi.
//
// Endi ikki qatlam:
//
//  1. `target` (telefon/email) hashga KIRADI — u har yozuvda boshqa,
//     ya'ni TUZ vazifasini bajaradi. Bitta jadval endi faqat BITTA
//     nishonga yaraydi, hammasiga emas.
//  2. HMAC + server tomonidagi PEPPER (`TokenIssuer.codePepper()`).
//     Pepper bazada YO'Q — u faqat serverning xotirasida. Ya'ni
//     bazani o'qish yolg'iz o'zi yetarli emas.
//
// Kalit cho'zish (Argon2) ATAYLAB qo'llanmadi: kod 5 daqiqa yashaydi
// va 5 urinish chegarasi bor, hashlash esa har `request-code` va har
// `verify` da bajariladi — sekin funksiya bu yerda DoS yuzasi
// bo'lardi. Pepper bir xil himoyani narxsiz beradi.
// └────────────────────────────────────────────────────────────────────┘
func (s *Service) hashCode(target, code string) string {
	mac := hmac.New(sha256.New, s.tokens.codePepper())
	mac.Write([]byte(target))
	mac.Write([]byte{0}) // ajratgich: "a"+"bc" va "ab"+"c" bir xil bo'lmasin
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

// sameCode — kiritilgan kodni saqlangan hash bilan solishtiradi.
//
// `subtle.ConstantTimeCompare` — oddiy `!=` EMAS: satrlarni taqqoslash
// birinchi farqda to'xtaydi va javob vaqti qancha belgi mos kelganini
// bildiradi. Amalda bu yerda uni ishlatish qiyin (6 xonali kod bor-yo'g'i
// 5 urinishga ega), lekin bu qoidaga har joyda amal qilish arzon va
// keyinchalik kod uzayganda/urinishlar chegarasi yumshaganda o'zi
// ishlab turadi.
func (s *Service) sameCode(target, input, storedHash string) bool {
	got := s.hashCode(target, strings.TrimSpace(input))
	return subtle.ConstantTimeCompare([]byte(got), []byte(storedHash)) == 1
}
