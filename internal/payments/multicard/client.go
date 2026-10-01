// Package multicard — Multicard (Rahmat) to'lov shlyuzi bilan ishlash.
//
// ┌─ RAHMAT VA MULTICARD — BITTA TIZIM ───────────────────────────────┐
// Multicard — savdogar tomoni va API; Rahmat (`app.rhmt.uz`) — o'sha
// tizimning mijoz ko'radigan to'lov sahifasi va ilovasi. Hujjat:
// https://docs.multicard.uz/
// └───────────────────────────────────────────────────────────────────┘
//
// ┌─ OCTO'DAN MUSTAQIL ───────────────────────────────────────────────┐
// Bu paket `internal/payments/octo` ni bilmaydi va aksincha. Umumiy
// narsa faqat `payments.Provider` interfeysi, ya'ni bittasi o'chsa
// ikkinchisi ishlayveradi.
// └───────────────────────────────────────────────────────────────────┘
//
// Hozircha FAQAT O'QISH: restoran Moliya sahifasi uchun tushum
// reestri. To'lov yaratish keyingi bosqichda — u shartnoma va
// `merchant_account` savollariga javob kelgandan keyin qo'shiladi.
package multicard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config — platforma darajasidagi kalitlar.
//
// ⚠️ `ApplicationID`/`Secret` IKKI manbadan kelishi mumkin:
//
//  1. OnDex'ning platforma kalitlari (`.env`, `MULTICARD_*`) — OnDex
//     shartnomasi ostidagi restoranlar uchun;
//  2. RESTORANNING O'Z kalitlari — u Multicard bilan o'zi shartnoma
//     tuzgan bo'lsa. Ular bazada SHIFRLANGAN holda yotadi
//     (`internal/secretstore`) va hech qachon panelga qaytarilmaydi.
//
// Ikkinchi holatda pul to'g'ridan-to'g'ri restoranning o'z bank
// hisobiga tushadi va OnDex oraliqda turmaydi.
type Config struct {
	BaseURL       string
	ApplicationID string
	Secret        string
	// HTTP — testda almashtiriladi. `nil` bo'lsa standart klient
	// (timeout bilan) ishlatiladi.
	HTTP *http.Client
	// Now — testda vaqtni boshqarish uchun.
	Now func() time.Time
}

