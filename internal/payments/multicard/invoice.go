package multicard

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // imzo algoritmini Multicard belgilaydi, tanlov bizda emas
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// TO'LOV YARATISH VA QAYTARISH.
//
// ┌─ BIR BOSQICHLI, IKKI BOSQICHLI EMAS ──────────────────────────────┐
// `POST /payment/invoice` da HOLD YO'Q — pulni bloklab, keyin yechish
// imkoni yo'q (hujjatning butun `invoice` bo'limida bunday maydon
// umuman mavjud emas; hold faqat karta tokeni bilan ishlaydigan
// alohida oqimda, u esa karta ma'lumotlarini o'zimiz qabul qilishni
// talab qiladi — PCI yuki).
//
// Oqibati: mijoz to'lagan zahoti pul restoran hisobiga TUSHADI.
// Restoran buyurtmani rad etsa, pul QAYTARILADI
// (`DELETE /payment/{uuid}/partial`). Octo'dagi "blokni bo'shatish"
// modeli bu yerda ishlamaydi.
// └───────────────────────────────────────────────────────────────────┘

// OFDItem — fiskal chek satri.
//
// ⚠️ `ofd` MAJBURIY: usiz invoys yaratilmaydi. `mxik` va
// `package_code` — tasnif.soliq.uz kataloglaridan; ular kassa
// sozlamasida standart qiymat sifatida turadi (`StoreInfo.TaxMXIK`)
// va ulash paytida o'qib olinadi.
type OFDItem struct {
	Name string `json:"name"`
	Qty  int    `json:"qty"`
	// Price — BITTA dona narxi (tiyin).
	Price int64 `json:"price"`
	// Total — qatorning umumiy summasi (tiyin).
	Total       int64  `json:"total"`
	MXIK        string `json:"mxik"`
	PackageCode string `json:"package_code"`
	// VAT — QQS foizi. 0 bo'lsa yuborilmaydi (soliq to'lovchi
	// bo'lmagan restoranda 0 va "yo'q" bir xil emas).
	VAT *int `json:"vat,omitempty"`
}

// InvoiceRequest — `POST /payment/invoice`.
type InvoiceRequest struct {
	StoreID     string
	AmountTiyin int64
	// InvoiceID — BIZNING tomondagi identifikator. Callback aynan shu
	// bilan qaytadi va takroriy to'lovdan himoya qiladi.
	InvoiceID   string
	CallbackURL string
	OFD         []OFDItem

	Lang           string
	ReturnURL      string
	ReturnErrorURL string
	// TTLSeconds — havolaning yashash muddati (0 — shlyuz standarti,
	// bir kun).
	TTLSeconds int
	// Phone — `998XXXXXXXXX`. Berilsa shlyuz SMS yuboradi.
	Phone string
}

// invoiceBody — JSON tanasi. Alohida tur: ixtiyoriy maydonlar bo'sh
// bo'lsa umuman yuborilmasligi kerak.
type invoiceBody struct {
	StoreID        string    `json:"store_id"`
	Amount         int64     `json:"amount"`
	InvoiceID      string    `json:"invoice_id"`
	CallbackURL    string    `json:"callback_url"`
	OFD            []OFDItem `json:"ofd"`
	Lang           string    `json:"lang,omitempty"`
	ReturnURL      string    `json:"return_url,omitempty"`
	ReturnErrorURL string    `json:"return_error_url,omitempty"`
	TTL            int       `json:"ttl,omitempty"`
	SMS            string    `json:"sms,omitempty"`
}

// Invoice — yaratilgan to'lov.
type Invoice struct {
	UUID string `json:"uuid"`
	// CheckoutURL — to'lov sahifasi (karta + Payme/Click/Uzum va h.k.).
	CheckoutURL string `json:"checkout_url"`
	// ShortLink — QR uchun qisqa havola.
	ShortLink string `json:"short_link"`
	// Deeplink — mobil ilovaga to'g'ridan-to'g'ri havola.
	Deeplink string `json:"deeplink"`
}

