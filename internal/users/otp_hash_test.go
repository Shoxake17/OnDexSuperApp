package users

import (
	"strings"
	"testing"
	"time"
)

// OTP kodini hashlash (bug.md 48-band).
//
// ┌─ NEGA MUHIM ───────────────────────────────────────────────────────┐
// Avval bu TUZSIZ SHA-256 edi. Kod maydoni — 10⁶, ya'ni bir million
// qiymatning oldindan hisoblangan jadvali (bir necha soniyalik ish)
// BARCHA hashlarni bir zumda ochardi: kodlar omborini o'qigan odam
// istalgan raqamning kodini bilardi.
// └────────────────────────────────────────────────────────────────────┘

func otpTestService(secret string) *Service {
	return NewService(nil, nil, nil,
		NewTokenIssuer(secret, time.Hour), func() string { return "id" })
}

// ASOSIY REGRESSIYA: hash TUZSIZ SHA-256 BO'LMASLIGI kerak.
//
// Oddiy SHA-256 da natija faqat kodga bog'liq bo'lardi — ya'ni bir xil
// kod har doim bir xil hash beradi va bitta jadval hammasiga yaraydi.
func TestHashCodeIsSaltedByTarget(t *testing.T) {
	s := otpTestService("test-secret")

	a := s.hashCode("+998900000001", "123456")
	b := s.hashCode("+998900000002", "123456")
	if a == b {
		t.Fatal("bir xil kod ikki xil raqam uchun BIR XIL hash berdi — " +
			"tuz yo'q, bitta jadval hammasini ochadi")
	}
}

// Pepper: server sirini bilmagan odam hashni qayta hisoblay olmaydi.
// Ya'ni bazani o'qishning O'ZI yetarli emas.
func TestHashCodeDependsOnServerSecret(t *testing.T) {
	a := otpTestService("birinchi-sir").hashCode("+998900000001", "123456")
	b := otpTestService("ikkinchi-sir").hashCode("+998900000001", "123456")
	if a == b {
		t.Fatal("hash server sirisiz hisoblanadi — bazani o'qish yetarli bo'lardi")
	}
}

// Pepper JWT sirining O'ZI bo'lmasligi kerak: bitta sir ikki maqsadda
// ishlatilsa, birining oqishi ikkinchisini ham ochadi.
func TestCodePepperIsDerivedNotRawSecret(t *testing.T) {
	const secret = "test-secret"
	pepper := NewTokenIssuer(secret, time.Hour).codePepper()
	if string(pepper) == secret {
		t.Fatal("pepper — JWT sirining O'ZI")
	}
	if strings.Contains(string(pepper), secret) {
		t.Fatal("pepper ichida JWT siri ochiq turibdi")
	}
}

// Ajratgich: `target` va `code` chegarasi aniq bo'lishi kerak, aks
// holda ("a"+"bc") va ("ab"+"c") bir xil hash berardi.
func TestHashCodeSeparatesTargetFromCode(t *testing.T) {
	s := otpTestService("test-secret")
	if s.hashCode("+99890000000", "1123456") == s.hashCode("+998900000001", "123456") {
		t.Fatal("target va code chegarasi yo'q — ajratgich ishlamayapti")
	}
}

// Bir xil kirish har doim bir xil natija berishi kerak (aks holda
// tasdiqlash umuman ishlamasdi).
func TestHashCodeIsDeterministic(t *testing.T) {
	s := otpTestService("test-secret")
	a, b := s.hashCode("+998900000001", "123456"), s.hashCode("+998900000001", "123456")
	if a != b {
		t.Fatal("hash deterministik emas")
	}
}

// `sameCode` bo'sh joylarni kesadi va faqat TO'G'RI kodni qabul
// qiladi.
func TestSameCode(t *testing.T) {
	s := otpTestService("test-secret")
	const target = "+998900000001"
	stored := s.hashCode(target, "123456")

	if !s.sameCode(target, " 123456 ", stored) {
		t.Error("bo'sh joyli to'g'ri kod rad etildi")
	}
	if s.sameCode(target, "654321", stored) {
		t.Error("noto'g'ri kod qabul qilindi")
	}
	// BOSHQA nishonning kodi — hash mos kelmasligi kerak.
	if s.sameCode("+998900000002", "123456", stored) {
		t.Error("kod boshqa raqam uchun ham ishladi")
	}
}
