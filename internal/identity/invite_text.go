package identity

import (
	"fmt"
	"strings"

	"chustapp/internal/identity/authz"
)

// inviteAppNames — taklif xabarida ilova nomi.
var inviteAppNames = map[string]string{
	authz.AppPanel: "OnDex Merchant panel",
	authz.AppPro:   "OnDexPro",
	authz.AppGo:    "OnDexGO",
}

// InviteText — xodimga Telegram xabari: qaysi biznes, qaysi rol, qaysi
// ilovaga kirish (telefon raqami + kod, 24 soat). Havola YO'Q — xodim
// ilovani o'zi ochadi (fishing xabarlaridan farqlash oson bo'lsin).
func InviteText(inv Invite) string {
	var b strings.Builder
	fmt.Fprintf(&b, "👋 Sizni «%s» ga xodim sifatida taklif qilishdi.\n", inv.BusinessName)
	if inv.RoleName != "" {
		fmt.Fprintf(&b, "Lavozim: %s\n", inv.RoleName)
	}
	apps := make([]string, 0, len(inv.Apps))
	for _, a := range inv.Apps {
		if n := inviteAppNames[a]; n != "" {
			apps = append(apps, n)
		}
	}
	if len(apps) > 0 {
		fmt.Fprintf(&b, "Ilova: %s\n", strings.Join(apps, ", "))
	}
	b.WriteString("\nIlovani oching va shu telefon raqami bilan kiring — kod Telegram'ga keladi. ")
	b.WriteString("Taklif 24 soat amal qiladi.\n")
	b.WriteString("Agar bu biznesni tanimasangiz — xabarni e'tiborsiz qoldiring.")
	return b.String()
}
