package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// OnDex OIDC provayderi (A4c, Plane ONDEXECOSY-199).
//
// ┌─ NEGA O'ZIMIZNIKI, HYDRA EMAS (egasi qarori, 2026-11-08) ─────────────┐
// Sessiya markazi BITTA bo'lishi shart (`auth_sessions`): markazda yopilgan
// sessiya hamma joyda tugaydi. Hydra o'z tokenlari va sessiya bazasini
// yuritardi — ikki markaz. Shuning uchun OIDC qatlami mavjud `Service`
// ustiga quriladi: kod almashilganda ODDIY markaziy sessiya ochiladi
// (`Service.Create`), access token — o'sha EdDSA token.
// └───────────────────────────────────────────────────────────────────────┘
//
// Oqim (Authorization Code + PKCE S256, RFC 6749 / 7636 / 9207):
//
//	klient -> GET /v2/oauth/authorize?...            (ParseAuthorize)
//	   SSO cookie bor -> kod -> redirect_uri?code&state&iss
//	   yo'q           -> OnDex kirish sahifasi (SignRequest) -> kirish
//	                      -> SSO sessiyasi (`ondex-sso`) -> kod
//	klient -> POST /v2/oauth/token (code + code_verifier + klient siri)
//	   -> markaziy sessiya (`parent_id` = SSO sessiyasi) + id_token
//
// Qat'iy qoidalar (o'zgartirmang):
//   - redirect_uri ro'yxatdagisi bilan BAYTMA-BAYT teng (prefiks yo'q);
//   - PKCE MAJBURIY va faqat S256 (`plain` rad etiladi);
//   - nonce MAJBURIY (id_token'ni qayta o'ynatish yo'q);
//   - kod 60 s, bir marta; qayta kelsa u bilan ochilgan sessiya YOPILADI;
//   - klient siri faqat sha256 xeshi bilan, doimiy vaqtda solishtiriladi;
//   - redirect_uri noto'g'ri bo'lsa xato o'sha manzilga QAYTARILMAYDI
//     (ochiq yo'naltirgich bo'lmasin).

// SSOClientID — OnDex kirish sahifasining o'z sessiyasi (brauzer cookie).
const SSOClientID = "ondex-sso"

// CodeTTL — Authorization Code umri.
const CodeTTL = 60 * time.Second

// requestTTL — kirish sahifasida o'tkaziladigan vaqt (imzolangan so'rov umri).
const requestTTL = 15 * time.Minute

// IDTokenTTL — id_token umri (faqat kirish paytida tekshiriladi).
const IDTokenTTL = 10 * time.Minute

// OIDCClient — ro'yxatdan o'tgan ilova (relying party).
type OIDCClient struct {
	ID   string
	Name string
	// SecretHash — sha256(client_secret). Bo'sh — public klient (faqat PKCE).
	SecretHash             []byte
	RedirectURIs           []string
	PostLogoutRedirectURIs []string
	// BackchannelLogoutURI — sessiya markazda yopilganda imzolangan
	// logout_token shu manzilga POST qilinadi (A4c.3).
	BackchannelLogoutURI string
}

// HashClientSecret — konfiguratsiyadagi sirdan xesh.
func HashClientSecret(secret string) []byte {
	s := sha256.Sum256([]byte(secret))
	return s[:]
}

// AuthCode — `oauth_codes` qatori.
type AuthCode struct {
	Hash            []byte
	ClientID        string
	RedirectURI     string
	Challenge       string
	Nonce           string
	Scope           string
	UserID          string
	ParentSessionID string
	OrgID           string
	AMR             []string
	AuthTime        time.Time
	CreatedAt       time.Time
	ExpiresAt       time.Time
	IssuedSessionID string
}

// CodeResult — `CodeStore.Consume` natijasi.
type CodeResult int

const (
	CodeOK      CodeResult = iota
	CodeInvalid            // yo'q yoki muddati o'tgan
	CodeReused             // allaqachon ishlatilgan — IssuedSessionID yopilishi kerak
)

// CodeStore — kodlar ombori. `Consume` ATOMIK bo'lishi SHART: bitta kod
// parallel ikki so'rovda ikki marta almashtirilmaydi.
type CodeStore interface {
	Put(ctx context.Context, c *AuthCode) error
	Consume(ctx context.Context, hash []byte, now time.Time) (*AuthCode, CodeResult, error)
	SetIssued(ctx context.Context, hash []byte, sessionID string) error
}

