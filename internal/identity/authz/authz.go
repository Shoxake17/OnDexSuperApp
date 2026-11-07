package authz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"chustapp/internal/ttlcache"
)

var (
	// ErrNotFound — biznes yo'q YOKI foydalanuvchi unga a'zo emas (HTTP 404:
	// begona biznes borligi oshkor bo'lmaydi).
	ErrNotFound = errors.New("topilmadi")
	// ErrForbidden — o'z biznesi, lekin bu amalga ruxsat yo'q (HTTP 403).
	ErrForbidden = errors.New("bu amal uchun ruxsat yo'q")
	// ErrSuspended — tashkilot yoki biznes to'xtatilgan (HTTP 403).
	ErrSuspended = errors.New("biznes to'xtatilgan — OnDex qo'llab-quvvatlash xizmatiga murojaat qiling")
	// ErrAppNotAllowed — a'zolik bu ilovaga ruxsat bermaydi (HTTP 403).
	ErrAppNotAllowed = errors.New("bu ilovaga kirish ruxsati yo'q")
	// ErrInvalid — kiritilgan ma'lumot noto'g'ri (HTTP 400).
	ErrInvalid = errors.New("noto'g'ri ma'lumot")
	// ErrConflict — unikal cheklov (HTTP 409).
	ErrConflict = errors.New("allaqachon mavjud")
)

// Holatlar (`organizations.status`, `businesses.status`, `memberships.status`).
const (
	OrgPending   = "pending"
	OrgActive    = "active"
	OrgSuspended = "suspended"
	OrgRejected  = "rejected"

	BusinessActive    = "active"
	BusinessSuspended = "suspended"
	BusinessClosed    = "closed"

	MemberInvited   = "invited"
	MemberActive    = "active"
	MemberSuspended = "suspended"
	MemberDismissed = "dismissed"
)

type Organization struct {
	ID          string
	Name        string
	LegalForm   string // "" | llc | sole | other
	TIN         string // STIR, 9 raqam yoki ""
	Status      string
	OwnerUserID string
	CreatedAt   time.Time
}

type Business struct {
	ID             string
	OrgID          string
	Type           BusinessType
	LegacyEntityID string
	Name           string
	Status         string
	CreatedAt      time.Time
}

type Role struct {
	ID           string
	OrgID        string // "" = tizim shabloni
	BusinessType BusinessType
	Key          string
	Name         string
	Permissions  []Permission
	CreatedBy    string
}

type Membership struct {
	ID            string
	UserID        string
	OrgID         string
	BusinessID    string // "" = egasi (tashkilotning hamma biznesi)
	RoleID        string // "" = egasi
	IsOwner       bool
	Apps          []string
	Status        string
	StaffMemberID string
	CreatedBy     string
	CreatedAt     time.Time
}

// Grant — ombordan kelgan xom huquq: foydalanuvchining shu biznesdagi FAOL
// a'zoligi (egalik yoki xodimlik) va tegishli rol. Effektiv ruxsatlarni
// `Authorizer` katalog bo'yicha hisoblaydi (ombor ruxsatga ishonmaydi).
type Grant struct {
	Business     Business
	OrgStatus    string
	MembershipID string
	IsOwner      bool
	Apps         []string
	Role         *Role // egasi uchun nil
}

