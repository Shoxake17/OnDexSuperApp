package mcconnect

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"chustapp/internal/payments/multicard"
	"chustapp/internal/secretstore"
)

const testKey = "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0"

// gateway — `/auth` ni sanaydigan soxta shlyuz.
type gateway struct {
	auths    atomic.Int32
	lastApp  atomic.Value
	lastSecr atomic.Value
	srv      *httptest.Server
}

func newGateway(t *testing.T) *gateway {
	t.Helper()
	g := &gateway{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		g.auths.Add(1)
		var b struct {
			ApplicationID string `json:"application_id"`
			Secret        string `json:"secret"`
		}
		//nolint:errcheck // soxta server
		json.NewDecoder(r.Body).Decode(&b)
		g.lastApp.Store(b.ApplicationID)
		g.lastSecr.Store(b.Secret)
		_, _ = w.Write([]byte(`{"token":"tok","role":"dev","expiry":"` +
			time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05") + `"}`))
	})
	mux.HandleFunc("GET /payment/store/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":6,"title":"Kassa"}}`))
	})
	g.srv = httptest.NewServer(mux)
	t.Cleanup(g.srv.Close)
	return g
}

func mustBox(t *testing.T) *secretstore.Box {
	t.Helper()
	b, err := secretstore.New(testKey)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func lookupOf(c Credentials) Lookup {
	return func(context.Context, string) (Credentials, error) { return c, nil }
}

func TestCanConnectNeedsBothURLAndKey(t *testing.T) {
	box := mustBox(t)
	cases := map[string]struct {
		url string
		box *secretstore.Box
		ok  bool
	}{
		"ikkalasi bor":    {"https://x", box, true},
		"manzil yo'q":     {"", box, false},
		"kalit yo'q":      {"https://x", nil, false},
		"hech narsa yo'q": {"", nil, false},
	}
	for name, c := range cases {
		r := NewRegistry(nil, c.box, c.url, nil)
		if r.CanConnect() != c.ok {
			t.Errorf("%s: %v kutildi", name, c.ok)
		}
	}
	var nilReg *Registry
	if nilReg.CanConnect() {
		t.Error("nil reyestr ulana oladi dedi")
	}
}

// O'z kalitlari bor restoran — O'Z klienti bilan ishlaydi.
func TestClientUsesOwnCredentials(t *testing.T) {
	g := newGateway(t)
	box := mustBox(t)
	enc, err := box.Seal("restoran-siri")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(lookupOf(Credentials{
		AppID: "bookcafe", SecretEnc: enc, StoreID: "6",
	}), box, g.srv.URL, nil)

	cl, creds, err := r.ClientFor(context.Background(), "rest-a")
	if err != nil {
		t.Fatal(err)
	}
	if !creds.OwnCredentials() || !creds.Connected() {
		t.Fatalf("ulanish: %+v", creds)
	}
	if _, err := cl.Store(context.Background(), "6"); err != nil {
		t.Fatal(err)
	}
	if got, _ := g.lastApp.Load().(string); got != "bookcafe" {
		t.Errorf("shlyuzga ketgan app: %q", got)
	}
	if got, _ := g.lastSecr.Load().(string); got != "restoran-siri" {
		t.Errorf("shlyuzga ketgan sir: %q", got)
	}
}

// Kesh: ikkinchi chaqiruvda yangi klient ham, yangi `/auth` ham
// bo'lmasligi kerak.
func TestClientIsCachedPerAppID(t *testing.T) {
	g := newGateway(t)
	box := mustBox(t)
	enc, _ := box.Seal("sir")
	creds := Credentials{AppID: "app-1", SecretEnc: enc, StoreID: "6"}
	r := NewRegistry(lookupOf(creds), box, g.srv.URL, nil)

	first, err := r.Client(creds)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Store(context.Background(), "6"); err != nil {
		t.Fatal(err)
	}
	second, err := r.Client(creds)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Error("kesh ishlamadi — yangi klient yasaldi")
	}
	if n := g.auths.Load(); n != 1 {
		t.Errorf("/auth chaqiruvlari: %d (1 kutildi)", n)
	}

	// Kalit almashsa kesh tozalanishi kerak.
	r.Invalidate("app-1")
	third, err := r.Client(creds)
	if err != nil {
		t.Fatal(err)
	}
	if third == first {
		t.Error("Invalidate keshni tozalamadi")
	}
}

// O'z kalitlari yo'q restoran — platforma klienti.
func TestClientFallsBackToPlatform(t *testing.T) {
	g := newGateway(t)
	platform, err := multicard.New(multicard.Config{
		BaseURL: g.srv.URL, ApplicationID: "ondex", Secret: "platforma",
	})
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(nil, mustBox(t), g.srv.URL, platform)

	cl, err := r.Client(Credentials{StoreID: "6"})
	if err != nil {
		t.Fatal(err)
	}
	if cl != platform {
		t.Error("platforma klienti qaytmadi")
	}

	// Platforma ham yo'q, o'z kalitlari ham yo'q — xato EMAS,
	// "ulanmagan" (sahifa ulash formasini ko'rsatadi).
	r2 := NewRegistry(nil, mustBox(t), g.srv.URL, nil)
	cl2, err := r2.Client(Credentials{StoreID: "6"})
	if err != nil || cl2 != nil {
		t.Errorf("nil, nil kutildi: %v, %v", cl2, err)
	}
}

// Shifrlash kaliti almashtirilgan bo'lsa — jimgina "ulanmagan" emas,
// ANIQ xato: restoran nima qilishni bilishi kerak.
func TestClientFailsOnUndecryptableSecret(t *testing.T) {
	g := newGateway(t)
	other, err := secretstore.New("ab" + testKey[2:])
	if err != nil {
		t.Fatal(err)
	}
	enc, err := other.Seal("sir")
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(nil, mustBox(t), g.srv.URL, nil)

	cl, err := r.Client(Credentials{AppID: "app-1", SecretEnc: enc, StoreID: "6"})
	if err == nil {
		t.Fatal("xato kutilgan edi")
	}
	if cl != nil {
		t.Error("klient qaytdi")
	}
	if got := err.Error(); got == "" {
		t.Error("xato matni bo'sh")
	}
}

func TestNewClientNeedsConfiguration(t *testing.T) {
	r := NewRegistry(nil, nil, "", nil)
	if _, err := r.NewClient("a", "s"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("ErrUnavailable kutildi: %v", err)
	}
	if _, err := r.Seal("s"); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Seal: ErrUnavailable kutildi: %v", err)
	}
}
