package staff

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ValidationError — foydalanuvchi kiritmasidagi xato; matnini mijozga
// ko'rsatish xavfsiz.
type ValidationError struct{ Msg string }

func (e ValidationError) Error() string { return e.Msg }

func invalid(msg string) error { return ValidationError{Msg: msg} }

const (
	MaxMembersPerRestaurant = 500
	MaxNameLen              = 50
	MaxNoteLen              = 300
	// MaxSalaryTiyin — oylik maosh chegarasi (1 mlrd so'm): xato kiritilgan
	// qo'shimcha nollar hisobotni buzmasin.
	MaxSalaryTiyin int64 = 100_000_000_000
)

var (
	ErrNotFound         = errors.New("xodim topilmadi")
	ErrPhoneTaken       = errors.New("bu telefon raqamli xodim allaqachon ro'yxatda bor")
	ErrLimitReached     = fmt.Errorf("bitta restoranda %d tadan ortiq xodim bo'lishi mumkin emas", MaxMembersPerRestaurant)
	ErrAccountConflict  = errors.New("bu telefon raqam boshqa OnDex akkauntiga biriktirilgan — ilovaga kirishni ochib bo'lmaydi")
	ErrAccountsDisabled = errors.New("ilova akkauntlari xizmati ulanmagan")
	// ErrCourierBusy — yetkazma o'rtasida kuryerning kirishini yopish
	// buyurtmani egasiz qoldirardi: "yetkazdim" deb bosadigan odam qolmaydi.
	ErrCourierBusy = errors.New("bu yetkazib beruvchida yakunlanmagan buyurtma bor — avval u yetkazib berilsin, keyin o'zgartiring")

	ErrFirstNameRequired = invalid("xodimning ismini kiriting")
	ErrNameTooLong       = invalid(fmt.Sprintf("ism va familiya %d belgidan oshmasligi kerak", MaxNameLen))
	ErrNameChars         = invalid("ism va familiyada faqat harflar, bo'sh joy, apostrof va chiziqcha bo'lishi mumkin")
	ErrUnknownPosition   = invalid("lavozim noto'g'ri")
	ErrUnknownStatus     = invalid("holat noto'g'ri")
	ErrBadSchedule       = invalid("ish jadvali noto'g'ri: kamida bitta kun (1–7) va vaqt SS:DD shaklida")
	ErrBadHiredOn        = invalid("ishga kirgan sana noto'g'ri (YYYY-MM-DD)")
	ErrNoteTooLong       = invalid(fmt.Sprintf("izoh %d belgidan oshmasligi kerak", MaxNoteLen))
	ErrNoteChars         = invalid("izohda ko'rinmas boshqaruv belgilari bo'lishi mumkin emas")
	ErrAccessNotAllowed  = invalid("bu lavozim uchun OnDex ilovasi yo'q — ilovaga kirishni faqat ofitsiant va yetkazib beruvchiga ochish mumkin")
	ErrBadSalary         = invalid("oylik maosh 0 dan katta va 1 mlrd so'mdan oshmasligi kerak")
)

func validSalary(v *int64) (*int64, error) {
	if v == nil {
		return nil, nil
	}
	if *v <= 0 || *v > MaxSalaryTiyin {
		return nil, ErrBadSalary
	}
	x := *v
	return &x, nil
}

