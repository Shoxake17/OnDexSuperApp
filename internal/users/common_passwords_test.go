package users

import (
	"errors"
	"testing"
)

// NIST 800-63B: sizib chiqqan/keng tarqalgan parollar va oddiy naqshlar rad.
func TestValidatePasswordRejectsWeak(t *testing.T) {
	for _, p := range []string{
		"12345678", "PASSWORD", "football", "sunshine1", "iloveyou",
		"toshkent123", "aaaaaaaa", "87654321", "abcdefgh",
		"ondex2026!", "Chust1234", "qwerty12345",
	} {
		if err := ValidatePassword(p); !errors.Is(err, ErrPasswordTooCommon) {
			t.Errorf("%q qabul qilindi (%v)", p, err)
		}
	}
	for _, p := range []string{"Kuchli-parol-1", "Yangi-Parol-2026!", "tog'-osh-ko'k-daryo", "Mijoz-Parol-77"} {
		if err := ValidatePassword(p); err != nil {
			t.Errorf("kuchli parol rad etildi: %q (%v)", p, err)
		}
	}
	commonOnce.Do(loadCommonPasswords)
	if len(commonSet) < 40_000 {
		t.Fatalf("ro'yxat yuklanmadi: %d", len(commonSet))
	}
}
