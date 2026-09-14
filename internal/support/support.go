// Package support — OnDex qo'llab-quvvatlash: platforma ALOQA ma'lumotlari
// (telefon, Telegram, pochta) va restoran ↔ OnDex administratori CHATI.
//
// ┌─ QOIDALAR ────────────────────────────────────────────────────────┐
//  1. Aloqa ma'lumotlarini FAQAT admin o'zgartiradi; ular barcha
//     panellarda ko'rinadi. Qiymatlar saqlashdan OLDIN qat'iy
//     normallashtiriladi — panel ularni `tel:`, `mailto:` va
//     `https://t.me/` havolasiga aylantiradi, ya'ni tekshirilmagan
//     matn havola in'ektsiyasiga yo'l ochardi.
//  2. Restoran FAQAT o'z suhbatini ko'radi (`EntityID` tokendan).
//     Affitsiant va mijoz chatga kira olmaydi.
//  3. Har xabar AVVAL bazaga yoziladi, keyin jonli kanalga yuboriladi.
//     `seq` o'suvchi: soket uzilsa panel "shu raqamdan keyingilari" ni
//     so'raydi va hech narsa tushib qolmaydi.
//  4. `client_id` — takroriy yuborish (tarmoq uzilib qayta urinish)
//     ikkinchi xabar yaratmaydi.
//  5. Rasm serverda QAYTA KODLANADI va ommaviy bucket'ga emas, bazaga
//     yoziladi; faqat suhbat egasi va admin token bilan oladi.
//
// └───────────────────────────────────────────────────────────────────┘
package support

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"chustapp/internal/users"
)

// Side — suhbatning qaysi tomoni.
type Side string

const (
	SideRestaurant Side = "restaurant"
	SideAdmin      Side = "admin"
)

func (s Side) Valid() bool { return s == SideRestaurant || s == SideAdmin }

const (
	MaxBodyLen  = 2000
	MaxNoteLen  = 60
	MaxEmailLen = 254
	MaxLimit    = 100
	MaxThreads  = 500
	// MaxUploadBytes — yuborilgan XOM fayl (telefon surati, ekran rasmi).
	MaxUploadBytes = 10 << 20
	// MaxAttachmentBytes — serverda qayta kodlangan WebP chegarasi.
	MaxAttachmentBytes    = 3 << 20
	AttachmentContentType = "image/webp"
	previewLen            = 140
	AdminName             = "OnDex qo'llab-quvvatlash"
	defaultLimit          = 50
	maxClientID           = 64
	minClientID           = 8
	maxTelegramID         = 32
)

// ErrAttachmentNotFound — rasm yo'q yoki BOSHQA restoran suhbatiga tegishli
// (ikkalasi ham bir xil javob: begona ID mavjudligi oshkor qilinmaydi).
var ErrAttachmentNotFound = errors.New("rasm topilmadi")

// ── Aloqa ma'lumotlari ────────────────────────────────────────────────

type Contacts struct {
	Phone      string
	PhoneHours string
	Telegram   string // "@" siz foydalanuvchi nomi
	Email      string
	EmailNote  string
	UpdatedAt  time.Time
	UpdatedBy  string
}

// ContactsInput — admin yuborgan xom qiymatlar.
type ContactsInput struct {
	Phone      string `json:"phone"`
	PhoneHours string `json:"phone_hours"`
	Telegram   string `json:"telegram"`
	Email      string `json:"email"`
	EmailNote  string `json:"email_note"`
}