// Client — Multicard API klienti.
//
// Token ICHKI keshlanadi: har so'rov uchun `POST /auth` chaqirish
// keraksiz tarmoq sayohati va ortiqcha yuk bo'lardi.
type Client struct {
	cfg  Config
	http *http.Client
	now  func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

// ProdHost / SandboxHost — shlyuzning jonli va sinov manzillari.
//
// ┌─ NEGA KODDA ──────────────────────────────────────────────────────┐
// Multicard'da Octo'dagi `OCTO_TEST` kabi ALOHIDA bayroq YO'Q: sinov
// va jonli muhit faqat MANZIL bilan ajraladi. Ya'ni `.env` dagi
// bitta satr xato yozilsa:
//
//   - production sinov manziliga qarasa — mijoz "to'ladi", pul
//     OLINMAYDI, buyurtma esa restoranga ketaveradi;
//   - dev jonli manzilga qarasa — sinov paytida HAQIQIY pul yechiladi.
//
// Ikkalasi ham jimgina sodir bo'ladi. `IsSandbox` shu farqni kodga
// ma'lum qiladi va server ishga tushganda ogohlantiradi
// (`cmd/api/main.go`) — xato deploy paytida ko'rinadi, birinchi
// to'lovda emas.
// └───────────────────────────────────────────────────────────────────┘
const (
	ProdHost    = "mesh.multicard.uz"
	SandboxHost = "dev-mesh.multicard.uz"
)

// IsSandbox — manzil SINOV stendimi.
//
// Noma'lum manzil (masalan mahalliy soxta server) ham sinov deb
// qaraladi: fail-safe tomoni — "jonli" deb xato hisoblashdan ko'ra
// ortiqcha ogohlantirish yaxshi.
func IsSandbox(baseURL string) bool {
	h := strings.ToLower(strings.TrimSpace(baseURL))
	h = strings.TrimPrefix(strings.TrimPrefix(h, "https://"), "http://")
	h = strings.TrimSuffix(h, "/")
	if i := strings.IndexAny(h, "/:"); i >= 0 {
		h = h[:i]
	}
	return h != ProdHost
}

// DefaultTimeout — shlyuzga bitta so'rovning eng uzun muddati.
//
// ┌─ NEGA MIJOZ ILOVASINIKIDAN ANCHA QISQA ───────────────────────────┐
// Timeout SHART: `package http` da standarti YO'Q va yarim ochiq
// ulanish so'rovni abadiy osib qo'yardi.
//
// Lekin UZUNLIGI ham muhim. Mijoz ilovasining HTTP muddati 20 soniya
// (`ondex_core.ApiClient`). Bu yerda ham 20 bo'lsa poyga chiqadi:
// shlyuz javob bermasa server 20 soniyadan KEYIN 502 qaytaradi,
// ilova esa AYNAN 20-soniyada taslim bo'lib "Serverga ulanib
// bo'lmadi — internetni tekshiring" deydi. Ya'ni foydalanuvchi
// internetini ayblaydi, aslida esa shlyuz o'chiq va server buni
// aniq aytmoqchi edi.
//
// To'lov yaratishda eng ko'pi ikki so'rov ketadi (`/auth` +
// `/payment/invoice`), ya'ni eng yomon holat 2×8 = 16 soniya —
// ilovaning 20 soniyasidan ichkarida. Shlyuz 8 soniyada javob
// bermasa, u amalda ishlamayapti.
// └───────────────────────────────────────────────────────────────────┘
const DefaultTimeout = 8 * time.Second

var (
	// ErrNotConfigured — kalitlar berilmagan (funksiya o'chiq).
	ErrNotConfigured = errors.New("multicard: MULTICARD_APPLICATION_ID/SECRET berilmagan")
	// ErrAuth — kalitlar noto'g'ri yoki rad etilgan.
	ErrAuth = errors.New("multicard: autentifikatsiya rad etildi")
)

// New — klient. Kalitlar bo'sh bo'lsa `ErrNotConfigured`.
func New(cfg Config) (*Client, error) {
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.ApplicationID = strings.TrimSpace(cfg.ApplicationID)
	cfg.Secret = strings.TrimSpace(cfg.Secret)
	if cfg.BaseURL == "" || cfg.ApplicationID == "" || cfg.Secret == "" {
		return nil, ErrNotConfigured
	}
	c := &Client{cfg: cfg, http: cfg.HTTP, now: cfg.Now}
	if c.http == nil {
		c.http = &http.Client{Timeout: DefaultTimeout}
	}
	if c.now == nil {
		c.now = time.Now
	}
	return c, nil
}

// ── Javob qobig'i ───────────────────────────────────────────────────
//
// Multicard HAR javobni bir xil shaklda qaytaradi:
//
//	{"success":true,"data":{...}}
//	{"success":false,"error":{"code":"...","details":"..."}}
//
// ⚠️ HTTP holati 200 bo'lsa ham `success:false` kelishi mumkin —
// shuning uchun faqat status kodga ishonib bo'lmaydi.
type envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Details string `json:"details"`
	} `json:"error"`
}

// APIError — Multicard qaytargan xato.
type APIError struct {
	Code    string
	Details string
	Status  int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("multicard: %s (%s, HTTP %d)", e.Details, e.Code, e.Status)
}

// ── Autentifikatsiya ────────────────────────────────────────────────

type authResponse struct {
	Token  string `json:"token"`
	Role   string `json:"role"`
	Expiry string `json:"expiry"` // "2026-10-01 17:49:56", GMT+5
}

// tashkentZone — Multicard `expiry` ni GMT+5 da qaytaradi va mintaqa
// belgisisiz. Mahalliy vaqt deb o'qilsa server boshqa mintaqada
// turganda muddat noto'g'ri hisoblanardi.
var tashkentZone = time.FixedZone("UTC+5", 5*60*60)

