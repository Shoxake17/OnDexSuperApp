package httpapi

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/orders"
	"chustapp/internal/revoke"
	"chustapp/internal/storage"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// ┌─ NEGA BU TEST BOR ──────────────────────────────────────────────────┐
// Har bir himoyalangan endpoint uchun alohida test yozilsa, YANGI
// endpoint qo'shilganda uni testsiz qoldirib ketish juda oson — va
// aynan o'sha unutilgan endpoint orqali begona rol ma'lumotga kiradi.
//
// Bu yerda TESKARI yondashuv: endpointlar ro'yxati va ularning rollari
// MANBA KODDAN o'qiladi (`s.auth([]users.Role{...})` e'lonlaridan), so'ng
// har biri BARCHA beshta rol bilan uriladi. Ruxsat etilmagan rol
// 401/403 olishi SHART.
//
// Qo'lda yozilgan jadval ATAYLAB YO'Q. Birinchi variantda u bor edi va
// darhol eskirdi: 61 ta himoyalangan endpointdan atigi 26 tasi
// jadvalga tushgan, qolgan 35 tasi tekshirilmay qolgan edi.
//
// Test "ishlaydi"ni emas, "ishlamasligi kerak"ni tekshiradi —
// spetsifikatsiyadagi INVARIANT 7.1 talabi.
// └─────────────────────────────────────────────────────────────────────┘

// authRoute — manba koddan o'qilgan bitta himoyalangan endpoint.
type authRoute struct {
	method  string
	pattern string       // e'londagi shakl: /restaurants/{id}/orders
	path    string       // so'rov uchun aniq yo'l
	roles   []users.Role // bo'sh = istalgan tizimga kirgan foydalanuvchi
	file    string
}

// authDecl — `mux.HandleFunc("METHOD /yo'l", s.auth([]users.Role{...},`
// Rol ro'yxati keyingi qatorga o'tishi mumkin, shuning uchun `(?s)` va
// cheklangan oraliq ishlatiladi.
var authDecl = regexp.MustCompile(
	`(?s)mux\.HandleFunc\("([A-Z]+) ([^"]+)",\s*s\.auth\(\s*(\[\]users\.Role\{([^}]*)\}|nil)`)

var roleName = map[string]users.Role{
	"RoleAdmin":      users.RoleAdmin,
	"RoleCustomer":   users.RoleCustomer,
	"RoleRestaurant": users.RoleRestaurant,
	"RoleCourier":    users.RoleCourier,
	"RoleWaiter":     users.RoleWaiter,
}

// concretePath — `{id}` kabi parametrlarni fikstura yaratgan HAQIQIY
// qiymatlar bilan to'ldiradi.
//
// NEGA MUHIM: rol tekshiruvi `auth` middleware'da, handlerdan OLDIN
// bajariladi — ya'ni ko'p endpoint uchun yo'ldagi qiymat ahamiyatsiz
// (noto'g'ri id 404 beradi, bu test uchun maqbul). LEKIN 9 ta joyda
// handler qo'shimcha EGALIK tekshiruvi qiladi (`claims.EntityID != ...`)
// va u ham 403 qaytaradi. O'sha joylarda "begona" id berilsa, test
// egalik rad etishini rol rad etishi deb o'qib, YOLG'ON o'tardi.
// Shuning uchun restoran va kuryer yo'llariga fikstura id'lari beriladi.
func concretePath(pattern string) string {
	p := pattern
	if strings.HasPrefix(p, "/restaurants/{id}") {
		p = strings.Replace(p, "{id}", "rest-a", 1)
	}
	if strings.HasPrefix(p, "/couriers/{id}") {
		p = strings.Replace(p, "{id}", "cour-ent-1", 1)
	}
	// Qolgan barcha parametrlar — mavjud bo'lmagan, lekin SHAKLI
	// to'g'ri qiymat.
	return regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(p, "x")
}