// ContactsView — HTTP javobi. `UpdatedBy` (admin ID'si) ATAYLAB chiqmaydi.
type ContactsView struct {
	Phone       string     `json:"phone"`
	PhoneHours  string     `json:"phone_hours"`
	Telegram    string     `json:"telegram"`
	TelegramURL string     `json:"telegram_url"`
	Email       string     `json:"email"`
	EmailNote   string     `json:"email_note"`
	Configured  bool       `json:"configured"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

func ToContactsView(c Contacts) ContactsView {
	v := ContactsView{Phone: c.Phone, PhoneHours: c.PhoneHours, Telegram: c.Telegram,
		Email: c.Email, EmailNote: c.EmailNote}
	if c.Telegram != "" {
		v.TelegramURL = "https://t.me/" + c.Telegram
	}
	v.Configured = c.Phone != "" || c.Telegram != "" || c.Email != ""
	if !c.UpdatedAt.IsZero() {
		t := c.UpdatedAt
		v.UpdatedAt = &t
	}
	return v
}

// NormalizeContacts — har maydon ixtiyoriy (bo'sh — ko'rsatilmaydi), lekin
// to'ldirilgan bo'lsa QAT'IY tekshiriladi va kesib tashlanmaydi.
func NormalizeContacts(in ContactsInput) (Contacts, error) {
	var c Contacts
	var err error
	if p := strings.TrimSpace(in.Phone); p != "" {
		if c.Phone, err = users.NormalizePhone(p); err != nil {
			return Contacts{}, invalid("Telefon raqam noto'g'ri (masalan: +998 90 123 45 67)")
		}
	}
	if c.PhoneHours, err = normalizeNote(in.PhoneHours, "Ish vaqti"); err != nil {
		return Contacts{}, err
	}
	if c.Telegram, err = NormalizeTelegram(in.Telegram); err != nil {
		return Contacts{}, err
	}
	if c.Email, err = NormalizeEmail(in.Email); err != nil {
		return Contacts{}, err
	}
	if c.EmailNote, err = normalizeNote(in.EmailNote, "Pochta izohi"); err != nil {
		return Contacts{}, err
	}
	return c, nil
}

// telegramRe — Telegram qoidasi: 5-32 belgi, lotin harfi bilan boshlanadi,
// `_` bilan tugamaydi.
var telegramRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{3,30}[A-Za-z0-9]$`)

// NormalizeTelegram — "@ondex_support", "t.me/ondex_support" yoki
// "https://t.me/ondex_support" → "ondex_support".
func NormalizeTelegram(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	lower := strings.ToLower(s)
	for _, p := range []string{"https://t.me/", "http://t.me/", "https://telegram.me/", "http://telegram.me/",
		"t.me/", "telegram.me/", "@"} {
		if strings.HasPrefix(lower, p) {
			s = s[len(p):]
			break
		}
	}
	s = strings.TrimSuffix(s, "/")
	if len(s) > maxTelegramID || !telegramRe.MatchString(s) || strings.Contains(s, "__") {
		return "", invalid("Telegram foydalanuvchi nomi noto'g'ri (masalan: @ondex_support)")
	}
	return s, nil
}

// emailRe — faqat ASCII: unicode domen (gomograf) va bo'sh joy, qo'shtirnoq,
// `<>` kabi havolani buzadigan belgilar o'tmaydi.
var emailRe = regexp.MustCompile(`^[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,24}$`)

func NormalizeEmail(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	bad := invalid("Elektron pochta manzili noto'g'ri (masalan: support@ondex.uz)")
	if len(s) > MaxEmailLen || !emailRe.MatchString(s) || strings.Contains(s, "..") {
		return "", bad
	}
	a, err := mail.ParseAddress(s)
	if err != nil || a.Name != "" || !strings.EqualFold(a.Address, s) {
		return "", bad
	}
	return strings.ToLower(s), nil
}

func normalizeNote(raw, field string) (string, error) {
	s := strings.TrimSpace(raw)
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", invalid(field + "da ko'rinmas boshqaruv belgilari bo'lishi mumkin emas")
		}
	}
	if utf8.RuneCountInString(s) > MaxNoteLen {
		return "", invalid(field + " 60 belgidan oshmasligi kerak")
	}
	return s, nil
}

// ── Chat ──────────────────────────────────────────────────────────────

// AttachmentMeta — xabarga biriktirilgan rasm (baytlarsiz).
type AttachmentMeta struct {
	ID          string
	ContentType string
	Width       int
	Height      int
	Size        int
}

// Attachment — baytlari bilan; faqat alohida, egalik tekshiriladigan
// endpoint beradi (ro'yxat va jonli kanalga baytlar tushmaydi).
type Attachment struct {
	AttachmentMeta
	RestaurantID string
	Data         []byte
	CreatedAt    time.Time
}

// ImageInput — handler QAYTA KODLAGAN rasm (`images.ProcessChatImage`).
type ImageInput struct {
	Data   []byte
	Width  int
	Height int
}

