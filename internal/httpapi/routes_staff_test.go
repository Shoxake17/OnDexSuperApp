package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/revoke"
	"chustapp/internal/staff"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

type staffFixture struct {
	h        http.Handler
	jwt      map[string]string
	users    *storage.MemoryUserRepo
	revoked  *revoke.Store
	couriers *storage.MemoryCourierRepo
	orders   *storage.MemoryOrderRepo
	tokens   *users.TokenIssuer
}

// staffServer — ikki restoran; tokenlar: "a", "b" (restoran), "admin",
// "waiter", "customer". Mijoz raqami +998900000075 — begona akkaunt.
func staffServer(t *testing.T) staffFixture {
	t.Helper()
	ctx := context.Background()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	revoked := revoke.New(nil, time.Hour)
	jwt := map[string]string{}
	mk := func(key, id string, role users.Role, entityID, phone string) {
		u := &users.User{ID: id, Phone: phone, Role: role, EntityID: entityID,
			PhoneVerified: true, CreatedAt: time.Now()}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		tok, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		jwt[key] = tok
	}
	mk("a", "u-a", users.RoleRestaurant, testRestA, "+998900000071")
	mk("b", "u-b", users.RoleRestaurant, testRestB, "+998900000072")
	mk("admin", "u-admin", users.RoleAdmin, "", "+998900000073")
	mk("waiter", "u-w", users.RoleWaiter, testRestA, "+998900000074")
	mk("customer", "u-c", users.RoleCustomer, "", "+998900000075")

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{{ID: testRestA, Name: "Book Cafe", Open: true}, {ID: testRestB, Name: "B", Open: true}}, nil)
	courierRepo := storage.NewMemoryCourierRepo()
	orderRepo := storage.NewMemoryOrderRepo()
	svc := staff.NewService(storage.NewMemoryStaffRepo(),
		&staff.UserAccounts{Users: userRepo, Couriers: courierRepo, Revoked: revoked, NewID: NewID,
			CourierBusy: func(ctx context.Context, courierID string) (bool, error) {
				return courierHasActiveOrder(ctx, orderRepo, courierID)
			}})
	h := New(Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      orderRepo,
		OrderSvc:       orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo()),
		CourierRepo:    courierRepo,
		Tokens:         tokens,
		Revoked:        revoked,
		StaffSvc:       svc,
		Hub:            ws.NewHub(nil),
		DevMode:        true,
	}).Routes(nil)
	return staffFixture{h: h, jwt: jwt, users: userRepo, revoked: revoked,
		couriers: courierRepo, orders: orderRepo, tokens: tokens}
}

const staffPathA = "/restaurants/" + testRestA + "/staff"

func decodeMember(t *testing.T, body string) staffMemberView {
	t.Helper()
	var v staffMemberView
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatalf("javob: %v — %s", err, body)
	}
	return v
}

func createStaff(t *testing.T, f staffFixture, body string) staffMemberView {
	t.Helper()
	w := do(t, f.h, "POST", staffPathA, f.jwt["a"], body)
	if w.Code != http.StatusCreated {
		t.Fatalf("yaratish: %d — %s", w.Code, w.Body.String())
	}
	return decodeMember(t, w.Body.String())
}

