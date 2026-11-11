// Package kms — maxfiy kalitlar jarayon xotirasiga KIRMASIN: imzo, HMAC va
// shifr ochish KMS ichida bajariladi.
//
// Hozirgi backend — OpenBao / HashiCorp Vault **Transit** (bir xil API).
// Kalit Transit ichida yaratiladi va eksport qilinmaydi (`exportable=false`);
// API faqat "shu xabarni imzola" deb so'raydi. API konteyneri buzilsa ham
// hujumchi kalitni olib keta olmaydi — faqat API ishlab turgan paytda va
// Transit audit jurnalida iz qoldirib imzo so'ray oladi.
//
// ┌─ AUTENTIFIKATSIYA ─────────────────────────────────────────────────────┐
// AppRole (tavsiya): `role_id` (sozlama) + `secret_id` (fayl, 600). Token
// muddati tugashidan oldin qayta kiriladi; 403 da bir marta qayta kirib
// takrorlanadi. Statik token (`VAULT_TOKEN_FILE`) faqat dev/sinov uchun.
// └────────────────────────────────────────────────────────────────────────┘
package kms

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"chustapp/internal/identity"
)

// Config — OpenBao/Vault ulanishi.
type Config struct {
	Addr       string // https://openbao:8200
	Mount      string // Transit mount, standart "transit"
	Token      string // statik token (dev)
	RoleID     string // AppRole
	SecretID   string // AppRole secret_id (fayldan o'qiladi)
	CACertFile string // o'z CA (ixtiyoriy)
	Timeout    time.Duration
	HTTPClient *http.Client // testlar uchun
}

// Client — Transit mijozi. Parallel ishlatish xavfsiz.
type Client struct {
	cfg  Config
	base *url.URL
	hc   *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time // nol — muddatsiz (statik token)
}

// ErrNotConfigured — manzil yoki autentifikatsiya berilmagan.
var ErrNotConfigured = errors.New("kms: OpenBao manzili yoki kirish ma'lumoti berilmagan")