// token — keshdan yoki yangisini oladi.
//
// Muddat 60 soniya ZAXIRA bilan tekshiriladi: so'rov yo'lda ketayotib
// token eskirib qolmasin.
func (c *Client) token_(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.token != "" && c.now().Add(time.Minute).Before(c.expires) {
		t := c.token
		c.mu.Unlock()
		return t, nil
	}
	c.mu.Unlock()

	body, _ := json.Marshal(map[string]string{
		"application_id": c.cfg.ApplicationID,
		"secret":         c.cfg.Secret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/auth", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		// ⚠️ Xato matnida SIR bo'lmasligi kerak — javobning o'zi
		// logga tushishi mumkin.
		return "", fmt.Errorf("%w (HTTP %d)", ErrAuth, res.StatusCode)
	}
	var a authResponse
	if err := json.Unmarshal(raw, &a); err != nil || a.Token == "" {
		return "", ErrAuth
	}

	exp := c.now().Add(30 * time.Minute) // hujjat muddat bermasa — ehtiyotkor standart
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", a.Expiry, tashkentZone); err == nil {
		exp = t
	}
	c.mu.Lock()
	c.token, c.expires = a.Token, exp
	c.mu.Unlock()
	return a.Token, nil
}

// get — avtorizatsiyalangan GET va qobiqni ochish.
func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// do — avtorizatsiyalangan so'rov va qobiqni ochish.
//
// `body` `nil` bo'lmasa JSON sifatida yuboriladi. Multicard `DELETE`
// da ham tana kutadi (qisman qaytarish), shuning uchun metod alohida
// parametr.
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	tok, err := c.token_(ctx)
	if err != nil {
		return err
	}
	var rdr io.Reader
	if body != nil {
		b, merr := json.Marshal(body)
		if merr != nil {
			return merr
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("multicard: javobni o'qib bo'lmadi (HTTP %d)", res.StatusCode)
	}
	if !env.Success {
		e := &APIError{Status: res.StatusCode}
		if env.Error != nil {
			e.Code, e.Details = env.Error.Code, env.Error.Details
		}
		return e
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

// ── Tushum reestri ──────────────────────────────────────────────────

// Tiyin — pul summasi, SON ham SATR ham bo'lishi mumkin.
//
// ┌─ NEGA MAXSUS TUR KERAK ───────────────────────────────────────────┐
// Multicard bir xil maydonni ikki xil turda qaytaradi va bu hujjatda
// yozilmagan — sinov stendida o'lchandi (2026-09-30):
//
//	data.stat[].payment_amount  -> "29333474890"  (SATR)
//	data.list[].payment_amount  -> 7480000        (SON)
//
// Oddiy `int64` bilan reestr `json: cannot unmarshal string into
// int64` bilan yiqilardi. Bu xatoni faqat jonli so'rov ko'rsatdi:
// soxta serverdagi javob son edi.
//
// `null` ham keladi (masalan `total_amount`) — u NOL deb qaraladi.
// └───────────────────────────────────────────────────────────────────┘
type Tiyin int64

func (t *Tiyin) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == `""` || s == "" {
		*t = 0
		return nil
	}
	s = strings.Trim(s, `"`)
	// Pul HAR DOIM butun tiyinda — kasr kutilmaydi va uni jimgina
	// yaxlitlash summani buzardi.
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("multicard: summa o'qilmadi (%s): %w", s, err)
	}
	*t = Tiyin(n)
	return nil
}

// Int64 — oddiy songa o'girish.
func (t Tiyin) Int64() int64 { return int64(t) }

// Status — tranzaksiya holati (`paymentStatusEnum`).
const (
	StatusSuccess   = "success"
	StatusCancelled = "cancelled"
	StatusError     = "error"
	StatusRevert    = "revert"
	StatusHold      = "hold"
	StatusDraft     = "draft"
	StatusBilling   = "billing"
	StatusProgress  = "progress"
)

