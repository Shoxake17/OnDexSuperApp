package users

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
)

// ┌─ TEST AKKAUNT: SOBIT OTP (`TEST_OTP=true`) ───────────────────────────┐
// Google Play / App Store sharhlovchisi ilovaga KIRA OLISHI shart
// ("App access"). Bizda kod faqat Telegram bot orqali keladi — sharhlovchi
// uni ololmaydi. Shu uchun bitta raqamga SOBIT kod beriladi:
//
//	+998 99 999 99 99  →  666666
//
// XAVFSIZLIK CHEGARALARI (hammasi majburiy):
//
//  1. O'chiq holat — standart. Bayroq `TEST_OTP=true` bo'lmasa bu fayldagi
//     hech narsa ishlamaydi (`WithTestLogin` chaqirilmaydi).
//  2. FAQAT kuryer (OnDexGO) yoki affitsiant (OnDexPro) akkaunti. "+998 99"
//     — haqiqiy operator kodi: raqam tirik odamniki bo'lishi mumkin. Mijoz
//     akkaunti (hamyon, manzil, buyurtmalar), restoran egasi yoki admin
//     sobit kod bilan HECH QACHON ochilmaydi — ular uchun raqam oddiy
//     raqamdek ishlaydi (tasodifiy kod, Telegram orqali).
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
const (
	TestLoginPhone = "+998999999999"
	TestLoginCode  = "666666"
)

var testCodeRe = regexp.MustCompile(`^\d{6}$`)

// WithTestLogin — sobit kodli test akkauntini yoqadi.
func (s *Service) WithTestLogin(phone, code string) (*Service, error) {
	p, err := NormalizePhone(phone)
	if err != nil {
		return nil, err
	}
	if !testCodeRe.MatchString(code) {
		return nil, errors.New("test kodi 6 xonali raqam bo'lishi kerak")
	}
	s.testPhone, s.testCode = p, code
	return s, nil
}

// testLoginRole — sobit kod ochishi MUMKIN bo'lgan rollar.
func testLoginRole(r Role) bool { return r == RoleCourier || r == RoleWaiter }

// testAccount — `phone` hozir sobit kod bilan ochiladigan test akkauntimi.
// Raqam normallashtirilgan bo'lishi kerak.
func (s *Service) testAccount(ctx context.Context, phone string) bool {
	if s.testPhone == "" || phone != s.testPhone {
		return false
	}
	u, err := s.users.GetByPhone(ctx, phone)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			slog.Warn("test akkaunt: foydalanuvchini o'qib bo'lmadi", "err", err)
		}
		return false
	}
	return testLoginRole(u.Role) && !u.IsDeleted()
}

// TestLoginActive — bu raqam uchun kod ILOVAGA TO'G'RIDAN-TO'G'RI
// kiritiladimi (Telegram kerak emasmi). `POST /auth/telegram/start` shu
// bilan sharhlovchini Telegram'ga jo'natmaydi.
func (s *Service) TestLoginActive(ctx context.Context, rawPhone string) bool {
	if s.testPhone == "" {
		return false
	}
	phone, err := NormalizePhone(rawPhone)
	if err != nil {
		return false
	}
	return s.testAccount(ctx, phone)
}
