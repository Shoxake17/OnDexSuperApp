package staff

import (
	"context"
	"errors"
	"time"

	"chustapp/internal/revoke"
	"chustapp/internal/users"
)

// UserAccounts — "OnDex Affitsiant" ilovasi akkauntlari (`users`).
//
// ┌─ BEGONA AKKAUNTGA TEGILMAYDI ─────────────────────────────────────┐
// Restoran telefon raqamini o'zi kiritadi. Shu raqamda allaqachon
// BOSHQA akkaunt bo'lsa (mijoz, kuryer, boshqa restoran xodimi) u
// affitsiantga AYLANTIRILMAYDI — aks holda restoran istalgan odamning
// akkauntini egallab olardi (`ErrAccountConflict`).
//
// Faqat quyidagilar ulanadi:
//   - raqamda akkaunt yo'q — yangi affitsiant akkaunti yaratiladi
//     (parolsiz: xodim ilovaga SMS/Telegram kod bilan kiradi, restoran
//     uning parolini hech qachon bilmaydi);
//   - akkaunt allaqachon SHU restoranning affitsianti;
//   - xodim yozuviga ilgari bog'langan va keyin yopilgan (mijoz roliga
//     qaytarilgan) aynan o'sha akkaunt — qayta ishga olish.
//
// └───────────────────────────────────────────────────────────────────┘
type UserAccounts struct {
	Users users.Repository
	// Revoked — nil bo'lsa sessiyalar bekor qilinmaydi (faqat testlar).
	Revoked *revoke.Store
	NewID   func() string
}

func (a *UserAccounts) Enable(ctx context.Context, m *Member) (string, error) {
	if m.UserID != "" {
		u, err := a.Users.GetByID(ctx, m.UserID)
		switch {
		case err == nil:
			if u.Role == users.RoleWaiter && u.EntityID == m.RestaurantID {
				return u.ID, nil
			}
			if u.Role == users.RoleCustomer && u.EntityID == "" && u.Phone == m.Phone {
				if err := a.Users.UpdateRole(ctx, u.ID, users.RoleWaiter, m.RestaurantID); err != nil {
					return "", err
				}
				return u.ID, nil
			}
			return "", ErrAccountConflict
		case !errors.Is(err, users.ErrUserNotFound):
			return "", err
		}
		// Bog'langan akkaunt o'chirilgan — raqam bo'yicha davom etiladi.
	}
	u, err := a.Users.GetByPhone(ctx, m.Phone)
	switch {
	case err == nil:
		if u.Role == users.RoleWaiter && u.EntityID == m.RestaurantID {
			return u.ID, nil
		}
		return "", ErrAccountConflict
	case !errors.Is(err, users.ErrUserNotFound):
		return "", err
	}
	account := users.User{
		ID: a.NewID(), Phone: m.Phone, Name: m.FullName(),
		Role: users.RoleWaiter, EntityID: m.RestaurantID, CreatedAt: time.Now(),
	}
	if err := a.Users.Create(ctx, &account); err != nil {
		if errors.Is(err, users.ErrPhoneTaken) {
			return "", ErrAccountConflict
		}
		return "", err
	}
	return account.ID, nil
}

// Disable — akkaunt o'chirilmaydi (odam oddiy mijoz sifatida qoladi),
// rol `customer` ga qaytariladi va sessiyalar DARHOL bekor qilinadi:
// `auth()` faqat imzoni tekshiradi, eski tokendagi `waiter` roli
// busiz 30 kun ishlayverardi.
func (a *UserAccounts) Disable(ctx context.Context, restaurantID, userID string) error {
	u, err := a.Users.GetByID(ctx, userID)
	if errors.Is(err, users.ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if u.Role != users.RoleWaiter || u.EntityID != restaurantID {
		return nil
	}
	if err := a.Users.UpdateRole(ctx, userID, users.RoleCustomer, ""); err != nil {
		return err
	}
	if a.Revoked != nil {
		a.Revoked.Revoke(ctx, userID)
	}
	return nil
}
