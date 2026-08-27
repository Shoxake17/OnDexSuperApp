// Package octo — Octo (secure.octo.uz) to'lov provayderi.
//
// ┌─ QAYSI SXEMA TANLANGAN VA NEGA ───────────────────────────────────┐
// "Оплата через платежную страницу OCTO": mijoz OCTO ning O'Z
// sahifasida karta ma'lumotini kiritadi, biz faqat havolaga
// yo'naltiramiz.
//
// Karta raqami bizning serverga UMUMAN kelmaydi — bu PCI DSS
// sertifikatini talab qilmaydi va eng katta xatarni (karta ma'lumoti
// bazasi) butunlay yo'q qiladi. "Оплата через сайт Партнёра" varianti
// esa PCI DSS talab qiladi, shuning uchun ATAYLAB ishlatilmadi.
// └───────────────────────────────────────────────────────────────────┘
//
// Hujjatlar: `image/octo/*.md`.
package octo

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"chustapp/internal/payments"
)

// BaseURL — Octo API manzili.
const BaseURL = "https://secure.octo.uz"

// Config — `.env` dan keladi. `Secret` va `SignatureKey` — SIR, hech
// qachon log qilinmaydi va klientga chiqarilmaydi.
type Config struct {
	// ShopID — ЛК dagi raqamli do'kon ID (`octo_shop_id`).
	ShopID int64
	// Secret — ЛК da generatsiya qilinadigan maxfiy kalit
	// (`octo_secret`).
	Secret string
	// SignatureKey — callback imzosini tekshirish uchun `unique_key`.
	// Octo texnik jamoasidan alohida so'raladi; BO'SH BO'LISHI MUMKIN
	// (VerifyCallbackSignature izohiga qarang).
	SignatureKey string
	// Test — true bo'lsa har bir to'lov `test: true` bilan yaratiladi
	// (haqiqiy pul harakatlanmaydi).
	Test bool
	// BaseURL — sinovda httptest serveriga qaratish uchun. Bo'sh
	// bo'lsa `BaseURL` ishlatiladi.
	BaseURL string
	// HTTPClient — sinov va timeout boshqaruvi uchun.
	HTTPClient *http.Client
}

type Client struct {
	cfg  Config
	http *http.Client
	base string
}

func New(cfg Config) (*Client, error) {
	if cfg.ShopID == 0 || strings.TrimSpace(cfg.Secret) == "" {
		return nil, payments.ErrNotConfigured
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	if base == "" {
		base = BaseURL
	}
	hc := cfg.HTTPClient
	if hc == nil {
		// Timeout MAJBURIY: to'lov provayderi javob bermay qolsa,
		// buyurtma so'rovi cheksiz osilib, mijoz ekranida "yuklanmoqda"
		// bo'lib qolardi.
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	return &Client{cfg: cfg, http: hc, base: base}, nil
}

func (c *Client) Name() string { return "octo" }

// TestMode — sinov rejimidami (log va diagnostika uchun).
func (c *Client) TestMode() bool { return c.cfg.Test }

// ── So'rov/javob umumiy qismi ──────────────────────────────────────

// response — Octo javobining umumiy o'ramchisi. Maydonlar javobning
// ILDIZIDA ham, `data` ichida ham keladi (hujjatda ikkalasi ham bor,
// va ular kelajakda faqat `data` ga ko'chishi aytilgan) — shuning
// uchun ikkalasi ham o'qiladi va `data` ustunlik qiladi.
type response struct {
	Error      int             `json:"error"`
	ErrMessage string          `json:"errMessage"`
	Data       json.RawMessage `json:"data"`
}

type paymentData struct {
	ShopTransactionID string      `json:"shop_transaction_id"`
	OctoPaymentUUID   string      `json:"octo_payment_UUID"`
	Status            string      `json:"status"`
	OctoPayURL        string      `json:"octo_pay_url"`
	TotalSum          json.Number `json:"total_sum"`
	RefundedSum       json.Number `json:"refunded_sum"`
}

func (c *Client) post(ctx context.Context, path string, body any) (*response, []byte, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("octo: so'rov yuborib bo'lmadi: %w", err)
	}
	defer resp.Body.Close()

	// Javob hajmi cheklanadi: buzilgan/cheksiz javob xotirani yeb
	// qo'ymasligi kerak.
	raw, err := readLimited(resp.Body, 1<<20)
	if err != nil {
		return nil, nil, fmt.Errorf("octo: javobni o'qib bo'lmadi: %w", err)
	}
	if resp.StatusCode >= 500 {
		return nil, raw, &payments.ProviderError{
			Code: resp.StatusCode, Message: "octo javob bermadi (5xx)"}
	}
	var out response
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, raw, fmt.Errorf("octo: javob JSON emas (status %d)", resp.StatusCode)
	}
	if out.Error != 0 {
		msg := out.ErrMessage
		if msg == "" {
			msg = "noma'lum xato"
		}
		return &out, raw, &payments.ProviderError{Code: out.Error, Message: msg}
	}
	return &out, raw, nil
}

