package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// Restoranning O'Z yetkazib beruvchilari ("Xodimlar" -> "OnDex Kuryer").

// courierHasActiveOrder — `cmd/api/main.go` dagi `CourierBusy` bilan bir xil.
func courierHasActiveOrder(ctx context.Context, repo *storage.MemoryOrderRepo, courierID string) (bool, error) {
	_, err := repo.GetActiveByCourier(ctx, courierID)
	if errors.Is(err, orders.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// staffCourier — A restoraniga ilovaga kirishi ochiq yetkazib beruvchi
// qo'shadi; xodim, akkaunt, kuryer yozuvi va token qaytadi.
func staffCourier(t *testing.T, f staffFixture, phone string) (staffMemberView, *users.User, *couriers.Courier, string) {
	t.Helper()
	ctx := context.Background()
	m := createStaff(t, f, `{"first_name":"Jasur","last_name":"Karimov","phone":"`+phone+`","position":"courier","app_access":true}`)
	if !m.AppAccess || !m.AppAccessActive {
		t.Fatalf("kirish ochilmadi: %+v", m)
	}
	u, err := f.users.GetByPhone(ctx, phone)
	if err != nil {
		t.Fatal(err)
	}
	c, err := f.couriers.GetByID(ctx, u.EntityID)
	if err != nil {
		t.Fatalf("kuryer yozuvi yaratilmadi: %v", err)
	}
	tok, err := f.tokens.Issue(u)
	if err != nil {
		t.Fatal(err)
	}
	return m, u, c, tok
}

func setStaffStatus(t *testing.T, f staffFixture, id, status string) int {
	t.Helper()
	return do(t, f.h, "POST", staffPathA+"/"+id+"/status", f.jwt["a"], `{"status":"`+status+`"}`).Code
}

func TestStaffCourierAccountFollowsRecord(t *testing.T) {
	f := staffServer(t)
	ctx := context.Background()
	m, u, c, tok := staffCourier(t, f, "+998901112233")

	if u.Role != users.RoleCourier || c.RestaurantID != testRestA || !c.Approved || c.Available || c.Name != "Jasur Karimov" {
		t.Fatalf("akkaunt/kuryer noto'g'ri: %+v %+v", u, c)
	}
	path := "/couriers/" + c.ID
	if w := do(t, f.h, "GET", path, tok, ""); w.Code != http.StatusOK {
		t.Fatalf("kuryer o'z holatini ko'ra olmadi: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, f.h, "POST", path+"/available", tok, `{"available":true}`); w.Code != http.StatusOK {
		t.Fatalf("onlayn bo'la olmadi: %d %s", w.Code, w.Body.String())
	}

	// Ism tahrirlansa kuryer yozuvi ham yangilanadi.
	if w := do(t, f.h, "PATCH", staffPathA+"/"+m.ID, f.jwt["a"], `{"first_name":"Sardor"}`); w.Code != http.StatusOK {
		t.Fatalf("tahrir: %d %s", w.Code, w.Body.String())
	}
	if cur, _ := f.couriers.GetByID(ctx, c.ID); cur.Name != "Sardor Karimov" {
		t.Fatalf("kuryer ismi yangilanmadi: %q", cur.Name)
	}

	// Ta'til: dispatch havuzidan chiqadi, rol mijozga qaytadi, sessiya yopiladi.
	if code := setStaffStatus(t, f, m.ID, "on_leave"); code != http.StatusOK {
		t.Fatalf("ta'til: %d", code)
	}
	cur, _ := f.couriers.GetByID(ctx, c.ID)
	acc, _ := f.users.GetByID(ctx, u.ID)
	if cur.Approved || cur.Available || acc.Role != users.RoleCustomer || acc.EntityID != "" {
		t.Fatalf("ta'tilda kirish yopilmadi: kuryer=%+v akkaunt=%+v", cur, acc)
	}
	// Sessiya bekor qilindi. `revoke` soniya aniqligida ishlaydi va AYNAN
	// shu soniyada chiqarilgan token ataylab yaroqli qoladi
	// (`revoke.IsRevoked` izohi) — shuning uchun avvalroq chiqarilgan
	// token bilan tekshiriladi (affitsiant testi bilan bir xil).
	if !f.revoked.IsRevoked(u.ID, time.Now().Add(-time.Minute)) {
		t.Fatal("ta'tilda kuryer sessiyalari bekor qilinmadi")
	}

	// Ishga qaytdi — AYNAN o'sha kuryer yozuvi (yetkazmalar tarixi uzilmaydi).
	if code := setStaffStatus(t, f, m.ID, "active"); code != http.StatusOK {
		t.Fatalf("qayta faollashtirish: %d", code)
	}
	cur, _ = f.couriers.GetByID(ctx, c.ID)
	acc, _ = f.users.GetByID(ctx, u.ID)
	if !cur.Approved || acc.Role != users.RoleCourier || acc.EntityID != c.ID {
		t.Fatalf("qayta ochilmadi: kuryer=%+v akkaunt=%+v", cur, acc)
	}
	if all, _ := f.couriers.ListAll(ctx); len(all) != 1 {
		t.Fatalf("yangi kuryer yozuvi yaratilmasligi kerak edi: %d ta", len(all))
	}

	// Ishdan bo'shatildi — butunlay yopiladi.
	if code := setStaffStatus(t, f, m.ID, "dismissed"); code != http.StatusOK {
		t.Fatalf("ishdan bo'shatish: %d", code)
	}
	if cur, _ := f.couriers.GetByID(ctx, c.ID); cur.Approved {
		t.Fatal("ishdan bo'shagan yetkazib beruvchi dispatch havuzida qoldi")
	}
}

func TestStaffCourierBusyBlocksClosing(t *testing.T) {
	f := staffServer(t)
	ctx := context.Background()
	m, u, c, tok := staffCourier(t, f, "+998901112234")
	if err := f.orders.Save(ctx, &orders.Order{
		ID: "ord-busy", CustomerID: "u-c", RestaurantID: testRestA, CourierID: c.ID,
		Status: orders.StatusPickedUp, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	for _, st := range []string{"dismissed", "on_leave"} {
		if code := setStaffStatus(t, f, m.ID, st); code != http.StatusConflict {
			t.Fatalf("%s: yetkazma o'rtasida 409 kutilgan, keldi %d", st, code)
		}
	}
	for _, body := range []string{`{"app_access":false}`, `{"position":"chef"}`} {
		if w := do(t, f.h, "PATCH", staffPathA+"/"+m.ID, f.jwt["a"], body); w.Code != http.StatusConflict {
			t.Fatalf("%s: 409 kutilgan, keldi %d %s", body, w.Code, w.Body.String())
		}
	}
	cur, _ := f.couriers.GetByID(ctx, c.ID)
	acc, _ := f.users.GetByID(ctx, u.ID)
	if !cur.Approved || acc.Role != users.RoleCourier {
		t.Fatalf("rad etilgan amal kuryerni baribir yopib qo'ydi: %+v %+v", cur, acc)
	}

	// Kuryerning o'zi ham yetkazma o'rtasida oflayn bo'la olmaydi.
	if w := do(t, f.h, "POST", "/couriers/"+c.ID+"/available", tok, `{"available":false}`); w.Code != http.StatusConflict {
		t.Fatalf("oflayn: 409 kutilgan, keldi %d %s", w.Code, w.Body.String())
	}

	// Ilova qayta ochilganda joriy buyurtma tiklanadi.
	w := do(t, f.h, "GET", "/couriers/"+c.ID+"/active-order", tok, "")
	var got struct {
		OrderID *string `json:"order_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || w.Code != http.StatusOK ||
		got.OrderID == nil || *got.OrderID != "ord-busy" {
		t.Fatalf("joriy buyurtma: %d %s", w.Code, w.Body.String())
	}
}

func TestCourierActiveOrderEmptyAndForeign(t *testing.T) {
	f := staffServer(t)
	_, _, c, tok := staffCourier(t, f, "+998901112235")
	_, _, other, _ := staffCourier(t, f, "+998901112236")

	w := do(t, f.h, "GET", "/couriers/"+c.ID+"/active-order", tok, "")
	if w.Code != http.StatusOK || w.Body.String() == "" {
		t.Fatalf("bo'sh holat: %d %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got["order_id"] != nil {
		t.Fatalf("buyurtma yo'q bo'lsa order_id null bo'lishi kerak: %s", w.Body.String())
	}
	if w := do(t, f.h, "GET", "/couriers/"+other.ID+"/active-order", tok, ""); w.Code != http.StatusForbidden {
		t.Fatalf("begona kuryer buyurtmasi: 403 kutilgan, keldi %d", w.Code)
	}
}

// Begona akkaunt (mijoz) raqamiga kuryer kirishi ochilmaydi va kuryer
// yozuvi ham yaratilmaydi.
func TestStaffCourierForeignAccountConflict(t *testing.T) {
	f := staffServer(t)
	w := do(t, f.h, "POST", staffPathA, f.jwt["a"],
		`{"first_name":"Begona","phone":"+998900000075","position":"courier","app_access":true}`)
	if w.Code != http.StatusConflict {
		t.Fatalf("409 kutilgan, keldi %d %s", w.Code, w.Body.String())
	}
	if all, _ := f.couriers.ListAll(context.Background()); len(all) != 0 {
		t.Fatalf("rad etilgan xodim uchun kuryer yozuvi qoldi: %d ta", len(all))
	}
	if u, _ := f.users.GetByID(context.Background(), "u-c"); u.Role != users.RoleCustomer {
		t.Fatalf("begona mijoz akkaunti o'zgartirildi: %+v", u)
	}
}

// OnDex platforma kuryerlari hozircha yopiq; admin restoran kuryerini
// tasdiqlash/bloklash orqali boshqara olmaydi (yagona manba — xodim yozuvi).
func TestPlatformCourierPathsClosed(t *testing.T) {
	f := staffServer(t)
	_, _, c, _ := staffCourier(t, f, "+998901112237")

	if w := do(t, f.h, "POST", "/couriers/register", f.jwt["customer"], `{"name":"Ali","vehicle_type":"moped"}`); w.Code != http.StatusGone {
		t.Fatalf("ro'yxatdan o'tish yopiq bo'lishi kerak: %d %s", w.Code, w.Body.String())
	}
	if u, _ := f.users.GetByID(context.Background(), "u-c"); u.Role != users.RoleCustomer {
		t.Fatal("yopiq ro'yxatdan o'tish baribir rolni o'zgartirdi")
	}
	for _, body := range []string{`{"approved":true}`, `{"approved":false}`} {
		if w := do(t, f.h, "POST", "/admin/couriers/"+c.ID+"/approve", f.jwt["admin"], body); w.Code != http.StatusConflict {
			t.Fatalf("%s: 409 kutilgan, keldi %d %s", body, w.Code, w.Body.String())
		}
	}
	if cur, _ := f.couriers.GetByID(context.Background(), c.ID); !cur.Approved {
		t.Fatal("admin so'rovi restoran kuryerining tasdig'ini o'zgartirdi")
	}
}
