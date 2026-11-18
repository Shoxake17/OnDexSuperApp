package identity

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"time"

	"chustapp/internal/users"
)

// ┌─ WEB PANEL BFF SESSIYALARI (RS-7 merchant, 479 admin) ─────────────────┐
// Flutter web panellar brauzerda token SAQLAMAYDI: XSS bo'lsa localStorage'
// dagi token o'g'irlanardi. Kirish odatdagi oqim bilan (merchant: parol/OTP +
// TOTP/passkey; admin: Telegram kodi + parol + TOTP har safar) BFF klientida
// sessiya ochadi; BFF (`httpapi/panelweb_bff.go`) tokenlarni javobdan olib
// tashlaydi va brauzerga faqat HttpOnly cookie beradi: `sid.HMAC`.
//
// Cookie — faqat ko'rsatkich: sessiyaning o'zi markazda, bekor qilish
// markazda. Refresh token ISHLATILMAYDI — BFF sessiya bo'yicha qisqa access
// chiqaradi (`WebBFFAccess`) va FAOL foydalanuvchining harakatsizlik
// muddatini uzaytiradi (`Store.Touch`) — admin 30 daqiqa harakatsiz
// qolsa chiqadi, ishlab o'tirgan bo'lsa yo'q.
//
// Har klientning cookie kaliti alohida (bir panel cookie'si ikkinchisida
// ishlamaydi). Sessiya ID'si audit va "faol qurilmalar" ro'yxatida
// ko'rinadi, shuning uchun cookie sid'ning o'zi emas — HMAC bilan imzolangan.
// └────────────────────────────────────────────────────────────────────────┘

// PanelWebClientID — merchant web panel (BFF) klienti.
const PanelWebClientID = "merchant-panel-web"

// AdminPanelWebClientID — admin web panel (BFF) klienti (Plane 479).
const AdminPanelWebClientID = "admin-panel-web"

// panelWebCookieMax — cookie qiymatining yuqori chegarasi (sid + "." + 43).
const panelWebCookieMax = 256

// webBFFTouchEvery — idle muddati shundan tez-tez yozilmaydi (har so'rovda
// bazaga yozuv bo'lmasin).
const webBFFTouchEvery = time.Minute

// IsWebBFFClient — cookie (BFF) klientimi.
func IsWebBFFClient(clientID string) bool {
	return clientID == PanelWebClientID || clientID == AdminPanelWebClientID
}

func (s *Service) webBFFKey(clientID string) []byte {
	if clientID == PanelWebClientID {
		// Tarixiy nom — o'zgarsa merchant panelda hamma chiqib ketardi.
		return s.issuer.keys.derivedSecret("panel-web-cookie")
	}
	// Admin web cookie kaliti PLATFORMA kalitidan (A6 bilan bir xil ajratish):
	// merchant kaliti sizib chiqsa ham admin cookie'sini yasab bo'lmaydi.
	if clientID == AdminPanelWebClientID && s.issuer.platform != nil {
		return s.issuer.platform.derivedSecret("web-bff-cookie:" + clientID)
	}
	return s.issuer.keys.derivedSecret("web-bff-cookie:" + clientID)
}