type AttachmentView struct {
	ID          string `json:"id"`
	ContentType string `json:"content_type"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Size        int    `json:"size"`
}

type Message struct {
	Seq          int64
	ID           string
	RestaurantID string
	Sender       Side
	SenderID     string
	SenderName   string
	Body         string
	ClientID     string
	Attachment   *AttachmentMeta
	CreatedAt    time.Time
}

// MessageView — HTTP va WebSocket javobi. `SenderID` (foydalanuvchi ID'si)
// chiqmaydi: restoranga admin akkaunti, adminga xodim akkaunti kerak emas.
type MessageView struct {
	ID         string          `json:"id"`
	Seq        int64           `json:"seq"`
	Sender     Side            `json:"sender"`
	SenderName string          `json:"sender_name"`
	Body       string          `json:"body"`
	ClientID   string          `json:"client_id"`
	Attachment *AttachmentView `json:"attachment,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

func ToMessageView(m *Message) MessageView {
	v := MessageView{ID: m.ID, Seq: m.Seq, Sender: m.Sender, SenderName: m.SenderName,
		Body: m.Body, ClientID: m.ClientID, CreatedAt: m.CreatedAt}
	if a := m.Attachment; a != nil {
		v.Attachment = &AttachmentView{ID: a.ID, ContentType: a.ContentType, Width: a.Width, Height: a.Height, Size: a.Size}
	}
	return v
}

// Thread — bitta restoranning suhbati (xulosa).
type Thread struct {
	RestaurantID      string
	MessageCount      int
	FirstAt           time.Time
	LastAt            time.Time
	LastSeq           int64
	LastSender        Side
	LastBody          string
	LastHasImage      bool
	RestaurantReadSeq int64
	AdminReadSeq      int64
	UnreadRestaurant  int
	UnreadAdmin       int
}

// ThreadView — KO'RUVCHI tomoniga moslangan xulosa: `unread` — shu
// tomonning o'qilmaganlari, `peer_read_seq` — qarshi tomon qayergacha
// o'qigani (xabar ostidagi "o'qildi" belgisi uchun).
type ThreadView struct {
	RestaurantID string     `json:"restaurant_id"`
	MessageCount int        `json:"message_count"`
	FirstAt      *time.Time `json:"first_at"`
	LastAt       *time.Time `json:"last_at"`
	LastSeq      int64      `json:"last_seq"`
	LastSender   Side       `json:"last_sender"`
	LastBody     string     `json:"last_body"`
	LastHasImage bool       `json:"last_has_image"`
	ReadSeq      int64      `json:"read_seq"`
	PeerReadSeq  int64      `json:"peer_read_seq"`
	Unread       int        `json:"unread"`
}

func ToThreadView(t *Thread, viewer Side) ThreadView {
	v := ThreadView{RestaurantID: t.RestaurantID, MessageCount: t.MessageCount, LastSeq: t.LastSeq,
		LastSender: t.LastSender, LastBody: preview(t.LastBody), LastHasImage: t.LastHasImage}
	if v.LastBody == "" && t.LastHasImage {
		v.LastBody = "Rasm"
	}
	if !t.FirstAt.IsZero() {
		f, l := t.FirstAt, t.LastAt
		v.FirstAt, v.LastAt = &f, &l
	}
	if viewer == SideAdmin {
		v.ReadSeq, v.PeerReadSeq, v.Unread = t.AdminReadSeq, t.RestaurantReadSeq, t.UnreadAdmin
	} else {
		v.ReadSeq, v.PeerReadSeq, v.Unread = t.RestaurantReadSeq, t.AdminReadSeq, t.UnreadRestaurant
	}
	return v
}

func preview(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) > previewLen {
		return string([]rune(s)[:previewLen-1]) + "…"
	}
	return s
}

type MessageQuery struct {
	// BeforeSeq — eskiroq sahifa; AfterSeq — qayta ulanganda o'tkazib
	// yuborilganlar. Ikkalasi ham nol — eng yangilari.
	BeforeSeq int64
	AfterSeq  int64
	Limit     int
}

var idRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func ValidClientID(s string) bool {
	return len(s) >= minClientID && len(s) <= maxClientID && idRe.MatchString(s)
}

func ValidAttachmentID(s string) bool {
	return len(s) >= 1 && len(s) <= 64 && idRe.MatchString(s)
}