// PayURL — mijozga beriladigan eng mos havola.
//
// `checkout_url` afzal: unda BARCHA to'lov usullari bor. `deeplink`
// bitta ilovaga olib boradi va u o'rnatilmagan bo'lsa mijoz tuzoqqa
// tushadi.
func (i *Invoice) PayURL() string {
	if i == nil {
		return ""
	}
	if i.CheckoutURL != "" {
		return i.CheckoutURL
	}
	if i.ShortLink != "" {
		return i.ShortLink
	}
	return i.Deeplink
}

// ErrOFDRequired — fiskal ma'lumotsiz to'lov yaratib bo'lmaydi.
var ErrOFDRequired = errors.New("multicard: fiskal chek ma'lumoti (ofd) bo'sh")

// CreateInvoice — to'lov sahifasini yaratadi.
func (c *Client) CreateInvoice(ctx context.Context, req InvoiceRequest) (*Invoice, error) {
	store := strings.TrimSpace(req.StoreID)
	switch {
	case store == "":
		return nil, errors.New("multicard: store_id bo'sh")
	case req.AmountTiyin <= 0:
		return nil, errors.New("multicard: summa noto'g'ri")
	case strings.TrimSpace(req.InvoiceID) == "":
		return nil, errors.New("multicard: invoice_id bo'sh")
	case strings.TrimSpace(req.CallbackURL) == "":
		// Callback'siz to'lov yaratish — pulni ko'r-ko'rona olish:
		// natijani hech qachon bilmaymiz.
		return nil, errors.New("multicard: callback_url bo'sh")
	case len(req.OFD) == 0:
		return nil, ErrOFDRequired
	}
	// ⚠️ Fiskal chek summasi to'lov summasiga TENG bo'lishi kerak.
	// Farq qolsa soliq idorasidagi chek haqiqiy to'lovdan boshqa
	// bo'ladi — buni shlyuz tekshirmaydi, biz tekshiramiz.
	var sum int64
	for _, it := range req.OFD {
		sum += it.Total
	}
	if sum != req.AmountTiyin {
		return nil, fmt.Errorf(
			"multicard: fiskal chek summasi (%d) to'lov summasiga (%d) teng emas",
			sum, req.AmountTiyin)
	}

	body := invoiceBody{
		StoreID:        store,
		Amount:         req.AmountTiyin,
		InvoiceID:      strings.TrimSpace(req.InvoiceID),
		CallbackURL:    strings.TrimSpace(req.CallbackURL),
		OFD:            req.OFD,
		Lang:           strings.TrimSpace(req.Lang),
		ReturnURL:      strings.TrimSpace(req.ReturnURL),
		ReturnErrorURL: strings.TrimSpace(req.ReturnErrorURL),
		TTL:            req.TTLSeconds,
		SMS:            strings.TrimSpace(req.Phone),
	}
	var out Invoice
	if err := c.do(ctx, http.MethodPost, "/payment/invoice", body, &out); err != nil {
		return nil, err
	}
	if out.UUID == "" {
		return nil, errors.New("multicard: javobda to'lov identifikatori yo'q")
	}
	return &out, nil
}

// ── To'g'ridan-to'g'ri xizmat orqali to'lov ─────────────────────────

// Payment system kodlari (`paymentServiceEnum`).
//
// ⚠️ Faqat ILOVA ochiladigan xizmatlar sanab chiqilgan. Karta
// (`uzcard`, `humo`, `visa`, `mastercard`) bu yo'l bilan ketmaydi:
// ularda karta ma'lumotlari kerak, ya'ni universal to'lov sahifasi
// (`CreateInvoice`) ishlatiladi.
const (
	PSPayme = "payme"
	PSClick = "click"
	PSUzum  = "uzum"
)