// OAuthError — RFC 6749 5.2 / 4.1.2.1 xatosi. `Description` foydalanuvchiga
// ko'rinadi — ichki tafsilot yozilmaydi.
type OAuthError struct {
	Code        string
	Description string
	Status      int
}

func (e *OAuthError) Error() string { return e.Code + ": " + e.Description }

func oauthErr(code, desc string, status int) *OAuthError {
	return &OAuthError{Code: code, Description: desc, Status: status}
}

var (
	errInvalidGrant  = oauthErr("invalid_grant", "kod yaroqsiz, ishlatilgan yoki muddati o'tgan", 400)
	errInvalidClient = oauthErr("invalid_client", "klient tekshiruvdan o'tmadi", 401)
)

// AuthorizeRequest — tekshirilgan /authorize so'rovi.
type AuthorizeRequest struct {
	ClientID      string `json:"c"`
	RedirectURI   string `json:"r"`
	State         string `json:"s,omitempty"`
	Nonce         string `json:"n"`
	Scope         string `json:"sc"`
	CodeChallenge string `json:"cc"`
	Prompt        string `json:"p,omitempty"`
	MaxAge        int    `json:"m"` // -1 = berilmagan
	Exp           int64  `json:"e,omitempty"`
}

// Provider — OIDC qatlami.
type Provider struct {
	svc     *Service
	codes   CodeStore
	clients map[string]OIDCClient
	cookieK []byte
	reqK    []byte
	post    backchannelPoster
}

var (
	reChallenge = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	reVerifier  = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
	reToken     = regexp.MustCompile(`^[\x21-\x7e]{1,512}$`) // state/nonce: bosma ASCII
)

// NewProvider — klientlar `Service` siyosatida (realm/muddat) ham e'lon
// qilingan bo'lishi SHART.
func NewProvider(svc *Service, codes CodeStore, clients []OIDCClient) (*Provider, error) {
	if svc == nil || codes == nil {
		return nil, errors.New("oidc: service va codes majburiy")
	}
	if _, ok := svc.clients[SSOClientID]; !ok {
		return nil, errors.New("oidc: " + SSOClientID + " sessiya siyosati e'lon qilinmagan")
	}
	p := &Provider{svc: svc, codes: codes, clients: map[string]OIDCClient{},
		cookieK: svc.issuer.keys.derivedSecret("sso-cookie"), reqK: svc.issuer.keys.derivedSecret("oidc-request"),
		post: postBackchannel}
	for _, c := range clients {
		if c.ID == SSOClientID {
			return nil, errors.New("oidc: " + SSOClientID + " OIDC klienti bo'la olmaydi")
		}
		if _, ok := svc.clients[c.ID]; !ok {
			return nil, errors.New("oidc: klient sessiya siyosatisiz: " + c.ID)
		}
		if len(c.RedirectURIs) == 0 {
			return nil, errors.New("oidc: redirect_uri yo'q: " + c.ID)
		}
		for _, u := range append(append([]string{}, c.RedirectURIs...), c.PostLogoutRedirectURIs...) {
			if err := validateClientURI(u); err != nil {
				return nil, errors.New("oidc: " + c.ID + ": " + err.Error())
			}
		}
		if c.BackchannelLogoutURI != "" {
			if err := validateClientURI(c.BackchannelLogoutURI); err != nil {
				return nil, errors.New("oidc: " + c.ID + " backchannel: " + err.Error())
			}
		}
		if c.SecretHash != nil && len(c.SecretHash) != sha256.Size {
			return nil, errors.New("oidc: sir xeshi noto'g'ri: " + c.ID)
		}
		p.clients[c.ID] = c
	}
	svc.OnRevoke(p.onRevoke)
	return p, nil
}

// validateClientURI — https (yoki dev uchun http://localhost / 127.0.0.1),
// fragmentsiz, mutlaq.
func validateClientURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || u.Fragment != "" || u.User != nil {
		return errors.New("noto'g'ri URI: " + raw)
	}
	host := u.Hostname()
	if u.Scheme == "https" || (u.Scheme == "http" && (host == "localhost" || host == "127.0.0.1")) {
		return nil
	}
	return errors.New("https bo'lishi kerak: " + raw)
}

