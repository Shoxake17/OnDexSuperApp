package storage

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/staff"
)

const (
	idxStaffActivePhone = "idx_staff_members_active_phone"
	idxStaffUser        = "idx_staff_members_user"
)

type PgStaffRepo struct{ pool *pgxpool.Pool }

func NewPgStaffRepo(pool *pgxpool.Pool) *PgStaffRepo { return &PgStaffRepo{pool: pool} }

const staffColumns = `id, restaurant_id, number, first_name, last_name, phone, position, status,
	schedule, hired_on, note, app_access, user_id, created_at, updated_at, dismissed_at, monthly_salary_tiyin`

func mapStaffErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case idxStaffActivePhone:
			return staff.ErrPhoneTaken
		case idxStaffUser:
			return staff.ErrAccountConflict
		}
	}
	return err
}

func scheduleParam(s *staff.Schedule) (any, error) {
	if s == nil {
		return nil, nil
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func nullableText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scanStaffInto(row pgx.Row, m *staff.Member) error {
	var (
		position, status string
		schedule         []byte
		userID           *string
	)
	if err := row.Scan(&m.ID, &m.RestaurantID, &m.Number, &m.FirstName, &m.LastName, &m.Phone,
		&position, &status, &schedule, &m.HiredOn, &m.Note, &m.AppAccess, &userID,
		&m.CreatedAt, &m.UpdatedAt, &m.DismissedAt, &m.SalaryTiyin); err != nil {
		return err
	}
	m.Position = staff.Position(position)
	m.Status = staff.Status(status)
	if len(schedule) > 0 {
		var s staff.Schedule
		if err := json.Unmarshal(schedule, &s); err != nil {
			return err
		}
		m.Schedule = &s
	}
	if userID != nil {
		m.UserID = *userID
	}
	return nil
}

func insertStaffEvents(ctx context.Context, tx pgx.Tx, events []staff.Event) error {
	for _, ev := range events {
		if _, err := tx.Exec(ctx, `
			INSERT INTO staff_events (id, restaurant_id, member_id, kind, from_value, to_value, actor_id, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			ev.ID, ev.RestaurantID, ev.MemberID, string(ev.Kind), ev.From, ev.To, ev.ActorID, ev.At); err != nil {
			return err
		}
	}
	return nil
}

// Create — tartib raqami tranzaksiya ichida, restoran bo'yicha qulf
// ostida beriladi: ikki parallel "Xodim qo'shish" bir xil #EMP olmaydi.
func (r *PgStaffRepo) Create(ctx context.Context, m *staff.Member, events []staff.Event) error {
	sched, err := scheduleParam(m.Schedule)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit'dan keyin no-op

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('staff:' || $1))`, m.RestaurantID); err != nil {
		return err
	}
	var number int
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(MAX(number), 0) + 1 FROM staff_members WHERE restaurant_id = $1`,
		m.RestaurantID).Scan(&number); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO staff_members (`+staffColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11,$12,$13,$14,$15,$16,$17)`,
		m.ID, m.RestaurantID, number, m.FirstName, m.LastName, m.Phone, string(m.Position), string(m.Status),
		sched, m.HiredOn, m.Note, m.AppAccess, nullableText(m.UserID), m.CreatedAt, m.UpdatedAt, m.DismissedAt,
		m.SalaryTiyin,
	); err != nil {
		return mapStaffErr(err)
	}
	if err := insertStaffEvents(ctx, tx, events); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	m.Number = number
	return nil
}

func (r *PgStaffRepo) Update(ctx context.Context, m *staff.Member, events []staff.Event) error {
	sched, err := scheduleParam(m.Schedule)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit'dan keyin no-op

	tag, err := tx.Exec(ctx, `
		UPDATE staff_members
		SET first_name = $3, last_name = $4, phone = $5, position = $6, status = $7,
		    schedule = $8::jsonb, hired_on = $9, note = $10, app_access = $11, user_id = $12,
		    updated_at = $13, dismissed_at = $14, monthly_salary_tiyin = $15
		WHERE id = $1 AND restaurant_id = $2`,
		m.ID, m.RestaurantID, m.FirstName, m.LastName, m.Phone, string(m.Position), string(m.Status),
		sched, m.HiredOn, m.Note, m.AppAccess, nullableText(m.UserID), m.UpdatedAt, m.DismissedAt, m.SalaryTiyin)
	if err != nil {
		return mapStaffErr(err)
	}
	if tag.RowsAffected() == 0 {
		return staff.ErrNotFound
	}
	if err := insertStaffEvents(ctx, tx, events); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PgStaffRepo) Get(ctx context.Context, restaurantID, id string) (*staff.Member, error) {
	var m staff.Member
	err := scanStaffInto(r.pool.QueryRow(ctx,
		`SELECT `+staffColumns+` FROM staff_members WHERE id = $1 AND restaurant_id = $2`, id, restaurantID), &m)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, staff.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *PgStaffRepo) List(ctx context.Context, restaurantID string) ([]*staff.Member, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+staffColumns+` FROM staff_members WHERE restaurant_id = $1 ORDER BY number`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*staff.Member
	for rows.Next() {
		var m staff.Member
		if err := scanStaffInto(rows, &m); err != nil {
			return nil, err
		}
		list = append(list, &m)
	}
	return list, rows.Err()
}

func (r *PgStaffRepo) Events(ctx context.Context, restaurantID, memberID string, since time.Time, limit int) ([]staff.Event, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, restaurant_id, member_id, kind, from_value, to_value, actor_id, created_at
		FROM staff_events
		WHERE restaurant_id = $1 AND ($2 = '' OR member_id = $2) AND created_at >= $3
		ORDER BY created_at DESC, seq DESC
		LIMIT $4`, restaurantID, memberID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []staff.Event
	for rows.Next() {
		var ev staff.Event
		var kind string
		if err := rows.Scan(&ev.ID, &ev.RestaurantID, &ev.MemberID, &kind, &ev.From, &ev.To, &ev.ActorID, &ev.At); err != nil {
			return nil, err
		}
		ev.Kind = staff.EventKind(kind)
		list = append(list, ev)
	}
	return list, rows.Err()
}
