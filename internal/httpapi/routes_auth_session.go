package httpapi

import (
	"log/slog"
	"net/http"

	"chustapp/internal/identity"
	"chustapp/internal/users"
)

// Mijoz sessiyasini markazlashtirish (Plane 137).
//
// ┌─ QANDAY ISHLAYDI ─────────────────────────────────────────────────────┐
// Mijoz ilovasining kirish/ro'yxatdan o'tish ekranlari va endpointlari
// O'ZGARMAYDI (parol, SMS/Telegram kodi, Google, Firebase, email). Faqat
// natija: ilova `X-Ondex-Session: v2` yuborsa, eski 30 kunlik token o'rniga
// markaziy sessiya (`ondex-customer`: 10 daqiqalik access + refresh
// rotatsiyasi, qurilma alohida ko'rinadi va alohida yopiladi) beriladi.
// Eski ilova versiyalari (sarlavhasiz) eski tokenni olishda davom etadi —
// hech kim tizimdan chiqib ketmaydi.
//
// Kimlik ALLAQACHON eski oqim tomonidan tekshirilgan (`token` shu oqim
// bergan). "Kod bilan kirdi" belgisi (parolsiz parol o'rnatish, 15 daqiqa)
// o'sha tokendan olinib, sessiyaga `otp` sifatida o'tadi.
// └───────────────────────────────────────────────────────────────────────┘

const sessionHeader = "X-Ondex-Session"

func wantsCentralSession(r *http.Request) bool { return r.Header.Get(sessionHeader) == "v2" }

// writeLogin — kirish natijasi: eski token yoki (so'ralsa) markaziy sessiya.
func (s *Server) writeLogin(w http.ResponseWriter, r *http.Request, legacyToken string, u *users.User) {
	noStore(w)
	realm := users.RealmCustomer
	if u != nil {
		realm = users.RealmForRole(u.Role)
	}
	central := u != nil && wantsCentralSession(r) && s.Identity != nil && realm == users.RealmCustomer
	// AUD-5: bu realm uchun eski token o'chirilgan — markaziy sessiyasiz
	// kirish yo'q. 426: ilova yangilanishi kerak (eski versiya). Merchant va
	// admin akkauntlari bu yo'ldan umuman kirmaydi (o'z ilovalari bor).
	if !central && s.Tokens.RealmDisabled(realm) {
		writeJSON(w, http.StatusUpgradeRequired, map[string]any{"error": users.ErrLegacyTokenDisabled.Error(), "upgrade_required": true})
		return
	}
	if !central {
		writeJSON(w, http.StatusOK, map[string]any{"token": legacyToken, "user": u})
		return
	}
	amr := []string{"legacy"}
	if c, err := s.Tokens.ParseAny(legacyToken); err == nil && c.PhoneProven {
		amr = []string{"otp"}
	}
	tok, err := s.Identity.Create(r.Context(), identity.CreateInput{UserID: u.ID, ClientID: identity.CustomerClientID,
		AMR: amr, Meta: identity.Meta{IP: clientIP(r), UserAgent: r.UserAgent(), DeviceName: r.Header.Get("X-Ondex-Client")}})
	if err != nil {
		// Markaziy sessiya ochilmasa — eski token (kirish buzilmasin).
		slog.Warn("mijoz markaziy sessiyasi ochilmadi — eski token berildi", "user", u.ID, "err", err)
		writeJSON(w, http.StatusOK, map[string]any{"token": legacyToken, "user": u})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		// `token` — eski ilova kodi shu maydonni o'qiydi; endi u access token.
		"token": tok.AccessToken, "access_token": tok.AccessToken, "refresh_token": tok.RefreshToken,
		"expires_at": tok.ExpiresAt, "session_id": tok.SessionID, "user": u,
	})
}

// isCentralRequest — so'rov markaziy sessiya tokeni bilan kelganmi.
func isCentralRequest(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	return len(h) > 7 && isEdDSA(h[7:])
}