// Client — klient ma'lumoti (kirish sahifasida nomini ko'rsatish uchun).
func (p *Provider) Client(id string) (OIDCClient, bool) {
	c, ok := p.clients[id]
	return c, ok
}

// ParseAuthorize — /authorize parametrlari. Ikkinchi qiymat `false` bo'lsa
// xato redirect_uri'ga QAYTARILMAYDI (klient/redirect noto'g'ri — sahifada
// ko'rsatiladi), aks holda klientga `error` parametri bilan qaytariladi.
func (p *Provider) ParseAuthorize(q url.Values) (*AuthorizeRequest, bool, error) {
	single := func(k string) (string, bool) {
		v := q[k]
		if len(v) > 1 {
			return "", false // RFC 6749 3.1: takrorlangan parametr — xato
		}
		if len(v) == 0 {
			return "", true
		}
		return v[0], true
	}
	cid, ok1 := single("client_id")
	ruri, ok2 := single("redirect_uri")
	if !ok1 || !ok2 {
		return nil, false, oauthErr("invalid_request", "parametr takrorlangan", 400)
	}
	c, ok := p.clients[cid]
	if !ok {
		return nil, false, oauthErr("invalid_client", "noma'lum ilova", 400)
	}
	if !slices.Contains(c.RedirectURIs, ruri) {
		return nil, false, oauthErr("invalid_request", "qaytish manzili ro'yxatda yo'q", 400)
	}
	req := &AuthorizeRequest{ClientID: cid, RedirectURI: ruri, MaxAge: -1}
	vals := map[string]string{}
	for _, k := range []string{"response_type", "scope", "state", "nonce", "code_challenge", "code_challenge_method", "prompt", "max_age"} {
		v, ok := single(k)
		if !ok {
			return req, true, oauthErr("invalid_request", "parametr takrorlangan: "+k, 400)
		}
		vals[k] = v
	}
	if s := vals["state"]; s != "" && !reToken.MatchString(s) {
		return req, false, oauthErr("invalid_request", "state noto'g'ri", 400)
	}
	req.State = vals["state"]
	if vals["response_type"] != "code" {
		return req, true, oauthErr("unsupported_response_type", "faqat response_type=code", 400)
	}
	scopes := strings.Fields(vals["scope"])
	if !slices.Contains(scopes, "openid") || len(vals["scope"]) > 256 {
		return req, true, oauthErr("invalid_scope", "scope'da openid bo'lishi shart", 400)
	}
	req.Scope = strings.Join(scopes, " ")
	if vals["code_challenge_method"] != "S256" || !reChallenge.MatchString(vals["code_challenge"]) {
		return req, true, oauthErr("invalid_request", "PKCE majburiy (code_challenge_method=S256)", 400)
	}
	req.CodeChallenge = vals["code_challenge"]
	if n := vals["nonce"]; n == "" || len(n) > 256 || !reToken.MatchString(n) {
		return req, true, oauthErr("invalid_request", "nonce majburiy", 400)
	}
	req.Nonce = vals["nonce"]
	switch vals["prompt"] {
	case "", "login", "none":
		req.Prompt = vals["prompt"]
	default:
		return req, true, oauthErr("invalid_request", "prompt qiymati qo'llab-quvvatlanmaydi", 400)
	}
	if m := vals["max_age"]; m != "" {
		n, err := strconv.Atoi(m)
		if err != nil || n < 0 || n > 365*24*3600 {
			return req, true, oauthErr("invalid_request", "max_age noto'g'ri", 400)
		}
		req.MaxAge = n
	}
	return req, true, nil
}

// ErrorRedirect — klientga xato bilan qaytish manzili (RFC 6749 4.1.2.1).
func (p *Provider) ErrorRedirect(req *AuthorizeRequest, e *OAuthError) string {
	q := url.Values{"error": {e.Code}, "error_description": {e.Description}, "iss": {p.svc.issuer.issuer}}
	if req.State != "" {
		q.Set("state", req.State)
	}
	return appendQuery(req.RedirectURI, q)
}

func appendQuery(base string, q url.Values) string {
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + q.Encode()
}

