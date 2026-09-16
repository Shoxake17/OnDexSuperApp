package notify

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// SMS o'chirilganda bir martalik KOD hech qayerga yozilmasligi kerak.
//
// Sabab bug.md 47-bandida: `LogSms` kodni ochiq logga yozadi va `nil`
// qaytaradi. Uni "SMS o'chirilgan" holati uchun ishlatish production'da
// akkaunt egallash yo'lini ochadi — logni o'qiy oladigan odam istalgan
// raqamga kirish oqimini boshlab, kodni logdan oladi.
func TestDisabledSmsNeverLogsTheCode(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	const code = "483920"
	err := DisabledSms{}.Send("+998901234567", "OnDex tasdiqlash kodi: "+code)

	if err == nil {
		t.Fatal("DisabledSms xato qaytarishi SHART — aks holda chaqiruvchi 'yuborildi' deb o'ylaydi")
	}
	if !errors.Is(err, ErrSmsDisabled) {
		t.Errorf("ErrSmsDisabled kutilgandi, olindi: %v", err)
	}
	if strings.Contains(buf.String(), code) {
		t.Fatalf("KOD LOGGA TUSHDI — bu 47-banddagi teshikning o'zi. Log: %s", buf.String())
	}
	if buf.Len() != 0 {
		t.Errorf("DisabledSms umuman log yozmasligi kerak, yozilgani: %s", buf.String())
	}
}

// Taqqoslash uchun: `LogSms` kodni ATAYLAB yozadi (dev qulayligi).
// Bu test ikkalasining farqini qulflaydi — kelajakda kimdir
// `DisabledSms` o'rniga `LogSms` qo'yib yuborsa yuqoridagi test
// qizil beradi, bu esa nega qizil ekanini tushuntiradi.
func TestLogSmsDoesLogTheCode(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)

	const code = "119274"
	if err := (LogSms{}).Send("+998901234567", "OnDex tasdiqlash kodi: "+code); err != nil {
		t.Fatalf("LogSms xato qaytarmasligi kerak: %v", err)
	}
	if !strings.Contains(buf.String(), code) {
		t.Error("LogSms dev uchun kodni logga yozishi kerak edi")
	}
}
