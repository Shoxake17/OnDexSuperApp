package tables

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type memRepo struct {
	byID map[string]Table
}

func (m *memRepo) conflict(t *Table, skip string) bool {
	for id, x := range m.byID {
		if id != skip && sameSlot(&x, t) {
			return true
		}
	}
	return false
}

func (m *memRepo) Create(_ context.Context, t *Table) error {
	if m.byID == nil {
		m.byID = map[string]Table{}
	}
	if m.conflict(t, "") {
		return ErrDuplicate
	}
	m.byID[t.ID] = *t
	return nil
}

func (m *memRepo) CreateMany(ctx context.Context, list []*Table) error {
	if m.byID == nil {
		m.byID = map[string]Table{}
	}
	for _, t := range list {
		if m.conflict(t, "") {
			return ErrDuplicate
		}
	}
	for _, t := range list {
		m.byID[t.ID] = *t
	}
	return nil
}

func (m *memRepo) GetByID(_ context.Context, id string) (*Table, error) {
	x, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := x
	return &cp, nil
}

func (m *memRepo) GetByToken(_ context.Context, token string) (*Table, error) {
	for _, x := range m.byID {
		if x.QRToken == token {
			cp := x
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (m *memRepo) ListByRestaurant(_ context.Context, restaurantID string) ([]*Table, error) {
	var out []*Table
	for _, x := range m.byID {
		if x.RestaurantID == restaurantID {
			cp := x
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *memRepo) Update(_ context.Context, t *Table) error {
	old, ok := m.byID[t.ID]
	if !ok {
		return ErrNotFound
	}
	cp := *t
	cp.QRToken = old.QRToken
	m.byID[t.ID] = cp
	return nil
}

func (m *memRepo) TouchScanned(_ context.Context, id string, at time.Time, minInterval time.Duration) error {
	x, ok := m.byID[id]
	if !ok {
		return nil
	}
	if x.LastScannedAt != nil && x.LastScannedAt.After(at.Add(-minInterval)) {
		return nil
	}
	x.LastScannedAt = &at
	m.byID[id] = x
	return nil
}

func (m *memRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.byID[id]; !ok {
		return ErrNotFound
	}
	delete(m.byID, id)
	return nil
}

func intp(v int) *int { return &v }

func TestCreateInZoneAllowsSameNumberInDifferentZones(t *testing.T) {
	svc := NewService(&memRepo{})
	svc.now = func() time.Time { return time.Unix(1, 0) }
	a, err := svc.CreateInZone(context.Background(), "r1", DefaultZone, "5")
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.CreateInZone(context.Background(), "r1", "Ayvon", "5")
	if err != nil {
		t.Fatal(err)
	}
	if a.QRToken == b.QRToken {
		t.Fatal("har bir stol o'zining abadiy tokeniga ega bo'lishi shart")
	}
	if _, err := svc.CreateInZone(context.Background(), "r1", DefaultZone, "5"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("bir zonada takroriy raqam: %v", err)
	}
	// Harf registri farqi takrorni yashira olmaydi.
	if _, err := svc.CreateInZone(context.Background(), "r1", "ayvon", "5"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("\"ayvon\" va \"Ayvon\" bir zal: %v", err)
	}
}

func TestSameLabelAllowedForDifferentKinds(t *testing.T) {
	svc := NewService(&memRepo{})
	ctx := context.Background()
	if _, err := svc.CreateTable(ctx, "r1", Spec{Label: "1"}); err != nil {
		t.Fatal(err)
	}
	cabin, err := svc.CreateTable(ctx, "r1", Spec{Label: "1", Kind: "cabin", Capacity: intp(6)})
	if err != nil {
		t.Fatalf("bir zalda \"Stol 1\" va \"Kabina 1\" birga bo'lishi kerak: %v", err)
	}
	if cabin.Kind != KindCabin || cabin.Capacity == nil || *cabin.Capacity != 6 {
		t.Fatalf("tur/sig'im saqlanmadi: %+v", cabin)
	}
	if _, err := svc.CreateTable(ctx, "r1", Spec{Label: "1", Kind: "CABIN"}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("ikkinchi \"Kabina 1\": %v", err)
	}
}

func TestCreateValidation(t *testing.T) {
	svc := NewService(&memRepo{})
	ctx := context.Background()
	cases := []struct {
		spec Spec
		want error
	}{
		{Spec{Label: ""}, ErrEmptyLabel},
		{Spec{Label: strings.Repeat("a", maxLabelLen+1)}, ErrLabelTooLong},
		{Spec{Label: "5\n6"}, ErrControlChars},
		{Spec{Label: "5\u202e"}, ErrControlChars}, // yo'nalish o'zgartirgich
		{Spec{Label: "5", Zone: "Zal\x00"}, ErrControlChars},
		{Spec{Label: "5", Kind: "sauna"}, ErrUnknownKind},
		{Spec{Label: "5", Capacity: intp(0)}, ErrBadCapacity},
		{Spec{Label: "5", Capacity: intp(MaxCapacity + 1)}, ErrBadCapacity},
	}
	for _, c := range cases {
		if _, err := svc.CreateTable(ctx, "r1", c.spec); !errors.Is(err, c.want) {
			t.Errorf("%+v: kutilgan %v, keldi %v", c.spec, c.want, err)
		}
	}
	// Sig'im berilmasa — nil (taxmin qilinmaydi).
	x, err := svc.CreateTable(ctx, "r1", Spec{Label: "7"})
	if err != nil {
		t.Fatal(err)
	}
	if x.Capacity != nil || x.Kind != KindTable || x.Zone != DefaultZone {
		t.Fatalf("standart qiymatlar: %+v", x)
	}
}

func TestQRTokenDoesNotChangeOnAnyEdit(t *testing.T) {
	svc := NewService(&memRepo{})
	ctx := context.Background()
	t0, err := svc.CreateInZone(ctx, "r1", DefaultZone, "1")
	if err != nil {
		t.Fatal(err)
	}
	token := t0.QRToken
	label, zone, kind, active, cleaning := "12", "VIP", "vip_room", false, true
	edited, err := svc.Edit(ctx, t0.ID, Patch{
		Label: &label, Zone: &zone, Kind: &kind, Capacity: intp(10),
		Active: &active, Cleaning: &cleaning,
	})
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := svc.Get(ctx, t0.ID)
	if edited.QRToken != token || stored.QRToken != token {
		t.Fatalf("QR token o'zgardi: %q → %q / %q", token, edited.QRToken, stored.QRToken)
	}
	if stored.Label != "12" || stored.Zone != "VIP" || stored.Kind != KindVIPRoom ||
		*stored.Capacity != 10 || stored.Active || stored.CleaningSince == nil {
		t.Fatalf("tahrir saqlanmadi: %+v", stored)
	}
}

func TestEditRejectsDuplicateAndKeepsOriginal(t *testing.T) {
	svc := NewService(&memRepo{})
	ctx := context.Background()
	if _, err := svc.Create(ctx, "r1", "1"); err != nil {
		t.Fatal(err)
	}
	two, err := svc.Create(ctx, "r1", "2")
	if err != nil {
		t.Fatal(err)
	}
	label := "1"
	if _, err := svc.Edit(ctx, two.ID, Patch{Label: &label, Capacity: intp(4)}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("takror nomga o'zgartirildi: %v", err)
	}
	got, _ := svc.Get(ctx, two.ID)
	if got.Label != "2" || got.Capacity != nil {
		t.Fatalf("yiqilgan tahrirning bir qismi saqlanib qoldi: %+v", got)
	}
}

func TestCleaningFlag(t *testing.T) {
	svc := NewService(&memRepo{})
	ctx := context.Background()
	clock := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return clock }
	x, _ := svc.Create(ctx, "r1", "1")

	on, off := true, false
	first, _ := svc.Edit(ctx, x.ID, Patch{Cleaning: &on})
	clock = clock.Add(10 * time.Minute)
	again, _ := svc.Edit(ctx, x.ID, Patch{Cleaning: &on})
	// Qayta bosilsa vaqt yangilanmaydi: "12 daqiqadan beri tozalanmoqda".
	if first.CleaningSince == nil || !again.CleaningSince.Equal(*first.CleaningSince) {
		t.Fatalf("tozalash boshlangan vaqt: %v → %v", first.CleaningSince, again.CleaningSince)
	}
	cleared, _ := svc.Edit(ctx, x.ID, Patch{Cleaning: &off})
	if cleared.CleaningSince != nil {
		t.Fatal("belgi olib tashlanmadi")
	}
}

func TestCreateBatch(t *testing.T) {
	svc := NewService(&memRepo{})
	ctx := context.Background()
	list, err := svc.CreateBatch(ctx, "r1", BatchSpec{Kind: "cabin", Prefix: "", From: 1, Count: 3, Capacity: intp(4)})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].Label != "1" || list[2].Label != "3" || list[1].Kind != KindCabin {
		t.Fatalf("partiya: %+v", list)
	}
	tokens := map[string]bool{}
	for _, x := range list {
		tokens[x.QRToken] = true
	}
	if len(tokens) != 3 {
		t.Fatal("partiyada tokenlar takrorlandi")
	}

	// 2..4 — "2" va "3" band: HECH NARSA yaratilmaydi.
	if _, err := svc.CreateBatch(ctx, "r1", BatchSpec{Kind: "cabin", From: 2, Count: 3}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("takror partiya: %v", err)
	}
	all, _ := svc.List(ctx, "r1")
	if len(all) != 3 {
		t.Fatalf("yiqilgan partiyadan joylar qolib ketdi: %d", len(all))
	}

	for _, bad := range []BatchSpec{{Count: 0}, {Count: MaxBatch + 1}, {Count: 1, From: -1}, {Count: 1, From: maxBatchStart + 1}} {
		if _, err := svc.CreateBatch(ctx, "r1", bad); !errors.Is(err, ErrBadBatch) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	if _, err := svc.CreateBatch(ctx, "r1", BatchSpec{Count: 1, Prefix: "A\n"}); !errors.Is(err, ErrControlChars) {
		t.Errorf("prefiksdagi boshqaruv belgisi: %v", err)
	}
}

func TestBatchLabel(t *testing.T) {
	for _, c := range []struct {
		prefix string
		n      int
		want   string
	}{
		{"", 5, "5"}, {"VIP", 5, "VIP 5"}, {"A-", 5, "A-5"}, {"  Kabina  ", 12, "Kabina 12"}, {"T2", 1, "T2 1"},
	} {
		if got := batchLabel(c.prefix, c.n); got != c.want {
			t.Errorf("%q+%d = %q, kutilgan %q", c.prefix, c.n, got, c.want)
		}
	}
}

func TestLimitPerRestaurant(t *testing.T) {
	svc := NewService(&memRepo{})
	ctx := context.Background()
	for from := 0; from < MaxTablesPerRestaurant; from += MaxBatch {
		if _, err := svc.CreateBatch(ctx, "r1", BatchSpec{From: from, Count: MaxBatch}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Create(ctx, "r1", "extra"); !errors.Is(err, ErrLimitReached) {
		t.Fatalf("chegaradan oshdi: %v", err)
	}
	if _, err := svc.CreateBatch(ctx, "r1", BatchSpec{From: 5000, Count: 1}); !errors.Is(err, ErrLimitReached) {
		t.Fatalf("partiya chegaradan oshdi: %v", err)
	}
	// Boshqa restoranga ta'sir qilmaydi.
	if _, err := svc.Create(ctx, "r2", "1"); err != nil {
		t.Fatal(err)
	}
}

func TestMarkScannedThrottled(t *testing.T) {
	svc := NewService(&memRepo{})
	ctx := context.Background()
	clock := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return clock }
	x, _ := svc.Create(ctx, "r1", "1")
	_ = svc.MarkScanned(ctx, x.ID)
	clock = clock.Add(20 * time.Second)
	_ = svc.MarkScanned(ctx, x.ID)
	got, _ := svc.Get(ctx, x.ID)
	if got.LastScannedAt == nil || !got.LastScannedAt.Equal(clock.Add(-20*time.Second)) {
		t.Fatalf("bir daqiqa ichida qayta yozildi: %v", got.LastScannedAt)
	}
	clock = clock.Add(time.Minute)
	_ = svc.MarkScanned(ctx, x.ID)
	got, _ = svc.Get(ctx, x.ID)
	if !got.LastScannedAt.Equal(clock) {
		t.Fatalf("bir daqiqadan keyin yangilanmadi: %v", got.LastScannedAt)
	}
}

func TestDisplayLabel(t *testing.T) {
	for _, c := range []struct {
		t    Table
		want string
	}{
		{Table{Zone: DefaultZone, Label: "3"}, "Asosiy zal · 3"},
		{Table{Zone: "", Kind: KindTable, Label: "3"}, "Asosiy zal · 3"},
		{Table{Zone: "Ayvon", Kind: KindCabin, Label: "2"}, "Ayvon · Kabina 2"},
		{Table{Zone: "Ayvon", Kind: KindCabin, Label: "kabina 2"}, "Ayvon · kabina 2"},
		{Table{Zone: "2-qavat", Kind: KindVIPRoom, Label: "Oltin"}, "2-qavat · VIP xona Oltin"},
	} {
		if got := c.t.DisplayLabel(); got != c.want {
			t.Errorf("%+v: %q, kutilgan %q", c.t, got, c.want)
		}
	}
}

func TestStatusOf(t *testing.T) {
	base := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	cleaning := base
	before := &OrderSnapshot{CreatedAt: base.Add(-time.Hour)}
	after := &OrderSnapshot{CreatedAt: base.Add(time.Minute)}
	active := []OrderSnapshot{{ID: "o1"}}

	cases := []struct {
		name   string
		t      Table
		active []OrderSnapshot
		last   *OrderSnapshot
		want   Status
	}{
		{"bo'sh", Table{Active: true}, nil, nil, StatusAvailable},
		{"band", Table{Active: true}, active, nil, StatusOccupied},
		{"yopiq, lekin ichida mehmon", Table{Active: false}, active, nil, StatusOccupied},
		{"yopiq", Table{Active: false}, nil, nil, StatusInactive},
		{"tozalanmoqda", Table{Active: true, CleaningSince: &cleaning}, nil, before, StatusCleaning},
		{"tozalanmoqda, buyurtma hech bo'lmagan", Table{Active: true, CleaningSince: &cleaning}, nil, nil, StatusCleaning},
		{"belgidan keyin yangi buyurtma — eskirgan", Table{Active: true, CleaningSince: &cleaning}, nil, after, StatusAvailable},
		{"tozalanayotgan joy band bo'lsa — band", Table{Active: true, CleaningSince: &cleaning}, active, after, StatusOccupied},
	}
	for _, c := range cases {
		if got := StatusOf(&c.t, c.active, c.last); got != c.want {
			t.Errorf("%s: %s, kutilgan %s", c.name, got, c.want)
		}
	}
}
