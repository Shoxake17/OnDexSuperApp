package staff

import "testing"

func i64(v int64) *int64 { return &v }

func courier(pay *int64, mut func(*Member)) *Member {
	m := &Member{
		Position: PositionCourier, Status: StatusActive, AppAccess: true,
		DeliveryPayTiyin: pay,
	}
	if mut != nil {
		mut(m)
	}
	return m
}

// Mijoz to'laydigan yetkazish haqi — kuryerning maosh usulidan.
func TestDeliveryFeeRule(t *testing.T) {
	cases := []struct {
		name    string
		members []*Member
		want    int64
	}{
		{"kuryer yo'q", nil, 0},
		{"buyurtmadan maosh", []*Member{courier(i64(800_000), nil)}, 800_000},
		{"oylik maosh — bepul", []*Member{courier(nil, func(m *Member) { m.SalaryTiyin = i64(350_000_000) })}, 0},
		{"maosh kiritilmagan — bepul", []*Member{courier(nil, nil)}, 0},
		{"eng yuqorisi", []*Member{courier(i64(600_000), nil), courier(i64(900_000), nil), courier(i64(700_000), nil)}, 900_000},
		{"oylik kuryer natijani pasaytirmaydi", []*Member{
			courier(nil, func(m *Member) { m.SalaryTiyin = i64(350_000_000) }), courier(i64(600_000), nil)}, 600_000},
		{"ta'tildagi hisobga olinmaydi", []*Member{courier(i64(800_000), func(m *Member) { m.Status = StatusOnLeave })}, 0},
		{"bo'shagan hisobga olinmaydi", []*Member{courier(i64(800_000), func(m *Member) { m.Status = StatusDismissed })}, 0},
		{"ilovaga kirishsiz hisobga olinmaydi", []*Member{courier(i64(800_000), func(m *Member) { m.AppAccess = false })}, 0},
		{"kuryer bo'lmagan hisobga olinmaydi", []*Member{courier(i64(800_000), func(m *Member) { m.Position = PositionWaiter })}, 0},
		{"nil xodim xavfsiz", []*Member{nil, courier(i64(500_000), nil)}, 500_000},
		{"faol kuryer ta'tildagidan ustun emas", []*Member{
			courier(i64(900_000), func(m *Member) { m.Status = StatusOnLeave }), courier(i64(500_000), nil)}, 500_000},
	}
	for _, c := range cases {
		if got := DeliveryFee(c.members); got != c.want {
			t.Errorf("%s: %d, %d kutilgan", c.name, got, c.want)
		}
	}
}

func TestValidDeliveryPayBounds(t *testing.T) {
	if v, err := validDeliveryPay(nil); v != nil || err != nil {
		t.Errorf("nil: %v %v", v, err)
	}
	for _, bad := range []int64{0, -1, MaxDeliveryPayTiyin + 1} {
		if _, err := validDeliveryPay(i64(bad)); err != ErrBadDeliveryPay {
			t.Errorf("%d: %v (ErrBadDeliveryPay kutilgan)", bad, err)
		}
	}
	for _, ok := range []int64{1, 800_000, MaxDeliveryPayTiyin} {
		v, err := validDeliveryPay(i64(ok))
		if err != nil || v == nil || *v != ok {
			t.Errorf("%d: %v %v", ok, v, err)
		}
	}
	// Nusxa qaytadi: chaqiruvchining o'zgaruvchisi bilan bog'lanib qolmaydi.
	src := i64(5)
	v, _ := validDeliveryPay(src)
	*src = 9
	if *v != 5 {
		t.Error("natija chaqiruvchining ko'rsatkichiga bog'langan")
	}
}

func TestCheckPayMode(t *testing.T) {
	if err := checkPayMode(courier(nil, nil)); err != nil {
		t.Errorf("maoshsiz kuryer: %v", err)
	}
	if err := checkPayMode(courier(i64(1), nil)); err != nil {
		t.Errorf("buyurtmadan: %v", err)
	}
	if err := checkPayMode(courier(nil, func(m *Member) { m.SalaryTiyin = i64(1) })); err != nil {
		t.Errorf("oylik: %v", err)
	}
	both := courier(i64(1), func(m *Member) { m.SalaryTiyin = i64(1) })
	if err := checkPayMode(both); err != ErrPayModeConflict {
		t.Errorf("ikkalasi: %v", err)
	}
	waiter := courier(i64(1), func(m *Member) { m.Position = PositionWaiter })
	if err := checkPayMode(waiter); err != ErrDeliveryPayNotCourier {
		t.Errorf("ofitsiant: %v", err)
	}
	// Oylik maoshli ofitsiant — oddiy holat, xato emas.
	if err := checkPayMode(&Member{Position: PositionWaiter, SalaryTiyin: i64(1)}); err != nil {
		t.Errorf("oylikli ofitsiant: %v", err)
	}
}

// Xotira repo'si nusxa oladi: bir nusxani o'zgartirish boshqasiga tegmasin.
func TestCloneMemberCopiesDeliveryPay(t *testing.T) {
	m := *courier(i64(800_000), nil)
	c := cloneMember(m)
	*c.DeliveryPayTiyin = 1
	if *m.DeliveryPayTiyin != 800_000 {
		t.Error("cloneMember buyurtmadan maoshni ulashib yubordi")
	}
}
