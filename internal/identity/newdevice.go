package identity

import (
	"fmt"
	"strings"
	"time"
)

// NewDeviceText — egasiga Telegram xabari: "yangi qurilmadan kirildi —
// sizmisiz?" (AUTH-PHASE-A.md 5-bo'lim). IP to'liq ko'rsatiladi — xabar
// faqat akkaunt egasining o'ziga boradi. Matnda havola faqat OnDex ID
// akkaunt sahifasi (qurilmani chiqarish, parolni almashtirish).
func NewDeviceText(d NewDevice, app, accountURL string, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	var b strings.Builder
	b.WriteString("🔐 OnDex ID: akkauntingizga YANGI qurilmadan kirildi.\n\n")
	fmt.Fprintf(&b, "Ilova: %s\n", app)
	if dev := DeviceSummary(d.UserAgent); dev != "" {
		fmt.Fprintf(&b, "Qurilma: %s\n", dev)
	} else if d.DeviceName != "" {
		fmt.Fprintf(&b, "Qurilma: %s\n", d.DeviceName)
	}
	if d.IP != "" {
		fmt.Fprintf(&b, "IP: %s\n", d.IP)
	}
	fmt.Fprintf(&b, "Vaqt: %s\n\n", d.At.In(loc).Format("02.01.2006 15:04"))
	b.WriteString("Bu siz bo'lsangiz — hech narsa qilish shart emas.\n")
	b.WriteString("Siz BO'LMASANGIZ: darhol parolni almashtiring va begona qurilmani chiqaring")
	if accountURL != "" {
		fmt.Fprintf(&b, ": %s", accountURL)
	}
	b.WriteString("\n")
	return b.String()
}

// DeviceSummary — User-Agent'dan "Chrome, Windows" kabi qisqa nom. Aniq
// emas (UA soxtalashtirilishi mumkin) — faqat foydalanuvchiga eslatma.
func DeviceSummary(ua string) string {
	if ua == "" {
		return ""
	}
	l := strings.ToLower(ua)
	browser := ""
	switch {
	case strings.Contains(l, "edg/"):
		browser = "Edge"
	case strings.Contains(l, "opr/") || strings.Contains(l, "opera"):
		browser = "Opera"
	case strings.Contains(l, "yabrowser"):
		browser = "Yandex Browser"
	case strings.Contains(l, "firefox/"):
		browser = "Firefox"
	case strings.Contains(l, "chrome/"):
		browser = "Chrome"
	case strings.Contains(l, "safari/"):
		browser = "Safari"
	case strings.Contains(l, "dart/") || strings.Contains(l, "okhttp"):
		browser = "OnDex ilovasi"
	}
	osName := ""
	switch {
	case strings.Contains(l, "windows"):
		osName = "Windows"
	case strings.Contains(l, "android"):
		osName = "Android"
	case strings.Contains(l, "iphone") || strings.Contains(l, "ipad"):
		osName = "iOS"
	case strings.Contains(l, "mac os"):
		osName = "macOS"
	case strings.Contains(l, "linux"):
		osName = "Linux"
	}
	switch {
	case browser != "" && osName != "":
		return browser + ", " + osName
	case browser != "":
		return browser
	default:
		return osName
	}
}
