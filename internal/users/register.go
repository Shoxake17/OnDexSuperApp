package users

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Ro'yxatdan o'tish va parol bilan kirish (image/register.png).
//
// MUHIM KONTEKST: platformaning ASOSIY kirish yo'li telefon + SMS kod
// bo'lib qoladi. Parol dizayn talabiga ko'ra QO'SHIMCHA yo'l sifatida
// qo'shildi. Shu sabab bu faylda parolning mavjudligidan kelib
// chiqadigan har bir xavfga qarshi aniq chora bor va har biri izohlangan.

const (
	MaxNameLength  = 50
	MaxEmailLength = 254 // RFC 5321 chegarasi
)

// emailRe — ATAYLAB sodda. To'liq RFC 5322 regexi amalda foydasiz
// (u deyarli hamma narsani o'tkazadi) — haqiqiy tekshiruv email'ga
// tasdiqlash xati yuborish orqali bo'ladi. Bu yerda faqat aniq
// noto'g'ri kiritmalar to'siladi.
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s.]+(\.[^@\s.]+)+$`)

// RegisterInput — ro'yxatdan o'tish formasidan kelgan ma'lumot.
type RegisterInput struct {
	Phone           string
	Email           string // ixtiyoriy
	FirstName       string
	LastName        string
	Password        string
	PasswordConfirm string
}

// ValidatedRegister — `ValidateRegisterInput` natijasi: tozalangan,
// tekshirilgan qiymatlar. Handler shu bosqichni tezlik cheklovidan
// OLDIN bajaradi (pastdagi izohga qarang).
type ValidatedRegister struct {
	Phone     string // normallashtirilgan; email rejimida bo'sh
	Email     string // kichik harfda; ixtiyoriy
	FirstName string
	LastName  string
	Password  string
	ByPhone   bool
}

// ValidateRegisterInput — ARZON tekshiruvlar (format, uzunlik, parol
// siyosati). Argon2 hash QILMAYDI va bazaga TEGMAYDI.
//
// NEGA ALOHIDA FUNKSIYA: tezlik cheklovi (`otpIPLimiter`) handlerning
// eng boshida turadi va IP bo'yicha bor-yo'g'i 5 ta portlashga ruxsat
// beradi (tiklanish ~5 daqiqada bitta). Agar cheklov validatsiyadan
// OLDIN sarflansa, "parollar mos kelmadi" kabi eng oddiy forma xatosi
// ham bitta tokenni yoqadi va formani besh marta xato to'ldirgan
// oddiy odam ~25 daqiqaga bloklanadi. Bu — `loginAccountLimiter` da
// allaqachon bir marta tuzatilgan xatoning aynan o'zi.
//
// Shu sabab handler tartibi: dekodlash -> SHU tekshiruv -> tezlik
// cheklovi -> Argon2 + baza + SMS. Ya'ni forma xatosi BEPUL, haqiqiy
// ish esa har doim cheklangan (Argon2 64 MB ni cheklovsiz qoldirish
// o'zi DoS vositasi bo'lardi).
func ValidateRegisterInput(in RegisterInput) (ValidatedRegister, error) {
	var v ValidatedRegister
	v.ByPhone = strings.TrimSpace(in.Phone) != ""
	if v.ByPhone {
		phone, err := NormalizePhone(in.Phone)
		if err != nil {
			return v, err
		}
		v.Phone = phone
	} else if strings.TrimSpace(in.Email) == "" {
		return v, invalidInput("telefon raqami yoki email kiriting")
	}

	v.FirstName = strings.TrimSpace(in.FirstName)
	v.LastName = strings.TrimSpace(in.LastName)
	if v.FirstName == "" {
		return v, invalidInput("ismni kiriting")
	}
	if utf8.RuneCountInString(v.FirstName) > MaxNameLength ||
		utf8.RuneCountInString(v.LastName) > MaxNameLength {
		return v, invalidInput("ism yoki familiya juda uzun")
	}

	v.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if v.Email != "" {
		if len(v.Email) > MaxEmailLength || !emailRe.MatchString(v.Email) {
			return v, ErrInvalidEmail
		}
	}
	// Parol tasdig'i AVVAL — foydalanuvchiga eng tushunarli xato
	// birinchi ko'rsatilsin.
	if in.Password != in.PasswordConfirm {
		return v, ErrPasswordMismatch
	}
	if err := ValidatePassword(in.Password); err != nil {
		return v, err
	}
	v.Password = in.Password
	return v, nil
}

// Register — yangi akkaunt yaratadi va telefonga SMS kod yuboradi.
//
// TOKEN QAYTARMAYDI. Akkaunt `phone_verified = false` holatida
// yaratiladi va kirish tokeni FAQAT `Verify` (SMS kod) muvaffaqiyatli
// bo'lgandan keyin beriladi.
//
// ┌─ XAVFSIZLIK: PAROL BU YERDA SAQLANMAYDI (telefon rejimi) ─────────┐
//
// Avval parol hash'i tasdiqlanmagan yozuvga darhol yozilardi va bu
// TO'LIQ AKKAUNT EGALLASHGA olib kelardi (jonli isbotlangan):
//
//  1. hujumchi qurbonning raqami bilan register qiladi, O'Z parolini
//     qo'yadi -> tasdiqlanmagan yozuv hujumchining hash'i bilan;
//     SMS esa QURBONNING telefoniga boradi;
//  2. qurbon kodni kiritadi (o'zi kirmoqchi deb o'ylaydi) -> `Verify`
//     yozuvni tasdiqlangan qiladi, lekin hash'ga TEGMAYDI;
//  3. hujumchi o'z paroli bilan kiradi -> 200.
//
// Ildiz sabab: SMS "bu raqam meniki" degan dalil, lekin "bu paroldan
// men xabardorman" degan dalil EMAS — kod ikkalasini bir deb qabul
// qilardi. Poyga ham shart emas edi: qurbon umuman ro'yxatdan
// o'tmagan bo'lsa ham, oddiy OTP bilan kirganda hujumchi oldindan
// yaratib qo'ygan yozuvni o'zi "tasdiqlab" berardi.
//
// YECHIM: telefon rejimida parol UMUMAN yozilmaydi. U faqat
// tasdiqdan keyin, `POST /me/password` orqali o'rnatiladi — ya'ni
// parolni faqat AMALDAGI TOKEN egasi (demak raqamni tasdiqlagan
// odam) qo'ya oladi. Ilova buni tasdiqdan keyin avtomatik bajaradi,
// foydalanuvchi uchun oqim o'zgarmaydi.
//
// └───────────────────────────────────────────────────────────────────┘
//
// ENUMERATION: band raqam/email uchun ALOHIDA xato QAYTARILMAYDI —
// javob har doim bir xil. Aks holda `/auth/register` "bu raqam
// ro'yxatda bormi?" degan savolga bepul javob beruvchi vositaga
// aylanardi va `LoginWithPassword` dagi puxta himoyani teshib
// qo'yardi. Band raqamga kod baribir yuboriladi — ya'ni oqim
// egasining o'zi uchun oddiy KIRISHGA aylanadi.
func (s *Service) Register(ctx context.Context, in RegisterInput) (devCode string, err error) {
	v, err := ValidateRegisterInput(in)
	if err != nil {
		return "", err
	}

	if !v.ByPhone {
		// EMAIL rejimi — telefon oqimi bilan BIR XIL qoidalar:
		// akkaunt TASDIQLANMAGAN holatda yaratiladi, PAROL SAQLANMAYDI
		// va emailga bir martalik kod yuboriladi. Parol faqat kod
		// tasdiqlangandan keyin, `POST /me/password` orqali qo'yiladi.
		//
		// Avval bu yo'l umuman tasdiqlanmasdi va parol darhol
		// saqlanardi — ya'ni begona odam sizning emailingiz bilan
		// akkaunt ochib, o'z parolini qo'yib qo'ya olardi.
		if !s.emailConfigured {
			return "", ErrEmailSendUnavailable
		}
		existing, err := s.users.GetByEmail(ctx, v.Email)
		if err == nil && !emailIsIdentity(existing) {
			// Manzil BEGONA yozuvga (telefon bilan ochilgan akkauntga)
			// tasdiqlanmagan holda yopishtirilgan — u KIMLIK EMAS
			// (`emailIsIdentity` izohidagi hujum). Bo'shatamiz, shunda
			// manzilning haqiqiy egasi o'z akkauntini ocha oladi;
			// pastda hammasi "manzil bo'sh edi" holatidek davom etadi.
			if rerr := s.releaseUnprovenEmail(ctx, existing); rerr != nil {
				return "", rerr
			}
			existing, err = nil, ErrUserNotFound
		}
		switch {
		case err == nil && existing.EmailVerified:
			// Band va TASDIQLANGAN — profilga tegmaymiz va xato ham
			// bermaymiz (enumeration). Pastda kod yuboriladi, ya'ni
			// oqim egasi uchun oddiy kirishga aylanadi.
		case err == nil:
			// Tugallanmagan urinish — qayta ishlatamiz. PAROL YOZILMAYDI.
			if err := s.users.UpdateProfile(ctx, existing.ID, ProfileUpdate{
				FirstName: &v.FirstName, LastName: &v.LastName,
			}); err != nil {
				return "", err
			}
		case errors.Is(err, ErrUserNotFound):
			u := &User{
				ID: s.idgen(), Role: RoleCustomer,
				FirstName: v.FirstName, LastName: v.LastName,
				Name:  strings.TrimSpace(v.FirstName + " " + v.LastName),
				Email: v.Email,
				// PasswordHash ATAYLAB BO'SH.
				// Telefon yo'q — `PhoneVerified` tekshiruvi kirishda
				// o'tkazib yuboriladi (LoginWithPassword'ga qarang).
				PhoneVerified: false,
				EmailVerified: false,
				CreatedAt:     s.now(),
			}
			if err := s.users.Create(ctx, u); err != nil {
				// Poyga (TOCTOU): ikki so'rov bir vaqtda tekshiruvdan
				// o'tishi mumkin, bazadagi unikal indeks ikkinchisini
				// rad etadi. Bu ham enumeration signali bo'lmasligi kerak.
				if errors.Is(err, ErrEmailTaken) {
					return "", nil
				}
				return "", err
			}
		default:
			return "", err
		}
		_, code, err := s.RequestEmailCode(ctx, v.Email)
		if err != nil {
			// Cooldown ham enumeration signali bo'lmasligi kerak.
			if errors.Is(err, ErrTooSoon) {
				return "", nil
			}
			return "", err
		}
		return code, nil
	}

	existing, err := s.users.GetByPhone(ctx, v.Phone)
	switch {
	case err == nil && existing.PhoneVerified:
		// Raqam TASDIQLANGAN egaga tegishli. Yangi akkaunt yaratmaymiz
		// va profilga TEGMAYMIZ, lekin xato ham qaytarmaymiz — pastda
		// kod yuboriladi va oqim egasi uchun oddiy kirishga aylanadi.
	case err == nil:
		// Tasdiqlanmagan yozuv bor (avval ro'yxatdan o'tish boshlangan,
		// lekin SMS kod kiritilmagan). Uni QAYTA ISHLATAMIZ, aks holda
		// tugallanmagan urinish raqamni abadiy band qilib qo'yardi.
		//
		// PAROL YOZILMAYDI (yuqoridagi izoh). Email faqat kiritilgan
		// bo'lsa yangilanadi — aks holda telefon bilan qayta urinish
		// avvalgi urinishda kiritilgan emailni o'chirib yuborardi.
		upd := ProfileUpdate{FirstName: &v.FirstName, LastName: &v.LastName}
		if v.Email != "" {
			upd.Email = &v.Email
		}
		if err := s.users.UpdateProfile(ctx, existing.ID, upd); err != nil {
			// Manzil BOSHQA yozuvda band — jimgina tashlab yuboramiz
			// (pastdagi ENUMERATION izohi).
			if !errors.Is(err, ErrEmailTaken) {
				return "", err
			}
			upd.Email = nil
			if err := s.users.UpdateProfile(ctx, existing.ID, upd); err != nil {
				return "", err
			}
		}
	case errors.Is(err, ErrUserNotFound):
		u := &User{
			ID: s.idgen(), Phone: v.Phone, Role: RoleCustomer,
			FirstName: v.FirstName, LastName: v.LastName,
			Name:  strings.TrimSpace(v.FirstName + " " + v.LastName),
			Email: v.Email,
			// PasswordHash ATAYLAB BO'SH — yuqoridagi izohga qarang.
			// TASDIQLANMAGAN — token faqat SMS kod tasdiqlangach beriladi.
			PhoneVerified: false,
			CreatedAt:     s.now(),
		}
		if err := s.users.Create(ctx, u); err != nil {
			// ┌─ ENUMERATION ─────────────────────────────────────────┐
			// Email BOSHQA akkauntda band bo'lsa `ErrEmailTaken`
			// qaytardi va u mijozga "bu email allaqachon ro'yxatdan
			// o'tgan" bo'lib borardi. Ya'ni TELEFON bilan ro'yxatdan
			// o'tish formasi istalgan manzil uchun "ro'yxatda bormi?"
			// degan savolga bepul javob beradigan vositaga aylanardi —
			// `/auth/login` va email rejimida puxta yopilgan
			// teshikning yon eshigi.
			//
			// Bu yerda email IXTIYORIY profil ma'lumoti, kimlik EMAS
			// (`emailIsIdentity`). Shu sabab band bo'lsa jimgina
			// TASHLAB YUBORILADI va ro'yxatdan o'tish telefon bo'yicha
			// odatdagidek davom etadi: SMS baribir raqam egasiga
			// boradi, hech kimning ma'lumoti ochilmaydi.
			// └───────────────────────────────────────────────────────┘
			if !errors.Is(err, ErrEmailTaken) {
				return "", err
			}
			u.Email = ""
			if err := s.users.Create(ctx, u); err != nil {
				return "", err
			}
		}
	default:
		return "", err
	}

	// Kod yuborish — mavjud `RequestCode` mantig'i qayta ishlatiladi
	// (cooldown, urinishlar chegarasi, hash bilan saqlash hammasi shu
	// yerda). Ikkinchi nusxa yozilmaydi.
	_, code, err := s.RequestCode(ctx, v.Phone)
	if err != nil {
		// Cooldown ham enumeration signali bo'lmasligi kerak: "juda
		// tez" javobi raqamga yaqinda kod yuborilganini oshkor qiladi.
		if errors.Is(err, ErrTooSoon) {
			return "", nil
		}
		return "", err
	}
	return code, nil
}

// LoginWithPassword — telefon YOKI email + parol.
//
// XAVFSIZLIK CHORALARI:
//   - Har qanday muvaffaqiyatsizlikda BIR XIL xato (`ErrInvalidCredentials`) —
//     "bunday foydalanuvchi yo'q" va "parol noto'g'ri" farqlanmaydi,
//     aks holda hujumchi ro'yxatdagi raqamlarni aniqlab olardi.
//   - Akkaunt topilmasa ham parol tekshirish ishi BAJARILADI
//     (`VerifyAgainstDummy`) — javob vaqti bo'yicha ham farq qolmasin.
//   - Tezlik cheklovi HTTP qatlamida (routes_auth.go) — bu funksiya
//     brute-force'dan o'zi himoyalanmaydi.
func (s *Service) LoginWithPassword(ctx context.Context, login, password string) (string, *User, error) {
	login = strings.TrimSpace(login)
	if login == "" || password == "" {
		return "", nil, ErrInvalidCredentials
	}

	var u *User
	var err error
	if strings.Contains(login, "@") {
		u, err = s.users.GetByEmail(ctx, login)
	} else {
		var phone string
		if phone, err = NormalizePhone(login); err == nil {
			u, err = s.users.GetByPhone(ctx, phone)
		}
	}
	if err != nil || u == nil {
		VerifyAgainstDummy(ctx, password) // vaqtni tenglashtirish
		return "", nil, ErrInvalidCredentials
	}
	if u.PasswordHash == "" {
		// Parol o'rnatilmagan akkaunt (eski foydalanuvchi yoki faqat
		// SMS bilan kirgan). Bu ham BIR XIL xato bilan qaytariladi —
		// "bu akkauntda parol yo'q" degan xabar akkauntning MAVJUDLIGINI
		// tasdiqlab qo'yardi.
		VerifyAgainstDummy(ctx, password)
		return "", nil, ErrInvalidCredentials
	}
	ok, err := VerifyPassword(ctx, password, u.PasswordHash)
	if err != nil || !ok {
		return "", nil, ErrInvalidCredentials
	}
	// Hash eski parametrlar bilan yaratilgan bo'lsa — SHU YERDA
	// kuchaytiramiz: ochiq parol butun tizimda faqat mana shu lahzada
	// mavjud. Xatosi kirishni to'xtatmaydi (foydalanuvchi aybdor emas),
	// faqat log qilinadi va keyingi kirishda qayta uriniladi.
	if NeedsRehash(u.PasswordHash) {
		if nh, herr := HashPassword(ctx, password); herr == nil {
			if uerr := s.users.SetPasswordHash(ctx, u.ID, nh); uerr != nil {
				slog.Warn("parol hash'ini yangilab bo'lmadi", "user", u.ID, "err", uerr)
			}
		}
	}
	// ┌─ O'CHIRILGAN AKKAUNT AVTOMATIK TIKLANMAYDI ────────────────────┐
	// Parol ESKI dalil — o'g'irlangan yoki eslab qolingan parol
	// akkauntni egasining xabarisiz tiklamasligi kerak
	// (`reactivateIfDeleted` izohiga qarang). ENUMERATION XAVFI YO'Q:
	// bu yerga faqat parol TO'G'RI kelgandagina yetib kelinadi.
	// └───────────────────────────────────────────────────────────────┘
	if u.IsDeleted() {
		return "", nil, ErrAccountDeleted
	}
	// Telefon TASDIQLANISHI faqat telefonli akkauntlar uchun talab
	// qilinadi. Email bilan ro'yxatdan o'tgan akkauntda telefon
	// umuman yo'q, shuning uchun bu tekshiruv ularni bloklamasligi
	// kerak edi.
	if u.Phone != "" && !u.PhoneVerified {
		// Parol to'g'ri, lekin raqam hali tasdiqlanmagan — bu ANIQ
		// xato bo'lishi mumkin, chunki parolni bilgan odam akkaunt
		// egasining o'zi.
		return "", nil, ErrPhoneNotVerified
	}
	if u.Phone == "" && u.Email != "" && !u.EmailVerified {
		// Email bilan ochilgan akkaunt: telefon yo'q, shuning uchun
		// yagona tasdiq — email kodi. Busiz begona manzilga ochilgan
		// akkaunt parol bilan ishlab ketaverardi.
		return "", nil, ErrEmailNotVerified
	}

	token, err := s.tokens.Issue(u)
	if err != nil {
		return "", nil, err
	}
	return token, u, nil
}

// SetPassword — mavjud foydalanuvchi parolni o'rnatadi/o'zgartiradi.
//
// Chaqiruvchi (HTTP handler) foydalanuvchini ALLAQACHON autentifikatsiya
// qilgan bo'lishi shart. Parol o'zgargach BARCHA eski sessiyalar bekor
// qilinishi kerak — buni handler `revoke` orqali qiladi.
//
// JORIY PAROL: akkauntda parol ALLAQACHON bo'lsa, uni bilish SHART.
// Avval bunday tekshiruv yo'q edi va o'g'irlangan token (masalan
// qulfsiz qolgan telefon) egasiga parolni jimgina almashtirib, keyin
// `Revoke` orqali HAQIQIY egani tizimdan chiqarib yuborish imkonini
// berardi — ya'ni token vaqtincha kirish emas, doimiy egallikka
// aylanardi.
//
// Parol hali YO'Q bo'lsa (SMS bilan kirgan yoki endigina raqamini
// tasdiqlagan foydalanuvchi) joriy parol so'ralmaydi — so'rash mumkin
// ham emas. Aynan shu holat ro'yxatdan o'tish oqimida ishlatiladi:
// parol tasdiqdan KEYIN, token bilan o'rnatiladi (`Register` izohiga
// qarang).
//
// `phoneProven` — chaqiruvchi SMS kod bilan telefon egaligini
// HOZIRGINA isbotlagan (`Claims.HasFreshPhoneProof`). Bunday holatda
// joriy parol so'ralmaydi: parolini UNUTGAN odam uni ayta olmaydi,
// SMS esa egalikni kamida parol darajasida isbotlaydi. Aynan shu
// "Parolni unutdingizmi?" oqimini ishlatadi — alohida tiklash
// havolasi yaratilmaydi, demak o'g'irlanadigan havola ham yo'q.
func (s *Service) SetPassword(ctx context.Context, userID, current, password, confirm string, phoneProven bool) error {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.PasswordHash != "" && !phoneProven {
		ok, err := VerifyPassword(ctx, current, u.PasswordHash)
		if err != nil || !ok {
			return ErrCurrentPasswordWrong
		}
	}
	if password != confirm {
		return ErrPasswordMismatch
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := HashPassword(ctx, password)
	if err != nil {
		return err
	}
	// `UpdateProfile` EMAS — u `name` ni first/last dan qayta hisoblab,
	// admin yaratgan akkauntlarning nomini o'chirib yuborardi
	// (`Repository.SetPasswordHash` izohiga qarang).
	return s.users.SetPasswordHash(ctx, userID, hash)
}

// UpdateName — mijozning ismi/familiyasi.
//
// Bu endpoint AVVAL UMUMAN YO'Q EDI: `users.name` ustuni bor edi-yu,
// mijoz ilovasida uni to'ldirishning hech qanday yo'li yo'q edi va
// profil har doim "Mijoz" deb ko'rsatardi.
// DeleteAccount — mijozning O'ZI "Akkauntni o'chirish"ni bosgan payt
// (`POST /me/delete-account`, faqat RoleCustomer).
//
// MA'LUMOT O'CHIRILMAYDI — faqat kirish yopiladi (`SoftDelete`
// izohiga qarang). HTTP handler bu chaqiruvdan keyin DARHOL
// `Revoke`ni ham bajaradi — token qo'lida qolgan bo'lsa ham keyingi
// so'rov 401 qaytaradi.
//
// JORIY PAROL: `SetPassword` bilan AYNAN bir xil qoida — akkauntda
// parol bo'lsa va token SMS/email kod bilan HOZIRGINA tasdiqlanmagan
// bo'lsa, joriy parol talab qilinadi. Busiz o'g'irlangan/qulfsiz
// qolgan telefon egasining ma'lumotlariga kirishni butunlay yopib
// qo'yishi mumkin edi — parol so'rash bu ehtimolni yopadi, xuddi
// parolni o'zgartirishda bo'lgani kabi.
func (s *Service) DeleteAccount(ctx context.Context, userID, currentPassword string, phoneProven bool) error {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.PasswordHash != "" && !phoneProven {
		ok, err := VerifyPassword(ctx, currentPassword, u.PasswordHash)
		if err != nil || !ok {
			return ErrCurrentPasswordWrong
		}
	}
	return s.users.SoftDelete(ctx, userID)
}

func (s *Service) UpdateName(ctx context.Context, userID, first, last string) error {
	first = strings.TrimSpace(first)
	last = strings.TrimSpace(last)
	if first == "" {
		return invalidInput("ismni kiriting")
	}
	if utf8.RuneCountInString(first) > MaxNameLength || utf8.RuneCountInString(last) > MaxNameLength {
		return invalidInput("ism yoki familiya juda uzun")
	}
	return s.users.UpdateProfile(ctx, userID, ProfileUpdate{FirstName: &first, LastName: &last})
}