// AppPaymentSystems — mijoz ilovasida alohida tugma sifatida
// ko'rsatiladigan xizmatlar.
var AppPaymentSystems = []string{PSPayme, PSClick, PSUzum}

// IsAppPaymentSystem — kod ro'yxatda bormi.
func IsAppPaymentSystem(ps string) bool {
	s := strings.ToLower(strings.TrimSpace(ps))
	for _, v := range AppPaymentSystems {
		if v == s {
			return true
		}
	}
	return false
}

// DirectRequest — `POST /payment`: mijoz AYNAN bitta xizmatni
// tanlagan (Payme / Click / Uzum).
//
// ┌─ INVOYSDAN FARQI ─────────────────────────────────────────────────┐
// `CreateInvoice` universal sahifa beradi: mijoz u yerda usulni O'ZI
// tanlaydi. Bu esa to'g'ridan-to'g'ri o'sha ilovaga havola
// (Universal/App Link) qaytaradi — bitta ortiqcha qadam yo'qoladi.
//
// Shuning uchun ikkalasi ham kerak: "Bank kartasi" → invoys,
// "Payme/Click/Uzum" → shu.
// └───────────────────────────────────────────────────────────────────┘
type DirectRequest struct {
	PaymentSystem string
	StoreID       string
	AmountTiyin   int64
	InvoiceID     string
	CallbackURL   string
	ReturnURL     string
	OFD           []OFDItem
}

type directBody struct {
	PaymentSystem string    `json:"payment_system"`
	StoreID       string    `json:"store_id"`
	Amount        int64     `json:"amount"`
	InvoiceID     string    `json:"invoice_id"`
	CallbackURL   string    `json:"callback_url,omitempty"`
	ReturnURL     string    `json:"return_url,omitempty"`
	OFD           []OFDItem `json:"ofd,omitempty"`
}

// CreatePayment — tanlangan xizmat orqali to'lov yaratadi.
func (c *Client) CreatePayment(ctx context.Context, req DirectRequest) (*Invoice, error) {
	ps := strings.ToLower(strings.TrimSpace(req.PaymentSystem))
	store := strings.TrimSpace(req.StoreID)
	switch {
	case !IsAppPaymentSystem(ps):
		return nil, fmt.Errorf("multicard: to'lov tizimi qo'llanmaydi: %q", req.PaymentSystem)
	case store == "":
		return nil, errors.New("multicard: store_id bo'sh")
	case req.AmountTiyin <= 0:
		return nil, errors.New("multicard: summa noto'g'ri")
	case strings.TrimSpace(req.InvoiceID) == "":
		return nil, errors.New("multicard: invoice_id bo'sh")
	case strings.TrimSpace(req.CallbackURL) == "":
		return nil, errors.New("multicard: callback_url bo'sh")
	}
	// `ofd` bu yerda ixtiyoriy, lekin berilgan bo'lsa summasi to'g'ri
	// bo'lishi SHART — chek bilan to'lov farq qilib qolmasin.
	if len(req.OFD) > 0 {
		var sum int64
		for _, it := range req.OFD {
			sum += it.Total
		}
		if sum != req.AmountTiyin {
			return nil, fmt.Errorf(
				"multicard: fiskal chek summasi (%d) to'lov summasiga (%d) teng emas",
				sum, req.AmountTiyin)
		}
	}
	body := directBody{
		PaymentSystem: ps,
		StoreID:       store,
		Amount:        req.AmountTiyin,
		InvoiceID:     strings.TrimSpace(req.InvoiceID),
		CallbackURL:   strings.TrimSpace(req.CallbackURL),
		ReturnURL:     strings.TrimSpace(req.ReturnURL),
		OFD:           req.OFD,
	}
	var out Invoice
	if err := c.do(ctx, http.MethodPost, "/payment", body, &out); err != nil {
		return nil, err
	}
	if out.UUID == "" {
		return nil, errors.New("multicard: javobda to'lov identifikatori yo'q")
	}
	return &out, nil
}

