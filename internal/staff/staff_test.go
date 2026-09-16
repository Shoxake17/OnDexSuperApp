package staff

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"
)

// fakeRepo — xotira ombori (storage paketi import sikli sababli bu yerda).
type fakeRepo struct {
	members   map[string]Member
	events    []Event
	failWrite error
}

func newFakeRepo() *fakeRepo { return &fakeRepo{members: map[string]Member{}} }

func (r *fakeRepo) Create(_ context.Context, m *Member, events []Event) error {
	if r.failWrite != nil {
		return r.failWrite
	}
	n := 0
	for _, x := range r.members {
		if x.RestaurantID == m.RestaurantID && x.Number > n {
			n = x.Number
		}
	}
	m.Number = n + 1
	r.members[m.ID] = Clone(*m)
	r.events = append(r.events, events...)
	return nil
}

func (r *fakeRepo) Update(_ context.Context, m *Member, events []Event) error {
	if r.failWrite != nil {
		return r.failWrite
	}
	r.members[m.ID] = Clone(*m)
	r.events = append(r.events, events...)
	return nil
}

func (r *fakeRepo) Get(_ context.Context, rid, id string) (*Member, error) {
	x, ok := r.members[id]
	if !ok || x.RestaurantID != rid {
		return nil, ErrNotFound
	}
	c := Clone(x)
	return &c, nil
}

