package users

import (
	"context"
	"errors"
	"testing"
)

// Telegram Mini App kimlik bog'lanishi (migration 0031).
//
// ┌─ ASOSIY VA'DA ────────────────────────────────────────────────────┐
// Foydalanuvchi Mini App'da ham, mobil ilovada ham, saytda ham BITTA
// akkauntda bo'lishi kerak. Buni ta'minlaydigan narsa — kimlik
// TELEFON RAQAMI ekani: `initData` faqat Telegram ID beradi, u esa
// botda ulashilgan kontakt orqali raqamga bog'lanadi.
//
// Quyidagi testlar aynan shu zanjirni qo'riqlaydi.
// └───────────────────────────────────────────────────────────────────┘

// ★ ASOSIY: TMA va SMS bir xil akkauntga olib boradi.
func TestTelegramAndSmsGiveSameAccount(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	const phone = "+998901112233"

	// 1. Foydalanuvchi botda kontaktini ulashdi.
	linked, err := s.LinkTelegramPhone(ctx, 555000111, phone)
	if err != nil {
		t.Fatalf("bog'lash: %v", err)
	}

	// 2. Mini App orqali kirdi.
	_, viaTma, err := s.LoginWithTelegramID(ctx, 555000111)
	if err != nil {
		t.Fatalf("Mini App kirishi: %v", err)
	}

	// 3. Keyin mobil ilovada SMS bilan kirdi (Firebase yo'li).
	_, viaPhone, err := s.LoginWithFirebasePhone(ctx, phone)
	if err != nil {
		t.Fatalf("telefon kirishi: %v", err)
	}

	if viaTma.ID != viaPhone.ID {
		t.Fatalf("BOSHQA AKKAUNTLAR: TMA=%s telefon=%s", viaTma.ID, viaPhone.ID)
	}
	if viaTma.ID != linked.ID {
		t.Fatalf("bog'langan akkaunt boshqa: %s != %s", viaTma.ID, linked.ID)
	}
	if viaTma.Phone != phone {
		t.Fatalf("raqam mos emas: %q", viaTma.Phone)
	}
}

// ★ Kontakt ulashilmagan bo'lsa — aniq xato, jimgina "yangi akkaunt" EMAS.
//
// NEGA MUHIM: `initData` da telefon yo'q. Agar bu holatda yangi
// akkaunt yaratilsa, foydalanuvchi Mini App'da BOSHQA hisobda
// bo'lardi va buyurtmalari mobil ilovada ko'rinmasdi.
func TestUnlinkedTelegramIsRejected(t *testing.T) {
	s := newTestService()
	_, _, err := s.LoginWithTelegramID(context.Background(), 777000222)
	if !errors.Is(err, ErrTelegramNotLinked) {
		t.Fatalf("bog'lanmagan Telegram uchun kutilgan xato emas: %v", err)
	}
}

// ★ EGALIK KO'CHISHI: bitta Telegram akkaunti faqat BITTA hisobga.
//
// Odam raqamini o'zgartirsa (yoki raqam boshqa egaga o'tsa), Telegram
// bog'lanishi YANGI hisobga o'tishi kerak. Aks holda u Mini App'da
// eski — endi begona — hisobga tushib qolardi.
func TestTelegramLinkMovesToNewAccount(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	const tgID = 888000333

	first, err := s.LinkTelegramPhone(ctx, tgID, "+998901112233")
	if err != nil {
		t.Fatalf("birinchi bog'lash: %v", err)
	}
	second, err := s.LinkTelegramPhone(ctx, tgID, "+998904445566")
	if err != nil {
		t.Fatalf("ikkinchi bog'lash: %v", err)
	}
	if first.ID == second.ID {
		t.Fatal("ikki xil raqam bitta akkauntga tushdi")
	}

	_, now, err := s.LoginWithTelegramID(ctx, tgID)
	if err != nil {
		t.Fatalf("kirish: %v", err)
	}
	if now.ID != second.ID {
		t.Fatalf("bog'lanish ESKI akkauntda qoldi: %s (kutilgan %s)", now.ID, second.ID)
	}

	// Eski akkaunt o'chmasligi kerak — buyurtmalari o'sha yerda.
	old, err := s.users.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("eski akkaunt yo'qoldi: %v", err)
	}
	if old.TelegramID != 0 {
		t.Fatalf("eski akkauntda bog'lanish qoldi: %d", old.TelegramID)
	}
}

// Mavjud (SMS bilan yaratilgan) akkauntga Telegram bog'lanadi —
// dublikat YARATILMAYDI.
func TestLinkingExistingAccountDoesNotDuplicate(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	const phone = "+998907778899"

	_, viaSms, err := s.LoginWithFirebasePhone(ctx, phone)
	if err != nil {
		t.Fatalf("SMS kirishi: %v", err)
	}
	linked, err := s.LinkTelegramPhone(ctx, 999000444, phone)
	if err != nil {
		t.Fatalf("bog'lash: %v", err)
	}
	if linked.ID != viaSms.ID {
		t.Fatalf("DUBLIKAT akkaunt yaratildi: %s != %s", linked.ID, viaSms.ID)
	}
}

// Telegram ID 0 — yaroqsiz kirish, akkaunt yaratilmaydi.
func TestZeroTelegramIDRejected(t *testing.T) {
	s := newTestService()
	ctx := context.Background()
	if _, err := s.LinkTelegramPhone(ctx, 0, "+998901112233"); err == nil {
		t.Fatal("telegram_id=0 bilan bog'lash o'tdi")
	}
	if _, _, err := s.LoginWithTelegramID(ctx, 0); !errors.Is(err, ErrTelegramNotLinked) {
		t.Fatalf("telegram_id=0 bilan kirish: %v", err)
	}
}

// Raqam formati noto'g'ri bo'lsa bog'lanmaydi.
func TestInvalidPhoneNotLinked(t *testing.T) {
	s := newTestService()
	if _, err := s.LinkTelegramPhone(context.Background(), 111000555, "salom"); err == nil {
		t.Fatal("yaroqsiz raqam bog'landi")
	}
}
