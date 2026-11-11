package identity

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"chustapp/internal/identity/authz"
	"chustapp/internal/identity/mfa"
	"chustapp/internal/users"
)

// Merchant katalogi (A3c, AUTH-PHASE-A.md 5, 7-bo'limlar): egasining
// ro'yxatdan o'tishi, `/v2/merchant/me`, xodimlar va rollar.
//
// ┌─ XAVFSIZLIK QOIDALARI ────────────────────────────────────────────────┐
// • Har amal `authz.Require` dan o'tadi: begona biznes = ErrNotFound (404).
// • Xodim qo'shish/o'zgartirish/bo'shatish — `staff.manage`; berilayotgan
//   rol `CanGrant` dan o'tadi (o'zida yo'q yoki xavfli ruxsatni bera olmaydi).
// • Egasi bo'lmagan menejer O'ZINI va o'zidan "yuqori" xodimni (rolida
//   menejerda yo'q ruxsat bor) o'zgartira/bo'shata olmaydi.
// • Egasining a'zoligi bu yerdan HECH QACHON o'zgarmaydi.
// • Bo'shatish: barcha sessiyalar DARHOL bekor + ruxsatlar keshi tozalanadi.
// • Xodim akkaunti — ALOHIDA merchant akkaunti (rol `merchant`); mijoz
//   akkaunti va uning tarixi tegilmaydi (0069).
// └───────────────────────────────────────────────────────────────────────┘

// ErrAccountExists — bu raqamda allaqachon merchant akkaunti bor (kirish kerak).
var ErrAccountExists = errors.New("bu raqamda biznes akkaunti bor — kirish sahifasidan kiring")

// SignupInput — yangi biznes egasi (telefon kodi bilan isbotlangan).
type SignupInput struct {
	Phone, Code, Password, PasswordConfirm string
	FirstName, LastName                    string
	OrgName, LegalForm, TIN                string
	BusinessType                           authz.BusinessType
	BusinessName                           string
	ClientID                               string
}

func invalid(msg string) error { return fmt.Errorf("%w: %s", authz.ErrInvalid, msg) }

func cleanName(v string, max int, what string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" || utf8.RuneCountInString(v) > max {
		return "", invalid(what + " 1–" + fmt.Sprint(max) + " belgi bo'lishi kerak")
	}
	return v, nil
}

// Signup — akkaunt + tashkilot (pending: KYC'ni platform tasdiqlaydi) +
// birinchi biznes + egalik. Telefon kodi VA parol shu yerda isbotlanadi,
// shuning uchun keyingi qadam darhol TOTP ulash.
func (m *MerchantAuth) Signup(ctx context.Context, in SignupInput, meta Meta) (*Outcome, error) {
	app, err := m.appFor(in.ClientID)
	if err != nil {
		return nil, err
	}
	if app != authz.AppPanel {
		return nil, invalid("ro'yxatdan o'tish faqat Merchant panelda")
	}
	// Arzon tekshiruvlar kod ISHLATILISHIDAN oldin: forma xatosi kodni yoqmasin.
	v, err := users.ValidateRegisterInput(users.RegisterInput{Phone: in.Phone, FirstName: in.FirstName,
		LastName: in.LastName, Password: in.Password, PasswordConfirm: in.PasswordConfirm})
	if err != nil {
		return nil, err
	}
	if !v.ByPhone {
		return nil, users.ErrInvalidPhone
	}
	orgName, err := cleanName(in.OrgName, 200, "tashkilot nomi")
	if err != nil {
		return nil, err
	}
	bizName, err := cleanName(in.BusinessName, 200, "biznes nomi")
	if err != nil {
		return nil, err
	}
	tin := strings.TrimSpace(in.TIN)
	if tin != "" && (len(tin) != 9 || strings.Trim(tin, "0123456789") != "") {
		return nil, invalid("STIR 9 ta raqamdan iborat")
	}
	if !slices.Contains([]string{"", "llc", "sole", "other"}, in.LegalForm) {
		return nil, invalid("yuridik shakl noma'lum")
	}
	if !in.BusinessType.Valid() {
		return nil, invalid("biznes turi noma'lum")
	}

	phone, err := m.d.Users.VerifyPhoneCode(ctx, v.Phone, in.Code)
	if err != nil {
		return nil, err
	}
	u, err := m.signupAccount(ctx, phone, v)
	if err != nil {
		return nil, err
	}
	az := m.d.Authz.Store()
	org := &authz.Organization{ID: m.svc.newID(), Name: orgName, LegalForm: in.LegalForm, TIN: tin,
		Status: authz.OrgPending, OwnerUserID: u.ID}
	if err := az.CreateOrganization(ctx, org); err != nil {
		if errors.Is(err, authz.ErrConflict) {
			return nil, fmt.Errorf("%w: bu STIR bilan tashkilot allaqachon ro'yxatdan o'tgan", authz.ErrConflict)
		}
		return nil, err
	}
	biz := &authz.Business{ID: m.svc.newID(), OrgID: org.ID, Type: in.BusinessType, Name: bizName}
	if err := az.CreateBusiness(ctx, biz); err != nil {
		return nil, err
	}
	if err := az.CreateMembership(ctx, &authz.Membership{ID: m.svc.newID(), UserID: u.ID, OrgID: org.ID,
		IsOwner: true, Status: authz.MemberActive, CreatedBy: u.ID}); err != nil {
		return nil, err
	}
	m.d.Authz.Invalidate()
	m.audit(ctx, u.ID, org.ID, biz.ID, "merchant.signup", org.ID, meta, map[string]any{"business_type": string(in.BusinessType)})
	// Ro'yxatdan o'tish formasi parolni hozircha so'raydi — egasi uni O'ZI
	// tanlagan, shuning uchun parol bilan kirish yoqiq (akkaunt sahifasida
	// o'chirsa bo'ladi).
	if err := m.EnablePasswordAtSignup(ctx, u.ID); err != nil {
		return nil, err
	}
	return m.secondFactor(ctx, u, in.ClientID, org.ID, []string{"otp", "pwd"})
}

