package httpapi

import (
	"context"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"chustapp/internal/notify"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// `GET /ws` obuna kalitlarining AVTORIZATSIYA testlari (bug.md 29-band).
//
// ┌─ NEGA AYNAN SHU DARAJADA ──────────────────────────────────────────┐
// Nosozlik WebSocket ramkalarida emas, KALIT HISOBLASHDA edi:
// `?restaurant_id=` so'rov parametri hech qanday tekshiruvsiz XODIM
// kanaliga obuna qilardi. Ulanishni haqiqatan ko'tarib, xabar kutib
// o'tirish bu qarorni sinash uchun kerak emas va testni sekin qiladi —
// shuning uchun kalit hisoblash sof funksiyaga ajratilgan va aynan
// shu yerda sinaladi.
// └────────────────────────────────────────────────────────────────────┘

func wsTestServer(t *testing.T) *Server {
	t.Helper()
	orderRepo := storage.NewMemoryOrderRepo()
	// Mijoz "u-cust" ning buyurtmasi — egalik tekshiruvini o'lchash uchun.
	if err := orderRepo.Save(context.Background(), &orders.Order{
		ID:           "ord-1",
		CustomerID:   "u-cust",
		RestaurantID: "rest-a",
		CreatedAt:    time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	return New(Deps{
		OrderRepo: orderRepo,
		UserRepo:  storage.NewMemoryUserRepo(),
		Tokens:    users.NewTokenIssuer("test-secret", time.Hour),
		DevMode:   true,
	})
}

func wsKeys(t *testing.T, s *Server, query, userID, entityID, role string) []string {
	t.Helper()
	r := httptest.NewRequest("GET", "/ws"+query, nil)
	return s.wsSubscriptionKeys(r, userID, entityID, role)
}

// Asosiy regressiya: begona mijoz `?restaurant_id=` bilan restoranning
// XODIM kanaliga (yangi buyurtma, summa, stol, kuryer) obuna bo'la
// olmasligi kerak.
func TestWsKeys_RestaurantParamNeverGivesStaffChannel(t *testing.T) {
	s := wsTestServer(t)

	keys := wsKeys(t, s, "?restaurant_id=rest-a", "u-cust", "", string(users.RoleCustomer))

	staff := notify.Entity(notify.ModuleFood, "rest-a")
	if slices.Contains(keys, staff) {
		t.Fatalf("mijoz XODIM kanaliga obuna bo'ldi: %q kalitlar ichida (%v)", staff, keys)
	}
	pub := notify.PublicEntity(notify.ModuleFood, "rest-a")
	if !slices.Contains(keys, pub) {
		t.Fatalf("ochiq menyu kanali yo'q: %q kutilgan, olingan %v", pub, keys)
	}
}

// Xuddi shu teshik KURYER uchun ham ochiq edi: kuryer ID'si buyurtma
// javoblarida ko'rinadi va u ham `Entity(food, ...)` fazosida yashaydi,
// ya'ni `?restaurant_id=<kuryer ID>` uning taklif oqimini (restoran
// manzili va KOORDINATASI) ochib berardi.
func TestWsKeys_RestaurantParamNeverGivesCourierChannel(t *testing.T) {
	s := wsTestServer(t)

	keys := wsKeys(t, s, "?restaurant_id=courier-7", "u-cust", "", string(users.RoleCustomer))

	if courier := notify.Entity(notify.ModuleFood, "courier-7"); slices.Contains(keys, courier) {
		t.Fatalf("mijoz kuryer kanaliga obuna bo'ldi: %v", keys)
	}
}

// Xodimning O'Z kanali tokendagi EntityID dan keladi — bu ishlashda
// davom etishi kerak (aks holda restoran paneli jonli buyurtma
// olmay qoladi).
func TestWsKeys_StaffChannelComesFromToken(t *testing.T) {
	s := wsTestServer(t)

	keys := wsKeys(t, s, "", "u-rest", "rest-a", string(users.RoleRestaurant))

	if staff := notify.Entity(notify.ModuleFood, "rest-a"); !slices.Contains(keys, staff) {
		t.Fatalf("restoran o'z xodim kanaliga obuna bo'lmadi: %v", keys)
	}
}

// `?order_id=` — kuryerning jonli GPS'i. Faqat buyurtma EGASI.
func TestWsKeys_OrderTopicOnlyForOwner(t *testing.T) {
	s := wsTestServer(t)
	topic := notify.Order("ord-1")

	owner := wsKeys(t, s, "?order_id=ord-1", "u-cust", "", string(users.RoleCustomer))
	if !slices.Contains(owner, topic) {
		t.Fatalf("egasi o'z buyurtmasini kuzata olmadi: %v", owner)
	}

	stranger := wsKeys(t, s, "?order_id=ord-1", "u-other", "", string(users.RoleCustomer))
	if slices.Contains(stranger, topic) {
		t.Fatalf("begona odam buyurtma kanaliga obuna bo'ldi: %v", stranger)
	}

	missing := wsKeys(t, s, "?order_id=yo-q", "u-cust", "", string(users.RoleCustomer))
	if slices.Contains(missing, notify.Order("yo-q")) {
		t.Fatalf("mavjud bo'lmagan buyurtma kanaliga obuna bo'ldi: %v", missing)
	}
}

// Ma'muriyat kanali — FAQAT admin roliga. Rol server manbasidan
// keladi, lekin mijoz roli bilan kelgan ulanish uni olmasligi kerak.
func TestWsKeys_AdminTopicOnlyForAdmin(t *testing.T) {
	s := wsTestServer(t)

	admin := wsKeys(t, s, "", "u-admin", "", string(users.RoleAdmin))
	if !slices.Contains(admin, notify.Admin()) {
		t.Fatalf("admin o'z kanalini olmadi: %v", admin)
	}

	for _, role := range []users.Role{
		users.RoleCustomer, users.RoleRestaurant, users.RoleCourier, users.RoleWaiter,
	} {
		keys := wsKeys(t, s, "", "u-x", "rest-a", string(role))
		if slices.Contains(keys, notify.Admin()) {
			t.Fatalf("%s roli ma'muriyat kanalini oldi: %v", role, keys)
		}
	}
}

// Ochiq va xodim kanallari HECH QACHON kesishmasligi kerak — aks holda
// ajratishning ma'nosi yo'q.
func TestPublicEntityNeverCollidesWithEntity(t *testing.T) {
	for _, id := range []string{"rest-a", "courier-7", "x"} {
		if notify.Entity(notify.ModuleFood, id) == notify.PublicEntity(notify.ModuleFood, id) {
			t.Fatalf("kalitlar bir xil: %q", id)
		}
	}
	// Bo'sh ID — kalit ham bo'sh (hub uni tashlab yuboradi).
	if notify.PublicEntity(notify.ModuleFood, "") != "" {
		t.Fatal("bo'sh ID bo'sh bo'lmagan kalit berdi")
	}
}
