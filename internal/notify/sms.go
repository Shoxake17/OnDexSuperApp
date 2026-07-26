package notify

import "log/slog"

// LogSms — dev muhit uchun SMS "yuboruvchi": haqiqiy SMS o'rniga logga yozadi.
// Production'da bu o'rinni Eskiz.uz implementatsiyasi egallaydi.
type LogSms struct{}

func (LogSms) Send(phone, text string) error {
	slog.Info("sms (dev rejim, yuborilmadi)", "phone", phone, "text", text)
	return nil
}