func (p *Provider) mac(key []byte, payload string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// SignRequest — kirish sahifasiga beriladigan imzolangan so'rov (bazasiz).
func (p *Provider) SignRequest(req *AuthorizeRequest) (string, error) {
	r := *req
	r.Exp = p.svc.now().Add(requestTTL).Unix()
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(b)
	return payload + "." + p.mac(p.reqK, payload), nil
}

// OpenRequest — imzo va muddatni tekshiradi, klient/redirect hali ro'yxatda.
func (p *Provider) OpenRequest(raw string) (*AuthorizeRequest, error) {
	bad := oauthErr("invalid_request", "kirish so'rovi yaroqsiz yoki muddati o'tgan — ilovadan qayta boshlang", 400)
	payload, sig, ok := strings.Cut(raw, ".")
	if !ok || len(raw) > 4096 || !hmac.Equal([]byte(sig), []byte(p.mac(p.reqK, payload))) {
		return nil, bad
	}
	b, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, bad
	}
	var r AuthorizeRequest
	if json.Unmarshal(b, &r) != nil || p.svc.now().Unix() > r.Exp {
		return nil, bad
	}
	c, ok := p.clients[r.ClientID]
	if !ok || !slices.Contains(c.RedirectURIs, r.RedirectURI) {
		return nil, bad
	}
	return &r, nil
}

// SSOCookie — SSO sessiyasi uchun cookie qiymati (sid + HMAC). Sessiyaning
// o'zi markazda: cookie faqat ko'rsatkich, bekor qilish markazda.
func (p *Provider) SSOCookie(sessionID string) string {
	return sessionID + "." + p.mac(p.cookieK, sessionID)
}

// SSOSession — cookie'dan faol SSO sessiyasi (yoki ErrNotFound).
func (p *Provider) SSOSession(ctx context.Context, cookie string) (*Session, error) {
	sid, sig, ok := strings.Cut(cookie, ".")
	if !ok || sid == "" || len(cookie) > 256 || !hmac.Equal([]byte(sig), []byte(p.mac(p.cookieK, sid))) {
		return nil, ErrNotFound
	}
	sess, err := p.svc.store.Get(ctx, sid)
	if err != nil {
		return nil, ErrNotFound
	}
	if sess.ClientID != SSOClientID || !sess.Active(p.svc.now()) {
		return nil, ErrNotFound
	}
	u, err := p.svc.users.GetByID(ctx, sess.UserID)
	if err != nil || u.IsDeleted() {
		return nil, ErrNotFound
	}
	return sess, nil
}

// NeedsLogin — SSO sessiyasi bu so'rov uchun yetarlimi (prompt/max_age).
func (p *Provider) NeedsLogin(req *AuthorizeRequest, sso *Session) bool {
	if sso == nil || req.Prompt == "login" {
		return true
	}
	return req.MaxAge >= 0 && p.svc.now().Sub(sso.CreatedAt) > time.Duration(req.MaxAge)*time.Second
}

// SSOSessionFromAccess — kirish sahifasi `ondex-sso` sessiyasini ochgach
// uning access tokenini yuboradi; shu sessiya cookie'ga aylanadi.
func (p *Provider) SSOSessionFromAccess(ctx context.Context, access string) (*Session, error) {
	c, err := p.svc.Authenticate(ctx, access)
	if err != nil || c.ClientID != SSOClientID {
		return nil, ErrTokenInvalid
	}
	sess, err := p.svc.store.Get(ctx, c.SessionID)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	return sess, nil
}

func newCodeValue() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	code := "oac_" + base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(code))
	return code, h[:], nil
}

// IssueCode — SSO sessiyasi asosida kod va klientga qaytish manzili.
func (p *Provider) IssueCode(ctx context.Context, req *AuthorizeRequest, sso *Session) (string, error) {
	if sso == nil || sso.ClientID != SSOClientID || !sso.Active(p.svc.now()) {
		return "", ErrNotFound
	}
	code, hash, err := newCodeValue()
	if err != nil {
		return "", err
	}
	now := p.svc.now()
	ac := &AuthCode{Hash: hash, ClientID: req.ClientID, RedirectURI: req.RedirectURI, Challenge: req.CodeChallenge,
		Nonce: req.Nonce, Scope: req.Scope, UserID: sso.UserID, ParentSessionID: sso.ID, OrgID: sso.OrgID,
		AMR: sso.AMR, AuthTime: sso.CreatedAt, CreatedAt: now, ExpiresAt: now.Add(CodeTTL)}
	if err := p.codes.Put(ctx, ac); err != nil {
		return "", err
	}
	p.svc.audit.Record(ctx, Event{Action: "oidc.code", Result: ResultOK, ActorUserID: sso.UserID,
		Realm: string(sso.Realm), Target: req.ClientID, Details: map[string]any{"sso_session": sso.ID}})
	q := url.Values{"code": {code}, "iss": {p.svc.issuer.issuer}}
	if req.State != "" {
		q.Set("state", req.State)
	}
	return appendQuery(req.RedirectURI, q), nil
}