func sameSalary(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// IsValidation — xato foydalanuvchi kiritmasidanmi (400).
func IsValidation(err error) bool {
	var ve ValidationError
	return errors.As(err, &ve)
}

// Location — ish jadvali Toshkent vaqtida (UTC+5, yozgi vaqt yo'q).
var Location = time.FixedZone("Asia/Tashkent", 5*60*60)

// ─── Holat ─────────────────────────────────────────────────────────────

type Status string

const (
	StatusActive    Status = "active"
	StatusOnLeave   Status = "on_leave"
	StatusDismissed Status = "dismissed"
)

func ParseStatus(s string) (Status, error) {
	switch st := Status(strings.ToLower(strings.TrimSpace(s))); st {
	case StatusActive, StatusOnLeave, StatusDismissed:
		return st, nil
	}
	return "", ErrUnknownStatus
}

func (s Status) Title() string {
	switch s {
	case StatusActive:
		return "Faol"
	case StatusOnLeave:
		return "Ta'tilda"
	case StatusDismissed:
		return "Ishdan bo'shagan"
	}
	return string(s)
}

// ─── Ish jadvali ───────────────────────────────────────────────────────

// Schedule — haftalik smena: ish kunlari (1 — dushanba ... 7 — yakshanba)
// va vaqt. `End` < `Start` — smena ertasi kuni tugaydi (18:00–02:00).
type Schedule struct {
	Days  []int  `json:"days"`
	Start string `json:"start"`
	End   string `json:"end"`
}

func validClock(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h, err1 := strconv.Atoi(s[:2])
	m, err2 := strconv.Atoi(s[3:])
	return err1 == nil && err2 == nil && s[0] != '+' && s[3] != '+' && h >= 0 && h <= 23 && m >= 0 && m <= 59
}

// Normalize — kunlar takrorlanmas va tartiblangan, vaqt qat'iy SS:DD.
func (s Schedule) Normalize() (Schedule, error) {
	if len(s.Days) == 0 || len(s.Days) > 7 || !validClock(s.Start) || !validClock(s.End) {
		return Schedule{}, ErrBadSchedule
	}
	days := slices.Clone(s.Days)
	slices.Sort(days)
	for i, d := range days {
		if d < 1 || d > 7 || (i > 0 && days[i-1] == d) {
			return Schedule{}, ErrBadSchedule
		}
	}
	return Schedule{Days: days, Start: s.Start, End: s.End}, nil
}

// WorksOn — shu hafta kuni (1..7) smena BOSHLANADIMI.
func (s *Schedule) WorksOn(day int) bool {
	return s != nil && slices.Contains(s.Days, day)
}

func (s *Schedule) key() string {
	if s == nil {
		return ""
	}
	parts := make([]string, len(s.Days))
	for i, d := range s.Days {
		parts[i] = strconv.Itoa(d)
	}
	return strings.Join(parts, ",") + " " + s.Start + "-" + s.End
}

// isoWeekday — 1 dushanba ... 7 yakshanba.
func isoWeekday(t time.Time) int {
	if d := int(t.Weekday()); d != 0 {
		return d
	}
	return 7
}

// ─── Xodim ─────────────────────────────────────────────────────────────

type Member struct {
	ID           string
	RestaurantID string
	// Number — restoran ichidagi tartib raqami (#EMP001). Bir marta
	// beriladi va hech qachon qayta ishlatilmaydi.
	Number    int
	FirstName string
	LastName  string
	// Phone — E.164 (+998XXXXXXXXX).
	Phone    string
	Position Position
	Status   Status
	Schedule *Schedule
	// HiredOn — faqat sana (UTC yarim tun).
	HiredOn *time.Time
	Note    string
	// SalaryTiyin — oylik maosh (tiyin), ixtiyoriy. Maxfiy: logga va
	// faoliyat jurnaliga QIYMATI yozilmaydi.
	SalaryTiyin *int64
	// AppAccess — restoranning TANLOVI ("ilovaga kirish ochiq bo'lsin").
	// Haqiqiy kirish `AccessEffective` — faol va ilovali lavozimda.
	AppAccess bool
	// UserID — bog'langan ilova akkaunti. HTTP javobiga CHIQMAYDI.
	UserID      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DismissedAt *time.Time
}

func (m *Member) FullName() string {
	return strings.TrimSpace(m.FirstName + " " + m.LastName)
}

// Code — "EMP001".
func (m *Member) Code() string { return fmt.Sprintf("EMP%03d", m.Number) }

// AccessEffective — ilovaga kirish HOZIR ochiq bo'lishi kerakmi.
func (m *Member) AccessEffective() bool {
	return m.AppAccess && m.Status == StatusActive && m.Position.AllowsAppAccess()
}

func cloneMember(m Member) Member {
	if m.Schedule != nil {
		s := *m.Schedule
		s.Days = slices.Clone(m.Schedule.Days)
		m.Schedule = &s
	}
	if m.HiredOn != nil {
		v := *m.HiredOn
		m.HiredOn = &v
	}
	if m.DismissedAt != nil {
		v := *m.DismissedAt
		m.DismissedAt = &v
	}
	if m.SalaryTiyin != nil {
		v := *m.SalaryTiyin
		m.SalaryTiyin = &v
	}
	return m
}

// Clone — chuqur nusxa (omborlar uchun).
func Clone(m Member) Member { return cloneMember(m) }

// ─── Hodisalar (faoliyat jurnali) ──────────────────────────────────────

type EventKind string

const (
	EventCreated         EventKind = "created"
	EventUpdated         EventKind = "updated"
	EventPositionChanged EventKind = "position_changed"
	EventScheduleChanged EventKind = "schedule_changed"
	EventStatusChanged   EventKind = "status_changed"
	EventAccessGranted   EventKind = "access_granted"
	EventAccessRevoked   EventKind = "access_revoked"
	// EventSalaryChanged — maosh o'zgargani; qiymatlar YOZILMAYDI.
	EventSalaryChanged EventKind = "salary_changed"
)

// Event — kim, qachon, nimani o'zgartirdi. Shaxsiy ma'lumot (telefon)
// YOZILMAYDI: `From`/`To` faqat lavozim/holat kaliti yoki jadval.
type Event struct {
	ID           string
	RestaurantID string
	MemberID     string
	Kind         EventKind
	From         string
	To           string
	ActorID      string
	At           time.Time
}

type Repository interface {
	// Create — `m.Number` ni o'zi beradi (restoran ichida ketma-ket) va
	// xodim + hodisalarni BITTA tranzaksiyada yozadi.
	Create(ctx context.Context, m *Member, events []Event) error
	// Update — xodim va hodisalar bitta tranzaksiyada. `number`,
	// `restaurant_id`, `created_at` ga TEGMAYDI.
	Update(ctx context.Context, m *Member, events []Event) error
	Get(ctx context.Context, restaurantID, id string) (*Member, error)
	List(ctx context.Context, restaurantID string) ([]*Member, error)
	// Events — eng yangisi birinchi. `memberID` bo'sh — butun restoran.
	Events(ctx context.Context, restaurantID, memberID string, since time.Time, limit int) ([]Event, error)
}

// Accounts — ilova akkauntini ochish/yopish (`UserAccounts`).
type Accounts interface {
	// Enable — xodim uchun ilovaga kirishni ochadi, akkaunt ID'sini qaytaradi.
	Enable(ctx context.Context, m *Member) (string, error)
	// Disable — akkauntni restorandan uzadi va sessiyalarini bekor qiladi.
	Disable(ctx context.Context, restaurantID, userID string) error
	// Refresh — bog'langan akkaunt ma'lumotini (ism) yozuvga moslaydi.
	Refresh(ctx context.Context, m *Member) error
}

// ─── Kiritmani tekshirish ──────────────────────────────────────────────

func normalizeName(s string, required bool) (string, error) {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		if required {
			return "", ErrFirstNameRequired
		}
		return "", nil
	}
	if utf8.RuneCountInString(s) > MaxNameLen {
		return "", ErrNameTooLong
	}
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.Is(unicode.Mn, r), r == ' ', r == '-', r == '.',
			r == '\'', r == '`', r == 'ʻ', r == 'ʼ', r == '’', r == '‘':
		default:
			return "", ErrNameChars
		}
	}
	return s, nil
}

func normalizeNote(s string) (string, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	for _, r := range s {
		if r == '\n' {
			continue
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", ErrNoteChars
		}
	}
	if utf8.RuneCountInString(s) > MaxNoteLen {
		return "", ErrNoteTooLong
	}
	return s, nil
}

func parseHiredOn(s string, now time.Time) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, ErrBadHiredOn
	}
	if d.Year() < 1950 || d.After(now.AddDate(1, 0, 0)) {
		return nil, ErrBadHiredOn
	}
	return &d, nil
}
