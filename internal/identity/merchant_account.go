package identity

import (
	"context"
	"errors"

	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// OnDex ID akkaunt sahifasi (passport.ondex.uz, Plane 270) va xavfli amallar
// uchun qayta tasdiq (step-up, Plane 135).

// ErrStepUpRequired — amal ikkinchi omil kodini talab qiladi (TOTP yoki
// tiklash kodi), kod berilmagan.
var ErrStepUpRequired = errors.New("bu amal uchun Authenticator kodi kerak")

// StepUp — xavfli amaldan oldin egalikni HOZIR qayta isbotlash: TOTP yoki
// tiklash kodi. TOTP ulanmagan akkauntda (xodim) — talab yo'q (nil); bunday
// amallarni chaqiruvchi o'zi boshqa dalil bilan cheklaydi (masalan joriy
// parol). Urinishlar kirishdagi bilan BIR hisobda: step-up orqali TOTP'ni
// terib chiqib bo'lmaydi.
func (m *MerchantAuth) StepUp(ctx context.Context, userID, code, action string, meta Meta) error {
	end, ok := m.guard.Begin(userID)
	if !ok {
		return ErrMFATooMany
	}
	res := attemptAbort
	defer func() { end(res) }()
	if _, err := m.d.MFA.Confirmed(ctx, userID); errors.Is(err, mfa.ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	if code == "" {
		return ErrStepUpRequired
	}
	if _, err := m.checkFactorCode(ctx, userID, code); err != nil {
		if errors.Is(err, ErrMFAInvalid) {
			res = attemptFail
			m.svc.audit.Record(ctx, Event{Action: "stepup." + action, Result: ResultDenied, ActorUserID: userID,
				Realm: string(users.RealmMerchant), IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
		}
		return err
	}
	res = attemptOK
	return nil
}

// HasTOTP — akkauntda tasdiqlangan TOTP bormi.
func (m *MerchantAuth) HasTOTP(ctx context.Context, userID string) (bool, error) {
	_, err := m.d.MFA.Confirmed(ctx, userID)
	if errors.Is(err, mfa.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// ChangePassword — akkaunt sahifasidan parolni almashtirish: joriy parol +
// (TOTP ulangan bo'lsa) Authenticator kodi. Muvaffaqiyatda foydalanuvchining
// BARCHA sessiyalari yopiladi — parol o'g'irlangan bo'lsa, hujumchi ham
// chiqariladi; foydalanuvchi yangi parol bilan qayta kiradi.
func (m *MerchantAuth) ChangePassword(ctx context.Context, userID, current, password, confirm, code string, meta Meta) error {
	end, ok := m.guard.Begin(userID)
	if !ok {
		return ErrMFATooMany
	}
	res := attemptAbort
	defer func() { end(res) }()
	u, err := m.svc.users.GetByID(ctx, userID)
	if err != nil || u.IsDeleted() || users.RealmForRole(u.Role) != users.RealmMerchant {
		return users.ErrInvalidCredentials
	}
	if u.PasswordHash == "" || current == "" {
		return users.ErrInvalidCredentials
	}
	if err := m.StepUp(ctx, userID, code, "password_change", meta); err != nil {
		return err
	}
	// Joriy parol `SetPassword` ichida tekshiriladi (vaqtga chidamli taqqoslash).
	if err := m.d.Users.SetPassword(ctx, u.ID, current, password, confirm, false); err != nil {
		if errors.Is(err, users.ErrCurrentPasswordWrong) {
			res = attemptFail
		}
		return err
	}
	res = attemptOK
	m.svc.audit.Record(ctx, Event{Action: "account.password_changed", Result: ResultOK, ActorUserID: u.ID,
		Realm: string(users.RealmMerchant), IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300)})
	if _, err := m.svc.RevokeAllForUser(ctx, u.ID, "password_changed"); err != nil {
		return err
	}
	return nil
}