// discoverProtectedRoutes — manba kodni o'qib, himoyalangan
// endpointlar ro'yxatini quradi.
func discoverProtectedRoutes(t *testing.T) []authRoute {
	t.Helper()
	files, err := filepath.Glob("routes_*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []authRoute
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range authDecl.FindAllStringSubmatch(string(src), -1) {
			var roles []users.Role
			for _, tok := range strings.Split(m[4], ",") {
				tok = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(tok), "users."))
				if r, ok := roleName[tok]; ok {
					roles = append(roles, r)
				}
			}
			out = append(out, authRoute{
				method: m[1], pattern: m[2], path: concretePath(m[2]),
				roles: roles, file: f,
			})
		}
	}
	if len(out) == 0 {
		t.Fatal("birorta himoyalangan endpoint topilmadi — regex buzilganmi?")
	}
	return out
}

// ---------------------------------------------------------------- fixture --

func authzServer(t *testing.T) (http.Handler, map[users.Role]string) {
	t.Helper()
	ctx := context.Background()

	userRepo := storage.NewMemoryUserRepo()
	courierRepo := storage.NewMemoryCourierRepo(
		couriers.Courier{ID: "cour-ent-1", Name: "Kuryer", Approved: true})
	tokens := users.NewTokenIssuer("test-secret", time.Hour)

	mk := func(id string, role users.Role, entityID, phone string) *users.User {
		u := &users.User{ID: id, Phone: phone, Role: role, EntityID: entityID,
			PhoneVerified: true, CreatedAt: time.Now()}
		if err := userRepo.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	// EntityID lar ATAYLAB "to'g'ri": shunda egalik tekshiruvi o'tadi va
	// test AYNAN rol tekshiruvini o'lchaydi.
	made := map[users.Role]*users.User{
		users.RoleAdmin:      mk("u-admin", users.RoleAdmin, "", "+998900000001"),
		users.RoleCustomer:   mk("u-cust", users.RoleCustomer, "", "+998900000002"),
		users.RoleRestaurant: mk("u-rest", users.RoleRestaurant, "rest-a", "+998900000003"),
		users.RoleCourier:    mk("u-cour", users.RoleCourier, "cour-ent-1", "+998900000004"),
		users.RoleWaiter:     mk("u-waiter", users.RoleWaiter, "rest-a", "+998900000005"),
	}
	jwt := map[users.Role]string{}
	for role, u := range made {
		tok, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		jwt[role] = tok
	}

	// HAMMA repo berilishi SHART: bittasi `nil` qolsa handler unga
	// murojaat qilganda nil-pointer panika bo'ladi va test avtorizatsiyaga
	// umuman aloqasiz sababdan qulaydi. (Birinchi ishga tushirishda
	// aynan shunday bo'ldi — `PromotionsRepo` va `Dispatcher` unutilgan edi.)
	orderRepo := storage.NewMemoryOrderRepo()
	promoRepo := storage.NewMemoryPromotionsRepo()

	deps := Deps{
		UserRepo:       userRepo,
		OrderRepo:      orderRepo,
		CourierRepo:    courierRepo,
		CatalogRepo:    storage.NewMemoryCatalogRepo(nil, nil),
		PromotionsRepo: promoRepo,
		FavoritesRepo:  storage.NewMemoryFavoritesRepo(),
		Devices:        storage.NewMemoryDeviceStore(),
		Tokens:         tokens,
		Revoked:        revoke.New(nil, time.Hour),
		Hub:            ws.NewHub(nil),
		WsTickets:      ws.NewTicketStore(),
		Dispatcher:     couriers.NewDispatcher(courierRepo, nil, nil, time.Second),
		// Parol o'zgartirish endpointi shunga murojaat qiladi.
		AuthSvc: users.NewService(userRepo, storage.NewMemoryCodeStore(),
			nil, tokens, func() string { return "gen-id" }),
		// Buyurtma o'qish/o'tkazish endpointlari shunga murojaat qiladi.
		OrderSvc: orders.NewService(orderRepo, nil,
			func() string { return "ord-1" }, promoRepo),
		DevMode: true,
	}
	return New(deps).Routes(nil), jwt
}

func allowsRole(r authRoute, role users.Role) bool {
	if len(r.roles) == 0 {
		return true // rol talab qilinmaydi — token yetarli
	}
	for _, a := range r.roles {
		if a == role {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------- tests ----

// ★ ASOSIY: har bir himoyalangan endpoint × har bir rol.
func TestAuthorizationMatrix(t *testing.T) {
	roles := []users.Role{
		users.RoleAdmin, users.RoleCustomer, users.RoleRestaurant,
		users.RoleCourier, users.RoleWaiter,
	}
	routes := discoverProtectedRoutes(t)
	t.Logf("himoyalangan endpointlar: %d ta", len(routes))

	for _, r := range routes {
		// ┌─ HAR ENDPOINT UCHUN YANGI FIKSTURA (tuzatilgan beqaror test) ───┐
		// Matritsa `POST /auth/logout` ni ham har bir rol tokeni bilan
		// uradi, u esa SHU foydalanuvchining oldingi barcha tokenlarini
		// bekor qiladi (`revoke`, soniya aniqligida). Fikstura butun test
		// uchun BITTA bo'lganda tokenlar N-soniyada chiqarilib, logout
		// N+1-soniyaga to'g'ri kelsa, undan keyingi BARCHA endpointlar
		// (routes_catalog.go, routes_me.go, ... alifbo tartibida) 401
		// olardi. Natija mashina tezligiga bog'liq edi: tez lokal
		// mashinada hammasi bir soniyaga sig'ib o'tardi, CI'da `-race`
		// sekinlashtirgani uchun yiqildi.
		//
		// Parol almashtirish, akkaunt/restoran/kuryer o'chirish ham
		// sessiyani bekor qiladi — shuning uchun "xavfli endpointlar
		// ro'yxati" emas (yangisi qo'shilganda unutilardi), to'liq
		// izolyatsiya: bitta endpointning yon ta'siri boshqasiga o'tmaydi.
		// └────────────────────────────────────────────────────────────────┘
		h, jwt := authzServer(t)
		for _, role := range roles {
			t.Run(string(role)+"_"+r.method+"_"+r.pattern, func(t *testing.T) {
				w := do(t, h, r.method, r.path, jwt[role], `{}`)
				denied := w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden

				if allowsRole(r, role) {
					// Ruxsat etilgan rol 400/404/409/503 olishi MUMKIN
					// (bo'sh tana, mavjud bo'lmagan resurs, o'chirilgan
					// xizmat) — bu normal. Faqat 401/403 bo'lmasligi kerak.
					if denied {
						t.Errorf("%s roli %s %s ga kira olishi kerak edi (%s), lekin %d oldi",
							role, r.method, r.pattern, r.file, w.Code)
					}
					return
				}
				if !denied {
					t.Errorf("XAVFSIZLIK: %s roli %s %s ga KIRA OLDI (%d) — 401/403 kutilgan edi (%s)",
						role, r.method, r.pattern, w.Code, r.file)
				}
			})
		}
	}
}

// Tokensiz so'rov hech qachon o'tmasligi kerak.
func TestProtectedRoutesRejectAnonymous(t *testing.T) {
	h, _ := authzServer(t)
	for _, r := range discoverProtectedRoutes(t) {
		w := do(t, h, r.method, r.path, "", `{}`)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("XAVFSIZLIK: %s %s tokensiz %d qaytardi — 401 kutilgan edi (%s)",
				r.method, r.pattern, w.Code, r.file)
		}
	}
}

// ★ IJARA (TENANCY) MATRITSASI — bug.md 101-band.
//
// ┌─ NEGA YUQORIDAGI MATRITSA YETARLI EMAS ────────────────────────────┐
// `TestAuthorizationMatrix` VERTIKAL imtiyozni o'lchaydi: "restoran
// roli admin endpointiga kira oladimi". GORIZONTALNI — "rest-a
// REST-B ning resursiga kira oladimi" — u ATAYLAB o'lchamaydi:
// `concretePath` yo'lga har doim foydalanuvchining O'Z id'sini
// qo'yadi, aks holda egalik rad etishi rol rad etishi deb o'qilib,
// test yolg'on o'tardi.
//
// Natijada butun bir xavf sinfi matritsadan tashqarida qolgandi va
// auditdagi TASDIQLANGAN gorizontal xatolarning HAMMASI aynan shu
// bo'shliqda edi: 19-band (begona taom ID'si), 77-band (to'liq
// almashtirish), 29-band (WS obunasi). Matritsa yashil, kod ochiq.
//
// Bu test o'sha bo'shliqni yopadi: rol TO'G'RI, resurs BEGONA.
// Endpointlar ro'yxati bir xil manbadan (`discoverProtectedRoutes`)
// olinadi, ya'ni yangi endpoint qo'shilganda bu test ham O'ZI
// kengayadi.
// └────────────────────────────────────────────────────────────────────┘
func TestTenancyMatrix(t *testing.T) {
	h, jwt := authzServer(t)

	// `rest-b` va `cour-ent-2` — fiksturada YO'Q (`authzServer` faqat
	// `rest-a` va `cour-ent-1` yaratadi). Ya'ni ular ham begona, ham
	// mavjud emas: har ikkala holatda ham javob 401/403/404 bo'lishi
	// kerak, hech qachon 2xx.
	cases := []struct {
		prefix   string     // yo'l prefiksi
		foreign  string     // begona egalik id'si
		role     users.Role // shu resursga "yaqin" rol
		roleName string
	}{
		{"/restaurants/{id}", "rest-b", users.RoleRestaurant, "restoran"},
		{"/restaurants/{id}", "rest-b", users.RoleWaiter, "affitsiant"},
		{"/couriers/{id}", "cour-ent-2", users.RoleCourier, "kuryer"},
	}

	var checked int
	for _, r := range discoverProtectedRoutes(t) {
		for _, c := range cases {
			if !strings.HasPrefix(r.pattern, c.prefix) {
				continue
			}
			// Bu rol umuman kira olmasa, bu test uning ishi emas —
			// uni `TestAuthorizationMatrix` allaqachon o'lchaydi.
			if !allowsRole(r, c.role) {
				continue
			}
			path := strings.Replace(r.pattern, "{id}", c.foreign, 1)
			path = regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(path, "x")

			checked++
			t.Run(c.roleName+"_"+r.method+"_"+r.pattern, func(t *testing.T) {
				w := do(t, h, r.method, path, jwt[c.role], `{}`)
				// 403 — egalik rad etildi (kutilgan).
				// 404 — resurs topilmadi (ham maqbul: ma'lumot chiqmadi).
				// 401 — token rad etildi (bu yerda bo'lmasligi kerak,
				//       lekin xavfsiz tomon).
				// 400 — tana bo'sh/noto'g'ri: handler egalikkacha
				//       yetmagan, ya'ni hech narsa oshkor bo'lmagan.
				switch w.Code {
				case http.StatusForbidden, http.StatusNotFound,
					http.StatusUnauthorized, http.StatusBadRequest:
					return
				}
				t.Errorf("XAVFSIZLIK (gorizontal): %s (%s) BEGONA %q resursiga %s %s bilan KIRDI — status %d (%s)",
					c.roleName, c.role, c.foreign, r.method, path, w.Code, r.file)
			})
		}
	}

	// Ro'yxat bo'shab qolmasin: regex yoki prefiks o'zgarsa test
	// jimgina "hech narsa tekshirmaydigan" holatga tushib qolardi —
	// bu 102-band bilan bir xil tuzoq.
	if checked == 0 {
		t.Fatal("birorta ham egalikka bog'liq endpoint tekshirilmadi — " +
			"prefikslar yoki `discoverProtectedRoutes` o'zgarganmi?")
	}
	t.Logf("egalik bo'yicha tekshirilgan endpointlar: %d ta", checked)
}

// Begona kalit bilan imzolangan token rad etilishi kerak.
func TestProtectedRoutesRejectForgedToken(t *testing.T) {
	h, _ := authzServer(t)
	other := users.NewTokenIssuer("boshqa-kalit", time.Hour)
	forged, err := other.Issue(&users.User{
		ID: "u-admin", Role: users.RoleAdmin, PhoneVerified: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range discoverProtectedRoutes(t) {
		w := do(t, h, r.method, r.path, forged, `{}`)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("XAVFSIZLIK: %s %s SOXTA token bilan %d qaytardi — 401 kutilgan edi (%s)",
				r.method, r.pattern, w.Code, r.file)
		}
	}
}
