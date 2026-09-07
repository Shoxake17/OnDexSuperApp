package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Eskiz.uz — O'zbekistondagi SMS provayderi.
//
// ── NEGA MAVJUD INTERFEYSGA TUSHADI ────────────────────────────────
// `users.SmsSender` allaqachon bor (`LogSms` — dev uchun). Bu yerdagi
// yagona vazifa — o'sha interfeysning haqiqiy implementatsiyasi.
// Shu sabab tasdiqlash mantiqi, kod yaratish, urinishlar chegarasi va
// tezlik cheklovi UMUMAN o'zgarmaydi: kod avvalgidek `CodeStore` ga
// tushadi va `POST /auth/verify` orqali tekshiriladi.
//
// ── AUTENTIFIKATSIYA ───────────────────────────────────────────────
// Eskiz email+parol evaziga JWT beradi va u ~30 kun amal qiladi.
// Token XOTIRADA saqlanadi va muddati tugaganda avtomatik yangilanadi
// — har SMS uchun qayta login qilish provayder tomonidan cheklangan
// va sekin bo'lardi.
//
// ── XAVFSIZLIK ─────────────────────────────────────────────────────
//   - Parol FAQAT `.env` da (`ESKIZ_PASSWORD`), kodda emas;
//   - Token va parol HECH QACHON log qilinmaydi;
//   - Xato matnida ham ular bo'lmaydi (faqat provayder xabari).

// ErrEskizNotConfigured — sozlanmagan.
var ErrEskizNotConfigured = errors.New("eskiz sozlanmagan (ESKIZ_EMAIL/ESKIZ_PASSWORD yo'q)")