func TestStaffTenancyAndRoles(t *testing.T) {
	f := staffServer(t)
	createStaff(t, f, `{"first_name":"Azizbek","phone":"+998901110001","position":"chef"}`)

	for _, c := range []struct {
		token string
		want  int
	}{
		{f.jwt["b"], http.StatusNotFound},
		{f.jwt["waiter"], http.StatusForbidden},
		{f.jwt["customer"], http.StatusForbidden},
		{"", http.StatusUnauthorized},
		{f.jwt["admin"], http.StatusOK},
		{f.jwt["a"], http.StatusOK},
	} {
		if w := do(t, f.h, "GET", staffPathA, c.token, ""); w.Code != c.want {
			t.Errorf("GET: kutilgan %d, keldi %d", c.want, w.Code)
		}
	}
	// B restorani A ning xodimini o'zgartira olmaydi — hatto ID'ni bilsa ham.
	list := do(t, f.h, "GET", staffPathA, f.jwt["a"], "")
	var ov struct {
		Items []staffMemberView `json:"items"`
	}
	_ = json.Unmarshal(list.Body.Bytes(), &ov)
	id := ov.Items[0].ID
	pathB := "/restaurants/" + testRestB + "/staff/" + id
	if w := do(t, f.h, "PATCH", pathB, f.jwt["b"], `{"note":"x"}`); w.Code != http.StatusNotFound {
		t.Fatalf("begona restoran xodimi topilmasligi kerak: %d", w.Code)
	}
	if w := do(t, f.h, "PATCH", staffPathA+"/"+id, f.jwt["b"], `{"note":"x"}`); w.Code != http.StatusNotFound {
		t.Fatalf("begona restoran yo'li: %d", w.Code)
	}
	if strings.Contains(list.Body.String(), "user_id") || strings.Contains(list.Body.String(), "u-") {
		t.Fatalf("javobda akkaunt ID'si bor: %s", list.Body.String())
	}
}

func TestStaffCreateValidation(t *testing.T) {
	f := staffServer(t)
	for _, c := range []struct {
		body string
		want int
	}{
		{`{"phone":"+998901110001","position":"chef"}`, http.StatusBadRequest},
		{`{"first_name":"Ali","phone":"12345","position":"chef"}`, http.StatusBadRequest},
		{`{"first_name":"Ali","phone":"+998901110001","position":"povar"}`, http.StatusBadRequest},
		{`{"first_name":"Ali","phone":"+998901110001","position":"chef","role":"admin"}`, http.StatusBadRequest},
		{`{"first_name":"Ali","phone":"+998901110001","position":"chef","app_access":true}`, http.StatusBadRequest},
		{`{"first_name":"Ali1","phone":"+998901110001","position":"chef"}`, http.StatusBadRequest},
		{`{"first_name":"<script>","phone":"+998901110001","position":"chef"}`, http.StatusBadRequest},
		{`{"first_name":"Ali","phone":"+998901110001","position":"chef","schedule":{"days":[],"start":"08:00","end":"22:00"}}`, http.StatusBadRequest},
		{`{"first_name":"Ali","phone":"+998901110001","position":"chef","schedule":{"days":[1,8],"start":"08:00","end":"22:00"}}`, http.StatusBadRequest},
		{`{"first_name":"Ali","phone":"+998901110001","position":"chef","schedule":{"days":[1],"start":"8:00","end":"22:00"}}`, http.StatusBadRequest},
		{`{"first_name":"Ali","phone":"+998901110001","position":"chef","hired_on":"01.09.2026"}`, http.StatusBadRequest},
		{`{"first_name":"Ali","phone":"+998901110001","position":"chef","note":"` + strings.Repeat("a", 301) + `"}`, http.StatusBadRequest},
	} {
		if w := do(t, f.h, "POST", staffPathA, f.jwt["a"], c.body); w.Code != c.want {
			t.Errorf("%.90s: kutilgan %d, keldi %d — %s", c.body, c.want, w.Code, w.Body.String())
		}
	}

	m1 := createStaff(t, f, `{"first_name":" Malika ","last_name":"To'xtayeva","phone":"+998 90 111 00 02","position":"cashier",`+
		`"schedule":{"days":[5,1,3],"start":"10:00","end":"23:00"},"hired_on":"2026-09-01","note":"Kechki smena"}`)
	if m1.Code != "EMP001" || m1.FullName != "Malika To'xtayeva" || m1.Phone != "+998901110002" ||
		m1.PositionTitle != "Kassir" || m1.Status != "active" || m1.Schedule == nil ||
		len(m1.Schedule.Days) != 3 || m1.Schedule.Days[0] != 1 || m1.HiredOn == nil || *m1.HiredOn != "2026-09-01" {
		t.Fatalf("yaratilgan xodim: %+v", m1)
	}
	m2 := createStaff(t, f, `{"first_name":"Sardor","phone":"+998901110003","position":"cleaner"}`)
	if m2.Code != "EMP002" {
		t.Fatalf("tartib raqami: %s", m2.Code)
	}
	if w := do(t, f.h, "POST", staffPathA, f.jwt["a"],
		`{"first_name":"Boshqa","phone":"+998901110002","position":"chef"}`); w.Code != http.StatusConflict {
		t.Fatalf("takror telefon: %d", w.Code)
	}
}

