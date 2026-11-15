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

// ┌─ MERCHANT WEB PANEL (RS-7) — BFF SESSIYASI ─────────────────────────────┐
// Flutter web panel (merchant.ondex.uz/r/) brauzerda token SAQLAMAYDI: XSS
// bo'lsa localStorage'dagi token o'g'irlanardi. Kirish odatdagi merchant
// oqimi bilan (parol/OTP + TOTP/passkey) `PanelWebClientID` klientida
// sessiya ochadi; BFF (`httpapi/panelweb_bff.go`) tokenlarni javobdan olib
// tashlaydi va brauzerga faqat HttpOnly cookie beradi: `sid.HMAC`.
//
// Cookie — faqat ko'rsatkich (SSO cookie kabi): sessiyaning o'zi markazda,
// bekor qilish markazda. Refresh token ISHLATILMAYDI — BFF shu jarayon
// ichida sessiya bo'yicha qisqa access token chiqaradi (`PanelWebAccess`),
// shuning uchun rotatsiya poygasi (parallel so'rovlar) muammosi yo'q.
// Klient `NoPublicRefresh`: uning refresh tokeni umumiy yo'ldan kelsa —
// o'g'irlik belgisi, sessiya yopiladi.
//
// Sessiya ID'si audit va "faol qurilmalar" ro'yxatida ko'rinadi, shuning
// uchun cookie sid'ning o'zi emas — kalit bilan imzolangan (HMAC).
// └─────────────────────────────────────────────────────────────────────────┘

// PanelWebClientID — merchant web panel (BFF) klienti.
const PanelWebClientID = "merchant-panel-web"

// panelWebCookieMax — cookie qiymatining yuqori chegarasi (sid + "." + 43).
const panelWebCookieMax = 256

func (s *Service) panelWebKey() []byte {
	return s.issuer.keys.derivedSecret("panel-web-cookie")
}

func (s *Service) panelWebMAC(sid string) string {
	m := hmac.New(sha256.New, s.panelWebKey())
	m.Write([]byte(sid))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// PanelWebCookie — sessiya uchun cookie qiymati. Faqat `PanelWebClientID`
// sessiyasi uchun chaqiriladi (chaqiruvchi tekshiradi).
func (s *Service) PanelWebCookie(sessionID string) string {
	return sessionID + "." + s.panelWebMAC(sessionID)
}

// PanelWebSessionID — imzosi to'g'ri cookie'dan sessiya ID'si (holati
// tekshirilMAYDI — chiqishda ishlatiladi).
func (s *Service) PanelWebSessionID(cookie string) (string, bool) {
	sid, sig, ok := strings.Cut(cookie, ".")
	if !ok || sid == "" || len(cookie) > panelWebCookieMax {
		return "", false
	}
	if !hmac.Equal([]byte(sig), []byte(s.panelWebMAC(sid))) {
		return "", false
	}
	return sid, true
}

// PanelWebAccess — cookie'dagi sessiya faol bo'lsa unga qisqa access token
// chiqaradi. Sessiya topilmasa, boshqa klientniki, yopilgan/muddati o'tgan,
// akkaunt o'chirilgan yoki realmi o'zgargan bo'lsa — ErrNotFound (BFF 401
// qaytaradi va cookie'ni o'chiradi).
func (s *Service) PanelWebAccess(ctx context.Context, cookie string) (string, time.Time, error) {
	sid, ok := s.PanelWebSessionID(cookie)
	if !ok {
		return "", time.Time{}, ErrNotFound
	}
	sess, err := s.store.Get(ctx, sid)
	if err != nil {
		return "", time.Time{}, ErrNotFound
	}
	if sess.ClientID != PanelWebClientID || !sess.Active(s.now()) {
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
	return s.issuer.issue(sess, sess.AbsExpiresAt)
}

// PanelWebSessionOf — kirish javobidagi access tokendan sessiya ID'si, faqat
// `PanelWebClientID` sessiyasi bo'lsa (BFF cookie qo'yishdan oldin).
func (s *Service) PanelWebSessionOf(ctx context.Context, access string) (string, bool) {
	c, err := s.Authenticate(ctx, access)
	if err != nil || c.ClientID != PanelWebClientID || c.SessionID == "" {
		return "", false
	}
	return c.SessionID, true
}

// PanelWebLogout — cookie sessiyasini yopadi (imzo to'g'ri bo'lsa).
func (s *Service) PanelWebLogout(ctx context.Context, cookie string) {
	sid, ok := s.PanelWebSessionID(cookie)
	if !ok {
		return
	}
	sess, err := s.store.Get(ctx, sid)
	if err != nil || sess.ClientID != PanelWebClientID || sess.RevokedAt != nil {
		return
	}
	_ = s.revoke(ctx, sess, "logout")
}
