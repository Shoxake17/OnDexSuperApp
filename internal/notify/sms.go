package notify

import "log/slog"

// LogSms — dev muhit uchun SMS "yuboruvchi": haqiqiy SMS o'rniga logga yozadi.
//
// ┌─ FAQAT DEV — BU ENDI MAJBURAN QO'LLANADI (bug.md 47-band) ─────────┐
// Bu implementatsiya bir martalik KODNI ochiq matnda logga yozadi.
// Avval `ESKIZ_*` sozlanmagan bo'lsa u PRODUCTION'da ham ishlatilardi
// va server faqat `slog.Warn` bilan davom etardi — ya'ni barcha OTP
// kodlar log faylida ochiq turardi (logga kirish huquqi bo'lgan har
// kim istalgan akkauntni egallay olardi), kirish esa jimgina
// buzilgan bo'lardi.
//
// Endi `cmd/api/main.go` production'da SMS sozlanmagan bo'lsa
// `os.Exit(1)` qiladi — ya'ni bu tur u yerga printsipial jihatdan
// yetib bora olmaydi.
//
// Kodni logda ko'rsatish DEV uchun ataylab saqlandi: lokal sinovda
// SMS provayderi bo'lmaydi va kod aynan shu yerdan olinadi.
// └────────────────────────────────────────────────────────────────────┘
type LogSms struct{}

func (LogSms) Send(phone, text string) error {
	slog.Info("sms (dev rejim, yuborilmadi)", "phone", phone, "text", text)
	return nil
}
