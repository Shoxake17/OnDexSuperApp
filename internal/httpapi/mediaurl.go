package httpapi

import (
	"net/url"
	"os"
	"strings"
)

// Mijozga ko'rsatiladigan media manzillarining YAGONA tekshiruvi.
//
// ┌─ TUZATILGAN NOSOZLIK (bug.md 24-band) ─────────────────────────────┐
// `CoverURL`, `PDFURL`, `Pages[]` (kitoblar) va `LogoURL`, `CoverURL`
// (restoranlar) uchun FAQAT UZUNLIK tekshirilardi. Ya'ni bu
// maydonlarga istalgan satr yozilardi: `javascript:`, `data:` yoki
// begona domen — va ular keyin mijoz ilovasida hamda vebda rasm/
// havola sifatida ishlatilardi.
//
// Oqibati: restoran xodimi (`staff` roli kifoya) mijozlarning
// ilovasini begona domendan rasm/PDF yuklashga majburlay olardi —
// kamida kuzatuv (IP to'plash), ko'pi bilan ilova ichidagi kontentni
// almashtirish.
//
// NOMUVOFIQLIK: xuddi shu loyihada `Scene3DURL` QAT'IY tekshiriladi
// (`routes_admin.go: isAllowedSceneURL` — faqat `https://` va faqat
// `R2_PUBLIC_URL` domeni). Qolgan URL maydonlarida shu qoida
// qo'llanmagandi.
//
// ┌─ NEGA `isAllowedSceneURL` DAN YUMSHOQROQ ──────────────────────────┐
// 3D maket ichida BAJARILADIGAN kod bor, shuning uchun u faqat bizning
// omborimizdan bo'lishi shart. Rasm va PDF esa kod bajarmaydi va
// ular tarixan tashqi manzillardan ham kelgan (masalan restoran
// logotipi). Shuning uchun bu yerda ikki daraja bor:
//
//	isSafeMediaURL   — sxema `https:` (yoki nisbiy yo'l). Xavfli
//	                   sxemalar (`javascript:`, `data:`, `file:`)
//	                   va sxemasiz `//host` RAD ETILADI.
//	isOwnMediaURL    — qo'shimcha: FAQAT `R2_PUBLIC_URL` domeni.
//
// Hozir `isSafeMediaURL` qo'llanadi (mavjud ma'lumotni buzmaydi),
// `isOwnMediaURL` esa media butunlay R2 ga ko'chgach yoqiladi.
// └────────────────────────────────────────────────────────────────────┘

// isSafeMediaURL — manzil mijozga ko'rsatish uchun xavfsizmi.
//
// Qabul qilinadi:
//   - bo'sh satr (maydon ixtiyoriy);
//   - `https://host/...`;
//   - nisbiy yo'l `/uploads/...` (lokal disk rejimi — `images.Store`
//     shunday manzil qaytaradi).
//
// Rad etiladi: `javascript:`, `data:`, `file:`, `http://` va sxemasiz
// `//host` (protokolga nisbiy — brauzer uni mutlaq deb o'qiydi).
func isSafeMediaURL(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return true // maydon ixtiyoriy
	}
	// Nisbiy yo'l — o'z serverimiz. `//host` BUNGA KIRMAYDI.
	if strings.HasPrefix(s, "/") {
		return !strings.HasPrefix(s, "//") && !strings.HasPrefix(s, "/\\")
	}
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	// `https` — yagona ruxsat etilgan mutlaq sxema. `http` ham rad
	// etiladi: aralash kontent sifatida brauzer uni baribir bloklaydi
	// (68-band aynan shu haqda edi).
	return u.Scheme == "https" && u.Host != ""
}

// isOwnMediaURL — manzil AYNAN bizning obyekt omborimizdanmi.
//
// `isAllowedSceneURL` bilan bir xil qoida, lekin umumiy: media
// butunlay R2 ga ko'chgach media maydonlariga ham shu qo'llanadi.
func isOwnMediaURL(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return true
	}
	return hasMediaBase(s, "")
}

// legacyMediaBaseURLs — `R2_LEGACY_PUBLIC_URLS` (vergul bilan): ESKI ochiq
// domenlar. Rasm domeni almashtirilganda (masalan `pub-xxx.r2.dev` ->
// `cdn.ondex.uz`) bazada eski manzilli yozuvlar qoladi. Ular hamon
// "bizniki" deb tan olinishi shart, aks holda:
//   - restoran sozlamalarida eski logo saqlanmaydi (`isOwnUploadURL`);
//   - 3D maket kaliti ajratilmaydi va maket yuklanmay qoladi (`scenes.ObjectKey`).
//
// Faqat `https://` qabul qilinadi.
func legacyMediaBaseURLs() []string {
	var out []string
	for _, p := range strings.Split(os.Getenv("R2_LEGACY_PUBLIC_URLS"), ",") {
		p = strings.TrimRight(strings.TrimSpace(p), "/")
		if strings.HasPrefix(p, "https://") {
			out = append(out, p)
		}
	}
	return out
}

// mediaBaseURLs — asosiy (`R2_PUBLIC_URL`, birinchi) va eski ochiq domenlar.
func mediaBaseURLs() []string {
	var out []string
	if b := strings.TrimRight(strings.TrimSpace(os.Getenv("R2_PUBLIC_URL")), "/"); strings.HasPrefix(b, "https://") {
		out = append(out, b)
	}
	return append(out, legacyMediaBaseURLs()...)
}

// hasMediaBase — manzil bizning omborlardan birining ichidami
// (`folder` berilsa, aynan shu papka ostida).
func hasMediaBase(s, folder string) bool {
	prefix := "/"
	if folder != "" {
		prefix = "/" + folder + "/"
	}
	for _, base := range mediaBaseURLs() {
		if strings.HasPrefix(s, base+prefix) {
			return true
		}
	}
	return false
}
