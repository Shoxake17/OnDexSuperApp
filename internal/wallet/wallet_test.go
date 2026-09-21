package wallet

import (
	"context"
	"errors"
	"testing"
)

// fakeRepo — Service mantig'ini (chegara/foiz qoidalari) repo'dan
// mustaqil sinash uchun soddalashtirilgan xotira ombori.
type fakeRepo struct {
	balances map[string]int64
	seen     map[string]bool // "orderID:type"
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{balances: map[string]int64{}, seen: map[string]bool{}}
}

func (r *fakeRepo) key(orderID, typ string) string { return orderID + ":" + typ }

func (r *fakeRepo) Credit(ctx context.Context, t *Transaction) (int64, bool, error) {
	if t.OrderID != "" && r.seen[r.key(t.OrderID, t.Type)] {
		return r.balances[t.UserID], false, nil
	}
	r.balances[t.UserID] += t.AmountTiyin
	if t.OrderID != "" {
		r.seen[r.key(t.OrderID, t.Type)] = true
	}
	return r.balances[t.UserID], true, nil
}

func (r *fakeRepo) Debit(ctx context.Context, t *Transaction) (int64, bool, error) {
	if t.OrderID != "" && r.seen[r.key(t.OrderID, t.Type)] {
		return r.balances[t.UserID], false, nil
	}
	if r.balances[t.UserID] < t.AmountTiyin {
		return r.balances[t.UserID], false, ErrInsufficientBalance
	}
	r.balances[t.UserID] -= t.AmountTiyin
	if t.OrderID != "" {
		r.seen[r.key(t.OrderID, t.Type)] = true
	}
	return r.balances[t.UserID], true, nil
}

func (r *fakeRepo) Balance(ctx context.Context, userID string) (int64, error) {
	return r.balances[userID], nil
}

func (r *fakeRepo) ListTransactions(ctx context.Context, userID string, limit int) ([]*Transaction, error) {
	return nil, nil
}

func newTestService(repo Repository) *Service {
	n := 0
	return NewService(repo, func() string {
		n++
		return "id" + string(rune('0'+n))
	})
}

func TestCreditCashback_BelowThreshold_NoOp(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()

	// 49 999 so'm — chegaradan (50 000) PAST, keshbek berilmaydi.
	err := svc.CreditCashback(ctx, "order1", "u1", 49_999*100)
	if err != nil {
		t.Fatalf("chegara ostida xatosiz jim o'tishi kerak: %v", err)
	}
	if b, _ := repo.Balance(ctx, "u1"); b != 0 {
		t.Fatalf("keshbek berilmasligi kerak edi, balans=%d", b)
	}
}

func TestCreditCashback_AtThreshold_OnePercent(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()

	// Aynan 50 000 so'm — chegara O'ZI ham hisobga kiradi ("kam bo'lsa
	// berilmaydi" — tengi kiradi).
	total := int64(50_000 * 100)
	if err := svc.CreditCashback(ctx, "order1", "u1", total); err != nil {
		t.Fatalf("CreditCashback: %v", err)
	}
	want := total / 100 // 1%
	if b, _ := repo.Balance(ctx, "u1"); b != want {
		t.Fatalf("keshbek = %d kutilgan edi, balans=%d", want, b)
	}

	// Idempotentlik: bir xil buyurtma uchun ikkinchi marta keshbek
	// qo'shilmasin (masalan Transition ikki marta chaqirilsa).
	if err := svc.CreditCashback(ctx, "order1", "u1", total); err != nil {
		t.Fatalf("CreditCashback (takror): %v", err)
	}
	if b, _ := repo.Balance(ctx, "u1"); b != want {
		t.Fatalf("takroriy chaqiruv balansni ikkilantirmasligi kerak: %d", b)
	}
}

func TestCreditCashback_HundredThousand(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()

	total := int64(100_000 * 100) // 100 000 so'm
	if err := svc.CreditCashback(ctx, "order1", "u1", total); err != nil {
		t.Fatalf("CreditCashback: %v", err)
	}
	want := int64(1_000 * 100) // 1000 so'm
	if b, _ := repo.Balance(ctx, "u1"); b != want {
		t.Fatalf("keshbek = %d kutilgan edi, balans=%d", want, b)
	}
}

func TestDebitForOrder_BelowSpendThreshold(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()
	repo.balances["u1"] = 15_000 * 100 // 20 000 dan PAST

	err := svc.DebitForOrder(ctx, "order1", "u1", 10_000*100)
	if !errors.Is(err, ErrBelowSpendThreshold) {
		t.Fatalf("ErrBelowSpendThreshold kutilgan edi, oldi: %v", err)
	}
	if b, _ := repo.Balance(ctx, "u1"); b != 15_000*100 {
		t.Fatalf("muvaffaqiyatsiz debit'dan keyin balans o'zgarmasligi kerak: %d", b)
	}
}

func TestDebitForOrder_InsufficientForOrder(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()
	// Balans 20 000 chegarasidan yuqori, lekin BUYURTMANI to'liq
	// qoplamaydi (25 000 < 30 000).
	repo.balances["u1"] = 25_000 * 100

	err := svc.DebitForOrder(ctx, "order1", "u1", 30_000*100)
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("ErrInsufficientBalance kutilgan edi, oldi: %v", err)
	}
}

func TestDebitForOrder_FullyCovered(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()
	repo.balances["u1"] = 25_000 * 100

	if err := svc.DebitForOrder(ctx, "order1", "u1", 20_000*100); err != nil {
		t.Fatalf("DebitForOrder: %v", err)
	}
	if b, _ := repo.Balance(ctx, "u1"); b != 5_000*100 {
		t.Fatalf("balans 5 000 so'm qolishi kerak edi: %d", b)
	}
}

func TestCanSpend(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()

	repo.balances["u1"] = 15_000 * 100
	if ok, _ := svc.CanSpend(ctx, "u1", 10_000*100); ok {
		t.Fatal("20 000 chegarasidan past balans bilan sarflab bo'lmasligi kerak")
	}

	repo.balances["u1"] = 25_000 * 100
	if ok, _ := svc.CanSpend(ctx, "u1", 30_000*100); ok {
		t.Fatal("buyurtmani to'liq qoplamasa (25 000 < 30 000) sarflab bo'lmasligi kerak")
	}
	if ok, _ := svc.CanSpend(ctx, "u1", 20_000*100); !ok {
		t.Fatal("balans yetarli va chegaradan yuqori bo'lsa sarflab bo'lishi kerak")
	}
}

func TestRefundForOrder(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	ctx := context.Background()

	if err := svc.RefundForOrder(ctx, "order1", "u1", 10_000*100); err != nil {
		t.Fatalf("RefundForOrder: %v", err)
	}
	if b, _ := repo.Balance(ctx, "u1"); b != 10_000*100 {
		t.Fatalf("qaytarilgan summa balansga qo'shilishi kerak: %d", b)
	}
}
