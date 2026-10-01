package multicard

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// MULTICARD SANDBOX: to'lov hayot sikli (haqiqiy shlyuzga qarshi).
//
// ┌─ NEGA BAYROQ BILAN, VA NEGA FAQAT SANDBOX ─────────────────────────┐
// Test TARMOQQA chiqadi (`live_test.go` bilan bir xil sabab). Qo'shimcha:
// u invoys YARATADI va bekor qiladi. Production shlyuziga bunday test
// yuborish mumkin emas, shuning uchun `IsSandbox` bo'lmasa test
// TO'XTAYDI (o'tkazib yuborilmaydi: jimgina o'tib ketsa, kimdir
// production kalitlari bilan "yashil" deb o'ylab qolardi).
//
// Ishga tushirish (kalitlar muhitdan; kodda sir yo'q):
//
//	$env:MULTICARD_SANDBOX=1
//	$env:MULTICARD_BASE_URL="https://dev-mesh.multicard.uz"
//	$env:MULTICARD_APPLICATION_ID="..."; $env:MULTICARD_SECRET="..."
//	$env:MULTICARD_STORE_ID="6"   # ixtiyoriy, standart 6 (hujjatdagi sinov kassasi)
//	go test ./internal/payments/multicard/ -run Sandbox -v
//
// └────────────────────────────────────────────────────────────────────┘
func sandboxClient(t *testing.T) (*Client, string, string) {
	t.Helper()
	if os.Getenv("MULTICARD_SANDBOX") == "" {
		t.Skip("MULTICARD_SANDBOX berilmagan - tarmoq testi o'tkazib yuborildi")
	}
	base := os.Getenv("MULTICARD_BASE_URL")
	if !IsSandbox(base) {
		t.Fatalf("RAD ETILDI: %q sandbox emas - bu test production'ga invoys yaratmaydi", base)
	}
	secret := os.Getenv("MULTICARD_SECRET")
	c, err := New(Config{BaseURL: base, ApplicationID: os.Getenv("MULTICARD_APPLICATION_ID"), Secret: secret})
	if err != nil {
		t.Fatal(err)
	}
	store := os.Getenv("MULTICARD_STORE_ID")
	if store == "" {
		store = "6"
	}
	return c, store, secret
}

func sandboxOFD(total int64) []OFDItem {
	return []OFDItem{{
		Name: "Sandbox sinov taomi", Qty: 1, Price: total, Total: total,
		MXIK: "10204001001000000", PackageCode: "1500169",
	}}
}

func TestSandboxPaymentLifecycle(t *testing.T) {
	c, store, _ := sandboxClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 1. Kassa mavjud va kalitlar ishlaydi (auth + store).
	info, err := c.Store(ctx, store)
	if err != nil {
		t.Fatalf("kassa o'qilmadi (kalit yoki store_id noto'g'ri?): %v", err)
	}
	t.Logf("kassa o'qildi: id=%d", info.ID)

	// 2. Invoys yaratish.
	const amount = int64(100_000) // 1000 so'm
	invID := fmt.Sprintf("ondex-sandbox-%d", time.Now().UnixNano())
	inv, err := c.CreateInvoice(ctx, InvoiceRequest{
		StoreID: store, AmountTiyin: amount, InvoiceID: invID,
		CallbackURL: "https://example.com/multicard-sandbox-callback",
		OFD:         sandboxOFD(amount), Lang: "uz", TTLSeconds: 600,
	})
	if err != nil {
		t.Fatalf("invoys yaratilmadi: %v", err)
	}
	if inv.UUID == "" {
		t.Fatal("invoys uuid bo'sh")
	}
	pay := inv.PayURL()
	u, perr := url.Parse(pay)
	if pay == "" || perr != nil || u.Scheme != "https" {
		t.Fatalf("to'lov havolasi yaroqsiz: %q (%v)", pay, perr)
	}
	t.Logf("invoys yaratildi: uuid=%s, havola xosti=%s", inv.UUID, u.Host)

	// 3. Holatni o'qish: summa va bizning identifikator mos, hali to'lanmagan.
	st, err := c.Payment(ctx, inv.UUID)
	if err != nil {
		t.Fatalf("holat o'qilmadi: %v", err)
	}
	if st.Amount() != amount {
		t.Errorf("summa: %d, kutilgan %d", st.Amount(), amount)
	}
	if st.Invoice() != invID {
		t.Errorf("invoice_id: %q, kutilgan %q", st.Invoice(), invID)
	}
	if MapStatusIsPaid(st.Status) {
		t.Errorf("to'lanmagan invoys to'langan ko'rinyapti: status=%q", st.Status)
	}
	t.Logf("boshlang'ich holat: %q", st.Status)

	// 4. Bekor qilish (to'lanmagan invoysni).
	if err := c.CancelInvoice(ctx, inv.UUID); err != nil {
		t.Fatalf("bekor qilib bo'lmadi: %v", err)
	}
	after, err := c.Payment(ctx, inv.UUID)
	if err != nil {
		t.Fatalf("bekordan keyingi holat o'qilmadi: %v", err)
	}
	if MapStatusIsPaid(after.Status) {
		t.Errorf("bekor qilingan invoys to'langan ko'rinyapti: %q", after.Status)
	}
	t.Logf("bekordan keyingi holat: %q", after.Status)

	// 5. Bekor qilingan invoysni QAYTA bekor qilish yoki to'lashga urinish xavfsiz bo'lishi kerak.
	if err := c.CancelInvoice(ctx, inv.UUID); err != nil {
		t.Logf("takroriy bekor rad etildi (kutilgan): %v", err)
	} else {
		t.Log("takroriy bekor xatosiz o'tdi (idempotent)")
	}
}

// Haqiqiy kalit bilan callback imzosi: to'g'ri imzo qabul, buzilgani rad.
func TestSandboxCallbackSignatureWithRealSecret(t *testing.T) {
	c, _, secret := sandboxClient(t)
	cb := Callback{UUID: "sandbox-uuid", InvoiceID: "ondex-sandbox-x", Amount: 100_000}
	cb.Sign = signFor(cb, secret)
	if !c.VerifyCallback(cb) {
		t.Fatal("to'g'ri imzo rad etildi")
	}
	for name, bad := range map[string]Callback{
		"summa o'zgardi": {UUID: cb.UUID, InvoiceID: cb.InvoiceID, Amount: 1, Sign: cb.Sign},
		"uuid o'zgardi":  {UUID: "boshqa", InvoiceID: cb.InvoiceID, Amount: cb.Amount, Sign: cb.Sign},
		"invoys boshqa":  {UUID: cb.UUID, InvoiceID: "boshqa", Amount: cb.Amount, Sign: cb.Sign},
		"imzo bo'sh":     {UUID: cb.UUID, InvoiceID: cb.InvoiceID, Amount: cb.Amount},
		"imzo xato":      {UUID: cb.UUID, InvoiceID: cb.InvoiceID, Amount: cb.Amount, Sign: strings.Repeat("0", 40)},
	} {
		if c.VerifyCallback(bad) {
			t.Errorf("%s: buzilgan callback QABUL qilindi", name)
		}
	}
}

// MapStatusIsPaid - sinov uchun: shlyuz holati "to'langan" ma'nosidami.
func MapStatusIsPaid(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "paid", "completed":
		return true
	}
	return false
}
