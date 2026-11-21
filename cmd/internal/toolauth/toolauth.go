// Package toolauth — ichki vositalar (yuklama testi, bench seed) uchun
// MARKAZIY sessiya ochish (ONDEXECOSY-483: eski HS256 token yo'q).
//
// API bilan BIR XIL issuer va audience ishlatiladi, sessiya bazadagi
// `auth_sessions` ga yoziladi — ya'ni token API'da odatiy mijoz tokeni kabi
// tekshiriladi va istalgan vaqt bekor qilinishi mumkin.
//
// Kalit: AUTH_SIGNING_KEY_FILE (Ed25519 PEM), issuer: AUTH_ISSUER yoki
// PUBLIC_API_URL. Access token qisqa muddatli (identity siyosati) — uzoq
// yuklama sinovi refresh token bilan yangilashi kerak.
package toolauth

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/httpapi"
	"chustapp/internal/identity"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// Audience — API kutadigan audience (`cmd/api` identityAudience bilan bir xil).
const Audience = "ondex-api"

// Sessions — vosita uchun identity xizmati.
type Sessions struct{ svc *identity.Service }

// New — kalit va issuer muhitdan olinadi.
func New(pool *pgxpool.Pool, ur users.Repository) (*Sessions, error) {
	path := strings.TrimSpace(os.Getenv("AUTH_SIGNING_KEY_FILE"))
	if path == "" {
		return nil, errors.New("AUTH_SIGNING_KEY_FILE kerak (markaziy sessiya kaliti)")
	}
	iss := strings.TrimRight(strings.TrimSpace(os.Getenv("AUTH_ISSUER")), "/")
	if iss == "" {
		iss = strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_API_URL")), "/")
	}
	if iss == "" {
		return nil, errors.New("AUTH_ISSUER (yoki PUBLIC_API_URL) kerak — API bilan bir xil bo'lishi shart")
	}
	key, err := identity.LoadPrivateKeyFile(path)
	if err != nil {
		return nil, err
	}
	keys, err := identity.NewKeySet(key)
	if err != nil {
		return nil, err
	}
	issuer, err := identity.NewIssuer(keys, iss, Audience)
	if err != nil {
		return nil, err
	}
	svc, err := identity.NewService(storage.NewPgAuthSessionStore(pool), issuer, ur, identity.NopAuditor{}, httpapi.NewID, nil)
	if err != nil {
		return nil, err
	}
	return &Sessions{svc: svc}, nil
}

// Open — foydalanuvchi uchun markaziy sessiya; mijoz -> ondex-customer,
// merchant xodimi -> ondex-pro. Admin (platform) ATAYLAB qo'llanmaydi.
func (s *Sessions) Open(ctx context.Context, u *users.User) (*identity.Tokens, error) {
	client := identity.CustomerClientID
	switch users.RealmForRole(u.Role) {
	case users.RealmCustomer:
	case users.RealmMerchant:
		client = "ondex-pro"
	default:
		return nil, errors.New("toolauth: bu realm uchun vosita sessiyasi ochilmaydi")
	}
	return s.svc.Create(ctx, identity.CreateInput{UserID: u.ID, ClientID: client, AMR: []string{"otp"},
		Meta: identity.Meta{DeviceName: "ondex-tool"}})
}