// signupAccount — yangi merchant akkaunti. Avvalgi urinish yarim qolgan
// bo'lsa (akkaunt bor, lekin hech qaysi biznesda emas) — o'sha akkaunt
// davom ettiriladi (telefon egaligi hozirgina isbotlandi).
func (m *MerchantAuth) signupAccount(ctx context.Context, phone string, v users.ValidatedRegister) (*users.User, error) {
	hash, err := users.HashPassword(ctx, v.Password)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(v.FirstName + " " + v.LastName)
	u, err := m.d.Repo.GetByPhoneRealm(ctx, users.RealmMerchant, phone)
	switch {
	case err == nil:
		gs, gerr := m.d.Authz.Store().Grants(ctx, u.ID)
		if gerr != nil {
			return nil, gerr
		}
		if u.Role != users.RoleMerchant || len(gs) > 0 || u.IsDeleted() {
			return nil, ErrAccountExists
		}
		if err := m.d.Repo.SetPasswordHash(ctx, u.ID, hash); err != nil {
			return nil, err
		}
		first, last := v.FirstName, v.LastName
		if err := m.d.Repo.UpdateProfile(ctx, u.ID, users.ProfileUpdate{FirstName: &first, LastName: &last}); err != nil {
			return nil, err
		}
		if err := m.d.Repo.MarkPhoneVerified(ctx, u.ID); err != nil {
			return nil, err
		}
		return m.d.Repo.GetByID(ctx, u.ID)
	case !errors.Is(err, users.ErrUserNotFound):
		return nil, err
	}
	u = &users.User{ID: m.svc.newID(), Phone: phone, Name: name, FirstName: v.FirstName, LastName: v.LastName,
		Role: users.RoleMerchant, PasswordHash: hash, PhoneVerified: true, CreatedAt: m.svc.now()}
	if err := m.d.Repo.Create(ctx, u); err != nil {
		if errors.Is(err, users.ErrPhoneTaken) {
			return nil, ErrAccountExists // parallel ikkinchi ro'yxatdan o'tish
		}
		return nil, err
	}
	return u, nil
}

func (m *MerchantAuth) audit(ctx context.Context, actor, org, biz, action, target string, meta Meta, details map[string]any) {
	m.svc.audit.Record(ctx, Event{Action: action, Result: ResultOK, ActorUserID: actor,
		Realm: string(users.RealmMerchant), OrgID: org, BusinessID: biz, Target: target,
		IPHash: m.svc.ipHash(meta.IP), UserAgent: clip(meta.UserAgent, 300), Details: details})
}

