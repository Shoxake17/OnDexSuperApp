package storage

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/catalog"
)

type PgCatalogRepo struct{ pool *pgxpool.Pool }

func NewPgCatalogRepo(pool *pgxpool.Pool) *PgCatalogRepo { return &PgCatalogRepo{pool: pool} }

const restaurantColumns = `id, name, address, lat, lng, open, logo_url, cover_url, tags`

func (r *PgCatalogRepo) ListRestaurants(ctx context.Context) ([]*catalog.Restaurant, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+restaurantColumns+` FROM restaurants ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*catalog.Restaurant
	for rows.Next() {
		var x catalog.Restaurant
		if err := rows.Scan(&x.ID, &x.Name, &x.Address, &x.Lat, &x.Lng, &x.Open,
			&x.LogoURL, &x.CoverURL, &x.Tags); err != nil {
			return nil, err
		}
		list = append(list, &x)
	}
	return list, rows.Err()
}

func (r *PgCatalogRepo) GetRestaurant(ctx context.Context, id string) (*catalog.Restaurant, error) {
	var x catalog.Restaurant
	err := r.pool.QueryRow(ctx,
		`SELECT `+restaurantColumns+` FROM restaurants WHERE id = $1`, id,
	).Scan(&x.ID, &x.Name, &x.Address, &x.Lat, &x.Lng, &x.Open, &x.LogoURL, &x.CoverURL, &x.Tags)
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
		INSERT INTO restaurants (id, name, address, lat, lng, open, logo_url, cover_url, tags)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name, address = EXCLUDED.address,
			lat = EXCLUDED.lat, lng = EXCLUDED.lng, open = EXCLUDED.open,
			logo_url = EXCLUDED.logo_url, cover_url = EXCLUDED.cover_url,
			tags = EXCLUDED.tags`,
		x.ID, x.Name, x.Address, x.Lat, x.Lng, x.Open, x.LogoURL, x.CoverURL, x.Tags)
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

const productColumns = `id, restaurant_id, name, price_tiyin, discount_price_tiyin, available, category, image_url, stock, weight, weight_unit, description, prep_time_text`

func (r *PgCatalogRepo) ListProducts(ctx context.Context, restaurantID string) ([]*catalog.Product, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+productColumns+` FROM products WHERE restaurant_id = $1 ORDER BY name`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (r *PgCatalogRepo) GetProductsByIDs(ctx context.Context, ids []string) ([]*catalog.Product, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+productColumns+` FROM products WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

// SearchProducts — barcha restoranlar bo'yicha nomi yoki turkumi so'rovga
// mos mavjud taomlarni qaytaradi, tegishli restoran nomi/logotipi/holati
// bilan birga (JOIN orqali). Moslik catalog.NormalizeForSearch orqali
// tekshiriladi (bo'shliq/registrga sezgir emas) — shuning uchun aniq SQL
// ILIKE o'rniga barcha mavjud taomlar olinib, Go kodida filtrlanadi.
func (r *PgCatalogRepo) SearchProducts(ctx context.Context, query string) ([]*catalog.ProductSearchResult, error) {
	nq := catalog.NormalizeForSearch(query)
	if nq == "" {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.id, p.restaurant_id, p.name, p.price_tiyin, p.discount_price_tiyin, p.available, p.category,
		       p.image_url, p.stock, p.weight, p.weight_unit, p.description, p.prep_time_text,
		       r.name, r.logo_url, r.open
		FROM products p
		JOIN restaurants r ON r.id = p.restaurant_id
		WHERE p.available = true
		ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*catalog.ProductSearchResult
	for rows.Next() {
		var x catalog.ProductSearchResult
		if err := rows.Scan(&x.ID, &x.RestaurantID, &x.Name, &x.PriceTiyin, &x.DiscountPriceTiyin, &x.Available,
			&x.Category, &x.ImageURL, &x.Stock, &x.Weight, &x.WeightUnit, &x.Description, &x.PrepTimeText,
			&x.RestaurantName, &x.RestaurantLogoURL, &x.RestaurantOpen); err != nil {
			return nil, err
		}
		if !strings.Contains(catalog.NormalizeForSearch(x.Name), nq) &&
			!strings.Contains(catalog.NormalizeForSearch(x.Category), nq) {
			continue
		}
		list = append(list, &x)
	}
	return list, rows.Err()
}

func scanProducts(rows pgx.Rows) ([]*catalog.Product, error) {
	var list []*catalog.Product
	for rows.Next() {
		var x catalog.Product
		if err := rows.Scan(&x.ID, &x.RestaurantID, &x.Name, &x.PriceTiyin, &x.DiscountPriceTiyin, &x.Available,
			&x.Category, &x.ImageURL, &x.Stock, &x.Weight, &x.WeightUnit, &x.Description, &x.PrepTimeText); err != nil {
			return nil, err
		}
		list = append(list, &x)
	}
	return list, rows.Err()
}

func (r *PgCatalogRepo) SaveProduct(ctx context.Context, x *catalog.Product) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO products (id, restaurant_id, name, price_tiyin, discount_price_tiyin, available, category, image_url, stock, weight, weight_unit, description, prep_time_text)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name, price_tiyin = EXCLUDED.price_tiyin,
			discount_price_tiyin = EXCLUDED.discount_price_tiyin,
			available = EXCLUDED.available, category = EXCLUDED.category,
			image_url = EXCLUDED.image_url, stock = EXCLUDED.stock,
			weight = EXCLUDED.weight, weight_unit = EXCLUDED.weight_unit,
			description = EXCLUDED.description, prep_time_text = EXCLUDED.prep_time_text`,
		x.ID, x.RestaurantID, x.Name, x.PriceTiyin, x.DiscountPriceTiyin, x.Available, x.Category, x.ImageURL, x.Stock,
		x.Weight, x.WeightUnit, x.Description, x.PrepTimeText)
	return err
}

func (r *PgCatalogRepo) DeleteProduct(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM products WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return catalog.ErrNotFound
	}
	return nil
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
			INSERT INTO products (id, restaurant_id, name, price_tiyin, available, category)
			VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO NOTHING`,
			cp.ID, cp.RestaurantID, cp.Name, cp.PriceTiyin, cp.Available, cp.Category); err != nil {
			return err
		}
	}
	return nil
}
