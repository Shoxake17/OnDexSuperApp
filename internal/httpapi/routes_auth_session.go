package httpapi

import (
	"errors"
	"log/slog"
	"net/http"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// Mijoz sessiyasi — FAQAT markaziy (Plane 137 -> ONDEXECOSY-483).
//
// ┌─ QANDAY ISHLAYDI ─────────────────────────────────────────────────────┐
// Mijozning kirish endpointlari (parol, telefon/Telegram kodi, Google,
// Firebase, email, Telegram Mini App) kimlikni tekshiradi va HAR DOIM
// markaziy sessiya (``ondex-customer``: qisqa access + refresh rotatsiyasi,
// qurilma alohida ko'rinadi va alohida yopiladi) beradi.
//
// Eski 30 kunlik HS256 token UMUMAN YO'Q: na chiqariladi, na qabul
// qilinadi. ``X-Ondex-Session: v2`` sarlavhasi endi shart emas (eski ilova
// versiyalari ham markaziy sessiya oladi; refresh'ni bilmaydigan juda eski
// versiya access muddati tugagach qayta kirishga yuboriladi).
//
// Merchant va admin akkauntlari bu yo'ldan kirmaydi — ularning o'z ``/v2``
// kirishi bor (ikkinchi omil, tashkilot tanlash va h.k.).
// └───────────────────────────────────────────────────────────────────────┘

// errUseMerchantLogin — merchant/admin akkaunti mijoz kirish yo'lidan keldi.
var errUseMerchantLogin = errors.New("bu akkaunt uchun OnDex Merchant yoki admin ilovasining kirishidan foydalaning")

// writeLogin — kirish natijasi: markaziy sessiya. “amr“ — kirish usuli
// (“users.AMR*“): "kod bilan HOZIRGINA kirdi" (otp/email) 15 daqiqa parolni
// joriy parolsiz o'rnatishga ruxsat beradi (“parseBearer“ -> PhoneProven).
func (s *Server) writeLogin(w http.ResponseWriter, r *http.Request, amr string, u *users.User) {
	noStore(w)
	if u == nil || s.Identity == nil {
		slog.Error("kirish: markaziy sessiya xizmati yo'q", "user_nil", u == nil)
		httpError(w, http.StatusServiceUnavailable, errors.New("sessiya ochilmadi, birozdan so'ng qayta urining"))
		return
	}
	if users.RealmForRole(u.Role) != users.RealmCustomer {
		writeJSON(w, http.StatusUpgradeRequired, map[string]any{"error": errUseMerchantLogin.Error(), "upgrade_required": true})
		return
	}
	if amr == "" {
		amr = users.AMRPassword
	}
	tok, err := s.Identity.Create(r.Context(), identity.CreateInput{UserID: u.ID, ClientID: identity.CustomerClientID,
		AMR: []string{amr}, Meta: identity.Meta{IP: clientIP(r), UserAgent: r.UserAgent(), DeviceName: r.Header.Get("X-Ondex-Client")}})
	if err != nil {
		slog.Error("mijoz markaziy sessiyasi ochilmadi", "user", u.ID, "err", err)
		httpError(w, http.StatusServiceUnavailable, errors.New("sessiya ochilmadi, birozdan so'ng qayta urining"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		// ``token`` — eski ilova kodi shu maydonni o'qiydi; u access token.
		"token": tok.AccessToken, "access_token": tok.AccessToken, "refresh_token": tok.RefreshToken,
		"expires_at": tok.ExpiresAt, "session_id": tok.SessionID, "user": u,
	})
}
