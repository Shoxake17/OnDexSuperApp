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

	// Ishonchli proksi zanjirning OXIRIGA o'zi ko'rgan manzilni
	// qo'shadi. Bizga kerakli qiymat — ishonchli proksilar ro'yxatiga
	// kirmaydigan ENG O'NGDAGI manzil.
	if got := clientIP(reqWith("10.1.2.3:9999", "203.0.113.7, 10.1.2.3")); got != "203.0.113.7" {
		t.Fatalf("ishonchli proksi XFF'si o'qilmadi: %q", got)
	}
	// Ishonchsiz manbadan kelgan XFF esa baribir e'tiborga olinmaydi.
	if got := clientIP(reqWith("203.0.113.9:51234", "1.2.3.4")); got != "203.0.113.9" {
		t.Fatalf("ishonchsiz manbadan XFF qabul qilindi: %q", got)
	}
}

// ★★ 18-BAND REGRESSIYASI — SOXTA PREFIKS QABUL QILINMASIN.
//
// Mijoz o'z so'roviga `X-Forwarded-For: 1.2.3.4` yozadi. Proksi uni
// O'CHIRMASDAN oxiriga haqiqiy manzilni qo'shadi:
//
//	"1.2.3.4, 203.0.113.7, 10.1.2.3"
//	  ^soxta    ^haqiqiy     ^proksi
//
// Eski kod eng CHAPDAGINI olardi — ya'ni hujumchi yozgan qiymatni.
// Har so'rovda uni o'zgartirib, IP bo'yicha barcha cheklovlarni
// (SMS byudjeti, login brute-force, pullik geokodlash) aylanib
// o'tish mumkin edi.
func TestClientIPIgnoresSpoofedPrefix(t *testing.T) {
	SetTrustedProxies([]string{"10.0.0.0/8"})
	t.Cleanup(func() { SetTrustedProxies(nil) })

	cases := map[string]string{
		// mijoz yozgani + haqiqiy + proksi
		"1.2.3.4, 203.0.113.7, 10.1.2.3": "203.0.113.7",
		// bir nechta soxta qiymat ham yordam bermaydi
		"1.1.1.1, 2.2.2.2, 203.0.113.7, 10.1.2.3": "203.0.113.7",
		// mijoz o'zini ishonchli proksi qilib ko'rsatmoqchi
		"10.9.9.9, 203.0.113.7, 10.1.2.3": "203.0.113.7",
	}
	for xff, want := range cases {
		if got := clientIP(reqWith("10.1.2.3:9999", xff)); got != want {
			t.Errorf("XFF %q -> %q, kutilgan %q — SOXTA qiymat qabul qilindi",
				xff, got, want)
		}
	}
}

// Butun zanjir ishonchli bo'lsa (faqat bizning proksilarimiz),
// ulanish manziliga qaytiladi — cheklov qattiqroq bo'ladi.
func TestClientIPFallsBackWhenChainAllTrusted(t *testing.T) {
	SetTrustedProxies([]string{"10.0.0.0/8"})
	t.Cleanup(func() { SetTrustedProxies(nil) })

	if got := clientIP(reqWith("10.1.2.3:9999", "10.0.0.5, 10.1.2.3")); got != "10.1.2.3" {
		t.Fatalf("hammasi ishonchli bo'lganda zaxira manzil ishlatilmadi: %q", got)
	}
}

// Buzilgan qiymat zanjirni to'xtatadi — undan chapdagilarga
// ishonib bo'lmaydi.
func TestClientIPStopsAtGarbage(t *testing.T) {
	SetTrustedProxies([]string{"10.0.0.0/8"})
	t.Cleanup(func() { SetTrustedProxies(nil) })

	// "axlat" o'qib bo'lmaydi -> to'xtaymiz -> zaxira manzil.
	if got := clientIP(reqWith("10.1.2.3:9999", "1.2.3.4, axlat, 10.1.2.3")); got != "10.1.2.3" {
		t.Fatalf("buzilgan zanjirdan keyin soxta qiymat olindi: %q", got)
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
