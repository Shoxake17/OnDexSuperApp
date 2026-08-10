package orders

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeOCCRepo — Repository'ning to'g'ri optimistik parallel boshqaruv
// semantikasiga ega minimal implementatsiyasi (xuddi internal/storage'dagi
// Memory/Pg repolar kabi). Haqiqiy storage.MemoryOrderRepo'ni shu yerdan
// import qilib bo'lmaydi (storage paketi orders'ni import qiladi — tsikl
// bo'lardi), shuning uchun testga xos kichik ekvivalenti yozilgan.
type fakeOCCRepo struct {
	mu   sync.Mutex
	data map[string]Order
}

func newFakeOCCRepo() *fakeOCCRepo { return &fakeOCCRepo{data: make(map[string]Order)} }

func (r *fakeOCCRepo) GetByID(_ context.Context, id string) (*Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	o, ok := r.data[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := o
	return &cp, nil
}

func (r *fakeOCCRepo) Save(_ context.Context, o *Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, exists := r.data[o.ID]
	if exists {
		if o.Version != existing.Version {
			return ErrConflict
		}
		o.Version++
	} else if o.IdempotencyKey != "" {
		for _, other := range r.data {
			if other.CustomerID == o.CustomerID && other.IdempotencyKey == o.IdempotencyKey {
				return ErrDuplicateIdempotencyKey
			}
		}
	}
	r.data[o.ID] = *o
	return nil
}

func (r *fakeOCCRepo) FindByIdempotencyKey(_ context.Context, customerID, key string) (*Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, o := range r.data {
		if o.CustomerID == customerID && o.IdempotencyKey == key && key != "" {
			cp := o
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (r *fakeOCCRepo) ListRecent(context.Context, int) ([]*Order, error) { return nil, nil }
func (r *fakeOCCRepo) HasActiveByRestaurant(context.Context, string) (bool, error) {
	return false, nil
}
func (r *fakeOCCRepo) GetActiveByCourier(context.Context, string) (*Order, error) {
	return nil, ErrNotFound
}
func (r *fakeOCCRepo) ListByRestaurant(context.Context, string, int) ([]*Order, error) {
	return nil, nil
}
func (r *fakeOCCRepo) ListByCustomer(context.Context, string, int) ([]*Order, error) {
	return nil, nil
}
func (r *fakeOCCRepo) CountByCustomerAndRestaurant(context.Context, string, string) (int, error) {
	return 0, nil
}

// TestChangeStatus_ConcurrentConflictingTransitions — xavfsizlik auditida
// topilgan aynan haqiqiy stsenariy: kuryer "picked_up" bosayotganda, AYNAN
// SHU ONDA admin "cancel" bossa. Optimistik parallel boshqaruvsiz ikkalasi
// ham eski StatusReady holatini o'qib, ikkalasi ham Save chaqirardi —
// oxirgi yozgan g'olib chiqar, YUTQAZGAN tomonning history yozuvi esa
// (uning Save'i g'olibning history'sidan XABARSIZ, eski bo'sh history
// asosida yozgani uchun) SILENTLY yo'qolib ketardi.
//
// MUHIM NUANS (test yozishda aniqlandi): picked_up -> cancelled ADMIN uchun
// HAQIQATDA ham ruxsat etilgan o'tish (statemachine.go) — ya'ni ikkala
// so'rov ham OXIR-OQIBAT muvaffaqiyatli bo'lishi TO'G'RI natija (admin
// olib ketilgan buyurtmani ham bekor qila olishi kerak), shuning uchun
// "faqat bittasi yutishi kerak" degan taxmin NOTO'G'RI edi. Tuzatishning
// haqiqiy kafolati boshqa narsa: nechta so'rov muvaffaqiyatli bo'lishidan
// qat'iy nazar, HECH BIR history yozuvi yo'qolmasligi va zanjir (har
// keyingi yozuvning "From"i oldingisining "To"siga teng) uzilmasligi kerak.
func TestChangeStatus_ConcurrentConflictingTransitions(t *testing.T) {
	repo := newFakeOCCRepo()
	svc := NewService(repo, nil, func() string { return "o1" }, nil)

	now := time.Now()
	repo.data["o1"] = Order{
		ID:        "o1",
		Status:    StatusReady,
		CourierID: "c1",
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := svc.ChangeStatus(context.Background(), "o1", StatusPickedUp, ActorCourier)
		results <- err
	}()
	go func() {
		defer wg.Done()
		_, err := svc.ChangeStatus(context.Background(), "o1", StatusCancelled, ActorAdmin)
		results <- err
	}()
	wg.Wait()
	close(results)

	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded == 0 {
		t.Fatalf("kamida bittasi muvaffaqiyatli bo'lishi kerak edi")
	}

	final, err := repo.GetByID(context.Background(), "o1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	// ENG MUHIM tekshiruv — aynan tuzatilgan bug shu yerda: eski kodda
	// yutqazgan tomonning Save'i g'olibning history yozuvini bosib
	// (overwrite qilib) yo'qotib yuborardi. Endi muvaffaqiyatli so'rovlar
	// SONI qancha bo'lishidan qat'iy nazar, history uzunligi ANIQ shunga
	// teng bo'lishi kerak — hech biri yo'qolmagan.
	if len(final.History) != succeeded {
		t.Fatalf("history uzunligi muvaffaqiyatli so'rovlar soniga mos kelishi kerak: succeeded=%d, history=%d",
			succeeded, len(final.History))
	}
	// Zanjir izchilligi: har bir keyingi yozuvning "From"i oldingisining
	// "To"siga teng bo'lishi kerak — aks holda bu ham yo'qolgan/almashtirilgan
	// oraliq bosqichni bildiradi.
	prev := StatusReady
	for i, h := range final.History {
		if h.From != prev {
			t.Fatalf("history[%d].From=%s, kutilgan=%s (zanjir uzilgan)", i, h.From, prev)
		}
		prev = h.To
	}
	if final.Status != prev {
		t.Fatalf("yakuniy status (%s) history zanjiridagi oxirgi holatga (%s) mos kelmayapti", final.Status, prev)
	}
}

// TestChangeStatus_ManyConcurrentSameTransition — bir xil o'tishga bir
// vaqtda ko'plab urinish (masalan tarmoq qayta yuborishi/double-click) —
// FAQAT bittasi qo'llanishi, history'da faqat bitta yozuv qolishi kerak.
func TestChangeStatus_ManyConcurrentSameTransition(t *testing.T) {
	repo := newFakeOCCRepo()
	svc := NewService(repo, nil, func() string { return "o1" }, nil)
	now := time.Now()
	repo.data["o1"] = Order{ID: "o1", Status: StatusCreated, Version: 1, CreatedAt: now, UpdatedAt: now}

	const n = 20
	var wg sync.WaitGroup
	successes := make(chan struct{}, n)
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err := svc.ChangeStatus(context.Background(), "o1", StatusAccepted, ActorRestaurant)
			if err == nil {
				successes <- struct{}{}
				return
			}
			if errors.Is(err, ErrConflict) {
				return
			}
			var terr *TransitionError
			if !errors.As(err, &terr) {
				t.Errorf("kutilmagan xato turi: %v", err)
			}
		}()
	}
	wg.Wait()
	close(successes)

	count := 0
	for range successes {
		count++
	}
	if count != 1 {
		t.Fatalf("aynan 1 ta so'rov muvaffaqiyatli bo'lishi kerak edi, bo'ldi: %d", count)
	}

	final, err := repo.GetByID(context.Background(), "o1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(final.History) != 1 {
		t.Fatalf("history'da aynan 1 ta yozuv bo'lishi kerak, topildi: %d", len(final.History))
	}
}

// newCountingIDGen — har chaqiruvda YANGI, noyob ID beradi (idempotentlik
// testlarida Create() haqiqatan ham bir necha marta chaqirilishi kerak —
// boshqa testlardagi kabi bitta qattiq "o1" idgen bu yerda mos emas, aks
// holda ikkinchi Create() chaqiruvi xuddi ShangeStatus kabi versiya
// ziddiyatiga tushib qolardi, idempotentlik yo'lini umuman sinamas edi).
func newCountingIDGen() func() string {
	n := 0
	return func() string {
		n++
		return fmt.Sprintf("order-%d", n)
	}
}

func sampleItems() []Item {
	return []Item{{ProductID: "p1", Name: "Osh", Qty: 1, PriceTiyin: 25000}}
}

// TestCreate_IdempotencyKey_ReturnsExistingOrder — xavfsizlik auditida
// topilgan muammo: mijoz "Buyurtma berish"ni bossa-yu, tarmoq javobi
// kelmay qolsa (yoki foydalanuvchi shoshilib qayta bossa), ESKI kodda
// server buni bilmay ikkinchi (dublikat) buyurtma yaratardi. Endi: xuddi
// shu idempotency-key bilan ikkinchi Create() chaqiruvi YANGI buyurtma
// yaratmasdan BIRINCHISINI qaytarishi kerak.
func TestCreate_IdempotencyKey_ReturnsExistingOrder(t *testing.T) {
	repo := newFakeOCCRepo()
	svc := NewService(repo, nil, newCountingIDGen(), nil)

	first, err := svc.Create(context.Background(), &Order{
		CustomerID:     "cust1",
		RestaurantID:   "r1",
		Items:          sampleItems(),
		IdempotencyKey: "key-abc",
	})
	if err != nil {
		t.Fatalf("birinchi Create: %v", err)
	}

	second, err := svc.Create(context.Background(), &Order{
		CustomerID:     "cust1",
		RestaurantID:   "r1",
		Items:          sampleItems(),
		IdempotencyKey: "key-abc",
	})
	if err != nil {
		t.Fatalf("ikkinchi (takroriy) Create: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("ikkinchi so'rov YANGI buyurtma yaratdi (dublikat!): first.ID=%s second.ID=%s", first.ID, second.ID)
	}
	if len(repo.data) != 1 {
		t.Fatalf("repo'da aynan 1 ta buyurtma bo'lishi kerak edi, topildi: %d", len(repo.data))
	}
}

// TestCreate_NoIdempotencyKey_AlwaysCreatesNew — kalit yuborilmasa (bo'sh
// qator — eski klientlar yoki kalit talab qilinmaydigan holatlar), tekshiruv
// butunlay o'tkazib yuboriladi, har chaqiruv YANGI buyurtma yaratadi —
// idempotentlik ixtiyoriy, majburiy emas.
func TestCreate_NoIdempotencyKey_AlwaysCreatesNew(t *testing.T) {
	repo := newFakeOCCRepo()
	svc := NewService(repo, nil, newCountingIDGen(), nil)

	first, err := svc.Create(context.Background(), &Order{
		CustomerID: "cust1", RestaurantID: "r1", Items: sampleItems(),
	})
	if err != nil {
		t.Fatalf("birinchi Create: %v", err)
	}
	second, err := svc.Create(context.Background(), &Order{
		CustomerID: "cust1", RestaurantID: "r1", Items: sampleItems(),
	})
	if err != nil {
		t.Fatalf("ikkinchi Create: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("kalitsiz so'rovlar bir xil ID olishi mumkin emas")
	}
	if len(repo.data) != 2 {
		t.Fatalf("repo'da 2 ta ALOHIDA buyurtma bo'lishi kerak edi, topildi: %d", len(repo.data))
	}
}

// TestCreate_ConcurrentSameIdempotencyKey — ikkita so'rov BIR VAQTDA
// (masalan foydalanuvchi ikki marta tez-tez bossa) xuddi shu kalit bilan
// kelsa: Service.Create()dagi oldindan tekshiruv o'zi atomik emas, shuning
// uchun ikkalasi ham "topilmadi" deb o'tib ketishi mumkin — lekin
// Save()dagi (fakeOCCRepo'da ham, haqiqiy DB'da ham) unique tekshiruv
// buni ushlab qolishi, va yutqazgan g'olibning buyurtmasini qaytarishi
// (dublikat yaratmasligi) kerak.
func TestCreate_ConcurrentSameIdempotencyKey(t *testing.T) {
	repo := newFakeOCCRepo()
	svc := NewService(repo, nil, newCountingIDGen(), nil)

	var wg sync.WaitGroup
	results := make(chan *Order, 2)
	errs := make(chan error, 2)
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			o, err := svc.Create(context.Background(), &Order{
				CustomerID:     "cust1",
				RestaurantID:   "r1",
				Items:          sampleItems(),
				IdempotencyKey: "race-key",
			})
			if err != nil {
				errs <- err
				return
			}
			results <- o
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Fatalf("Create xato qaytarmasligi kerak edi (race avtomatik hal bo'lishi kerak): %v", err)
	}
	ids := map[string]bool{}
	for o := range results {
		ids[o.ID] = true
	}
	if len(ids) != 1 {
		t.Fatalf("ikkalasi ham BIR XIL buyurtma ID'ini qaytarishi kerak edi (dublikat yaratilmasligi), olindi: %v", ids)
	}
	if len(repo.data) != 1 {
		t.Fatalf("repo'da aynan 1 ta buyurtma bo'lishi kerak edi, topildi: %d", len(repo.data))
	}
}