// readLimited — javobni CHEKLANGAN hajmda o'qiydi. Provayder (yoki
// o'rtadagi biror qurilma) cheksiz oqim yuborsa, cheklovsiz o'qish
// serverning xotirasini yeb qo'yardi.
func readLimited(r io.Reader, max int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, max))
}

// unwrap — `data` bo'lsa undan, aks holda ildizdan o'qiydi.
func unwrap(resp *response, raw []byte) (*paymentData, error) {
	var d paymentData
	src := raw
	if len(resp.Data) > 0 && string(resp.Data) != "null" {
		src = resp.Data
	}
	if err := json.Unmarshal(src, &d); err != nil {
		return nil, fmt.Errorf("octo: javob tuzilishi kutilmagan: %w", err)
	}
	return &d, nil
}

// ── Create ─────────────────────────────────────────────────────────

type prepareRequest struct {
	ShopID            int64       `json:"octo_shop_id"`
	Secret            string      `json:"octo_secret"`
	ShopTransactionID string      `json:"shop_transaction_id"`
	AutoCapture       bool        `json:"auto_capture"`
	Test              bool        `json:"test"`
	InitTime          string      `json:"init_time"`
	TotalSum          json.Number `json:"total_sum"`
	Currency          string      `json:"currency"`
	Description       string      `json:"description"`
	ReturnURL         string      `json:"return_url,omitempty"`
	NotifyURL         string      `json:"notify_url,omitempty"`
	Language          string      `json:"language,omitempty"`
	TTL               int         `json:"ttl,omitempty"`
	UserData          *userData   `json:"user_data,omitempty"`
}

type userData struct {
	UserID string `json:"user_id"`
	Phone  string `json:"phone"`
	Email  string `json:"email"`
}

// Create — `prepare_payment`.
//
// `basket` bloki ATAYLAB yuborilmaydi: uning ichidagi `inn`,
// `package_code`, `nds` maydonlari MAJBURIY va ular soliq
// (fiskalizatsiya) ma'lumotlari. Ularni to'qib yuborish — soxta soliq
// hujjati demak. Fiskalizatsiya alohida ish sifatida qo'shilganda,
// haqiqiy qiymatlar bilan yoqiladi.
func (c *Client) Create(ctx context.Context, req payments.CreateRequest) (*payments.CreateResult, error) {
	if req.AmountTiyin <= 0 {
		return nil, errors.New("octo: to'lov summasi musbat bo'lishi kerak")
	}
	body := prepareRequest{
		ShopID:            c.cfg.ShopID,
		Secret:            c.cfg.Secret,
		ShopTransactionID: req.PaymentID,
		// Hold => auto_capture=false: pul bloklanadi, yechilmaydi.
		AutoCapture: !req.Hold,
		Test:        c.cfg.Test,
		// ┌─ VAQT UTC'DA ─────────────────────────────────────────────┐
		// Hujjatda format bor (`yyyy-MM-dd HH:mm:ss`), lekin vaqt
		// zonasi aytilmagan. Octo serverining javoblari UTC'da keladi
		// (xato javobidagi `+00:00`), shuning uchun biz ham UTC
		// yuboramiz.
		//
		// Nega muhim: Toshkent vaqti UTC+5. Mahalliy vaqt yuborilsa,
		// Octo uchun to'lov 5 soat KELAJAKDA boshlangan bo'lib
		// ko'rinadi va `ttl` (yashash muddati) hisobi buziladi.
		// └───────────────────────────────────────────────────────────┘
		InitTime:    time.Now().UTC().Format("2006-01-02 15:04:05"),
		TotalSum:    json.Number(payments.DecimalFromTiyin(req.AmountTiyin)),
		Currency:    "UZS",
		Description: req.Description,
		ReturnURL:   req.ReturnURL,
		NotifyURL:   req.NotifyURL,
		Language:    req.Language,
		TTL:         req.TTLMinutes,
	}
	if req.CustomerID != "" || req.CustomerPhone != "" {
		body.UserData = &userData{
			UserID: req.CustomerID,
			Phone:  req.CustomerPhone,
			Email:  req.CustomerEmail,
		}
	}
	resp, raw, err := c.post(ctx, "/prepare_payment", body)
	if err != nil {
		return nil, err
	}
	d, err := unwrap(resp, raw)
	if err != nil {
		return nil, err
	}
	if d.OctoPayURL == "" || d.OctoPaymentUUID == "" {
		return nil, errors.New("octo: javobda to'lov havolasi yo'q")
	}
	return &payments.CreateResult{
		ProviderPaymentID: d.OctoPaymentUUID,
		PayURL:            d.OctoPayURL,
		Status:            MapStatus(d.Status),
	}, nil
}

