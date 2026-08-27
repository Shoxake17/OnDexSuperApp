package payments

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// ═══════════════════════════════════════════════════════════════════
// SOXTA BOG'LIQLIKLAR
// ═══════════════════════════════════════════════════════════════════

type fakeProvider struct {
	mu sync.Mutex

	createErr error
	statusErr error
	status    Status
	statusSum int64

	captured  []int64
	canceled  int
	refunded  int
	createdID string
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) Create(_ context.Context, req CreateRequest) (*CreateResult, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.createdID = req.PaymentID
	return &CreateResult{
		ProviderPaymentID: "prov-" + req.PaymentID,
		PayURL:            "https://pay.example/" + req.PaymentID,
		Status:            StatusPending,
	}, nil
}

func (f *fakeProvider) Status(_ context.Context, _ string) (*StatusResult, error) {
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	return &StatusResult{Status: f.status, PaidAmountTiyin: f.statusSum}, nil
}

func (f *fakeProvider) Capture(_ context.Context, _ string, amount int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.captured = append(f.captured, amount)
	return nil
}

func (f *fakeProvider) Cancel(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.canceled++
	return nil
}

func (f *fakeProvider) Refund(_ context.Context, _, _ string, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refunded++
	return nil
}

// fakeOrders — buyurtma tomonining soxta nusxasi.
type fakeOrders struct {
	amount int64
	held   int
	failed int
}

func (f *fakeOrders) OrderAmountTiyin(context.Context, string) (int64, error) {
	return f.amount, nil
}
func (f *fakeOrders) OnPaymentHeld(context.Context, string) error   { f.held++; return nil }
func (f *fakeOrders) OnPaymentFailed(context.Context, string) error { f.failed++; return nil }

// memRepo — eng sodda ombor (storage paketini import qilmaslik uchun).
type memRepo struct {
	mu    sync.Mutex
	items map[string]Payment
}

func newMemRepo() *memRepo { return &memRepo{items: map[string]Payment{}} }

func (r *memRepo) Create(_ context.Context, p *Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[p.ID] = *p
	return nil
}
func (r *memRepo) Update(_ context.Context, p *Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[p.ID] = *p
	return nil
}
func (r *memRepo) GetByID(_ context.Context, id string) (*Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &p, nil
}
func (r *memRepo) GetByProviderID(_ context.Context, _, pid string) (*Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.items {
		if p.ProviderPaymentID == pid {
			out := p
			return &out, nil
		}
	}
	return nil, ErrNotFound
}
func (r *memRepo) ListByOrder(_ context.Context, orderID string) ([]*Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*Payment
	for _, p := range r.items {
		if p.OrderID == orderID {
			cp := p
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (r *memRepo) ListExpired(_ context.Context, now time.Time, _ int) ([]*Payment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*Payment
	for _, p := range r.items {
		if p.Status == StatusPending && p.ExpiresAt.Before(now) {
			cp := p
			out = append(out, &cp)
		}
	}
	return out, nil
}

func newTestService(t *testing.T, prov *fakeProvider, ord *fakeOrders, opts Options) (*Service, *memRepo) {
	t.Helper()
	repo := newMemRepo()
	if opts.NotifyURL == "" {
		opts.NotifyURL = "https://api.example/callback"
	}
	n := 0
	svc, err := NewService(repo, prov, ord, func() string {
		n++
		return "id-" + string(rune('a'+n-1))
	}, opts)
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo
}

// ═══════════════════════════════════════════════════════════════════
// XAVFSIZLIK: CALLBACK'GA DALILSIZ ISHONILMAYDI
// ═══════════════════════════════════════════════════════════════════

// ★ ENG MUHIM TEST: imzo kaliti yo'q va provayder API'si ham javob
// bermayapti. Bunday holatda "to'landi" degan callback QABUL
// QILINMASLIGI kerak — aks holda soxta so'rov yuborgan odam buyurtmani
// bepul olib ketardi.
func TestCallbackWithoutProofIsNotTrusted(t *testing.T) {
	prov := &fakeProvider{statusErr: errors.New("HTTP 500")}
	ord := &fakeOrders{amount: 1200000}
	svc, repo := newTestService(t, prov, ord, Options{})

	p, err := svc.StartForOrder(context.Background(), "order-1", "cust", "rest", "test", false)
	if err != nil {
		t.Fatal(err)
	}

	err = svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusPaid, AmountTiyin: 1200000,
		SignatureChecked: false, // kalit yo'q
	})
	if err != nil {
		t.Fatalf("xato kutilmagan edi: %v", err)
	}

	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status == StatusPaid {
		t.Error("DALILSIZ callback to'lovni TASDIQLADI — soxta so'rov bilan buyurtma bepul olinardi")
	}
	if !got.NeedsReview {
		t.Error("tasdiqlanmagan to'lov `needs_review` bilan belgilanishi kerak")
	}
	if ord.held != 0 {
		t.Error("buyurtma oshxonaga yuborilmasligi kerak edi")
	}
}

// Imzo tekshirildi va NOTO'G'RI — xabar butunlay rad etiladi.
func TestCallbackWithBadSignatureRejected(t *testing.T) {
	prov := &fakeProvider{status: StatusPaid}
	ord := &fakeOrders{amount: 1000}
	svc, repo := newTestService(t, prov, ord, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)

	err := svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusPaid,
		SignatureChecked: true, SignatureValid: false,
	})
	if err == nil {
		t.Error("noto'g'ri imzo xato qaytarishi kerak edi")
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status == StatusPaid || ord.held != 0 {
		t.Error("noto'g'ri imzoli xabar holatni o'zgartirmasligi kerak")
	}
}