// activateInvites — telefon kodi bilan kirgan xodimning muddati o'tmagan
// takliflari faollashadi (raqam egaligi hozirgina isbotlandi).
func (m *MerchantAuth) activateInvites(ctx context.Context, u *users.User, meta Meta) {
	acts, err := m.d.Authz.Store().ActivateInvites(ctx, u.ID, m.svc.now().Add(-authz.InviteTTL))
	if err != nil || len(acts) == 0 {
		return
	}
	m.d.Authz.Invalidate()
	if !u.PhoneVerified {
		_ = m.d.Repo.MarkPhoneVerified(ctx, u.ID)
	}
	for _, a := range acts {
		m.audit(ctx, u.ID, a.OrgID, a.BusinessID, "staff.invite_accepted", a.ID, meta, nil)
	}
}

// Access — foydalanuvchining biznesdagi effektiv huquqi (HTTP qatlami
// sessiya ilovasini a'zolik ilovalari bilan solishtiradi).
func (m *MerchantAuth) Access(ctx context.Context, userID, bizID string) (*authz.Access, error) {
	return m.d.Authz.Check(ctx, userID, bizID)
}

// ── /v2/merchant/me ───────────────────────────────────────────────────

type MeUser struct {
	ID    string `json:"id"`
	Phone string `json:"phone"`
	Name  string `json:"name"`
}

type BusinessView struct {
	ID          string             `json:"id"`
	OrgID       string             `json:"org_id"`
	OrgStatus   string             `json:"org_status"`
	Type        authz.BusinessType `json:"type"`
	Name        string             `json:"name"`
	Status      string             `json:"status"`
	IsOwner     bool               `json:"is_owner"`
	Role        string             `json:"role"`
	Apps        []string           `json:"apps"`
	Permissions []authz.Permission `json:"permissions"`
}

type MeView struct {
	User       MeUser         `json:"user"`
	MFAEnabled bool           `json:"mfa_enabled"`
	Businesses []BusinessView `json:"businesses"`
}

func (m *MerchantAuth) Me(ctx context.Context, userID string) (*MeView, error) {
	u, err := m.d.Repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	list, err := m.d.Authz.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := &MeView{User: MeUser{ID: u.ID, Phone: u.Phone, Name: u.Name}, Businesses: []BusinessView{}}
	if _, err := m.d.MFA.Confirmed(ctx, userID); err == nil {
		out.MFAEnabled = true
	} else if !errors.Is(err, mfa.ErrNotFound) {
		return nil, err
	}
	for _, a := range list {
		out.Businesses = append(out.Businesses, BusinessView{ID: a.Business.ID, OrgID: a.Business.OrgID,
			OrgStatus: a.OrgStatus, Type: a.Business.Type, Name: a.Business.Name, Status: a.Business.Status,
			IsOwner: a.IsOwner, Role: a.RoleKey, Apps: a.Apps(), Permissions: a.Permissions()})
	}
	return out, nil
}

// ── Xodimlar ──────────────────────────────────────────────────────────

type StaffView struct {
	MembershipID  string    `json:"membership_id"`
	UserID        string    `json:"user_id"`
	Phone         string    `json:"phone"`
	Name          string    `json:"name"`
	RoleID        string    `json:"role_id"`
	RoleKey       string    `json:"role_key"`
	RoleName      string    `json:"role_name"`
	Apps          []string  `json:"apps"`
	Status        string    `json:"status"`
	InvitedAt     time.Time `json:"invited_at,omitzero"`
	InviteExpired bool      `json:"invite_expired"`
}

func (m *MerchantAuth) staffView(ctx context.Context, mb *authz.Membership) StaffView {
	v := StaffView{MembershipID: mb.ID, UserID: mb.UserID, RoleID: mb.RoleID, Apps: mb.Apps, Status: mb.Status,
		InvitedAt: mb.InvitedAt}
	if v.Apps == nil {
		v.Apps = []string{}
	}
	if mb.Status == authz.MemberInvited && m.svc.now().Sub(mb.InvitedAt) > authz.InviteTTL {
		v.InviteExpired = true
	}
	if u, err := m.d.Repo.GetByID(ctx, mb.UserID); err == nil {
		v.Phone, v.Name = u.Phone, u.Name
	}
	if r, err := m.d.Authz.Store().GetRole(ctx, mb.RoleID); err == nil {
		v.RoleKey, v.RoleName = r.Key, r.Name
	}
	return v
}

// ListStaff — `staff.view`.
func (m *MerchantAuth) ListStaff(ctx context.Context, actorID, bizID string) ([]StaffView, error) {
	if _, err := m.d.Authz.Require(ctx, actorID, bizID, authz.PermStaffView); err != nil {
		return nil, err
	}
	ms, err := m.d.Authz.Store().Members(ctx, bizID)
	if err != nil {
		return nil, err
	}
	out := make([]StaffView, 0, len(ms))
	for _, mb := range ms {
		out = append(out, m.staffView(ctx, mb))
	}
	return out, nil
}