// ── Status ─────────────────────────────────────────────────────────

type statusRequest struct {
	ShopID            int64  `json:"octo_shop_id"`
	Secret            string `json:"octo_secret"`
	ShopTransactionID string `json:"shop_transaction_id"`
}

// Status — HAQIQIY holatni Octo'dan so'raydi (callback'dagi ma'lumotga
// ishonmasdan). Hujjatda bu ham `prepare_payment` metodi, faqat uchta
// parametr bilan.
func (c *Client) Status(ctx context.Context, paymentID string) (*payments.StatusResult, error) {
	resp, raw, err := c.post(ctx, "/prepare_payment", statusRequest{
		ShopID:            c.cfg.ShopID,
		Secret:            c.cfg.Secret,
		ShopTransactionID: paymentID,
	})
	if err != nil {
		return nil, err
	}
	d, err := unwrap(resp, raw)
	if err != nil {
		return nil, err
	}
	if d.OctoPaymentUUID == "" && d.Status == "" {
		return nil, payments.ErrNotFound
	}
	out := &payments.StatusResult{
		ProviderPaymentID: d.OctoPaymentUUID,
		Status:            MapStatus(d.Status),
		Raw:               raw,
	}
	if s := d.TotalSum.String(); s != "" {
		if t, err := payments.TiyinFromDecimal(s); err == nil {
			out.PaidAmountTiyin = t
		}
	}
	if s := d.RefundedSum.String(); s != "" {
		if t, err := payments.TiyinFromDecimal(s); err == nil {
			out.RefundedTiyin = t
		}
	}
	return out, nil
}

// ── Capture / Cancel ───────────────────────────────────────────────

type acceptRequest struct {
	ShopID          int64        `json:"octo_shop_id"`
	Secret          string       `json:"octo_secret"`
	OctoPaymentUUID string       `json:"octo_payment_UUID"`
	AcceptStatus    string       `json:"accept_status"`
	FinalAmount     *json.Number `json:"final_amount,omitempty"`
}

// Capture — bloklangan pulni yechadi (`set_accept`, accept_status=capture).
func (c *Client) Capture(ctx context.Context, providerPaymentID string, amountTiyin int64) error {
	if amountTiyin <= 0 {
		return errors.New("octo: yechiladigan summa musbat bo'lishi kerak")
	}
	amount := json.Number(payments.DecimalFromTiyin(amountTiyin))
	_, _, err := c.post(ctx, "/set_accept", acceptRequest{
		ShopID:          c.cfg.ShopID,
		Secret:          c.cfg.Secret,
		OctoPaymentUUID: providerPaymentID,
		AcceptStatus:    "capture",
		FinalAmount:     &amount,
	})
	return err
}

// Cancel — blokni bo'shatadi (`set_accept`, accept_status=cancel).
// Mijozdan pul YECHILMAYDI va qaytarish (refund) kutish ham kerak emas.
func (c *Client) Cancel(ctx context.Context, providerPaymentID string) error {
	_, _, err := c.post(ctx, "/set_accept", acceptRequest{
		ShopID:          c.cfg.ShopID,
		Secret:          c.cfg.Secret,
		OctoPaymentUUID: providerPaymentID,
		AcceptStatus:    "cancel",
	})
	return err
}

// ── Refund ─────────────────────────────────────────────────────────

type refundRequest struct {
	ShopID          int64       `json:"octo_shop_id"`
	ShopRefundID    string      `json:"shop_refund_id"`
	Secret          string      `json:"octo_secret"`
	OctoPaymentUUID string      `json:"octo_payment_UUID"`
	Amount          json.Number `json:"amount"`
}

