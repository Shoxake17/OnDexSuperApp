package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// RESTORAN RO'YXATI MIJOZNING SHAHRI BILAN CHEKLANADI.
//
// ┌─ NEGA KERAK ───────────────────────────────────────────────────────┐
// Buyurtma bosqichida shahar allaqachon tekshirilardi
// (`TestCreateOrder_RestaurantAndAddressCity`), lekin RO'YXAT
// filtrlanmasdi: Chustdagi mijoz Toshkentdagi restoranlarni ko'rar,
// menyuni ochar, savatni to'ldirar va faqat oxirida "boshqa shahar"
// xatosiga urilardi.
// └────────────────────────────────────────────────────────────────────┘

// cityListServer — ikkita restoran (Chust va Toshkent) va bitta
// foydalanuvchi. `addrLat/addrLng` nol bo'lsa manzil YO'Q hisoblanadi.
func cityListServer(t *testing.T, addrLat, addrLng float64) (http.Handler, string) {
	return cityListServerAs(t, users.RoleCustomer, addrLat, addrLng)
}

func cityListServerAs(t *testing.T, role users.Role, addrLat, addrLng float64) (http.Handler, string) {
	t.Helper()
	ctx := context.Background()

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	customer := &users.User{
		ID: "cust-1", Phone: "+998901234567",
		Role: role, PhoneVerified: true,
		Address: users.AddressDetails{Lat: addrLat, Lng: addrLng, Text: "Manzil"},
	}
	if err := userRepo.Create(ctx, customer); err != nil {
		t.Fatal(err)
	}
	jwt, err := tokens.Issue(customer)
	if err != nil {
		t.Fatal(err)
	}

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{
			{ID: "rest-chust", Name: "Chust oshxonasi", Open: true, Lat: 41.0004, Lng: 71.2394},
			{ID: "rest-tash", Name: "Toshkent kafesi", Open: true, Lat: 41.2995, Lng: 69.2401},
			// Koordinatasiz restoran — hech bir shaharga biriktirilmaydi
			// va HAMMAGA ko'rinadi (`CheckServes` dagi o'sha qoida).
			{ID: "rest-nowhere", Name: "Koordinatasiz", Open: true},
		},
		nil,
	)
	deps := Deps{
		UserRepo:    userRepo,
		OrderRepo:   storage.NewMemoryOrderRepo(),
		CatalogRepo: catalogRepo,
		Tokens:      tokens,
		CatalogSvc:  catalog.NewService(catalogRepo),
		OrderSvc:    orders.NewService(storage.NewMemoryOrderRepo(), nil, NewID, storage.NewMemoryPromotionsRepo()),
		DevMode:     true,
	}
	return New(deps).Routes(nil), jwt
}

// ids — javobdagi restoran id'lari.
func ids(t *testing.T, body []byte) []string {
	t.Helper()
	var list []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("javobni o'qib bo'lmadi: %v — %s", err, body)
	}
	out := make([]string, 0, len(list))
	for _, r := range list {
		out = append(out, r.ID)
	}
	return out
}

