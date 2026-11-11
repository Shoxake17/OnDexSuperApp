package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"chustapp/internal/identity/passkey"
	"chustapp/internal/users"
)

// Passkey (WebAuthn) — fishingga chidamli ikkinchi bosqich (AUD-4).
//
// ┌─ QOIDALAR ────────────────────────────────────────────────────────────┐
//  1. Passkey TOTP'ga MUQOBIL (ikkinchi bosqichda), birinchi bosqich
//     (parol / telefon kodi) o'zgarmaydi.
//  2. Ulash faqat TOTP ulangan akkauntda va Authenticator kodi bilan
//     (step-up): o'g'irlangan sessiya o'z passkey'ini qo'sha olmaydi.
//     O'chirish ham kod bilan.
//  3. Foydalanuvchini tekshirish (UV: PIN/barmoq izi/yuz) MAJBURIY — faqat
//     "qurilma yonida" emas, egasi ekanligi.
//  4. Challenge BIR MARTALIK (bazada), holat (state) imzolangan va 5 daqiqa;
//     kirishdagi holat aynan shu mfa_token'ga bog'langan (nonce).
//  5. Imzo hisoblagichi orqaga ketsa (klonlangan kalit belgisi) — rad.
//  6. Xatolar TOTP bilan BIR hisobda (`guard`): terib chiqib bo'lmaydi.
// └───────────────────────────────────────────────────────────────────────┘

var (
	ErrPasskeyDisabled  = errors.New("passkey hali yoqilmagan")
	ErrPasskeyNone      = errors.New("bu akkauntda passkey yo'q")
	ErrPasskeyNeedsTOTP = errors.New("passkey ulashdan oldin Authenticator (TOTP) ulang")
	ErrPasskeyState     = errors.New("passkey so'rovi eskirgan — qaytadan urinib ko'ring")
)

const (
	passkeyStateTTL     = 5 * time.Minute
	passkeyStatePrefix  = "pks1."
	passkeyPurposeLogin = "login"
	passkeyPurposeReg   = "register"
	maxPasskeyResponse  = 64 << 10
)

// PasskeyConfig — RP (relying party). Bo'sh bo'lsa issuer'dan olinadi:
// RPID = issuer host, Origins = [issuer origin]. Kirish va akkaunt
// sahifalari aynan shu manzildan beriladi.
type PasskeyConfig struct {
	RPID    string
	Origins []string
	// OpaqueOrigins — Android ilova imzolari (android:apk-key-hash:...);
	// ResolvePasskeyConfig Origins'dan ajratadi.
	OpaqueOrigins []string
	// AndroidApps — PASSKEY_ANDROID_APPS: origin'lari OpaqueOrigins'ga,
	// o'zi assetlinks.json'ga. AppleAppIDs — apple-app-site-association.
	AndroidApps []AndroidApp
	AppleAppIDs []string
}

// passkeyUser — webauthn.User adapteri.
type passkeyUser struct {
	u     *users.User
	creds []webauthn.Credential
}

func (p passkeyUser) WebAuthnID() []byte { return []byte(p.u.ID) }
func (p passkeyUser) WebAuthnName() string {
	if p.u.Phone != "" {
		return p.u.Phone
	}
	return p.u.ID
}

// WebAuthnDisplayName — passkey ro'yxatida ko'rinadi. Bitta raqamda bir
// nechta akkaunt (mijoz, biznes, admin) bo'lishi mumkin — realm yoziladi.
func (p passkeyUser) WebAuthnDisplayName() string {
	name := strings.TrimSpace(p.u.Name)
	if name == "" {
		name = p.WebAuthnName()
	}
	switch users.RealmForRole(p.u.Role) {
	case users.RealmMerchant:
		return name + " (OnDex Biznes)"
	case users.RealmPlatform:
		return name + " (OnDex Admin)"
	}
	return name + " (OnDex)"
}
func (p passkeyUser) WebAuthnCredentials() []webauthn.Credential { return p.creds }

type passkeyState struct {
	Purpose string               `json:"p"`
	User    string               `json:"u"`
	Nonce   string               `json:"n,omitempty"` // kirish: mfa_token nonce'i
	Client  string               `json:"c,omitempty"` // parolsiz kirish: klient
	Exp     int64                `json:"e"`
	Session webauthn.SessionData `json:"s"`
}

