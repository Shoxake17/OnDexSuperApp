package storage

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/tables"
)

// Migration 0032'dagi unikal indeks nomlari. Save/Create ularni
// pgconn.PgError.ConstraintName orqali aniqlab, domen xatosiga
// aylantiradi — chaqiruvchi "23505" degan xom Postgres kodini
// bilishi shart emas.
const (
	idxTablesToken     = "idx_restaurant_tables_token"
	idxTablesLabel     = "idx_restaurant_tables_label" // 0032, 0042 dan keyin tushadi
	idxTablesZoneLabel = "idx_restaurant_tables_zone_label"
)

type PgTableRepo struct{ pool *pgxpool.Pool }

func NewPgTableRepo(pool *pgxpool.Pool) *PgTableRepo { return &PgTableRepo{pool: pool} }

const tableColumns = `id, restaurant_id, zone, label, qr_token, active, created_at`

func (r *PgTableRepo) Create(ctx context.Context, t *tables.Table) error {
	zone := t.Zone
	if strings.TrimSpace(zone) == "" {
		zone = tables.DefaultZone
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO restaurant_tables (`+tableColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		t.ID, t.RestaurantID, zone, t.Label, t.QRToken, t.Active, t.CreatedAt)
	return mapTableErr(err)
}

// mapTableErr — Postgres unikallik buzilishini domen xatosiga
// aylantiradi.
func mapTableErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case idxTablesLabel, idxTablesZoneLabel:
			return tables.ErrDuplicate
		case idxTablesToken:
			// Amalda imkonsiz (32 tasodifiy bayt), lekin jimgina
			// o'tkazib yuborilsa stol boshqa restoranning tokeni
			// bilan yaratilgan bo'lardi.
			return errors.New("QR token kolliziyasi — qaytadan urinib ko'ring")
		}
	}
	return err
}

func scanTable(row pgx.Row) (*tables.Table, error) {
	var t tables.Table
	err := row.Scan(&t.ID, &t.RestaurantID, &t.Zone, &t.Label, &t.QRToken, &t.Active, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, tables.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *PgTableRepo) GetByID(ctx context.Context, id string) (*tables.Table, error) {
	return scanTable(r.pool.QueryRow(ctx,
		`SELECT `+tableColumns+` FROM restaurant_tables WHERE id = $1`, id))
}

func (r *PgTableRepo) GetByToken(ctx context.Context, token string) (*tables.Table, error) {
	return scanTable(r.pool.QueryRow(ctx,
		`SELECT `+tableColumns+` FROM restaurant_tables WHERE qr_token = $1`, token))
}

func (r *PgTableRepo) ListByRestaurant(ctx context.Context, restaurantID string) ([]*tables.Table, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+tableColumns+` FROM restaurant_tables
		 WHERE restaurant_id = $1 ORDER BY zone, label`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*tables.Table
	for rows.Next() {
		var t tables.Table
		if err := rows.Scan(&t.ID, &t.RestaurantID, &t.Zone, &t.Label, &t.QRToken,
			&t.Active, &t.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &t)
	}
	return list, rows.Err()
}

// Update — nom va faollikni yangilaydi.
//
// ┌─ `qr_token` ATAYLAB RO'YXATDA YO'Q ───────────────────────────────┐
// QR kod menyu varaqasiga chop etilgan va stolda abadiy turadi
// (`internal/tables/table.go` dagi izohga qarang). Token o'zgarsa,
// butun zaldagi varaqalar bir zumda ishlamay qolardi.
//
// Xizmat qatlamida uni o'zgartiradigan metod umuman yo'q, lekin
// tekshiruv SHU YERDA ham takrorlanadi: kelajakda kimdir
// `t.QRToken` ni o'zgartirib `Update` chaqirsa, o'zgarish jimgina
// E'TIBORSIZ qoldiriladi — bazadagi qiymat tegilmaydi.
// └───────────────────────────────────────────────────────────────────┘
func (r *PgTableRepo) Update(ctx context.Context, t *tables.Table) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE restaurant_tables
		SET zone = $2, label = $3, active = $4
		WHERE id = $1`, t.ID, t.Zone, t.Label, t.Active)
	if err != nil {
		return mapTableErr(err)
	}
	if tag.RowsAffected() == 0 {
		return tables.ErrNotFound
	}
	return nil
}

func (r *PgTableRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM restaurant_tables WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tables.ErrNotFound
	}
	return nil
}