func (s *Service) webBFFMAC(clientID, sid string) string {
	m := hmac.New(sha256.New, s.webBFFKey(clientID))
	m.Write([]byte(sid))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// WebBFFCookie — sessiya uchun cookie qiymati (chaqiruvchi sessiya shu
// klientniki ekanini tekshiradi).
func (s *Service) WebBFFCookie(clientID, sessionID string) string {
	return sessionID + "." + s.webBFFMAC(clientID, sessionID)
}

// WebBFFSessionID — imzosi to'g'ri cookie'dan sessiya ID'si (holat
// tekshirilMAYDI — chiqishda ishlatiladi).
func (s *Service) WebBFFSessionID(clientID, cookie string) (string, bool) {
	sid, sig, ok := strings.Cut(cookie, ".")
	if !ok || sid == "" || len(cookie) > panelWebCookieMax {
		return "", false
	}
	if !hmac.Equal([]byte(sig), []byte(s.webBFFMAC(clientID, sid))) {
		return "", false
	}
	return sid, true
}

// WebBFFAccess — cookie'dagi sessiya faol bo'lsa unga qisqa access token
// chiqaradi va harakatsizlik muddatini uzaytiradi. Sessiya topilmasa,
// boshqa klientniki, yopilgan/muddati o'tgan, akkaunt o'chirilgan yoki
// realmi o'zgargan bo'lsa — ErrNotFound (BFF 401 qaytaradi, cookie o'chadi).
func (s *Service) WebBFFAccess(ctx context.Context, clientID, cookie string) (string, time.Time, error) {
	sid, ok := s.WebBFFSessionID(clientID, cookie)
	if !ok {
		return "", time.Time{}, ErrNotFound
	}
	sess, err := s.store.Get(ctx, sid)
	if err != nil {
		return "", time.Time{}, ErrNotFound
	}
	now := s.now()
	if sess.ClientID != clientID || !sess.Active(now) {
		return "", time.Time{}, ErrNotFound
	}
	u, err := s.users.GetByID(ctx, sess.UserID)
	if err != nil || u.IsDeleted() {
		_ = s.revoke(ctx, sess, "account_gone")
		return "", time.Time{}, ErrNotFound
	}
	realm := u.Realm
	if realm == "" {
		realm = users.RealmForRole(u.Role)
	}
	if realm != sess.Realm {
		_ = s.revoke(ctx, sess, "realm_changed")
		return "", time.Time{}, ErrNotFound
	}
	// Faol foydalanuvchi — idle uzayadi (abs'dan oshmaydi, Store kafolati).
	if pol, ok := DefaultClients[clientID]; ok && pol.Idle < pol.Abs && now.Sub(sess.LastUsedAt) >= webBFFTouchEvery {
		if err := s.store.Touch(ctx, sess.ID, now, pol.Idle); err == nil {
			s.active.Invalidate(sess.ID)
		}
	}
	return s.issuer.issue(sess, sess.AbsExpiresAt)
}

// WebBFFSessionOf — kirish javobidagi access tokendan sessiya ID'si, faqat
// shu BFF klientining sessiyasi bo'lsa (cookie qo'yishdan oldin).
func (s *Service) WebBFFSessionOf(ctx context.Context, clientID, access string) (string, bool) {
	c, err := s.Authenticate(ctx, access)
	if err != nil || c.ClientID != clientID || c.SessionID == "" {
		return "", false
	}
	return c.SessionID, true
}

// WebBFFLogout — cookie sessiyasini yopadi (imzo to'g'ri bo'lsa).
func (s *Service) WebBFFLogout(ctx context.Context, clientID, cookie string) {
	sid, ok := s.WebBFFSessionID(clientID, cookie)
	if !ok {
		return
	}
	sess, err := s.store.Get(ctx, sid)
	if err != nil || sess.ClientID != clientID || sess.RevokedAt != nil {
		return
	}
	_ = s.revoke(ctx, sess, "logout")
}

// ── Merchant panel (RS-7) uchun eski nomlar ──────────────────────────────

func (s *Service) PanelWebCookie(sessionID string) string {
	return s.WebBFFCookie(PanelWebClientID, sessionID)
}

func (s *Service) PanelWebSessionID(cookie string) (string, bool) {
	return s.WebBFFSessionID(PanelWebClientID, cookie)
}

func (s *Service) PanelWebAccess(ctx context.Context, cookie string) (string, time.Time, error) {
	return s.WebBFFAccess(ctx, PanelWebClientID, cookie)
}

func (s *Service) PanelWebSessionOf(ctx context.Context, access string) (string, bool) {
	return s.WebBFFSessionOf(ctx, PanelWebClientID, access)
}

func (s *Service) PanelWebLogout(ctx context.Context, cookie string) {
	s.WebBFFLogout(ctx, PanelWebClientID, cookie)
}