// Refund — YECHILGAN pulni qaytaradi. Faqat `succeeded` to'lovlar
// uchun; ikki bosqichli sxemada bu kamdan-kam kerak bo'ladi (buyurtma
// rad etilsa `Cancel` yetarli).
func (c *Client) Refund(ctx context.Context, providerPaymentID, refundID string, amountTiyin int64) error {
	if amountTiyin <= 0 {
		return errors.New("octo: qaytariladigan summa musbat bo'lishi kerak")
	}
	_, _, err := c.post(ctx, "/refund", refundRequest{
		ShopID:          c.cfg.ShopID,
		ShopRefundID:    refundID,
		Secret:          c.cfg.Secret,
		OctoPaymentUUID: providerPaymentID,
		Amount:          json.Number(payments.DecimalFromTiyin(amountTiyin)),
	})
	return err
}

// ── Statuslarni moslashtirish ──────────────────────────────────────

// MapStatus — Octo statuslarini bizning holatlarimizga o'giradi
// (`image/octo/Cтатусы.md`).
func MapStatus(s string) payments.Status {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "created", "wait_user_action":
		return payments.StatusPending
	case "waiting_for_capture":
		return payments.StatusHeld
	case "succeeded":
		return payments.StatusPaid
	case "canceled", "cancelled":
		return payments.StatusCanceled
	case "":
		return payments.StatusPending
	default:
		// Noma'lum status — "to'landi" deb qabul qilinmaydi. Yangi
		// status paydo bo'lsa, u pulni tasdiqlangan deb hisoblashdan
		// ko'ra muvaffaqiyatsiz deb qaralgani xavfsizroq.
		return payments.StatusFailed
	}
}

// ── Callback imzosi ────────────────────────────────────────────────

// Callback — Octo `notify_url` ga yuboradigan xabar
// (`image/octo/Уведомления.md`).
type Callback struct {
	ShopTransactionID string      `json:"shop_transaction_id"`
	OctoPaymentUUID   string      `json:"octo_payment_UUID"`
	Status            string      `json:"status"`
	Signature         string      `json:"signature"`
	HashKey           string      `json:"hash_key"`
	TotalSum          json.Number `json:"total_sum"`
	RefundedSum       json.Number `json:"refunded_sum"`
	PayedTime         string      `json:"payed_time"`
	MaskedPan         string      `json:"maskedPan"`
	CardVendor        string      `json:"card_vendor"`
	// AcceptStatus — ikki bosqichli to'lovda Octo "summani
	// tasdiqlaysizmi?" deb so'raganda keladi.
	AcceptStatus string `json:"accept_status"`
}

// VerifyCallbackSignature — callback imzosini tekshiradi:
// `sha1(unique_key, uuid, status)` (`image/octo/Уведомления.md`).
//
// ┌─ NEGA BU "YAGONA HIMOYA" EMAS ────────────────────────────────────┐
// Hujjatda konkatenatsiya formati (ajratuvchi bor-yo'qligi, tartib)
// ANIQ yozilmagan va `unique_key` ni Octo texnik jamoasi alohida
// beradi. Shuning uchun tizim imzoga TAYANMAYDI:
//
//	callback kelganda holat HAR DOIM Octo API'sidan qayta so'raladi
//	(`Status`) va buyurtma faqat SHU javob asosida "to'landi" bo'ladi.
//
// Ya'ni imzo — ikkinchi qatlam. Kalit sozlanmagan bo'lsa `false, false`
// qaytadi ("tekshirilmadi"), sozlangan bo'lsa natija ishonchli.
// └───────────────────────────────────────────────────────────────────┘
//
// Qaytaradi: (mos keldimi, umuman tekshirildimi).
func (c *Client) VerifyCallbackSignature(cb Callback) (ok bool, checked bool) {
	key := strings.TrimSpace(c.cfg.SignatureKey)
	if key == "" || cb.Signature == "" {
		return false, false
	}
	sum := sha1.Sum([]byte(key + cb.OctoPaymentUUID + cb.Status))
	want := hex.EncodeToString(sum[:])
	got := strings.ToLower(strings.TrimSpace(cb.Signature))
	// `subtle` — imzoni belgima-belgi solishtirishda vaqt bo'yicha
	// sizib chiqishning oldini oladi.
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1, true
}
