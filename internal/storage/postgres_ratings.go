package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/ratings"
)

type PgRatingRepo struct{ pool *pgxpool.Pool }

func NewPgRatingRepo(pool *pgxpool.Pool) *PgRatingRepo { return &PgRatingRepo{pool: pool} }

// uniqueViolation — Postgres "23505" kodi (`order_id` UNIQUE buzilishi).
func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *PgRatingRepo) Create(ctx context.Context, rt *ratings.Rating) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO restaurant_ratings (id, order_id, restaurant_id, customer_id, stars, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		rt.ID, rt.OrderID, rt.RestaurantID, rt.CustomerID, rt.Stars, rt.CreatedAt)
	if uniqueViolation(err) {
		return ratings.ErrAlreadyRated
	}
	return err
}

func (r *PgRatingRepo) GetByOrder(ctx context.Context, orderID string) (*ratings.Rating, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, order_id, restaurant_id, customer_id, stars, created_at
		 FROM restaurant_ratings WHERE order_id = $1`, orderID)
	var rt ratings.Rating
	if err := row.Scan(&rt.ID, &rt.OrderID, &rt.RestaurantID, &rt.CustomerID, &rt.Stars, &rt.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ratings.ErrNotFound
		}
		return nil, err
	}
	return &rt, nil
}

func (r *PgRatingRepo) Summary(ctx context.Context, restaurantID string) (float64, int, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT COALESCE(AVG(stars), 0), COUNT(*) FROM restaurant_ratings WHERE restaurant_id = $1`,
		restaurantID)
	var avg float64
	var count int
	if err := row.Scan(&avg, &count); err != nil {
		return 0, 0, err
	}
	return avg, count, nil
}
