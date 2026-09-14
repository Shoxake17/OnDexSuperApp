package catalog

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"chustapp/internal/orders"
)

func week(open, closeAt string) WorkingHours {
	w := WorkingHours{}
	for d := 1; d <= 7; d++ {
		w.Days = append(w.Days, DayHours{Day: d, Enabled: true, Open: open, Close: closeAt})
	}
	return w
}

// tk — Toshkent vaqti; 2026-09-14 — dushanba.
func tk(day, hour, minute int) time.Time {
	return time.Date(2026, 9, day, hour, minute, 0, 0, Location)
}

func TestWorkingHoursValidate(t *testing.T) {
	ok, err := week("08:00", "23:00").Validate()
	if err != nil || len(ok.Days) != 7 {
		t.Fatalf("to'g'ri jadval: %v", err)
	}

	shuffled := week("08:00", "23:00")
	shuffled.Days[0], shuffled.Days[6] = shuffled.Days[6], shuffled.Days[0]
	sorted, err := shuffled.Validate()
	if err != nil || sorted.Days[0].Day != 1 || sorted.Days[6].Day != 7 {
		t.Fatalf("saralanmadi: %+v %v", sorted.Days, err)
	}

	bad := []WorkingHours{
		{},
		{Days: week("08:00", "23:00").Days[:6]},
		func() WorkingHours { w := week("08:00", "23:00"); w.Days[1].Day = 1; return w }(),
		func() WorkingHours { w := week("08:00", "23:00"); w.Days[2].Day = 8; return w }(),
		week("8:00", "23:00"),
		week("08:00", "24:00"),
		week("+1:00", "23:00"),
		week("08:60", "23:00"),
		week("", "23:00"),
	}
	for i, w := range bad {
		if _, err := w.Validate(); !errors.Is(err, ErrBadHours) {
			t.Errorf("%d: rad etilmadi (%v)", i, err)
		}
	}
}

func TestWorkingHoursIsOpenAt(t *testing.T) {
	var none *WorkingHours
	if !none.IsOpenAt(tk(14, 3, 0)) {
		t.Fatal("belgilanmagan jadval — har doim ochiq")
	}

	day := week("08:00", "23:00")
	for _, c := range []struct {
		at   time.Time
		want bool
	}{
		{tk(14, 7, 59), false},
		{tk(14, 8, 0), true},
		{tk(14, 22, 59), true},
		{tk(14, 23, 0), false},
		// Toshkent vaqti: UTC 03:00 = 08:00.
		{time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC), true},
	} {
		if got := day.IsOpenAt(c.at); got != c.want {
			t.Errorf("08-23 %v: %v, kutilgan %v", c.at, got, c.want)
		}
	}

	// Tungi: 18:00-02:00; yakshanba dam olish kuni.
	night := week("18:00", "02:00")
	night.Days[6].Enabled = false
	for _, c := range []struct {
		name string
		at   time.Time
		want bool
	}{
		{"dushanba 17:59", tk(14, 17, 59), false},
		{"dushanba 23:30", tk(14, 23, 30), true},
		{"seshanba 01:59 — dushanbaning davomi", tk(15, 1, 59), true},
		{"seshanba 02:00", tk(15, 2, 0), false},
		{"yakshanba 20:00 — dam olish", tk(20, 20, 0), false},
		{"yakshanba 01:00 — shanbaning davomi", tk(20, 1, 0), true},
		{"dushanba 01:00 — yakshanba dam, davomi yo'q", tk(21, 1, 0), false},
	} {
		if got := night.IsOpenAt(c.at); got != c.want {
			t.Errorf("%s: %v, kutilgan %v", c.name, got, c.want)
		}
	}

	allDay := week("00:00", "00:00")
	if !allDay.IsOpenAt(tk(16, 4, 17)) {
		t.Fatal("ochilish = yopilish — kun bo'yi")
	}
	closed := week("08:00", "23:00")
	for i := range closed.Days {
		closed.Days[i].Enabled = false
	}
	if closed.IsOpenAt(tk(16, 12, 0)) {
		t.Fatal("hamma kun o'chirilgan — yopiq")
	}
}

func TestRestaurantKinds(t *testing.T) {
	if k, err := ParseRestaurantKind(""); err != nil || k != "" {
		t.Fatalf("bo'sh — belgilanmagan: %q %v", k, err)
	}
	if k, err := ParseRestaurantKind(" Teahouse "); err != nil || k.Title() != "Choyxona" {
		t.Fatalf("choyxona: %q %v", k, err)
	}
	if _, err := ParseRestaurantKind("sauna"); !errors.Is(err, ErrUnknownRestaurantKind) {
		t.Fatal("noma'lum tur qabul qilindi")
	}
	seen := map[string]bool{}
	for _, k := range RestaurantKinds() {
		if k.Title == "" || seen[k.Title] {
			t.Errorf("nom bo'sh yoki takror: %+v", k)
		}
		seen[k.Title] = true
	}
}

