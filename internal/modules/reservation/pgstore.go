package reservation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore — Postgres ombori (`reservation` sxemasi, migratsiya 0076).
type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

// mapErr — EXCLUDE buzilishi (23P01) -> ErrSlotTaken.
func mapErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23P01" {
		return ErrSlotTaken
	}
	return err
}

func (p *PGStore) GetSettings(ctx context.Context, restaurantID string) (*Settings, error) {
	var s Settings
	var zones []byte
	err := p.pool.QueryRow(ctx, `
		SELECT restaurant_id, enabled, deposit_per_person_tiyin, zone_deposits, slot_minutes,
		       min_advance_minutes, max_days_ahead, updated_at
		FROM reservation.settings WHERE restaurant_id = $1`, restaurantID).
		Scan(&s.RestaurantID, &s.Enabled, &s.DepositPerPersonTiyin, &zones, &s.SlotMinutes,
			&s.MinAdvanceMinutes, &s.MaxDaysAhead, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.ZoneDeposits = map[string]int64{}
	if len(zones) > 0 {
		if err := json.Unmarshal(zones, &s.ZoneDeposits); err != nil {
			return nil, err
		}
	}
	return &s, nil
}

func (p *PGStore) PutSettings(ctx context.Context, s *Settings) error {
	zones, err := json.Marshal(s.ZoneDeposits)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `
		INSERT INTO reservation.settings (restaurant_id, enabled, deposit_per_person_tiyin, zone_deposits,
			slot_minutes, min_advance_minutes, max_days_ahead, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (restaurant_id) DO UPDATE SET enabled = EXCLUDED.enabled,
			deposit_per_person_tiyin = EXCLUDED.deposit_per_person_tiyin, zone_deposits = EXCLUDED.zone_deposits,
			slot_minutes = EXCLUDED.slot_minutes, min_advance_minutes = EXCLUDED.min_advance_minutes,
			max_days_ahead = EXCLUDED.max_days_ahead, updated_at = EXCLUDED.updated_at`,
		s.RestaurantID, s.Enabled, s.DepositPerPersonTiyin, zones, s.SlotMinutes,
		s.MinAdvanceMinutes, s.MaxDaysAhead, s.UpdatedAt)
	return err
}

const rsvColumns = `id, restaurant_id, table_id, table_label, customer_id, party_size, starts_at, ends_at,
	status, deposit_tiyin, deposit_method, deposit_state, comment, hold_expires_at, cancel_reason,
	cancelled_by, reminded_at, created_at, updated_at`

func scanRsv(row pgx.Row) (*Reservation, error) {
	var r Reservation
	err := row.Scan(&r.ID, &r.RestaurantID, &r.TableID, &r.TableLabel, &r.CustomerID, &r.PartySize,
		&r.StartsAt, &r.EndsAt, &r.Status, &r.DepositTiyin, &r.DepositMethod, &r.DepositState,
		&r.Comment, &r.HoldExpiresAt, &r.CancelReason, &r.CancelledBy, &r.RemindedAt,
		&r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &r, err
}

func (p *PGStore) Insert(ctx context.Context, r *Reservation) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO reservation.reservations (`+rsvColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		r.ID, r.RestaurantID, r.TableID, r.TableLabel, r.CustomerID, r.PartySize, r.StartsAt, r.EndsAt,
		r.Status, r.DepositTiyin, r.DepositMethod, r.DepositState, r.Comment, r.HoldExpiresAt,
		r.CancelReason, r.CancelledBy, r.RemindedAt, r.CreatedAt, r.UpdatedAt)
	return mapErr(err)
}

func (p *PGStore) Get(ctx context.Context, id string) (*Reservation, error) {
	return scanRsv(p.pool.QueryRow(ctx, `SELECT `+rsvColumns+` FROM reservation.reservations WHERE id = $1`, id))
}

// Update — shartli: bazadagi holat `from` bo'lmasa ErrConflict (ikki
// xodim/nazoratchi bir vaqtda o'zgartirsa, ikkinchisi yutqazadi).
func (p *PGStore) Update(ctx context.Context, r *Reservation, from Status) error {
	tag, err := p.pool.Exec(ctx, `
		UPDATE reservation.reservations SET status = $3, deposit_state = $4, hold_expires_at = $5,
			cancel_reason = $6, cancelled_by = $7, reminded_at = $8, updated_at = $9
		WHERE id = $1 AND status = $2`,
		r.ID, from, r.Status, r.DepositState, r.HoldExpiresAt, r.CancelReason, r.CancelledBy,
		r.RemindedAt, r.UpdatedAt)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrConflict
	}
	return nil
}

func (p *PGStore) list(ctx context.Context, q string, args ...any) ([]*Reservation, error) {
	rows, err := p.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Reservation{}
	for rows.Next() {
		r, err := scanRsv(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *PGStore) ListByRestaurant(ctx context.Context, restaurantID string, from, to time.Time) ([]*Reservation, error) {
	return p.list(ctx, `SELECT `+rsvColumns+` FROM reservation.reservations
		WHERE restaurant_id = $1 AND starts_at < $3 AND ends_at > $2 ORDER BY starts_at LIMIT 2000`,
		restaurantID, from, to)
}

func (p *PGStore) ListByCustomer(ctx context.Context, customerID string, limit int) ([]*Reservation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return p.list(ctx, `SELECT `+rsvColumns+` FROM reservation.reservations
		WHERE customer_id = $1 ORDER BY starts_at DESC LIMIT $2`, customerID, limit)
}

// ListDue — `isDue` (memstore.go) bilan bir xil shart.
func (p *PGStore) ListDue(ctx context.Context, now time.Time, limit int) ([]*Reservation, error) {
	if limit <= 0 {
		limit = 200
	}
	return p.list(ctx, `SELECT `+rsvColumns+` FROM reservation.reservations
		WHERE (status = 'held' AND hold_expires_at < $1)
		   OR (status = 'confirmed' AND (starts_at + make_interval(secs => $2) < $1
		       OR (reminded_at IS NULL AND starts_at - make_interval(secs => $3) <= $1)))
		   OR (status = 'seated' AND ends_at + interval '2 hours' < $1)
		ORDER BY starts_at LIMIT $4`,
		now, NoShowAfter.Seconds(), ReminderBefore.Seconds(), limit)
}
