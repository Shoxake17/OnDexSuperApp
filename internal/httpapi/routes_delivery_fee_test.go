package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// YETKAZISH HAQI — YETKAZIB BERUVCHINING MAOSH USULIDAN.
//
// ┌─ QOIDA ───────────────────────────────────────────────────────────┐
// Restoran yetkazish haqini alohida kiritmaydi. U yetkazib beruvchining
// maosh usulidan kelib chiqadi:
//
//	"buyurtmadan maosh"  -> shu summa mijozning buyurtmasiga QO'SHILADI;
//	"oylik maosh"        -> yetkazish mijoz uchun BEPUL.
//
// Bu fayl butun zanjirni haqiqiy HTTP orqali tekshiradi: xodim yaratish,
// narxlash (`/quote`) va sozlamalar. Qoidaning o'zi `staff.DeliveryFee`
// da va uning birlik testlari `internal/staff` paketida.
// └───────────────────────────────────────────────────────────────────┘

const quotePathA = "/restaurants/" + testRestA + "/quote"

type quoteResp struct {
	SubtotalTiyin    int64 `json:"subtotal_tiyin"`
	DeliveryFeeTiyin int64 `json:"delivery_fee_tiyin"`
	TotalTiyin       int64 `json:"total_tiyin"`
}

func quoteFor(t *testing.T, f staffFixture, dineIn bool) quoteResp {
	t.Helper()
	body := `{"items":[{"product_id":"prod-a","qty":1}],"dine_in":` + map[bool]string{true: "true", false: "false"}[dineIn] + `}`
	w := do(t, f.h, "POST", quotePathA, f.jwt["customer"], body)
	if w.Code != http.StatusOK {
		t.Fatalf("quote: %d — %s", w.Code, w.Body.String())
	}
	var q quoteResp
	if err := json.Unmarshal(w.Body.Bytes(), &q); err != nil {
		t.Fatal(err)
	}
	return q
}

// courierBody — ilovaga kirishi ochiq yetkazib beruvchi, qo'shimcha maydonlar bilan.
func courierBody(phone, extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return `{"first_name":"Jasur","phone":"` + phone + `","position":"courier","app_access":true` + extra + `}`
}

func patchStaff(t *testing.T, f staffFixture, id, body string) (int, string) {
	t.Helper()
	w := do(t, f.h, "PATCH", staffPathA+"/"+id, f.jwt["a"], body)
	return w.Code, w.Body.String()
}

// Yetkazib beruvchi yo'q — yetkazish bepul.
func TestDeliveryFeeFreeWithoutCouriers(t *testing.T) {
	f := staffServer(t)
	q := quoteFor(t, f, false)
	if q.DeliveryFeeTiyin != 0 || q.TotalTiyin != staffTestGoodsTiyin {
		t.Errorf("kuryersiz: haq %d, jami %d", q.DeliveryFeeTiyin, q.TotalTiyin)
	}
}

// "Buyurtmadan maosh" — shu summa mijozga qo'shiladi.
func TestDeliveryFeeFromPerOrderCourier(t *testing.T) {
	f := staffServer(t)
	createStaff(t, f, courierBody("+998901110001", `"delivery_pay_tiyin":800000`))

	q := quoteFor(t, f, false)
	if q.DeliveryFeeTiyin != 800_000 {
		t.Fatalf("haq %d, 800000 kutilgan", q.DeliveryFeeTiyin)
	}
	if want := int64(staffTestGoodsTiyin + 800_000); q.TotalTiyin != want {
		t.Errorf("jami %d, %d kutilgan", q.TotalTiyin, want)
	}
}