// Transaction — reestrdagi bitta yozuv.
type Transaction struct {
	UUID string `json:"uuid"`
	// Status — `success` dan boshqasi PUL TUSHGANINI bildirmaydi.
	Status string `json:"status"`
	// PS — to'lov tizimi (uzcard, humo, payme, click, ...).
	PS string `json:"ps"`
	// StoreInvoiceID — BIZNING tomondagi identifikator (buyurtma).
	StoreInvoiceID string `json:"store_invoice_id"`
	// AmountTiyin — to'lov summasi (tiyin).
	AmountTiyin Tiyin `json:"payment_amount"`
	// CommissionTiyin — Multicard ushlagan komissiya (tiyin).
	// Foiz do'kon shartnomasiga bog'liq — kodda qadalmaydi.
	CommissionTiyin Tiyin  `json:"commission_amount"`
	CardPAN         string `json:"card_pan"`
	PaymentTime     string `json:"payment_time"`
	RefundTime      string `json:"refund_time"`
}

// StatRow — holat × to'lov tizimi bo'yicha jamlanma.
type StatRow struct {
	Status      string `json:"status"`
	PS          string `json:"ps"`
	AmountTiyin Tiyin  `json:"payment_amount"`
}

// History — `GET /payment/store/{id}/history` javobi.
type History struct {
	Pagination struct {
		Total  int `json:"total"`
		Offset int `json:"offset"`
		Limit  int `json:"limit"`
	} `json:"pagination"`
	Stat []StatRow     `json:"stat"`
	List []Transaction `json:"list"`
}

// SuccessTiyin — MUVAFFAQIYATLI to'lovlar yig'indisi.
//
// ┌─ NEGA FAQAT `success` ─────────────────────────────────────────────┐
// Reestrda `draft`, `cancelled`, `error`, `hold` holatlari ham bor va
// ular summasi jami tushumdan ANCHA katta bo'lishi mumkin (sinov
// stendida `draft` yig'indisi `success` dan ko'p edi). Ularni qo'shib
// yuborish restoranga bir necha barobar oshirilgan "daromad"
// ko'rsatardi.
// └────────────────────────────────────────────────────────────────────┘
func (h History) SuccessTiyin() int64 {
	var sum Tiyin
	for _, s := range h.Stat {
		if s.Status == StatusSuccess {
			sum += s.AmountTiyin
		}
	}
	return sum.Int64()
}

// RefundedTiyin — qaytarilgan summa (`revert`).
func (h History) RefundedTiyin() int64 {
	var sum Tiyin
	for _, s := range h.Stat {
		if s.Status == StatusRevert {
			sum += s.AmountTiyin
		}
	}
	return sum.Int64()
}

// ── Do'kon (kassa) ma'lumoti ────────────────────────────────────────

// StoreInfo — `GET /payment/store/{id}` javobi (kerakli qismi).
//
// ┌─ NEGA BU KERAK ───────────────────────────────────────────────────┐
// Restoran kalitlarini kiritganda ularni SAQLASHDAN OLDIN tekshirish
// kerak. Faqat `POST /auth` yetarli emas: kalitlar to'g'ri bo'lib,
// do'kon raqami xato bo'lishi mumkin (aynan shunday bo'ldi — "65"
// kiritilib, xato faqat Moliya sahifasi ochilganda bilindi).
//
// Bundan tashqari javob PULNING QAYERGA tushishini aytadi: hisob
// raqami, tashkilot nomi, STIR. Restoran ulashni tasdiqlashdan oldin
// o'z rekvizitlarini ko'rishi kerak — noto'g'ri do'kon raqami
// kiritilsa pul BEGONA hisobga ketadi.
// └───────────────────────────────────────────────────────────────────┘
type StoreInfo struct {
	ID    int    `json:"id"`
	UUID  string `json:"uuid"`
	Title string `json:"title"`

	// ┌─ FISKAL STANDART QIYMATLAR ────────────────────────────────────┐
	// `ofd` har to'lovda MAJBURIY, uning ichida esa `mxik` va
	// `package_code` kerak. Har taomga alohida kod kiritish restoran
	// uchun og'ir bo'lardi, shuning uchun kassaning STANDART qiymati
	// ishlatiladi — u ulash paytida o'qib olinadi va saqlanadi.
	//
	// `TaxRegistration`: 0 — fiskallashtirish O'CHIQ.
	// └────────────────────────────────────────────────────────────────┘
	TaxMXIK         string `json:"tax_mxik"`
	TaxPackageCode  string `json:"tax_package_code"`
	TaxRegistration int    `json:"tax_registration"`

	Merchant struct {
		Name string `json:"name"`
		TIN  string `json:"tin"`
	} `json:"merchant"`

	// MerchantAccount — pul tushadigan bank rekvizitlari.
	MerchantAccount struct {
		AccountNo    string `json:"account_no"`
		MFO          string `json:"mfo"`
		OfficialName string `json:"official_name"`
	} `json:"merchant_account"`

	// Contract.Fee — komissiya foizi. ⚠️ Shartnomaga bog'liq va
	// do'kondan do'konga farq qiladi, shuning uchun kodda QADALMAYDI
	// (sinov stendida 1.5% o'lchandi, egasi 2% degan edi).
	Contract struct {
		Num Str `json:"num"`
		Fee struct {
			Up   Str `json:"up"`
			Down Str `json:"down"`
		} `json:"fee"`
	} `json:"contract"`
}