// TokenResponse — /token javobi (RFC 6749 5.1 + OIDC Core 3.1.3.3).
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	SessionID    string `json:"session_id"`
}

// authenticateClient — konfidensial klient sirini doimiy vaqtda tekshiradi.
func (p *Provider) authenticateClient(clientID, secret string) (OIDCClient, error) {
	c, ok := p.clients[clientID]
	if !ok {
		// Mavjud bo'lmagan klient uchun ham xesh hisoblanadi — vaqt farqi bilinmasin.
		_ = HashClientSecret(secret)
		return OIDCClient{}, errInvalidClient
	}
	if c.SecretHash == nil {
		if secret != "" {
			return OIDCClient{}, errInvalidClient
		}
		return c, nil
	}
	if subtle.ConstantTimeCompare(HashClientSecret(secret), c.SecretHash) != 1 {
		return OIDCClient{}, errInvalidClient
	}
	return c, nil
}

func pkceS256(verifier string) string {
	s := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(s[:])
}

// ExchangeCode — grant_type=authorization_code.
func (p *Provider) ExchangeCode(ctx context.Context, clientID, secret, code, redirectURI, verifier string, meta Meta) (*TokenResponse, error) {
	c, err := p.authenticateClient(clientID, secret)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(code, "oac_") || len(code) > 128 || !reVerifier.MatchString(verifier) {
		return nil, errInvalidGrant
	}
	h := sha256.Sum256([]byte(code))
	now := p.svc.now()
	ac, res, err := p.codes.Consume(ctx, h[:], now)
	if err != nil {
		return nil, err
	}
	switch res {
	case CodeReused:
		// RFC 6749 4.1.2: kod ikkinchi marta keldi — u bilan berilgan
		// tokenlar BEKOR qilinadi (kod o'g'irlangan bo'lishi mumkin).
		if ac != nil && ac.IssuedSessionID != "" {
			if sess, gerr := p.svc.store.Get(ctx, ac.IssuedSessionID); gerr == nil {
				_ = p.svc.revoke(ctx, sess, "code_reuse")
			}
		}
		p.svc.audit.Record(ctx, Event{Action: "oidc.code_reuse", Result: ResultDenied, Target: clientID})
		return nil, errInvalidGrant
	case CodeOK:
	default:
		return nil, errInvalidGrant
	}
	if ac.ClientID != c.ID || ac.RedirectURI != redirectURI ||
		subtle.ConstantTimeCompare([]byte(pkceS256(verifier)), []byte(ac.Challenge)) != 1 {
		p.svc.audit.Record(ctx, Event{Action: "oidc.exchange", Result: ResultDenied, ActorUserID: ac.UserID,
			Target: clientID, Details: map[string]any{"reason": "binding_mismatch"}})
		return nil, errInvalidGrant
	}
	parent, err := p.svc.store.Get(ctx, ac.ParentSessionID)
	if err != nil || !parent.Active(now) {
		return nil, errInvalidGrant
	}
	tok, err := p.svc.Create(ctx, CreateInput{UserID: ac.UserID, ClientID: c.ID, OrgID: ac.OrgID,
		AMR: ac.AMR, Meta: meta, ParentID: parent.ID})
	if err != nil {
		if errors.Is(err, ErrRealmMismatch) {
			return nil, errInvalidGrant
		}
		return nil, err
	}
	if err := p.codes.SetIssued(ctx, ac.Hash, tok.SessionID); err != nil {
		return nil, err
	}
	idt, err := p.idToken(c.ID, ac.UserID, tok.SessionID, ac.Nonce, ac.AuthTime, ac.AMR)
	if err != nil {
		return nil, err
	}
	return &TokenResponse{AccessToken: tok.AccessToken, TokenType: "Bearer", ExpiresIn: p.expiresIn(tok),
		RefreshToken: tok.RefreshToken, IDToken: idt, Scope: ac.Scope, SessionID: tok.SessionID}, nil
}

