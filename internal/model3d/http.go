package model3d

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// httpClient — paketdagi BARCHA tashqi so'rovlar uchun yagona klient
// (provayder API'si ham, model yuklab olish ham).
//
// ┌─ NEGA `http.DefaultClient` EMAS ──────────────────────────────────┐
// Standart klientda TIMEOUT umuman yo'q: javob bermayotgan tashqi
// xizmat gorutinani cheksiz ushlab turadi va ular asta-sekin
// to'planib, serverni bosadi. Bu yerdagi chegara — butun so'rov uchun.
//
// `Redirect` cheklovi ham SHART: SSRF tekshiruvi FAQAT birinchi
// havolaga qo'llanadi, provayder esa bizni ichki manzilga
// yo'naltirishi mumkin edi. Shuning uchun har bir qayta yo'naltirish
// QAYTA tekshiriladi.
// └───────────────────────────────────────────────────────────────────┘
var httpClient = &http.Client{
	Timeout: 60 * time.Second,
	// ┌─ TOCTOU / DNS-REBINDING YOPILDI ──────────────────────────────────┐
	// Avval SSRF himoyasi FAQAT `safeHTTPSURL` da edi: u domenni bir marta
	// yechib (LookupIP) IP'ni tekshirardi, LEKIN haqiqiy ulanishda transport
	// DNS'ni QAYTA, mustaqil yechardi. Tekshiruv ("check") va ulanish ("use")
	// orasida DNS boshqa (ichki) IP qaytarishi mumkin edi — klassik DNS
	// rebinding bilan filtrni chetlab o'tish.
	//
	// Endi ulanish `safeDialContext` orqali: DNS BIR marta yechiladi, har
	// bir IP tekshiriladi va ulanish AYNAN o'sha yechilgan IP'ga bo'ladi —
	// transport uni qayta yechmaydi. Bu httpClient orqali ketadigan BARCHA
	// so'rovni (provayder API'si, model yuklab olish, redirect'lar) qamraydi.
	// TLS baribir asl host nomi bilan bajariladi (SNI + sertifikat), chunki
	// biz faqat TCP dial'ini boshqaramiz.
	// └───────────────────────────────────────────────────────────────────┘
	Transport: &http.Transport{
		DialContext:           safeDialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("juda ko'p qayta yo'naltirish")
		}
		// Ikkinchi qatlam: sxema (https) tekshiruvi. IP tekshiruvining
		// o'zi endi `safeDialContext` da, ulanish paytida bo'ladi.
		if _, err := safeHTTPSURL(req.URL.String()); err != nil {
			return err
		}
		return nil
	},
}

// safeDialer — `safeDialContext` ishlatadigan asosiy TCP dialer.
var safeDialer = &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}

// safeDialContext — SSRF himoyasining ISHONCHLI qatlami: TCP ulanish
// paytida host yechiladi, IP tekshiriladi va ulanish AYNAN o'sha IP'ga
// bo'ladi. Shu tufayli tekshiruv va ulanish o'rtasida DNS o'zgarishi
// (rebinding) filtrni teshib o'tolmaydi.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("%w: host topilmadi", ErrUnsafeURL)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("%w: host topilmadi", ErrUnsafeURL)
	}
	// Yechilgan HAR BIR IP ichki bo'lmasligi kerak — bittasi ham ichki
	// bo'lsa rad etamiz (aralash javob orqali chetlab o'tishning oldini
	// oladi).
	for _, ip := range ips {
		if isInternalIP(ip) {
			return nil, fmt.Errorf("%w: ichki manzil (%s)", ErrUnsafeURL, ip)
		}
	}
	// Tekshirilgan IP'ning AYNAN o'ziga ulanamiz (DNS qayta yechilmaydi).
	return safeDialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

func newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OnDex/1.0")
	return req, nil
}

// safeHTTPSURL — SSRF (Server-Side Request Forgery) himoyasi.
//
// ┌─ MUAMMO ──────────────────────────────────────────────────────────┐
// Model havolasi TASHQI xizmatdan keladi, ya'ni uni biz nazorat
// qilmaymiz. Agar shu havolani so'zsiz yuklab olsak, provayder (yoki
// uning javobini o'zgartira olgan hujumchi) serverimizni ICHKI
// tarmoqqa so'rov yuborishga majbur qila oladi:
//
//	http://169.254.169.254/...  — bulut metama'lumotlari (kalitlar!)
//	http://localhost:5432/      — bazaga
//	http://10.0.0.5/            — ichki xizmatlarga
//
// Server bu manzillarga tashqaridan yopiq bo'lsa ham, O'ZI uchun ochiq.
// └───────────────────────────────────────────────────────────────────┘
//
// Uch shart: (1) faqat HTTPS, (2) IP manzil to'g'ridan-to'g'ri
// berilmasin, (3) domen nomi YECHILGANDA hech bir IP ichki bo'lmasin.
//
// Domen "oq ro'yxati" ATAYLAB ishlatilmadi: provayderning CDN domeni
// ogohlantirishsiz o'zgarishi mumkin va integratsiya jimgina buzilardi.
// IP darajasidagi tekshiruv esa provayder domenidan qat'i nazar
// ishlaydi.
func safeHTTPSURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("%w: havola o'qilmadi", ErrUnsafeURL)
	}
	if u.Scheme != "https" {
		return nil, fmt.Errorf("%w: faqat https ruxsat etiladi", ErrUnsafeURL)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("%w: host bo'sh", ErrUnsafeURL)
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("%w: host topilmadi", ErrUnsafeURL)
	}
	for _, ip := range ips {
		if isInternalIP(ip) {
			return nil, fmt.Errorf("%w: ichki manzil (%s)", ErrUnsafeURL, ip)
		}
	}
	return u, nil
}

// isInternalIP — manzil ichki/xizmat tarmog'iga tegishlimi.
func isInternalIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast() ||
		// 100.64.0.0/10 (CGNAT) — `IsPrivate` buni qamramaydi, lekin
		// ko'p bulutlarda ichki tarmoq sifatida ishlatiladi.
		(ip.To4() != nil && ip.To4()[0] == 100 && ip.To4()[1] >= 64 && ip.To4()[1] <= 127)
}
