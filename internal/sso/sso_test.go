package sso

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"chustapp/internal/users"
)

const (
	testSecret   = "0123456789abcdef0123456789abcdef0123456789abcdef"
	testRedirect = "https://pos.ondex.uz/auth/callback"
)

type fakeUsers map[string]*users.User

func (f fakeUsers) GetByID(_ context.Context, id string) (*users.User, error) {
	if u, ok := f[id]; ok {
		return u, nil
	}
	return nil, users.ErrUserNotFound
}

func newTestService(t *testing.T) (*Service, *time.Time) {
	t.Helper()
	svc, err := New([]Client{{
		ID: "merchant", Name: "OnDex Merchant Panel", Secret: testSecret,
		RedirectURIs: []string{testRedirect, "http://localhost:5173/auth/callback"},
	}}, fakeUsers{"u1": {ID: "u1", Phone: "+998901234567", Name: "Ali"}})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	return svc, &now
}

// codeFrom — qaytish manzilidan `code` va `state` ni ajratadi.
func codeFrom(t *testing.T, location string) (code, state string) {
	t.Helper()
	u, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query().Get("code"), u.Query().Get("state")
}

func TestHappyPath(t *testing.T) {
	svc, _ := newTestService(t)

	loc, err := svc.Authorize("u1", "merchant", testRedirect, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(loc, testRedirect+"?") {
		t.Fatalf("qaytish manzili boshqa joyga ketdi: %s", loc)
	}
	code, state := codeFrom(t, loc)
	if code == "" || state != "abc123" {
		t.Fatalf("code=%q state=%q", code, state)
	}

	id, err := svc.Exchange(context.Background(), "merchant", testSecret, code, testRedirect)
	if err != nil {
		t.Fatal(err)
	}
	if id.UserID != "u1" || id.Phone != "+998901234567" || id.Name != "Ali" {
		t.Fatalf("kimlik noto'g'ri: %+v", id)
	}
}

// Hamkor akkauntlari (restoran, do'kon) rol va muassasa nomi bilan
// qaytadi; oddiy foydalanuvchi va admin uchun bu maydonlar BO'SH.
func TestIdentityCarriesPartnerRole(t *testing.T) {
	svc, err := New([]Client{{
		ID: "merchant", Name: "OnDex Merchant Panel", Secret: testSecret,
		RedirectURIs: []string{testRedirect},
	}}, fakeUsers{
		"store": {ID: "store", Phone: "+998901110001", Name: "Baraka do'koni", Role: users.RoleStore, EntityID: "s1"},
		"qurt":  {ID: "qurt", Phone: "+998901110007", Name: "Qurt", Role: users.RoleStore, EntityID: "s2", StoreCategory: users.StoreCategoryQurt},
		"rest":  {ID: "rest", Phone: "+998901110002", Name: "Aziz", Role: users.RoleRestaurant, EntityID: "r1"},
		"restx": {ID: "restx", Phone: "+998901110003", Name: "Bobur", Role: users.RoleRestaurant, EntityID: "r2"},
		"cust":  {ID: "cust", Phone: "+998901110004", Name: "Ali", Role: users.RoleCustomer},
		"admin": {ID: "admin", Phone: "+998901110005", Name: "Admin", Role: users.RoleAdmin, EntityID: "x"},
		"noent": {ID: "noent", Phone: "+998901110006", Name: "Bosh", Role: users.RoleStore},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc.WithEntityNamer(func(_ context.Context, _ users.Role, id string) string {
		if id == "r1" {
			return "Choyxona Navo"
		}
		return "" // r2: katalogda nom yo'q
	})

	exchange := func(uid string) *Identity {
		t.Helper()
		loc, err := svc.Authorize(uid, "merchant", testRedirect, "")
		if err != nil {
			t.Fatal(err)
		}
		code, _ := codeFrom(t, loc)
		id, err := svc.Exchange(context.Background(), "merchant", testSecret, code, testRedirect)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	if id := exchange("qurt"); id.StoreCategory != "qurt" {
		t.Errorf("qurt kategoriyasi berilmadi: %+v", id)
	}
	if id := exchange("store"); id.Role != "store" || id.EntityID != "s1" || id.EntityName != "Baraka do'koni" || id.StoreCategory != "store" {
		t.Errorf("do'kon: %+v", id)
	}
	if id := exchange("rest"); id.StoreCategory != "" || id.Role != "restaurant" || id.EntityID != "r1" || id.EntityName != "Choyxona Navo" {
		t.Errorf("restoran: nom katalogdan olinishi kerak: %+v", id)
	}
	if id := exchange("restx"); id.EntityName != "Bobur" {
		t.Errorf("katalogda nom yo'q bo'lsa akkaunt ismi: %+v", id)
	}
	for _, uid := range []string{"cust", "admin", "noent"} {
		if id := exchange(uid); id.Role != "" || id.EntityID != "" || id.EntityName != "" {
			t.Errorf("%s uchun rol/muassasa BERILMASLIGI kerak: %+v", uid, id)
		}
	}
}

func TestStateIsOptionalAndLengthLimited(t *testing.T) {
	svc, _ := newTestService(t)

	loc, err := svc.Authorize("u1", "merchant", testRedirect, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, state := codeFrom(t, loc); state != "" {
		t.Fatalf("bo'sh state qaytarilmasligi kerak edi: %q", state)
	}

	long := strings.Repeat("a", MaxStateLen+1)
	if _, err := svc.Authorize("u1", "merchant", testRedirect, long); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("uzun state rad etilishi kerak edi: %v", err)
	}
}

// Kod faqat bir marta ishlaydi.
func TestCodeIsSingleUse(t *testing.T) {
	svc, _ := newTestService(t)
	loc, _ := svc.Authorize("u1", "merchant", testRedirect, "")
	code, _ := codeFrom(t, loc)

	if _, err := svc.Exchange(context.Background(), "merchant", testSecret, code, testRedirect); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Exchange(context.Background(), "merchant", testSecret, code, testRedirect); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("ikkinchi almashtirish rad etilishi kerak edi: %v", err)
	}
}

func TestCodeExpires(t *testing.T) {
	svc, now := newTestService(t)
	loc, _ := svc.Authorize("u1", "merchant", testRedirect, "")
	code, _ := codeFrom(t, loc)

	*now = now.Add(CodeTTL + time.Second)
	if _, err := svc.Exchange(context.Background(), "merchant", testSecret, code, testRedirect); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("muddati o'tgan kod rad etilishi kerak edi: %v", err)
	}
}

// Eng xavfli hujum: kodni hujumchi saytiga yo'naltirish. Manzil ANIQ mos
// kelishi shart — prefiks, qo'shimcha yo'l, boshqa domen o'tmaydi.
func TestRedirectMustMatchExactly(t *testing.T) {
	svc, _ := newTestService(t)
	bad := []string{
		"https://evil.example/auth/callback",
		"https://pos.ondex.uz/auth/callback/",             // oxirida /
		"https://pos.ondex.uz/auth/callback?next=evil",    // query
		"https://pos.ondex.uz.evil.example/auth/callback", // domen ichiga o'xshash
		"https://pos.ondex.uz@evil.example/auth/callback", // user@host
		"http://pos.ondex.uz/auth/callback",               // http
		"https://POS.ondex.uz/auth/callback",              // registr
		"",
	}
	for _, uri := range bad {
		if _, err := svc.Authorize("u1", "merchant", uri, ""); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("manzil %q qabul qilindi (xato: %v)", uri, err)
		}
		if _, _, err := svc.ClientInfo("merchant", uri); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("ClientInfo manzil %q ni qabul qildi (xato: %v)", uri, err)
		}
	}
	if _, err := svc.Authorize("u1", "yoq-klient", testRedirect, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("noma'lum klient qabul qilindi: %v", err)
	}
}

