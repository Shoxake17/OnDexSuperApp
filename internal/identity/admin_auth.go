package identity

import (
	"context"
	"strings"
	"unicode/utf8"

	"chustapp/internal/identity/authz"
	"chustapp/internal/users"
)

// AdminClientID — admin paneli (platform realmi, A6). Sessiya: 30 daqiqa
// harakatsiz, 12 soat mutlaq (`DefaultClients`).
const AdminClientID = "admin-web"

// AdminLoginCode — OnDex egasi (platform realmi) kirishi, A6 / CC-0.1.
//
// Oqim: telefon + Telegram kodi -> parol (yo'q bo'lsa o'rnatiladi) ->
// TOTP: HAR kirishda (ulanmagan bo'lsa — ulash). Merchant egasidan farqi:
// "ishonchli qurilma" yo'q, kod + parol + TOTP har safar. Mavjudlik
// oshkor qilinmaydi: admin bo'lmagan raqam — oddiy "noto'g'ri".
func (m *MerchantAuth) AdminLoginCode(ctx context.Context, phone, code string, meta Meta) (*Outcome, error) {
	p, err := m.d.Users.VerifyPhoneCode(ctx, phone, code)
	if err != nil {
		return nil, err
	}
	u, err := m.d.Repo.GetByPhoneRealm(ctx, users.RealmPlatform, p)
	if err != nil || u.Role != users.RoleAdmin || u.IsDeleted() {
		m.svc.audit.Record(ctx, Event{Action: "admin.login", Result: ResultDenied,
			Realm: string(users.RealmPlatform), IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300),
			Details: map[string]any{"reason": "not_admin"}})
		return nil, users.ErrInvalidCredentials
	}
	if m.guard.Blocked(u.ID) {
		return nil, ErrMFATooMany
	}
	return m.adminSecondFactor(ctx, u, []string{"otp"})
}

// ── KYC: tashkilotni tasdiqlash (A6, AUTH-PHASE-A.md: "tasdiqlashni FAQAT
// egasi (platform) qiladi") ───────────────────────────────────────────────

// KYC qarorlari.
const (
	KYCApprove = "approve" // -> active: biznes mijozlarga ko'rinadi, to'lov ochiladi
	KYCReject  = "reject"  // -> rejected (sabab majburiy)
	KYCSuspend = "suspend" // -> suspended (sabab majburiy): kirish to'xtaydi
)

var kycStatus = map[string]string{KYCApprove: authz.OrgActive, KYCReject: authz.OrgRejected, KYCSuspend: authz.OrgSuspended}

func (m *MerchantAuth) requireAdmin(ctx context.Context, adminID string) error {
	u, err := m.d.Repo.GetByID(ctx, adminID)
	if err != nil || u.Role != users.RoleAdmin || u.IsDeleted() {
		return authz.ErrForbidden
	}
	return nil
}

// Organizations — KYC navbati (admin).
func (m *MerchantAuth) Organizations(ctx context.Context, adminID, status string) ([]*authz.Organization, error) {
	if err := m.requireAdmin(ctx, adminID); err != nil {
		return nil, err
	}
	if status != "" && status != authz.OrgPending && status != authz.OrgActive &&
		status != authz.OrgRejected && status != authz.OrgSuspended {
		return nil, invalid("holat noma'lum")
	}
	return m.d.Authz.Store().ListOrganizations(ctx, status, 200)
}

// DecideOrganization — KYC qarori: admin + Authenticator kodi (step-up) +
// sabab (rad/to'xtatishda) + audit. Ruxsatlar keshi darhol tozalanadi.
func (m *MerchantAuth) DecideOrganization(ctx context.Context, adminID, orgID, decision, reason, code string, meta Meta) (*authz.Organization, error) {
	if err := m.requireAdmin(ctx, adminID); err != nil {
		return nil, err
	}
	status, ok := kycStatus[decision]
	if !ok {
		return nil, invalid("qaror: approve, reject yoki suspend")
	}
	reason = strings.TrimSpace(reason)
	if decision != KYCApprove && reason == "" {
		return nil, invalid("rad etish yoki to'xtatish sababi majburiy")
	}
	if utf8.RuneCountInString(reason) > 500 {
		return nil, invalid("sabab juda uzun")
	}
	org, err := m.d.Authz.Store().GetOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if err := m.StepUp(ctx, adminID, code, "kyc."+decision, meta); err != nil {
		return nil, err
	}
	if err := m.d.Authz.Store().UpdateOrganizationStatus(ctx, org.ID, status, reason, adminID); err != nil {
		return nil, err
	}
	m.d.Authz.Invalidate()
	m.svc.audit.Record(ctx, Event{Action: "kyc." + decision, Result: ResultOK, ActorUserID: adminID,
		Realm: string(users.RealmPlatform), OrgID: org.ID, Target: org.ID, IPHash: m.svc.ipHash(meta.IP),
		UserAgent: clip(meta.UserAgent, 300), Details: map[string]any{"from": org.Status, "to": status, "reason": reason}})
	return m.d.Authz.Store().GetOrganization(ctx, org.ID)
}

// adminSecondFactor — admin uchun parol HAR DOIM so'raladi (kod yetarli
// emas), keyin TOTP. `secondFactor` TOTP ulangan akkauntda parolni
// so'ramaydi — admin uchun bu yetarli emas.
func (m *MerchantAuth) adminSecondFactor(ctx context.Context, u *users.User, amr []string) (*Outcome, error) {
	if u.PasswordHash == "" {
		// Parolsiz admin: kod bilan telefon isbotlangan — parol o'rnatiladi.
		tok, exp, err := m.signMFA(mfaClaims{User: u.ID, Client: AdminClientID, AMR: amr, Purpose: mfaPurposeFactor}, mfaVerifyTTL)
		if err != nil {
			return nil, err
		}
		return &Outcome{MFA: &Challenge{Token: tok, Method: "password_set", ExpiresAt: exp}}, nil
	}
	tok, exp, err := m.signMFA(mfaClaims{User: u.ID, Client: AdminClientID, AMR: amr, Purpose: mfaPurposeFactor}, mfaVerifyTTL)
	if err != nil {
		return nil, err
	}
	return &Outcome{MFA: &Challenge{Token: tok, Method: "password", ExpiresAt: exp}}, nil
}