func TestNormalizeDescription(t *testing.T) {
	got, err := NormalizeDescription("  Mazali taomlar\r\nva ajoyib muhit 👨‍🍳  ")
	if err != nil || got != "Mazali taomlar\nva ajoyib muhit 👨‍🍳" {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := NormalizeDescription(strings.Repeat("я", MaxDescriptionLen)); err != nil {
		t.Fatalf("500 belgi (kirill — bayt emas): %v", err)
	}
	if _, err := NormalizeDescription(strings.Repeat("a", MaxDescriptionLen+1)); !errors.Is(err, ErrDescriptionTooLong) {
		t.Fatal("uzun tavsif qabul qilindi")
	}
	for _, bad := range []string{"a\x00b", "a\u202eb", "a\u0007b"} {
		if _, err := NormalizeDescription(bad); !errors.Is(err, ErrDescriptionChars) {
			t.Errorf("%q qabul qilindi", bad)
		}
	}
}

func TestPaymentMethods(t *testing.T) {
	legacy := &Restaurant{}
	if !legacy.AcceptsPayment(orders.PaymentCash) || !legacy.AcceptsPayment(orders.PaymentCard) {
		t.Fatal("sozlanmagan restoran avvalgidek naqd va kartani qabul qilishi kerak")
	}
	terminalOnly := &Restaurant{PaymentMethods: &PaymentMethods{CardTerminal: true}}
	if !terminalOnly.AcceptsPayment(orders.PaymentCash) || terminalOnly.AcceptsPayment(orders.PaymentCard) {
		t.Fatal("faqat terminal: joyida to'lov bor, onlayn yo'q")
	}
	onlineOnly := &Restaurant{PaymentMethods: &PaymentMethods{CardOnline: true}}
	if onlineOnly.AcceptsPayment(orders.PaymentCash) || !onlineOnly.AcceptsPayment(orders.PaymentCard) {
		t.Fatal("faqat onlayn karta")
	}
	if onlineOnly.AcceptsPayment("payme") {
		t.Fatal("noma'lum usul qabul qilindi")
	}
	if err := (PaymentMethods{}).Validate(); !errors.Is(err, ErrNoPaymentMethod) {
		t.Fatal("hammasi o'chiq — rad etilishi kerak")
	}
	// OnDex Wallet: saqlangan qiymatdan qat'i nazar doim yoqilgan...
	if !legacy.EffectivePaymentMethods().OnDexWallet || !onlineOnly.EffectivePaymentMethods().OnDexWallet {
		t.Fatal("OnDex Wallet doim yoqilgan bo'lishi kerak")
	}
	// ...lekin hamyonning o'zi "kamida bitta usul" o'rnini bosmaydi.
	if err := (PaymentMethods{OnDexWallet: true}).Validate(); !errors.Is(err, ErrNoPaymentMethod) {
		t.Fatal("faqat hamyon — rad etilishi kerak (hamyon to'lovi hali ishlamaydi)")
	}
}

func TestPriceOrderRespectsWorkingHours(t *testing.T) {
	repo := newFakeRepo()
	hours := week("08:00", "23:00")
	repo.restaurants["r1"].WorkingHours = &hours
	svc := NewService(repo)

	svc.now = func() time.Time { return tk(14, 12, 0) }
	if _, _, err := svc.PriceOrder(context.Background(), []ItemRequest{{ProductID: "p1", Qty: 1}}); err != nil {
		t.Fatalf("ish vaqtida: %v", err)
	}
	svc.now = func() time.Time { return tk(14, 23, 30) }
	_, _, err := svc.PriceOrder(context.Background(), []ItemRequest{{ProductID: "p1", Qty: 1}})
	if !errors.Is(err, ErrOutsideHours) || !errors.Is(err, ErrRestaurantClosed) {
		t.Fatalf("ish vaqtidan tashqarida: %v", err)
	}
}

func TestCheckPayment(t *testing.T) {
	repo := newFakeRepo()
	repo.restaurants["r1"].PaymentMethods = &PaymentMethods{Cash: true}
	svc := NewService(repo)
	ctx := context.Background()
	if err := svc.CheckPayment(ctx, "r1", orders.PaymentCash); err != nil {
		t.Fatal(err)
	}
	if err := svc.CheckPayment(ctx, "r1", orders.PaymentCard); !errors.Is(err, ErrPaymentNotAccepted) {
		t.Fatalf("o'chirilgan onlayn karta: %v", err)
	}
	if err := svc.CheckPayment(ctx, "yoq", orders.PaymentCash); err == nil {
		t.Fatal("mavjud bo'lmagan restoran")
	}
}