func TestAuthorizeNeedsUser(t *testing.T) {
	svc, _ := newTestService(t)
	if _, err := svc.Authorize("", "merchant", testRedirect, ""); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("foydalanuvchisiz ruxsat berildi: %v", err)
	}
}

// Kod avval berilgan manzilga bog'langan: boshqa (hatto ro'yxatdagi)
// manzil bilan almashtirib bo'lmaydi.
func TestExchangeRejectsDifferentRedirect(t *testing.T) {
	svc, _ := newTestService(t)
	loc, _ := svc.Authorize("u1", "merchant", testRedirect, "")
	code, _ := codeFrom(t, loc)

	_, err := svc.Exchange(context.Background(), "merchant", testSecret, code,
		"http://localhost:5173/auth/callback")
	if !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("boshqa manzil bilan almashtirildi: %v", err)
	}
}

// Noto'g'ri sir kodni ISTE'MOL QILMASLIGI kerak: aks holda sirni bilmaydigan
// odam foydalanuvchining kirishini buzib turishi mumkin.
func TestWrongSecretDoesNotBurnCode(t *testing.T) {
	svc, _ := newTestService(t)
	loc, _ := svc.Authorize("u1", "merchant", testRedirect, "")
	code, _ := codeFrom(t, loc)

	if _, err := svc.Exchange(context.Background(), "merchant", "noto'g'ri-sir", code, testRedirect); !errors.Is(err, ErrClientAuth) {
		t.Fatalf("noto'g'ri sir qabul qilindi: %v", err)
	}
	if _, err := svc.Exchange(context.Background(), "merchant", testSecret, code, testRedirect); err != nil {
		t.Fatalf("to'g'ri sir bilan almashtirish ishlashi kerak edi: %v", err)
	}
}

