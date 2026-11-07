package identity_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// fakeUsers — parol va telefon kodi (bir martalik). Haqiqiy `users.Service`
// qoidalari users paketining o'z testlarida; bu yerda — oqim mantig'i.
type fakeUsers struct {
	mu        sync.Mutex
	repo      users.Repository
	passwords map[string]string // telefon -> parol
	codes     map[string]string // telefon -> kod
}

func (f *fakeUsers) AuthenticatePasswordRealm(ctx context.Context, realm users.Realm, login, password string) (*users.User, error) {
	f.mu.Lock()
	pw, ok := f.passwords[login]
	f.mu.Unlock()
	if !ok || pw != password {
		return nil, users.ErrInvalidCredentials
	}
	u, err := f.repo.GetByPhoneRealm(ctx, realm, login)
	if err != nil {
		return nil, users.ErrInvalidCredentials // haqiqiy servis kabi: topilmadi = noto'g'ri
	}
	return u, nil
}

func (f *fakeUsers) VerifyPhoneCode(_ context.Context, phone, code string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.codes[phone]; !ok || c != code {
		return "", users.ErrInvalidCode
	}
	delete(f.codes, phone)
	return phone, nil
}

func (f *fakeUsers) sendCode(phone string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.codes[phone] = "424242"
	return "424242"
}

type fakeBox struct{}

func (fakeBox) Seal(p string) (string, error) { return "enc:" + p, nil }
func (fakeBox) Open(e string) (string, error) {
	if !strings.HasPrefix(e, "enc:") {
		return "", errors.New("buzuq")
	}
	return strings.TrimPrefix(e, "enc:"), nil
}

type merchantFixture struct {
	*fixture
	m     *identity.MerchantAuth
	fu    *fakeUsers
	az    *authz.MemStore
	mfaSt *mfa.MemStore
}

func newMerchantFixture(t *testing.T) *merchantFixture {
	f := newFixture(t)
	fu := &fakeUsers{repo: f.users, passwords: map[string]string{}, codes: map[string]string{}}
	az := authz.NewMemStore()
	if err := authz.EnsureTemplates(f.ctx, az); err != nil {
		t.Fatal(err)
	}
	ms := mfa.NewMemStore()
	m, err := identity.NewMerchantAuth(f.svc, identity.MerchantDeps{Users: fu, Repo: f.users,
		Authz: authz.New(az), MFA: ms, Box: fakeBox{}})
	if err != nil {
		t.Fatal(err)
	}
	return &merchantFixture{fixture: f, m: m, fu: fu, az: az, mfaSt: ms}
}

func (f *merchantFixture) phoneOf(t *testing.T, id string) string {
	u, err := f.users.GetByID(f.ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return u.Phone
}

var meta = identity.Meta{IP: "10.0.0.1", UserAgent: "test", DeviceName: "Windows"}

// enroll — egasini birinchi marta to'liq o'tkazadi: parol -> kod -> TOTP ulash.
func (f *merchantFixture) enroll(t *testing.T, id string) (secret string, recovery []string) {
	t.Helper()
	phone := f.phoneOf(t, id)
	out, err := f.m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta)
	if err != nil || out.MFA == nil || out.MFA.Method != "otp" || out.Tokens != nil {
		t.Fatalf("parol bosqichi: %+v %v", out, err)
	}
	out, err = f.m.AddFactor(f.ctx, out.MFA.Token, f.fu.sendCode(phone), meta)
	if err != nil || out.MFA == nil || out.MFA.Method != "totp_enroll" {
		t.Fatalf("kod bosqichi: %+v %v", out, err)
	}
	en, err := f.m.EnrollStart(f.ctx, out.MFA.Token)
	if err != nil || !strings.HasPrefix(en.URI, "otpauth://totp/") {
		t.Fatalf("EnrollStart: %+v %v", en, err)
	}
	code, _ := mfa.Code(en.Secret, f.clk.now())
	tok, rec, err := f.m.EnrollConfirm(f.ctx, out.MFA.Token, code, meta)
	if err != nil || tok == nil || len(rec) != mfa.RecoveryCount {
		t.Fatalf("EnrollConfirm: %v %d", err, len(rec))
	}
	c, err := f.svc.Authenticate(f.ctx, tok.AccessToken, users.RealmMerchant)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"pwd", "otp", "totp"} {
		if !slices.Contains(c.AMR, a) {
			t.Fatalf("amr %v da %q yo'q", c.AMR, a)
		}
	}
	return en.Secret, rec
}