func (r *fakeRepo) List(_ context.Context, rid string) ([]*Member, error) {
	var out []*Member
	for _, x := range r.members {
		if x.RestaurantID == rid {
			c := Clone(x)
			out = append(out, &c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out, nil
}

func (r *fakeRepo) Events(_ context.Context, rid, mid string, since time.Time, limit int) ([]Event, error) {
	var out []Event
	for i := len(r.events) - 1; i >= 0 && len(out) < limit; i-- {
		ev := r.events[i]
		if ev.RestaurantID == rid && (mid == "" || ev.MemberID == mid) && !ev.At.Before(since) {
			out = append(out, ev)
		}
	}
	return out, nil
}

type fakeAccounts struct {
	enabled     map[string]bool
	disableErr  error
	disabled    []string
	enableCalls int
}

func (a *fakeAccounts) Enable(_ context.Context, m *Member) (string, error) {
	a.enableCalls++
	id := m.UserID
	if id == "" {
		id = "acc-" + m.Phone
	}
	a.enabled[id] = true
	return id, nil
}

func (a *fakeAccounts) Refresh(context.Context, *Member) error { return nil }

func (a *fakeAccounts) Disable(_ context.Context, _, userID string) error {
	if a.disableErr != nil {
		return a.disableErr
	}
	a.disabled = append(a.disabled, userID)
	a.enabled[userID] = false
	return nil
}

func TestScheduleNormalize(t *testing.T) {
	ok, err := Schedule{Days: []int{7, 1, 3}, Start: "18:00", End: "02:00"}.Normalize()
	if err != nil || ok.Days[0] != 1 || ok.Days[2] != 7 {
		t.Fatalf("%+v %v", ok, err)
	}
	for _, s := range []Schedule{
		{Days: nil, Start: "08:00", End: "22:00"},
		{Days: []int{0}, Start: "08:00", End: "22:00"},
		{Days: []int{1, 1}, Start: "08:00", End: "22:00"},
		{Days: []int{1}, Start: "24:00", End: "22:00"},
		{Days: []int{1}, Start: "08:60", End: "22:00"},
		{Days: []int{1}, Start: "8:00", End: "22:00"},
		{Days: []int{1}, Start: "+8:00", End: "22:00"},
	} {
		if _, err := s.Normalize(); !errors.Is(err, ErrBadSchedule) {
			t.Errorf("%+v qabul qilindi", s)
		}
	}
}

func TestNames(t *testing.T) {
	for _, good := range []string{"Gulnora", "O'ktam", "Oʻktam", "Jo‘rayev", "Анна-Мария", "Ali  Vali"} {
		if _, err := normalizeName(good, true); err != nil {
			t.Errorf("%q rad etildi: %v", good, err)
		}
	}
	for _, bad := range []string{"", "Ali7", "<b>", "a\u202eb", "Ali" + string(rune(0)) + "Vali"} {
		if _, err := normalizeName(bad, true); err == nil {
			t.Errorf("%q qabul qilindi", bad)
		}
	}
	// Bo'sh joylar (yangi qator ham) bitta probelga keltiriladi.
	for _, in := range []string{"  Ali   Vali ", "Ali\nVali", "Ali\tVali"} {
		if v, _ := normalizeName(in, true); v != "Ali Vali" {
			t.Fatalf("%q -> %q", in, v)
		}
	}
}

func TestAccessFailsClosed(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	acc := &fakeAccounts{enabled: map[string]bool{}}
	svc := NewService(repo, acc)
	m, err := svc.Create(ctx, "r1", "actor", Input{FirstName: "Ali", Phone: "+998901234567", Position: "waiter", AppAccess: true})
	if err != nil || m.UserID == "" || !acc.enabled[m.UserID] {
		t.Fatalf("%+v %v", m, err)
	}

	// Akkauntni yopib bo'lmasa — holat SAQLANMAYDI (yozuv va kirish mos qoladi).
	acc.disableErr = errors.New("baza yo'q")
	if _, err := svc.SetStatus(ctx, "r1", m.ID, "actor", "dismissed"); err == nil {
		t.Fatal("xato kutilgan edi")
	}
	if cur, _ := repo.Get(ctx, "r1", m.ID); cur.Status != StatusActive {
		t.Fatal("akkaunt yopilmagan holda yozuv 'ishdan bo'shagan' bo'lib qoldi")
	}
	acc.disableErr = nil

	// Saqlash yiqilsa — yopilgan kirish yopiqligicha qoladi (xavfsiz tomon).
	repo.failWrite = errors.New("yozib bo'lmadi")
	if _, err := svc.SetStatus(ctx, "r1", m.ID, "actor", "on_leave"); err == nil {
		t.Fatal("xato kutilgan edi")
	}
	if acc.enabled[m.UserID] {
		t.Fatal("saqlash yiqilganda kirish ochiq qoldi")
	}
	repo.failWrite = nil

	// Yangi xodimda saqlash yiqilsa ochilgan akkaunt yana yopiladi.
	repo.failWrite = errors.New("yozib bo'lmadi")
	if _, err := svc.Create(ctx, "r1", "actor", Input{FirstName: "Vali", Phone: "+998901234568", Position: "waiter", AppAccess: true}); err == nil {
		t.Fatal("xato kutilgan edi")
	}
	if acc.enabled["acc-+998901234568"] {
		t.Fatal("saqlanmagan xodim uchun kirish ochiq qoldi")
	}
	repo.failWrite = nil

	// Telefon o'zgarsa eski akkaunt uziladi, yangi raqamga yangisi ochiladi.
	if _, err := svc.SetStatus(ctx, "r1", m.ID, "actor", "active"); err != nil {
		t.Fatal(err)
	}
	phone := "+998909999999"
	upd, err := svc.Update(ctx, "r1", m.ID, "actor", Patch{Phone: &phone})
	if err != nil || upd.UserID != "acc-+998909999999" || acc.enabled["acc-+998901234567"] {
		t.Fatalf("telefon almashganda: %+v %v %+v", upd, err, acc.enabled)
	}
}

// Ilovali ikki lavozim o'rtasidagi almashuv (ofitsiant -> yetkazib
// beruvchi) akkauntni QAYTA bog'laydi: eski rol yopiladi, yangisi
// ochiladi. Avval bu "o'zgarish yo'q" deb o'tkazib yuborilardi.
func TestPositionSwitchRelinksAccount(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	acc := &fakeAccounts{enabled: map[string]bool{}}
	svc := NewService(repo, acc)
	m, err := svc.Create(ctx, "r1", "actor", Input{FirstName: "Ali", Phone: "+998901234567", Position: "waiter", AppAccess: true})
	if err != nil || acc.enableCalls != 1 {
		t.Fatalf("%+v %v", m, err)
	}

	courier := "courier"
	upd, err := svc.Update(ctx, "r1", m.ID, "actor", Patch{Position: &courier})
	if err != nil {
		t.Fatal(err)
	}
	if len(acc.disabled) != 1 || acc.enableCalls != 2 {
		t.Fatalf("almashuvda eski rol yopilib yangisi ochilishi kerak: disabled=%v enable=%d", acc.disabled, acc.enableCalls)
	}
	if !upd.AppAccess || upd.UserID == "" || !acc.enabled[upd.UserID] || !upd.AccessEffective() {
		t.Fatalf("yetkazib beruvchida kirish ochiq bo'lishi kerak: %+v", upd)
	}

	// Ilovasiz lavozimga o'tsa kirish yopiladi va tanlov o'chadi.
	chef := "chef"
	upd, err = svc.Update(ctx, "r1", m.ID, "actor", Patch{Position: &chef})
	if err != nil || upd.AppAccess || acc.enabled[upd.UserID] {
		t.Fatalf("oshpazga o'tganda kirish yopilishi kerak: %+v %v", upd, err)
	}
}

func TestCourierPositionAllowsApp(t *testing.T) {
	if !PositionCourier.AllowsAppAccess() || !PositionWaiter.AllowsAppAccess() || PositionChef.AllowsAppAccess() {
		t.Fatal("ilovali lavozimlar: faqat ofitsiant va yetkazib beruvchi")
	}
}

func TestSummarizeWeek(t *testing.T) {
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, Location) // dushanba
	old := now.AddDate(0, -1, 0)
	fresh := now.Add(-24 * time.Hour)
	all := &Schedule{Days: []int{1, 2, 3, 4, 5, 6, 7}, Start: "08:00", End: "22:00"}
	sunday := &Schedule{Days: []int{7}, Start: "08:00", End: "22:00"}
	list := []*Member{
		{ID: "a", Status: StatusActive, Position: PositionChef, CreatedAt: old, Schedule: all},
		{ID: "b", Status: StatusDismissed, Position: PositionWaiter, CreatedAt: old},
		{ID: "c", Status: StatusActive, Position: PositionWaiter, CreatedAt: fresh, Schedule: sunday},
		{ID: "d", Status: StatusActive, Position: PositionCashier, CreatedAt: old},
	}
	// b — shu hafta ishdan bo'shatildi; d — hafta ichida ta'tildan qaytdi.
	events := []Event{
		{MemberID: "b", Kind: EventStatusChanged, From: "active", To: "dismissed", At: now.Add(-2 * time.Hour)},
		{MemberID: "d", Kind: EventStatusChanged, From: "on_leave", To: "active", At: now.Add(-3 * 24 * time.Hour)},
	}
	s := Summarize(list, events, now)
	// 7 kun oldin: a faol, b faol, d ta'tilda, c yo'q edi.
	if s.Total != 4 || s.TotalWeekDelta != 1 || s.Active != 3 || s.ActiveWeekDelta != 1 ||
		s.Dismissed != 1 || s.DismissedWeekDelta != 1 || s.WorkingToday != 1 || s.WorkingYesterday != 2 {
		t.Fatalf("%+v", s)
	}
	if s.ByPosition[PositionWaiter] != 1 || s.ByPosition[PositionChef] != 1 {
		t.Fatalf("taqsimot: %+v", s.ByPosition)
	}
}
