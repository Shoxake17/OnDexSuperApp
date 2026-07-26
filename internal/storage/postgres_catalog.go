package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/catalog"
)

type PgCatalogRepo struct{ pool *pgxpool.Pool }

func NewPgCatalogRepo(pool *pgxpool.Pool) *PgCatalogRepo { return &PgCatalogRepo{pool: pool} }

func (r *PgCatalogRepo) ListRestaurants(ctx context.Context) ([]*catalog.Restaurant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, address, lat, lng, open FROM restaurants ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*catalog.Restaurant
	for rows.Next() {
		var x catalog.Restaurant
		if err := rows.Scan(&x.ID, &x.Name, &x.Address, &x.Lat, &x.Lng, &x.Open); err != nil {
			return nil, err
		}
		list = append(list, &x)
	}
	return list, rows.Err()
}

func (r *PgCatalogRepo) GetRestaurant(ctx context.Context, id string) (*catalog.Restaurant, error) {
	var x catalog.Restaurant
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, address, lat, lng, open FROM restaurants WHERE id = $1`, id,
	).Scan(&x.ID, &x.Name, &x.Address, &x.Lat, &x.Lng, &x.Open)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, catalog.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &x, nil
}

func (r *PgCatalogRepo) SaveRestaurant(ctx context.Context, x *catalog.Restaurant) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO restaurants (id, name, address, lat, lng, open)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name, address = EXCLUDED.address,
			lat = EXCLUDED.lat, lng = EXCLUDED.lng, open = EXCLUDED.open`,
		x.ID, x.Name, x.Address, x.Lat, x.Lng, x.Open)
	return err
}

// DeleteRestaurant — tranzaksiya ichida: avval taomlar, keyin restoran.
// Yarim o'chirilgan holat qolmasligi kafolatlanadi.
func (r *PgCatalogRepo) DeleteRestaurant(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM products WHERE restaurant_id = $1`, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM restaurants WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return catalog.ErrNotFound
	}
	return tx.Commit(ctx)
}

func (r *PgCatalogRepo) ListProducts(ctx context.Context, restaurantID string) ([]*catalog.Product, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, restaurant_id, name, price_tiyin, available
		 FROM products WHERE restaurant_id = $1 ORDER BY name`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (r *PgCatalogRepo) GetProductsByIDs(ctx context.Context, ids []string) ([]*catalog.Product, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, restaurant_id, name, price_tiyin, available
		 FROM products WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func scanProducts(rows pgx.Rows) ([]*catalog.Product, error) {
	var list []*catalog.Product
	for rows.Next() {
		var x catalog.Product
		if err := rows.Scan(&x.ID, &x.RestaurantID, &x.Name, &x.PriceTiyin, &x.Available); err != nil {
			return nil, err
		}
		list = append(list, &x)
	}
	return list, rows.Err()
}

func (r *PgCatalogRepo) SaveProduct(ctx context.Context, x *catalog.Product) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO products (id, restaurant_id, name, price_tiyin, available)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name, price_tiyin = EXCLUDED.price_tiyin,
			available = EXCLUDED.available`,
		x.ID, x.RestaurantID, x.Name, x.PriceTiyin, x.Available)
	return err
}

// SeedDemoCatalog — dev muhit uchun boshlang'ich restoran va menyu.
func SeedDemoCatalog(ctx context.Context, pool *pgxpool.Pool) error {
	repo := NewPgCatalogRepo(pool)
	for _, x := range DemoRestaurants() {
		if _, err := repo.GetRestaurant(ctx, x.ID); errors.Is(err, catalog.ErrNotFound) {
			cp := x
			if err := repo.SaveRestaurant(ctx, &cp); err != nil {
				return err
			}
		}
	}
	for _, p := range DemoProducts() {
		cp := p
		if _, err := pool.Exec(ctx, `
			INSERT INTO products (id, restaurant_id, name, price_tiyin, available)
			VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO NOTHING`,
			cp.ID, cp.RestaurantID, cp.Name, cp.PriceTiyin, cp.Available); err != nil {
			return err
		}
	}
	return nil
}