// stepUpForGrant — xavfli ruxsat (pul yechish, xodimlarni boshqarish va h.k.,
// `authz.IsDangerous`) berilayotgan bo'lsa egasi Authenticator kodi bilan
// HOZIR qayta tasdiqlashi SHART (AUTH-PHASE-A.md 5-bo'lim, Plane 135):
// o'g'irlangan sessiya bilan o'ziga yoki sherigiga pul huquqini berib
// bo'lmaydi. Tekshiruv forma tekshiruvlaridan KEYIN — xato forma TOTP
// qadamini yemaydi.
func (m *MerchantAuth) stepUpForGrant(ctx context.Context, acc *authz.Access, perms []authz.Permission, code, action string, meta Meta) error {
	for _, p := range perms {
		if authz.IsDangerous(acc.Business.Type, p) {
			return m.StepUp(ctx, acc.UserID, code, action, meta)
		}
	}
	return nil
}

// grantableRole — rol shu biznesda ishlatila oladi va `acc` uni bera oladi.
func (m *MerchantAuth) grantableRole(ctx context.Context, acc *authz.Access, roleID string) (*authz.Role, error) {
	r, err := m.d.Authz.Store().GetRole(ctx, roleID)
	if errors.Is(err, authz.ErrNotFound) {
		return nil, invalid("rol topilmadi")
	}
	if err != nil {
		return nil, err
	}
	if r.BusinessType != acc.Business.Type || (r.OrgID != "" && r.OrgID != acc.Business.OrgID) {
		return nil, invalid("rol bu biznes uchun emas")
	}
	if err := authz.CanGrant(acc, r.Permissions); err != nil {
		return nil, err
	}
	return r, nil
}