type EskizSms struct {
	email    string
	password string
	from     string
	baseURL  string
	http     *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

// NewEskizFromEnv — `.env` dan o'qiydi. Sozlanmagan bo'lsa
// `configured=false` qaytadi va chaqiruvchi `LogSms` ga qaytadi.
func NewEskizFromEnv() (sender *EskizSms, configured bool) {
	email := strings.TrimSpace(os.Getenv("ESKIZ_EMAIL"))
	password := os.Getenv("ESKIZ_PASSWORD")
	if email == "" || password == "" {
		return nil, false
	}
	base := strings.TrimSpace(os.Getenv("ESKIZ_BASE_URL"))
	if base == "" {
		base = "https://notify.eskiz.uz/api"
	}
	from := strings.TrimSpace(os.Getenv("ESKIZ_FROM"))
	if from == "" {
		// 4546 — Eskiz'ning sinov jo'natuvchisi. Haqiqiy nom
		// (masalan "OnDex") operatorlarda alohida ro'yxatdan
		// o'tkaziladi va tasdiqlangach shu yerga yoziladi.
		from = "4546"
	}
	return &EskizSms{
		email:    email,
		password: password,
		from:     from,
		baseURL:  strings.TrimRight(base, "/"),
		http:     &http.Client{Timeout: 20 * time.Second},
	}, true
}

// Send — `users.SmsSender` interfeysi.
func (e *EskizSms) Send(phone, text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	token, err := e.ensureToken(ctx)
	if err != nil {
		return err
	}
	err = e.sendOnce(ctx, token, phone, text)
	if err == nil {
		return nil
	}
	// Token muddati tugagan bo'lishi mumkin — BIR marta yangilab
	// qayta urinamiz. Cheksiz sikl bo'lmasligi uchun aynan bir marta.
	if errors.Is(err, errUnauthorized) {
		e.invalidate()
		token, err2 := e.ensureToken(ctx)
		if err2 != nil {
			return err2
		}
		return e.sendOnce(ctx, token, phone, text)
	}
	return err
}

var errUnauthorized = errors.New("eskiz: token yaroqsiz")

func (e *EskizSms) invalidate() {
	e.mu.Lock()
	e.token = ""
	e.tokenExp = time.Time{}
	e.mu.Unlock()
}

func (e *EskizSms) ensureToken(ctx context.Context) (string, error) {
	e.mu.Lock()
	if e.token != "" && time.Now().Before(e.tokenExp) {
		t := e.token
		e.mu.Unlock()
		return t, nil
	}
	e.mu.Unlock()

	var buf bytes.Buffer
	mw := multipartForm(&buf, map[string]string{
		"email":    e.email,
		"password": e.password,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/auth/login", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw)
	resp, err := e.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("eskiz: login so'rovi: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		// PAROL LOG QILINMAYDI — faqat holat kodi.
		return "", fmt.Errorf("eskiz: login rad etildi (HTTP %d)", resp.StatusCode)
	}
	var out struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Data.Token == "" {
		return "", errors.New("eskiz: login javobida token yo'q")
	}

	e.mu.Lock()
	e.token = out.Data.Token
	// Eskiz tokeni ~30 kun yashaydi; ehtiyot uchun 25 kun deb
	// hisoblaymiz va undan oldin yangilaymiz.
	e.tokenExp = time.Now().Add(25 * 24 * time.Hour)
	e.mu.Unlock()
	return out.Data.Token, nil
}

func (e *EskizSms) sendOnce(ctx context.Context, token, phone, text string) error {
	// Eskiz raqamni "998901234567" ko'rinishida kutadi (plyussiz).
	msisdn := strings.TrimPrefix(strings.TrimSpace(phone), "+")

	var buf bytes.Buffer
	ct := multipartForm(&buf, map[string]string{
		"mobile_phone": msisdn,
		"message":      text,
		"from":         e.from,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/message/sms/send", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := e.http.Do(req)
	if err != nil {
		return fmt.Errorf("eskiz: yuborish so'rovi: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var out struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &out)
		msg := out.Message
		if msg == "" {
			msg = "HTTP " + fmt.Sprint(resp.StatusCode)
		}
		return fmt.Errorf("eskiz: %s", msg)
	}
	// SMS MATNI LOG QILINMAYDI — ichida bir martalik kod bor.
	slog.Info("sms yuborildi (eskiz)", "phone", phone)
	return nil
}

// multipartForm — Eskiz `multipart/form-data` kutadi.
//
// ┌─ TUZATILGAN NOSOZLIK (bug.md 13-band) ─────────────────────────────┐
// Chegara (`boundary`) qat'iy satr, qiymatlar esa buferga XOM holda
// yozilardi: na `\r\n`, na chegara satrining o'zi filtrlanardi.
// Qiymat ichida `\r\n----ondex-eskiz-boundary\r\n` bo'lsa, so'rovga
// qo'shimcha maydon kiritish mumkin edi (multipart inyeksiyasi).
//
// Amaldagi zarar yo'q edi — `message` serverda yasaladi
// (`users/service.go`), `mobile_phone` esa normalizatsiyadan o'tgan
// raqam. Lekin bu LATENT: kelajakda restoran nomi yoki foydalanuvchi
// kiritgan matn SMS'ga qo'shilsa, teshik darhol ochilardi.
//
// Loyihada bu qoida allaqachon bor: `notify/email.go` dagi
// `guardHeader` aynan shunday ishlaydi va izohida sababi yozilgan —
// *"Tekshiruv YUBORUVCHI QATLAMDA turadi: chaqiruvchi uni unutsa
// ham himoya ishlaydi"*. SMS qatlamida shu qoida qo'llanmagandi.
//
// Endi qiymatdan CR/LF olib tashlanadi — chegara satri faqat yangi
// qatordan keyin ma'noga ega, ya'ni CR/LF siz uni yasab bo'lmaydi.
// Maydon NOMI ham xuddi shunday tozalanadi.
// └────────────────────────────────────────────────────────────────────┘
func multipartForm(buf *bytes.Buffer, fields map[string]string) string {
	const boundary = "----ondex-eskiz-boundary"
	for k, v := range fields {
		fmt.Fprintf(buf, "--%s\r\n", boundary)
		fmt.Fprintf(buf, "Content-Disposition: form-data; name=%q\r\n\r\n",
			stripCRLF(k))
		buf.WriteString(stripCRLF(v))
		buf.WriteString("\r\n")
	}
	fmt.Fprintf(buf, "--%s--\r\n", boundary)
	return "multipart/form-data; boundary=" + boundary
}

// stripCRLF — satrdan CR va LF ni olib tashlaydi.
//
// Multipart tuzilishi FAQAT yangi qatorlar bilan belgilanadi, ya'ni
// ular bo'lmasa qiymat hech qanday holatda "yangi maydon" bo'lib
// ketolmaydi. Belgilar o'chiriladi (probel bilan almashtirilmaydi):
// SMS matnida ular baribir ko'rinmaydi.
func stripCRLF(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}