// To'g'ri imzo — tasdiqlanadi (provayder API'si kerak emas).
func TestCallbackWithValidSignatureAccepted(t *testing.T) {
	prov := &fakeProvider{statusErr: errors.New("status API buzuq")}
	ord := &fakeOrders{amount: 1200000}
	svc, repo := newTestService(t, prov, ord, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)

	if err := svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusHeld, AmountTiyin: 1200000,
		SignatureChecked: true, SignatureValid: true,
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status != StatusHeld {
		t.Errorf("holat: %s, kutilgan held", got.Status)
	}
	if ord.held != 1 {
		t.Errorf("buyurtma bir marta oshxonaga yuborilishi kerak edi: %d", ord.held)
	}
}

// Imzo yo'q, lekin provayder API'si tasdiqladi — qabul qilinadi.
// DIQQAT: holat CALLBACK'dan emas, API javobidan olinadi.
func TestCallbackConfirmedByStatusAPI(t *testing.T) {
	prov := &fakeProvider{status: StatusHeld, statusSum: 1200000}
	ord := &fakeOrders{amount: 1200000}
	svc, repo := newTestService(t, prov, ord, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)

	// Callback "to'landi" deydi, API esa "bloklandi" — API ustun.
	if err := svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusPaid, AmountTiyin: 999999999,
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status != StatusHeld {
		t.Errorf("holat: %s, kutilgan held (API javobi ustun bo'lishi kerak)", got.Status)
	}
}

// KAM to'langan buyurtma oshxonaga tushmaydi.
func TestCallbackRejectsUnderpayment(t *testing.T) {
	prov := &fakeProvider{status: StatusHeld, statusSum: 500000}
	ord := &fakeOrders{amount: 1200000}
	svc, repo := newTestService(t, prov, ord, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)

	if err := svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusHeld, AmountTiyin: 500000,
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status == StatusHeld || ord.held != 0 {
		t.Error("kam to'langan buyurtma tasdiqlanmasligi kerak")
	}
	if !got.NeedsReview {
		t.Error("kam to'lov `needs_review` bilan belgilanishi kerak")
	}
}