func validApps(apps []string) ([]string, error) {
	out := []string{}
	for _, a := range apps {
		if !authz.ValidApp(a) {
			return nil, invalid("ilova noma'lum: " + a)
		}
		if !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out, nil
}

// AddStaffInput — yangi xodim (taklif).
type AddStaffInput struct {
	Phone, FirstName, LastName string
	RoleID                     string
	Apps                       []string // nil — rol shablonidagi standart ilovalar
	// StepUpCode — xavfli ruxsatli rol berilsa: egasining Authenticator kodi.
	StepUpCode string
}

// AddStaff — merchant akkaunti (yo'q bo'lsa yaratiladi) + `invited` a'zolik.
// Xodim shu raqamga kelgan kod bilan 24 soat ichida kirsa — faollashadi.
func (m *MerchantAuth) AddStaff(ctx context.Context, actorID, bizID string, in AddStaffInput, meta Meta) (*StaffView, error) {
	acc, err := m.d.Authz.Require(ctx, actorID, bizID, authz.PermStaffManage)
	if err != nil {
		return nil, err
	}
	role, err := m.grantableRole(ctx, acc, in.RoleID)
	if err != nil {
		return nil, err
	}
	apps := in.Apps
	if apps == nil {
		for _, t := range authz.Templates() {
			if t.ID() == role.ID {
				apps = t.Apps
			}
		}
	}
	if apps, err = validApps(apps); err != nil {
		return nil, err
	}
	phone, err := users.NormalizePhone(in.Phone)
	if err != nil {
		return nil, err
	}
	first, err := cleanName(in.FirstName, users.MaxNameLength, "ism")
	if err != nil {
		return nil, err
	}
	last := strings.TrimSpace(in.LastName)
	if utf8.RuneCountInString(last) > users.MaxNameLength {
		return nil, invalid("familiya juda uzun")
	}
	if err := m.stepUpForGrant(ctx, acc, role.Permissions, in.StepUpCode, "staff.invite", meta); err != nil {
		return nil, err
	}

	u, err := m.d.Repo.GetByPhoneRealm(ctx, users.RealmMerchant, phone)
	if errors.Is(err, users.ErrUserNotFound) {
		u = &users.User{ID: m.svc.newID(), Phone: phone, FirstName: first, LastName: last,
			Name: strings.TrimSpace(first + " " + last), Role: users.RoleMerchant, CreatedAt: m.svc.now()}
		if err = m.d.Repo.Create(ctx, u); errors.Is(err, users.ErrPhoneTaken) {
			u, err = m.d.Repo.GetByPhoneRealm(ctx, users.RealmMerchant, phone) // parallel yaratilgan
		}
	}
	if err != nil {
		return nil, err
	}
	if u.ID == actorID {
		return nil, invalid("o'zingizni xodim sifatida qo'sha olmaysiz")
	}
	if u.IsDeleted() {
		return nil, invalid("bu akkaunt o'chirilgan")
	}
	mb := &authz.Membership{ID: m.svc.newID(), UserID: u.ID, OrgID: acc.Business.OrgID, BusinessID: bizID,
		RoleID: role.ID, Apps: apps, Status: authz.MemberInvited, CreatedBy: actorID, InvitedAt: m.svc.now()}
	if err := m.d.Authz.Store().CreateMembership(ctx, mb); err != nil {
		if errors.Is(err, authz.ErrConflict) {
			return nil, fmt.Errorf("%w: bu odam allaqachon shu biznes xodimi (qayta taklif yuboring)", authz.ErrConflict)
		}
		return nil, err
	}
	m.d.Authz.Invalidate()
	m.audit(ctx, actorID, acc.Business.OrgID, bizID, "staff.invite", mb.ID, meta,
		map[string]any{"role": role.Key, "apps": apps})
	m.notifyInvite(ctx, mb, acc.Business.Name)
	v := m.staffView(ctx, mb)
	return &v, nil
}

// target — o'zgartiriladigan xodim: shu biznesda, egasi emas, va `acc`
// unga tegishga haqli (egasi bo'lmasa: o'zi emas, rolidagi hamma ruxsat
// `acc` da bor).
func (m *MerchantAuth) target(ctx context.Context, acc *authz.Access, mid string) (*authz.Membership, error) {
	mb, err := m.d.Authz.Store().GetMembership(ctx, mid)
	if err != nil || mb.IsOwner || mb.BusinessID != acc.Business.ID {
		return nil, authz.ErrNotFound
	}
	if acc.IsOwner {
		return mb, nil
	}
	if mb.UserID == acc.UserID {
		return nil, fmt.Errorf("%w: o'z a'zoligingizni o'zgartira olmaysiz", authz.ErrForbidden)
	}
	r, err := m.d.Authz.Store().GetRole(ctx, mb.RoleID)
	if err != nil {
		return nil, err
	}
	if err := authz.CanGrant(acc, r.Permissions); err != nil {
		return nil, fmt.Errorf("%w: bu xodimning huquqlari sizdan yuqori", authz.ErrForbidden)
	}
	return mb, nil
}

// UpdateStaffInput — bo'sh RoleID / nil Apps = o'zgarmaydi.
type UpdateStaffInput struct {
	RoleID     string
	Apps       []string
	StepUpCode string // xavfli ruxsatli rolga o'tkazishda
}

func (m *MerchantAuth) UpdateStaff(ctx context.Context, actorID, bizID, mid string, in UpdateStaffInput, meta Meta) (*StaffView, error) {
	acc, err := m.d.Authz.Require(ctx, actorID, bizID, authz.PermStaffManage)
	if err != nil {
		return nil, err
	}
	mb, err := m.target(ctx, acc, mid)
	if err != nil {
		return nil, err
	}
	p := authz.MembershipPatch{}
	if in.RoleID != "" {
		r, err := m.grantableRole(ctx, acc, in.RoleID)
		if err != nil {
			return nil, err
		}
		if err := m.stepUpForGrant(ctx, acc, r.Permissions, in.StepUpCode, "staff.update", meta); err != nil {
			return nil, err
		}
		p.RoleID = r.ID
	}
	if in.Apps != nil {
		if p.Apps, err = validApps(in.Apps); err != nil {
			return nil, err
		}
	}
	if err := m.d.Authz.Store().UpdateMembership(ctx, mb.ID, p); err != nil {
		return nil, err
	}
	m.d.Authz.Invalidate()
	m.audit(ctx, actorID, acc.Business.OrgID, bizID, "staff.update", mb.ID, meta,
		map[string]any{"role_id": p.RoleID, "apps": p.Apps})
	mb, err = m.d.Authz.Store().GetMembership(ctx, mb.ID)
	if err != nil {
		return nil, err
	}
	v := m.staffView(ctx, mb)
	return &v, nil
}

// DismissStaff — bo'shatish: a'zolik `dismissed`, barcha sessiyalar DARHOL bekor.
func (m *MerchantAuth) DismissStaff(ctx context.Context, actorID, bizID, mid string, meta Meta) error {
	acc, err := m.d.Authz.Require(ctx, actorID, bizID, authz.PermStaffManage)
	if err != nil {
		return err
	}
	mb, err := m.target(ctx, acc, mid)
	if err != nil {
		return err
	}
	if err := m.d.Authz.Store().UpdateMembership(ctx, mb.ID, authz.MembershipPatch{Status: authz.MemberDismissed}); err != nil {
		return err
	}
	m.d.Authz.Invalidate()
	if _, err := m.svc.RevokeAllForUser(ctx, mb.UserID, "staff_dismissed"); err != nil {
		return err
	}
	m.audit(ctx, actorID, acc.Business.OrgID, bizID, "staff.dismiss", mb.ID, meta, nil)
	return nil
}

// ReinviteStaff — taklifni yangilaydi (eskirgan taklif yoki bo'shatilgan
// xodimni qaytarish). Faol xodimga — xato.
func (m *MerchantAuth) ReinviteStaff(ctx context.Context, actorID, bizID, mid string, meta Meta) (*StaffView, error) {
	acc, err := m.d.Authz.Require(ctx, actorID, bizID, authz.PermStaffManage)
	if err != nil {
		return nil, err
	}
	mb, err := m.target(ctx, acc, mid)
	if err != nil {
		return nil, err
	}
	if mb.Status == authz.MemberActive {
		return nil, invalid("xodim allaqachon faol")
	}
	now := m.svc.now()
	if err := m.d.Authz.Store().UpdateMembership(ctx, mb.ID,
		authz.MembershipPatch{Status: authz.MemberInvited, InvitedAt: &now}); err != nil {
		return nil, err
	}
	m.d.Authz.Invalidate()
	m.audit(ctx, actorID, acc.Business.OrgID, bizID, "staff.reinvite", mb.ID, meta, nil)
	mb, err = m.d.Authz.Store().GetMembership(ctx, mb.ID)
	if err != nil {
		return nil, err
	}
	m.notifyInvite(ctx, mb, acc.Business.Name)
	v := m.staffView(ctx, mb)
	return &v, nil
}

// ── Rollar ────────────────────────────────────────────────────────────

type RolesView struct {
	Roles   []*authz.Role    `json:"roles"`
	Catalog []authz.PermInfo `json:"catalog"`
}

// Roles — shablonlar + tashkilot rollari + ruxsatlar katalogi (`staff.view`).
func (m *MerchantAuth) Roles(ctx context.Context, actorID, bizID string) (*RolesView, error) {
	acc, err := m.d.Authz.Require(ctx, actorID, bizID, authz.PermStaffView)
	if err != nil {
		return nil, err
	}
	rs, err := m.d.Authz.Store().Roles(ctx, acc.Business.OrgID, acc.Business.Type)
	if err != nil {
		return nil, err
	}
	if rs == nil {
		rs = []*authz.Role{}
	}
	return &RolesView{Roles: rs, Catalog: authz.Catalog(acc.Business.Type)}, nil
}

// CreateRoleInput — tashkilotning maxsus roli.
type CreateRoleInput struct {
	Key, Name   string
	Permissions []authz.Permission
	StepUpCode  string // xavfli ruxsatli rol yaratishda
}

func (m *MerchantAuth) CreateRole(ctx context.Context, actorID, bizID string, in CreateRoleInput, meta Meta) (*authz.Role, error) {
	acc, err := m.d.Authz.Require(ctx, actorID, bizID, authz.PermStaffManage)
	if err != nil {
		return nil, err
	}
	if err := authz.CanGrant(acc, in.Permissions); err != nil {
		return nil, err
	}
	name, err := cleanName(in.Name, 80, "rol nomi")
	if err != nil {
		return nil, err
	}
	if err := m.stepUpForGrant(ctx, acc, in.Permissions, in.StepUpCode, "role.create", meta); err != nil {
		return nil, err
	}
	r := &authz.Role{ID: m.svc.newID(), OrgID: acc.Business.OrgID, BusinessType: acc.Business.Type,
		Key: strings.TrimSpace(in.Key), Name: name, Permissions: in.Permissions, CreatedBy: actorID}
	if err := m.d.Authz.Store().CreateRole(ctx, r); err != nil {
		return nil, err
	}
	m.audit(ctx, actorID, acc.Business.OrgID, bizID, "role.create", r.ID, meta,
		map[string]any{"key": r.Key, "permissions": r.Permissions})
	return r, nil
}
