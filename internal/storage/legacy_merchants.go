package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/identity/legacymigrate"
	"chustapp/internal/users"
)

// LegacySchemaReady — A1b uchun kerakli sxema (0063–0070) bazada bormi.
// Yo'q bo'lsa: avval yangi API versiyasini deploy qilish kerak.
func LegacySchemaReady(ctx context.Context, pool *pgxpool.Pool) error {
	var n int
	err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE (table_name = 'users' AND column_name = 'realm')
		   OR (table_name = 'memberships' AND column_name = 'invited_at')`).Scan(&n)
	if err != nil {
		return err
	}
	if n < 2 {
		return fmt.Errorf("bazada A3 sxemasi yo'q (migratsiya 0063–0070) — avval yangi API'ni deploy qiling")
	}
	return nil
}

// LoadLegacyMerchants — A1b uchun eski holat (faqat O'QISH). Restoran nomlari
// katalogda (Mongo) — chaqiruvchi `RestaurantNames` ni o'zi to'ldiradi.
func LoadLegacyMerchants(ctx context.Context, pool *pgxpool.Pool) (*legacymigrate.Input, error) {
	in := &legacymigrate.Input{
		RestaurantNames: map[string]string{}, CourierPools: map[string]string{},
		StaffByUser: map[string]legacymigrate.StaffLink{}, CustomerOrders: map[string]int{},
		ExistingBusinesses: map[string]string{}, ExistingBusinessOrg: map[string]string{},
		ExistingMemberships: map[string]bool{},
	}
	rows, err := pool.Query(ctx, `SELECT `+userColumns+` FROM users
		WHERE role IN ('restaurant', 'store', 'waiter', 'courier') AND deleted_at IS NULL
		ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var u users.User
		if err := scanUser(rows, &u); err != nil {
			rows.Close()
			return nil, err
		}
		in.Users = append(in.Users, &u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	type q struct {
		sql  string
		scan func(pgxRows) error
	}
	for _, x := range []q{
		{`SELECT id, restaurant_id FROM couriers WHERE deleted_at IS NULL`, func(r pgxRows) error {
			var id, p string
			if err := r.Scan(&id, &p); err != nil {
				return err
			}
			in.CourierPools[id] = p
			return nil
		}},
		{`SELECT user_id, id, restaurant_id, status FROM staff_members WHERE user_id IS NOT NULL`, func(r pgxRows) error {
			var uid string
			var l legacymigrate.StaffLink
			if err := r.Scan(&uid, &l.MemberID, &l.RestaurantID, &l.Status); err != nil {
				return err
			}
			// bitta akkauntda bir nechta yozuv bo'lsa — faoli ustun
			if prev, ok := in.StaffByUser[uid]; !ok || prev.Status != "active" {
				in.StaffByUser[uid] = l
			}
			return nil
		}},
		{`SELECT o.customer_id, count(*) FROM orders o JOIN users u ON u.id = o.customer_id
			WHERE u.role IN ('waiter', 'courier') GROUP BY o.customer_id`, func(r pgxRows) error {
			var uid string
			var n int
			if err := r.Scan(&uid, &n); err != nil {
				return err
			}
			in.CustomerOrders[uid] = n
			return nil
		}},
		{`SELECT id, org_id, type, legacy_entity_id FROM businesses WHERE legacy_entity_id <> ''`, func(r pgxRows) error {
			var id, org, typ, legacy string
			if err := r.Scan(&id, &org, &typ, &legacy); err != nil {
				return err
			}
			in.ExistingBusinesses[typ+"|"+legacy] = id
			in.ExistingBusinessOrg[id] = org
			return nil
		}},
		{`SELECT user_id, COALESCE(business_id, '') FROM memberships`, func(r pgxRows) error {
			var uid, biz string
			if err := r.Scan(&uid, &biz); err != nil {
				return err
			}
			in.ExistingMemberships[uid+"|"+biz] = true
			return nil
		}},
	} {
		if err := queryEach(ctx, pool, x.sql, x.scan); err != nil {
			return nil, err
		}
	}
	return in, nil
}

type pgxRows interface{ Scan(dest ...any) error }

func queryEach(ctx context.Context, pool *pgxpool.Pool, sql string, scan func(pgxRows) error) error {
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// LegacyApplyResult — haqiqatda qo'shilgan qatorlar (ON CONFLICT bilan
// o'tkazib yuborilganlar sanalmaydi).
type LegacyApplyResult struct{ Orgs, Businesses, Memberships int }

// ApplyLegacyPlan — rejani BITTA tranzaksiyada qo'llaydi: biror qadam
// yiqilsa hech narsa yozilmaydi. Takroriy ishga tushirish xavfsiz (unikal
// kalitlar bo'yicha ON CONFLICT DO NOTHING). Tashkilotlar `active`:
// akkauntlarni superadmin yaratgan. SHART: rol shablonlari bazada bo'lishi
// (`authz.EnsureTemplates`) — a'zoliklar ularga tashqi kalit bilan bog'langan.
func ApplyLegacyPlan(ctx context.Context, pool *pgxpool.Pool, p *legacymigrate.Plan, now time.Time) (*LegacyApplyResult, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit muvaffaqiyatli bo'lsa no-op
	res := &LegacyApplyResult{}
	for _, o := range p.Orgs {
		tag, err := tx.Exec(ctx, `INSERT INTO organizations (id, name, status, owner_user_id, verified_at, status_reason, created_at)
			VALUES ($1, $2, 'active', $3, $4, 'A1b: superadmin yaratgan akkaunt', $4)
			ON CONFLICT (id) DO NOTHING`, o.ID, o.Name, o.OwnerUserID, now)
		if err != nil {
			return nil, fmt.Errorf("tashkilot %s: %w", o.ID, err)
		}
		res.Orgs += int(tag.RowsAffected())
	}
	for _, b := range p.Businesses {
		tag, err := tx.Exec(ctx, `INSERT INTO businesses (id, org_id, type, legacy_entity_id, name, status, created_at)
			VALUES ($1, $2, $3, $4, $5, 'active', $6)
			ON CONFLICT DO NOTHING`, b.ID, b.OrgID, string(b.Type), b.LegacyEntityID, b.Name, now)
		if err != nil {
			return nil, fmt.Errorf("biznes %s: %w", b.ID, err)
		}
		res.Businesses += int(tag.RowsAffected())
	}
	for _, m := range p.Memberships {
		var biz, role any
		if !m.IsOwner {
			biz, role = m.BusinessID, m.RoleID
		}
		tag, err := tx.Exec(ctx, `INSERT INTO memberships (id, user_id, org_id, business_id, role_id, is_owner, apps,
				status, staff_member_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT DO NOTHING`, m.ID, m.UserID, m.OrgID, biz, role, m.IsOwner, nonNil(m.Apps), m.Status,
			m.StaffMemberID, now)
		if err != nil {
			return nil, fmt.Errorf("a'zolik %s: %w", m.ID, err)
		}
		res.Memberships += int(tag.RowsAffected())
	}
	return res, tx.Commit(ctx)
}