// ★ IDEMPOTENTLIK: Octo javob olmaguncha callback'ni takrorlaydi
// (jonli sinovda ~50 soniyada bir marta). Buyurtma bir marta
// oshxonaga tushishi kerak.
func TestCallbackIsIdempotent(t *testing.T) {
	prov := &fakeProvider{status: StatusHeld, statusSum: 1000}
	ord := &fakeOrders{amount: 1000}
	svc, _ := newTestService(t, prov, ord, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)

	cb := CallbackData{PaymentID: p.ID, Status: StatusHeld, AmountTiyin: 1000}
	for i := 0; i < 5; i++ {
		if err := svc.HandleCallback(context.Background(), cb); err != nil {
			t.Fatal(err)
		}
	}
	if ord.held != 1 {
		t.Errorf("buyurtma %d marta yuborildi, kutilgan 1", ord.held)
	}
}

// Kechikkan eski xabar yakunlangan to'lovni orqaga qaytarmaydi.
func TestLateCallbackDoesNotRevertFinalStatus(t *testing.T) {
	prov := &fakeProvider{status: StatusPaid, statusSum: 1000}
	ord := &fakeOrders{amount: 1000}
	svc, repo := newTestService(t, prov, ord, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)

	_ = svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusPaid, AmountTiyin: 1000,
		SignatureChecked: true, SignatureValid: true,
	})
	// Endi "bekor qilindi" degan eski xabar keladi.
	_ = svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusCanceled,
		SignatureChecked: true, SignatureValid: true,
	})

	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status != StatusPaid {
		t.Errorf("holat: %s, kutilgan paid (yakuniy holatdan qaytmaydi)", got.Status)
	}
}

// ═══════════════════════════════════════════════════════════════════
// SOZLAMALAR XAVFSIZLIGI
// ═══════════════════════════════════════════════════════════════════

// Dalilsiz ishonish rejimi PRODUCTION'da yoqib bo'lmaydi — server
// ishga tushmaydi (jimgina o'chirib qo'yish yomonroq bo'lardi).
func TestTrustWithoutProofRefusedInProduction(t *testing.T) {
	_, err := NewService(newMemRepo(), &fakeProvider{}, &fakeOrders{},
		func() string { return "id" },
		Options{NotifyURL: "https://x/cb", TrustCallbackWithoutProof: true, Production: true})
	if err == nil {
		t.Error("production'da dalilsiz ishonish rad etilishi kerak edi")
	}
}

func TestNotifyURLRequired(t *testing.T) {
	_, err := NewService(newMemRepo(), &fakeProvider{}, &fakeOrders{},
		func() string { return "id" }, Options{})
	if err == nil {
		t.Error("notify_url'siz xizmat yaratilmasligi kerak")
	}
}

// Dev rejimida dalilsiz qabul qilish MUMKIN (uchdan-uchiga sinash
// uchun) — lekin bu ataylab alohida bayroq bilan yoqiladi.
func TestDevTrustAcceptsCallback(t *testing.T) {
	prov := &fakeProvider{statusErr: errors.New("500")}
	ord := &fakeOrders{amount: 1000}
	svc, repo := newTestService(t, prov, ord, Options{TrustCallbackWithoutProof: true})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)

	if err := svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusHeld, AmountTiyin: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status != StatusHeld || ord.held != 1 {
		t.Errorf("dev rejimda qabul qilinishi kerak edi: %s, held=%d", got.Status, ord.held)
	}
}

// ═══════════════════════════════════════════════════════════════════
// TO'LOV BOSHLASH
// ═══════════════════════════════════════════════════════════════════

// Summa BUYURTMADAN olinadi (klient yuborgan qiymatdan emas).
func TestStartUsesOrderAmount(t *testing.T) {
	prov := &fakeProvider{}
	ord := &fakeOrders{amount: 4321000}
	svc, _ := newTestService(t, prov, ord, Options{})

	p, err := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)
	if err != nil {
		t.Fatal(err)
	}
	if p.AmountTiyin != 4321000 {
		t.Errorf("summa: %d, kutilgan 4321000", p.AmountTiyin)
	}
	if p.PayURL == "" || p.ProviderPaymentID == "" {
		t.Errorf("to'lov havolasi olinmadi: %+v", p)
	}
}

