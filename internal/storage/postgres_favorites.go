package storage

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PgFavoritesRepo struct{ pool *pgxpool.Pool }

func NewPgFavoritesRepo(pool *pgxpool.Pool) *PgFavoritesRepo { return &PgFavoritesRepo{pool: pool} }

func (r *PgFavoritesRepo) Add(ctx context.Context, customerID, productID string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO favorites (customer_id, product_id) VALUES ($1, $2)
		 ON CONFLICT (customer_id, product_id) DO NOTHING`,
		customerID, productID)
	return err
}

func (r *PgFavoritesRepo) Remove(ctx context.Context, customerID, productID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM favorites WHERE customer_id = $1 AND product_id = $2`,
		customerID, productID)
	return err
}

func (r *PgFavoritesRepo) ListProductIDs(ctx context.Context, customerID string) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT product_id FROM favorites WHERE customer_id = $1 ORDER BY created_at DESC`,
		customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