func TestListRestaurants_FilteredByCity(t *testing.T) {
	const (
		chustLat, chustLng       = 40.99454866260851, 71.24035120010376
		tashkentLat, tashkentLng = 41.3650, 69.2880
	)

	t.Run("Chust mijozi Toshkent restoranini KO'RMAYDI", func(t *testing.T) {
		h, jwt := cityListServer(t, chustLat, chustLng)
		w := do(t, h, "GET", "/restaurants", jwt, "")
		if w.Code != http.StatusOK {
			t.Fatalf("kod %d: %s", w.Code, w.Body.String())
		}
		got := ids(t, w.Body.Bytes())
		want := map[string]bool{"rest-chust": true, "rest-nowhere": true}
		if len(got) != len(want) {
			t.Fatalf("ro'yxat %v, kutilgan %v", got, want)
		}
		for _, id := range got {
			if !want[id] {
				t.Errorf("%s ko'rinmasligi kerak edi", id)
			}
		}
	})

	t.Run("Toshkent mijozi Chust restoranini KO'RMAYDI", func(t *testing.T) {
		h, jwt := cityListServer(t, tashkentLat, tashkentLng)
		got := ids(t, do(t, h, "GET", "/restaurants", jwt, "").Body.Bytes())
		for _, id := range got {
			if id == "rest-chust" {
				t.Fatalf("Chust restorani Toshkent mijoziga ko'rindi: %v", got)
			}
		}
		var tash bool
		for _, id := range got {
			if id == "rest-tash" {
				tash = true
			}
		}
		if !tash {
			t.Errorf("o'z shahri restorani yo'qolgan: %v", got)
		}
	})

	// ┌─ FILTR BUYURTMA QOIDASI BILAN BIR XIL ─────────────────────────┐
	// Koordinatasiz restoran `CheckServes` da ham cheklanmaydi —
	// ro'yxat va buyurtma bir xil javob berishi kerak.
	// └────────────────────────────────────────────────────────────────┘
	t.Run("koordinatasiz restoran HAMMAGA ko'rinadi", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			lat, lng float64
		}{
			{"Chust mijozi", chustLat, chustLng},
			{"Toshkent mijozi", tashkentLat, tashkentLng},
		} {
			h, jwt := cityListServer(t, c.lat, c.lng)
			var bor bool
			for _, id := range ids(t, do(t, h, "GET", "/restaurants", jwt, "").Body.Bytes()) {
				if id == "rest-nowhere" {
					bor = true
				}
			}
			if !bor {
				t.Errorf("%s: koordinatasiz restoran yo'qolgan", c.name)
			}
		}
	})

	// ┌─ BOSH SAHIFA BO'SH OCHILMASIN ─────────────────────────────────┐
	// Kirmagan yoki hali manzil tanlamagan odam TO'LIQ ro'yxatni
	// ko'radi — avval nimalar borligini ko'rib, keyin manzil tanlaydi.
	// └────────────────────────────────────────────────────────────────┘
	t.Run("kirmagan foydalanuvchi hammasini ko'radi", func(t *testing.T) {
		h, _ := cityListServer(t, chustLat, chustLng)
		got := ids(t, do(t, h, "GET", "/restaurants", "", "").Body.Bytes())
		if len(got) != 3 {
			t.Fatalf("tokensiz ro'yxat %v, 3 ta kutilgan", got)
		}
	})

	t.Run("manzilsiz mijoz hammasini ko'radi", func(t *testing.T) {
		h, jwt := cityListServer(t, 0, 0)
		got := ids(t, do(t, h, "GET", "/restaurants", jwt, "").Body.Bytes())
		if len(got) != 3 {
			t.Fatalf("manzilsiz ro'yxat %v, 3 ta kutilgan", got)
		}
	})

	// ┌─ ADMIN PANELINI SINDIRMASLIK ──────────────────────────────────┐
	// `GET /restaurants` ni admin paneli HAM o'qiydi
	// (`apps/admin_panel/lib/api.dart`). Adminning shaxsiy yetkazish
	// manzili saqlangan bo'lsa va filtr unga ham qo'llansa, panelda
	// boshqa shaharlarning restoranlari YO'QOLARDI — ularni boshqarib
	// bo'lmasdi.
	// └────────────────────────────────────────────────────────────────┘
	t.Run("xodim rollari filtrlanmaydi", func(t *testing.T) {
		for _, role := range []users.Role{
			users.RoleAdmin, users.RoleRestaurant, users.RoleWaiter,
		} {
			h, jwt := cityListServerAs(t, role, chustLat, chustLng)
			got := ids(t, do(t, h, "GET", "/restaurants", jwt, "").Body.Bytes())
			if len(got) != 3 {
				t.Errorf("%s: ro'yxat %v, 3 ta kutilgan — panel sinadi", role, got)
			}
		}
	})

	// Kuryer ham ovqat buyurtma qiladi — u mijoz kabi filtrlanadi.
	t.Run("kuryer mijoz kabi filtrlanadi", func(t *testing.T) {
		h, jwt := cityListServerAs(t, users.RoleCourier, chustLat, chustLng)
		got := ids(t, do(t, h, "GET", "/restaurants", jwt, "").Body.Bytes())
		for _, id := range got {
			if id == "rest-tash" {
				t.Fatalf("kuryerga boshqa shahar restorani ko'rindi: %v", got)
			}
		}
	})

	// Yaroqsiz token 401 BERMAYDI — ochiq endpoint, anonim deb qaraladi.
	t.Run("yaroqsiz token — anonim, 401 emas", func(t *testing.T) {
		h, _ := cityListServer(t, chustLat, chustLng)
		w := do(t, h, "GET", "/restaurants", "buzuq.token.qiymati", "")
		if w.Code != http.StatusOK {
			t.Fatalf("kod %d, 200 kutilgan: %s", w.Code, w.Body.String())
		}
		if got := ids(t, w.Body.Bytes()); len(got) != 3 {
			t.Fatalf("ro'yxat %v, 3 ta kutilgan", got)
		}
	})
}
