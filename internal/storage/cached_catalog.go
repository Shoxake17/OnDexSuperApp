package storage

import (
	"context"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/ttlcache"
)

// CachedCatalog — katalog ombori ustidan QISQA kesh: restoran va taomni
// bitta-bittadan o'qish (narxlash, buyurtma yaratish, menyu mavjudligi).
//
// Faqat `GetRestaurant` va `GetProductsByIDs` keshlanadi. Ro'yxat va qidiruv
// yo'llari (`ListRestaurants`, `ListProducts`, `SearchProducts`) o'zgarishsiz
// o'tadi: ular Redis keshi bilan allaqachon qoplangan.
//
// Yozish yo'llari (`Save*`, `Delete*`) shu dekorator orqali o'tadi va
// kalitni DARHOL bekor qiladi — shuning uchun restoran yopilsa yoki taom narxi
// o'zgarsa, shu jarayonda keyingi so'rov yangi qiymatni ko'radi
// (`ttlcache.Cache` izohiga qarang). Server nusxasi bitta; ko'p nusxada eskirish
// chegarasi TTL (standart 2 s).
type CachedCatalog struct {
	catalog.Repository
	rests *ttlcache.Cache[catalog.Restaurant]
	prods *ttlcache.Cache[catalog.Product]
}

// NewCachedCatalog — `ttl <= 0` bo'lsa kesh o'chiq (ichki ombor o'zi qaytadi).
func NewCachedCatalog(inner catalog.Repository, ttl time.Duration) catalog.Repository {
	if ttl <= 0 {
		return inner
	}
	return &CachedCatalog{
		Repository: inner,
		rests:      ttlcache.New[catalog.Restaurant](ttl, 4096),
		prods:      ttlcache.New[catalog.Product](ttl, 16384),
	}
}

func (c *CachedCatalog) GetRestaurant(ctx context.Context, id string) (*catalog.Restaurant, error) {
	v, err := c.rests.Get(id, func() (catalog.Restaurant, error) {
		// Birinchi chaqiruvchi bekor qilinsa, kutayotganlar xato olmasin.
		r, err := c.Repository.GetRestaurant(context.WithoutCancel(ctx), id)
		if err != nil {
			return catalog.Restaurant{}, err
		}
		return cloneRestaurant(*r), nil
	})
	if err != nil {
		return nil, err
	}
	out := cloneRestaurant(v)
	return &out, nil
}

func (c *CachedCatalog) GetProductsByIDs(ctx context.Context, ids []string) ([]*catalog.Product, error) {
	out := make([]*catalog.Product, 0, len(ids))
	var missing []string
	seen := make(map[string]bool, len(ids))
	var epoch uint64
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		v, ok, e := c.prods.Peek(id)
		epoch = e
		if ok {
			cp := v
			out = append(out, &cp)
		} else {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return out, nil
	}
	fetched, err := c.Repository.GetProductsByIDs(ctx, missing)
	if err != nil {
		return nil, err
	}
	for _, p := range fetched {
		c.prods.Put(p.ID, *p, epoch)
		cp := *p
		out = append(out, &cp)
	}
	return out, nil
}

func (c *CachedCatalog) SaveRestaurant(ctx context.Context, r *catalog.Restaurant) error {
	err := c.Repository.SaveRestaurant(ctx, r)
	c.rests.Invalidate(r.ID) // xato bo'lsa ham: holat noaniq — ishonchli tomoni
	return err
}

func (c *CachedCatalog) DeleteRestaurant(ctx context.Context, id string) error {
	err := c.Repository.DeleteRestaurant(ctx, id)
	c.rests.Invalidate(id)
	c.prods.Clear() // restoran taomlari ham ketadi (ID'lari noma'lum)
	return err
}

func (c *CachedCatalog) SaveProduct(ctx context.Context, p *catalog.Product) error {
	err := c.Repository.SaveProduct(ctx, p)
	c.prods.Invalidate(p.ID)
	return err
}

func (c *CachedCatalog) DeleteProduct(ctx context.Context, id string) error {
	err := c.Repository.DeleteProduct(ctx, id)
	c.prods.Invalidate(id)
	return err
}