// AppURL — ilovaga olib boradigan havola.
//
// Bu yerda `deeplink` AFZAL: mijoz allaqachon xizmatni tanlagan, uni
// yana umumiy sahifaga yuborish ortiqcha qadam bo'lardi.
func (i *Invoice) AppURL() string {
	if i == nil {
		return ""
	}
	if i.Deeplink != "" {
		return i.Deeplink
	}
	return i.PayURL()
}

// PaymentInfo — `GET /payment/{uuid}`.
//
// Callback'ga ISHONMASDAN holatni shlyuzning o'zidan so'rash uchun:
// callback ochiq internetdan keladi, bu esa avtorizatsiyalangan
// so'rov (`payments.Service` qoidasi).
type PaymentInfo struct {
	UUID   string `json:"uuid"`
	Status string `json:"status"`
	PS     string `json:"ps"`
	// InvoiceID — bizning identifikatorimiz. Shlyuz uni ba'zi
	// javoblarda `store_invoice_id` deb ataydi.
	InvoiceID       string `json:"invoice_id"`
	StoreInvoiceID  string `json:"store_invoice_id"`
	AmountTiyin     Tiyin  `json:"payment_amount"`
	AmountAlt       Tiyin  `json:"amount"`
	CommissionTiyin Tiyin  `json:"commission_amount"`
	PaymentTime     string `json:"payment_time"`
	RefundTime      string `json:"refund_time"`
	CardPAN         string `json:"card_pan"`
}

// Amount — summa qaysi maydonda kelganidan qat'i nazar.
//
// Reestrda `payment_amount`, invoys javobida `amount` — ikkalasi ham
// uchraydi va qaysi biri kelishini oldindan aytib bo'lmaydi.
func (p *PaymentInfo) Amount() int64 {
	if p == nil {
		return 0
	}
	if v := p.AmountTiyin.Int64(); v != 0 {
		return v
	}
	return p.AmountAlt.Int64()
}

// Invoice — bizning identifikatorimiz (ikki nomdan qaysi biri kelsa).
func (p *PaymentInfo) Invoice() string {
	if p == nil {
		return ""
	}
	if p.InvoiceID != "" {
		return p.InvoiceID
	}
	return p.StoreInvoiceID
}