func TestMerchant_OwnerEnrollThenTOTPEveryLogin(t *testing.T) {
	f := newMerchantFixture(t)
	f.user(t, "u_owner01", users.RoleRestaurant)
	phone := f.phoneOf(t, "u_owner01")
	f.fu.passwords[phone] = "parol-123"
	secret, _ := f.enroll(t, "u_owner01")

	// Keyingi kirish: parol -> TOTP (ulash qayta so'ralmaydi).
	out, err := f.m.LoginPassword(f.ctx, phone, "parol-123", "merchant-desktop", meta)
	if err != nil || out.MFA == nil || out.MFA.Method != "totp" {
		t.Fatalf("ikkinchi kirish: %+v %v", out, err)
	}
	// Ulashda ishlatilgan kod (o'sha qadam) qayta qabul qilinmaydi.
	same, _ := mfa.Code(secret, f.clk.now())
	if _, err := f.m.VerifyMFA(f.ctx, out.MFA.Token, same, meta); !errors.Is(err, identity.ErrMFAInvalid) {
		t.Fatalf("takroriy TOTP: %v", err)
	}
	f.clk.add(mfa.Period)
	next, _ := mfa.Code(secret, f.clk.now())
	tok, err := f.m.VerifyMFA(f.ctx, out.MFA.Token, next, meta)
	if err != nil || tok == nil {
		t.Fatalf("to'g'ri TOTP: %v", err)
	}
	if _, err := f.m.VerifyMFA(f.ctx, out.MFA.Token, next, meta); !errors.Is(err, identity.ErrMFAInvalid) {
		t.Fatalf("bitta kod ikki sessiya ochdi: %v", err)
	}

	// Kod bilan kirgan egasi ham TOTP'siz kirmaydi.
	out, err = f.m.LoginCode(f.ctx, phone, f.fu.sendCode(phone), "merchant-web", meta)
	if err != nil || out.Tokens != nil || out.MFA == nil || out.MFA.Method != "totp" {
		t.Fatalf("kod bilan egasi: %+v %v", out, err)
	}
}

// Faqat parolni bilgan hujumchi TOTP'ni O'ZI ulab ololmaydi.
func TestMerchant_EnrollNeedsBothPasswordAndPhone(t *testing.T) {
	f := newMerchantFixture(t)
	f.user(t, "u_owner02", users.RoleStore)
	phone := f.phoneOf(t, "u_owner02")
	f.fu.passwords[phone] = "parol-123"
	out, err := f.m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta)
	if err != nil || out.MFA.Method != "otp" {
		t.Fatalf("%+v %v", out, err)
	}
	if _, err := f.m.EnrollStart(f.ctx, out.MFA.Token); !errors.Is(err, identity.ErrMFATokenInvalid) {
		t.Fatalf("faqat parol bilan ulash boshlandi: %v", err)
	}
	if _, err := f.m.AddFactor(f.ctx, out.MFA.Token, "000000", meta); err == nil {
		t.Fatal("noto'g'ri telefon kodi qabul qilindi")
	}
	// Kod bilan boshlaganda — parol so'raladi.
	out, err = f.m.LoginCode(f.ctx, phone, f.fu.sendCode(phone), "merchant-web", meta)
	if err != nil || out.MFA.Method != "password" {
		t.Fatalf("kod bilan boshlagan egasi: %+v %v", out, err)
	}
	if _, err := f.m.AddFactor(f.ctx, out.MFA.Token, "notogri", meta); !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("noto'g'ri parol: %v", err)
	}
	out, err = f.m.AddFactor(f.ctx, out.MFA.Token, "parol-123", meta)
	if err != nil || out.MFA.Method != "totp_enroll" {
		t.Fatalf("parol qo'shilgach: %+v %v", out, err)
	}
}

