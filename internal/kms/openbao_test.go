package kms

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"chustapp/internal/identity"
)

// fakeBao — OpenBao Transit API'ning kichik soxtasi (faqat biz ishlatadigan
// yo'llar). Haqiqiy OpenBao bilan sinov: TestOpenBaoLive (VAULT_TEST_ADDR).
type fakeBao struct {
	mu         sync.Mutex
	keys       map[int]ed25519.PrivateKey
	created    map[int]time.Time
	hmacKey    []byte
	exportable bool
	logins     atomic.Int32
	token      string
	revokeOnce atomic.Bool
}

func newFakeBao(t *testing.T) (*fakeBao, *httptest.Server) {
	t.Helper()
	f := &fakeBao{keys: map[int]ed25519.PrivateKey{}, created: map[int]time.Time{}, hmacKey: []byte("hmac-key-32-bytes-long-for-tests")}
	f.rotate(time.Now().Add(-time.Hour))
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeBao) rotate(at time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	v := len(f.keys) + 1
	f.keys[v], f.created[v] = priv, at
}

func (f *fakeBao) serve(w http.ResponseWriter, r *http.Request) {
	write := func(code int, v any) {
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	if r.URL.Path == "/v1/auth/approle/login" {
		if in["role_id"] != "role" || in["secret_id"] != "secret" {
			write(400, map[string]any{"errors": []string{"invalid role or secret ID"}})
			return
		}
		n := f.logins.Add(1)
		f.mu.Lock()
		f.token = fmt.Sprintf("tok-%d", n)
		f.mu.Unlock()
		write(200, map[string]any{"auth": map[string]any{"client_token": f.token, "lease_duration": 3600}})
		return
	}
	f.mu.Lock()
	tok := f.token
	f.mu.Unlock()
	if r.Header.Get("X-Vault-Token") != tok || f.revokeOnce.CompareAndSwap(true, false) {
		write(403, map[string]any{"errors": []string{"permission denied"}})
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case r.URL.Path == "/v1/transit/keys/ondex-identity" && r.Method == http.MethodGet:
		keys := map[string]any{}
		for v, k := range f.keys {
			keys[fmt.Sprint(v)] = map[string]any{"public_key": base64.StdEncoding.EncodeToString(k.Public().(ed25519.PublicKey)),
				"creation_time": f.created[v].Format(time.RFC3339Nano), "name": "ed25519"}
		}
		write(200, map[string]any{"data": map[string]any{"type": "ed25519", "exportable": f.exportable, "keys": keys}})
	case r.URL.Path == "/v1/transit/sign/ondex-identity":
		msg, _ := base64.StdEncoding.DecodeString(in["input"].(string))
		v := int(in["key_version"].(float64))
		k, ok := f.keys[v]
		if !ok {
			write(400, map[string]any{"errors": []string{"invalid key version"}})
			return
		}
		write(200, map[string]any{"data": map[string]any{"signature": fmt.Sprintf("vault:v%d:%s", v,
			base64.StdEncoding.EncodeToString(ed25519.Sign(k, msg)))}})
	case r.URL.Path == "/v1/transit/hmac/ondex-derive/sha2-256":
		msg, _ := base64.StdEncoding.DecodeString(in["input"].(string))
		m := hmac.New(sha256.New, f.hmacKey)
		m.Write(msg)
		write(200, map[string]any{"data": map[string]any{"hmac": "vault:v1:" + base64.StdEncoding.EncodeToString(m.Sum(nil))}})
	case r.URL.Path == "/v1/transit/decrypt/ondex-secrets":
		ct := in["ciphertext"].(string)
		write(200, map[string]any{"data": map[string]any{"plaintext": strings.TrimPrefix(ct, "vault:v1:")}})
	default:
		write(404, map[string]any{"errors": []string{"no handler"}})
	}
}

func newTestClient(t *testing.T, srv *httptest.Server) *Client {
	t.Helper()
	c, err := New(Config{Addr: srv.URL, RoleID: "role", SecretID: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Asosiy oqim: KMS kaliti bilan token chiqariladi va JWKS orqali tekshiriladi;
// jarayonda maxfiy kalit YO'Q.
func TestRemoteKeySetSignsAndVerifies(t *testing.T) {
	_, srv := newFakeBao(t)
	c := newTestClient(t, srv)
	ctx := context.Background()
	root, err := c.HMAC(ctx, "ondex-derive", 1, []byte("ondex-identity-root-v1"))
	if err != nil {
		t.Fatal(err)
	}
	ks, err := identity.NewRemoteKeySet(ctx, c.SigningKeys("ondex-identity"), root, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if ks.Active().Private != nil {
		t.Fatal("XAVFSIZLIK: KMS rejimida maxfiy kalit jarayonda")
	}
	msg := []byte("header.payload")
	sig, err := ks.Active().Signer().Sign(rand.Reader, msg, nil)
	if err != nil {
		t.Fatal(err)
	}
	pub, ok := ks.PublicKey(ks.Active().KID)
	if !ok || !ed25519.Verify(pub, msg, sig) {
		t.Fatal("KMS imzosi JWKS kaliti bilan tasdiqlanmadi")
	}
	// HMAC deterministik (rotatsiyaga bog'liq emas).
	root2, _ := c.HMAC(ctx, "ondex-derive", 1, []byte("ondex-identity-root-v1"))
	if string(root) != string(root2) {
		t.Fatal("HMAC ildizi barqaror emas")
	}
}

// Rotatsiya: yangi versiya DARHOL JWKS'da, lekin imzolash uchun faqat
// publishDelay o'tgach (boshqa servislar keshida paydo bo'lishi uchun).
func TestRotationPublishesBeforeSigning(t *testing.T) {
	f, srv := newFakeBao(t)
	c := newTestClient(t, srv)
	ctx := context.Background()
	ks, err := identity.NewRemoteKeySet(ctx, c.SigningKeys("ondex-identity"), make([]byte, 32), 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	old := ks.Active().KID
	f.rotate(time.Now())
	if err := ks.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if ks.Active().KID != old {
		t.Fatal("yosh kalit darhol imzolashga o'tdi — boshqa servislar uni hali bilmaydi")
	}
	var j struct {
		Keys []struct{ Kid string } `json:"keys"`
	}
	raw, _ := ks.JWKS()
	_ = json.Unmarshal(raw, &j)
	if len(j.Keys) != 2 {
		t.Fatalf("yangi kalit JWKS'da e'lon qilinmadi: %d", len(j.Keys))
	}
	// Vaqt o'tdi — yangi versiya faol.
	f.mu.Lock()
	f.created[2] = time.Now().Add(-20 * time.Minute)
	f.mu.Unlock()
	if err := ks.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if ks.Active().KID == old {
		t.Fatal("publishDelay o'tgach yangi kalitga o'tilmadi")
	}
}

// Eksport qilinadigan Transit kaliti RAD etiladi.
func TestExportableKeyRejected(t *testing.T) {
	f, srv := newFakeBao(t)
	f.exportable = true
	c := newTestClient(t, srv)
	if _, err := c.SigningKeys("ondex-identity").Versions(context.Background()); err == nil {
		t.Fatal("XAVFSIZLIK: exportable=true kalit qabul qilindi")
	}
}

// Token bekor qilinsa (403) — AppRole bilan qayta kirib takrorlanadi.
func TestReloginOn403(t *testing.T) {
	f, srv := newFakeBao(t)
	c := newTestClient(t, srv)
	ctx := context.Background()
	if _, err := c.SigningKeys("ondex-identity").Versions(ctx); err != nil {
		t.Fatal(err)
	}
	f.revokeOnce.Store(true)
	if _, err := c.SigningKeys("ondex-identity").Versions(ctx); err != nil {
		t.Fatalf("403 dan keyin qayta kirilmadi: %v", err)
	}
	if n := f.logins.Load(); n != 2 {
		t.Fatalf("kirishlar soni %d, 2 kutilgan", n)
	}
}

// Redirect kuzatilmaydi — token boshqa hostga ketmaydi.
func TestNoRedirectFollow(t *testing.T) {
	var leaked atomic.Bool
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "" {
			leaked.Store(true)
		}
	}))
	defer evil.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	c, _ := New(Config{Addr: srv.URL, Token: "static"})
	_, _ = c.SigningKeys("ondex-identity").Versions(context.Background())
	if leaked.Load() {
		t.Fatal("XAVFSIZLIK: token redirect orqali boshqa hostga ketdi")
	}
}

func TestDecryptRequiresVaultPrefix(t *testing.T) {
	_, srv := newFakeBao(t)
	c := newTestClient(t, srv)
	if _, err := c.Decrypt(context.Background(), "ondex-secrets", "plain"); err == nil {
		t.Fatal("o'ralmagan qiymat qabul qilindi")
	}
	got, err := c.Decrypt(context.Background(), "ondex-secrets", "vault:v1:"+base64.StdEncoding.EncodeToString([]byte("dek")))
	if err != nil || string(got) != "dek" {
		t.Fatalf("decrypt: %q %v", got, err)
	}
}

// TestOpenBaoAppRoleLive — deploy/openbao/bootstrap.sh bilan sozlangan
// HAQIQIY OpenBao, API'ning AppRole huquqlari bilan: kerakli amallar
// ishlaydi, kalitni olib ketish/o'zgartirish yo'llari esa 403.
//
//	VAULT_TEST_ADDR, VAULT_TEST_ROLE_ID, VAULT_TEST_SECRET_ID,
//	VAULT_TEST_WRAPPED (secrets_wrapped), VAULT_TEST_WRAPPED_PLAIN (asl kalit)
func TestOpenBaoAppRoleLive(t *testing.T) {
	addr, role, secret := os.Getenv("VAULT_TEST_ADDR"), os.Getenv("VAULT_TEST_ROLE_ID"), os.Getenv("VAULT_TEST_SECRET_ID")
	if addr == "" || role == "" {
		t.Skip("VAULT_TEST_ROLE_ID berilmagan")
	}
	c, err := New(Config{Addr: addr, RoleID: role, SecretID: secret})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, name := range []string{"ondex-identity", "ondex-platform"} {
		root, err := c.HMAC(ctx, "ondex-derive", 1, []byte("ondex-identity-root-v1:"+name))
		if err != nil {
			t.Fatalf("hmac: %v", err)
		}
		ks, err := identity.NewRemoteKeySet(ctx, c.SigningKeys(name), root, identity.DefaultPublishDelay)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		msg := []byte("live-" + name)
		sig, err := ks.Active().Signer().Sign(rand.Reader, msg, nil)
		if err != nil {
			t.Fatalf("%s imzo: %v", name, err)
		}
		pub, _ := ks.PublicKey(ks.Active().KID)
		if !ed25519.Verify(pub, msg, sig) {
			t.Fatalf("%s imzosi tasdiqlanmadi", name)
		}
	}
	if w := os.Getenv("VAULT_TEST_WRAPPED"); w != "" {
		plain, err := c.Decrypt(ctx, "ondex-secrets", w)
		if err != nil || strings.TrimSpace(string(plain)) != os.Getenv("VAULT_TEST_WRAPPED_PLAIN") {
			t.Fatalf("o'ralgan kalit ochilmadi: %v", err)
		}
	}
	denied := []struct{ method, path string }{
		{http.MethodGet, "transit/export/signing-key/ondex-identity"},
		{http.MethodGet, "transit/backup/ondex-identity"},
		{http.MethodPost, "transit/keys/ondex-identity/rotate"},
		{http.MethodPost, "transit/keys/ondex-identity/config"},
		{http.MethodDelete, "transit/keys/ondex-identity"},
		{http.MethodPost, "transit/encrypt/ondex-secrets"},
		{http.MethodPost, "transit/keys/yangi"},
		{http.MethodGet, "sys/policies/acl/ondex-api"},
		{http.MethodPost, "auth/approle/role/ondex-api/secret-id"},
	}
	for _, d := range denied {
		err := c.do(ctx, d.method, d.path, map[string]any{"exportable": true, "plaintext": "eA=="}, nil, true)
		var ae *apiError
		if !errors.As(err, &ae) || ae.Status != http.StatusForbidden {
			t.Errorf("XAVFSIZLIK: %s %s rad etilmadi: %v", d.method, d.path, err)
		}
	}
}

// TestOpenBaoLive — HAQIQIY OpenBao (dev rejim) bilan:
//
//	docker run --rm -d -p 8299:8200 -e BAO_DEV_ROOT_TOKEN_ID=root openbao/openbao
//	VAULT_TEST_ADDR=http://127.0.0.1:8299 VAULT_TEST_TOKEN=root go test ./internal/kms -run Live
func TestOpenBaoLive(t *testing.T) {
	addr, tok := os.Getenv("VAULT_TEST_ADDR"), os.Getenv("VAULT_TEST_TOKEN")
	if addr == "" {
		t.Skip("VAULT_TEST_ADDR berilmagan")
	}
	c, err := New(Config{Addr: addr, Token: tok})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_ = c.call(ctx, http.MethodPost, "sys/mounts/transit", map[string]any{"type": "transit"}, nil)
	if err := c.call(ctx, http.MethodPost, "transit/keys/live-sign", map[string]any{"type": "ed25519"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := c.call(ctx, http.MethodPost, "transit/keys/live-derive", map[string]any{"type": "hmac", "key_size": 32}, nil); err != nil {
		t.Fatal(err)
	}
	root, err := c.HMAC(ctx, "live-derive", 1, []byte("ondex-identity-root-v1"))
	if err != nil {
		t.Fatal(err)
	}
	ks, err := identity.NewRemoteKeySet(ctx, c.SigningKeys("live-sign"), root, 0)
	if err != nil {
		t.Fatal(err)
	}
	msg := []byte("live")
	sig, err := ks.Active().Signer().Sign(rand.Reader, msg, nil)
	if err != nil {
		t.Fatal(err)
	}
	pub, _ := ks.PublicKey(ks.Active().KID)
	if !ed25519.Verify(pub, msg, sig) {
		t.Fatal("haqiqiy OpenBao imzosi tasdiqlanmadi")
	}
}

// Fayldan KMS'ga o'tish: eski fayl kaliti faqat TEKSHIRISH uchun qoladi va
// rotatsiya yangilanishida yo'qolmaydi; imzolashda ishlatilmaydi.
func TestVerifyOnlyKeySurvivesRefresh(t *testing.T) {
	f, srv := newFakeBao(t)
	c := newTestClient(t, srv)
	ctx := context.Background()
	ks, err := identity.NewRemoteKeySet(ctx, c.SigningKeys("ondex-identity"), make([]byte, 32), 0)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := identity.GenerateKey()
	ks.AddVerifyOnly(&identity.SigningKey{KID: old.KID, Public: old.Public})
	f.rotate(time.Now().Add(-time.Hour))
	if err := ks.Refresh(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok := ks.PublicKey(old.KID); !ok {
		t.Fatal("eski fayl kaliti refresh'da yo'qoldi — o'tish paytidagi tokenlar rad etiladi")
	}
	if ks.Active().KID == old.KID {
		t.Fatal("XAVFSIZLIK: faqat-tekshirish kaliti imzolashga tanlandi")
	}
}
