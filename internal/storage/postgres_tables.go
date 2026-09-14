package storage

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/tables"
)

// Unikal indeks nomlari. Create/Update ularni pgconn.PgError.ConstraintName
// orqali aniqlab, domen xatosiga aylantiradi — chaqiruvchi "23505" degan
// xom Postgres kodini bilishi shart emas.
const (
	idxTablesToken         = "idx_restaurant_tables_token"
	idxTablesLabel         = "idx_restaurant_tables_label"      // 0032, 0042 da tushgan
	idxTablesZoneLabel     = "idx_restaurant_tables_zone_label" // 0042, 0044 da tushgan
	idxTablesZoneKindLabel = "idx_restaurant_tables_zone_kind_label"
)

type PgTableRepo struct{ pool *pgxpool.Pool }

func NewPgTableRepo(pool *pgxpool.Pool) *PgTableRepo { return &PgTableRepo{pool: pool} }

const tableColumns = `id, restaurant_id, zone, kind, label, capacity, qr_token, active,
	cleaning_since, last_scanned_at, created_at`

func tableArgs(t *tables.Table) []any {
	zone := t.Zone
	if strings.TrimSpace(zone) == "" {
		zone = tables.DefaultZone
	}
	return []any{t.ID, t.RestaurantID, zone, string(t.Kind.Normalized()), t.Label, t.Capacity,
		t.QRToken, t.Active, t.CleaningSince, t.LastScannedAt, t.CreatedAt}
}

const insertTableSQL = `INSERT INTO restaurant_tables (` + tableColumns + `)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

func (r *PgTableRepo) Create(ctx context.Context, t *tables.Table) error {
	_, err := r.pool.Exec(ctx, insertTableSQL, tableArgs(t)...)
	return mapTableErr(err)
}

// CreateMany — bitta tranzaksiyada: biror qator yiqilsa hech biri qolmaydi.
func (r *PgTableRepo) CreateMany(ctx context.Context, list []*tables.Table) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit'dan keyin no-op
	for _, t := range list {
		if _, err := tx.Exec(ctx, insertTableSQL, tableArgs(t)...); err != nil {
			return mapTableErr(err)
		}
	}
	return tx.Commit(ctx)
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
		case idxTablesLabel, idxTablesZoneLabel, idxTablesZoneKindLabel:
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

func scanTableInto(row pgx.Row, t *tables.Table) error {
	var kind string
	if err := row.Scan(&t.ID, &t.RestaurantID, &t.Zone, &kind, &t.Label, &t.Capacity, &t.QRToken,
		&t.Active, &t.CleaningSince, &t.LastScannedAt, &t.CreatedAt); err != nil {
		return err
	}
	t.Kind = tables.Kind(kind)
	return nil
}

func scanTable(row pgx.Row) (*tables.Table, error) {
	var t tables.Table
	err := scanTableInto(row, &t)
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
		 WHERE restaurant_id = $1 ORDER BY zone, kind, label`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*tables.Table
	for rows.Next() {
		var t tables.Table
		if err := scanTableInto(rows, &t); err != nil {
			return nil, err
		}
		list = append(list, &t)
	}
	return list, rows.Err()
}

// Update — zona, tur, nom, sig'im, faollik va tozalash holatini yozadi.
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
// `last_scanned_at` ham bu yerda yozilmaydi: tahrir paytidagi eski
// nusxa yangi skanerlash vaqtini bosib qolmasin.
// └───────────────────────────────────────────────────────────────────┘
func (r *PgTableRepo) Update(ctx context.Context, t *tables.Table) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE restaurant_tables
		SET zone = $2, kind = $3, label = $4, capacity = $5, active = $6, cleaning_since = $7
		WHERE id = $1`,
		t.ID, t.Zone, string(t.Kind.Normalized()), t.Label, t.Capacity, t.Active, t.CleaningSince)
	if err != nil {
		return mapTableErr(err)
	}
	if tag.RowsAffected() == 0 {
		return tables.ErrNotFound
	}
	return nil
}

// TouchScanned — shart bazada: parallel skanerlashlar orasida "o'qib,
// keyin yozish" poygasi yo'q.
func (r *PgTableRepo) TouchScanned(ctx context.Context, id string, at time.Time, minInterval time.Duration) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE restaurant_tables SET last_scanned_at = $2
		WHERE id = $1 AND (last_scanned_at IS NULL OR last_scanned_at <= $3)`,
		id, at, at.Add(-minInterval))
	return err
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
