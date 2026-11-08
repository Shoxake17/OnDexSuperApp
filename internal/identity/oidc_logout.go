package identity

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Chiqish (A4c.3, Plane ONDEXECOSY-201).
//
//   - RP-initiated logout (`/v2/oauth/logout`): OnDex SSO sessiyasi va
//     undan ochilgan barcha ilova sessiyalari yopiladi.
//   - Back-channel logout (OIDC Back-Channel Logout 1.0): sessiya markazda
//     QAYSI yo'l bilan yopilmasin (chiqish, "Qurilmalar" sahifasi, parol
//     almashishi, refresh o'g'irligi, xodim bo'shatilishi) — klientning
//     backchannel manziliga imzolangan `logout_token` POST qilinadi va
//     ShoxPos o'sha zahoti web sessiyani o'chiradi.
//
// Yuborish ASINXRON va bloklamaydi: chiqish tugmasi tashqi serverni kutib
// qotmaydi. Yetib bormasa ham xavfsizlik buzilmaydi — BFF har 10 daqiqada
// refresh qiladi va markaz yopilgan sessiyaga yangi token BERMAYDI (eng
// yomon holatda kechikish shu).

const backchannelEvent = "http://schemas.openid.net/event/backchannel-logout"

// LogoutClaims — logout_token (OIDC BCL 2.4). `nonce` ATAYLAB yo'q.
type LogoutClaims struct {
	SID    string                    `json:"sid"`
	Events map[string]map[string]any `json:"events"`
	jwt.RegisteredClaims
}

// backchannelPoster — test uchun almashtiriladi.
type backchannelPoster func(ctx context.Context, uri, token string) error

var bcHTTP = &http.Client{Timeout: 5 * time.Second,
	// Yo'naltirishga ergashilmaydi: logout_token boshqa hostga ketmasin.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func postBackchannel(ctx context.Context, uri, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uri,
		strings.NewReader(url.Values{"logout_token": {token}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := bcHTTP.Do(req)
	if err != nil {
		return err
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("backchannel: HTTP %d", res.StatusCode)
	}
	return nil
}

// WithBackchannelPoster — testlar uchun (tarmoqsiz).
func (p *Provider) WithBackchannelPoster(fn func(ctx context.Context, uri, token string) error) *Provider {
	p.post = fn
	return p
}

// onRevoke — `Service.OnRevoke` kuzatuvchisi.
func (p *Provider) onRevoke(ctx context.Context, sess *Session) {
	c, ok := p.clients[sess.ClientID]
	if !ok || c.BackchannelLogoutURI == "" {
		return
	}
	tok, err := p.logoutToken(c.ID, sess)
	if err != nil {
		slog.Error("oidc: logout_token yaratilmadi", "client", c.ID, "err", err)
		return
	}
	post := p.post
	go func() {
		bctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		var err error
		for i, wait := 0, 500*time.Millisecond; i < 3; i, wait = i+1, wait*4 {
			if err = post(bctx, c.BackchannelLogoutURI, tok); err == nil {
				return
			}
			select {
			case <-bctx.Done():
				i = 3
			case <-time.After(wait):
			}
		}
		slog.Warn("oidc: back-channel logout yetkazilmadi (BFF keyingi refresh'da baribir to'xtaydi)",
			"client", c.ID, "session", sess.ID, "err", err)
	}()
}

func (p *Provider) logoutToken(clientID string, sess *Session) (string, error) {
	now := p.svc.now()
	claims := LogoutClaims{SID: sess.ID, Events: map[string]map[string]any{backchannelEvent: {}},
		RegisteredClaims: jwt.RegisteredClaims{Issuer: p.svc.issuer.issuer, Subject: sess.UserID,
			Audience: jwt.ClaimStrings{clientID}, IssuedAt: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Minute)), ID: newJTI()}}
	t := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	t.Header["kid"] = p.svc.issuer.keys.active.KID
	t.Header["typ"] = "logout+jwt"
	return t.SignedString(p.svc.issuer.keys.active.Private)
}

// Hint — id_token_hint dan olingan ma'lumot.
type Hint struct {
	ClientID  string
	UserID    string
	SessionID string
}

// ParseIDTokenHint — imzo va iss tekshiriladi; MUDDAT tekshirilMAYDI
// (chiqish eski id_token bilan ham ishlashi kerak — OIDC RP-Initiated 2).
func (p *Provider) ParseIDTokenHint(raw string) (*Hint, error) {
	var c IDClaims
	_, err := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(p.svc.issuer.issuer), jwt.WithoutClaimsValidation()).
		ParseWithClaims(raw, &c, func(t *jwt.Token) (any, error) {
			kid, _ := t.Header["kid"].(string)
			pub, ok := p.svc.issuer.keys.PublicKey(kid)
			if !ok {
				return nil, errors.New("noma'lum kid")
			}
			return pub, nil
		})
	if err != nil || c.Issuer != p.svc.issuer.issuer || len(c.Audience) != 1 || c.SID == "" {
		return nil, ErrTokenInvalid
	}
	if _, ok := p.clients[c.Audience[0]]; !ok {
		return nil, ErrTokenInvalid
	}
	return &Hint{ClientID: c.Audience[0], UserID: c.Subject, SessionID: c.SID}, nil
}

// EndClientSession — id_token_hint dagi ilova sessiyasini yopadi (SSO
// cookie yo'q bo'lsa ham chiqish ishlasin). Begona sessiya — jim o'tadi.
func (p *Provider) EndClientSession(ctx context.Context, h *Hint) {
	if h == nil {
		return
	}
	sess, err := p.svc.store.Get(ctx, h.SessionID)
	if err != nil || sess.UserID != h.UserID || sess.ClientID != h.ClientID || sess.RevokedAt != nil {
		return
	}
	_ = p.svc.revoke(ctx, sess, "rp_logout")
	if sess.ParentID != "" {
		if parent, err := p.svc.store.Get(ctx, sess.ParentID); err == nil && parent.RevokedAt == nil {
			_, _ = p.svc.revokeTree(ctx, parent, "rp_logout")
		}
	}
}

// PostLogoutRedirect — ro'yxatdagi manzil bo'lsa (state bilan), aks holda "".
func (p *Provider) PostLogoutRedirect(clientID, uri, state string) string {
	c, ok := p.clients[clientID]
	if !ok || uri == "" || !slices.Contains(c.PostLogoutRedirectURIs, uri) {
		return ""
	}
	if state == "" {
		return uri
	}
	return appendQuery(uri, url.Values{"state": {state}})
}
