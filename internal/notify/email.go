package notify

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// Email yuborish qatlami — SMS bilan bir xil falsafa: dev'da logga
// yoziladi, production'da haqiqiy provayder orqali ketadi. Qaysi biri
// ishlatilishi `.env` ga qarab AVTOMATIK tanlanadi (`NewEmailSender`).

// ErrEmailNotConfigured — SMTP sozlanmagan. Handler buni foydalanuvchiga
// ANIQ xabar sifatida ko'rsatadi ("email orqali tasdiqlash hali
// ulanmagan"), jimgina muvaffaqiyat qaytarmaydi.
var ErrEmailNotConfigured = errors.New("email yuborish sozlanmagan (SMTP_HOST kiritilmagan)")

// LogEmail — dev muhit uchun: haqiqiy xat o'rniga logga yozadi.
// `LogSms` bilan bir xil naqsh.
type LogEmail struct{}

func (LogEmail) Send(to, subject, textBody, htmlBody string) error {
	if err := guardHeader(to, subject); err != nil {
		return err
	}
	// HTML logga yozilmaydi — u uzun va o'qib bo'lmaydi; dev'da kerak
	// bo'ladigan narsa matnli nusxadagi kod.
	slog.Info("email (dev rejim, yuborilmadi)",
		"to", to, "subject", subject, "body", textBody, "html_bytes", len(htmlBody))
	return nil
}

// SmtpEmail — haqiqiy SMTP orqali yuboradi.
//
// XAVFSIZLIK:
//   - Parol/kalit FAQAT `.env` da (`SMTP_PASSWORD`), kodda emas;
//   - Ulanish HAR DOIM shifrlanadi: 465 — bevosita TLS, boshqa portlar
//     (587) — STARTTLS. Shifrlanmagan ulanishda autentifikatsiya
//     UMUMAN bajarilmaydi (`smtp.PlainAuth` ning o'zi ham buni rad
//     etadi, lekin biz bunga tayanmaymiz va aniq tekshiramiz);
//   - Sertifikat tekshiruvi YOQIQ (`InsecureSkipVerify` yo'q) — aks
//     holda o'rtadagi hujumchi SMTP parolini o'qib olardi;
//   - `to`/`subject` da CR/LF RAD ETILADI (header injection).
type SmtpEmail struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	// FromName — xatda ko'rinadigan jo'natuvchi nomi.
	FromName string
}

// NewEmailSender — `.env` ga qarab yuboruvchini tanlaydi.
//
// SMTP_HOST bo'lsa — haqiqiy yuborish; bo'lmasa — dev log.
// Bu SMS'dagi `LogSms` -> Eskiz.uz o'tishi bilan bir xil naqsh, ya'ni
// kod ikkala holatda ham bir xil ishlaydi va sozlash faqat `.env` da.
func NewEmailSender() (sender interface {
	Send(to, subject, textBody, htmlBody string) error
}, configured bool) {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if host == "" {
		return LogEmail{}, false
	}
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" {
		port = "587"
	}
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if from == "" {
		from = strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	}
	fromName := strings.TrimSpace(os.Getenv("SMTP_FROM_NAME"))
	if fromName == "" {
		fromName = "OnDex"
	}
	return &SmtpEmail{
		Host:     host,
		Port:     port,
		Username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     from,
		FromName: fromName,
	}, true
}

func (s *SmtpEmail) Send(to, subject, textBody, htmlBody string) error {
	if err := guardHeader(to, subject); err != nil {
		return err
	}
	addr := net.JoinHostPort(s.Host, s.Port)
	msg := buildMessage(s.FromName, s.From, to, subject, textBody, htmlBody)

	c, err := s.dial(addr)
	if err != nil {
		return fmt.Errorf("smtp ulanish: %w", err)
	}
	defer c.Close()

	// 465 da TLS allaqachon o'rnatilgan; boshqa portlarda STARTTLS
	// MAJBURIY — shifrlanmagan kanalda parol yuborilmaydi.
	if s.Port != "465" {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("smtp: server STARTTLS'ni qo'llab-quvvatlamaydi — parol ochiq ketishi mumkin edi, to'xtatildi")
		}
		if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if s.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(s.From); err != nil {
		return fmt.Errorf("smtp from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp yozish: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp yakunlash: %w", err)
	}
	return c.Quit()
}