// Takroriy chaqiruv YANGI to'lov yaratmaydi: provayder bir xil
// tranzaksiya ID'sini qayta ishlatishga ruxsat bermaydi.
func TestStartReusesActivePayment(t *testing.T) {
	prov := &fakeProvider{}
	ord := &fakeOrders{amount: 1000}
	svc, _ := newTestService(t, prov, ord, Options{})

	first, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)
	second, err := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Errorf("yangi to'lov yaratildi: %s -> %s", first.ID, second.ID)
	}
}

// "Qayta urinish" YANGI tranzaksiya ochadi. Bank urinishni o'ldirgan
// bo'lishi mumkin (OTP xato kiritilgan / SMS ko'p marta so'ralgan) —
// o'sha havolani qaytarish mijozni tuzoqda qoldirardi.
func TestStartRetryOpensNewPayment(t *testing.T) {
	prov := &fakeProvider{}
	ord := &fakeOrders{amount: 1000}
	svc, repo := newTestService(t, prov, ord, Options{})

	first, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)

	// Urinish "eskirgan" bo'lsin, aks holda spamdan himoya ishlaydi.
	svc.now = func() time.Time { return time.Now().Add(2 * minRetryAge) }

	second, err := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", true)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatalf("yangi urinish ochilmadi: %s", second.ID)
	}
	old, _ := repo.GetByID(context.Background(), first.ID)
	if old.Status != StatusFailed {
		t.Errorf("eski urinish yopilmadi: %s", old.Status)
	}
}

// Tugmani ikki marta bosish Octo'da ikkita tranzaksiya ochmasin.
func TestStartRetryIgnoredForFreshAttempt(t *testing.T) {
	prov := &fakeProvider{}
	ord := &fakeOrders{amount: 1000}
	svc, _ := newTestService(t, prov, ord, Options{})

	first, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)
	second, err := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", true)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Errorf("yangi urinish ochilib ketdi: %s -> %s", first.ID, second.ID)
	}
}

// Mijoz eski (yopilgan) havolani baribir to'lasa, pul provayderda
// bloklanadi — bu xabarni RAD ETIB bo'lmaydi, aks holda pul bloklanib,
// buyurtma esa to'lanmagan bo'lib qolardi.
func TestLateCallbackOnClosedAttemptIsAccepted(t *testing.T) {
	prov := &fakeProvider{status: StatusHeld, statusSum: 1000}
	ord := &fakeOrders{amount: 1000}
	svc, repo := newTestService(t, prov, ord, Options{})

	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)
	svc.now = func() time.Time { return time.Now().Add(2 * minRetryAge) }
	if _, err := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", true); err != nil {
		t.Fatal(err)
	}

	if err := svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusHeld, AmountTiyin: 1000}); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status != StatusHeld {
		t.Errorf("kechikkan to'lov qabul qilinmadi: %s", got.Status)
	}
	if ord.held != 1 {
		t.Errorf("buyurtma bloklangan deb belgilanmadi: %d", ord.held)
	}
}