func TestMerchant_RecoveryCodeOnce(t *testing.T) {
	f := newMerchantFixture(t)
	f.user(t, "u_owner03", users.RoleRestaurant)
	phone := f.phoneOf(t, "u_owner03")
	f.fu.passwords[phone] = "parol-123"
	_, rec := f.enroll(t, "u_owner03")
	login := func() string {
		out, err := f.m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta)
		if err != nil {
			t.Fatal(err)
		}
		return out.MFA.Token
	}
	tok, err := f.m.VerifyMFA(f.ctx, login(), strings.ToLower(rec[0]), meta)
	if err != nil {
		t.Fatalf("tiklash kodi: %v", err)
	}
	c, _ := f.svc.Authenticate(f.ctx, tok.AccessToken)
	if !slices.Contains(c.AMR, "recovery") {
		t.Fatalf("amr: %v", c.AMR)
	}
	if _, err := f.m.VerifyMFA(f.ctx, login(), rec[0], meta); !errors.Is(err, identity.ErrMFAInvalid) {
		t.Fatalf("tiklash kodi ikki marta: %v", err)
	}
}

func TestMerchant_StaffCodeOnlyAndAppScoped(t *testing.T) {
	f := newMerchantFixture(t)
	// Eski (ko'chirilmagan) ofitsiant: faqat OnDexPro.
	f.user(t, "u_waiter1", users.RoleWaiter)
	wp := f.phoneOf(t, "u_waiter1")
	f.fu.passwords[wp] = "parol-123"
	if _, err := f.m.LoginPassword(f.ctx, wp, "parol-123", "ondex-pro", meta); !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("xodim parol bilan kirdi: %v", err)
	}
	out, err := f.m.LoginCode(f.ctx, wp, f.fu.sendCode(wp), "ondex-pro", meta)
	if err != nil || out.Tokens == nil {
		t.Fatalf("ofitsiant OnDexPro: %+v %v", out, err)
	}
	if _, err := f.m.LoginCode(f.ctx, wp, f.fu.sendCode(wp), "ondex-go", meta); !errors.Is(err, identity.ErrAppNotAllowed) {
		t.Fatalf("ofitsiant OnDexGO: %v", err)
	}

	// A'zolik orqali xodim: ilovalar a'zolikdan.
	f.user(t, "u_ownerX1", users.RoleRestaurant)
	f.user(t, "u_cook001", users.RoleWaiter)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.az.CreateOrganization(f.ctx, &authz.Organization{ID: "o1", Name: "O", OwnerUserID: "u_ownerX1"}))
	must(f.az.CreateBusiness(f.ctx, &authz.Business{ID: "b1", OrgID: "o1", Type: authz.TypeRestaurant, Name: "B"}))
	must(f.az.CreateMembership(f.ctx, &authz.Membership{ID: "m1", UserID: "u_cook001", OrgID: "o1", BusinessID: "b1",
		RoleID: authz.Template{Type: authz.TypeRestaurant, Key: "cook"}.ID(), Apps: []string{authz.AppPanel}, Status: authz.MemberActive}))
	cp := f.phoneOf(t, "u_cook001")
	out, err = f.m.LoginCode(f.ctx, cp, f.fu.sendCode(cp), "merchant-web", meta)
	if err != nil || out.Tokens == nil {
		t.Fatalf("a'zolikdagi xodim panelga: %+v %v", out, err)
	}
	c, _ := f.svc.Authenticate(f.ctx, out.Tokens.AccessToken)
	if c.OrgID != "o1" {
		t.Fatalf("sessiya tashkiloti %q", c.OrgID)
	}
	// a'zolik bor, lekin 'pro' ruxsat etilmagan (eski rol ko'prigi a'zolikdan ustun EMAS)
	if _, err := f.m.LoginCode(f.ctx, cp, f.fu.sendCode(cp), "ondex-pro", meta); !errors.Is(err, identity.ErrAppNotAllowed) {
		t.Fatalf("ruxsatsiz ilova: %v", err)
	}
}