// initPasskeys — NewMerchantAuth ichidan; Passkeys nil = o'chiq.
func (m *MerchantAuth) initPasskeys() error {
	if m.d.Passkeys == nil {
		return nil
	}
	cfg, err := ResolvePasskeyConfig(m.d.Passkey, m.svc.issuer.issuer)
	if err != nil {
		return err
	}
	m.rpID = cfg.RPID
	m.pkCfg = cfg
	wa, err := webauthn.New(&webauthn.Config{
		RPID: cfg.RPID, RPDisplayName: "OnDex", RPOrigins: cfg.Origins, RPOpaqueOrigins: cfg.OpaqueOrigins,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			// Required: kalit qurilmada SAQLANADI (discoverable) — telefon
			// raqamisiz kirish (Plane 317) shunga tayanadi.
			ResidentKey: protocol.ResidentKeyRequirementRequired, RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification: protocol.VerificationRequired,
		},
		AttestationPreference: protocol.PreferNoAttestation,
	})
	if err != nil {
		return err
	}
	m.wa = wa
	m.pkKey = m.svc.issuer.keys.derivedSecret("passkey-state")
	return nil
}

// PasskeysEnabled — passkey xizmati yoqilganmi.
func (m *MerchantAuth) PasskeysEnabled() bool { return m.wa != nil }

func (m *MerchantAuth) passkeyUserOf(ctx context.Context, userID string) (*passkeyUser, []passkey.Credential, error) {
	u, err := m.d.Repo.GetByID(ctx, userID)
	if err != nil || u.IsDeleted() {
		return nil, nil, ErrMFATokenInvalid
	}
	list, err := m.d.Passkeys.List(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	pu := &passkeyUser{u: u}
	for _, c := range list {
		var wc webauthn.Credential
		if err := json.Unmarshal(c.Data, &wc); err != nil {
			slog.Error("passkey: saqlangan yozuv o'qilmadi", "user", userID, "err", err)
			continue
		}
		pu.creds = append(pu.creds, wc)
	}
	return pu, list, nil
}

// hasPasskey — kirish sahifasi passkey tugmasini ko'rsatishi uchun.
func (m *MerchantAuth) hasPasskey(ctx context.Context, userID string) bool {
	if m.wa == nil {
		return false
	}
	list, err := m.d.Passkeys.List(ctx, userID)
	return err == nil && len(list) > 0
}

func (m *MerchantAuth) signPasskeyState(st passkeyState) (string, error) {
	st.Exp = m.svc.now().Add(passkeyStateTTL).Unix()
	raw, err := json.Marshal(st)
	if err != nil {
		return "", err
	}
	p := base64.RawURLEncoding.EncodeToString(raw)
	h := hmac.New(sha256.New, m.pkKey)
	h.Write([]byte(p))
	return passkeyStatePrefix + p + "." + base64.RawURLEncoding.EncodeToString(h.Sum(nil)), nil
}

func (m *MerchantAuth) parsePasskeyState(tok, purpose, userID string) (*passkeyState, error) {
	if len(tok) > 16<<10 || !strings.HasPrefix(tok, passkeyStatePrefix) {
		return nil, ErrPasskeyState
	}
	p, sig, ok := strings.Cut(strings.TrimPrefix(tok, passkeyStatePrefix), ".")
	if !ok {
		return nil, ErrPasskeyState
	}
	h := hmac.New(sha256.New, m.pkKey)
	h.Write([]byte(p))
	want := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return nil, ErrPasskeyState
	}
	raw, err := base64.RawURLEncoding.DecodeString(p)
	if err != nil {
		return nil, ErrPasskeyState
	}
	var st passkeyState
	if json.Unmarshal(raw, &st) != nil || st.Purpose != purpose || st.User != userID || m.svc.now().Unix() > st.Exp {
		return nil, ErrPasskeyState
	}
	return &st, nil
}

// useChallenge — bir martalik (replay himoyasi).
func (m *MerchantAuth) useChallenge(ctx context.Context, st *passkeyState) error {
	sum := sha256.Sum256([]byte(st.Session.Challenge))
	ok, err := m.d.Passkeys.UseChallenge(ctx, sum[:], time.Unix(st.Exp, 0).Add(time.Minute))
	if err != nil {
		return err
	}
	if !ok {
		return ErrPasskeyState
	}
	return nil
}

// ---------- Kirish: ikkinchi bosqich ----------

