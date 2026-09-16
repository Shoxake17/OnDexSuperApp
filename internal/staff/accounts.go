package staff

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"chustapp/internal/couriers"
	"chustapp/internal/revoke"
	"chustapp/internal/users"
)

// UserAccounts — xodimning OnDex ilovasi akkaunti (`users`) va, yetkazib
// beruvchi uchun, uning kuryer yozuvi (`couriers`).
//
// Lavozim → ilova:
//
//	ofitsiant         → "OnDex Affitsiant", rol `waiter`,  EntityID = restoran ID
//	yetkazib beruvchi → "OnDex Kuryer",     rol `courier`, EntityID = kuryer yozuvi
//
// ┌─ BEGONA AKKAUNTGA TEGILMAYDI ─────────────────────────────────────┐
// Restoran telefon raqamini o'zi kiritadi. Shu raqamda allaqachon
// BOSHQA akkaunt bo'lsa (mijoz, OnDex kuryeri, boshqa restoran xodimi)
// u xodim akkauntiga AYLANTIRILMAYDI — aks holda restoran istalgan
// odamning akkauntini egallab olardi (`ErrAccountConflict`).
//
// Faqat quyidagilar ulanadi:
//   - raqamda akkaunt yo'q — yangi akkaunt yaratiladi (parolsiz: xodim
//     ilovaga SMS/Telegram kod bilan kiradi, restoran uning parolini
//     hech qachon bilmaydi);
//   - akkaunt allaqachon SHU xodim yozuviga tegishli;
//   - xodim yozuviga ilgari bog'langan va keyin yopilgan (mijoz roliga
//     qaytarilgan) aynan o'sha akkaunt — qayta ishga olish yoki
//     ofitsiant ↔ yetkazib beruvchi almashuvi.
//
// └───────────────────────────────────────────────────────────────────┘
//
// ┌─ KURYER YOZUVI XODIMGA ERGASHADI ─────────────────────────────────┐
// Kuryer yozuvi ID'si xodim ID'sidan olinadi (`courierIDFor`): qayta
// ishga olinganda xuddi o'sha yozuv qaytadi va yetkazmalar tarixi
// (`completed_orders`, buyurtmalardagi `courier_id`) uzilmaydi.
//
// Tasdiq (`approved`) — dispatch havuziga kirish kaliti — FAQAT akkaunt
// bog'langandan KEYIN yoqiladi va yopishda BIRINCHI o'chiriladi. Oraliq
// qadam yiqilsa kuryer tasdiqsiz qoladi, ya'ni taklif olmaydi va onlayn
// bo'la olmaydi (fail closed).
// └───────────────────────────────────────────────────────────────────┘
type UserAccounts struct {
	Users users.Repository
	// Couriers — yetkazib beruvchining kuryer yozuvi. nil bo'lsa
	// kuryerga kirish ochilmaydi (`ErrAccountsDisabled`).
	Couriers couriers.Repository
	// CourierBusy — kuryerda yakunlanmagan yetkazma bormi. nil bo'lsa
	// kuryer kirishini YOPISH rad etiladi: faol buyurtmani tekshirib
	// bo'lmasa, uni egasiz qoldirish xavfi bor.
	CourierBusy func(ctx context.Context, courierID string) (bool, error)
	// Revoked — nil bo'lsa sessiyalar bekor qilinmaydi (faqat testlar).
	Revoked *revoke.Store
	NewID   func() string
}

// courierIDFor — xodimning kuryer yozuvi ID'si (barqaror, qayta
// ishlatiladi). Xodim ID'si tasodifiy va global noyob.
func courierIDFor(m *Member) string { return "staff-" + m.ID }

// appRole — lavozimning ilova roli va akkaunt `EntityID` si.
func appRole(m *Member) (users.Role, string, bool) {
	switch m.Position {
	case PositionWaiter:
		return users.RoleWaiter, m.RestaurantID, true
	case PositionCourier:
		return users.RoleCourier, courierIDFor(m), true
	}
	return "", "", false
}

