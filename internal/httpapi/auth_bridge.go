package httpapi

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// O'tish davri ko'prigi (A4a, AUTH-PHASE-A.md 4-bo'lim "o'tish davri").
//
// ┌─ NIMA UCHUN ──────────────────────────────────────────────────────────┐
// OnDexPro va OnDexGO markaziy sessiyaga (EdDSA, 10 daqiqa + refresh)
// o'tdi, lekin ular chaqiradigan marshrutlar (`/waiter/*`, `/courier/*`,
// `/me`, `/ws/ticket`) hali eski `s.auth` va `users.Claims` (rol +
// entity_id) bilan ishlaydi. Ko'prik yangi tokenni qabul qiladi va
// claims'ni AKKAUNTNING HOZIRGI holatidan quradi:
//
//   - sessiya markazda faolligi tekshiriladi (`Identity.Authenticate`):
//     markazda yopilgan sessiya bu yerda ham darhol o'ladi;
//   - akkaunt HAR SO'ROVDA o'qiladi: xodimning ilovaga kirishi yopilsa
//     (eski staff moduli rolni mijozga qaytaradi) yoki akkaunt o'chirilsa —
//     keyingi so'rov 401, refresh'ni kutmasdan;
//   - hozircha FAQAT merchant realmi (mijoz realmi keyingi bosqichda).
//
// A5 da eski marshrutlar `authz` ga o'tganda ko'prik olib tashlanadi.
// └───────────────────────────────────────────────────────────────────────┘

var errSessionGone = errors.New("sessiya tugagan — qaytadan kiring")

// parseBearer — eski HS256 yoki yangi EdDSA token. Natija — eski claims.
func (s *Server) parseBearer(ctx context.Context, raw string) (*users.Claims, error) {
	claims, err := s.Tokens.Parse(raw)
	if err == nil {
		return claims, nil
	}
	if s.Identity == nil || !isEdDSA(raw) {
		return nil, err
	}
	c, aerr := s.Identity.Authenticate(ctx, raw, users.RealmMerchant, users.RealmPlatform, users.RealmCustomer)
	if aerr != nil {
		return nil, errSessionGone
	}
	// A6: admin tokeni faqat TOTP (yoki tiklash kodi) bilan ochilgan
	// sessiyadan — ikkinchi omilsiz admin sessiyasi hech qachon o'tmaydi.
	// PK-2: TOTP, tiklash kodi YOKI UV bilan passkey (identity.StrongAMR).
	if c.Realm == users.RealmPlatform && !identity.StrongAMR(c.AMR) {
		return nil, errSessionGone
	}
	u, uerr := s.UserRepo.GetByID(ctx, c.Subject)
	if uerr != nil || u.IsDeleted() || users.RealmForRole(u.Role) != c.Realm {
		return nil, errSessionGone
	}
	// IssuedAt = KIRISH vaqti (auth_time), token yangilangan vaqt emas:
	//   - foydalanuvchi darajasidagi bekor qilish (`Revoked`: parol
	//     almashtirish, bloklash, "hamma joydan chiqish") shu vaqtdan oldin
	//     ochilgan sessiyalarni ham o'ldiradi — refresh uni aylanib o'tolmaydi;
	//   - "kod bilan HOZIRGINA kirdi" (PhoneProven, 15 daqiqa — joriy
	//     parolsiz parol o'rnatish) refresh bilan uzaymaydi.
	iat := c.IssuedAt
	if c.AuthTime > 0 {
		iat = jwt.NewNumericDate(time.Unix(c.AuthTime, 0))
	}
	return &users.Claims{Role: u.Role, EntityID: u.EntityID,
		PhoneProven:      c.Realm == users.RealmCustomer && (slices.Contains(c.AMR, "otp") || slices.Contains(c.AMR, "email")),
		RegisteredClaims: jwt.RegisteredClaims{Subject: u.ID, IssuedAt: iat, ExpiresAt: c.ExpiresAt, ID: c.ID},
	}, nil
}

// isEdDSA — token sarlavhasida `alg: EdDSA` (imzo tekshirilMAYDI — bu faqat
// qaysi tekshiruvchiga berishni tanlash; haqiqiy tekshiruv Authenticate'da).
func isEdDSA(raw string) bool {
	t, _, err := jwt.NewParser().ParseUnverified(raw, jwt.MapClaims{})
	return err == nil && t.Method != nil && t.Method.Alg() == jwt.SigningMethodEdDSA.Alg()
}
