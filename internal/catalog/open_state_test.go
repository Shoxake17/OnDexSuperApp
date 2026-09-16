package catalog

import (
	"errors"
	"testing"
	"time"
)

func TestNextChangeAfter(t *testing.T) {
	var none *WorkingHours
	if _, ok := none.NextChangeAfter(tk(14, 12, 0)); ok {
		t.Fatal("jadvalsiz — o'zgarish yo'q")
	}

	day := week("08:00", "23:00")
	night := week("18:00", "02:00")
	night.Days[6].Enabled = false // yakshanba dam
	allDay := week("00:00", "00:00")
	off := week("08:00", "23:00")
	for i := range off.Days {
		off.Days[i].Enabled = false
	}

	for _, c := range []struct {
		name string
		w    WorkingHours
		at   time.Time
		want time.Time
		ok   bool
	}{
		{"ochiq — bugun yopiladi", day, tk(14, 12, 0), tk(14, 23, 0), true},
		{"sekund bilan ham — daqiqa chegarasi", day, tk(14, 22, 59).Add(30 * time.Second), tk(14, 23, 0), true},
		{"yopildi — ertaga ochiladi", day, tk(14, 23, 30), tk(15, 8, 0), true},
		{"tong — bugun ochiladi", day, tk(14, 6, 0), tk(14, 8, 0), true},
		{"tungi davom — 02:00 da yopiladi", night, tk(15, 1, 30), tk(15, 2, 0), true},
		{"shanba kechasi — yakshanba 02:00 gacha", night, tk(19, 23, 0), tk(20, 2, 0), true},
		{"yakshanba dam — dushanba 18:00 da", night, tk(20, 3, 0), tk(21, 18, 0), true},
		{"kun bo'yi ochiq — o'zgarmaydi", allDay, tk(14, 12, 0), time.Time{}, false},
		{"hamma kun o'chiq — ochilmaydi", off, tk(14, 12, 0), time.Time{}, false},
	} {
		got, ok := c.w.NextChangeAfter(c.at)
		if ok != c.ok || (ok && !got.Equal(c.want)) {
			t.Errorf("%s: %v %v, kutilgan %v %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestOpenStateAndOrderable(t *testing.T) {
	hours := week("09:00", "22:00")
	r := Restaurant{ID: "r1", Open: true, WorkingHours: &hours}

	st := r.OpenStateAt(tk(14, 21, 0))
	if !st.Open || st.Reason != "" || st.ChangesAt == nil || !st.ChangesAt.Equal(tk(14, 22, 0)) {
		t.Fatalf("ish vaqtida: %+v", st)
	}
	if err := r.OrderableAt(tk(14, 21, 0)); err != nil {
		t.Fatalf("ish vaqtida buyurtma: %v", err)
	}

	st = r.OpenStateAt(tk(14, 22, 30))
	if st.Open || st.Reason != ClosedHours || st.ChangesAt == nil || !st.ChangesAt.Equal(tk(15, 9, 0)) {
		t.Fatalf("ish vaqtidan keyin: %+v", st)
	}
	if err := r.OrderableAt(tk(14, 22, 30)); !errors.Is(err, ErrOutsideHours) || !errors.Is(err, ErrRestaurantClosed) {
		t.Fatalf("ish vaqtidan keyin buyurtma: %v", err)
	}

	r.Open = false
	st = r.OpenStateAt(tk(14, 12, 0))
	if st.Open || st.Reason != ClosedManual || st.ChangesAt != nil {
		t.Fatalf("qo'lda yopilgan — vaqt bo'yicha ochilmaydi: %+v", st)
	}
	if err := r.OrderableAt(tk(14, 12, 0)); !errors.Is(err, ErrRestaurantClosed) || errors.Is(err, ErrOutsideHours) {
		t.Fatalf("qo'lda yopilgan buyurtma: %v", err)
	}

	// Jadvalsiz restoran — faqat tugmaga bo'ysunadi, o'zgarish vaqti yo'q.
	plain := Restaurant{Open: true}
	if st := plain.OpenStateAt(tk(14, 3, 0)); !st.Open || st.ChangesAt != nil {
		t.Fatalf("jadvalsiz: %+v", st)
	}
}

func TestWithOpenStateFillsResponseFields(t *testing.T) {
	hours := week("09:00", "22:00")
	r := Restaurant{ID: "r1", Open: true, WorkingHours: &hours}

	v := r.WithOpenState(tk(14, 23, 0))
	if v.OpenNow == nil || *v.OpenNow || v.ClosedReason != ClosedHours || v.OpenChangesAt == nil {
		t.Fatalf("yopiq javob: %+v", v)
	}
	if v.OpenChangesAt.Location() != time.UTC || !v.OpenChangesAt.Equal(tk(15, 9, 0)) {
		t.Fatalf("ochilish vaqti UTC da bo'lishi kerak: %v", v.OpenChangesAt)
	}
	if r.OpenNow != nil {
		t.Fatal("asl yozuv o'zgarmasligi kerak (nusxa)")
	}

	v = r.WithOpenState(tk(15, 10, 0))
	if v.OpenNow == nil || !*v.OpenNow || v.ClosedReason != "" {
		t.Fatalf("ochiq javob: %+v", v)
	}
}

func TestAttachRestaurantUsesWorkingHours(t *testing.T) {
	closedAll := week("09:00", "22:00")
	for i := range closedAll.Days {
		closedAll.Days[i].Enabled = false
	}
	res := &ProductSearchResult{Product: Product{ID: "p1"}}
	res.AttachRestaurant(&Restaurant{Name: "Kafe", LogoURL: "/l.png", Open: true, WorkingHours: &closedAll})
	if res.RestaurantOpen || res.RestaurantName != "Kafe" || res.RestaurantLogoURL != "/l.png" {
		t.Fatalf("ish vaqti hisobga olinmadi: %+v", res)
	}
	res.AttachRestaurant(&Restaurant{Name: "Kafe", Open: true})
	if !res.RestaurantOpen {
		t.Fatal("jadvalsiz ochiq restoran — ochiq")
	}
}