func (a *UserAccounts) Enable(ctx context.Context, m *Member) (string, error) {
	role, entity, ok := appRole(m)
	if !ok {
		return "", ErrAccessNotAllowed
	}
	if role == users.RoleCourier && a.Couriers == nil {
		return "", ErrAccountsDisabled
	}

	if m.UserID != "" {
		u, err := a.Users.GetByID(ctx, m.UserID)
		switch {
		case err == nil:
			if u.Role == role && u.EntityID == entity {
				return u.ID, a.activate(ctx, m, role)
			}
			if u.Role == users.RoleCustomer && u.EntityID == "" && u.Phone == m.Phone {
				if err := a.prepare(ctx, m, role); err != nil {
					return "", err
				}
				if err := a.Users.UpdateRole(ctx, u.ID, role, entity); err != nil {
					return "", err
				}
				return u.ID, a.activate(ctx, m, role)
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
		if u.Role == role && u.EntityID == entity {
			return u.ID, a.activate(ctx, m, role)
		}
		return "", ErrAccountConflict
	case !errors.Is(err, users.ErrUserNotFound):
		return "", err
	}

	if err := a.prepare(ctx, m, role); err != nil {
		return "", err
	}
	account := users.User{
		ID: a.NewID(), Phone: m.Phone, Name: m.FullName(),
		Role: role, EntityID: entity, CreatedAt: time.Now(),
	}
	if err := a.Users.Create(ctx, &account); err != nil {
		if errors.Is(err, users.ErrPhoneTaken) {
			return "", ErrAccountConflict
		}
		return "", err
	}
	return account.ID, a.activate(ctx, m, role)
}

// prepare — kuryer yozuvini TASDIQSIZ holda tayyorlaydi (yo'q bo'lsa
// yaratadi, bor bo'lsa ismini yangilaydi). Ofitsiant uchun hech narsa.
func (a *UserAccounts) prepare(ctx context.Context, m *Member, role users.Role) error {
	if role != users.RoleCourier {
		return nil
	}
	id := courierIDFor(m)
	c, err := a.Couriers.GetByID(ctx, id)
	switch {
	case err == nil:
		// ID xodimdan olinadi, ya'ni boshqa restoranniki bo'lishi
		// mumkin emas — lekin tekshiruv baribir shu yerda turadi.
		if c.RestaurantID != m.RestaurantID {
			return ErrAccountConflict
		}
		if c.Name != m.FullName() {
			return a.Couriers.SetName(ctx, id, m.FullName())
		}
		return nil
	case !errors.Is(err, couriers.ErrNoCourier):
		return err
	}
	return a.Couriers.Create(ctx, &couriers.Courier{
		ID: id, Name: m.FullName(), RestaurantID: m.RestaurantID,
		VehicleType: couriers.VehicleMoped, Rating: 5.0,
		Approved: false, Available: false,
	})
}

// activate — akkaunt bog'langach kuryerni dispatch havuziga qo'yadi.
// `Available` ga TEGILMAYDI: onlayn bo'lishni kuryer o'zi tanlaydi.
func (a *UserAccounts) activate(ctx context.Context, m *Member, role users.Role) error {
	if role != users.RoleCourier {
		return nil
	}
	if err := a.prepare(ctx, m, role); err != nil {
		return err
	}
	return a.Couriers.SetApproved(ctx, courierIDFor(m), true)
}

// Disable — akkaunt o'chirilmaydi (odam oddiy mijoz sifatida qoladi),
// rol `customer` ga qaytariladi va sessiyalar DARHOL bekor qilinadi:
// `auth()` faqat imzoni tekshiradi, eski tokendagi rol busiz 30 kun
// ishlayverardi.
//
// Kuryer: avval tasdiq olib tashlanadi (yangi taklif ham, yangi
// biriktirish ham `ClaimIfAvailable` da to'xtaydi), keyin faol yetkazma
// tekshiriladi. Tartib ATAYLAB shunday: teskarisida tekshiruv va tasdiqni
// olish orasida kuryer buyurtmani olib ulgurishi mumkin edi.
func (a *UserAccounts) Disable(ctx context.Context, restaurantID, userID string) error {
	u, err := a.Users.GetByID(ctx, userID)
	if errors.Is(err, users.ErrUserNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	switch u.Role {
	case users.RoleWaiter:
		if u.EntityID != restaurantID {
			return nil
		}
	case users.RoleCourier:
		if a.Couriers == nil || a.CourierBusy == nil {
			return ErrAccountsDisabled
		}
		c, err := a.Couriers.GetByID(ctx, u.EntityID)
		if errors.Is(err, couriers.ErrNoCourier) {
			return nil
		}
		if err != nil {
			return err
		}
		// Faqat SHU restoranning kuryeri — platforma kuryeri yoki boshqa
		// restoran xodimiga tegilmaydi.
		if c.RestaurantID == "" || c.RestaurantID != restaurantID {
			return nil
		}
		if err := a.Couriers.SetApproved(ctx, c.ID, false); err != nil {
			return err
		}
		busy, err := a.CourierBusy(ctx, c.ID)
		if err == nil && busy {
			err = ErrCourierBusy
		}
		if err != nil {
			// Yopilmaydi — tasdiq qaytariladi, aks holda yetkazma
			// o'rtasidagi kuryer keyingi buyurtmani ololmay qolardi.
			if rerr := a.Couriers.SetApproved(context.WithoutCancel(ctx), c.ID, true); rerr != nil {
				slog.Error("xodim: kuryer tasdig'ini qaytarib bo'lmadi", "courier", c.ID, "err", rerr)
			}
			return err
		}
		if err := a.Couriers.SetAvailable(ctx, c.ID, false); err != nil {
			return err
		}
	default:
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

// Refresh — bog'langan akkaunt ma'lumotini xodim yozuviga moslaydi
// (hozircha kuryer ismi: ilova profili va superadmin ro'yxati uchun).
func (a *UserAccounts) Refresh(ctx context.Context, m *Member) error {
	if m.Position != PositionCourier || a.Couriers == nil {
		return nil
	}
	err := a.Couriers.SetName(ctx, courierIDFor(m), m.FullName())
	if errors.Is(err, couriers.ErrNoCourier) {
		return nil
	}
	return err
}
