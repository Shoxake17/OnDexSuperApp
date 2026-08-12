package users

import (
	"context"
	"errors"
	"time"
)

type Role string

const (
	RoleCustomer   Role = "customer"
	RoleCourier    Role = "courier"
	RoleRestaurant Role = "restaurant"
	RoleAdmin      Role = "admin"
)

type User struct {
	ID    string `json:"id"`
	Phone string `json:"phone"`
	// Name — to'liq ism ("Ism Familiya"). FirstName/LastName dan
	// avtomatik quriladi; eski kod (profil, panellar) shuni o'qiydi.
	Name      string `json:"name"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
	// Email — ixtiyoriy, bo'sh bo'lishi mumkin. Bo'sh bo'lmasa UNIKAL.
	Email string `json:"email,omitempty"`
	// PasswordHash — Argon2id (internal/users/password.go). Bo'sh =
	// parol o'rnatilmagan, foydalanuvchi faqat SMS kod bilan kiradi.
	//
	// XAVFSIZLIK: `json:"-"` — hash HECH QACHON HTTP javobiga
	// tushmasligi kerak. `User` struct'i `/me`, `/auth/verify` va
	// admin ro'yxatlarida to'g'ridan-to'g'ri serializatsiya qilinadi,
	// shuning uchun bu teg yagona himoya.
	PasswordHash string `json:"-"`
	// PhoneVerified — SMS kod bilan tasdiqlanganmi. Ro'yxatdan
	// o'tishda akkaunt yaratiladi, lekin tasdiqlanmaguncha token
	// berilmaydi (begona raqamni band qilib qo'yishga qarshi).
	PhoneVerified bool `json:"phone_verified"`
	// EmailVerified — email bir martalik kod bilan tasdiqlanganmi.
	// Telefon bilan bir xil qoida: email orqali ro'yxatdan o'tilsa
	// akkaunt tasdiqlanmagan holatda yaratiladi va parol bilan kirish
	// FAQAT tasdiqdan keyin ochiladi. Busiz istalgan odam BEGONA
	// email bilan akkaunt ochib, o'sha manzilni band qilib qo'yardi.
	EmailVerified bool `json:"email_verified"`
	Role          Role `json:"role"`
	// EntityID — rol bog'langan obyekt: kuryer uchun kuryer IDsi (c1),
	// restoran xodimi uchun restoran IDsi (r1). Mijoz/admin uchun bo'sh.
	EntityID  string    `json:"entity_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	// TelegramID — Telegram Mini App uchun bog'lanish (migration 0031).
	//
	// ┌─ NEGA KERAK ──────────────────────────────────────────────────┐
	// Mini App `initData` da FAQAT Telegram ID bo'ladi — telefon
	// raqami u yerda YO'Q va hech qachon bo'lmaydi. OnDex'da esa
	// kimlik telefon raqami.
	//
	// Foydalanuvchi botda kontaktini ulashganda shu maydon
	// to'ldiriladi. Keyin Mini App faqat ID bilan kelsa ham server
	// uni to'g'ri akkauntga bog'lay oladi.
	// └───────────────────────────────────────────────────────────────┘
	//
	// `json:"-"` — bu ichki bog'lanish, HTTP javobida chiqishi shart
	// emas va boshqa foydalanuvchilarga ko'rinmasligi kerak.
	TelegramID int64 `json:"-"`

	// Mijozning saqlangan yetkazib berish manzili (xaritadan tanlangan).
	// Hammasi ixtiyoriy — hali tanlanmagan bo'lsa bo'sh/0 qiymatlar.
	Address AddressDetails `json:"address"`
}

// AddressDetails — mijoz xaritadan tanlagan yetkazib berish nuqtasi va
// kuryer uchun qo'shimcha tafsilotlar (Yandex Go uslubidagi manzil shakli).
type AddressDetails struct {
	Lat       float64 `json:"lat,omitempty"`
	Lng       float64 `json:"lng,omitempty"`
	Text      string  `json:"text,omitempty"`      // masalan "Mirobod ko'chasi, 41"
	Entrance  string  `json:"entrance,omitempty"`  // podъezd
	Floor     string  `json:"floor,omitempty"`     // qavat
	Apartment string  `json:"apartment,omitempty"` // kvartira
	Intercom  string  `json:"intercom,omitempty"`  // domofon kodi
	Comment   string  `json:"comment,omitempty"`   // kuryer uchun izoh
}

