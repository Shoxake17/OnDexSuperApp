package e2e

import (
	"crypto/md5"
	"crypto/sha1" //nolint:gosec // imzo algoritmini shlyuz belgilaydi
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Gateway — Multicard (Rahmat) shlyuzining soxta nusxasi.
//
// Haqiqiy pul va tashqi tarmoqsiz butun to'lov yo'lini (invoys yaratish,
// kanal tanlash, callback, holat tekshiruvi) sinash uchun. Sandbox
// o'chiq turgan kuni ham E2E o'tadi.
type Gateway struct {
	Srv *httptest.Server

	mu          sync.Mutex
	LastInvoice map[string]any
	LastDirect  map[string]any
	Status      string // GET /payment/{uuid} qaytaradigan holat
	StatusFails bool   // status API ishlamaydi (yagona dalil imzo bo'lsin)
	uuidSeq     int
	amounts     map[string]int64 // uuid -> invoys summasi (tiyin)
	// Reversals — shlyuzga yuborilgan QAYTARISH (refund/bekor) so'rovlari.
	Reversals []string
	// InvoiceCancels — TO'LANMAGAN invoysni bekor qilish so'rovlari
	// (`DELETE /payment/invoice/{uuid}`).
	InvoiceCancels []string
	// Misrouted — noto'g'ri yo'lga kelgan so'rovlar. Haqiqiy shlyuz
	// `DELETE /payment/{uuid}` ni to'lanmagan invoysda `404` bilan rad
	// etadi (sandbox'da tasdiqlangan); shuning uchun bunday so'rov
	// xato deb sanaladi va test uni ushlaydi.
	Misrouted []string
}

const (
	GatewaySecret = "e2e-restoran-siri"
	GatewayAppID  = "e2e_app"
	GatewayStore  = "6"
)

func NewGateway() *Gateway {
	g := &Gateway{Status: "success", amounts: map[string]int64{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"tok","role":"dev","expiry":"` +
			time.Now().Add(24*time.Hour).Format("2006-01-02 15:04:05") + `"}`))
	})
	create := func(into *map[string]any) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			g.mu.Lock()
			*into = body
			g.uuidSeq++
			id := "gw-uuid-" + strconv.Itoa(g.uuidSeq)
			if a, ok := body["amount"].(float64); ok {
				g.amounts[id] = int64(a)
			}
			g.mu.Unlock()
			_, _ = w.Write([]byte(`{"success":true,"data":{"uuid":"` + id +
				`","checkout_url":"https://pay.example/invoice/` + id + `"}}`))
		}
	}
	mux.HandleFunc("POST /payment/invoice", create(&g.LastInvoice))
	mux.HandleFunc("POST /payment", create(&g.LastDirect))
	mux.HandleFunc("GET /payment/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		g.mu.Lock()
		fails, status := g.StatusFails, g.Status
		amount := g.amounts[r.PathValue("uuid")]
		g.mu.Unlock()
		if fails {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"uuid":"` + r.PathValue("uuid") +
			`","status":"` + status + `","payment_amount":` + strconv.FormatInt(amount, 10) + `}}`))
	})
	// Kassa ma'lumoti (Moliya "ulash"): faqat GatewayStore mavjud.
	mux.HandleFunc("GET /payment/store/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != GatewayStore {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":"ERROR_STORE_NOT_FOUND","details":"kassa topilmadi"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"id":6,"uuid":"store-uuid","title":"E2E kassa",
		  "tax_mxik":"10204001001000000","tax_package_code":"1500169","tax_registration":0,
		  "merchant":{"name":"E2E MCHJ","tin":"123456789"},
		  "merchant_account":{"account_no":"20208000900000000001","mfo":"00014","official_name":"E2E"},
		  "contract":{"num":"E2E-1","fee":{"up":1.5,"down":"1.5"}}}}`))
	})
	// Tushum reestri (Moliya sahifasi).
	mux.HandleFunc("GET /payment/store/{id}/history", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"data":{"pagination":{"total":2,"offset":0,"limit":200},
		  "stat":[{"status":"success","ps":"uzum","payment_amount":7000000},
		          {"status":"revert","ps":"payme","payment_amount":500000},
		          {"status":"draft","ps":"click","payment_amount":99000000}],
		  "list":[{"uuid":"t1","status":"success","ps":"uzum","store_invoice_id":"o-1","payment_amount":7000000,
		           "commission_amount":105000,"card_pan":"8600****1234","payment_time":"2026-10-01 12:00:00"},
		          {"uuid":"t2","status":"draft","ps":"click","store_invoice_id":"o-2","payment_amount":99000000,
		           "commission_amount":0,"card_pan":"","payment_time":"2026-10-01 12:05:00"},
		          {"uuid":"t3","status":"cancelled","ps":"payme","store_invoice_id":"o-3","payment_amount":3000000,
		           "commission_amount":0,"card_pan":"","payment_time":"2026-10-01 12:06:00"}]}}`))
	})
	// DELETE yo'llari BITTA yo'naltiruvchida (ServeMux'da `/payment/{uuid}/partial` va
	// `/payment/invoice/{uuid}` bir-biriga zid shablonlar). Xatti-harakat HAQIQIY
	// Multicard sandbox sinovidan olingan:
	//
	//	/payment/invoice/{uuid}  - to'lanmagan invoysni bekor qilish
	//	/payment/{uuid}          - to'langanni TO'LIQ qaytarish (holat `revert`)
	//	/payment/{uuid}/partial  - sandbox kassa "частичную отмену" ni QO'LLAMAYDI (400);
	//	                           to'liq qaytarish shu yo'ldan yuborilsa test yiqiladi
	mux.HandleFunc("DELETE /payment/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/payment/"), "/"), "/")
		g.mu.Lock()
		defer g.mu.Unlock()
		switch {
		case len(parts) == 2 && parts[0] == "invoice":
			g.InvoiceCancels = append(g.InvoiceCancels, parts[1])
			_, _ = w.Write([]byte(`{"success":true,"data":null}`))
		case len(parts) == 1:
			g.Reversals = append(g.Reversals, parts[0])
			_, _ = w.Write([]byte(`{"success":true,"data":{"uuid":"` + parts[0] + `","status":"revert"}}`))
		default:
			g.Misrouted = append(g.Misrouted, "DELETE "+r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":"ERROR_FIELDS","details":"Касса не поддерживает частичную отмену"}}`))
		}
	})
	g.Srv = httptest.NewServer(mux)
	return g
}

