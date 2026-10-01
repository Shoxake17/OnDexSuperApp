package storage

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/promotions"
	"chustapp/internal/staff"
)

// ─── Katalog ───────────────────────────────────────────────────────────

type countingCatalog struct {
	catalog.Repository
	restGets atomic.Int32
	prodGets atomic.Int32
	lastIDs  atomic.Value // []string
}

func (c *countingCatalog) GetRestaurant(ctx context.Context, id string) (*catalog.Restaurant, error) {
	c.restGets.Add(1)
	return c.Repository.GetRestaurant(ctx, id)
}

func (c *countingCatalog) GetProductsByIDs(ctx context.Context, ids []string) ([]*catalog.Product, error) {
	c.prodGets.Add(1)
	c.lastIDs.Store(append([]string(nil), ids...))
	return c.Repository.GetProductsByIDs(ctx, ids)
}

func newCachedCatalogForTest() (catalog.Repository, *countingCatalog) {
	inner := &countingCatalog{Repository: NewMemoryCatalogRepo(
		[]catalog.Restaurant{{ID: "r1", Name: "Birinchi", Open: true,
			PaymentMethods: &catalog.PaymentMethods{Cash: true}}},
		[]catalog.Product{
			{ID: "p1", RestaurantID: "r1", Name: "Osh", PriceTiyin: 1000, Available: true},
			{ID: "p2", RestaurantID: "r1", Name: "Somsa", PriceTiyin: 500, Available: true},
		})}
	return NewCachedCatalog(inner, time.Minute), inner
}

func TestCachedCatalogRestaurantHitsCacheOnce(t *testing.T) {
	repo, inner := newCachedCatalogForTest()
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		r, err := repo.GetRestaurant(ctx, "r1")
		if err != nil || r.Name != "Birinchi" {
			t.Fatalf("%v %v", r, err)
		}
	}
	if n := inner.restGets.Load(); n != 1 {
		t.Fatalf("ichki ombor %d marta o'qildi, 1 kutilgan", n)
	}
}

// Qaytarilgan nusxani o'zgartirish boshqa so'rovlarga SIZMASLIGI kerak.
func TestCachedCatalogReturnsIsolatedCopies(t *testing.T) {
	repo, _ := newCachedCatalogForTest()
	ctx := context.Background()
	a, _ := repo.GetRestaurant(ctx, "r1")
	a.Name = "BUZILDI"
	a.PaymentMethods.Cash = false
	a.Open = false

	b, _ := repo.GetRestaurant(ctx, "r1")
	if b.Name != "Birinchi" || !b.PaymentMethods.Cash || !b.Open {
		t.Fatalf("birinchi so'rov o'zgartirishi ikkinchisiga sizdi: %+v %+v", b, b.PaymentMethods)
	}
}