// PasskeyLoginOptions — mfa_token (method=totp) uchun passkey so'rovi.
// Natija: brauzerga `navigator.credentials.get` parametrlari va imzolangan holat.
func (m *MerchantAuth) PasskeyLoginOptions(ctx context.Context, mfaToken string) (*protocol.CredentialAssertion, string, error) {
	if m.wa == nil {
		return nil, "", ErrPasskeyDisabled
	}
	c, err := m.parseMFA(mfaToken, mfaPurposeVerify)
	if err != nil {
		return nil, "", err
	}
	if m.guard.Blocked(c.User) {
		return nil, "", ErrMFATooMany
	}
	pu, _, err := m.passkeyUserOf(ctx, c.User)
	if err != nil {
		return nil, "", err
	}
	if len(pu.creds) == 0 {
		return nil, "", ErrPasskeyNone
	}
	opts, sess, err := m.wa.BeginLogin(pu, webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, "", err
	}
	st, err := m.signPasskeyState(passkeyState{Purpose: passkeyPurposeLogin, User: c.User, Nonce: c.Nonce, Session: *sess})
	return opts, st, err
}

// VerifyPasskey — passkey imzosi to'g'ri bo'lsa sessiya ochiladi (VerifyMFA o'rniga).
func (m *MerchantAuth) VerifyPasskey(ctx context.Context, mfaToken, state string, response []byte, meta Meta) (*Tokens, error) {
	if m.wa == nil {
		return nil, ErrPasskeyDisabled
	}
	c, err := m.parseMFA(mfaToken, mfaPurposeVerify)
	if err != nil {
		return nil, err
	}
	if m.guard.Blocked(c.User) {
		return nil, ErrMFATooMany
	}
	st, err := m.parsePasskeyState(state, passkeyPurposeLogin, c.User)
	if err != nil || st.Nonce != c.Nonce {
		return nil, ErrPasskeyState
	}
	if err := m.useChallenge(ctx, st); err != nil {
		return nil, err
	}
	if len(response) > maxPasskeyResponse {
		m.mfaFailed(ctx, c, meta)
		return nil, ErrMFAInvalid
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		m.mfaFailed(ctx, c, meta)
		return nil, ErrMFAInvalid
	}
	pu, _, err := m.passkeyUserOf(ctx, c.User)
	if err != nil {
		return nil, err
	}
	cred, err := m.wa.ValidateLogin(pu, st.Session, parsed)
	if err != nil {
		slog.Warn("passkey: imzo rad etildi", "user", c.User, "err", err)
		m.mfaFailed(ctx, c, meta)
		return nil, ErrMFAInvalid
	}
	if cred.Authenticator.CloneWarning {
		// Imzo hisoblagichi orqaga ketdi — kalit nusxalangan bo'lishi mumkin.
		m.svc.audit.Record(ctx, Event{Action: "mfa.passkey_clone_warning", Result: ResultDenied, ActorUserID: c.User,
			Realm: string(users.RealmMerchant), Target: c.Client, IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
		m.mfaFailed(ctx, c, meta)
		return nil, ErrMFAInvalid
	}
	if data, err := json.Marshal(cred); err == nil {
		if err := m.d.Passkeys.Touch(ctx, c.User, base64.RawURLEncoding.EncodeToString(cred.ID), data, m.svc.now()); err != nil {
			slog.Warn("passkey: hisoblagich yangilanmadi", "user", c.User, "err", err)
		}
	}
	m.guard.Reset(c.User)
	if err := m.consumeMFA(ctx, c); err != nil {
		return nil, err
	}
	// RFC 8176: hwk — apparat kaliti egaligi; user — foydalanuvchi tekshirildi.
	return m.svc.Create(ctx, CreateInput{UserID: c.User, ClientID: c.Client, OrgID: c.Org,
		AMR: append(slices.Clone(c.AMR), "hwk", "user"), Meta: meta})
}

// ---------- Akkaunt: ulash, ro'yxat, o'chirish ----------

// PasskeyView — akkaunt sahifasi uchun (kalit ma'lumotisiz).
type PasskeyView struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	Synced     bool       `json:"synced"` // bulutda sinxronlanadigan passkey (BS flag)
}

func (m *MerchantAuth) ListPasskeys(ctx context.Context, userID string) ([]PasskeyView, error) {
	if m.wa == nil {
		return nil, ErrPasskeyDisabled
	}
	list, err := m.d.Passkeys.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]PasskeyView, 0, len(list))
	for _, c := range list {
		var wc webauthn.Credential
		_ = json.Unmarshal(c.Data, &wc)
		out = append(out, PasskeyView{ID: c.ID, Name: c.Name, CreatedAt: c.CreatedAt, LastUsedAt: c.LastUsedAt,
			Synced: wc.Flags.BackupState})
	}
	return out, nil
}

