package staff

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"chustapp/internal/users"
)

type Service struct {
	repo     Repository
	accounts Accounts
	now      func() time.Time
	// observer — saqlangan hodisalar (restoran bildirishnomalari uchun).
	observer func(m Member, events []Event)
}

// WithObserver — yozuv SAQLANGANDAN keyin hodisalarni oladi. Kuzatuvchi
// uzoq ish qilmasligi kerak (o'z goroutine'ida bajaradi).
func (s *Service) WithObserver(fn func(m Member, events []Event)) *Service {
	s.observer = fn
	return s
}

func (s *Service) emit(m *Member, events []Event) {
	if s.observer == nil || m == nil || len(events) == 0 {
		return
	}
	s.observer(Clone(*m), append([]Event(nil), events...))
}

// NewService — `accounts` nil bo'lsa ilovaga kirishni ochib bo'lmaydi
// (qolgan hamma narsa ishlaydi).
func NewService(repo Repository, accounts Accounts) *Service {
	return &Service{repo: repo, accounts: accounts, now: time.Now}
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("ID yaratib bo'lmadi: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Input — yangi xodim.
type Input struct {
	FirstName string
	LastName  string
	Phone     string
	Position  string
	Schedule  *Schedule
	HiredOn   string
	Note      string
	AppAccess bool
	// SalaryTiyin — oylik maosh, ixtiyoriy (nil — kiritilmagan).
	SalaryTiyin *int64
}

// Patch — tahrirlash; nil maydon o'zgarmaydi.
type Patch struct {
	FirstName *string
	LastName  *string
	Phone     *string
	Position  *string
	// Schedule — yangi jadval; ClearSchedule — jadvalni olib tashlash.
	Schedule      *Schedule
	ClearSchedule bool
	// HiredOn — "" sanani olib tashlaydi.
	HiredOn   *string
	Note      *string
	AppAccess *bool
	// SalaryTiyin — yangi maosh; ClearSalary — maoshni olib tashlash.
	SalaryTiyin *int64
	ClearSalary bool
}

func (s *Service) newEvent(m *Member, actorID string, kind EventKind, from, to string) (Event, error) {
	id, err := newID()
	if err != nil {
		return Event{}, err
	}
	return Event{ID: id, RestaurantID: m.RestaurantID, MemberID: m.ID, Kind: kind,
		From: from, To: to, ActorID: actorID, At: s.now()}, nil
}

// phoneFree — telefon restoranning boshqa (ishdan bo'shamagan) xodimida yo'q.
func (s *Service) phoneFree(ctx context.Context, m *Member) error {
	if m.Status == StatusDismissed {
		return nil
	}
	list, err := s.repo.List(ctx, m.RestaurantID)
	if err != nil {
		return err
	}
	for _, x := range list {
		if x.ID != m.ID && x.Status != StatusDismissed && x.Phone == m.Phone {
			return ErrPhoneTaken
		}
	}
	return nil
}

// Create — yangi xodim (holati "faol").
func (s *Service) Create(ctx context.Context, restaurantID, actorID string, in Input) (*Member, error) {
	if strings.TrimSpace(restaurantID) == "" {
		return nil, errors.New("restaurant_id bo'sh")
	}
	first, err := normalizeName(in.FirstName, true)
	if err != nil {
		return nil, err
	}
	last, err := normalizeName(in.LastName, false)
	if err != nil {
		return nil, err
	}
	phone, err := users.NormalizePhone(in.Phone)
	if err != nil {
		return nil, invalid(err.Error())
	}
	pos, err := ParsePosition(in.Position)
	if err != nil {
		return nil, err
	}
	var sched *Schedule
	if in.Schedule != nil {
		v, err := in.Schedule.Normalize()
		if err != nil {
			return nil, err
		}
		sched = &v
	}
	now := s.now()
	hired, err := parseHiredOn(in.HiredOn, now)
	if err != nil {
		return nil, err
	}
	note, err := normalizeNote(in.Note)
	if err != nil {
		return nil, err
	}
	salary, err := validSalary(in.SalaryTiyin)
	if err != nil {
		return nil, err
	}
	if in.AppAccess && !pos.AllowsAppAccess() {
		return nil, ErrAccessNotAllowed
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	m := &Member{
		ID: id, RestaurantID: restaurantID, FirstName: first, LastName: last, Phone: phone,
		Position: pos, Status: StatusActive, Schedule: sched, HiredOn: hired, Note: note,
		AppAccess: in.AppAccess, SalaryTiyin: salary, CreatedAt: now, UpdatedAt: now,
	}

	list, err := s.repo.List(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	// Chegara "yumshoq" (parallel so'rovlar bittaga oshirishi mumkin) —
	// maqsad suiiste'molni to'xtatish.
	if len(list) >= MaxMembersPerRestaurant {
		return nil, ErrLimitReached
	}
	if err := s.phoneFree(ctx, m); err != nil {
		return nil, err
	}

	created, err := s.newEvent(m, actorID, EventCreated, "", string(pos))
	if err != nil {
		return nil, err
	}
	events := []Event{created}
	rollback, err := s.syncAccess(ctx, &Member{RestaurantID: restaurantID}, m, actorID, &events)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, m, events); err != nil {
		rollback()
		return nil, err
	}
	s.emit(m, events)
	return m, nil
}

// Update — ma'lumotlarni tahrirlash (holat alohida: `SetStatus`).
func (s *Service) Update(ctx context.Context, restaurantID, id, actorID string, p Patch) (*Member, error) {
	cur, err := s.repo.Get(ctx, restaurantID, id)
	if err != nil {
		return nil, err
	}
	old := cloneMember(*cur)
	m := cloneMember(*cur)
	var events []Event
	detailsChanged := false

	if p.FirstName != nil {
		v, err := normalizeName(*p.FirstName, true)
		if err != nil {
			return nil, err
		}
		detailsChanged = detailsChanged || v != m.FirstName
		m.FirstName = v
	}
	if p.LastName != nil {
		v, err := normalizeName(*p.LastName, false)
		if err != nil {
			return nil, err
		}
		detailsChanged = detailsChanged || v != m.LastName
		m.LastName = v
	}
	if p.Phone != nil {
		v, err := users.NormalizePhone(*p.Phone)
		if err != nil {
			return nil, invalid(err.Error())
		}
		detailsChanged = detailsChanged || v != m.Phone
		m.Phone = v
	}
	if p.Position != nil {
		v, err := ParsePosition(*p.Position)
		if err != nil {
			return nil, err
		}
		if v != m.Position {
			ev, err := s.newEvent(&m, actorID, EventPositionChanged, string(m.Position), string(v))
			if err != nil {
				return nil, err
			}
			events = append(events, ev)
			m.Position = v
			// Ilovasiz lavozimga o'tsa tanlov ham o'chadi: keyin qaytib
			// ofitsiant bo'lsa kirish JIMGINA qayta ochilib qolmasin.
			if !v.AllowsAppAccess() {
				m.AppAccess = false
			}
		}
	}
	if p.ClearSchedule || p.Schedule != nil {
		var next *Schedule
		if !p.ClearSchedule {
			v, err := p.Schedule.Normalize()
			if err != nil {
				return nil, err
			}
			next = &v
		}
		if next.key() != m.Schedule.key() {
			ev, err := s.newEvent(&m, actorID, EventScheduleChanged, m.Schedule.key(), next.key())
			if err != nil {
				return nil, err
			}
			events = append(events, ev)
			m.Schedule = next
		}
	}
	if p.HiredOn != nil {
		v, err := parseHiredOn(*p.HiredOn, s.now())
		if err != nil {
			return nil, err
		}
		if !sameDate(v, m.HiredOn) {
			detailsChanged = true
		}
		m.HiredOn = v
	}
	if p.Note != nil {
		v, err := normalizeNote(*p.Note)
		if err != nil {
			return nil, err
		}
		detailsChanged = detailsChanged || v != m.Note
		m.Note = v
	}
	if p.ClearSalary || p.SalaryTiyin != nil {
		var next *int64
		if !p.ClearSalary {
			v, err := validSalary(p.SalaryTiyin)
			if err != nil {
				return nil, err
			}
			next = v
		}
		if !sameSalary(next, m.SalaryTiyin) {
			ev, err := s.newEvent(&m, actorID, EventSalaryChanged, "", "")
			if err != nil {
				return nil, err
			}
			events = append(events, ev)
			m.SalaryTiyin = next
		}
	}
	if p.AppAccess != nil {
		if *p.AppAccess && !m.Position.AllowsAppAccess() {
			return nil, ErrAccessNotAllowed
		}
		m.AppAccess = *p.AppAccess
	}
	if detailsChanged {
		ev, err := s.newEvent(&m, actorID, EventUpdated, "", "")
		if err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	if m.Phone != old.Phone {
		if err := s.phoneFree(ctx, &m); err != nil {
			return nil, err
		}
	}
	return s.save(ctx, &old, &m, actorID, events)
}

// SetStatus — faol / ta'tilda / ishdan bo'shagan. Yozuv O'CHIRILMAYDI:
// mehnat va moliyaviy hisobot uchun tarix saqlanadi.
func (s *Service) SetStatus(ctx context.Context, restaurantID, id, actorID, status string) (*Member, error) {
	st, err := ParseStatus(status)
	if err != nil {
		return nil, err
	}
	cur, err := s.repo.Get(ctx, restaurantID, id)
	if err != nil {
		return nil, err
	}
	if cur.Status == st {
		return cur, nil
	}
	old := cloneMember(*cur)
	m := cloneMember(*cur)
	m.Status = st
	if st == StatusDismissed {
		now := s.now()
		m.DismissedAt = &now
	} else {
		m.DismissedAt = nil
		// Qayta ishga olinayotganda raqam boshqa faol xodimda bo'lishi mumkin.
		if old.Status == StatusDismissed {
			if err := s.phoneFree(ctx, &m); err != nil {
				return nil, err
			}
		}
	}
	ev, err := s.newEvent(&m, actorID, EventStatusChanged, string(old.Status), string(st))
	if err != nil {
		return nil, err
	}
	return s.save(ctx, &old, &m, actorID, []Event{ev})
}

func (s *Service) save(ctx context.Context, old, m *Member, actorID string, events []Event) (*Member, error) {
	rollback, err := s.syncAccess(ctx, old, m, actorID, &events)
	if err != nil {
		return nil, err
	}
	// Har bir maydon o'zgarishi hodisa yozadi; hodisasiz faqat kirish
	// tanlovi (masalan ta'tildagi xodimga) va akkaunt bog'lanishi o'zgaradi.
	if len(events) == 0 && old.AppAccess == m.AppAccess && old.UserID == m.UserID {
		return m, nil
	}
	m.UpdatedAt = s.now()
	if err := s.repo.Update(ctx, m, events); err != nil {
		rollback()
		return nil, err
	}
	// Ism o'zgarsa bog'langan akkaunt ham yangilanadi (kuryer ilovasi
	// profili, superadmin ro'yxati). Yozuv allaqachon saqlangan — bu
	// yerdagi xato amalni bekor qilmaydi, faqat logga tushadi.
	if s.accounts != nil && m.UserID != "" && m.AccessEffective() && old.FullName() != m.FullName() {
		if err := s.accounts.Refresh(ctx, m); err != nil {
			slog.Warn("xodim: akkaunt ma'lumotini yangilab bo'lmadi",
				"restaurant", m.RestaurantID, "staff", m.ID, "err", err)
		}
	}
	s.emit(m, events)
	return m, nil
}

// syncAccess — ilova akkaunti xodim yozuviga ERGASHADI.
//
// ┌─ TARTIB: AVVAL YOPISH, KEYIN SAQLASH ─────────────────────────────┐
// Kirish yopilishi kerak bo'lsa akkaunt SAQLASHDAN OLDIN yopiladi.
// Saqlash yiqilsa — akkaunt yopiq, yozuv esa eski holida qoladi: bu
// "xavfsiz tomonga" xato (fail closed). Teskari tartibda yozuv "ishdan
// bo'shagan" deb tursa-yu, ilova ishlab turishi mumkin edi.
//
// Kirish OCHILSA va saqlash yiqilsa, qaytariladigan `rollback` uni
// yana yopadi.
// └───────────────────────────────────────────────────────────────────┘
func (s *Service) syncAccess(ctx context.Context, old, m *Member, actorID string, events *[]Event) (func(), error) {
	noop := func() {}
	want := m.AccessEffective()
	had := old.UserID != "" && old.AccessEffective()
	phoneChanged := old.UserID != "" && old.Phone != m.Phone
	// Ilovali IKKI lavozim o'rtasidagi almashuv (ofitsiant ↔ yetkazib
	// beruvchi): akkaunt BOSHQA rolga o'tadi. Avval bu holat "o'zgarish
	// yo'q" deb o'tkazib yuborilardi — ilovali lavozim bitta bo'lganda
	// sezilmasdi, ikkinchisi qo'shilgach esa affitsiant akkaunti kuryer
	// yozuvida qolib ketardi.
	positionChanged := old.UserID != "" && old.Position != m.Position
	relink := phoneChanged || positionChanged

	if old.UserID != "" && (!want || relink) {
		if s.accounts != nil {
			if err := s.accounts.Disable(ctx, m.RestaurantID, old.UserID); err != nil {
				return noop, err
			}
		}
		if had {
			ev, err := s.newEvent(m, actorID, EventAccessRevoked, "", "")
			if err != nil {
				return noop, err
			}
			*events = append(*events, ev)
		}
		// Akkaunt ESKI raqamga tegishli — yangi raqam bilan bog'lanmaydi.
		if phoneChanged {
			m.UserID = ""
		}
	}
	if !want || (had && !relink) {
		return noop, nil
	}
	if s.accounts == nil {
		return noop, ErrAccountsDisabled
	}
	uid, err := s.accounts.Enable(ctx, m)
	if err != nil {
		return noop, err
	}
	m.UserID = uid
	ev, err := s.newEvent(m, actorID, EventAccessGranted, "", "")
	if err != nil {
		_ = s.accounts.Disable(ctx, m.RestaurantID, uid)
		return noop, err
	}
	*events = append(*events, ev)
	return func() {
		if err := s.accounts.Disable(context.WithoutCancel(ctx), m.RestaurantID, uid); err != nil {
			slog.Error("xodim: saqlanmagan yozuv uchun ochilgan kirishni yopib bo'lmadi",
				"restaurant", m.RestaurantID, "staff", m.ID, "err", err)
		}
	}, nil
}

func sameDate(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func (s *Service) Get(ctx context.Context, restaurantID, id string) (*Member, error) {
	return s.repo.Get(ctx, restaurantID, id)
}

// Activity — bitta xodimning faoliyat jurnali.
func (s *Service) Activity(ctx context.Context, restaurantID, id string, limit int) ([]Event, error) {
	if _, err := s.repo.Get(ctx, restaurantID, id); err != nil {
		return nil, err
	}
	return s.repo.Events(ctx, restaurantID, id, time.Time{}, limit)
}

// Overview — "Xodimlar" sahifasi uchun hammasi bitta javobda.
type Overview struct {
	Members []*Member
	Summary Summary
	Recent  []Event
}

const recentLimit = 12

func (s *Service) Overview(ctx context.Context, restaurantID string) (Overview, error) {
	list, err := s.repo.List(ctx, restaurantID)
	if err != nil {
		return Overview{}, err
	}
	now := s.now()
	week, err := s.repo.Events(ctx, restaurantID, "", now.Add(-7*24*time.Hour), 5000)
	if err != nil {
		return Overview{}, err
	}
	recent, err := s.repo.Events(ctx, restaurantID, "", time.Time{}, recentLimit)
	if err != nil {
		return Overview{}, err
	}
	return Overview{Members: list, Summary: Summarize(list, week, now), Recent: recent}, nil
}

// Summary — yuqoridagi to'rt karta va lavozimlar taqsimoti.
type Summary struct {
	Total              int `json:"total"`
	TotalWeekDelta     int `json:"total_week_delta"`
	Active             int `json:"active"`
	ActiveWeekDelta    int `json:"active_week_delta"`
	OnLeave            int `json:"on_leave"`
	Dismissed          int `json:"dismissed"`
	DismissedWeekDelta int `json:"dismissed_week_delta"`
	// WorkingToday — faol va jadvaliga ko'ra smenasi BUGUN boshlanadigan.
	WorkingToday int `json:"working_today"`
	// WorkingYesterday — xuddi shu hisob kecha uchun (hozirgi jadval bo'yicha).
	WorkingYesterday int `json:"working_yesterday"`
	// ByPosition — ishdan bo'shamaganlar lavozim bo'yicha.
	ByPosition map[Position]int `json:"by_position"`
}

// Summarize — "oxirgi hafta" farqlari TAXMIN EMAS: 7 kun oldingi holat
// hodisalar jurnalidan tiklanadi (har bir holat o'zgarishi yoziladi).
// 7 kun ichida qo'shilganlar o'sha paytda hali yo'q edi.
func Summarize(list []*Member, weekEvents []Event, now time.Time) Summary {
	cutoff := now.Add(-7 * 24 * time.Hour)
	then := make(map[string]Status, len(list))
	for _, m := range list {
		then[m.ID] = m.Status
	}
	// Eng yangidan eskisiga: har bir holat o'zgarishi "oldingi" qiymatni tiklaydi.
	for _, ev := range weekEvents {
		if ev.Kind == EventStatusChanged && !ev.At.Before(cutoff) {
			if _, ok := then[ev.MemberID]; ok {
				then[ev.MemberID] = Status(ev.From)
			}
		}
	}
	local := now.In(Location)
	today := isoWeekday(local)
	yesterday := isoWeekday(local.AddDate(0, 0, -1))

	sum := Summary{ByPosition: map[Position]int{}}
	var total7, active7, dismissed7 int
	for _, m := range list {
		sum.Total++
		switch m.Status {
		case StatusActive:
			sum.Active++
			if m.Schedule.WorksOn(today) {
				sum.WorkingToday++
			}
			if m.Schedule.WorksOn(yesterday) {
				sum.WorkingYesterday++
			}
		case StatusOnLeave:
			sum.OnLeave++
		case StatusDismissed:
			sum.Dismissed++
		}
		if m.Status != StatusDismissed {
			sum.ByPosition[m.Position]++
		}
		if !m.CreatedAt.Before(cutoff) {
			continue
		}
		total7++
		switch then[m.ID] {
		case StatusActive:
			active7++
		case StatusDismissed:
			dismissed7++
		}
	}
	sum.TotalWeekDelta = sum.Total - total7
	sum.ActiveWeekDelta = sum.Active - active7
	sum.DismissedWeekDelta = sum.Dismissed - dismissed7
	return sum
}