var (
	ErrUserNotFound    = errors.New("foydalanuvchi topilmadi")
	ErrInvalidPhone    = errors.New("telefon raqam formati noto'g'ri (+998XXXXXXXXX bo'lishi kerak)")
	ErrInvalidCode     = errors.New("kod noto'g'ri yoki muddati tugagan")
	ErrTooManyAttempts = errors.New("juda ko'p noto'g'ri urinish — yangi kod so'rang")
	ErrTooSoon         = errors.New("yangi kod so'rash uchun biroz kuting")
	ErrPhoneTaken      = errors.New("bu telefon raqam allaqachon ro'yxatdan o'tgan")
	ErrEmailTaken      = errors.New("bu email allaqachon ro'yxatdan o'tgan")
	// ErrInvalidCredentials — parol bilan kirishda YAGONA xato.
	//
	// ATAYLAB "foydalanuvchi topilmadi" va "parol noto'g'ri" holatlari
	// AJRATILMAYDI: aks holda hujumchi qaysi telefon/email ro'yxatda
	// borligini aniqlab olardi (user enumeration).
	ErrInvalidCredentials = errors.New("telefon/email yoki parol noto'g'ri")
	ErrPhoneNotVerified   = errors.New("telefon raqam tasdiqlanmagan — SMS kodni kiriting")
	// ErrServerBusy — parol tekshirish navbati to'lgan (Argon2 chegarasi,
	// `password.go` dagi izohga qarang). Vaqtinchalik holat: mijoz
	// biroz kutib qayta urinishi kerak, shuning uchun 503 + Retry-After.
	ErrServerBusy = errors.New("server hozir band — biroz kutib qayta urinib ko'ring")
	ErrEmailNotVerified   = errors.New("email tasdiqlanmagan — emailingizga yuborilgan kodni kiriting")
	ErrInvalidEmail       = errors.New("email formati noto'g'ri")
	// ErrEmailSendUnavailable — SMTP sozlanmagan. Mijozga ANIQ
	// ko'rsatiladi; jimgina "yuborildi" deyilmaydi.
	ErrEmailSendUnavailable = errors.New("email yuborish hali ulanmagan — telefon raqami orqali davom eting")
	// ErrCurrentPasswordWrong — parolni O'ZGARTIRISHDA joriy parol
	// noto'g'ri. Bu yerda enumeration xavfi YO'Q: foydalanuvchi
	// allaqachon o'z tokeni bilan kirgan, ya'ni akkauntning mavjudligi
	// undan sir emas.
	ErrCurrentPasswordWrong = errors.New("joriy parol noto'g'ri")
)

// TooSoonError — kod QAYTA so'rashgacha qancha qolganini ham
// bildiradigan `ErrTooSoon`.
//
// NEGA KERAK: "yangi kod so'rash uchun biroz kuting" foydalanuvchiga
// hech narsa aytmaydi — u qayta-qayta bosaveradi va har safar o'sha
// javobni oladi. `errors.Is(err, ErrTooSoon)` ishlashda davom etadi
// (`Unwrap` orqali), ya'ni mavjud kod buzilmaydi.
type TooSoonError struct{ Wait time.Duration }

func (e TooSoonError) Error() string { return ErrTooSoon.Error() }
func (e TooSoonError) Unwrap() error { return ErrTooSoon }

// ValidationError — foydalanuvchi kiritmasidagi xato. Matni MIJOZGA
// KO'RSATISH XAVFSIZ (u foydalanuvchining o'zi yozgan narsa haqida).
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return e.Msg }

func invalidInput(msg string) error { return ValidationError{Msg: msg} }

// userFacing — matni mijozga O'ZGARISHSIZ uzatilishi mumkin bo'lgan
// xatolar. Ro'yxatda yo'q xato — kutilmagan ichki nosozlik (pgx/mongo
// drayveri, tarmoq) va uning matni MIJOZGA CHIQMAYDI.
var userFacing = []error{
	ErrInvalidPhone, ErrInvalidCode, ErrTooManyAttempts, ErrTooSoon,
	ErrPhoneTaken, ErrEmailTaken, ErrInvalidCredentials,
	ErrPhoneNotVerified, ErrInvalidEmail, ErrCurrentPasswordWrong,
	ErrPasswordTooShort, ErrPasswordTooLong, ErrPasswordTooCommon,
	ErrPasswordMismatch, ErrEmailNotVerified, ErrEmailSendUnavailable,
	ErrServerBusy,
}