// Store — merchant katalogi ombori (Postgres: 0064, 0065).
type Store interface {
	// Grant — `userID` ning `businessID` dagi FAOL a'zoligi. Egalik ustun.
	// A'zolik yo'q yoki biznes yo'q — ErrNotFound.
	Grant(ctx context.Context, userID, businessID string) (*Grant, error)
	// Grants — foydalanuvchining barcha bizneslari (egasi bo'lgan tashkilot
	// bizneslari + xodim sifatida faol a'zoliklari).
	Grants(ctx context.Context, userID string) ([]*Grant, error)

	CreateOrganization(ctx context.Context, o *Organization) error
	GetOrganization(ctx context.Context, id string) (*Organization, error)
	CreateBusiness(ctx context.Context, b *Business) error
	GetBusiness(ctx context.Context, id string) (*Business, error)

	// UpsertTemplates — tizim shablonlarini (org_id NULL) yozadi/yangilaydi.
	UpsertTemplates(ctx context.Context, roles []*Role) error
	CreateRole(ctx context.Context, r *Role) error
	GetRole(ctx context.Context, id string) (*Role, error)
	// Roles — shablonlar + tashkilotning o'z rollari (shu tur uchun).
	Roles(ctx context.Context, orgID string, t BusinessType) ([]*Role, error)

	CreateMembership(ctx context.Context, m *Membership) error
	GetMembership(ctx context.Context, id string) (*Membership, error)
	// Members — biznes xodimlari (egasi kirmaydi), bo'shatilganlar ham.
	Members(ctx context.Context, businessID string) ([]*Membership, error)
	// UpdateMembership — rol, ilovalar, holat. Bo'sh qiymat = o'zgarmaydi
	// (`apps` uchun nil). Topilmasa ErrNotFound.
	UpdateMembership(ctx context.Context, id string, p MembershipPatch) error
}

type MembershipPatch struct {
	RoleID string
	Apps   []string
	Status string
}

// Access — tekshirilgan, effektiv huquq.
type Access struct {
	UserID       string
	Business     Business
	MembershipID string
	IsOwner      bool
	RoleKey      string
	apps         []string
	perms        map[Permission]bool
}

func (a *Access) Has(p Permission) bool { return a.perms[p] }

// Permissions — tartiblangan ro'yxat (`/v2/merchant/me` javobi).
func (a *Access) Permissions() []Permission {
	out := make([]Permission, 0, len(a.perms))
	for p := range a.perms {
		out = append(out, p)
	}
	return sortPerms(out)
}

// Apps — kirish mumkin bo'lgan ilovalar. Egasi — hammasi.
func (a *Access) Apps() []string {
	if a.IsOwner {
		return []string{AppPanel, AppPro, AppGo}
	}
	return slices.Clone(a.apps)
}

func (a *Access) AllowsApp(app string) bool { return app != "" && slices.Contains(a.Apps(), app) }

// accessCacheTTL — boshqa nusxada o'zgargan a'zolik ko'pi bilan shuncha
// vaqt eski holatda ishlaydi; shu nusxadagi o'zgarish keshni DARHOL tozalaydi.
const accessCacheTTL = 30 * time.Second

// Authorizer — `Require` ning yagona manbai.
type Authorizer struct {
	store Store
	cache *ttlcache.Cache[*Access]
}

func New(store Store) *Authorizer {
	return &Authorizer{store: store, cache: ttlcache.New[*Access](accessCacheTTL, 50_000)}
}

func (a *Authorizer) Store() Store { return a.store }

// Invalidate — a'zolik/rol/holat o'zgargach (shu nusxada darhol kuchga kiradi).
func (a *Authorizer) Invalidate() { a.cache.Clear() }

// Check — foydalanuvchining biznesdagi effektiv huquqi.
func (a *Authorizer) Check(ctx context.Context, userID, businessID string) (*Access, error) {
	if userID == "" || businessID == "" || strings.Contains(userID, "|") {
		return nil, ErrNotFound
	}
	acc, err := a.cache.Get(userID+"|"+businessID, func() (*Access, error) {
		g, err := a.store.Grant(ctx, userID, businessID)
		if err != nil {
			return nil, err
		}
		return effective(userID, g)
	})
	if err != nil {
		return nil, err
	}
	return acc, nil
}

// Require — huquq + ruxsat. Rad etish: ErrNotFound (404) | ErrSuspended,
// ErrForbidden (403).
func (a *Authorizer) Require(ctx context.Context, userID, businessID string, p Permission) (*Access, error) {
	acc, err := a.Check(ctx, userID, businessID)
	if err != nil {
		return nil, err
	}
	if !acc.Has(p) {
		return nil, ErrForbidden
	}
	return acc, nil
}