func TestStaffWaiterAccessFollowsRecord(t *testing.T) {
	f := staffServer(t)
	ctx := context.Background()
	m := createStaff(t, f, `{"first_name":"Jasurbek","phone":"+998901110010","position":"waiter","app_access":true}`)
	if !m.AppAccess || !m.AppAccessActive {
		t.Fatalf("kirish ochilmadi: %+v", m)
	}
	acc, err := f.users.GetByPhone(ctx, "+998901110010")
	if err != nil || acc.Role != users.RoleWaiter || acc.EntityID != testRestA {
		t.Fatalf("akkaunt: %+v %v", acc, err)
	}
	itemPath := staffPathA + "/" + m.ID

	status := func(st string) staffMemberView {
		w := do(t, f.h, "POST", itemPath+"/status", f.jwt["a"], `{"status":"`+st+`"}`)
		if w.Code != http.StatusOK {
			t.Fatalf("holat %s: %d — %s", st, w.Code, w.Body.String())
		}
		return decodeMember(t, w.Body.String())
	}
	roleOf := func() users.Role {
		u, err := f.users.GetByID(ctx, acc.ID)
		if err != nil {
			t.Fatal(err)
		}
		return u.Role
	}

	// Ta'til — kirish yopiladi, sessiya bekor.
	if v := status("on_leave"); v.AppAccessActive || !v.AppAccess {
		t.Fatalf("ta'tilda: %+v", v)
	}
	if roleOf() != users.RoleCustomer || !f.revoked.IsRevoked(acc.ID, time.Now().Add(-time.Minute)) {
		t.Fatal("ta'tilda akkaunt yopilmadi yoki sessiya bekor qilinmadi")
	}
	// Qaytdi — o'sha akkaunt qayta ochiladi (yangisi yaratilmaydi).
	if v := status("active"); !v.AppAccessActive {
		t.Fatalf("qaytganda: %+v", v)
	}
	if roleOf() != users.RoleWaiter {
		t.Fatal("qaytganda kirish ochilmadi")
	}
	// Ilovali BOSHQA lavozimga (yetkazib beruvchi) o'tdi — o'sha akkaunt
	// QAYTA bog'lanadi: affitsiant roli yopiladi, kuryer roli ochiladi.
	w := do(t, f.h, "PATCH", itemPath, f.jwt["a"], `{"position":"courier"}`)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if v := decodeMember(t, w.Body.String()); !v.AppAccess || !v.AppAccessActive || roleOf() != users.RoleCourier {
		t.Fatalf("yetkazib beruvchiga o'tganda: %+v rol=%s", v, roleOf())
	}
	// Ilovasiz lavozim — kirish ham, tanlov ham o'chadi.
	w = do(t, f.h, "PATCH", itemPath, f.jwt["a"], `{"position":"cashier"}`)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if v := decodeMember(t, w.Body.String()); v.AppAccess || v.AppAccessActive || roleOf() != users.RoleCustomer {
		t.Fatalf("lavozim o'zgarganda: %+v rol=%s", v, roleOf())
	}
	// Ilovasiz lavozimga kirish ochib bo'lmaydi.
	if w := do(t, f.h, "PATCH", itemPath, f.jwt["a"], `{"app_access":true}`); w.Code != http.StatusBadRequest {
		t.Fatalf("kassirga ilova: %d", w.Code)
	}

	// Begona akkaunt (mijoz) raqami — egallab bo'lmaydi, yozuv ham yaratilmaydi.
	if w := do(t, f.h, "POST", staffPathA, f.jwt["a"],
		`{"first_name":"Begona","phone":"+998900000075","position":"waiter","app_access":true}`); w.Code != http.StatusConflict {
		t.Fatalf("begona akkaunt: %d — %s", w.Code, w.Body.String())
	}
	if u, _ := f.users.GetByID(ctx, "u-c"); u.Role != users.RoleCustomer {
		t.Fatal("XAVFSIZLIK: mijoz akkaunti affitsiantga aylantirildi")
	}
	// Boshqa restoranning affitsianti ham.
	if w := do(t, f.h, "POST", "/restaurants/"+testRestB+"/staff", f.jwt["b"],
		`{"first_name":"Jasur","phone":"+998900000074","position":"waiter","app_access":true}`); w.Code != http.StatusConflict {
		t.Fatalf("boshqa restoran affitsianti: %d", w.Code)
	}
	// Kirishsiz qo'shish — mumkin (akkaunt yaratilmaydi).
	createStaff(t, f, `{"first_name":"Kirishsiz","phone":"+998901110011","position":"waiter"}`)
	if _, err := f.users.GetByPhone(ctx, "+998901110011"); err == nil {
		t.Fatal("kirish so'ralmagan bo'lsa akkaunt yaratilmasligi kerak")
	}
}