// IsUserFacing — xatoni mijozga o'zgarishsiz ko'rsatish mumkinmi.
//
// NEGA KERAK: `respond.go` faqat 5xx uchun matnni yashiradi, auth
// handlerlari esa HAR QANDAY xatoni 400/401 qilib qaytarardi. Ya'ni
// bazadagi unikal indeks buzilishi mijozga cheklov (constraint) nomi
// bilan birga borardi — sxema haqida keraksiz ma'lumot.
func IsUserFacing(err error) bool {
	var ve ValidationError
	if errors.As(err, &ve) {
		return true
	}
	for _, known := range userFacing {
		if errors.Is(err, known) {
			return true
		}
	}
	return false
}

type Repository interface {
	GetByPhone(ctx context.Context, phone string) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	Create(ctx context.Context, u *User) error
	// UpdateRole — mijoz kuryer bo'lganda (yoki admin rol berganda) ishlatiladi.
	UpdateRole(ctx context.Context, id string, role Role, entityID string) error
	ListByRole(ctx context.Context, role Role) ([]*User, error)
	// DeleteByRoleEntity — obyekt (restoran/kuryer) o'chirilganda unga
	// bog'langan akkauntlarni ham o'chirish uchun. O'CHIRILGAN
	// foydalanuvchi ID'larini qaytaradi — chaqiruvchi ularning
	// tokenlarini ham bekor qilishi SHART (`internal/revoke`), aks holda
	// akkaunt bazadan yo'q bo'lsa ham qo'lidagi token amal muddati
	// tugagunicha (30 kun) ishlashda davom etardi: `auth()` faqat
	// imzoni tekshiradi, bazaga qaramaydi.
	DeleteByRoleEntity(ctx context.Context, role Role, entityID string) ([]string, error)
	// UpdateAddress — mijoz xaritadan yetkazib berish manzilini tanlab
	// saqlaganda ("Tayyor" tugmasi) ishlatiladi.
	UpdateAddress(ctx context.Context, id string, addr AddressDetails) error
	// GetByEmail — parol bilan kirishda (email varianti). Topilmasa
	// ErrUserNotFound. Qidiruv REGISTRGA BOG'LIQ EMAS (lower(email)).
	GetByEmail(ctx context.Context, email string) (*User, error)
	// UpdateProfile — ism/familiya/email va parol hash'ini yangilaydi.
	// Bo'sh `passwordHash` — parolni O'ZGARTIRMASLIK degani (chaqiruvchi
	// faqat ismni yangilayotgan bo'lishi mumkin).
	UpdateProfile(ctx context.Context, id string, p ProfileUpdate) error
	// MarkPhoneVerified — SMS kod tasdiqlangandan keyin.
	MarkPhoneVerified(ctx context.Context, id string) error
	// MarkEmailVerified — email kodi tasdiqlangandan keyin.
	MarkEmailVerified(ctx context.Context, id string) error
	// SetPasswordHash — FAQAT parol hash'ini yozadi (bo'sh satr =
	// parolni olib tashlash) va BOSHQA HECH NARSAGA TEGMAYDI.
	//
	// NEGA ALOHIDA METOD: buni `UpdateProfile(PasswordHash: &h)` bilan
	// qilish mumkin ko'rinadi, LEKIN `UpdateProfile` yon ta'sir
	// sifatida `name` ustunini `first_name`/`last_name` dan QAYTA
	// HISOBLAYDI. Admin yaratgan restoran/kuryer akkauntlarida `name`
	// to'ldirilgan, `first_name`/`last_name` esa bo'sh — natijada
	// parolga tegadigan HAR QANDAY amal (parol o'rnatish, tasdiqlashda
	// tozalash, eski hash'ni kuchaytirish) ularning nomini bo'sh
	// satrga aylantirib yuborardi (bazada tekshirilgan).
	SetPasswordHash(ctx context.Context, id, hash string) error

	// GetByTelegramID — Telegram Mini App kirishi uchun (migration 0031).
	// Topilmasa ErrUserNotFound.
	GetByTelegramID(ctx context.Context, telegramID int64) (*User, error)

	// LinkTelegram — telegram_id ni foydalanuvchiga bog'laydi.
	//
	// ┌─ EGALIKNI KO'CHIRISH ─────────────────────────────────────────┐
	// Bir xil Telegram akkaunti oldin BOSHQA foydalanuvchiga
	// bog'langan bo'lsa (masalan odam raqamini o'zgartirdi yoki
	// telefon boshqa egaga o'tdi), bog'lanish YANGI egasiga o'tishi
	// kerak. Aks holda `UNIQUE` indeks tufayli yozuv umuman
	// saqlanmasdi va Mini App eski akkauntni ochishda davom etardi —
	// ya'ni odam boshqa birovning hisobiga tushardi.
	//
	// Shu sabab implementatsiya eski bog'lanishni AVVAL uzadi.
	// └───────────────────────────────────────────────────────────────┘
	LinkTelegram(ctx context.Context, userID string, telegramID int64) error
}