func TestUnknownClientAndEmptySecretRejected(t *testing.T) {
	svc, _ := newTestService(t)
	loc, _ := svc.Authorize("u1", "merchant", testRedirect, "")
	code, _ := codeFrom(t, loc)

	for _, c := range []struct{ id, secret string }{
		{"yoq-klient", testSecret},
		{"yoq-klient", ""},
		{"merchant", ""},
	} {
		if _, err := svc.Exchange(context.Background(), c.id, c.secret, code, testRedirect); !errors.Is(err, ErrClientAuth) {
			t.Errorf("klient %q sir %q qabul qilindi: %v", c.id, c.secret, err)
		}
	}
}

// Bir klientning kodi boshqa klientda ishlamaydi.
func TestCodeBoundToClient(t *testing.T) {
	svc, err := New([]Client{
		{ID: "merchant", Name: "A", Secret: testSecret, RedirectURIs: []string{testRedirect}},
		{ID: "other", Name: "B", Secret: strings.Repeat("z", 40), RedirectURIs: []string{"https://other.example/cb"}},
	}, fakeUsers{"u1": {ID: "u1", Phone: "+998901234567"}})
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := svc.Authorize("u1", "merchant", testRedirect, "")
	code, _ := codeFrom(t, loc)

	_, err = svc.Exchange(context.Background(), "other", strings.Repeat("z", 40), code, "https://other.example/cb")
	if !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("boshqa klient kodni oldi: %v", err)
	}
}

func TestExchangeUnknownOrDeletedUser(t *testing.T) {
	svc, _ := newTestService(t)

	// Kod berilgandan keyin foydalanuvchi o'chiriladi.
	loc, _ := svc.Authorize("u1", "merchant", testRedirect, "")
	code, _ := codeFrom(t, loc)
	del := time.Now()
	svc.users.(fakeUsers)["u1"].DeletedAt = &del
	if _, err := svc.Exchange(context.Background(), "merchant", testSecret, code, testRedirect); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("o'chirilgan akkaunt kirdi: %v", err)
	}

	// Ombordan butunlay yo'q foydalanuvchi.
	loc, _ = svc.Authorize("yoq", "merchant", testRedirect, "")
	code, _ = codeFrom(t, loc)
	if _, err := svc.Exchange(context.Background(), "merchant", testSecret, code, testRedirect); !errors.Is(err, ErrInvalidGrant) {
		t.Fatalf("mavjud bo'lmagan foydalanuvchi kirdi: %v", err)
	}
}

// Xotirada kodning o'zi emas, xeshi turadi.
func TestOnlyHashIsStored(t *testing.T) {
	svc, _ := newTestService(t)
	loc, _ := svc.Authorize("u1", "merchant", testRedirect, "")
	code, _ := codeFrom(t, loc)

	if _, ok := svc.codes[hashCode(code)]; !ok {
		t.Fatal("kod xeshi ombordan topilmadi")
	}
	if len(svc.codes) != 1 {
		t.Fatalf("kutilgan 1 ta yozuv, bor: %d", len(svc.codes))
	}
}

