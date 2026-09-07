package notify

import (
	"errors"
	"log/slog"
)

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

// ErrSmsDisabled — SMS kanali ATAYLAB o'chirilgan. Foydalanuvchiga
// ko'rsatish uchun yaroqli matn: ichki tafsilot oshkor qilmaydi.
var ErrSmsDisabled = errors.New("SMS kanali o'chirilgan — kodni Telegram bot orqali oling")

// DisabledSms — SMS'ni PRODUCTION'da ataylab o'chirish (`OTP_CHANNEL=telegram`).
//
// ┌─ NEGA `LogSms` EMAS ───────────────────────────────────────────────┐
// Ikkalasi ham "SMS ketmaydi" degani, lekin xavfsizlik jihatidan
// qarama-qarshi:
//
//	LogSms      — kodni logga OCHIQ yozadi va `nil` qaytaradi, ya'ni
//	              chaqiruvchi "yuborildi" deb o'ylaydi. Production'da
//	              bu to'liq akkaunt egallash yo'li (bug.md 47-band).
//	DisabledSms — kodni HECH QAYERGA yozmaydi va HAR DOIM xato
//	              qaytaradi, ya'ni "jimgina muvaffaqiyat" bo'lmaydi.
//
// Shu sabab SMS'siz production faqat SHU tur bilan mumkin. 47-banddagi
// `os.Exit(1)` qoidasi kuchida qoladi: farq shundaki, endi SMS'siz
// ishlash TASODIF emas, `.env` dagi ANIQ qaror bo'lishi shart.
//
// Kod baribir yetkaziladi — Telegram bot uni chatning o'zida beradi
// (`users.IssueCode` izohiga qarang).
// └────────────────────────────────────────────────────────────────────┘
type DisabledSms struct{}

func (DisabledSms) Send(phone, text string) error { return ErrSmsDisabled }