// Str — SON ham, SATR ham bo'lishi mumkin bo'lgan matnli maydon.
//
// `Tiyin` bilan bir xil sabab: Multicard bir maydonni ikki xil turda
// qaytaradi va bu hujjatda yozilmagan. Hujjatda `contract.fee.up`
// SATR deb ko'rsatilgan, sinov stendi esa SON qaytardi (2026-09-30 da
// o'lchandi) — qattiq `string` bilan butun ulash oqimi yiqilardi.
type Str string

func (s *Str) UnmarshalJSON(b []byte) error {
	t := strings.TrimSpace(string(b))
	if t == "null" || t == "" {
		*s = ""
		return nil
	}
	if t[0] == '"' {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*s = Str(v)
		return nil
	}
	*s = Str(t)
	return nil
}

func (s Str) String() string { return string(s) }

// Store — do'kon ma'lumoti. Kalitlar bu do'konga ega bo'lmasa shlyuz
// xato qaytaradi — aynan shu tekshiruv uchun ishlatiladi.
func (c *Client) Store(ctx context.Context, storeID string) (*StoreInfo, error) {
	id := strings.TrimSpace(storeID)
	if id == "" {
		return nil, errors.New("multicard: store_id bo'sh")
	}
	var s StoreInfo
	if err := c.get(ctx, "/payment/store/"+url.PathEscape(id), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// HistoryQuery — reestr so'rovi.
type HistoryQuery struct {
	StoreID string
	From    time.Time
	To      time.Time
	Offset  int
	Limit   int
	// OnlyStatus — bo'sh bo'lsa hamma holat.
	OnlyStatus string
}

// maxLimit — bitta so'rovda olinadigan eng ko'p yozuv.
const maxLimit = 200

// StoreHistory — do'kon bo'yicha tushum reestri.
//
// Sana formati `YYYY-mm-dd H:i:s`, GMT+5 — hujjat shuni talab qiladi.
func (c *Client) StoreHistory(ctx context.Context, q HistoryQuery) (*History, error) {
	store := strings.TrimSpace(q.StoreID)
	if store == "" {
		return nil, errors.New("multicard: store_id bo'sh")
	}
	limit := q.Limit
	if limit <= 0 || limit > maxLimit {
		limit = maxLimit
	}
	offset := max(q.Offset, 0)

	v := url.Values{}
	v.Set("offset", strconv.Itoa(offset))
	v.Set("limit", strconv.Itoa(limit))
	v.Set("start_date", q.From.In(tashkentZone).Format("2006-01-02 15:04:05"))
	v.Set("end_date", q.To.In(tashkentZone).Format("2006-01-02 15:04:05"))
	if s := strings.TrimSpace(q.OnlyStatus); s != "" {
		v.Set("only_status", s)
	}

	var h History
	// `store` yo'l qismiga kiradi — kutilmagan belgi manzilni buzmasin.
	path := "/payment/store/" + url.PathEscape(store) + "/history?" + v.Encode()
	if err := c.get(ctx, path, &h); err != nil {
		return nil, err
	}
	return &h, nil
}
