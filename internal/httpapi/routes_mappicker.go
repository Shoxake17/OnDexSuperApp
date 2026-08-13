package httpapi

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"net/http"
)

//go:embed assets/map_picker.html
var mapPickerFS embed.FS

// cspNoncePlaceholder — HTML ichida server tomonidan almashtiriladigan
// belgi. Sahifada aynan shu nom bilan `<script nonce="…">` turibdi.
const cspNoncePlaceholder = "__CSP_NONCE__"

// registerMapPickerRoutes — Windows desktop paneli uchun xarita sahifasi.
//
// ┌─ NEGA BU SAHIFA UMUMAN BOR ────────────────────────────────────────┐
// `google_maps_flutter` faqat android/ios/web platformalarini e'lon
// qiladi; Windows uchun implementatsiya MAVJUD EMAS. Desktop panelda
// `GoogleMap` vidjeti "TargetPlatform.windows is not yet supported by
// the maps plugin" xatosini chizadi. Shuning uchun desktopda xarita
// WebView2 ichida, shu sahifa orqali ko'rsatiladi.
// └────────────────────────────────────────────────────────────────────┘
//
// ┌─ NEGA AUTH TALAB QILINMAYDI ───────────────────────────────────────┐
// Sahifada SIR YO'Q: Maps kaliti ichiga yozilmagan. Kalitni sahifa
// keyinroq `/config/maps` dan oladi va u endpoint auth talab qiladi.
// Ya'ni himoya joyida qoladi, faqat statik HTML ochiq bo'ladi.
//
// Agar bu sahifa auth talab qilsa, WebView navigatsiyasiga
// `Authorization` sarlavhasini qo'shish kerak bo'lardi yoki token
// URL'ga yozilardi — ikkinchisi tokenni brauzer tarixi va server
// loglariga tushirardi.
// └────────────────────────────────────────────────────────────────────┘
func (s *Server) registerMapPickerRoutes(mux *http.ServeMux) {
	page, err := mapPickerFS.ReadFile("assets/map_picker.html")
	if err != nil {
		// `go:embed` fayli build vaqtida tekshiriladi — bu yerga
		// tushish deyarli imkonsiz, lekin jimgina o'tkazib yuborish
		// "xarita ochilmayapti" degan tushunarsiz nosozlikka olib
		// kelardi.
		panic("map_picker.html embed qilinmadi: " + err.Error())
	}

	// Sahifa bir marta ikkiga bo'linadi — har so'rovda qidirish/
	// almashtirish o'rniga ikki bo'lakni nonce bilan yozib chiqamiz.
	//
	// Belgi yo'qolib qolsa DARHOL yiqilamiz: busiz sahifa nonce'siz
	// ketardi, CSP uni bloklardi va biz yana o'sha "Xarita
	// yuklanmoqda…" jim qotishiga qaytardik.
	head, tail, found := bytes.Cut(page, []byte(cspNoncePlaceholder))
	if !found {
		panic("map_picker.html ichida " + cspNoncePlaceholder + " yo'q — " +
			"CSP nonce'siz sahifa skripti bloklanadi")
	}

	mux.HandleFunc("GET /map-picker", func(w http.ResponseWriter, r *http.Request) {
		nonce, err := randomNonce()
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}

		// ┌─ NEGA BU SAHIFAGA ALOHIDA CSP ─────────────────────────────┐
		// Global sarlavha (`writeSecurityHeaders`) `default-src 'none'`
		// qo'yadi. U JSON API uchun to'g'ri — u yerda umuman resurs
		// yuklanmaydi. Lekin `default-src` `script-src` ni ham qamrab
		// oladi, ya'ni BU SAHIFANING inline skripti bloklanardi va
		// sahifa "Xarita yuklanmoqda…" da abadiy qotib qolardi.
		//
		// Quyidagi siyosat imkon qadar tor:
		//   script-src — faqat SHU nonce va undan keyin `strict-dynamic`
		//     (nonce'li skript yuklagan skriptlar, ya'ni Maps JS va
		//     uning ichki modullari). `https:` — `strict-dynamic` ni
		//     tushunmaydigan eski brauzerlar uchun zaxira;
		//   style-src 'unsafe-inline' — Maps JS uslublarni DOM'ga
		//     dinamik yozadi va ularga nonce qo'yib bo'lmaydi. Uslub
		//     injektsiyasi skriptga qaraganda ancha kam xavfli;
		//   connect-src 'self' — sahifa `/config/maps` ni SHU domendan
		//     oladi (sahifaning o'zi ham shu domendan yuklangan).
		// └────────────────────────────────────────────────────────────┘
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; "+
				"script-src 'nonce-"+nonce+"' 'strict-dynamic' https:; "+
				"style-src 'unsafe-inline' https://fonts.googleapis.com; "+
				"img-src 'self' data: blob: https://*.googleapis.com https://*.gstatic.com; "+
				"connect-src 'self' https://*.googleapis.com; "+
				"font-src https://fonts.gstatic.com; "+
				"worker-src blob:; "+
				"frame-ancestors 'none'; base-uri 'none'")

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Sahifa o'zgarmas (embed qilingan), lekin keshlanmasin —
		// yangilanish darhol yetib borsin. Nonce har so'rovda
		// boshqacha bo'lgani uchun keshlash ZARARLI ham bo'lardi:
		// eski nonce yangi CSP sarlavhasiga mos kelmasdi.
		w.Header().Set("Cache-Control", "no-store")
		// Bu sahifa faqat WebView ichida ochilishi kerak. Boshqa
		// saytga freym qilib qo'yilishining ma'nosi yo'q.
		w.Header().Set("X-Frame-Options", "DENY")

		_, _ = w.Write(head)
		_, _ = w.Write([]byte(nonce))
		_, _ = w.Write(tail)
	})
}

// randomNonce — CSP uchun bir martalik qiymat (128 bit).
//
// Har SO'ROVDA yangi bo'lishi shart: nonce qayta ishlatilsa, u
// hujumchiga oldindan ma'lum bo'lib qoladi va inline skript
// himoyasining ma'nosi qolmaydi.
func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}
