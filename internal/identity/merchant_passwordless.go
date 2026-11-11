package identity

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"

	"chustapp/internal/users"
)

// Passkey ASOSIY kirish usuli, parol IXTIYORIY (Plane 317).
//
// ┌─ QOIDALAR ────────────────────────────────────────────────────────────┐
//  1. Kirish sahifasida birinchi yo'l — passkey (telefon raqamisiz,
//     "discoverable" kalit). UV majburiy: passkey = egalik (qurilma) +
//     foydalanuvchi (PIN/biometrik) — o'zi ko'p omilli va fishingga
//     chidamli (NIST SP 800-63B AAL3 ga yaqin). Qo'shimcha TOTP so'ralmaydi.
//  2. Parol bilan kirish faqat egasi akkaunt sahifasida YOQSA ishlaydi.
//     Yangi akkaunt: o'chiq. Migratsiya 0075: hozir parol bilan kirayotgan
//     akkauntlar uchun yoqiq (ilovalar uzilmasin).
//  3. Passkey yo'qolsa: telefon kodi + Authenticator kodi (yoki tiklash
//     kodi) -> sessiya -> akkaunt sahifasida yangi passkey.
//  4. Xatolar TOTP bilan BIR hisobda (`guard`), challenge bir martalik.
// └───────────────────────────────────────────────────────────────────────┘

// ErrPasswordLoginDisabled — akkauntda parol bilan kirish o'chiq.
var ErrPasswordLoginDisabled = errors.New("parol bilan kirish o'chirilgan — passkey yoki Telegram kodi bilan kiring (parolni akkaunt sozlamalarida yoqish mumkin)")

// ErrPasswordNeeded — parolni yoqish uchun avval parol o'rnatilishi kerak.
var ErrPasswordNeeded = errors.New("parol bilan kirishni yoqish uchun yangi parol kiriting")

// LoginPrefs — kirish sozlamalari ombori (`merchant_login_prefs`).
// Yozuv yo'q = parol bilan kirish O'CHIQ.
type LoginPrefs interface {
	PasswordLogin(ctx context.Context, userID string) (bool, error)
	SetPasswordLogin(ctx context.Context, userID string, enabled bool, at time.Time) error
}

// MemoryLoginPrefs — xotiradagi `LoginPrefs` (dev/test).
type MemoryLoginPrefs struct {
	mu sync.Mutex
	m  map[string]bool
}

func NewMemoryLoginPrefs() *MemoryLoginPrefs { return &MemoryLoginPrefs{m: map[string]bool{}} }

func (p *MemoryLoginPrefs) PasswordLogin(_ context.Context, userID string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.m[userID], nil
}

func (p *MemoryLoginPrefs) SetPasswordLogin(_ context.Context, userID string, enabled bool, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.m[userID] = enabled
	return nil
}

// passwordLoginAllowed — `Prefs` berilmagan bo'lsa (eski joylashuv) — ruxsat.
// Ombor xatosida — RAD (fail closed): parol eng zaif yo'l.
func (m *MerchantAuth) passwordLoginAllowed(ctx context.Context, userID string) bool {
	if m.d.Prefs == nil {
		return true
	}
	ok, err := m.d.Prefs.PasswordLogin(ctx, userID)
	if err != nil {
		slog.Error("identity: kirish sozlamasi o'qilmadi — parol bilan kirish rad", "user", userID, "err", err)
		return false
	}
	return ok
}

// PasswordLoginEnabled — akkaunt sahifasi uchun.
func (m *MerchantAuth) PasswordLoginEnabled(ctx context.Context, userID string) (bool, error) {
	if m.d.Prefs == nil {
		return true, nil
	}
	return m.d.Prefs.PasswordLogin(ctx, userID)
}

// EnablePasswordAtSignup — ro'yxatdan o'tishda parolni O'ZI kiritgan egasi
// uchun (aniq tanlov). Yozib bo'lmasa — xato (akkaunt parolsiz qolmasin).
func (m *MerchantAuth) EnablePasswordAtSignup(ctx context.Context, userID string) error {
	if m.d.Prefs == nil {
		return nil
	}
	return m.d.Prefs.SetPasswordLogin(ctx, userID, true, m.svc.now())
}

