package qurtorders

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ┌─ DO'KON POS SERVERIGA JONLI SIGNAL ────────────────────────────────────┐
// Yangi buyurtma kelganda (yoki bekor qilinganda) do'konning POS serveriga
// (ShoxPos) IMZOLANGAN xabar yuboriladi: panelda ovoz va belgi chiqadi.
//
// Xavfsizlik:
//   * xabar HMAC-SHA256 bilan imzolanadi (umumiy sir ≥ 32 belgi); imzo vaqt
//     tamg'asini ham qamraydi, qabul qiluvchi eskirgan xabarni rad etadi;
//   * har xabarning yagona `event_id` si bor — qabul qiluvchi takrorni rad etadi;
//   * xabarda SHAXSIY MA'LUMOT YO'Q: mijoz ismi, telefoni, manzili yuborilmaydi —
//     faqat buyurtma raqami, holat va summa. Tafsilotni POS o'zi OnDex'dan so'raydi;
//   * manzil FAQAT sozlamadan (env) olinadi, so'rovdan emas; sir hech qachon logga yozilmaydi.
//
// Xabar yetib bormasa buyurtmaga hech narsa bo'lmaydi: panel baribir davriy
// so'raydi (polling), signal faqat tezlashtiradi.
// └────────────────────────────────────────────────────────────────────────┘

// MinWebhookSecretLen — umumiy sirning eng kam uzunligi.
const MinWebhookSecretLen = 32

// notifyEvents — do'konga bildiriladigan voqealar.
var notifyEvents = map[string]bool{"created": true, string(StatusCancelled): true}

// WebhookPayload — yuboriladigan tana (PII yo'q).
type WebhookPayload struct {
	Event      string `json:"event"`
	EventID    string `json:"event_id"`
	OrderID    string `json:"order_id"`
	Number     string `json:"number"`
	StoreID    string `json:"store_id"`
	Status     Status `json:"status"`
	TotalTiyin int64  `json:"total_tiyin"`
	At         int64  `json:"at"` // unix soniya
}

// POSNotifier — POS serveriga imzolangan xabar yuboradi.
type POSNotifier struct {
	url    string
	secret string
	client *http.Client
	// backoff — urinishlar orasidagi kutish (so'nggi urinishdan keyin kutilmaydi).
	backoff []time.Duration
	now     func() time.Time
}

// NewPOSNotifier — sozlamani tekshirib yaratadi. URL https bo'lishi shart
// (faqat localhost uchun http ruxsat), sir kamida MinWebhookSecretLen belgi.
func NewPOSNotifier(rawURL, secret string) (*POSNotifier, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return nil, errors.New("qurt webhook: manzil noto'g'ri")
	}
	host := u.Hostname()
	local := host == "localhost" || strings.HasSuffix(host, ".localhost") ||
		(net.ParseIP(host) != nil && net.ParseIP(host).IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return nil, errors.New("qurt webhook: manzil https bo'lishi shart (faqat localhost uchun http)")
	}
	if len(secret) < MinWebhookSecretLen {
		return nil, fmt.Errorf("qurt webhook: sir kamida %d belgi bo'lsin", MinWebhookSecretLen)
	}
	return &POSNotifier{
		url: u.String(), secret: secret,
		client:  &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		backoff: []time.Duration{time.Second, 4 * time.Second},
		now:     time.Now,
	}, nil
}

// Sign — `ts.body` ning HMAC-SHA256 imzosi (hex). Qabul qiluvchi ham shu formulani ishlatadi.
func Sign(secret, ts string, body []byte) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(ts))
	m.Write([]byte{'.'})
	m.Write(body)
	return hex.EncodeToString(m.Sum(nil))
}

// Notify — `Service.OnEvent` imzosi. Faqat do'konga kerak voqealar yuboriladi; yuborish
// fonda (buyurtma yaratish kutib turmaydi).
func (n *POSNotifier) Notify(_ context.Context, event string, o *Order) {
	if n == nil || !notifyEvents[event] || o == nil {
		return
	}
	p := WebhookPayload{
		Event: event, EventID: fmt.Sprintf("%s:%s:%d", o.ID, event, o.Version),
		OrderID: o.ID, Number: o.Number, StoreID: o.StoreID, Status: o.Status,
		TotalTiyin: o.TotalTiyin, At: n.now().Unix(),
	}
	body, err := json.Marshal(p)
	if err != nil {
		return
	}
	go n.deliver(p.EventID, body)
}

func (n *POSNotifier) deliver(eventID string, body []byte) {
	var last error
	for attempt := 0; ; attempt++ {
		last = n.post(body)
		if last == nil {
			return
		}
		if attempt >= len(n.backoff) {
			break
		}
		time.Sleep(n.backoff[attempt])
	}
	// Xato matnida sir/URL parametrlari yo'q (faqat holat); tafsilot uchun event_id yetadi.
	slog.Warn("qurt: POS ga xabar yetib bormadi (panel polling bilan o'zi ko'radi)", "event_id", eventID, "err", last)
}

func (n *POSNotifier) post(body []byte) error {
	ts := strconv.FormatInt(n.now().Unix(), 10)
	req, err := http.NewRequest(http.MethodPost, n.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ondex-Timestamp", ts)
	req.Header.Set("X-Ondex-Signature", "sha256="+Sign(n.secret, ts, body))
	res, err := n.client.Do(req)
	if err != nil {
		return errors.New("ulanib bo'lmadi")
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("POS %d qaytardi", res.StatusCode)
	}
	return nil
}
