package users

import (
	"strings"
	"testing"
)

// Manzil maydonlarining uzunlik chegarasi (bug.md 26-band).
//
// ┌─ NEGA MUHIM ───────────────────────────────────────────────────────┐
// Chegara UMUMAN yo'q edi: yagona to'siq — umumiy 1 MB tana chegarasi.
// Ya'ni foydalanuvchi profiliga ~1 MB matn saqlashi mumkin edi va u
// keyin restoran paneliga, kuryer ilovasiga va CHEKKA chiqardi.
// └────────────────────────────────────────────────────────────────────┘

func TestAddressValidateRejectsOverlongFields(t *testing.T) {
	cases := []struct {
		name string
		addr AddressDetails
	}{
		{"text", AddressDetails{Text: strings.Repeat("a", MaxAddressTextLength+1)}},
		{"entrance", AddressDetails{Entrance: strings.Repeat("a", MaxAddressPartLength+1)}},
		{"floor", AddressDetails{Floor: strings.Repeat("a", MaxAddressPartLength+1)}},
		{"apartment", AddressDetails{Apartment: strings.Repeat("a", MaxAddressPartLength+1)}},
		{"intercom", AddressDetails{Intercom: strings.Repeat("a", MaxAddressPartLength+1)}},
		{"comment", AddressDetails{Comment: strings.Repeat("a", MaxAddressCommentLength+1)}},
		// Auditda tasvirlangan HAQIQIY holat: ~1 MB matn.
		{"1 MB matn", AddressDetails{Comment: strings.Repeat("x", 1<<20)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := tc.addr
			if err := a.Validate(); err == nil {
				t.Fatalf("%s maydoni chegarasiz o'tdi", tc.name)
			}
		})
	}
}

// Chegaradagi qiymat qabul qilinishi kerak (bir belgi kam emas).
func TestAddressValidateAcceptsLimit(t *testing.T) {
	a := AddressDetails{
		Text:      strings.Repeat("a", MaxAddressTextLength),
		Entrance:  strings.Repeat("a", MaxAddressPartLength),
		Floor:     strings.Repeat("a", MaxAddressPartLength),
		Apartment: strings.Repeat("a", MaxAddressPartLength),
		Intercom:  strings.Repeat("a", MaxAddressPartLength),
		Comment:   strings.Repeat("a", MaxAddressCommentLength),
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("aynan chegaradagi qiymat rad etildi: %v", err)
	}
}

// Uzunlik BELGILAR bo'yicha o'lchanishi kerak, baytlar bo'yicha emas:
// o'zbekcha/kirill harflar bir necha bayt egallaydi va bayt chegarasi
// haqiqiy manzilni asossiz kesib qo'yardi.
func TestAddressValidateCountsRunesNotBytes(t *testing.T) {
	// Har bir "ў" — 2 bayt. Belgilar soni chegarada, baytlar ikki
	// barobar ko'p.
	a := AddressDetails{Text: strings.Repeat("ў", MaxAddressTextLength)}
	if err := a.Validate(); err != nil {
		t.Fatalf("ko'p baytli matn asossiz rad etildi: %v", err)
	}
	if len(a.Text) <= MaxAddressTextLength {
		t.Fatal("test ma'lumoti noto'g'ri: bayt soni chegaradan oshmadi")
	}
}

// Bo'sh joylar kesiladi va bu SAQLANADIGAN qiymatga qo'llanadi
// (ko'rsatkich orqali) — chaqiruvchi boshqa nusxani saqlab
// qo'ymasin.
func TestAddressValidateTrimsInPlace(t *testing.T) {
	a := AddressDetails{Text: "   Mirobod ko'chasi, 41  ", Comment: "\t izoh \n"}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	if a.Text != "Mirobod ko'chasi, 41" {
		t.Fatalf("matn kesilmadi: %q", a.Text)
	}
	if a.Comment != "izoh" {
		t.Fatalf("izoh kesilmadi: %q", a.Comment)
	}
}

// Faqat probeldan iborat, lekin JUDA uzun qiymat — kesilgandan keyin
// bo'sh bo'ladi, ya'ni rad etilmaydi.
func TestAddressValidateWhitespaceOnlyBecomesEmpty(t *testing.T) {
	a := AddressDetails{Comment: strings.Repeat(" ", MaxAddressCommentLength+100)}
	if err := a.Validate(); err != nil {
		t.Fatalf("faqat probel rad etildi: %v", err)
	}
	if a.Comment != "" {
		t.Fatalf("probellar tozalanmadi: %q", a.Comment)
	}
}

// Bo'sh manzil — hech qanday xato bermasligi kerak (koordinata
// tekshiruvi bu funksiyaning ishi emas).
func TestAddressValidateAcceptsEmpty(t *testing.T) {
	var a AddressDetails
	if err := a.Validate(); err != nil {
		t.Fatalf("bo'sh manzil rad etildi: %v", err)
	}
}
