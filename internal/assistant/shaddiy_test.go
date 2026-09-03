package assistant

import (
	"strings"
	"testing"
)

// Tashqi provayderga ICHKI foydalanuvchi ID'si chiqmasligi kerak.
//
// ┌─ NEGA TEST BILAN QULFLANGAN ──────────────────────────────────────┐
// Bu invariantni buzish juda oson: `UserRef: userRef` deb yozib
// qo'yish yetarli va hech qanday test yiqilmasdi, hech qanday xato
// ko'rinmasdi. Oqibati esa jimgina — ichki ID tashqi tizim loglarida
// qolib ketardi va u yerdan bizning buyurtma/grant yozuvlarimizga
// bog'lash mumkin bo'lardi.
// └───────────────────────────────────────────────────────────────────┘
func TestPseudonymHidesInternalUserID(t *testing.T) {
	s := &Shaddiy{apiKey: "shaddiy_live_test_key"}
	const userID = "usr_01HXYZ"

	ref := s.pseudonym(userID)

	if ref == "" {
		t.Fatal("psevdonim bo'sh — provayder tezlik cheklovi ishlamaydi")
	}
	if strings.Contains(ref, userID) {
		t.Fatalf("ICHKI ID psevdonim ichida qoldi: %q", ref)
	}
	// Barqaror bo'lishi shart — aks holda har so'rov yangi
	// foydalanuvchidek ko'rinib, tezlik cheklovi ma'nosiz bo'lardi.
	if again := s.pseudonym(userID); again != ref {
		t.Errorf("psevdonim barqaror emas: %q va %q", ref, again)
	}
	// Turli foydalanuvchi — turli psevdonim.
	if other := s.pseudonym("usr_BOSHQA"); other == ref {
		t.Error("ikki foydalanuvchi bitta psevdonim oldi")
	}
	// Kalit almashsa psevdonim ham almashadi (bog'lashga qarshi).
	other := &Shaddiy{apiKey: "boshqa_kalit"}
	if other.pseudonym(userID) == ref {
		t.Error("boshqa kalit bilan ham bir xil psevdonim chiqdi")
	}
	// Bo'sh ID — bo'sh natija (soxta psevdonim yaratilmaydi).
	if s.pseudonym("") != "" {
		t.Error("bo'sh ID uchun psevdonim yaratildi")
	}
}
