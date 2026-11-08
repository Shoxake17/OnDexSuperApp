package identity_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// Yangi qurilma xabari (135): faqat merchant realmi, faqat yangi UserAgent,
// OIDC bola sessiyasi alohida xabar bermaydi.
func TestNewDevice_OnlyUnknownMerchantDevices(t *testing.T) {
	f := newFixture(t)
	f.user(t, "u_newdev01", users.RoleRestaurant)
	f.user(t, "u_newdev02", users.RoleCustomer)

	var mu sync.Mutex
	var got []identity.NewDevice
	done := make(chan struct{}, 10)
	f.svc.OnNewDevice(func(_ context.Context, d identity.NewDevice) {
		mu.Lock()
		got = append(got, d)
		mu.Unlock()
		done <- struct{}{}
	})
	create := func(uid, client, ua, parent string) *identity.Tokens {
		t.Helper()
		tok, err := f.svc.Create(f.ctx, identity.CreateInput{UserID: uid, ClientID: client, ParentID: parent,
			AMR: []string{"otp"}, Meta: identity.Meta{IP: "198.51.100.7", UserAgent: ua}})
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	wait := func(n int) {
		t.Helper()
		for i := 0; i < n; i++ {
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("xabar kelmadi (%d/%d)", i, n)
			}
		}
		select {
		case <-done:
			t.Fatal("ortiqcha xabar")
		case <-time.After(100 * time.Millisecond):
		}
	}

	const chrome = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/130.0 Safari/537.36"
	sso := create("u_newdev01", identity.SSOClientID, chrome, "")
	wait(1) // birinchi kirish — yangi
	create("u_newdev01", identity.SSOClientID, chrome, "")
	wait(0) // o'sha brauzer — xabar yo'q
	create("u_newdev01", "merchant-web", chrome+" x", sso.SessionID)
	wait(0) // OIDC bola sessiyasi — ota xabar bergan
	create("u_newdev01", "ondex-pro", "Dart/3.5 (dart:io) android", "")
	wait(1) // boshqa qurilma

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 2 || got[0].UserID != "u_newdev01" || got[0].IP != "198.51.100.7" || got[1].ClientID != "ondex-pro" {
		t.Fatalf("xabarlar: %+v", got)
	}
	txt := identity.NewDeviceText(got[0], "OnDex ID (brauzer)", "https://passport.ondex.uz/v2/oauth/account", nil)
	for _, want := range []string{"YANGI qurilma", "Chrome, Windows", "198.51.100.7", "passport.ondex.uz/v2/oauth/account"} {
		if !strings.Contains(txt, want) {
			t.Errorf("xabarda %q yo'q:\n%s", want, txt)
		}
	}
}

func TestDeviceSummary(t *testing.T) {
	for ua, want := range map[string]string{
		"Mozilla/5.0 (Windows NT 10.0) AppleWebKit Chrome/130 Safari/537 Edg/130": "Edge, Windows",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Safari/604.1":     "Safari, iOS",
		"Dart/3.5 (dart:io)": "OnDex ilovasi",
		"":                   "",
	} {
		if got := identity.DeviceSummary(ua); got != want {
			t.Errorf("%q -> %q, kutilgan %q", ua, got, want)
		}
	}
}
