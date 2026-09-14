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
	"chustapp/internal/storage"
	"chustapp/internal/tables"
	"chustapp/internal/users"
	"chustapp/internal/ws"
)

// settingsServer — ikki restoran (A — taom bilan), stol xizmati;
// tokenlar: "a", "b" (restoran xodimlari), "admin", "waiter", "customer".
func settingsServer(t *testing.T) (http.Handler, *tables.Service, map[string]string) {
	t.Helper()
	ctx := context.Background()
	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
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
	mk("a", "u-a", users.RoleRestaurant, testRestA, "+998900000061")
	mk("b", "u-b", users.RoleRestaurant, testRestB, "+998900000062")
	mk("admin", "u-admin", users.RoleAdmin, "", "+998900000063")
	mk("waiter", "u-w", users.RoleWaiter, testRestA, "+998900000064")
	mk("customer", "u-c", users.RoleCustomer, "", "+998900000065")

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{
			{ID: testRestA, Name: "Book Cafe", Address: "M. Fayozov ko'chasi", Kind: catalog.KindCafe,
				Lat: 41.0, Lng: 71.23, Open: true},
			{ID: testRestB, Name: "B", Open: true},
		},
		[]catalog.Product{{ID: "prod-a", RestaurantID: testRestA, Name: "Osh", PriceTiyin: 3500000, Available: true}},
	)
	orderRepo := storage.NewMemoryOrderRepo()
	tableSvc := tables.NewService(storage.NewMemoryTableRepo())
	h := New(Deps{
		UserRepo:       userRepo,
		CatalogRepo:    catalogRepo,
		PromotionsRepo: storage.NewMemoryPromotionsRepo(),
		OrderRepo:      orderRepo,
		TableSvc:       tableSvc,
		Tokens:         tokens,
		CatalogSvc:     catalog.NewService(catalogRepo),
		OrderSvc:       orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo()),
		Hub:            ws.NewHub(nil),
		DevMode:        true,
	}).Routes(nil)
	return h, tableSvc, jwt
}

type settingsResp struct {
	Name           string                 `json:"name"`
	Kind           string                 `json:"kind"`
	KindTitle      string                 `json:"kind_title"`
	Phone          string                 `json:"phone"`
	Address        string                 `json:"address"`
	LogoURL        string                 `json:"logo_url"`
	CoverURL       string                 `json:"cover_url"`
	Description    string                 `json:"description"`
	WorkingHours   *catalog.WorkingHours  `json:"working_hours"`
	PaymentMethods catalog.PaymentMethods `json:"payment_methods"`
	Editable       []string               `json:"editable"`
	Locked         []string               `json:"locked"`
}

const settingsPathA = "/restaurants/" + testRestA + "/settings"