// ProfileUpdate — UpdateProfile uchun maydonlar. Ko'rsatkich (pointer)
// bo'lganlari faqat nil BO'LMAGANDA yoziladi, ya'ni "tegilmasin" va
// "bo'sh qilinsin" bir-biridan farqlanadi.
type ProfileUpdate struct {
	FirstName    *string
	LastName     *string
	Email        *string
	PasswordHash *string
}

// Code — yuborilgan tasdiqlash kodi. Kod ochiq saqlanmaydi, faqat hash.
type Code struct {
	// Target — kod QAYERGA yuborilgani: telefon raqami (`+998...`)
	// YOKI email manzili. Ikkalasi bitta do'konda saqlanadi va
	// to'qnashmaydi, chunki formatlari hech qachon bir xil bo'lmaydi.
	// (Avval bu maydon `Phone` deb atalardi — email kodlar
	// qo'shilgach nom chalg'ituvchi bo'lib qoldi.)
	Target    string
	CodeHash  string
	ExpiresAt time.Time
	CreatedAt time.Time
	Attempts  int
}

type CodeStore interface {
	Save(ctx context.Context, c *Code) error // bor bo'lsa ustidan yozadi
	Get(ctx context.Context, target string) (*Code, error)
	// IncrementAttempts — urinishlar sonini ATOMIK oshiradi va YANGI
	// qiymatni qaytaradi.
	//
	// MUHIM: qaytariladigan qiymat shart. Avval oqim "o'qi → tekshir →
	// oshir" ko'rinishida edi va bu TOCTOU yaratardi: bir vaqtda kelgan
	// yuzlab `verify` so'rovi hammasi `Attempts=0` ni o'qib, hammasi
	// tekshiruvdan o'tardi — ya'ni "5 urinish" chegarasi amalda ishlamay,
	// 6 xonali kodni brute-force qilish mumkin bo'lardi. Endi chegara
	// oshirilgandan KEYIN qaytgan qiymat bo'yicha tekshiriladi.
	IncrementAttempts(ctx context.Context, target string) (int, error)
	Delete(ctx context.Context, target string) error
}

// SmsSender — SMS yuborish qatlami. Dev'da log, production'da Eskiz.uz bo'ladi.
type SmsSender interface {
	Send(phone, text string) error
}

// EmailSender — email yuborish qatlami. Dev'da log, production'da SMTP.
//
// XAVFSIZLIK: implementatsiya `to` manzilida CR/LF belgilarini RAD
// ETISHI shart — aks holda hujumchi manzil maydoniga sarlavha
// qo'shib, xatni boshqa odamlarga ham yuborishi mumkin (SMTP header
// injection). Qarang: `internal/notify/email.go`.
// `htmlBody` bo'sh bo'lsa faqat matnli xat ketadi. Bo'sh bo'lmasa —
// `multipart/alternative`: HTML va matn BIRGA. Faqat HTML yuborish
// spam balini oshiradi va HTML'ni o'chirgan mijozlarda xat bo'sh
// ko'rinadi.
type EmailSender interface {
	Send(to, subject, textBody, htmlBody string) error
}