func (s *SmtpEmail) dial(addr string) (*smtp.Client, error) {
	const timeout = 15 * time.Second
	if s.Port == "465" {
		// Implicit TLS (SMTPS).
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: timeout}, "tcp", addr,
			&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
		if err != nil {
			return nil, err
		}
		return smtp.NewClient(conn, s.Host)
	}
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	return smtp.NewClient(conn, s.Host)
}

// guardHeader — SMTP header injection'ga qarshi.
//
// NEGA KERAK: manzil yoki mavzu foydalanuvchi kiritmasidan keladi.
// Ichida `\r\n` bo'lsa, hujumchi xat sarlavhalariga o'z qatorini
// qo'shib qo'yadi (masalan `Bcc:`) va bizning serverimiz orqali
// begona odamlarga xat jo'natadi — ya'ni domenimiz spam ro'yxatiga
// tushadi. Tekshiruv YUBORUVCHI QATLAMDA turadi: chaqiruvchi uni
// unutsa ham himoya ishlaydi.
// randomToken — Message-ID uchun noyob qism.
func randomToken() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// Bu yerda xato deyarli bo'lmaydi; bo'lsa ham xatni yuborishni
		// to'xtatish ortiqcha — vaqtga asoslangan zaxira yetarli.
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// domainOf — "a@b.uz" dan "b.uz". Message-ID domeni jo'natuvchi
// domeni bilan mos bo'lishi kerak, aks holda filtrlar shubhalanadi.
func domainOf(addr string) string {
	if i := strings.LastIndex(addr, "@"); i >= 0 && i+1 < len(addr) {
		return addr[i+1:]
	}
	return "localhost"
}

func guardHeader(to, subject string) error {
	for _, v := range []string{to, subject} {
		if strings.ContainsAny(v, "\r\n") {
			return errors.New("email sarlavhasida ruxsat etilmagan belgi")
		}
	}
	if to == "" || !strings.Contains(to, "@") {
		return errors.New("email manzili noto'g'ri")
	}
	return nil
}

func buildMessage(fromName, from, to, subject, textBody, htmlBody string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s <%s>\r\n", fromName, from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	// Message-ID — spam filtrlari uchun MUHIM: usiz xat "to'g'ri
	// yig'ilmagan" deb baholanadi va spam bali oshadi. Har bir xat
	// uchun noyob, tasodifiy qiymat.
	fmt.Fprintf(&b, "Message-ID: <%s@%s>\r\n", randomToken(), domainOf(from))
	// RFC 3834 — xat AVTOMATIK yuborilganini bildiradi. Ikki foydasi:
	// avtomatik javoblar ("отпуск"/out-of-office) qaytmaydi va filtrlar
	// buni tranzaksion xat deb tanidi (marketing rassilkasi emas).
	b.WriteString("Auto-Submitted: auto-generated\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")

	if htmlBody == "" {
		b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
		b.WriteString(dotStuff(textBody))
		b.WriteString("\r\n")
		return b.String()
	}

	// `multipart/alternative` — mijoz IKKALASIDAN birini tanlaydi.
	// TARTIB MUHIM: RFC 2046 bo'yicha oxirgi qism "eng boyi" deb
	// qaraladi, shuning uchun matn BIRINCHI, HTML IKKINCHI bo'ladi.
	// Teskari yozilsa ko'p mijozlar HTML o'rniga matnni ko'rsatadi.
	boundary := "ondex-" + randomToken()
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", boundary)

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n\r\n")
	b.WriteString(dotStuff(textBody))
	b.WriteString("\r\n")

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n\r\n")
	b.WriteString(dotStuff(htmlBody))
	b.WriteString("\r\n")

	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.String()
}

// dotStuff — tana ichidagi yakka "." qatori SMTP'da xat OXIRI deb
// qabul qilinadi, ya'ni xat o'sha joyda kesilib qolardi. Nuqtani
// ikkilantirish (dot-stuffing) — RFC 5321 talabi.
// QATORMA-QATOR ishlanadi. Avval ikki `ReplaceAll` ("\r\n." va "\n.")
// ishlatilgan edi va bu XATO: birinchisi qo'shgan nuqtani ikkinchisi
// yana ikkilantirib, `\r\n` holatida UCHTA nuqta chiqarardi (test
// bilan ushlandi). Qatorlarga bo'lish ikkala qator oxiri uchun ham
// bir marta va to'g'ri ishlaydi.
func dotStuff(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if strings.HasPrefix(ln, ".") {
			lines[i] = "." + ln
		}
	}
	return strings.Join(lines, "\n")
}