func TestStaffSalaryAndReport(t *testing.T) {
	f := staffServer(t)
	for _, body := range []string{
		`{"first_name":"Ali","phone":"+998901110031","position":"chef","monthly_salary_tiyin":0}`,
		`{"first_name":"Ali","phone":"+998901110031","position":"chef","monthly_salary_tiyin":-5}`,
		`{"first_name":"Ali","phone":"+998901110031","position":"chef","monthly_salary_tiyin":"3500000"}`,
		`{"first_name":"Ali","phone":"+998901110031","position":"chef","monthly_salary_tiyin":100000000001}`,
	} {
		if w := do(t, f.h, "POST", staffPathA, f.jwt["a"], body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
	m := createStaff(t, f, `{"first_name":"Ali","phone":"+998901110031","position":"chef","monthly_salary_tiyin":350000000,`+
		`"schedule":{"days":[1,2,3,4,5,6],"start":"09:00","end":"18:00"}}`)
	if m.MonthlySalaryTiyin == nil || *m.MonthlySalaryTiyin != 350000000 {
		t.Fatalf("maosh: %+v", m)
	}
	item := staffPathA + "/" + m.ID
	if w := do(t, f.h, "PATCH", item, f.jwt["a"], `{"monthly_salary_tiyin":null}`); w.Code != http.StatusOK ||
		decodeMember(t, w.Body.String()).MonthlySalaryTiyin != nil {
		t.Fatalf("maoshni olib tashlash: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, f.h, "PATCH", item, f.jwt["a"], `{"monthly_salary_tiyin":420000000}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	// Faoliyatda maosh QIYMATI yo'q.
	act := do(t, f.h, "GET", item+"/activity", f.jwt["a"], "")
	if !strings.Contains(act.Body.String(), "salary_changed") || strings.Contains(act.Body.String(), "420000000") {
		t.Fatalf("faoliyat: %s", act.Body.String())
	}

	for _, c := range []struct {
		path  string
		token string
		want  int
	}{
		{staffPathA + "/report?month=2026-13", f.jwt["a"], http.StatusBadRequest},
		{staffPathA + "/report?month=1999-01", f.jwt["a"], http.StatusBadRequest},
		{"/restaurants/" + testRestA + "/staff/report", f.jwt["b"], http.StatusNotFound},
		{"/restaurants/" + testRestA + "/staff/report", f.jwt["waiter"], http.StatusForbidden},
	} {
		if w := do(t, f.h, "GET", c.path, c.token, ""); w.Code != c.want {
			t.Errorf("%s: kutilgan %d, keldi %d", c.path, c.want, w.Code)
		}
	}
	w := do(t, f.h, "GET", staffPathA+"/report", f.jwt["a"], "")
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	var rep struct {
		Month       string `json:"month"`
		DaysInMonth int    `json:"days_in_month"`
		Items       []struct {
			ID      string `json:"id"`
			Days    string `json:"days"`
			Salary  *int64 `json:"monthly_salary_tiyin"`
			Accrued *int64 `json:"accrued_salary_tiyin"`
		} `json:"items"`
		Totals staff.ReportTotals `json:"totals"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.Items) != 1 || rep.Items[0].ID != m.ID || len(rep.Items[0].Days) != rep.DaysInMonth ||
		rep.Items[0].Salary == nil || rep.Items[0].Accrued == nil || rep.Totals.Members != 1 {
		t.Fatalf("hisobot: %s", w.Body.String())
	}
}

func TestStaffSummaryActivityAndPatch(t *testing.T) {
	f := staffServer(t)
	a := createStaff(t, f, `{"first_name":"A","phone":"+998901110021","position":"chef","schedule":{"days":[1,2,3,4,5,6,7],"start":"08:00","end":"22:00"}}`)
	b := createStaff(t, f, `{"first_name":"B","phone":"+998901110022","position":"waiter"}`)
	c := createStaff(t, f, `{"first_name":"C","phone":"+998901110023","position":"chef"}`)
	_ = a
	if w := do(t, f.h, "POST", staffPathA+"/"+b.ID+"/status", f.jwt["a"], `{"status":"on_leave"}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if w := do(t, f.h, "POST", staffPathA+"/"+c.ID+"/status", f.jwt["a"], `{"status":"dismissed"}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if w := do(t, f.h, "POST", staffPathA+"/"+c.ID+"/status", f.jwt["a"], `{"status":"fired"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("noma'lum holat: %d", w.Code)
	}

	w := do(t, f.h, "GET", staffPathA, f.jwt["a"], "")
	var ov struct {
		Items   []staffMemberView `json:"items"`
		Summary staff.Summary     `json:"summary"`
		Recent  []staffEventView  `json:"recent_activity"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &ov); err != nil {
		t.Fatal(err)
	}
	s := ov.Summary
	if s.Total != 3 || s.Active != 1 || s.OnLeave != 1 || s.Dismissed != 1 || s.WorkingToday != 1 ||
		s.TotalWeekDelta != 3 || s.ActiveWeekDelta != 1 || s.DismissedWeekDelta != 1 ||
		s.ByPosition[staff.PositionChef] != 1 || s.ByPosition[staff.PositionWaiter] != 1 {
		t.Fatalf("summary: %+v", s)
	}
	if len(ov.Recent) != 5 || ov.Recent[0].Kind != "status_changed" || ov.Recent[0].ToTitle != "Ishdan bo'shagan" ||
		ov.Recent[0].MemberName != "C" {
		t.Fatalf("so'nggi faoliyat: %+v", ov.Recent)
	}

	// PATCH: bo'sh, noma'lum, holat — rad; jadvalni olib tashlash — mumkin.
	itemA := staffPathA + "/" + a.ID
	for _, body := range []string{`{}`, `{"status":"dismissed"}`, `{"number":7}`, `{"schedule":{"days":[1],"start":"08:00","end":"22:00","x":1}}`, `[]`} {
		if w := do(t, f.h, "PATCH", itemA, f.jwt["a"], body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", body, w.Code)
		}
	}
	w = do(t, f.h, "PATCH", itemA, f.jwt["a"], `{"schedule":null,"last_name":"Karimov","hired_on":null}`)
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if v := decodeMember(t, w.Body.String()); v.Schedule != nil || v.LastName != "Karimov" || v.Code != "EMP001" {
		t.Fatalf("patch: %+v", v)
	}
	// Ishdan bo'shagan C ning raqami bo'shadi — yangi xodimga berish mumkin.
	createStaff(t, f, `{"first_name":"D","phone":"+998901110023","position":"cleaner"}`)
	// C ni qayta ishga olish — raqam band.
	if w := do(t, f.h, "POST", staffPathA+"/"+c.ID+"/status", f.jwt["a"], `{"status":"active"}`); w.Code != http.StatusConflict {
		t.Fatalf("band raqam bilan qayta ishga olish: %d", w.Code)
	}

	act := do(t, f.h, "GET", itemA+"/activity", f.jwt["a"], "")
	if act.Code != http.StatusOK || !strings.Contains(act.Body.String(), "schedule_changed") {
		t.Fatalf("faoliyat: %d %s", act.Code, act.Body.String())
	}
	if w := do(t, f.h, "GET", "/restaurants/"+testRestB+"/staff/"+a.ID+"/activity", f.jwt["b"], ""); w.Code != http.StatusNotFound {
		t.Fatalf("begona faoliyat: %d", w.Code)
	}
}