// SetPasswordLogin — akkaunt sahifasidan parol bilan kirishni yoqish/o'chirish.
// Ikkala yo'nalishda ham Authenticator kodi (step-up) SHART. Yoqishda parol
// hali yo'q bo'lsa — yangi parol (kuchlilik qoidalari bilan) o'rnatiladi.
// Faqat TOTP ulangan merchant akkaunti (egasi): xodimlar parol bilan kirmaydi.
func (m *MerchantAuth) SetPasswordLogin(ctx context.Context, userID string, enabled bool, code, password, confirm string, meta Meta) error {
	if m.d.Prefs == nil {
		return errors.New("kirish sozlamalari yoqilmagan")
	}
	u, err := m.svc.users.GetByID(ctx, userID)
	if err != nil || u.IsDeleted() || users.RealmForRole(u.Role) != users.RealmMerchant {
		return users.ErrInvalidCredentials
	}
	if ok, err := m.HasTOTP(ctx, userID); err != nil {
		return err
	} else if !ok {
		return ErrPasskeyNeedsTOTP
	}
	if err := m.StepUp(ctx, userID, code, "password_login", meta); err != nil {
		return err
	}
	if enabled && u.PasswordHash == "" {
		if password == "" {
			return ErrPasswordNeeded
		}
		// Birinchi parol: egalik HOZIRGINA Authenticator bilan isbotlandi.
		if err := m.d.Users.SetPassword(ctx, u.ID, "", password, confirm, true); err != nil {
			return err
		}
	}
	if err := m.d.Prefs.SetPasswordLogin(ctx, userID, enabled, m.svc.now()); err != nil {
		return err
	}
	action := "account.password_login_disabled"
	if enabled {
		action = "account.password_login_enabled"
	}
	m.svc.audit.Record(ctx, Event{Action: action, Result: ResultOK, ActorUserID: userID,
		Realm: string(users.RealmMerchant), IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
	return nil
}

// ---------- Parolsiz kirish: passkey (telefon raqamisiz) ----------

const passkeyPurposeDiscover = "discover"

// PasskeyDiscoverOptions — kirish sahifasining birinchi qadami: brauzer
// qurilmadagi OnDex passkey'larini o'zi taklif qiladi (allowCredentials
// BO'SH — akkaunt mavjudligi oshkor bo'lmaydi).
func (m *MerchantAuth) PasskeyDiscoverOptions(ctx context.Context, clientID string) (*protocol.CredentialAssertion, string, error) {
	if m.wa == nil {
		return nil, "", ErrPasskeyDisabled
	}
	if _, _, err := m.passkeyClient(clientID); err != nil {
		return nil, "", err
	}
	opts, sess, err := m.wa.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, "", err
	}
	st, err := m.signPasskeyState(passkeyState{Purpose: passkeyPurposeDiscover, Client: clientID, Session: *sess})
	return opts, st, err
}

// PasskeyDiscoverLogin — imzo to'g'ri bo'lsa sessiya (amr: hwk, user).
// Javob har qanday xatoda bir xil (ErrMFAInvalid) — qaysi akkaunt yoki
// qaysi tekshiruv ekanligi oshkor bo'lmaydi.
func (m *MerchantAuth) PasskeyDiscoverLogin(ctx context.Context, clientID, state string, response []byte, meta Meta) (*Outcome, error) {
	if m.wa == nil {
		return nil, ErrPasskeyDisabled
	}
	realm, app, err := m.passkeyClient(clientID)
	if err != nil {
		return nil, err
	}
	st, err := m.parsePasskeyState(state, passkeyPurposeDiscover, "")
	if err != nil || st.Client != clientID {
		return nil, ErrPasskeyState
	}
	if err := m.useChallenge(ctx, st); err != nil {
		return nil, err
	}
	if len(response) > maxPasskeyResponse {
		return nil, ErrMFAInvalid
	}
	parsed, err := protocol.ParseCredentialRequestResponseBytes(response)
	if err != nil {
		return nil, ErrMFAInvalid
	}

	var found *passkeyUser
	blocked := false
	handler := func(_, userHandle []byte) (webauthn.User, error) {
		uid := string(userHandle)
		if uid == "" || len(uid) > 128 {
			return nil, ErrMFAInvalid
		}
		if m.guard.Blocked(uid) {
			blocked = true
			return nil, ErrMFATooMany
		}
		pu, _, err := m.passkeyUserOf(ctx, uid)
		if err != nil {
			return nil, ErrMFAInvalid
		}
		found = pu
		return *pu, nil
	}
	_, cred, err := m.wa.ValidatePasskeyLogin(handler, st.Session, parsed)
	if blocked {
		return nil, ErrMFATooMany
	}
	if err != nil {
		if found != nil {
			m.guard.Fail(found.u.ID)
			m.deny(ctx, found.u.ID, clientID, "passkey_invalid", meta)
		}
		slog.Warn("passkey: parolsiz kirish rad etildi", "err", err)
		return nil, ErrMFAInvalid
	}
	u := found.u
	if cred.Authenticator.CloneWarning {
		m.svc.audit.Record(ctx, Event{Action: "mfa.passkey_clone_warning", Result: ResultDenied, ActorUserID: u.ID,
			Realm: string(users.RealmMerchant), Target: clientID, IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
		m.guard.Fail(u.ID)
		return nil, ErrMFAInvalid
	}
	org, err := m.passkeySessionOrg(ctx, u, clientID, realm, app, meta)
	if err != nil {
		return nil, err
	}
	if data, err := json.Marshal(cred); err == nil {
		if err := m.d.Passkeys.Touch(ctx, u.ID, base64.RawURLEncoding.EncodeToString(cred.ID), data, m.svc.now()); err != nil {
			slog.Warn("passkey: hisoblagich yangilanmadi", "user", u.ID, "err", err)
		}
	}
	m.guard.Reset(u.ID)
	tok, err := m.svc.Create(ctx, CreateInput{UserID: u.ID, ClientID: clientID, OrgID: org,
		AMR: []string{"hwk", "user"}, Meta: meta})
	if err != nil {
		return nil, err
	}
	return &Outcome{Tokens: tok}, nil
}
