package identity_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// RS-7: web panel BFF sessiyasi — cookie imzosi, faqat o'z klienti, chiqish,
// muddat va umumiy refresh yo'li taqiqi.
func TestPanelWebSession(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_pw000001", users.RoleRestaurant)
	tok := f.login(t, "u_pw000001", identity.PanelWebClientID)

	sid, ok := f.svc.PanelWebSessionOf(f.ctx, tok.AccessToken)
	if !ok || sid != tok.SessionID {
		t.Fatalf("PanelWebSessionOf: %q %v", sid, ok)
	}
	cookie := f.svc.PanelWebCookie(sid)
	if strings.Contains(cookie, tok.RefreshToken) || !strings.HasPrefix(cookie, sid+".") {
		t.Fatalf("cookie shakli: %q", cookie)
	}

	access, _, err := f.svc.PanelWebAccess(f.ctx, cookie)
	if err != nil {
		t.Fatalf("PanelWebAccess: %v", err)
	}
	c, err := f.svc.Authenticate(f.ctx, access)
	if err != nil || c.ClientID != identity.PanelWebClientID || c.SessionID != sid || c.Subject != "u_pw000001" {
		t.Fatalf("access token: %+v %v", c, err)
	}

	// Imzosiz / soxta / boshqa sessiyaga ko'chirilgan imzo — rad.
	for _, bad := range []string{sid, sid + ".AAAA", "sess_999999." + strings.SplitN(cookie, ".", 2)[1], "", strings.Repeat("x", 300)} {
		if _, _, err := f.svc.PanelWebAccess(f.ctx, bad); !errors.Is(err, identity.ErrNotFound) {
			t.Fatalf("soxta cookie %q qabul qilindi: %v", bad, err)
		}
	}

	// Boshqa klient sessiyasi (imzo to'g'ri bo'lsa ham) — rad.
	other := f.login(t, "u_pw000001", "merchant-desktop")
	if _, ok := f.svc.PanelWebSessionOf(f.ctx, other.AccessToken); ok {
		t.Fatal("merchant-desktop sessiyasi panel sessiyasi deb olindi")
	}
	if _, _, err := f.svc.PanelWebAccess(f.ctx, f.svc.PanelWebCookie(other.SessionID)); err == nil {
		t.Fatal("boshqa klient sessiyasiga cookie orqali access berildi")
	}

	// Chiqish — cookie o'ladi, eski access ham.
	f.svc.PanelWebLogout(f.ctx, cookie)
	if _, _, err := f.svc.PanelWebAccess(f.ctx, cookie); err == nil {
		t.Fatal("chiqishdan keyin cookie ishlayapti")
	}
	if _, err := f.svc.Authenticate(f.ctx, access); err == nil {
		t.Fatal("chiqishdan keyin access ishlayapti")
	}
}

func TestPanelWebSessionExpiresAfter7Days(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_pw000002", users.RoleRestaurant)
	tok := f.login(t, "u_pw000002", identity.PanelWebClientID)
	cookie := f.svc.PanelWebCookie(tok.SessionID)
	f.clk.add(6 * 24 * time.Hour)
	if _, _, err := f.svc.PanelWebAccess(f.ctx, cookie); err != nil {
		t.Fatalf("6-kun: %v", err)
	}
	f.clk.add(25 * time.Hour)
	if _, _, err := f.svc.PanelWebAccess(f.ctx, cookie); err == nil {
		t.Fatal("7 kundan keyin sessiya hali tirik")
	}
}

// Panel sessiyasining refresh tokeni umumiy yo'ldan kelsa — o'g'irlik
// belgisi: rad etiladi va sessiya yopiladi.
func TestPanelWebRefreshPublicForbidden(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_pw000003", users.RoleRestaurant)
	tok := f.login(t, "u_pw000003", identity.PanelWebClientID)
	if _, err := f.svc.RefreshPublic(f.ctx, tok.RefreshToken, identity.Meta{}); err == nil {
		t.Fatal("panel refresh tokeni umumiy yo'ldan qabul qilindi")
	}
	if _, _, err := f.svc.PanelWebAccess(f.ctx, f.svc.PanelWebCookie(tok.SessionID)); err == nil {
		t.Fatal("refresh_public_forbidden dan keyin sessiya tirik")
	}
}

// Mijoz akkaunti panel klientida sessiya ocholmaydi (realm).
func TestPanelWebRejectsCustomer(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_pw000004", users.RoleCustomer)
	if _, err := f.svc.Create(f.ctx, identity.CreateInput{UserID: "u_pw000004", ClientID: identity.PanelWebClientID,
		AMR: []string{"otp"}}); !errors.Is(err, identity.ErrRealmMismatch) {
		t.Fatalf("mijoz panelga kirdi: %v", err)
	}
}
