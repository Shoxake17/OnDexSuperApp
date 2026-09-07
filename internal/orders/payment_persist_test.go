package orders

import (
	"context"
	"testing"
)

// To'lov holatining SAQLANISHI (bug.md 35-band).
//
// ┌─ NEGA BU ENG XAVFLI HOLAT ─────────────────────────────────────────┐
// `CaptureForOrder` muvaffaqiyatli bo'ldi — PUL PROVAYDERDA YECHILDI.
// Shundan keyin `Save` yiqilsa, bazada `PaymentState` `held` bo'lib
// qolardi va xato faqat logga yozilardi.
//
// Oqibati: buyurtma keyin bekor qilinsa, `ReleaseForOrder`
// `StatusPaid` emas, `StatusHeld` yo'lidan borib ALLAQACHON YECHILGAN
// to'lovni "bo'shatishga" urinardi — ya'ni mijozning puli qaytmasdi.
// └────────────────────────────────────────────────────────────────────┘

// conflictOnceRepo — birinchi `Save` da `ErrConflict`, keyin muvaffaqiyat.
// Optimistik qulf ostidagi haqiqiy holatni taqlid qiladi: boshqa
// so'rov bizdan oldin yozib ulgurgan.
type conflictOnceRepo struct {
	*fakeOCCRepo
	saveCalls int
	stored    *Order
}

func (r *conflictOnceRepo) Save(ctx context.Context, o *Order) error {
	r.saveCalls++
	if r.saveCalls == 1 {
		return ErrConflict
	}
	cp := *o
	r.stored = &cp
	return nil
}

func (r *conflictOnceRepo) GetByID(_ context.Context, id string) (*Order, error) {
	// "Yangi" versiya — boshqa so'rov yozgan holat.
	return &Order{
		ID:            id,
		PaymentMethod: PaymentCard,
		PaymentState:  PaymentHeld,
		Version:       2,
	}, nil
}

// ASOSIY REGRESSIYA: konfliktda holat QAYTA urinish bilan saqlanadi.
func TestPersistPaymentStateRetriesOnConflict(t *testing.T) {
	repo := &conflictOnceRepo{fakeOCCRepo: newFakeOCCRepo()}
	svc := NewService(repo, nil, func() string { return "id" }, nil)

	o := &Order{ID: "o1", PaymentMethod: PaymentCard, PaymentState: PaymentHeld}
	svc.persistPaymentState(context.Background(), o, PaymentPaid)

	if repo.saveCalls < 2 {
		t.Fatalf("konfliktda qayta urinilmadi: Save %d marta chaqirildi", repo.saveCalls)
	}
	if repo.stored == nil {
		t.Fatal("holat UMUMAN saqlanmadi — pul yechilgan, baza bilmaydi")
	}
	if repo.stored.PaymentState != PaymentPaid {
		t.Fatalf("saqlangan holat noto'g'ri: %q (kutilgan %q)",
			repo.stored.PaymentState, PaymentPaid)
	}
	// Qayta o'qilgan versiya ishlatilishi kerak — aks holda keyingi
	// yozuv ham konflikt berardi.
	if repo.stored.Version != 2 {
		t.Errorf("yangi versiya olinmadi: %d", repo.stored.Version)
	}
}

// Konflikt BO'LMAGANDA bitta yozuv yetarli — keraksiz qayta o'qish
// bo'lmasin.
func TestPersistPaymentStateSavesOnceWhenNoConflict(t *testing.T) {
	repo := &okRepo{fakeOCCRepo: newFakeOCCRepo()}
	svc := NewService(repo, nil, func() string { return "id" }, nil)

	o := &Order{ID: "o1", PaymentMethod: PaymentCard, PaymentState: PaymentHeld}
	svc.persistPaymentState(context.Background(), o, PaymentPaid)

	if repo.saveCalls != 1 {
		t.Fatalf("Save %d marta chaqirildi (1 kutilgan)", repo.saveCalls)
	}
	if repo.stored.PaymentState != PaymentPaid {
		t.Fatalf("holat saqlanmadi: %q", repo.stored.PaymentState)
	}
}

type okRepo struct {
	*fakeOCCRepo
	saveCalls int
	stored    *Order
}

func (r *okRepo) Save(_ context.Context, o *Order) error {
	r.saveCalls++
	cp := *o
	r.stored = &cp
	return nil
}

// Bekor qilingan kontekst holatni saqlashga TO'SIQ bo'lmasligi kerak:
// pul allaqachon harakatlangan, HTTP so'rovi esa tugagan bo'lishi
// mumkin.
func TestPersistPaymentStateWorksWithCancelledContext(t *testing.T) {
	repo := &okRepo{fakeOCCRepo: newFakeOCCRepo()}
	svc := NewService(repo, nil, func() string { return "id" }, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // so'rov tugadi

	o := &Order{ID: "o1", PaymentMethod: PaymentCard, PaymentState: PaymentHeld}
	svc.persistPaymentState(ctx, o, PaymentPaid)

	if repo.stored == nil || repo.stored.PaymentState != PaymentPaid {
		t.Fatal("bekor qilingan kontekstda holat saqlanmadi — pul yechilgan, baza bilmaydi")
	}
}