func (p *Provider) expiresIn(t *Tokens) int {
	n := int(t.ExpiresAt.Sub(p.svc.now()).Seconds())
	if n < 0 {
		return 0
	}
	return n
}

// RefreshGrant — grant_type=refresh_token. Refresh faqat O'Z klientiga.
func (p *Provider) RefreshGrant(ctx context.Context, clientID, secret, refresh string, meta Meta) (*TokenResponse, error) {
	c, err := p.authenticateClient(clientID, secret)
	if err != nil {
		return nil, err
	}
	tok, err := p.svc.RefreshForClient(ctx, refresh, c.ID, meta)
	if err != nil {
		return nil, errInvalidGrant
	}
	return &TokenResponse{AccessToken: tok.AccessToken, TokenType: "Bearer", ExpiresIn: p.expiresIn(tok),
		RefreshToken: tok.RefreshToken, SessionID: tok.SessionID}, nil
}

// IDClaims — id_token (OIDC Core 2).
type IDClaims struct {
	Nonce    string           `json:"nonce"`
	AuthTime *jwt.NumericDate `json:"auth_time"`
	SID      string           `json:"sid"`
	AMR      []string         `json:"amr,omitempty"`
	AZP      string           `json:"azp"`
	jwt.RegisteredClaims
}

func (p *Provider) idToken(clientID, userID, sid, nonce string, authTime time.Time, amr []string) (string, error) {
	now := p.svc.now()
	claims := IDClaims{Nonce: nonce, AuthTime: jwt.NewNumericDate(authTime), SID: sid, AMR: amr, AZP: clientID,
		RegisteredClaims: jwt.RegisteredClaims{Issuer: p.svc.issuer.issuer, Subject: userID,
			Audience: jwt.ClaimStrings{clientID}, IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(IDTokenTTL)), ID: newJTI()}}
	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	t.Header["kid"] = p.svc.issuer.keys.active.KID
	return t.SignedString(p.svc.issuer.keys.active.Private)
}

// EndSSO — SSO sessiyasini va undan ochilgan barcha ilova sessiyalarini
// yopadi (OnDex'dan chiqish, brauzerda boshqa akkaunt bilan kirish).
// Yopilgan bolalar qaytariladi — back-channel logout yuborish uchun (A4c.3).
func (p *Provider) EndSSO(ctx context.Context, sso *Session, reason string) error {
	if sso == nil || sso.ClientID != SSOClientID {
		return ErrNotFound
	}
	_, err := p.svc.revokeTree(ctx, sso, reason)
	return err
}

// IssuerURL — `iss`.
func (p *Provider) IssuerURL() string { return p.svc.issuer.issuer }

// Discovery — /.well-known/openid-configuration.
func (p *Provider) Discovery() map[string]any {
	iss := p.svc.issuer.issuer
	return map[string]any{
		"issuer":                                         iss,
		"authorization_endpoint":                         iss + "/v2/oauth/authorize",
		"token_endpoint":                                 iss + "/v2/oauth/token",
		"userinfo_endpoint":                              iss + "/v2/oauth/userinfo",
		"end_session_endpoint":                           iss + "/v2/oauth/logout",
		"jwks_uri":                                       iss + "/.well-known/jwks.json",
		"response_types_supported":                       []string{"code"},
		"grant_types_supported":                          []string{"authorization_code", "refresh_token"},
		"subject_types_supported":                        []string{"public"},
		"id_token_signing_alg_values_supported":          []string{"EdDSA"},
		"code_challenge_methods_supported":               []string{"S256"},
		"token_endpoint_auth_methods_supported":          []string{"client_secret_basic", "client_secret_post", "none"},
		"scopes_supported":                               []string{"openid", "profile", "phone"},
		"claims_supported":                               []string{"sub", "name", "phone_number", "sid", "auth_time", "amr", "nonce"},
		"prompt_values_supported":                        []string{"none", "login"},
		"backchannel_logout_supported":                   true,
		"backchannel_logout_session_supported":           true,
		"authorization_response_iss_parameter_supported": true,
	}
}
