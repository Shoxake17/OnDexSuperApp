package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/banners"
)

type PgBannerRepo struct{ pool *pgxpool.Pool }

func NewPgBannerRepo(pool *pgxpool.Pool) *PgBannerRepo { return &PgBannerRepo{pool: pool} }

const bannerColumns = `id, image_url, restaurant_id, active, sort_order, created_at, updated_at,
	placement, title, subtitle, button_text, action`

func scanBanner(row pgx.Row) (*banners.Banner, error) {
	var b banners.Banner
	if err := row.Scan(&b.ID, &b.ImageURL, &b.RestaurantID, &b.Active, &b.SortOrder,
		&b.CreatedAt, &b.UpdatedAt, &b.Placement, &b.Title, &b.Subtitle, &b.ButtonText, &b.Action); err != nil {
		return nil, err
	}
	return &b, nil
}

func scanBanners(rows pgx.Rows) ([]*banners.Banner, error) {
	defer rows.Close()
	list := make([]*banners.Banner, 0)
	for rows.Next() {
		b, err := scanBanner(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, rows.Err()
}

func (r *PgBannerRepo) List(ctx context.Context) ([]*banners.Banner, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+bannerColumns+` FROM promo_banners ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, err
	}
	return scanBanners(rows)
}

func (r *PgBannerRepo) ListActive(ctx context.Context) ([]*banners.Banner, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+bannerColumns+` FROM promo_banners WHERE active ORDER BY sort_order, created_at`)
	if err != nil {
		return nil, err
	}
	return scanBanners(rows)
}

func (r *PgBannerRepo) Get(ctx context.Context, id string) (*banners.Banner, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+bannerColumns+` FROM promo_banners WHERE id = $1`, id)
	b, err := scanBanner(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, banners.ErrNotFound
		}
		return nil, err
	}
	return b, nil
}

func (r *PgBannerRepo) Create(ctx context.Context, b *banners.Banner) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO promo_banners (id, image_url, restaurant_id, active, sort_order, created_at, updated_at,
		                            placement, title, subtitle, button_text, action)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		b.ID, b.ImageURL, b.RestaurantID, b.Active, b.SortOrder, b.CreatedAt, b.UpdatedAt,
		b.Placement, b.Title, b.Subtitle, b.ButtonText, b.Action)
	return err
}

func (r *PgBannerRepo) Update(ctx context.Context, b *banners.Banner) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE promo_banners
		 SET image_url = $2, restaurant_id = $3, active = $4, sort_order = $5, updated_at = $6,
		     placement = $7, title = $8, subtitle = $9, button_text = $10, action = $11
		 WHERE id = $1`,
		b.ID, b.ImageURL, b.RestaurantID, b.Active, b.SortOrder, b.UpdatedAt,
		b.Placement, b.Title, b.Subtitle, b.ButtonText, b.Action)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return banners.ErrNotFound
	}
	return nil
}

func (r *PgBannerRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM promo_banners WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return banners.ErrNotFound
	}
	return nil
}
