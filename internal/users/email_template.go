package users

import (
	"fmt"
	"html"
	"strings"
)

// Tasdiqlash kodi xatining ko'rinishi (OnDex uslubi).
//
// ── NEGA HTML EMAIL ODDIY HTML EMAS ────────────────────────────────
// Pochta mijozlari brauzer emas. Outlook Windows'da Word'ning render
// dvigatelini ishlatadi, Gmail `<style>` bloklarini qirqadi, ko'plab
// mijozlar `flex`/`grid`/`position` ni umuman bilmaydi. Shuning uchun:
//
//   - joylashuv FAQAT `<table>` bilan (div+flex emas);
//   - CSS FAQAT `style="..."` ichida (tashqi/`<style>` blok emas);
//   - TASHQI RASM YO'Q — Gmail ularni standart holda bloklaydi
//     (xat buzilib ko'rinadi) va uzoq rasm kuzatuv pikseli sifatida
//     qaraladi, bu spam balini oshiradi. Logotip MATN bilan
//     chizilgan: "On" qora, "Dex" to'q sariq — ilovadagi bilan bir xil.
//   - ranglar ANIQ yozilgan (mijoz qorong'i rejimda invert qilmasin).
//
// ── NEGA MATNLI NUSXA HAM KERAK ────────────────────────────────────
// Xat `multipart/alternative` bo'lib ketadi: HTML va oddiy matn birga.
// FAQAT HTML yuborish spam balini sezilarli oshiradi va HTML'ni
// o'chirib qo'ygan mijozlarda xat bo'sh ko'rinadi.

// Ilova bilan bir xil palitra (`apps/customer_app/lib/widgets/auth_ui.dart`).
const (
	brandOrange = "#F64E03"
	brandText   = "#1A1A1A"
	brandMuted  = "#7C7671"
	brandBg     = "#FDFBFA"
	brandBorder = "#EDE5DF"
	brandSoft   = "#FFF3ED" // kod qutisi foni
)

// VerificationEmail — tasdiqlash kodi xatining mavzusi, matnli va
// HTML ko'rinishi.
//
// Kod har doim raqamlardan iborat (`randomCode`), lekin baribir
// ekranlaymiz: shablonga kelajakda boshqa qiymat uzatilsa ham
// HTML ichiga xom holda tushib qolmasin.
func VerificationEmail(code string, ttlMinutes int) (subject, text, htmlBody string) {
	safeCode := html.EscapeString(code)

	// Mavzuda kod ham bor: foydalanuvchi xatni ochmasdan, bildirishnoma
	// yoki ro'yxat ko'rinishidayoq o'qiy oladi (Google, Apple, GitHub
	// shu naqshni ishlatadi).
	subject = fmt.Sprintf("%s — OnDex tasdiqlash kodi", code)

	text = fmt.Sprintf(
		"OnDex tasdiqlash kodi: %s\n\n"+
			"Kod %d daqiqa amal qiladi.\n\n"+
			"Kodni HECH KIM bilan ulashmang — OnDex xodimlari uni hech qachon so'ramaydi.\n"+
			"Agar bu so'rovni siz yubormagan bo'lsangiz, bu xatni e'tiborsiz qoldiring.\n",
		code, ttlMinutes)

	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><head><meta charset="UTF-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width,initial-scale=1"></head>`)
	fmt.Fprintf(&b, `<body style="margin:0;padding:0;background:%s;">`, brandBg)

	// Ko'rinmas oldindan ko'rish matni (Gmail ro'yxatida mavzudan
	// keyin chiqadi). Busiz u yerga xatning tasodifiy boshi tushadi.
	fmt.Fprintf(&b,
		`<div style="display:none;max-height:0;overflow:hidden;opacity:0;">`+
			`Tasdiqlash kodingiz: %s. Kod %d daqiqa amal qiladi.</div>`,
		safeCode, ttlMinutes)

	// Tashqi jadval — markazlashtirish uchun (div+margin:auto ba'zi
	// mijozlarda ishlamaydi).
	fmt.Fprintf(&b, `<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" `+
		`style="background:%s;padding:24px 12px;"><tr><td align="center">`, brandBg)

	fmt.Fprintf(&b, `<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" `+
		`style="max-width:480px;background:#FFFFFF;border:1px solid %s;border-radius:14px;">`,
		brandBorder)

	// Sarlavha: logotip
	fmt.Fprintf(&b, `<tr><td style="padding:28px 28px 0 28px;">`+
		`<div style="font-family:Arial,Helvetica,sans-serif;font-size:28px;font-weight:bold;`+
		`letter-spacing:-0.5px;color:%s;">On<span style="color:%s;">Dex</span></div>`+
		`<div style="font-family:Arial,Helvetica,sans-serif;font-size:11px;color:%s;`+
		`margin-top:2px;">Xalq ilovasi</div></td></tr>`,
		brandText, brandOrange, brandMuted)

	// Asosiy matn
	fmt.Fprintf(&b, `<tr><td style="padding:22px 28px 0 28px;`+
		`font-family:Arial,Helvetica,sans-serif;">`+
		`<div style="font-size:19px;font-weight:bold;color:%s;">Tasdiqlash kodi</div>`+
		`<div style="font-size:14px;color:%s;margin-top:6px;line-height:20px;">`+
		`Quyidagi kodni ilovaga kiriting.</div></td></tr>`,
		brandText, brandMuted)

	// Kod qutisi
	fmt.Fprintf(&b, `<tr><td style="padding:18px 28px 0 28px;">`+
		`<table role="presentation" width="100%%" cellpadding="0" cellspacing="0">`+
		`<tr><td align="center" style="background:%s;border:1px solid %s;border-radius:12px;`+
		`padding:18px 10px;font-family:'Courier New',Courier,monospace;font-size:32px;`+
		`font-weight:bold;letter-spacing:8px;color:%s;">%s</td></tr></table></td></tr>`,
		brandSoft, brandOrange, brandText, safeCode)

	fmt.Fprintf(&b, `<tr><td style="padding:12px 28px 0 28px;`+
		`font-family:Arial,Helvetica,sans-serif;font-size:13px;color:%s;">`+
		`Kod <b style="color:%s;">%d daqiqa</b> amal qiladi.</td></tr>`,
		brandMuted, brandText, ttlMinutes)

	// Ajratgich
	fmt.Fprintf(&b, `<tr><td style="padding:22px 28px 0 28px;">`+
		`<div style="height:1px;background:%s;line-height:1px;font-size:0;">&nbsp;</div></td></tr>`,
		brandBorder)

	// Ogohlantirish
	fmt.Fprintf(&b, `<tr><td style="padding:16px 28px 26px 28px;`+
		`font-family:Arial,Helvetica,sans-serif;font-size:12.5px;color:%s;line-height:19px;">`+
		`Kodni <b style="color:%s;">hech kim bilan ulashmang</b> — OnDex xodimlari uni `+
		`hech qachon so'ramaydi.<br>`+
		`Agar bu so'rovni siz yubormagan bo'lsangiz, bu xatni e'tiborsiz qoldiring.`+
		`</td></tr>`,
		brandMuted, brandText)

	b.WriteString(`</table>`)

	fmt.Fprintf(&b, `<div style="font-family:Arial,Helvetica,sans-serif;font-size:11px;`+
		`color:%s;margin-top:14px;">OnDex — Chust</div>`, brandMuted)

	b.WriteString(`</td></tr></table></body></html>`)
	return subject, text, b.String()
}
