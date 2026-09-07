package telegram

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Bot tokenining logga tushishi (bug.md 46-band).
//
// ┌─ NEGA BU JIM XATO EDI ─────────────────────────────────────────────┐
// Telegram API tokenni URL YO'LIDA kutadi. Tarmoq xatosida Go ning
// `*url.Error` xabari BUTUN URL ni o'z ichiga oladi va u
// to'g'ridan-to'g'ri `slog.Warn` ga uzatilardi. Polling sikli
// UZLUKSIZ ishlaydi, ya'ni bu muqarrar edi: token Docker loglarida
// 50 MB gacha saqlanardi.
//
// "Sirlarni logga yozish topilmadi" degan avtomatik tekshiruv buni
// KO'RMAGAN, chunki sir o'zgaruvchi nomi bilan emas, tayyor satr
// ichida uzatiladi. Shu sabab test SATR bo'yicha tekshiradi.
// └────────────────────────────────────────────────────────────────────┘

const testToken = "7123456789:AAF-fake-token-for-tests"

// ASOSIY REGRESSIYA: haqiqiy `*url.Error` shaklidagi xato tozalanishi
// kerak.
func TestRedactTokenRemovesTokenFromURLError(t *testing.T) {
	c := &Client{token: testToken}

	// `http.Client.Do` aynan shunday xato qaytaradi.
	raw := fmt.Errorf(`Post "https://api.telegram.org/bot%s/getUpdates": dial tcp: i/o timeout`,
		testToken)

	got := c.redactToken(raw)
	if strings.Contains(got.Error(), testToken) {
		t.Fatalf("TOKEN XATO MATNIDA QOLDI: %s", got)
	}
	if !strings.Contains(got.Error(), "<token>") {
		t.Fatalf("token o'rniga belgi qo'yilmadi: %s", got)
	}
	// Nosozlikni tushunish uchun qolgan qism saqlanishi kerak.
	if !strings.Contains(got.Error(), "i/o timeout") {
		t.Fatalf("xato sababi yo'qoldi: %s", got)
	}
}

// O'ralgan xato zanjiri (`fmt.Errorf("%w")`, retry qatlamlari) ham
// tozalanishi kerak — shuning uchun `*url.Error` ni ochish emas, SATR
// bo'yicha almashtirish tanlangan.
func TestRedactTokenWorksThroughWrappedErrors(t *testing.T) {
	c := &Client{token: testToken}

	inner := fmt.Errorf(`Post "https://api.telegram.org/bot%s/sendMessage": EOF`, testToken)
	wrapped := fmt.Errorf("telegram: qayta urinish tugadi: %w", inner)

	if got := c.redactToken(wrapped); strings.Contains(got.Error(), testToken) {
		t.Fatalf("o'ralgan xatoda token qoldi: %s", got)
	}
}

// Token uchramasa asl xato SAQLANISHI kerak — `errors.Is/As` bilan
// tekshiradigan chaqiruvchilar buzilmasin.
func TestRedactTokenKeepsErrorWhenNothingToRedact(t *testing.T) {
	c := &Client{token: testToken}
	sentinel := errors.New("oddiy tarmoq xatosi")

	if got := c.redactToken(sentinel); !errors.Is(got, sentinel) {
		t.Fatal("token uchramaganda asl xato almashtirildi")
	}
}

// Chegaraviy holatlar: nil xato va sozlanmagan klient.
func TestRedactTokenEdgeCases(t *testing.T) {
	c := &Client{token: testToken}
	if got := c.redactToken(nil); got != nil {
		t.Fatalf("nil xato o'zgardi: %v", got)
	}

	// Token bo'sh bo'lsa almashtiradigan narsa yo'q — va bo'sh satrni
	// almashtirish butun matnni buzardi.
	empty := &Client{}
	orig := errors.New("xato matni")
	if got := empty.redactToken(orig); !errors.Is(got, orig) {
		t.Fatal("sozlanmagan klient xatoni o'zgartirdi")
	}
}