func (g *Gateway) Close() { g.Srv.Close() }

// ReversalCount — nechta to'lov qaytarilgani.
func (g *Gateway) ReversalCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.Reversals)
}

// InvoiceCancelCount — nechta to'lanmagan invoys bekor qilingan.
func (g *Gateway) InvoiceCancelCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.InvoiceCancels)
}

// MisroutedCount — noto'g'ri yo'lga kelgan so'rovlar soni (0 bo'lishi kerak).
func (g *Gateway) MisroutedCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.Misrouted)
}

// Invoice — oxirgi yaratilgan invoys/to'lov tanasi (qaysi biri bo'lsa).
func (g *Gateway) Last() (invoice, direct map[string]any) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.LastInvoice, g.LastDirect
}

// Sign — callback imzosi: SHA1(uuid + invoice_id + amount + secret).
func Sign(uuid, invoiceID string, amount int64, secret string) string {
	sum := sha1.Sum([]byte(uuid + invoiceID + strconv.FormatInt(amount, 10) + secret)) //nolint:gosec
	return hex.EncodeToString(sum[:])
}

// SignSuccess - SUCCESS callback imzosi: MD5(store_id + invoice_id + amount + secret).
func SignSuccess(storeID, invoiceID string, amount int64, secret string) string {
	h := md5.Sum([]byte(storeID + invoiceID + strconv.FormatInt(amount, 10) + secret)) //nolint:gosec // algoritmni shlyuz belgilaydi
	return hex.EncodeToString(h[:])
}