// Payment — to'lovning HAQIQIY holati.
func (c *Client) Payment(ctx context.Context, uuid string) (*PaymentInfo, error) {
	id := strings.TrimSpace(uuid)
	if id == "" {
		return nil, errors.New("multicard: to'lov identifikatori bo'sh")
	}
	var out PaymentInfo
	if err := c.get(ctx, "/payment/"+url.PathEscape(id), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelInvoice — TO'LANMAGAN invoysni bekor qiladi: mijoz havolani keyin
// ochsa ham to'lay olmaydi.
//
// ┌─ ENDPOINT: `/payment/invoice/{uuid}`, `/payment/{uuid}` EMAS ──────┐
// Hujjatga ko'ra `DELETE /payment/invoice/{uuid}` "to'lanmagan invoysni
// annulyatsiya qiladi". `DELETE /payment/{uuid}` esa BOSHQA amal: u
// to'langan to'lovni bekor qilish (qaytarish). Avval kod aynan shu
// ikkinchisini chaqirardi, sandbox'dagi haqiqiy sinov esa to'lanmagan
// invoysda `404 ERROR_NOT_FOUND` berdi: ya'ni yopilmasdi va mijoz
// bekor qilingan buyurtma uchun keyin to'lab qo'yishi mumkin edi.
// └────────────────────────────────────────────────────────────────────┘
//
// ⚠️ Allaqachon to'langan invoysda shlyuz xato qaytaradi (HTTP 400) —
// pulni qaytarish uchun `RefundPartial` ishlatiladi.
func (c *Client) CancelInvoice(ctx context.Context, uuid string) error {
	id := strings.TrimSpace(uuid)
	if id == "" {
		return errors.New("multicard: invoys identifikatori bo'sh")
	}
	return c.do(ctx, http.MethodDelete, "/payment/invoice/"+url.PathEscape(id), nil, nil)
}

// RefundPartial — to'langan pulni qaytaradi (to'liq yoki qisman).
//
// ⚠️ `ofd` bu yerda ham MAJBURIY: eski chek bekor qilinib, yangisi
// chiqariladi. Shuning uchun qaytarilayotgan summa bo'yicha chek
// satrlari berilishi kerak.
func (c *Client) RefundPartial(ctx context.Context, uuid string, amountTiyin int64, ofd []OFDItem) error {
	id := strings.TrimSpace(uuid)
	switch {
	case id == "":
		return errors.New("multicard: to'lov identifikatori bo'sh")
	case amountTiyin <= 0:
		return errors.New("multicard: qaytarish summasi noto'g'ri")
	case len(ofd) == 0:
		return ErrOFDRequired
	}
	body := struct {
		RefundAmount int64     `json:"refund_amount"`
		OFD          []OFDItem `json:"ofd"`
	}{RefundAmount: amountTiyin, OFD: ofd}
	return c.do(ctx, http.MethodDelete, "/payment/"+url.PathEscape(id)+"/partial", body, nil)
}

// ── Callback ────────────────────────────────────────────────────────

// Callback — shlyuz yuboradigan xabar.
type Callback struct {
	UUID        string `json:"uuid"`
	Amount      Tiyin  `json:"amount"`
	InvoiceID   string `json:"invoice_id"`
	Status      string `json:"status"`
	BillingID   string `json:"billing_id"`
	PaymentTime string `json:"payment_time"`
	RefundTime  string `json:"refund_time"`
	Phone       string `json:"phone"`
	CardPAN     string `json:"card_pan"`
	PS          string `json:"ps"`
	ReceiptURL  string `json:"receipt_url"`
	Sign        string `json:"sign"`
}

// CallbackSourceIP — shlyuz callback yuboradigan manzil (hujjatda
// ko'rsatilgan).
//
// ⚠️ Bu QO'SHIMCHA qatlam, ASOSIY himoya EMAS: IP o'zgarishi mumkin va
// proksi orqasida ishonchsiz. Haqiqiy tekshiruv — imzo.
const CallbackSourceIP = "195.158.26.90"

// VerifyCallback — imzoni tekshiradi.
//
// ┌─ IMZODA VAQT YO'Q ────────────────────────────────────────────────┐
// Formula: `SHA1(uuid + invoice_id + amount + secret)`. Unda na vaqt,
// na bir martalik son bor — ya'ni bir marta ushlangan xabarni keyin
// QAYTA yuborish mumkin va imzo baribir to'g'ri chiqadi.
//
// Shuning uchun imzo YAGONA himoya bo'la olmaydi: chaqiruvchi `uuid`
// bo'yicha idempotent bo'lishi va holatni shlyuzdan qayta so'rashi
// SHART (`payments.Service` shunday qiladi).
//
// Solishtirish doimiy vaqtda: aks holda javob vaqti bo'yicha imzoni
// belgima-belgi topish mumkin bo'lardi.
// └───────────────────────────────────────────────────────────────────┘
func (c *Client) VerifyCallback(cb Callback) bool {
	if c == nil || cb.Sign == "" {
		return false
	}
	return verifySign(cb, c.cfg.Secret)
}

func verifySign(cb Callback, secret string) bool {
	if secret == "" {
		return false
	}
	raw := cb.UUID + cb.InvoiceID + strconv.FormatInt(cb.Amount.Int64(), 10) + secret
	sum := sha1.Sum([]byte(raw)) //nolint:gosec // algoritmni shlyuz belgilaydi
	want := hex.EncodeToString(sum[:])
	return hmac.Equal([]byte(strings.ToLower(strings.TrimSpace(cb.Sign))), []byte(want))
}