func TestExpiredCodesAreSweptAndCapEnforced(t *testing.T) {
	svc, now := newTestService(t)
	for i := 0; i < 5; i++ {
		if _, err := svc.Authorize("u1", "merchant", testRedirect, ""); err != nil {
			t.Fatal(err)
		}
	}
	*now = now.Add(CodeTTL + time.Second)
	if _, err := svc.Authorize("u1", "merchant", testRedirect, ""); err != nil {
		t.Fatal(err)
	}
	if len(svc.codes) != 1 {
		t.Fatalf("eskirgan kodlar tozalanmadi: %d ta qoldi", len(svc.codes))
	}

	// Chegaraga yetganda yangi kod berilmaydi.
	svc.mu.Lock()
	for i := 0; i < maxPending; i++ {
		var k [32]byte
		k[0], k[1], k[2] = byte(i), byte(i>>8), byte(i>>16)
		svc.codes[k] = entry{expires: now.Add(time.Hour)}
	}
	svc.mu.Unlock()
	if _, err := svc.Authorize("u1", "merchant", testRedirect, ""); !errors.Is(err, ErrBusy) {
		t.Fatalf("chegaradan keyin kod berildi: %v", err)
	}
}

// Zaif sozlama server ishga tushishida rad etilishi kerak.
func TestNewRejectsWeakConfig(t *testing.T) {
	ok := Client{ID: "m", Name: "M", Secret: testSecret, RedirectURIs: []string{testRedirect}}
	users := fakeUsers{}

	cases := map[string]func(c *Client){
		"qisqa sir":            func(c *Client) { c.Secret = "qisqa" },
		"bo'sh nom":            func(c *Client) { c.Name = " " },
		"yomon id":             func(c *Client) { c.ID = "Katta Harf" },
		"bo'sh id":             func(c *Client) { c.ID = "" },
		"manzillar yo'q":       func(c *Client) { c.RedirectURIs = nil },
		"http (localhost'siz)": func(c *Client) { c.RedirectURIs = []string{"http://pos.ondex.uz/cb"} },
		"fragment":             func(c *Client) { c.RedirectURIs = []string{"https://pos.ondex.uz/cb#x"} },
		"user@host":            func(c *Client) { c.RedirectURIs = []string{"https://a@pos.ondex.uz/cb"} },
		"sxemasiz":             func(c *Client) { c.RedirectURIs = []string{"pos.ondex.uz/cb"} },
		"ftp":                  func(c *Client) { c.RedirectURIs = []string{"ftp://pos.ondex.uz/cb"} },
	}
	for name, mutate := range cases {
		c := ok
		c.RedirectURIs = append([]string(nil), ok.RedirectURIs...)
		mutate(&c)
		if _, err := New([]Client{c}, users); err == nil {
			t.Errorf("%s: zaif sozlama qabul qilindi", name)
		}
	}

	if _, err := New([]Client{ok, ok}, users); err == nil {
		t.Error("takroriy klient id qabul qilindi")
	}
	if _, err := New(nil, users); err == nil {
		t.Error("klientsiz xizmat yaratildi")
	}
	if _, err := New([]Client{ok}, nil); err == nil {
		t.Error("foydalanuvchi omborisiz xizmat yaratildi")
	}
	if _, err := New([]Client{ok}, users); err != nil {
		t.Fatalf("to'g'ri sozlama rad etildi: %v", err)
	}
}

func TestParseRedirectURIs(t *testing.T) {
	got := ParseRedirectURIs(" https://a.uz/cb , ,http://localhost:1/cb,")
	if len(got) != 2 || got[0] != "https://a.uz/cb" || got[1] != "http://localhost:1/cb" {
		t.Fatalf("noto'g'ri bo'laklandi: %q", got)
	}
	if len(ParseRedirectURIs("")) != 0 {
		t.Fatal("bo'sh qator bo'sh ro'yxat berishi kerak edi")
	}
}

// Lookup: klient siri bilan, kodsiz; Exchange bilan bir xil kimlik.
func TestLookupNeedsClientSecretAndMatchesExchange(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	if _, err := svc.Lookup(ctx, "merchant", "noto'g'ri-sir", "u1"); !errors.Is(err, ErrClientAuth) {
		t.Errorf("noto'g'ri sir: %v", err)
	}
	if _, err := svc.Lookup(ctx, "yoq", testSecret, "u1"); !errors.Is(err, ErrClientAuth) {
		t.Errorf("noma'lum klient: %v", err)
	}
	if _, err := svc.Lookup(ctx, "merchant", testSecret, "yoq"); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("noma'lum foydalanuvchi: %v", err)
	}
	if _, err := svc.Lookup(ctx, "merchant", testSecret, ""); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("bo'sh user_id: %v", err)
	}
	id, err := svc.Lookup(ctx, "merchant", testSecret, "u1")
	if err != nil || id.UserID != "u1" || id.Phone != "+998901234567" {
		t.Fatalf("lookup: %+v %v", id, err)
	}
}