// ┌─ OYLIK MAOSH — YETKAZISH BEPUL ───────────────────────────────────┐
// Aynan shu holat Book Cafe'da bor: kuryer Sherzod 35 000 so'm emas,
// 3 500 000 so'm OYLIK oladi. Uning restoranida mijoz yetkazish uchun
// to'lamasligi kerak.
// └───────────────────────────────────────────────────────────────────┘
func TestDeliveryFeeFreeForMonthlyCourier(t *testing.T) {
	f := staffServer(t)
	createStaff(t, f, courierBody("+998901110002", `"monthly_salary_tiyin":350000000`))

	q := quoteFor(t, f, false)
	if q.DeliveryFeeTiyin != 0 || q.TotalTiyin != staffTestGoodsTiyin {
		t.Errorf("oylikda: haq %d, jami %d (bepul kutilgan)", q.DeliveryFeeTiyin, q.TotalTiyin)
	}
}

// Maosh kiritilmagan kuryer — haq yo'q (bepul). Restoran buni aniq
// ko'rishi uchun panelda "mijoz uchun bepul" deb yoziladi.
func TestDeliveryFeeFreeWhenNoPayConfigured(t *testing.T) {
	f := staffServer(t)
	createStaff(t, f, courierBody("+998901110003", ""))
	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 0 {
		t.Errorf("haq %d, 0 kutilgan", q.DeliveryFeeTiyin)
	}
}

// Usulni almashtirish BITTA so'rovda: oylik -> buyurtmadan.
func TestSwitchCourierPayMode(t *testing.T) {
	f := staffServer(t)
	m := createStaff(t, f, courierBody("+998901110004", `"monthly_salary_tiyin":350000000`))
	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 0 {
		t.Fatalf("oylikda haq %d", q.DeliveryFeeTiyin)
	}

	code, body := patchStaff(t, f, m.ID, `{"monthly_salary_tiyin":null,"delivery_pay_tiyin":600000}`)
	if code != http.StatusOK {
		t.Fatalf("almashtirish: %d — %s", code, body)
	}
	got := decodeMember(t, body)
	if got.MonthlySalaryTiyin != nil || got.DeliveryPayTiyin == nil || *got.DeliveryPayTiyin != 600_000 {
		t.Errorf("holat: oylik=%v buyurtmadan=%v", got.MonthlySalaryTiyin, got.DeliveryPayTiyin)
	}
	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 600_000 {
		t.Errorf("almashgandan keyin haq %d, 600000 kutilgan", q.DeliveryFeeTiyin)
	}

	// Va teskarisi: buyurtmadan -> oylik. Yetkazish yana bepul.
	code, body = patchStaff(t, f, m.ID, `{"delivery_pay_tiyin":null,"monthly_salary_tiyin":350000000}`)
	if code != http.StatusOK {
		t.Fatalf("teskari almashtirish: %d — %s", code, body)
	}
	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 0 {
		t.Errorf("oylikka qaytgach haq %d, 0 kutilgan", q.DeliveryFeeTiyin)
	}
}