// FromEnv — muhit o'zgaruvchilaridan:
//
//	VAULT_ADDR             — https://openbao:8200 (majburiy)
//	VAULT_TRANSIT_MOUNT    — standart "transit"
//	VAULT_ROLE_ID          — AppRole role_id
//	VAULT_SECRET_ID_FILE   — AppRole secret_id fayli (600)
//	VAULT_TOKEN_FILE       — statik token fayli (faqat dev)
//	VAULT_CACERT           — o'z CA sertifikati (ixtiyoriy)
func FromEnv() (Config, error) {
	cfg := Config{
		Addr:       strings.TrimRight(strings.TrimSpace(os.Getenv("VAULT_ADDR")), "/"),
		Mount:      strings.Trim(strings.TrimSpace(os.Getenv("VAULT_TRANSIT_MOUNT")), "/"),
		RoleID:     strings.TrimSpace(os.Getenv("VAULT_ROLE_ID")),
		CACertFile: strings.TrimSpace(os.Getenv("VAULT_CACERT")),
	}
	read := func(env string) (string, error) {
		p := strings.TrimSpace(os.Getenv(env))
		if p == "" {
			return "", nil
		}
		b, err := os.ReadFile(p) //nolint:gosec // yo'l faqat operator sozlamasidan
		if err != nil {
			return "", fmt.Errorf("kms: %s o'qilmadi: %w", env, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	var err error
	if cfg.SecretID, err = read("VAULT_SECRET_ID_FILE"); err != nil {
		return cfg, err
	}
	if cfg.Token, err = read("VAULT_TOKEN_FILE"); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// New — mijoz yaratadi (tarmoqqa murojaat qilmaydi).
func New(cfg Config) (*Client, error) {
	if cfg.Addr == "" || (cfg.Token == "" && (cfg.RoleID == "" || cfg.SecretID == "")) {
		return nil, ErrNotConfigured
	}
	u, err := url.Parse(cfg.Addr)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("kms: VAULT_ADDR noto'g'ri: %q", cfg.Addr)
	}
	if cfg.Mount == "" {
		cfg.Mount = "transit"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	hc := cfg.HTTPClient
	if hc == nil {
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		if cfg.CACertFile != "" {
			pem, err := os.ReadFile(cfg.CACertFile)
			if err != nil {
				return nil, fmt.Errorf("kms: VAULT_CACERT o'qilmadi: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return nil, errors.New("kms: VAULT_CACERT'da sertifikat yo'q")
			}
			tr.TLSClientConfig.RootCAs = pool
		}
		hc = &http.Client{
			Transport: tr,
			Timeout:   cfg.Timeout,
			// Token sarlavhasi boshqa hostga ketib qolmasin.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	c := &Client{cfg: cfg, base: u, hc: hc}
	if cfg.Token != "" {
		c.token = cfg.Token
	}
	return c, nil
}

// apiError — OpenBao xatosi (matnida sir yo'q: faqat "errors" maydoni).
type apiError struct {
	Status int
	Msg    string
}

func (e *apiError) Error() string { return fmt.Sprintf("kms: OpenBao %d: %s", e.Status, e.Msg) }

const maxBody = 1 << 20

// do — so'rov; `auth=false` faqat AppRole kirishi uchun.
func (c *Client) do(ctx context.Context, method, path string, in, out any, auth bool) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base.String()+"/v1/"+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth {
		tok, err := c.currentToken(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("X-Vault-Token", tok)
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("kms: OpenBao'ga ulanib bo'lmadi: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		var e struct {
			Errors []string `json:"errors"`
		}
		_ = json.Unmarshal(raw, &e)
		return &apiError{Status: resp.StatusCode, Msg: strings.Join(e.Errors, "; ")}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// call — avtorizatsiyali so'rov; 403 da (token bekor qilingan/eskirgan)
// AppRole bilan bir marta qayta kirib takrorlaydi.
func (c *Client) call(ctx context.Context, method, path string, in, out any) error {
	err := c.do(ctx, method, path, in, out, true)
	var ae *apiError
	if errors.As(err, &ae) && ae.Status == http.StatusForbidden && c.cfg.RoleID != "" {
		c.mu.Lock()
		c.token, c.tokenExp = "", time.Time{}
		c.mu.Unlock()
		return c.do(ctx, method, path, in, out, true)
	}
	return err
}

// currentToken — amaldagi token; muddatining 2/3 qismi o'tgan bo'lsa
// AppRole bilan yangisi olinadi.
func (c *Client) currentToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && (c.tokenExp.IsZero() || time.Now().Before(c.tokenExp)) {
		return c.token, nil
	}
	if c.cfg.RoleID == "" {
		if c.token != "" {
			return c.token, nil
		}
		return "", ErrNotConfigured
	}
	var out struct {
		Auth struct {
			ClientToken   string `json:"client_token"`
			LeaseDuration int    `json:"lease_duration"`
		} `json:"auth"`
	}
	if err := c.do(ctx, http.MethodPost, "auth/approle/login",
		map[string]string{"role_id": c.cfg.RoleID, "secret_id": c.cfg.SecretID}, &out, false); err != nil {
		return "", fmt.Errorf("kms: AppRole kirishi: %w", err)
	}
	if out.Auth.ClientToken == "" {
		return "", errors.New("kms: AppRole token qaytarmadi")
	}
	c.token = out.Auth.ClientToken
	c.tokenExp = time.Time{}
	if d := out.Auth.LeaseDuration; d > 0 {
		c.tokenExp = time.Now().Add(time.Duration(d) * time.Second * 2 / 3)
	}
	return c.token, nil
}

func (c *Client) keyPath(op, key string) string {
	return c.cfg.Mount + "/" + op + "/" + url.PathEscape(key)
}

// parseVaultValue — "vault:v3:BASE64" -> (3, bayt).
func parseVaultValue(s string) (int, []byte, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 || parts[0] != "vault" || !strings.HasPrefix(parts[1], "v") {
		return 0, nil, errors.New("kms: OpenBao javobi formati noto'g'ri")
	}
	v, err := strconv.Atoi(parts[1][1:])
	if err != nil || v < 1 {
		return 0, nil, errors.New("kms: OpenBao javobida versiya noto'g'ri")
	}
	b, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return 0, nil, errors.New("kms: OpenBao javobi base64 emas")
	}
	return v, b, nil
}

// SigningKeys — Transit'dagi Ed25519 kalit (identity.KeySource).
type SigningKeys struct {
	c   *Client
	key string
}

// SigningKeys — `name` nomli Transit kaliti uchun manba.
func (c *Client) SigningKeys(name string) *SigningKeys { return &SigningKeys{c: c, key: name} }

// Versions — kalitning barcha versiyalari (min_decryption_version'dan
// eskilari OpenBao tomonidan qaytarilmaydi — "trim").
func (s *SigningKeys) Versions(ctx context.Context) ([]identity.RemoteKey, error) {
	var out struct {
		Data struct {
			Type       string `json:"type"`
			Exportable bool   `json:"exportable"`
			Keys       map[string]struct {
				PublicKey    string    `json:"public_key"`
				CreationTime time.Time `json:"creation_time"`
			} `json:"keys"`
		} `json:"data"`
	}
	if err := s.c.call(ctx, http.MethodGet, s.c.keyPath("keys", s.key), nil, &out); err != nil {
		return nil, err
	}
	if out.Data.Type != "ed25519" {
		return nil, fmt.Errorf("kms: %q kaliti ed25519 emas (%s)", s.key, out.Data.Type)
	}
	if out.Data.Exportable {
		// Eksport qilinadigan kalit — KMS'ning butun ma'nosini yo'qqa chiqaradi.
		return nil, fmt.Errorf("kms: %q kaliti exportable=true — xavfsiz emas, exportable=false bilan qayta yarating", s.key)
	}
	res := make([]identity.RemoteKey, 0, len(out.Data.Keys))
	for ver, k := range out.Data.Keys {
		v, err := strconv.Atoi(ver)
		if err != nil || v < 1 {
			continue
		}
		pub, err := base64.StdEncoding.DecodeString(k.PublicKey)
		if err != nil || len(pub) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("kms: %q v%d ochiq kaliti noto'g'ri", s.key, v)
		}
		res = append(res, identity.RemoteKey{Version: v, Public: ed25519.PublicKey(pub), Created: k.CreationTime,
			Signer: &remoteSigner{c: s.c, key: s.key, version: v, pub: ed25519.PublicKey(pub)}})
	}
	return res, nil
}

// remoteSigner — crypto.Signer: Ed25519 imzo Transit'da.
type remoteSigner struct {
	c       *Client
	key     string
	version int
	pub     ed25519.PublicKey
}

func (r *remoteSigner) Public() crypto.PublicKey { return r.pub }

// Sign — `crypto.Signer` kontekst bermaydi: muddat mijoz sozlamasidan.
func (r *remoteSigner) Sign(_ io.Reader, msg []byte, opts crypto.SignerOpts) ([]byte, error) {
	if opts != nil && opts.HashFunc() != crypto.Hash(0) {
		return nil, errors.New("kms: Ed25519 oldindan xeshlangan xabarni imzolamaydi")
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.c.cfg.Timeout)
	defer cancel()
	var out struct {
		Data struct {
			Signature string `json:"signature"`
		} `json:"data"`
	}
	if err := r.c.call(ctx, http.MethodPost, r.c.keyPath("sign", r.key),
		map[string]any{"input": base64.StdEncoding.EncodeToString(msg), "key_version": r.version}, &out); err != nil {
		return nil, err
	}
	v, sig, err := parseVaultValue(out.Data.Signature)
	if err != nil {
		return nil, err
	}
	if v != r.version || len(sig) != ed25519.SignatureSize {
		return nil, errors.New("kms: imzo kutilgan versiyada emas")
	}
	// Himoya: KMS boshqa kalit bilan imzolab qo'ysa (noto'g'ri sozlama) —
	// yaroqsiz token chiqarishdan ko'ra xato yaxshi.
	if !ed25519.Verify(r.pub, msg, sig) {
		return nil, errors.New("kms: KMS imzosi ochiq kalit bilan tasdiqlanmadi")
	}
	return sig, nil
}

// HMAC — Transit HMAC (sha2-256), ANIQ `version` bilan: kalit rotatsiya
// qilinsa ham natija o'zgarmaydi (sessiyalar uzilmaydi).
func (c *Client) HMAC(ctx context.Context, key string, version int, input []byte) ([]byte, error) {
	var out struct {
		Data struct {
			HMAC string `json:"hmac"`
		} `json:"data"`
	}
	if err := c.call(ctx, http.MethodPost, c.keyPath("hmac", key)+"/sha2-256",
		map[string]any{"input": base64.StdEncoding.EncodeToString(input), "key_version": version}, &out); err != nil {
		return nil, err
	}
	_, mac, err := parseVaultValue(out.Data.HMAC)
	if err != nil {
		return nil, err
	}
	if len(mac) != 32 {
		return nil, errors.New("kms: HMAC uzunligi noto'g'ri")
	}
	return mac, nil
}

// Decrypt — Transit bilan o'ralgan ma'lumot kalitini (DEK) ochadi
// (envelope shifrlash: `SECRETS_ENC_KEY_WRAPPED`).
func (c *Client) Decrypt(ctx context.Context, key, ciphertext string) ([]byte, error) {
	if !strings.HasPrefix(ciphertext, "vault:v") {
		return nil, errors.New("kms: o'ralgan qiymat 'vault:vN:' bilan boshlanishi kerak")
	}
	var out struct {
		Data struct {
			Plaintext string `json:"plaintext"`
		} `json:"data"`
	}
	if err := c.call(ctx, http.MethodPost, c.keyPath("decrypt", key),
		map[string]any{"ciphertext": ciphertext}, &out); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(out.Data.Plaintext)
}