func getSettings(t *testing.T, h http.Handler, token string) settingsResp {
	t.Helper()
	w := do(t, h, "GET", settingsPathA, token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET settings: %d — %s", w.Code, w.Body.String())
	}
	var s settingsResp
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func weekJSON(open, closeAt string, enabled bool) string {
	var days []string
	for d := 1; d <= 7; d++ {
		days = append(days, `{"day":`+string(rune('0'+d))+`,"enabled":`+map[bool]string{true: "true", false: "false"}[enabled]+
			`,"open":"`+open+`","close":"`+closeAt+`"}`)
	}
	return `{"days":[` + strings.Join(days, ",") + `]}`
}

func TestSettingsOwnershipAndView(t *testing.T) {
	h, _, jwt := settingsServer(t)

	s := getSettings(t, h, jwt["a"])
	if s.Name != "Book Cafe" || s.KindTitle != "Kafe" || s.Phone != "+998900000061" ||
		s.Address != "M. Fayozov ko'chasi" {
		t.Fatalf("ko'rinish: %+v", s)
	}
	if !s.PaymentMethods.Cash || !s.PaymentMethods.CardOnline || !s.PaymentMethods.OnDexWallet || s.WorkingHours != nil {
		t.Fatalf("standart qiymatlar: %+v", s)
	}
	for _, k := range []string{"name", "kind", "phone", "address"} {
		if !contains(s.Locked, k) || contains(s.Editable, k) {
			t.Errorf("%s yopiq bo'lishi kerak: locked=%v editable=%v", k, s.Locked, s.Editable)
		}
	}

	for _, c := range []struct {
		method, token string
		want          int
	}{
		{"GET", jwt["b"], http.StatusForbidden},
		{"PATCH", jwt["b"], http.StatusForbidden},
		{"GET", jwt["waiter"], http.StatusForbidden},
		{"PATCH", jwt["customer"], http.StatusForbidden},
		{"GET", "", http.StatusUnauthorized},
	} {
		if w := do(t, h, c.method, settingsPathA, c.token, `{"description":"x"}`); w.Code != c.want {
			t.Errorf("%s: kutilgan %d, keldi %d", c.method, c.want, w.Code)
		}
	}
	if w := do(t, h, "GET", settingsPathA, jwt["admin"], ""); w.Code != http.StatusOK {
		t.Fatalf("admin: %d", w.Code)
	}
}

// ★★ Nomi, turi, telefoni, manzili restoran panelidan O'ZGARMAYDI.
func TestSettingsRejectsLockedFields(t *testing.T) {
	h, _, jwt := settingsServer(t)
	for _, body := range []string{
		`{"name":"Boshqa nom"}`,
		`{"kind":"restaurant"}`,
		`{"phone":"+998901112233"}`,
		`{"address":"Toshkent"}`,
		`{"lat":40.1,"lng":70.1}`,
		`{"description":"yangi","address":"Toshkent"}`,
	} {
		if w := do(t, h, "PATCH", settingsPathA, jwt["a"], body); w.Code != http.StatusForbidden {
			t.Errorf("%s: kutilgan 403, keldi %d — %s", body, w.Code, w.Body.String())
		}
	}
	s := getSettings(t, h, jwt["a"])
	if s.Name != "Book Cafe" || s.Address != "M. Fayozov ko'chasi" || s.Kind != "cafe" || s.Description != "" {
		t.Fatalf("XAVFSIZLIK: yopiq so'rovdan keyin ma'lumot o'zgardi: %+v", s)
	}
	for _, body := range []string{`{}`, `{"foo":1}`, `[1,2]`, `not json`, `{"open":false}`} {
		if w := do(t, h, "PATCH", settingsPathA, jwt["a"], body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: kutilgan 400, keldi %d", body, w.Code)
		}
	}
}

func TestSettingsPatchValidationAndPersistence(t *testing.T) {
	h, _, jwt := settingsServer(t)

	for _, c := range []struct {
		body string
		want int
	}{
		{`{"description":"` + strings.Repeat("a", catalog.MaxDescriptionLen+1) + `"}`, http.StatusBadRequest},
		{`{"description":"a‮b"}`, http.StatusBadRequest},
		{`{"logo_url":"https://evil.example/pixel.webp"}`, http.StatusBadRequest},
		{`{"logo_url":"/uploads/products/x.webp"}`, http.StatusBadRequest},
		{`{"logo_url":"/uploads/logos/../covers/x.webp"}`, http.StatusBadRequest},
		{`{"logo_url":"/uploads/logos/x.png"}`, http.StatusBadRequest},
		{`{"cover_url":"javascript:alert(1)"}`, http.StatusBadRequest},
		{`{"working_hours":{"days":[]}}`, http.StatusBadRequest},
		{`{"working_hours":` + weekJSON("8:00", "23:00", true) + `}`, http.StatusBadRequest},
		{`{"working_hours":{"days":[],"timezone":"UTC"}}`, http.StatusBadRequest},
		{`{"payment_methods":{"cash":false,"card_terminal":false,"card_online":false}}`, http.StatusBadRequest},
		// OnDex Wallet — o'chirib bo'lmaydi; uning o'zi haqiqiy usul o'rnini bosmaydi.
		{`{"payment_methods":{"cash":true,"card_terminal":false,"card_online":false,"ondex_wallet":false}}`, http.StatusForbidden},
		{`{"payment_methods":{"cash":false,"card_terminal":false,"card_online":false,"ondex_wallet":true}}`, http.StatusBadRequest},
		{`{"payment_methods":{"cash":true,"bitcoin":true}}`, http.StatusBadRequest},
	} {
		if w := do(t, h, "PATCH", settingsPathA, jwt["a"], c.body); w.Code != c.want {
			t.Errorf("%.80s: kutilgan %d, keldi %d — %s", c.body, c.want, w.Code, w.Body.String())
		}
	}

	body := `{"description":"  Mazali taomlar va ajoyib muhit  ",` +
		`"logo_url":"/uploads/logos/abc.webp","cover_url":"/uploads/covers/def.webp",` +
		`"working_hours":` + weekJSON("08:00", "23:00", true) + `,` +
		`"payment_methods":{"cash":true,"card_terminal":true,"card_online":false}}`
	if w := do(t, h, "PATCH", settingsPathA, jwt["a"], body); w.Code != http.StatusOK {
		t.Fatalf("to'g'ri so'rov: %d — %s", w.Code, w.Body.String())
	}
	s := getSettings(t, h, jwt["a"])
	if s.Description != "Mazali taomlar va ajoyib muhit" || s.LogoURL != "/uploads/logos/abc.webp" ||
		s.CoverURL != "/uploads/covers/def.webp" || s.WorkingHours == nil || len(s.WorkingHours.Days) != 7 ||
		s.PaymentMethods.CardOnline || !s.PaymentMethods.CardTerminal || !s.PaymentMethods.OnDexWallet {
		t.Fatalf("saqlanmadi: %+v", s)
	}

	// Onlayn karta tizimda ulanmagan (Payments yo'q) — qayta yoqib bo'lmaydi.
	if w := do(t, h, "PATCH", settingsPathA, jwt["a"],
		`{"payment_methods":{"cash":true,"card_terminal":false,"card_online":true}}`); w.Code != http.StatusBadRequest {
		t.Fatalf("ulanmagan onlayn to'lov yoqildi: %d", w.Code)
	}
	// Ish vaqtini olib tashlash — cheklanmagan.
	if w := do(t, h, "PATCH", settingsPathA, jwt["a"], `{"working_hours":null,"logo_url":""}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if s := getSettings(t, h, jwt["a"]); s.WorkingHours != nil || s.LogoURL != "" {
		t.Fatalf("tozalanmadi: %+v", s)
	}
}

// Sozlamalar faqat panelda ko'rinmaydi — buyurtma berishda MAJBURIY.
func TestSettingsEnforcedOnOrders(t *testing.T) {
	h, tableSvc, jwt := settingsServer(t)
	table, err := tableSvc.Create(context.Background(), testRestA, "5")
	if err != nil {
		t.Fatal(err)
	}
	order := func() int {
		body := `{"items":[{"product_id":"prod-a","qty":1}],"table_token":"` + table.QRToken + `","payment_method":"cash"}`
		return do(t, h, "POST", "/orders", jwt["customer"], body).Code
	}
	if code := order(); code != http.StatusCreated {
		t.Fatalf("standart sozlamada buyurtma: %d", code)
	}

	// Faqat onlayn karta — joyida (naqd) to'lov rad etiladi.
	if w := do(t, h, "PATCH", settingsPathA, jwt["a"],
		`{"payment_methods":{"cash":false,"card_terminal":false,"card_online":true}}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if code := order(); code != http.StatusBadRequest {
		t.Fatalf("o'chirilgan to'lov usuli bilan buyurtma yaratildi: %d", code)
	}

	// Naqd qayta yoqiladi, lekin hamma kun dam olish — ish vaqti tashqarisida.
	if w := do(t, h, "PATCH", settingsPathA, jwt["a"],
		`{"payment_methods":{"cash":true,"card_terminal":false,"card_online":true},"working_hours":`+
			weekJSON("08:00", "23:00", false)+`}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if code := order(); code != http.StatusBadRequest {
		t.Fatalf("ish vaqtidan tashqarida buyurtma yaratildi: %d", code)
	}

	// Ochiq javobda "hozir qabul qiladimi" ko'rinadi.
	w := do(t, h, "GET", "/restaurants/"+testRestA, "", "")
	var pub map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &pub)
	if pub["open"] != true || pub["open_now"] != false {
		t.Fatalf("ochiq javob: open=%v open_now=%v", pub["open"], pub["open_now"])
	}
}

func TestAdminSetsRestaurantKind(t *testing.T) {
	h, _, jwt := settingsServer(t)
	path := "/admin/restaurants/" + testRestA
	base := `"name":"Book Cafe","address":"M. Fayozov ko'chasi","lat":41.0,"lng":71.23`
	if w := do(t, h, "POST", path, jwt["admin"], `{`+base+`,"kind":"teahouse"}`); w.Code != http.StatusOK {
		t.Fatalf("admin tur: %d — %s", w.Code, w.Body.String())
	}
	if s := getSettings(t, h, jwt["a"]); s.KindTitle != "Choyxona" {
		t.Fatalf("tur saqlanmadi: %+v", s)
	}
	// Eski admin panel `kind` yubormaydi — tur o'chib ketmasin.
	if w := do(t, h, "POST", path, jwt["admin"], `{`+base+`}`); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	if s := getSettings(t, h, jwt["a"]); s.Kind != "teahouse" {
		t.Fatalf("eski so'rov turni o'chirdi: %+v", s)
	}
	if w := do(t, h, "POST", path, jwt["admin"], `{`+base+`,"kind":"sauna"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("noma'lum tur: %d", w.Code)
	}
	if w := do(t, h, "POST", path, jwt["a"], `{`+base+`,"kind":"cafe"}`); w.Code != http.StatusForbidden {
		t.Fatalf("XAVFSIZLIK: restoran admin endpointiga kirdi: %d", w.Code)
	}
}

func TestIsOwnUploadURL(t *testing.T) {
	t.Setenv("R2_PUBLIC_URL", "https://cdn.ondex.uz")
	for raw, want := range map[string]bool{
		"":                                       true,
		"/uploads/logos/a.webp":                  true,
		"https://cdn.ondex.uz/logos/a.webp":      true,
		"https://cdn.ondex.uz/covers/a.webp":     false,
		"https://cdn.ondex.uz.evil/logos/a.webp": false,
		"https://evil.com/logos/a.webp":          false,
		"/uploads/logos/a.webp?x=1":              false,
		"/uploads/logos/../a.webp":               false,
		"//evil.com/uploads/logos/a.webp":        false,
		"/uploads/logos/a.jpg":                   false,
	} {
		if got := isOwnUploadURL(raw, "logos"); got != want {
			t.Errorf("%q: %v, kutilgan %v", raw, got, want)
		}
	}
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
