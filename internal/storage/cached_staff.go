package storage

import (
	"context"
	"time"

	"chustapp/internal/staff"
	"chustapp/internal/ttlcache"
)

// CachedStaff — xodimlar ombori ustidan QISQA kesh: har `/quote` va buyurtma
// yaratishda yetkazish haqi uchun restoran xodimlari ro'yxati o'qiladi
// (`NewDeliveryPricer`).
//
// Yetkazish haqi PUL: xodim qo'shilsa, maosh usuli o'zgarsa, ta'tilga
// chiqsa yoki bo'shatilsa kesh DARHOL bekor qilinadi (`Create`/`Update`),
// shuning uchun mijoz eski haqni ko'rmaydi. Boshqa o'qish yo'llari (`Get`,
// `Events`) keshlanmaydi.
type CachedStaff struct {
	staff.Repository
	lists *ttlcache.Cache[[]staff.Member]
}

// NewCachedStaff — `ttl <= 0` bo'lsa kesh o'chiq.
func NewCachedStaff(inner staff.Repository, ttl time.Duration) staff.Repository {
	if ttl <= 0 {
		return inner
	}
	return &CachedStaff{Repository: inner, lists: ttlcache.New[[]staff.Member](ttl, 4096)}
}

func (c *CachedStaff) List(ctx context.Context, restaurantID string) ([]*staff.Member, error) {
	v, err := c.lists.Get(restaurantID, func() ([]staff.Member, error) {
		list, err := c.Repository.List(context.WithoutCancel(ctx), restaurantID)
		if err != nil {
			return nil, err
		}
		out := make([]staff.Member, 0, len(list))
		for _, m := range list {
			out = append(out, staff.Clone(*m))
		}
		return out, nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]*staff.Member, 0, len(v))
	for _, m := range v {
		cp := staff.Clone(m)
		out = append(out, &cp)
	}
	return out, nil
}

func (c *CachedStaff) Create(ctx context.Context, m *staff.Member, events []staff.Event) error {
	err := c.Repository.Create(ctx, m, events)
	c.lists.Invalidate(m.RestaurantID)
	return err
}

func (c *CachedStaff) Update(ctx context.Context, m *staff.Member, events []staff.Event) error {
	err := c.Repository.Update(ctx, m, events)
	c.lists.Invalidate(m.RestaurantID)
	return err
}
