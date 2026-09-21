package users

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
)

// ┌─ TEST AKKAUNTLAR: SOBIT OTP (`TEST_OTP=true`) ────────────────────────┐
// Google Play / App Store sharhlovchisi ilovaga KIRA OLISHI shart
// ("App access"). Bizda kod faqat Telegram bot orqali keladi — sharhlovchi
// uni ololmaydi. Shu uchun har ilovaga ALOHIDA raqam va sobit kod:
//
//	OnDexGO  (kuryer)     +998 88 888 88 88  →  666666
//	OnDexPro (affitsiant) +998 99 999 99 99  →  666666
//
// Nega ikki raqam: bitta raqam = bitta akkaunt = bitta rol. Kuryer va
// affitsiant akkauntini bitta raqamga ochib bo'lmaydi.
//
// XAVFSIZLIK CHEGARALARI (hammasi majburiy):
//
//  1. O'chiq holat — standart. Bayroq `TEST_OTP=true` bo'lmasa bu fayldagi
//     hech narsa ishlamaydi (`WithTestLogin` chaqirilmaydi).
//  2. Har raqam FAQAT O'Z rolini ochadi (888 — kuryer, 999 — affitsiant).
//     "+998 88" va "+998 99" — haqiqiy operator kodlari: raqam tirik
//     odamniki bo'lishi mumkin. Mijoz akkaunti (hamyon, manzil,
//     buyurtmalar), restoran egasi yoki admin sobit kod bilan HECH QACHON
//     ochilmaydi — ular uchun raqam oddiy raqamdek ishlaydi (tasodifiy kod,
//     Telegram orqali).
//  3. Akkaunt YARATILMAYDI. Test akkauntini restoran "Xodimlar" bo'limida
//     ochadi (yetkazib beruvchi yoki ofitsiant) — ya'ni u faqat o'sha
//     restoran doirasidagi huquqqa ega bo'ladi.
//  4. O'chirilgan akkaunt tiklanmaydi.
//  5. Oddiy kod mexanizmi O'ZGARMAYDI: 5 daqiqa muddat, 5 urinish,
//     60 soniyalik pauza, bir martalik ishlatish, IP bo'yicha cheklovlar.
//  6. Rol tekshiruvi ikki marta: kod berilganda VA kiritilganda (oraliqda
//     rol o'zgargan bo'lsa ham sobit kod o'tmaydi).
//  7. Har bir kirish logga yoziladi (akkaunt ID va roli — raqam emas).
//
// └───────────────────────────────────────────────────────────────────────┘

// TestLogin — bitta test raqami va u ochishi mumkin bo'lgan YAGONA rol.
type TestLogin struct {
	Phone string
	Role  Role
}

// TestLogins — `TEST_OTP=true` da yoqiladigan test raqamlari.
var TestLogins = []TestLogin{
	{Phone: "+998888888888", Role: RoleCourier}, // OnDexGO
	{Phone: "+998999999999", Role: RoleWaiter},  // OnDexPro
}

// TestLoginCode — barcha test raqamlari uchun sobit kod.
const TestLoginCode = "666666"

var testCodeRe = regexp.MustCompile(`^\d{6}$`)

// WithTestLogin — sobit kodli test akkauntlarini yoqadi.
func (s *Service) WithTestLogin(logins []TestLogin, code string) (*Service, error) {
	if len(logins) == 0 {
		return nil, errors.New("test raqamlari ro'yxati bo'sh")
	}
	if !testCodeRe.MatchString(code) {
		return nil, errors.New("test kodi 6 xonali raqam bo'lishi kerak")
	}
	roles := make(map[string]Role, len(logins))
	for _, l := range logins {
		p, err := NormalizePhone(l.Phone)
		if err != nil {
			return nil, fmt.Errorf("test raqami %q: %w", l.Phone, err)
		}
		if l.Role != RoleCourier && l.Role != RoleWaiter {
			// Imtiyozli yoki mijoz roliga sobit kod — ataylab imkonsiz.
			return nil, fmt.Errorf("test raqami %s: rol %q ruxsat etilmagan (faqat courier/waiter)", p, l.Role)
		}
		if _, dup := roles[p]; dup {
			return nil, fmt.Errorf("test raqami %s takrorlangan", p)
		}
		roles[p] = l.Role
	}
	s.testRoles, s.testCode = roles, code
	return s, nil
}

// isTestPhone — normallashtirilgan raqam test ro'yxatidami.
func (s *Service) isTestPhone(phone string) bool {
	_, ok := s.testRoles[phone]
	return ok
}

// testAccount — `phone` hozir sobit kod bilan ochiladigan test akkauntimi:
// akkaunt mavjud, o'chirilmagan va roli AYNAN shu raqamga biriktirilgan
// rol. Raqam normallashtirilgan bo'lishi kerak.
func (s *Service) testAccount(ctx context.Context, phone string) bool {
	role, ok := s.testRoles[phone]
	if !ok {
		return false
	}
	u, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			slog.Warn("test akkaunt: foydalanuvchini o'qib bo'lmadi", "err", err)
		}
		return false
	}
	return u.Role == role && !u.IsDeleted()
}

// TestLoginActive — bu raqam uchun kod ILOVAGA TO'G'RIDAN-TO'G'RI
// kiritiladimi (Telegram kerak emasmi). `POST /auth/telegram/start` shu
// bilan sharhlovchini Telegram'ga jo'natmaydi.
func (s *Service) TestLoginActive(ctx context.Context, rawPhone string) bool {
	if len(s.testRoles) == 0 {
		return false
	}
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return false
	}
	return s.testAccount(ctx, phone)
}
