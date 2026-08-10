package users

import (
	"context"
	"strings"
	"testing"
)

// tctx — testlar uchun qisqa yorliq. Argon2 navbati (`argonGate`)
// kontekst orqali bekor qilinadi; testlarda bekor qilish kerak emas.
func tctx() context.Context { return context.Background() }

func TestHashAndVerifyRoundTrip(t *testing.T) {
	const pw = "juda-yaxshi-parol-2026"
	h, err := HashPassword(tctx(),pw)
	if err != nil {
		t.Fatalf("hash xatosi: %v", err)
	}
	ok, err := VerifyPassword(tctx(),pw, h)
	if err != nil || !ok {
		t.Fatalf("to'g'ri parol qabul qilinishi kerak edi (ok=%v, err=%v)", ok, err)
	}
	ok, err = VerifyPassword(tctx(),pw+"x", h)
	if err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	if ok {
		t.Error("noto'g'ri parol RAD ETILISHI kerak edi")
	}
}

// Har bir hash NOYOB tuz (salt) bilan bo'lishi shart — aks holda bir xil
// parolli ikki foydalanuvchi bazada bir xil hash bilan turadi va
// oldindan hisoblangan jadval (rainbow table) bilan ikkalasi birdan
// ochiladi.
func TestHashIsSaltedUniquely(t *testing.T) {
	a, _ := HashPassword(tctx(),"bir-xil-parol")
	b, _ := HashPassword(tctx(),"bir-xil-parol")
	if a == b {
		t.Error("bir xil parol uchun hash'lar bir xil chiqdi — tuz ishlamayapti")
	}
}

func TestHashFormatIsPHC(t *testing.T) {
	h, _ := HashPassword(tctx(),"tekshiruv-paroli")
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=2$") {
		t.Errorf("kutilmagan format: %s", h)
	}
	if len(strings.Split(h, "$")) != 6 {
		t.Errorf("PHC formatida 6 qism bo'lishi kerak: %s", h)
	}
}

// Buzilgan/soxta hash PANIKA qilmasligi va jimgina `true` qaytarmasligi
// kerak — aks holda bazaga bo'sh satr yozib qo'yish kirishni ochib
// yuborardi.
func TestVerifyRejectsMalformedHash(t *testing.T) {
	bad := []string{
		"", "not-a-hash", "$argon2id$", "$bcrypt$v=19$m=1,t=1,p=1$aaa$bbb",
		"$argon2id$v=19$m=65536,t=3,p=2$$", "$argon2id$v=1$m=1,t=1,p=1$YWJj$YWJj",
		"$argon2id$v=19$m=65536,t=3,p=2$!!!$###",
	}
	for _, h := range bad {
		ok, err := VerifyPassword(tctx(),"istalgan", h)
		if ok {
			t.Errorf("buzilgan hash qabul qilindi: %q", h)
		}
		if err == nil {
			t.Errorf("buzilgan hash uchun xato kutilgan edi: %q", h)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name    string
		pw      string
		wantErr error
	}{
		{"juda qisqa", "1234567", ErrPasswordTooShort},
		{"eng qisqa ruxsat etilgan", "12345678x", nil},
		{"juda uzun", strings.Repeat("a", MaxPasswordLength+1), ErrPasswordTooLong},
		{"ommaviy parol", "password123", ErrPasswordTooCommon},
		{"ommaviy — katta harf bilan ham", "PASSWORD123", ErrPasswordTooCommon},
		{"normal", "mening-kuchli-parolim", nil},
	}
	for _, c := range cases {
		if got := ValidatePassword(c.pw); got != c.wantErr {
			t.Errorf("%s: %v kutilgan, olindi %v", c.name, c.wantErr, got)
		}
	}
}

// Uzunlik BELGILARDA sanaladi, baytlarda emas — kirill/o'zbek harflari
// ko'p baytli va bayt bo'yicha sanash foydalanuvchini adashtiradi.
func TestPasswordLengthCountsRunesNotBytes(t *testing.T) {
	// 8 ta kirill harfi = 16 bayt, lekin 8 belgi — qabul qilinishi kerak.
	if err := ValidatePassword("парольчи"); err != nil {
		t.Errorf("8 belgili kirill parol qabul qilinishi kerak edi: %v", err)
	}
	// 7 ta belgi — rad etilishi kerak (baytda 14 bo'lsa ham).
	if err := ValidatePassword("парольч"); err != ErrPasswordTooShort {
		t.Errorf("7 belgili parol rad etilishi kerak edi, olindi %v", err)
	}
}

// Mavjud bo'lmagan akkaunt uchun ham parol tekshirish ishi bajarilishi
// kerak (user enumeration'ga qarshi) — funksiya panika qilmasligi shart.
func TestVerifyAgainstDummyDoesNotPanic(t *testing.T) {
	VerifyAgainstDummy(tctx(),"istalgan parol")
}