// PasskeyRegisterOptions — yangi passkey ulash boshlanishi. TOTP ulangan
// bo'lishi va Authenticator kodi (step-up) SHART.
func (m *MerchantAuth) PasskeyRegisterOptions(ctx context.Context, userID, code string, meta Meta) (*protocol.CredentialCreation, string, error) {
	if m.wa == nil {
		return nil, "", ErrPasskeyDisabled
	}
	// PK-2: TOTP ulangan — Authenticator kodi; xodim/mijoz — Telegram kodi.
	if err := m.passkeyStepUp(ctx, userID, code, "passkey_add", meta); err != nil {
		return nil, "", err
	}
	pu, list, err := m.passkeyUserOf(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	if len(list) >= passkey.MaxPerUser {
		return nil, "", passkey.ErrTooMany
	}
	excl := make([]protocol.CredentialDescriptor, 0, len(pu.creds))
	for i := range pu.creds {
		excl = append(excl, pu.creds[i].Descriptor())
	}
	opts, sess, err := m.wa.BeginRegistration(pu, webauthn.WithExclusions(excl))
	if err != nil {
		return nil, "", err
	}
	st, err := m.signPasskeyState(passkeyState{Purpose: passkeyPurposeReg, User: userID, Session: *sess})
	return opts, st, err
}

// PasskeyRegister — brauzer javobini tekshirib passkey'ni saqlaydi.
func (m *MerchantAuth) PasskeyRegister(ctx context.Context, userID, state string, response []byte, name string, meta Meta) (*PasskeyView, error) {
	if m.wa == nil {
		return nil, ErrPasskeyDisabled
	}
	st, err := m.parsePasskeyState(state, passkeyPurposeReg, userID)
	if err != nil {
		return nil, err
	}
	if err := m.useChallenge(ctx, st); err != nil {
		return nil, err
	}
	if len(response) > maxPasskeyResponse {
		return nil, ErrPasskeyState
	}
	parsed, err := protocol.ParseCredentialCreationResponseBytes(response)
	if err != nil {
		return nil, ErrPasskeyState
	}
	pu, _, err := m.passkeyUserOf(ctx, userID)
	if err != nil {
		return nil, err
	}
	cred, err := m.wa.CreateCredential(pu, st.Session, parsed)
	if err != nil {
		slog.Warn("passkey: ulash rad etildi", "user", userID, "err", err)
		return nil, ErrPasskeyState
	}
	if !cred.Flags.UserVerified {
		return nil, ErrPasskeyState
	}
	data, err := json.Marshal(cred)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Passkey"
	}
	if r := []rune(name); len(r) > 64 {
		name = string(r[:64])
	}
	pc := &passkey.Credential{ID: base64.RawURLEncoding.EncodeToString(cred.ID), UserID: userID, Name: name,
		Data: data, CreatedAt: m.svc.now()}
	if err := m.d.Passkeys.Add(ctx, pc); err != nil {
		return nil, err
	}
	m.svc.audit.Record(ctx, Event{Action: "account.passkey_added", Result: ResultOK, ActorUserID: userID,
		Realm: string(users.RealmMerchant), Target: clip(name, 200), IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
	return &PasskeyView{ID: pc.ID, Name: pc.Name, CreatedAt: pc.CreatedAt, Synced: cred.Flags.BackupState}, nil
}

// DeletePasskey — Authenticator kodi bilan.
func (m *MerchantAuth) DeletePasskey(ctx context.Context, userID, id, code string, meta Meta) error {
	if m.wa == nil {
		return ErrPasskeyDisabled
	}
	if err := m.passkeyStepUp(ctx, userID, code, "passkey_remove", meta); err != nil {
		return err
	}
	if err := m.d.Passkeys.Delete(ctx, userID, id); err != nil {
		return err
	}
	m.svc.audit.Record(ctx, Event{Action: "account.passkey_removed", Result: ResultOK, ActorUserID: userID,
		Realm: string(users.RealmMerchant), IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
	return nil
}

// PasskeyRPID — passkey RP ID (bo'sh — passkey o'chiq).
func (m *MerchantAuth) PasskeyRPID() string { return m.rpID }
