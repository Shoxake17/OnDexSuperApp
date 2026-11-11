package identity

import (
	"context"
	"errors"
	"slices"

	"chustapp/internal/identity/authz"
	"chustapp/internal/users"
)

// ┌─ PASSKEY BARCHA REALMLARDA (PK-2, Plane 319) ─────────────────────────┐
//   merchant (egasi, xodim) — klientning ilovasiga a'zolik bo'yicha;
//   platform (admin)        — faqat admin roli; IP ro'yxati HTTP qatlamida;
//   customer (mijoz)        — mijoz akkaunti.
// Passkey qo'shish/o'chirish — egalikni HOZIR isbotlash bilan:
//   TOTP ulangan akkaunt -> Authenticator yoki tiklash kodi;
//   TOTP'siz (xodim, mijoz) -> telefon kodi (Telegram/SMS).
//   Egasi va admin uchun TOTP MAJBURIY (passkey uning o'rnini bosmaydi,
//   zaxira yo'li).
// └────────────────────────────────────────────────────────────────────────┘

// ErrPhoneStepUpRequired — amal uchun telefon (Telegram) kodi kerak.
var ErrPhoneStepUpRequired = errors.New("bu amal uchun Telegram'ga kelgan kod kerak")

// StrongAMR — sessiya kuchli ikkinchi omil bilan ochilganmi: TOTP, tiklash
// kodi yoki foydalanuvchi tekshirilgan passkey (hwk + user, RFC 8176).
func StrongAMR(amr []string) bool {
	return slices.Contains(amr, "totp") || slices.Contains(amr, "recovery") ||
		(slices.Contains(amr, "hwk") && slices.Contains(amr, "user"))
}

// passkeyClient — klientning realmi va (merchant uchun) ilovasi.
func (m *MerchantAuth) passkeyClient(clientID string) (users.Realm, string, error) {
	pol, ok := m.svc.clients[clientID]
	if !ok {
		return "", "", ErrUnknownClient
	}
	switch pol.Realm {
	case users.RealmMerchant:
		app := authz.AppForClient(clientID)
		if app == "" {
			return "", "", ErrUnknownClient
		}
		return pol.Realm, app, nil
	case users.RealmPlatform, users.RealmCustomer:
		return pol.Realm, "", nil
	}
	return "", "", ErrUnknownClient
}

// passkeySessionOrg — passkey egasining shu klientga kirish huquqi va
// tashkiloti. Rad — ErrMFAInvalid (sababi oshkor emas) yoki ErrAppNotAllowed.
func (m *MerchantAuth) passkeySessionOrg(ctx context.Context, u *users.User, clientID string, realm users.Realm, app string, meta Meta) (string, error) {
	if u.IsDeleted() || users.RealmForRole(u.Role) != realm {
		m.deny(ctx, u.ID, clientID, "passkey_wrong_realm", meta)
		return "", ErrMFAInvalid
	}
	switch realm {
	case users.RealmPlatform:
		if u.Role != users.RoleAdmin {
			m.deny(ctx, u.ID, clientID, "not_admin", meta)
			return "", ErrMFAInvalid
		}
		return "", nil
	case users.RealmCustomer:
		return "", nil
	}
	kind, org, err := m.classify(ctx, u, app)
	switch {
	case errors.Is(err, ErrAppNotAllowed):
		m.deny(ctx, u.ID, clientID, "app_not_allowed", meta)
		return "", ErrAppNotAllowed
	case err != nil:
		return "", err
	case kind == kindNone:
		m.deny(ctx, u.ID, clientID, "not_merchant", meta)
		return "", ErrMFAInvalid
	}
	return org, nil
}

// requiresTOTP — TOTP majburiy akkaunt (egasi yoki admin).
func (m *MerchantAuth) requiresTOTP(ctx context.Context, u *users.User) (bool, error) {
	if u.Role == users.RoleAdmin {
		return true, nil
	}
	if users.RealmForRole(u.Role) != users.RealmMerchant {
		return false, nil
	}
	list, err := m.d.Authz.List(ctx, u.ID)
	if err != nil {
		return false, err
	}
	for _, a := range list {
		if a.IsOwner {
			return true, nil
		}
	}
	// A'zoliksiz eski egasi akkaunti (rol ko'prigi).
	return len(list) == 0 && (u.Role == users.RoleRestaurant || u.Role == users.RoleStore), nil
}

// passkeyStepUp — passkey qo'shish/o'chirishdan oldin egalikni HOZIR isbotlash.
func (m *MerchantAuth) passkeyStepUp(ctx context.Context, userID, code, action string, meta Meta) error {
	hasTOTP, err := m.HasTOTP(ctx, userID)
	if err != nil {
		return err
	}
	if hasTOTP {
		if code == "" {
			return ErrStepUpRequired
		}
		return m.StepUp(ctx, userID, code, action, meta)
	}
	u, err := m.svc.users.GetByID(ctx, userID)
	if err != nil || u.IsDeleted() || u.Phone == "" {
		return users.ErrInvalidCredentials
	}
	if need, err := m.requiresTOTP(ctx, u); err != nil {
		return err
	} else if need {
		return ErrPasskeyNeedsTOTP
	}
	if code == "" {
		return ErrPhoneStepUpRequired
	}
	end, ok := m.guard.Begin(userID)
	if !ok {
		return ErrMFATooMany
	}
	res := attemptAbort
	defer func() { end(res) }()
	if _, err := m.d.Users.VerifyPhoneCode(ctx, u.Phone, code); err != nil {
		res = attemptFail
		m.svc.audit.Record(ctx, Event{Action: "stepup." + action, Result: ResultDenied, ActorUserID: userID,
			Realm: string(users.RealmForRole(u.Role)), IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
		return ErrMFAInvalid
	}
	res = attemptOK
	return nil
}
