package httpapi

import (
	"strings"

	"chustapp/internal/users"
)

// loginKey — akkaunt bo'yicha cheklov kaliti (parol bilan kirish).
//
// ┌─ TUZATILGAN ZAIFLIK (SSO auditi, 2026-11-10) ────────────────────────┐
// Kalit avval foydalanuvchi YOZGAN satr edi: "+998901234567",
// "998901234567" va "+998 90 123 45 67" — uchta ALOHIDA hisoblagich. Ko'p
// IP'dan bitta akkauntga parol terish (credential stuffing) yozilish
// shaklini almashtirib cheklovni cheksiz qayta boshlardi. Endi kalit —
// normallashtirilgan telefon/email: bitta akkaunt = bitta hisoblagich.
// └───────────────────────────────────────────────────────────────────────┘
func loginKey(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.Contains(s, "@") {
		if e, err := users.NormalizeEmail(s); err == nil {
			return "e:" + e
		}
		return "e:" + strings.ToLower(s)
	}
	if p, err := users.NormalizePhone(s); err == nil {
		return "p:" + p
	}
	return "x:" + strings.ToLower(s)
}