// NormalizeBody — chat matni (bo'sh bo'lmasligi kerak).
func NormalizeBody(raw string) (string, error) { return normalizeText(raw, false) }

// NormalizeCaption — rasm ostidagi izoh (bo'sh bo'lishi mumkin).
func NormalizeCaption(raw string) (string, error) { return normalizeText(raw, true) }

// normalizeText — olib tashlanadi: boshqaruv belgilari (qator ko'chirish va
// tabdan tashqari) va ko'rinmas formatlash belgilari — ayniqsa yo'nalish
// almashtiruvchilar (U+202E): ular bilan "fayl.exe" ni "exe.lif" qilib
// ko'rsatib aldash mumkin. ZWJ/ZWNJ QOLADI — ularsiz murakkab emoji
// (oila, kasb) bo'laklarga bo'linib ketardi.
func normalizeText(raw string, allowEmpty bool) (string, error) {
	raw = strings.ReplaceAll(strings.ReplaceAll(raw, "\r\n", "\n"), "\r", "\n")
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r == '\n':
			b.WriteRune(r)
		case r == '\t':
			b.WriteRune(' ')
		case r == 0x200c || r == 0x200d:
			b.WriteRune(r)
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r), r == utf8.RuneError:
			// tashlanadi
		default:
			b.WriteRune(r)
		}
	}
	s := strings.TrimSpace(b.String())
	if s == "" && !allowEmpty {
		return "", invalid("Xabar bo'sh bo'lishi mumkin emas")
	}
	if utf8.RuneCountInString(s) > MaxBodyLen {
		return "", invalid("Xabar 2000 belgidan oshmasligi kerak")
	}
	return s, nil
}

// Store — Postgres (`storage.PgSupportStore`) yoki xotira.
type Store interface {
	// GetContacts — hali saqlanmagan bo'lsa (nil, nil).
	GetContacts(ctx context.Context) (*Contacts, error)
	SaveContacts(ctx context.Context, c *Contacts) error

	// InsertMessage — `Seq` ni beradi, rasmni (bo'lsa) va suhbat xulosasini
	// BITTA tranzaksiyada yozadi (yuboruvchi tomonning o'qish belgisi shu
	// xabargacha suriladi: javob yozgan odam avvalgilarni ko'rgan). Shu
	// restoranda shu yuboruvchidan shu `ClientID` allaqachon bo'lsa hech
	// narsa yozmaydi (yangi rasm ham), `m` ni mavjud xabar bilan to'ldiradi
	// va `false` qaytaradi.
	InsertMessage(ctx context.Context, m *Message, att *Attachment) (bool, error)
	// ListMessages — `AfterSeq` bo'lsa o'sish, aks holda kamayish tartibida.
	// Rasm baytlari QAYTMAYDI, faqat metama'lumot.
	ListMessages(ctx context.Context, restaurantID string, q MessageQuery) ([]*Message, error)
	// GetAttachment — faqat SHU restoran suhbatidagi rasm; aks holda
	// `ErrAttachmentNotFound`.
	GetAttachment(ctx context.Context, restaurantID, id string) (*Attachment, error)
	// GetThread — suhbat hali yo'q bo'lsa bo'sh (`MessageCount == 0`) xulosa.
	GetThread(ctx context.Context, restaurantID string) (*Thread, error)
	// ListThreads — oxirgi xabar bo'yicha eng yangisidan.
	ListThreads(ctx context.Context, limit int) ([]*Thread, error)
	// MarkRead — `side` ning o'qish belgisini `upToSeq` gacha (suhbatdagi
	// oxirgi xabardan oshmasdan) FAQAT OLDINGA suradi; o'zgargan bo'lsa true.
	MarkRead(ctx context.Context, restaurantID string, side Side, upToSeq int64) (bool, error)
	// AdminUnreadTotal — barcha suhbatlardagi admin o'qimagan xabarlar.
	AdminUnreadTotal(ctx context.Context) (int, error)
}

// ValidationError — foydalanuvchi kiritmasidagi xato (400).
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return e.Msg }

func invalid(msg string) error { return ValidationError{Msg: msg} }

func IsValidation(err error) bool {
	var ve ValidationError
	return errors.As(err, &ve)
}
