package httpapi

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"chustapp/internal/ratings"
	"chustapp/internal/storage"
)

// countingRatings — Summary necha marta bazaga borganini sanaydi.
type countingRatings struct {
	ratings.Repository
	calls atomic.Int64
	avg   atomic.Value // float64
}

func (c *countingRatings) Summary(context.Context, string) (float64, int, error) {
	c.calls.Add(1)
	v, _ := c.avg.Load().(float64)
	return v, 1, nil
}

// Yuklama testida eng sekin yo'l (har so'rovda reyting + ETA) keshlangan:
// takroriy va PARALLEL so'rovlar bazaga bitta so'rov bilan xizmat qilinadi,
// yangi baho esa kalitni DARHOL bekor qiladi (eski reyting ko'rinmaydi).
func TestRestaurantDerivedCacheHitAndInvalidate(t *testing.T) {
	cr := &countingRatings{}
	cr.avg.Store(4.0)
	s := New(Deps{RatingsRepo: cr, OrderRepo: storage.NewMemoryOrderRepo()})
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = s.derivedFor(ctx, "r1") }()
	}
	wg.Wait()
	if d := s.derivedFor(ctx, "r1"); d.rating != 4.0 {
		t.Fatalf("reyting 4.0 kutilgan, keldi %v", d.rating)
	}
	if n := cr.calls.Load(); n != 1 {
		t.Fatalf("51 ta so'rov bazaga 1 marta borishi kerak, %d marta bordi", n)
	}

	// Yangi baho: kalit bekor qilinadi, keyingi so'rov YANGI qiymatni oladi.
	cr.avg.Store(4.5)
	s.invalidateRestaurantDerived("r1")
	if d := s.derivedFor(ctx, "r1"); d.rating != 4.5 {
		t.Fatalf("bekor qilingach yangi reyting 4.5 kutilgan, keldi %v", d.rating)
	}
	if n := cr.calls.Load(); n != 2 {
		t.Fatalf("bekor qilingach bazaga yana 1 marta borishi kerak, jami %d", n)
	}
	// Boshqa restoran kaliti alohida.
	_ = s.derivedFor(ctx, "r2")
	if n := cr.calls.Load(); n != 3 {
		t.Fatalf("r2 alohida kalit bo'lishi kerak, jami %d", n)
	}
}
