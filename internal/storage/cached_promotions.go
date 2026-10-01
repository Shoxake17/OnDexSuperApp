package storage

import (
	"context"
	"slices"
	"time"

	"chustapp/internal/promotions"
	"chustapp/internal/ttlcache"
)

// CachedPromotions — aksiyalar ombori ustidan QISQA kesh: har `/quote` va
// buyurtma yaratishda restoran aksiyalari ro'yxati o'qiladi.
//
// Aksiyaning "faol/muddati o'tgan" holati keshlanmaydi — u har safar `now`
// bilan hisoblanadi (`promotions.Apply`), shuning uchun muddat chegarasida
// kesh xato bermaydi. Saqlash va o'chirish ro'yxatni darhol bekor qiladi.
//
// `IncrementUsage` ro'yxatni bekor QILMAYDI: u har buyurtmada chaqiriladi va
// faqat sanagichni o'zgartiradi (narxlashga ta'sir qilmaydi); panelda sanagich
// TTL ichida biroz eskirishi mumkin.
type CachedPromotions struct {
	promotions.Repository
	lists *ttlcache.Cache[[]promotions.Promotion]
}

// NewCachedPromotions — `ttl <= 0` bo'lsa kesh o'chiq.
func NewCachedPromotions(inner promotions.Repository, ttl time.Duration) promotions.Repository {
	if ttl <= 0 {
		return inner
	}
	return &CachedPromotions{Repository: inner, lists: ttlcache.New[[]promotions.Promotion](ttl, 4096)}
}

func clonePromotion(p promotions.Promotion) promotions.Promotion {
	p.TargetProductIDs = slices.Clone(p.TargetProductIDs)
	p.TargetCategories = slices.Clone(p.TargetCategories)
	return p
}

func (c *CachedPromotions) ListByRestaurant(ctx context.Context, restaurantID string) ([]*promotions.Promotion, error) {
	v, err := c.lists.Get(restaurantID, func() ([]promotions.Promotion, error) {
		list, err := c.Repository.ListByRestaurant(context.WithoutCancel(ctx), restaurantID)
		if err != nil {
			return nil, err
		}
		out := make([]promotions.Promotion, 0, len(list))
		for _, p := range list {
			out = append(out, clonePromotion(*p))
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	var out []*promotions.Promotion
	for _, p := range v {
		cp := clonePromotion(p)
		out = append(out, &cp)
	}
	return out, nil
}

func (c *CachedPromotions) Save(ctx context.Context, p *promotions.Promotion) error {
	err := c.Repository.Save(ctx, p)
	c.lists.Invalidate(p.RestaurantID)
	return err
}

func (c *CachedPromotions) Delete(ctx context.Context, id string) error {
	err := c.Repository.Delete(ctx, id)
	c.lists.Clear() // aksiya qaysi restoranniki ekani noma'lum
	return err
}

func (c *CachedPromotions) DeleteByRestaurant(ctx context.Context, restaurantID string) (int, error) {
	n, err := c.Repository.DeleteByRestaurant(ctx, restaurantID)
	c.lists.Invalidate(restaurantID)
	return n, err
}
