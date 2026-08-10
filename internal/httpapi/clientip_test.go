package httpapi

import (
	"net/http"
	"testing"
)

// `clientIP` — BARCHA IP-asosidagi tezlik cheklovlarining tayanchi:
// SMS byudjeti (`otpIPLimiter`), brute-force (`loginIPLimiter`) va
// pullik geokodlash (`geoIPLimiter`). Avval u `X-Forwarded-For` ga
// SHARTSIZ ishonardi va uchalasini ham bitta sarlavha bilan yo'q
// qilish mumkin edi (jonli o'lchov: soxta XFF bilan 60/60 so'rov
// o'tdi, XFF'siz 32-so'rovda 429).

func reqWith(remote, xff string) *http.Request {
	r, _ := http.NewRequest("POST", "/auth/login", nil)
	r.RemoteAddr = remote
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	return r
}

func TestClientIPIgnoresForwardedHeaderByDefault(t *testing.T) {
	SetTrustedProxies(nil)
	t.Cleanup(func() { SetTrustedProxies(nil) })

	got := clientIP(reqWith("203.0.113.9:51234", "1.2.3.4"))
	if got != "203.0.113.9" {
		t.Fatalf("soxtalashtirilgan XFF qabul qilindi: %q (kutilgan 203.0.113.9)", got)
	}
}

func TestClientIPHonoursForwardedFromTrustedProxy(t *testing.T) {
	SetTrustedProxies([]string{"10.0.0.0/8"})
	t.Cleanup(func() { SetTrustedProxies(nil) })

	// Ishonchli proksidan kelgan zanjirning BIRINCHI qiymati — asl mijoz.
	if got := clientIP(reqWith("10.1.2.3:9999", "1.2.3.4, 10.1.2.3")); got != "1.2.3.4" {
		t.Fatalf("ishonchli proksi XFF'si o'qilmadi: %q", got)
	}
	// Ishonchsiz manbadan kelgan XFF esa baribir e'tiborga olinmaydi.
	if got := clientIP(reqWith("203.0.113.9:51234", "1.2.3.4")); got != "203.0.113.9" {
		t.Fatalf("ishonchsiz manbadan XFF qabul qilindi: %q", got)
	}
}

func TestClientIPTrustsSingleAddressEntry(t *testing.T) {
	SetTrustedProxies([]string{"127.0.0.1", "buzuq-qiymat"})
	t.Cleanup(func() { SetTrustedProxies(nil) })

	if got := clientIP(reqWith("127.0.0.1:8080", "8.8.8.8")); got != "8.8.8.8" {
		t.Fatalf("yakka IP ishonchli deb qabul qilinmadi: %q", got)
	}
}

// Ishonchli proksi XFF yubormasa (masalan sarlavha bo'sh), ulanish
// manzilining o'ziga qaytiladi — cheklov kalitsiz qolmasligi kerak.
func TestClientIPFallsBackWhenHeaderEmpty(t *testing.T) {
	SetTrustedProxies([]string{"10.0.0.0/8"})
	t.Cleanup(func() { SetTrustedProxies(nil) })

	if got := clientIP(reqWith("10.1.2.3:9999", "   ")); got != "10.1.2.3" {
		t.Fatalf("bo'sh XFF da zaxira manzil ishlatilmadi: %q", got)
	}
}