// ┌─ IKKALASI BIRGA BO'LMAYDI ────────────────────────────────────────┐
// Kuryer yo oylik oladi, yo buyurtmadan. Ikkalasi bo'lsa mijoz uchun
// haq nima bo'lishi noaniq: 0 mi yoki kuryer haqimi?
// └───────────────────────────────────────────────────────────────────┘
func TestPayModesAreMutuallyExclusive(t *testing.T) {
	f := staffServer(t)

	w := do(t, f.h, "POST", staffPathA, f.jwt["a"],
		courierBody("+998901110005", `"monthly_salary_tiyin":350000000,"delivery_pay_tiyin":800000`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("yaratishda ikkalasi: %d — %s", w.Code, w.Body.String())
	}

	m := createStaff(t, f, courierBody("+998901110006", `"delivery_pay_tiyin":800000`))
	code, body := patchStaff(t, f, m.ID, `{"monthly_salary_tiyin":350000000}`)
	if code != http.StatusBadRequest {
		t.Fatalf("tahrirlashda ikkalasi: %d — %s", code, body)
	}
	// Rad etilgan so'rov hech narsani o'zgartirmaydi.
	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 800_000 {
		t.Errorf("rad etilgandan keyin haq %d, 800000 kutilgan", q.DeliveryFeeTiyin)
	}
}

// "Buyurtmadan maosh" faqat yetkazib beruvchida.
func TestDeliveryPayOnlyForCouriers(t *testing.T) {
	f := staffServer(t)
	w := do(t, f.h, "POST", staffPathA, f.jwt["a"],
		`{"first_name":"Ali","phone":"+998901110007","position":"chef","delivery_pay_tiyin":800000}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("oshpaz uchun: %d — %s", w.Code, w.Body.String())
	}
}

// Lavozim kuryerdan boshqasiga o'tsa, "buyurtmadan maosh" ham ketadi:
// jimgina qolib, mijozning haqiga ta'sir qilishda davom etmasligi kerak.
func TestDeliveryPayClearedWhenNoLongerCourier(t *testing.T) {
	f := staffServer(t)
	m := createStaff(t, f, courierBody("+998901110008", `"delivery_pay_tiyin":800000`))
	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 800_000 {
		t.Fatalf("boshida haq %d", q.DeliveryFeeTiyin)
	}
	code, body := patchStaff(t, f, m.ID, `{"position":"chef"}`)
	if code != http.StatusOK {
		t.Fatalf("lavozim o'zgarishi: %d — %s", code, body)
	}
	if got := decodeMember(t, body); got.DeliveryPayTiyin != nil {
		t.Errorf("lavozim o'zgargach buyurtmadan maosh qoldi: %v", *got.DeliveryPayTiyin)
	}
	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 0 {
		t.Errorf("oshpazning maoshi mijozga ta'sir qildi: %d", q.DeliveryFeeTiyin)
	}
}

// Chegara: ortiqcha nol (8 000 o'rniga 800 000) rad etiladi.
func TestDeliveryPayUpperBound(t *testing.T) {
	f := staffServer(t)
	for name, v := range map[string]string{
		"juda katta": "10000001",
		"nol":        "0",
		"manfiy":     "-1",
	} {
		w := do(t, f.h, "POST", staffPathA, f.jwt["a"],
			courierBody("+998901110009", `"delivery_pay_tiyin":`+v))
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d kutilgan 400 — %s", name, w.Code, w.Body.String())
		}
	}
	// Aynan chegara o'tadi (100 000 so'm).
	w := do(t, f.h, "POST", staffPathA, f.jwt["a"],
		courierBody("+998901110010", `"delivery_pay_tiyin":10000000`))
	if w.Code != http.StatusCreated {
		t.Errorf("chegaraning o'zi: %d — %s", w.Code, w.Body.String())
	}
}

// Ta'tildagi kuryerning narxi mijozga ta'sir qilmaydi.
func TestDeliveryFeeIgnoresCourierOnLeave(t *testing.T) {
	f := staffServer(t)
	m := createStaff(t, f, courierBody("+998901110011", `"delivery_pay_tiyin":800000`))
	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 800_000 {
		t.Fatalf("boshida haq %d", q.DeliveryFeeTiyin)
	}
	if code := setStaffStatus(t, f, m.ID, "on_leave"); code != http.StatusOK {
		t.Fatalf("ta'tilga chiqarish: %d", code)
	}
	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 0 {
		t.Errorf("ta'tildagi kuryer haqi mijozga qo'shilmoqda: %d", q.DeliveryFeeTiyin)
	}
}

// Bir nechta kuryer — ENG YUQORI buyurtmadan maosh.
func TestDeliveryFeeTakesHighestPerOrderPay(t *testing.T) {
	f := staffServer(t)
	createStaff(t, f, courierBody("+998901110012", `"delivery_pay_tiyin":600000`))
	createStaff(t, f, courierBody("+998901110013", `"delivery_pay_tiyin":900000`))
	// Oylik kuryer natijani PASAYTIRMAYDI: u haqni belgilamaydi.
	createStaff(t, f, courierBody("+998901110014", `"monthly_salary_tiyin":350000000`))

	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 900_000 {
		t.Errorf("haq %d, 900000 (eng yuqorisi) kutilgan", q.DeliveryFeeTiyin)
	}
}

// ┌─ STOL BUYURTMASI HECH QACHON TO'LAMAYDI ──────────────────────────┐
// Ofitsiant stolga olib boradi, kuryer ishtirok etmaydi.
// └───────────────────────────────────────────────────────────────────┘
func TestDineInNeverPaysDelivery(t *testing.T) {
	f := staffServer(t)
	createStaff(t, f, courierBody("+998901110015", `"delivery_pay_tiyin":800000`))

	if q := quoteFor(t, f, true); q.DeliveryFeeTiyin != 0 || q.TotalTiyin != staffTestGoodsTiyin {
		t.Errorf("stolda: haq %d, jami %d", q.DeliveryFeeTiyin, q.TotalTiyin)
	}
	// Yetkazishda esa to'laydi.
	if q := quoteFor(t, f, false); q.DeliveryFeeTiyin != 800_000 {
		t.Errorf("yetkazishda: haq %d", q.DeliveryFeeTiyin)
	}
}

// Xodimlar ro'yxati mijoz HOZIR to'laydigan haqni ham beradi: panel
// "mijoz nimani ko'radi" deb ko'rsata olsin (qoida bitta joyda).
func TestStaffListReportsCustomerDeliveryFee(t *testing.T) {
	f := staffServer(t)
	get := func() int64 {
		w := do(t, f.h, "GET", staffPathA, f.jwt["a"], "")
		if w.Code != http.StatusOK {
			t.Fatalf("ro'yxat: %d — %s", w.Code, w.Body.String())
		}
		var out struct {
			Fee int64 `json:"customer_delivery_fee_tiyin"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.Fee
	}
	if got := get(); got != 0 {
		t.Errorf("kuryersiz: %d", got)
	}
	createStaff(t, f, courierBody("+998901110016", `"delivery_pay_tiyin":800000`))
	if got := get(); got != 800_000 {
		t.Errorf("kuryer bilan: %d, 800000 kutilgan", got)
	}
}

// ─── Sozlamalardan olib tashlangan ─────────────────────────────────────

// Yetkazish haqi endi "Xodimlar" da. Sozlamalar sahifasi uni
// o'zgartira olmaydi va BUNI ANIQ AYTADI (jimgina e'tiborsiz qoldirmaydi).
func TestSettingsNoLongerEditsDeliveryFee(t *testing.T) {
	h, _, jwt := settingsServer(t)
	for _, key := range []string{"delivery_fee_tiyin", "delivery_free_from_tiyin"} {
		w := do(t, h, "PATCH", settingsPathA, jwt["a"], `{"`+key+`":800000}`)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d kutilgan 400 — %s", key, w.Code, w.Body.String())
			continue
		}
		if !strings.Contains(w.Body.String(), "Xodimlar") {
			t.Errorf("%s: javob \"Xodimlar\" ga yo'naltirmaydi — %s", key, w.Body.String())
		}
	}
}

func TestSettingsViewHasNoDeliveryFields(t *testing.T) {
	h, _, jwt := settingsServer(t)
	w := do(t, h, "GET", settingsPathA, jwt["a"], "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET: %d", w.Code)
	}
	for _, key := range []string{"delivery_fee_tiyin", "delivery_free_from_tiyin", "delivery_fee_max_tiyin"} {
		if strings.Contains(w.Body.String(), key) {
			t.Errorf("sozlamalar javobida o'lik maydon %q qoldi: %s", key, w.Body.String())
		}
	}
	var s struct {
		Editable []string `json:"editable"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	for _, f := range s.Editable {
		if strings.HasPrefix(f, "delivery") {
			t.Errorf("tahrirlanadigan maydonlarda %q bor", f)
		}
	}
}
