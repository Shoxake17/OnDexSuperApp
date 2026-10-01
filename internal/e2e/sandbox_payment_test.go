package e2e

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"chustapp/internal/payments/multicard"
)

// MULTICARD SANDBOX: butun to'lov yo'li haqiqiy shlyuzga qarshi.
//
// ┌─ NEGA ALOHIDA, QO'LDA YOQILADIGAN TEST ────────────────────────────┐
// Soxta shlyuz (`gateway.go`) tez va barqaror, lekin u BIZ O'YLAGAN
// shlyuz: haqiqiy endpoint yo'llari, javob shakllari va callback
// yetkazilishini tekshirmaydi. Aynan shu bo'shliq to'lanmagan invoysni
// yopib bo'lmasligini yashirgan edi. Bu test tarmoqqa chiqadi va BIR
// qadamda odam kerak (to'lov sahifasida test karta), shuning uchun CI'da
// ishlamaydi.
//
// ┌─ CALLBACK XAVFSIZ YETKAZILADI ─────────────────────────────────────┐
// Shlyuz callback yuborishi uchun server internetdan ko'rinishi kerak.
// Butun testni ochib qo'ymaymiz: tunnel FAQAT mana bu cheklangan proksiga
// (`127.0.0.1:18090`) yo'naltiriladi, u esa FAQAT
// `POST /payments/multicard/callback` ni o'tkazadi, qolgan hamma narsaga 404.
// └────────────────────────────────────────────────────────────────────┘
//
// Ishga tushirish (tunnel va kalitlar `scripts/payflow.ps1` da):
//
//	MULTICARD_SANDBOX=1 PAYFLOW_CALLBACK_BASE=https://xxxx.trycloudflare.com \
//	  go test ./internal/e2e/ -run MulticardSandbox -v -count=1 -timeout 25m
func TestMulticardSandboxEndToEnd(t *testing.T) {
	if os.Getenv("MULTICARD_SANDBOX") == "" {
		t.Skip("MULTICARD_SANDBOX berilmagan - tarmoq testi o'tkazib yuborildi")
	}
	base := os.Getenv("MULTICARD_BASE_URL")
	if !multicard.IsSandbox(base) {
		t.Fatalf("RAD ETILDI: %q sandbox emas - production'ga to'lov yaratilmaydi", base)
	}
	cb := strings.TrimRight(os.Getenv("PAYFLOW_CALLBACK_BASE"), "/")
	if !strings.HasPrefix(cb, "https://") {
		t.Fatal("PAYFLOW_CALLBACK_BASE (tunnelning https manzili) kerak")
	}
	store := os.Getenv("MULTICARD_STORE_ID")
	if store == "" {
		store = "6"
	}
	live := LiveMulticard{
		BaseURL: base, AppID: os.Getenv("MULTICARD_APPLICATION_ID"), Secret: os.Getenv("MULTICARD_SECRET"),
		StoreID: store, NotifyURL: cb + "/payments/multicard/callback",
	}
	w := NewWorldWith(t, "Multicard sandbox (haqiqiy shlyuz)", Options{Live: &live})

	// Shlyuz tomonidagi HAQIQIY holatni o'qish uchun mustaqil klient.
	gw, err := multicard.New(multicard.Config{BaseURL: base, ApplicationID: live.AppID, Secret: live.Secret})
	if err != nil {
		t.Fatal(err)
	}
	providerStatus := func(uuid string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		info, err := gw.Payment(ctx, uuid)
		if err != nil {
			t.Fatalf("shlyuzdan holat o'qilmadi (%s): %v", uuid, err)
		}
		return strings.ToLower(info.Status)
	}

	// ── Cheklangan callback proksi ───────────────────────────────────
	target, _ := url.Parse(w.Srv.URL)
	rp := httputil.NewSingleHostReverseProxy(target)
	var hitsMu sync.Mutex
	hits := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /payments/multicard/callback", func(rw http.ResponseWriter, r *http.Request) {
		hitsMu.Lock()
		hits++
		hitsMu.Unlock()
		rp.ServeHTTP(rw, r)
	})
	// Boshqa hamma yo'l: 404 (tunnel orqali ichkariga kirib bo'lmaydi).
	addr := os.Getenv("PAYFLOW_PROXY_ADDR")
	if addr == "" {
		addr = "127.0.0.1:18090"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("proksi %s da ochilmadi: %v", addr, err)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	callbackHits := func() int { hitsMu.Lock(); defer hitsMu.Unlock(); return hits }

	payment := func(orderID string) (id, uuid, status string, amount int64) {
		list, _ := w.Payments.ListByOrder(context.Background(), orderID)
		if len(list) == 0 {
			return "", "", "", 0
		}
		p := list[len(list)-1]
		return p.ID, p.ProviderPaymentID, string(p.Status), p.AmountTiyin
	}

	// ── 1. To'lanmagan invoys bekor qilinsa, SHLYUZDA ham yopiladi (avtomatik) ──
	w.Step(t, "to'lanmagan buyurtma bekor qilinsa invoys shlyuzda ham yopiladi", func(t *testing.T) {
		o := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "card", "live-unpaid-1")
		id, _ := o["id"].(string)
		resp := w.Do(t, w.Customer, "POST", "/orders/"+id+"/pay", nil).Expect(t, 200).Map(t)
		if u, _ := resp["pay_url"].(string); !strings.HasPrefix(u, "https://") {
			t.Fatalf("to'lov havolasi yaroqsiz: %v", resp["pay_url"])
		}
		_, uuid, _, _ := payment(id)
		if uuid == "" {
			t.Fatal("shlyuz uuid yozilmagan")
		}
		if st := providerStatus(uuid); st == "success" {
			t.Fatalf("to'lanmagan invoys shlyuzda to'langan ko'rinadi: %s", st)
		}
		w.Transition(t, w.Customer, id, "cancelled")
		if st := providerStatus(uuid); st != "cancelled" {
			t.Errorf("shlyuzdagi holat %q, kutilgan cancelled: invoys yopilmadi, mijoz keyin to'lab qo'yishi mumkin", st)
		}
	})

	// ── 2. HAQIQIY to'lov (odam kerak) va rad etilganda QAYTARISH ──────
	w.Step(t, "haqiqiy to'lov callback orqali tasdiqlanadi, rad etilsa pul qaytadi", func(t *testing.T) {
		o := w.PlaceOrder(t, w.Customer, Cart(ProdOsh, 1), "card", "live-paid-1")
		id, _ := o["id"].(string)
		resp := w.Do(t, w.Customer, "POST", "/orders/"+id+"/pay", nil).Expect(t, 200).Map(t)
		payURL, _ := resp["pay_url"].(string)
		pid, uuid, _, amount := payment(id)

		// Odam uchun: havola faylga ham yoziladi (-v chiqishi aralashib ketmasligi uchun).
		_ = os.MkdirAll(filepath.Join("..", "..", "logs", "e2e"), 0o750)
		_ = os.WriteFile(filepath.Join("..", "..", "logs", "e2e", "payflow_url.txt"),
			[]byte(payURL+"\n"), 0o600)
		t.Logf("PAYFLOW_PAY_URL=%s", payURL)
		t.Logf("PAYFLOW_AMOUNT_TIYIN=%d", amount)
		t.Log("PAYFLOW_WAITING: havolani oching va sandbox test kartasi bilan to'lang")

		wait := 12 * time.Minute
		if v := os.Getenv("PAYFLOW_WAIT"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				wait = d
			}
		}
		deadline := time.Now().Add(wait)
		for {
			if _, _, st, _ := payment(id); st == "paid" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("to'lov %v ichida tasdiqlanmadi (callback keldi: %d marta)", wait, callbackHits())
			}
			time.Sleep(2 * time.Second)
		}
		t.Logf("to'lov tasdiqlandi; callback soni: %d", callbackHits())
		if callbackHits() == 0 {
			t.Error("holat callback orqali emas, boshqa yo'l bilan o'zgardi (shlyuz callback yubormadi?)")
		}

		// Shlyuz tomonidagi haqiqat bilan solishtirish.
		if st := providerStatus(uuid); st != "success" {
			t.Errorf("shlyuzdagi holat %q, kutilgan success", st)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		info, err := gw.Payment(ctx, uuid)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if info.Amount() != amount {
			t.Errorf("summa: shlyuzda %d, bizda %d", info.Amount(), amount)
		}
		if info.Invoice() != pid {
			t.Errorf("invoice_id: shlyuzda %q, bizda %q", info.Invoice(), pid)
		}
		got := w.Do(t, w.Customer, "GET", "/orders/"+id, nil).Expect(t, 200).Map(t)
		if st, _ := got["payment_state"].(string); st != "paid" && st != "held" {
			t.Errorf("buyurtma to'lov holati: %q", st)
		}

		// Restoran rad etadi: pul QAYTADI (haqiqiy `DELETE /payment/{uuid}/partial`).
		w.Transition(t, w.Owner, id, "rejected")
		w.Eventually(t, "to'lov qaytarildi", 30*time.Second, func() bool {
			_, _, st, _ := payment(id)
			return st == "refunded"
		})
		if st := providerStatus(uuid); st != "revert" {
			t.Errorf("shlyuzdagi holat %q, kutilgan revert (pul qaytmadi)", st)
		}
	})
}