// Ikkala havola ham to'langan bo'lsa: bittasi yechiladi, ortiqcha blok
// DARHOL bo'shatiladi — mijozning puli osilib qolmasin.
func TestCaptureReleasesExtraHolds(t *testing.T) {
	prov := &fakeProvider{status: StatusHeld, statusSum: 1000}
	ord := &fakeOrders{amount: 1000}
	svc, repo := newTestService(t, prov, ord, Options{})

	first, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)
	svc.now = func() time.Time { return time.Now().Add(2 * minRetryAge) }
	second, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", true)

	for _, id := range []string{first.ID, second.ID} {
		if err := svc.HandleCallback(context.Background(), CallbackData{
			PaymentID: id, Status: StatusHeld, AmountTiyin: 1000}); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.CaptureForOrder(context.Background(), "order-1", 1000); err != nil {
		t.Fatal(err)
	}
	list, _ := repo.ListByOrder(context.Background(), "order-1")
	var paid, canceled int
	for _, p := range list {
		switch p.Status {
		case StatusPaid:
			paid++
		case StatusCanceled:
			canceled++
		}
	}
	if paid != 1 || canceled != 1 {
		t.Errorf("paid=%d canceled=%d, kutilgan 1/1", paid, canceled)
	}
}

// Allaqachon to'langan buyurtmaga ikkinchi marta to'lov boshlanmaydi.
func TestStartRefusesWhenAlreadyPaid(t *testing.T) {
	prov := &fakeProvider{status: StatusHeld, statusSum: 1000}
	ord := &fakeOrders{amount: 1000}
	svc, _ := newTestService(t, prov, ord, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)
	_ = svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusHeld, AmountTiyin: 1000})

	if _, err := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false); !errors.Is(err, ErrAlreadyPaid) {
		t.Errorf("ErrAlreadyPaid kutilgan edi: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════
// CAPTURE / RELEASE / MUDDAT
// ═══════════════════════════════════════════════════════════════════

func TestCaptureAndRelease(t *testing.T) {
	prov := &fakeProvider{status: StatusHeld, statusSum: 1200000}
	ord := &fakeOrders{amount: 1200000}
	svc, _ := newTestService(t, prov, ord, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)
	_ = svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusHeld, AmountTiyin: 1200000})

	if err := svc.CaptureForOrder(context.Background(), "order-1", 1200000); err != nil {
		t.Fatal(err)
	}
	if len(prov.captured) != 1 || prov.captured[0] != 1200000 {
		t.Errorf("yechilgan summa: %v", prov.captured)
	}

	// Endi qaytarish: pul allaqachon yechilgan -> refund.
	if err := svc.ReleaseForOrder(context.Background(), "order-1"); err != nil {
		t.Fatal(err)
	}
	if prov.refunded != 1 {
		t.Errorf("refund chaqirilmadi: %d", prov.refunded)
	}
}

// Bloklangan (yechilmagan) to'lov CANCEL bilan bo'shatiladi — mijozdan
// pul umuman yechilmaydi va qaytarishni kutish kerak emas.
func TestReleaseCancelsHold(t *testing.T) {
	prov := &fakeProvider{status: StatusHeld, statusSum: 1000}
	ord := &fakeOrders{amount: 1000}
	svc, _ := newTestService(t, prov, ord, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)
	_ = svc.HandleCallback(context.Background(), CallbackData{
		PaymentID: p.ID, Status: StatusHeld, AmountTiyin: 1000})

	if err := svc.ReleaseForOrder(context.Background(), "order-1"); err != nil {
		t.Fatal(err)
	}
	if prov.canceled != 1 || prov.refunded != 0 {
		t.Errorf("cancel=%d refund=%d, kutilgan cancel=1 refund=0", prov.canceled, prov.refunded)
	}
}

// Muddati o'tgan to'lov yopiladi va buyurtma bekor qilinadi.
func TestExpireStale(t *testing.T) {
	prov := &fakeProvider{}
	ord := &fakeOrders{amount: 1000}
	svc, repo := newTestService(t, prov, ord, Options{TTLMinutes: 1})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", false)

	// Vaqtni oldinga suramiz.
	svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) }

	n, err := svc.ExpireStale(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("yopilgan to'lovlar: %d, kutilgan 1", n)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status != StatusFailed {
		t.Errorf("holat: %s, kutilgan failed", got.Status)
	}
	if ord.failed != 1 {
		t.Errorf("buyurtma bekor qilinmadi: %d", ord.failed)
	}
}
