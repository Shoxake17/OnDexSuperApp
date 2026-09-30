package multicard

import (
	"context"
	"os"
	"testing"
	"time"
)

// HAQIQIY SINOV STENDIGA QARSHI TEKSHIRUV.
//
// ┌─ NEGA BAYROQ BILAN ────────────────────────────────────────────────┐
// Bu test TARMOQQA chiqadi va begona xizmatga bog'liq. CI'da doim
// ishlasa, Multicard tomonidagi nosozlik bizning quvurni yiqitardi va
// sabab bizda deb o'ylanardi.
//
// Shuning uchun u FAQAT qo'lda yoqiladi:
//
//	$env:MULTICARD_LIVE=1
//	go test ./internal/payments/multicard/ -run Live -v
//
// Kalitlar ham muhitdan olinadi — kodda sir yozilmaydi.
// └────────────────────────────────────────────────────────────────────┘
func TestLiveStoreHistory(t *testing.T) {
	if os.Getenv("MULTICARD_LIVE") == "" {
		t.Skip("MULTICARD_LIVE berilmagan — tarmoq testi o'tkazib yuborildi")
	}
	c, err := New(Config{
		BaseURL:       os.Getenv("MULTICARD_BASE_URL"),
		ApplicationID: os.Getenv("MULTICARD_APPLICATION_ID"),
		Secret:        os.Getenv("MULTICARD_SECRET"),
	})
	if err != nil {
		t.Fatalf("sozlanmadi: %v", err)
	}
	store := os.Getenv("MULTICARD_STORE_ID")
	if store == "" {
		t.Fatal("MULTICARD_STORE_ID kerak")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	h, err := c.StoreHistory(ctx, HistoryQuery{
		StoreID: store,
		From:    time.Now().AddDate(0, 0, -30),
		To:      time.Now(),
		Limit:   5,
	})
	if err != nil {
		t.Fatalf("reestr olinmadi: %v", err)
	}
	t.Logf("jami yozuv: %d", h.Pagination.Total)
	t.Logf("muvaffaqiyatli tushum: %d tiyin", h.SuccessTiyin())
	t.Logf("qaytarilgan: %d tiyin", h.RefundedTiyin())
	for _, tr := range h.List {
		t.Logf("  %s %s %d tiyin (komissiya %d) %s",
			tr.PaymentTime, tr.Status, tr.AmountTiyin, tr.CommissionTiyin, tr.PS)
	}

	// Ikkinchi chaqiruv — token keshdan kelishi kerak, ya'ni
	// `POST /auth` qaytarilmaydi. Buni bu yerda o'lchab bo'lmaydi,
	// lekin xato bo'lmasligi kerak.
	if _, err := c.StoreHistory(ctx, HistoryQuery{
		StoreID: store, From: time.Now().AddDate(0, 0, -1), To: time.Now(), Limit: 1,
	}); err != nil {
		t.Errorf("ikkinchi chaqiruv: %v", err)
	}
}