// List — foydalanuvchining barcha bizneslaridagi huquqlari (to'xtatilganlari
// tashlab ketiladi).
func (a *Authorizer) List(ctx context.Context, userID string) ([]*Access, error) {
	gs, err := a.store.Grants(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]*Access, 0, len(gs))
	for _, g := range gs {
		if acc, err := effective(userID, g); err == nil {
			out = append(out, acc)
		}
	}
	return out, nil
}

// effective — xom huquqdan effektiv ruxsatlar. Ombordagi ma'lumotga TO'LIQ
// ishonilmaydi: rol boshqa tur/tashkilotniki bo'lsa yoki ruxsat katalogda
// bo'lmasa — hisobga olinmaydi.
func effective(userID string, g *Grant) (*Access, error) {
	if g == nil {
		return nil, ErrNotFound
	}
	switch g.Business.Status {
	case BusinessActive:
	case BusinessClosed:
		return nil, ErrNotFound
	default:
		return nil, ErrSuspended
	}
	if g.OrgStatus != OrgPending && g.OrgStatus != OrgActive {
		return nil, ErrSuspended
	}
	t := g.Business.Type
	acc := &Access{UserID: userID, Business: g.Business, MembershipID: g.MembershipID,
		IsOwner: g.IsOwner, perms: map[Permission]bool{}}
	if g.IsOwner {
		for _, p := range Catalog(t) {
			acc.perms[p.Key] = true
		}
		acc.RoleKey = "owner"
		return acc, nil
	}
	r := g.Role
	if r == nil || r.BusinessType != t || (r.OrgID != "" && r.OrgID != g.Business.OrgID) {
		return nil, ErrNotFound
	}
	acc.RoleKey = r.Key
	for _, p := range r.Permissions {
		if Known(t, p) {
			acc.perms[p] = true
		}
	}
	for _, app := range g.Apps {
		if ValidApp(app) && !slices.Contains(acc.apps, app) {
			acc.apps = append(acc.apps, app)
		}
	}
	return acc, nil
}

// ValidatePermissions — hammasi shu tur katalogida, takrorsiz.
func ValidatePermissions(t BusinessType, perms []Permission) error {
	if !t.Valid() {
		return fmt.Errorf("%w: biznes turi %q", ErrInvalid, t)
	}
	seen := map[Permission]bool{}
	for _, p := range perms {
		if !Known(t, p) {
			return fmt.Errorf("%w: noma'lum ruxsat %q", ErrInvalid, p)
		}
		if seen[p] {
			return fmt.Errorf("%w: takroriy ruxsat %q", ErrInvalid, p)
		}
		seen[p] = true
	}
	return nil
}

// CanGrant — `granter` shu ruxsatlarni bera oladimi (huquqni oshirishga
// qarshi): katalogda bo'lishi, berayotganda O'ZIDA bo'lishi va xavfli
// ruxsatni faqat egasi berishi SHART. Xodimlarni boshqarishning o'zi
// `staff.manage` talab qiladi.
func CanGrant(granter *Access, perms []Permission) error {
	if granter == nil {
		return ErrForbidden
	}
	if err := ValidatePermissions(granter.Business.Type, perms); err != nil {
		return err
	}
	if !granter.Has(PermStaffManage) {
		return ErrForbidden
	}
	if granter.IsOwner {
		return nil
	}
	for _, p := range perms {
		if IsDangerous(granter.Business.Type, p) || !granter.Has(p) {
			return fmt.Errorf("%w: %q ni bera olmaysiz", ErrForbidden, p)
		}
	}
	return nil
}

// TemplateRoles — katalog shablonlari `roles` qatorlari ko'rinishida.
func TemplateRoles() []*Role {
	ts := Templates()
	out := make([]*Role, len(ts))
	for i, t := range ts {
		out[i] = &Role{ID: t.ID(), BusinessType: t.Type, Key: t.Key, Name: t.Name, Permissions: t.Permissions}
	}
	return out
}

// EnsureTemplates — ishga tushishda shablonlarni bazaga yozadi (kod = manba).
func EnsureTemplates(ctx context.Context, s Store) error {
	return s.UpsertTemplates(ctx, TemplateRoles())
}