// ASOSIY: yozishdan keyin o'qish DARHOL yangi qiymatni ko'radi (restoran yopildi,
// narx o'zgardi). Aks holda yopiq restoranga buyurtma qabul qilinardi.
func TestCachedCatalogWriteIsVisibleImmediately(t *testing.T) {
	repo, _ := newCachedCatalogForTest()
	ctx := context.Background()
	_, _ = repo.GetRestaurant(ctx, "r1") // keshga tushdi
	ps, _ := repo.GetProductsByIDs(ctx, []string{"p1"})
	if ps[0].PriceTiyin != 1000 {
		t.Fatal("boshlang'ich narx")
	}

	r, _ := repo.GetRestaurant(ctx, "r1")
	r.Open = false
	if err := repo.SaveRestaurant(ctx, r); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetRestaurant(ctx, "r1"); got.Open {
		t.Fatal("yopilgan restoran keshda hali ochiq ko'rinadi")
	}

	p := *ps[0]
	p.PriceTiyin = 2500
	if err := repo.SaveProduct(ctx, &p); err != nil {
		t.Fatal(err)
	}
	ps2, _ := repo.GetProductsByIDs(ctx, []string{"p1"})
	if ps2[0].PriceTiyin != 2500 {
		t.Fatalf("yangi narx %d, 2500 kutilgan (eski narx keshda qolgan)", ps2[0].PriceTiyin)
	}

	if err := repo.DeleteProduct(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if ps3, _ := repo.GetProductsByIDs(ctx, []string{"p1"}); len(ps3) != 0 {
		t.Fatal("o'chirilgan taom keshda qolgan")
	}
}

func TestCachedCatalogProductsFetchOnlyMissing(t *testing.T) {
	repo, inner := newCachedCatalogForTest()
	ctx := context.Background()
	if _, err := repo.GetProductsByIDs(ctx, []string{"p1"}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetProductsByIDs(ctx, []string{"p1", "p2", "p1"}) // takroriy ID ham
	if err != nil || len(got) != 2 {
		t.Fatalf("2 ta noyob taom kutilgan: %v %v", len(got), err)
	}
	ids, _ := inner.lastIDs.Load().([]string)
	if len(ids) != 1 || ids[0] != "p2" {
		t.Fatalf("bazadan faqat p2 so'ralishi kerak edi: %v", ids)
	}
	before := inner.prodGets.Load()
	_, _ = repo.GetProductsByIDs(ctx, []string{"p1", "p2"})
	if inner.prodGets.Load() != before {
		t.Fatal("ikkalasi ham keshda — bazaga murojaat bo'lmasligi kerak")
	}
}

func TestCachedCatalogNotFoundIsNotCached(t *testing.T) {
	repo, _ := newCachedCatalogForTest()
	ctx := context.Background()
	if _, err := repo.GetRestaurant(ctx, "yangi"); err == nil {
		t.Fatal("topilmasligi kerak edi")
	}
	if err := repo.SaveRestaurant(ctx, &catalog.Restaurant{ID: "yangi", Name: "Yangi", Open: true}); err != nil {
		t.Fatal(err)
	}
	if r, err := repo.GetRestaurant(ctx, "yangi"); err != nil || r.Name != "Yangi" {
		t.Fatalf("yangi restoran darhol ko'rinishi kerak: %v %v", r, err)
	}
}

func TestCachedCatalogDeleteRestaurantDropsProducts(t *testing.T) {
	repo, _ := newCachedCatalogForTest()
	ctx := context.Background()
	_, _ = repo.GetProductsByIDs(ctx, []string{"p1"})
	if err := repo.DeleteRestaurant(ctx, "r1"); err != nil {
		t.Skipf("xotira ombori restoran o'chirishni qo'llamaydi: %v", err)
	}
	if _, err := repo.GetRestaurant(ctx, "r1"); err == nil {
		t.Fatal("o'chirilgan restoran keshda qolgan")
	}
}

func TestCachedWrappersDisabledWhenTTLZero(t *testing.T) {
	inner := NewMemoryCatalogRepo(nil, nil)
	if got := NewCachedCatalog(inner, 0); got != catalog.Repository(inner) {
		t.Fatal("ttl=0 bo'lsa ichki ombor qaytishi kerak")
	}
	pr := NewMemoryPromotionsRepo()
	if got := NewCachedPromotions(pr, 0); got != promotions.Repository(pr) {
		t.Fatal("ttl=0 (aksiya)")
	}
	sr := NewMemoryStaffRepo()
	if got := NewCachedStaff(sr, 0); got != staff.Repository(sr) {
		t.Fatal("ttl=0 (xodim)")
	}
}

// ─── Aksiyalar ─────────────────────────────────────────────────────────

type countingPromos struct {
	promotions.Repository
	lists atomic.Int32
}

func (c *countingPromos) ListByRestaurant(ctx context.Context, id string) ([]*promotions.Promotion, error) {
	c.lists.Add(1)
	return c.Repository.ListByRestaurant(ctx, id)
}

func TestCachedPromotions(t *testing.T) {
	ctx := context.Background()
	inner := &countingPromos{Repository: NewMemoryPromotionsRepo()}
	repo := NewCachedPromotions(inner, time.Minute)
	save := func(id, rid string, ids ...string) {
		t.Helper()
		if err := repo.Save(ctx, &promotions.Promotion{ID: id, RestaurantID: rid, Name: id, TargetProductIDs: ids}); err != nil {
			t.Fatal(err)
		}
	}
	save("a1", "r1", "p1")

	for i := 0; i < 5; i++ {
		l, _ := repo.ListByRestaurant(ctx, "r1")
		if len(l) != 1 {
			t.Fatalf("ro'yxat %d", len(l))
		}
	}
	if inner.lists.Load() != 1 {
		t.Fatalf("ichki ombor %d marta o'qildi, 1 kutilgan", inner.lists.Load())
	}

	// Nusxa izolyatsiyasi (slice ham).
	l, _ := repo.ListByRestaurant(ctx, "r1")
	l[0].Name = "BUZILDI"
	l[0].TargetProductIDs[0] = "BUZILDI"
	l2, _ := repo.ListByRestaurant(ctx, "r1")
	if l2[0].Name != "a1" || l2[0].TargetProductIDs[0] != "p1" {
		t.Fatalf("o'zgartirish keshga sizdi: %+v", l2[0])
	}

	// Saqlash — darhol ko'rinadi.
	save("a2", "r1")
	if l3, _ := repo.ListByRestaurant(ctx, "r1"); len(l3) != 2 {
		t.Fatalf("yangi aksiya darhol ko'rinishi kerak: %d", len(l3))
	}
	// Boshqa restoran keshi tegilmaydi, o'chirish hammasini tozalaydi.
	save("b1", "r2")
	_, _ = repo.ListByRestaurant(ctx, "r2")
	if err := repo.Delete(ctx, "a1"); err != nil {
		t.Fatal(err)
	}
	if l4, _ := repo.ListByRestaurant(ctx, "r1"); len(l4) != 1 {
		t.Fatalf("o'chirilgan aksiya keshda qolgan: %d", len(l4))
	}
}

// ─── Xodimlar ──────────────────────────────────────────────────────────

type countingStaff struct {
	staff.Repository
	lists atomic.Int32
}

func (c *countingStaff) List(ctx context.Context, rid string) ([]*staff.Member, error) {
	c.lists.Add(1)
	return c.Repository.List(ctx, rid)
}

func TestCachedStaffDeliveryFeeFollowsWritesImmediately(t *testing.T) {
	ctx := context.Background()
	inner := &countingStaff{Repository: NewMemoryStaffRepo()}
	repo := NewCachedStaff(inner, time.Minute)

	pay := int64(800_000)
	m := &staff.Member{ID: "s1", RestaurantID: "r1", FirstName: "Jasur", Position: staff.PositionCourier,
		Status: staff.StatusActive, AppAccess: true, DeliveryPayTiyin: &pay}
	if err := repo.Create(ctx, m, nil); err != nil {
		t.Fatal(err)
	}
	fee := func() int64 {
		l, err := repo.List(ctx, "r1")
		if err != nil {
			t.Fatal(err)
		}
		return staff.DeliveryFee(l)
	}
	for i := 0; i < 5; i++ {
		if fee() != 800_000 {
			t.Fatalf("yetkazish haqi %d", fee())
		}
	}
	if inner.lists.Load() != 1 {
		t.Fatalf("ichki ombor %d marta o'qildi, 1 kutilgan", inner.lists.Load())
	}

	// PUL: kuryer ta'tilga chiqdi — haq DARHOL 0 bo'lishi kerak (keshda 800 000 qolmasin).
	m.Status = staff.StatusOnLeave
	if err := repo.Update(ctx, m, nil); err != nil {
		t.Fatal(err)
	}
	if got := fee(); got != 0 {
		t.Fatalf("ta'tildagi kuryer haqi keshdan %d, 0 kutilgan", got)
	}

	// Qaytarilgan nusxani o'zgartirish keshga sizmaydi.
	l, _ := repo.List(ctx, "r1")
	l[0].Status = staff.StatusActive
	*l[0].DeliveryPayTiyin = 1
	if got := fee(); got != 0 {
		t.Fatalf("o'zgartirish keshga sizdi: %d", got)
	}
}

func TestCachedStaffListErrorIsNotCached(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("baza yiqildi")
	fs := &failingStaff{Repository: NewMemoryStaffRepo(), err: boom}
	repo := NewCachedStaff(fs, time.Minute)
	if _, err := repo.List(ctx, "r1"); !errors.Is(err, boom) {
		t.Fatalf("xato kutilgan: %v", err)
	}
	fs.err = nil
	if _, err := repo.List(ctx, "r1"); err != nil {
		t.Fatalf("xato keshlangan: %v", err)
	}
}

type failingStaff struct {
	staff.Repository
	err error
}

func (f *failingStaff) List(ctx context.Context, rid string) ([]*staff.Member, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.Repository.List(ctx, rid)
}