func TestMerchant_RejectsCustomersAndForeignClients(t *testing.T) {
	f := newMerchantFixture(t)
	f.user(t, "u_custom1", users.RoleCustomer)
	p := f.phoneOf(t, "u_custom1")
	f.fu.passwords[p] = "parol-123"
	if _, err := f.m.LoginPassword(f.ctx, p, "parol-123", "merchant-web", meta); !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("mijoz parol bilan: %v", err)
	}
	if _, err := f.m.LoginCode(f.ctx, p, f.fu.sendCode(p), "merchant-web", meta); !errors.Is(err, users.ErrInvalidCredentials) {
		t.Fatalf("mijoz kod bilan: %v", err)
	}
	for _, c := range []string{"admin-web", "noma'lum", ""} {
		if _, err := f.m.LoginCode(f.ctx, p, "1", c, meta); !errors.Is(err, identity.ErrUnknownClient) {
			t.Errorf("klient %q: %v", c, err)
		}
	}
}

func TestMerchant_MFATokenHardening(t *testing.T) {
	f := newMerchantFixture(t)
	f.user(t, "u_owner04", users.RoleRestaurant)
	phone := f.phoneOf(t, "u_owner04")
	f.fu.passwords[phone] = "parol-123"
	secret, _ := f.enroll(t, "u_owner04")
	out, err := f.m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta)
	if err != nil {
		t.Fatal(err)
	}
	tok := out.MFA.Token
	f.clk.add(mfa.Period)
	code, _ := mfa.Code(secret, f.clk.now())
	for name, bad := range map[string]string{
		"imzo buzilgan": tok[:len(tok)-2] + "xx",
		"prefikssiz":    strings.TrimPrefix(tok, "omfa_"),
		"bo'sh":         "",
		"uzun":          strings.Repeat("a", 5000),
	} {
		if _, err := f.m.VerifyMFA(f.ctx, bad, code, meta); !errors.Is(err, identity.ErrMFATokenInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Maqsad boshqa: TOTP tokeni bilan ulash boshlanmaydi.
	if _, err := f.m.EnrollStart(f.ctx, tok); !errors.Is(err, identity.ErrMFATokenInvalid) {
		t.Errorf("TOTP tokeni bilan EnrollStart: %v", err)
	}
	f.clk.add(6 * time.Minute)
	code, _ = mfa.Code(secret, f.clk.now())
	if _, err := f.m.VerifyMFA(f.ctx, tok, code, meta); !errors.Is(err, identity.ErrMFATokenInvalid) {
		t.Errorf("muddati o'tgan token: %v", err)
	}
}

func TestMerchant_TOTPBruteForceLimited(t *testing.T) {
	f := newMerchantFixture(t)
	f.user(t, "u_owner05", users.RoleRestaurant)
	phone := f.phoneOf(t, "u_owner05")
	f.fu.passwords[phone] = "parol-123"
	f.enroll(t, "u_owner05")
	out, _ := f.m.LoginPassword(f.ctx, phone, "parol-123", "merchant-web", meta)
	var last error
	for range 10 {
		_, last = f.m.VerifyMFA(f.ctx, out.MFA.Token, "000000", meta)
	}
	if !errors.Is(last, identity.ErrMFATooMany) {
		t.Fatalf("10 ta noto'g'ri urinishdan keyin: %v", last)
	}
}
