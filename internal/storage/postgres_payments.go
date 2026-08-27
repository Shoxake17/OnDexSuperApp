package storage

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/payments"
)

// PostgresPaymentRepo — to'lov urinishlari (migratsiya 0038).
type PostgresPaymentRepo struct{ pool *pgxpool.Pool }

func NewPostgresPaymentRepo(pool *pgxpool.Pool) *PostgresPaymentRepo {
	return &PostgresPaymentRepo{pool: pool}
}

const paymentColumns = `id, order_id, customer_id, restaurant_id, provider,
	provider_payment_id, amount_tiyin, status, pay_url, needs_review, review_reason,
	created_at, updated_at, paid_at, expires_at, raw`

func scanPayment(row interface{ Scan(...any) error }) (*payments.Payment, error) {
	var p payments.Payment
	var paidAt *time.Time
	var raw []byte
	err := row.Scan(&p.ID, &p.OrderID, &p.CustomerID, &p.RestaurantID, &p.Provider,
		&p.ProviderPaymentID, &p.AmountTiyin, &p.Status, &p.PayURL, &p.NeedsReview,
		&p.ReviewReason, &p.CreatedAt, &p.UpdatedAt, &paidAt, &p.ExpiresAt, &raw)
	if err != nil {
		return nil, err
	}
	p.PaidAt = paidAt
	p.Raw = raw
	return &p, nil
}

func (r *PostgresPaymentRepo) Create(ctx context.Context, p *payments.Payment) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO payments (`+paymentColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		p.ID, p.OrderID, p.CustomerID, p.RestaurantID, p.Provider,
		p.ProviderPaymentID, p.AmountTiyin, p.Status, p.PayURL, p.NeedsReview,
		p.ReviewReason, p.CreatedAt, p.UpdatedAt, p.PaidAt, p.ExpiresAt, rawOrNil(p.Raw))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		// Bir xil provayder to'lovi ikkinchi yozuvga bog'lanmaydi
		// (unique indeks) — bu ma'lumot buzilishining oldini oladi.
		return errors.New("bu to'lov allaqachon qayd etilgan")
	}
	return err
}

func (r *PostgresPaymentRepo) Update(ctx context.Context, p *payments.Payment) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE payments SET
			provider_payment_id = $2, amount_tiyin = $3, status = $4, pay_url = $5,
			needs_review = $6, review_reason = $7, updated_at = $8, paid_at = $9,
			expires_at = $10, raw = COALESCE($11, raw)
		WHERE id = $1`,
		p.ID, p.ProviderPaymentID, p.AmountTiyin, p.Status, p.PayURL,
		p.NeedsReview, p.ReviewReason, p.UpdatedAt, p.PaidAt, p.ExpiresAt, rawOrNil(p.Raw))
	return err
}

func (r *PostgresPaymentRepo) GetByID(ctx context.Context, id string) (*payments.Payment, error) {
	p, err := scanPayment(r.pool.QueryRow(ctx,
		`SELECT `+paymentColumns+` FROM payments WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, payments.ErrNotFound
	}
	return p, err
}

func (r *PostgresPaymentRepo) GetByProviderID(ctx context.Context, provider, providerPaymentID string) (*payments.Payment, error) {
	if providerPaymentID == "" {
		return nil, payments.ErrNotFound
	}
	p, err := scanPayment(r.pool.QueryRow(ctx,
		`SELECT `+paymentColumns+` FROM payments
		 WHERE provider = $1 AND provider_payment_id = $2`, provider, providerPaymentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, payments.ErrNotFound
	}
	return p, err
}

func (r *PostgresPaymentRepo) ListByOrder(ctx context.Context, orderID string) ([]*payments.Payment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+paymentColumns+` FROM payments WHERE order_id = $1 ORDER BY created_at DESC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*payments.Payment
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PostgresPaymentRepo) ListExpired(ctx context.Context, now time.Time, limit int) ([]*payments.Payment, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+paymentColumns+` FROM payments
		 WHERE status = $1 AND expires_at < $2
		 ORDER BY expires_at LIMIT $3`, payments.StatusPending, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*payments.Payment
	for rows.Next() {
		p, err := scanPayment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// rawOrNil — bo'sh baytlarni JSONB ustuniga `NULL` sifatida yozadi:
// bo'sh massiv yaroqsiz JSON bo'lib, INSERT'ni yiqitardi.
func rawOrNil(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
