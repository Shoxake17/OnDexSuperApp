package storage

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/promotions"
)

type PgPromotionsRepo struct{ pool *pgxpool.Pool }

func NewPgPromotionsRepo(pool *pgxpool.Pool) *PgPromotionsRepo { return &PgPromotionsRepo{pool: pool} }

const promotionColumns = `id, restaurant_id, name, description, type, discount_unit, discount_value,
	min_order_amount_tiyin, max_discount_amount_tiyin, start_at, end_at, indefinite, active,
	applies_to_products, applies_to_orders, applies_to_categories, target_product_ids, target_categories,
	min_previous_orders, usage_count, sales_total_tiyin, image_url, created_at`

func scanPromotions(rows pgx.Rows) ([]*promotions.Promotion, error) {
	var list []*promotions.Promotion
	for rows.Next() {
		var x promotions.Promotion
		var typ, unit string
		if err := rows.Scan(&x.ID, &x.RestaurantID, &x.Name, &x.Description, &typ, &unit, &x.DiscountValue,
			&x.MinOrderAmountTiyin, &x.MaxDiscountAmountTiyin, &x.StartAt, &x.EndAt, &x.Indefinite, &x.Active,
			&x.AppliesToProducts, &x.AppliesToOrders, &x.AppliesToCategories, &x.TargetProductIDs, &x.TargetCategories,
			&x.MinPreviousOrders, &x.UsageCount, &x.SalesTotalTiyin, &x.ImageURL, &x.CreatedAt); err != nil {
			return nil, err
		}
		x.Type = promotions.Type(typ)
		x.DiscountUnit = promotions.DiscountUnit(unit)
		list = append(list, &x)
	}
	return list, rows.Err()
}

func (r *PgPromotionsRepo) ListByRestaurant(ctx context.Context, restaurantID string) ([]*promotions.Promotion, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+promotionColumns+` FROM promotions WHERE restaurant_id = $1 ORDER BY created_at DESC`, restaurantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPromotions(rows)
}

func (r *PgPromotionsRepo) GetByID(ctx context.Context, id string) (*promotions.Promotion, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+promotionColumns+` FROM promotions WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanPromotions(rows)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, promotions.ErrNotFound
	}
	return list[0], nil
}

// Save — usage_count/sales_total_tiyin ATAYLAB UPDATE SET ro'yxatiga
// kiritilmagan: ular faqat IncrementUsage orqali o'zgaradi, aksiyani
// tahrirlash (Save) ularni hech qachon 0'ga qaytarib qo'ymasligi kerak.
func (r *PgPromotionsRepo) Save(ctx context.Context, x *promotions.Promotion) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO promotions (id, restaurant_id, name, description, type, discount_unit, discount_value,
			min_order_amount_tiyin, max_discount_amount_tiyin, start_at, end_at, indefinite, active,
			applies_to_products, applies_to_orders, applies_to_categories, target_product_ids, target_categories,
			min_previous_orders, usage_count, sales_total_tiyin, image_url, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name, description = EXCLUDED.description, type = EXCLUDED.type,
			discount_unit = EXCLUDED.discount_unit, discount_value = EXCLUDED.discount_value,
			min_order_amount_tiyin = EXCLUDED.min_order_amount_tiyin,
			max_discount_amount_tiyin = EXCLUDED.max_discount_amount_tiyin,
			start_at = EXCLUDED.start_at, end_at = EXCLUDED.end_at, indefinite = EXCLUDED.indefinite,
			active = EXCLUDED.active, applies_to_products = EXCLUDED.applies_to_products,
			applies_to_orders = EXCLUDED.applies_to_orders, applies_to_categories = EXCLUDED.applies_to_categories,
			target_product_ids = EXCLUDED.target_product_ids, target_categories = EXCLUDED.target_categories,
			min_previous_orders = EXCLUDED.min_previous_orders, image_url = EXCLUDED.image_url`,
		x.ID, x.RestaurantID, x.Name, x.Description, string(x.Type), string(x.DiscountUnit), x.DiscountValue,
		x.MinOrderAmountTiyin, x.MaxDiscountAmountTiyin, x.StartAt, x.EndAt, x.Indefinite, x.Active,
		x.AppliesToProducts, x.AppliesToOrders, x.AppliesToCategories, x.TargetProductIDs, x.TargetCategories,
		x.MinPreviousOrders, x.UsageCount, x.SalesTotalTiyin, x.ImageURL, x.CreatedAt)
	return err
}

func (r *PgPromotionsRepo) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM promotions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return promotions.ErrNotFound
	}
	return nil
}

// DeleteByRestaurant — qarang: promotions.Repository izohi (FK).
func (r *PgPromotionsRepo) DeleteByRestaurant(ctx context.Context, restaurantID string) (int, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM promotions WHERE restaurant_id = $1`, restaurantID)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (r *PgPromotionsRepo) IncrementUsage(ctx context.Context, id string, amountTiyin int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE promotions SET usage_count = usage_count + 1, sales_total_tiyin = sales_total_tiyin + $2 WHERE id = $1`,
		id, amountTiyin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return promotions.ErrNotFound
	}
	return nil
}
