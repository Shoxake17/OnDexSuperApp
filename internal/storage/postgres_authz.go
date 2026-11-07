package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/identity/authz"
)

// PgAuthzStore — merchant katalogi (0064 tashkilot/biznes, 0065 rol/a'zolik).
// `authz.Store`; xotira ombori bilan bir xil shartnoma (authztest).
type PgAuthzStore struct{ pool *pgxpool.Pool }

func NewPgAuthzStore(pool *pgxpool.Pool) *PgAuthzStore { return &PgAuthzStore{pool: pool} }

// authzErr — baza xatolarini katalog xatolariga: unikal = Conflict,
// CHECK/FK/format = Invalid.
func authzErr(err error) error {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		switch pe.Code {
		case "23505":
			return authz.ErrConflict
		case "23514", "23503", "23502", "22P02", "22001":
			return authz.ErrInvalid
		}
	}
	return err
}

func perms2s(p []authz.Permission) []string {
	out := make([]string, len(p))
	for i, x := range p {
		out[i] = string(x)
	}
	return out
}

func s2perms(s []string) []authz.Permission {
	out := make([]authz.Permission, len(s))
	for i, x := range s {
		out[i] = authz.Permission(x)
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (r *PgAuthzStore) CreateOrganization(ctx context.Context, o *authz.Organization) error {
	if o.Status == "" {
		o.Status = authz.OrgPending
	}
	err := r.pool.QueryRow(ctx, `INSERT INTO organizations (id, name, legal_form, tin, status, owner_user_id)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at`,
		o.ID, o.Name, o.LegalForm, o.TIN, o.Status, o.OwnerUserID).Scan(&o.CreatedAt)
	return authzErr(err)
}

func (r *PgAuthzStore) GetOrganization(ctx context.Context, id string) (*authz.Organization, error) {
	var o authz.Organization
	err := r.pool.QueryRow(ctx, `SELECT id, name, legal_form, tin, status, owner_user_id, created_at
		FROM organizations WHERE id = $1`, id).Scan(&o.ID, &o.Name, &o.LegalForm, &o.TIN, &o.Status, &o.OwnerUserID, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, authz.ErrNotFound
	}
	return &o, err
}

func (r *PgAuthzStore) CreateBusiness(ctx context.Context, b *authz.Business) error {
	if b.Status == "" {
		b.Status = authz.BusinessActive
	}
	err := r.pool.QueryRow(ctx, `INSERT INTO businesses (id, org_id, type, legacy_entity_id, name, status)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at`,
		b.ID, b.OrgID, string(b.Type), b.LegacyEntityID, b.Name, b.Status).Scan(&b.CreatedAt)
	return authzErr(err)
}

const businessCols = `b.id, b.org_id, b.type, b.legacy_entity_id, b.name, b.status, b.created_at`

func scanBusiness(row pgx.Row, extra ...any) (*authz.Business, error) {
	var b authz.Business
	var typ string
	dst := append([]any{&b.ID, &b.OrgID, &typ, &b.LegacyEntityID, &b.Name, &b.Status, &b.CreatedAt}, extra...)
	if err := row.Scan(dst...); err != nil {
		return nil, err
	}
	b.Type = authz.BusinessType(typ)
	return &b, nil
}

func (r *PgAuthzStore) GetBusiness(ctx context.Context, id string) (*authz.Business, error) {
	b, err := scanBusiness(r.pool.QueryRow(ctx, `SELECT `+businessCols+` FROM businesses b WHERE b.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, authz.ErrNotFound
	}
	return b, err
}

// UpsertTemplates — bitta tranzaksiyada; shablon ID barqaror (`tpl_<tur>_<kalit>`).
func (r *PgAuthzStore) UpsertTemplates(ctx context.Context, roles []*authz.Role) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit muvaffaqiyatli bo'lsa no-op
	for _, x := range roles {
		if x.OrgID != "" {
			return authz.ErrInvalid
		}
		if _, err := tx.Exec(ctx, `INSERT INTO roles (id, org_id, business_type, key, name, permissions)
			VALUES ($1, NULL, $2, $3, $4, $5)
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, permissions = EXCLUDED.permissions, updated_at = now()
			WHERE roles.org_id IS NULL`,
			x.ID, string(x.BusinessType), x.Key, x.Name, perms2s(x.Permissions)); err != nil {
			return authzErr(err)
		}
	}
	return tx.Commit(ctx)
}

func (r *PgAuthzStore) CreateRole(ctx context.Context, x *authz.Role) error {
	if x.OrgID == "" {
		return authz.ErrInvalid
	}
	var createdBy any
	if x.CreatedBy != "" {
		createdBy = x.CreatedBy
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO roles (id, org_id, business_type, key, name, permissions, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		x.ID, x.OrgID, string(x.BusinessType), x.Key, x.Name, perms2s(x.Permissions), createdBy)
	return authzErr(err)
}

const roleCols = `id, COALESCE(org_id, ''), business_type, key, name, permissions, COALESCE(created_by, '')`

func scanRole(row pgx.Row) (*authz.Role, error) {
	var x authz.Role
	var typ string
	var perms []string
	if err := row.Scan(&x.ID, &x.OrgID, &typ, &x.Key, &x.Name, &perms, &x.CreatedBy); err != nil {
		return nil, err
	}
	x.BusinessType, x.Permissions = authz.BusinessType(typ), s2perms(perms)
	return &x, nil
}

func (r *PgAuthzStore) GetRole(ctx context.Context, id string) (*authz.Role, error) {
	x, err := scanRole(r.pool.QueryRow(ctx, `SELECT `+roleCols+` FROM roles WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, authz.ErrNotFound
	}
	return x, err
}

func (r *PgAuthzStore) Roles(ctx context.Context, orgID string, t authz.BusinessType) ([]*authz.Role, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+roleCols+` FROM roles
		WHERE business_type = $1 AND (org_id IS NULL OR ($2 <> '' AND org_id = $2))
		ORDER BY org_id IS NOT NULL, key`, string(t), orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*authz.Role
	for rows.Next() {
		x, err := scanRole(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// CreateMembership — xodim a'zoligi faqat biznes SHU tashkilotniki va rol
// biznes turiga mos (shablon yoki shu tashkilot roli) bo'lsa yoziladi. Bu
// mosliklarni CHECK tekshira olmaydi — shart INSERT ... SELECT ichida.
func (r *PgAuthzStore) CreateMembership(ctx context.Context, m *authz.Membership) error {
	if m.Status == "" {
		m.Status = authz.MemberInvited
	}
	var biz, role, createdBy any
	if m.BusinessID != "" {
		biz = m.BusinessID
	}
	if m.RoleID != "" {
		role = m.RoleID
	}
	if m.CreatedBy != "" {
		createdBy = m.CreatedBy
	}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO memberships (id, user_id, org_id, business_id, role_id, is_owner, apps, status, staff_member_id, created_by)
		SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		WHERE $6::boolean
		   OR EXISTS (SELECT 1 FROM businesses b JOIN roles ro ON ro.id = $5
		              WHERE b.id = $4 AND b.org_id = $3 AND ro.business_type = b.type
		                AND (ro.org_id IS NULL OR ro.org_id = $3))
		RETURNING created_at`,
		m.ID, m.UserID, m.OrgID, biz, role, m.IsOwner, nonNil(m.Apps), m.Status, m.StaffMemberID, createdBy,
	).Scan(&m.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return authz.ErrInvalid
	}
	return authzErr(err)
}

const memberCols = `id, user_id, org_id, COALESCE(business_id, ''), COALESCE(role_id, ''), is_owner, apps, status,
	staff_member_id, COALESCE(created_by, ''), created_at`

func scanMember(row pgx.Row) (*authz.Membership, error) {
	var m authz.Membership
	err := row.Scan(&m.ID, &m.UserID, &m.OrgID, &m.BusinessID, &m.RoleID, &m.IsOwner, &m.Apps, &m.Status,
		&m.StaffMemberID, &m.CreatedBy, &m.CreatedAt)
	return &m, err
}

func (r *PgAuthzStore) GetMembership(ctx context.Context, id string) (*authz.Membership, error) {
	m, err := scanMember(r.pool.QueryRow(ctx, `SELECT `+memberCols+` FROM memberships WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, authz.ErrNotFound
	}
	return m, err
}

func (r *PgAuthzStore) Members(ctx context.Context, businessID string) ([]*authz.Membership, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+memberCols+` FROM memberships
		WHERE business_id = $1 AND NOT is_owner ORDER BY created_at, id LIMIT 1000`, businessID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*authz.Membership
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateMembership — yangi rol ham biznes turiga/tashkilotga mos bo'lishi SHART.
func (r *PgAuthzStore) UpdateMembership(ctx context.Context, id string, p authz.MembershipPatch) error {
	var apps any
	if p.Apps != nil {
		apps = p.Apps
	}
	tag, err := r.pool.Exec(ctx, `
		UPDATE memberships m SET
			role_id = CASE WHEN $2 = '' THEN m.role_id ELSE $2 END,
			apps    = COALESCE($3::text[], m.apps),
			status  = CASE WHEN $4 = '' THEN m.status ELSE $4 END,
			updated_at = now()
		WHERE m.id = $1
		  AND ($2 = '' OR (NOT m.is_owner AND EXISTS (
		        SELECT 1 FROM businesses b JOIN roles ro ON ro.id = $2
		        WHERE b.id = m.business_id AND ro.business_type = b.type
		          AND (ro.org_id IS NULL OR ro.org_id = m.org_id))))`,
		id, p.RoleID, apps, p.Status)
	if err != nil {
		return authzErr(err)
	}
	if tag.RowsAffected() == 0 {
		if _, err := r.GetMembership(ctx, id); err != nil {
			return err
		}
		return authz.ErrInvalid
	}
	return nil
}

// grantSelect — biznes + tashkilot holati + FAOL a'zolik (egalik yoki shu
// biznesdagi xodimlik) + rol. Egalik birinchi.
const grantSelect = `SELECT ` + businessCols + `, o.status, m.id, m.is_owner, m.apps,
		ro.id, COALESCE(ro.org_id, ''), ro.business_type, ro.key, ro.name, ro.permissions
	FROM businesses b
	JOIN organizations o ON o.id = b.org_id
	JOIN memberships m ON m.user_id = $1 AND m.status = 'active'
	     AND ((m.is_owner AND m.org_id = b.org_id) OR (NOT m.is_owner AND m.business_id = b.id))
	LEFT JOIN roles ro ON ro.id = m.role_id`

func scanAuthzGrant(row pgx.Row) (*authz.Grant, error) {
	var g authz.Grant
	var roleID, roleOrg, roleType, roleKey, roleName *string
	var perms []string
	b, err := scanBusiness(row, &g.OrgStatus, &g.MembershipID, &g.IsOwner, &g.Apps,
		&roleID, &roleOrg, &roleType, &roleKey, &roleName, &perms)
	if err != nil {
		return nil, err
	}
	g.Business = *b
	if roleID != nil && !g.IsOwner {
		g.Role = &authz.Role{ID: *roleID, OrgID: *roleOrg, BusinessType: authz.BusinessType(*roleType),
			Key: *roleKey, Name: *roleName, Permissions: s2perms(perms)}
	}
	return &g, nil
}

func (r *PgAuthzStore) Grant(ctx context.Context, userID, businessID string) (*authz.Grant, error) {
	g, err := scanAuthzGrant(r.pool.QueryRow(ctx, grantSelect+` WHERE b.id = $2 ORDER BY m.is_owner DESC LIMIT 1`,
		userID, businessID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, authz.ErrNotFound
	}
	return g, err
}

func (r *PgAuthzStore) Grants(ctx context.Context, userID string) ([]*authz.Grant, error) {
	// Egasi o'z biznesida xodim sifatida ham yozilgan bo'lishi mumkin: har
	// biznes uchun birinchi qator (egalik ustun) olinadi.
	rows, err := r.pool.Query(ctx, grantSelect+` ORDER BY b.created_at, b.id, m.is_owner DESC LIMIT 2000`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*authz.Grant
	seen := map[string]bool{}
	for rows.Next() {
		g, err := scanAuthzGrant(rows)
		if err != nil {
			return nil, err
		}
		if seen[g.Business.ID] {
			continue
		}
		seen[g.Business.ID] = true
		out = append(out, g)
	}
	return out, rows.Err()
}
