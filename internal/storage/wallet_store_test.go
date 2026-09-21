package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/wallet"
)

func TestWalletRepoMemory(t *testing.T) {
	runWalletRepoSuite(t, NewMemoryWalletRepo(), "wm_user_"+fmt.Sprint(time.Now().UnixNano()))
}

// runWalletRepoSuite — Xotira va Postgres implementatsiyalari BIR XIL
// javob berishi kerak (`courier_geo_test.go`dagi bir xil falsafa).
func runWalletRepoSuite(t *testing.T, repo wallet.Repository, userID string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now()

	// Boshlang'ich balans — 0.
	if b, err := repo.Balance(ctx, userID); err != nil || b != 0 {
		t.Fatalf("boshlang'ich balans: %d, %v", b, err)
	}

	// Keshbek: 1500 tiyin kredit.
	bal, applied, err := repo.Credit(ctx, &wallet.Transaction{
		ID: "tx1", UserID: userID, OrderID: "order1", AmountTiyin: 1500,
		Type: wallet.TypeCashback, CreatedAt: now,
	})
	if err != nil || !applied || bal != 1500 {
		t.Fatalf("Credit: bal=%d applied=%v err=%v", bal, applied, err)
	}

	// Idempotentlik: xuddi shu (order_id, type) qayta kredit qo'shmaydi.
	bal, applied, err = repo.Credit(ctx, &wallet.Transaction{
		ID: "tx1-retry", UserID: userID, OrderID: "order1", AmountTiyin: 1500,
		Type: wallet.TypeCashback, CreatedAt: now,
	})
	if err != nil || applied || bal != 1500 {
		t.Fatalf("Credit idempotent bo'lishi kerak edi: bal=%d applied=%v err=%v", bal, applied, err)
	}

	// Yana bitta buyurtmadan keshbek — jamlanadi.
	bal, applied, err = repo.Credit(ctx, &wallet.Transaction{
		ID: "tx2", UserID: userID, OrderID: "order2", AmountTiyin: 500,
		Type: wallet.TypeCashback, CreatedAt: now,
	})
	if err != nil || !applied || bal != 2000 {
		t.Fatalf("ikkinchi Credit: bal=%d applied=%v err=%v", bal, applied, err)
	}

	// Debit: balansdan katta summa — rad etiladi, balans o'zgarmaydi.
	_, applied, err = repo.Debit(ctx, &wallet.Transaction{
		ID: "tx3", UserID: userID, OrderID: "order3", AmountTiyin: 5000,
		Type: wallet.TypeSpend, CreatedAt: now,
	})
	if !errors.Is(err, wallet.ErrInsufficientBalance) || applied {
		t.Fatalf("yetarsiz balansda ErrInsufficientBalance kutilgan edi: applied=%v err=%v", applied, err)
	}
	if b, _ := repo.Balance(ctx, userID); b != 2000 {
		t.Fatalf("muvaffaqiyatsiz debit'dan keyin balans o'zgarmasligi kerak: %d", b)
	}

	// Debit: yetarli summa.
	bal, applied, err = repo.Debit(ctx, &wallet.Transaction{
		ID: "tx4", UserID: userID, OrderID: "order4", AmountTiyin: 800,
		Type: wallet.TypeSpend, CreatedAt: now,
	})
	if err != nil || !applied || bal != 1200 {
		t.Fatalf("Debit: bal=%d applied=%v err=%v", bal, applied, err)
	}

	// Debit idempotentligi.
	bal, applied, err = repo.Debit(ctx, &wallet.Transaction{
		ID: "tx4-retry", UserID: userID, OrderID: "order4", AmountTiyin: 800,
		Type: wallet.TypeSpend, CreatedAt: now,
	})
	if err != nil || applied || bal != 1200 {
		t.Fatalf("Debit idempotent bo'lishi kerak edi: bal=%d applied=%v err=%v", bal, applied, err)
	}

	// Refund (Credit turi boshqa) — bir xil order_id, boshqa type — bloklanmaydi.
	bal, applied, err = repo.Credit(ctx, &wallet.Transaction{
		ID: "tx5", UserID: userID, OrderID: "order4", AmountTiyin: 800,
		Type: wallet.TypeRefund, CreatedAt: now,
	})
	if err != nil || !applied || bal != 2000 {
		t.Fatalf("Refund (boshqa type): bal=%d applied=%v err=%v", bal, applied, err)
	}

	list, err := repo.ListTransactions(ctx, userID, 10)
	if err != nil {
		t.Fatalf("ListTransactions: %v", err)
	}
	// Faqat MUVAFFAQIYATLI amallar yozuv qoldiradi: tx1, tx2, tx4, tx5
	// (tx1-retry va tx4-retry idempotentlik tufayli, tx3 yetarsiz
	// balans tufayli — yozuv QOLDIRMAYDI).
	if len(list) != 4 {
		t.Fatalf("4 ta yozuv kutilgan edi, %d keldi", len(list))
	}
	for _, tr := range list {
		if tr.AmountTiyin <= 0 {
			t.Fatalf("ListTransactions har doim MUSBAT miqdor qaytarishi kerak (yo'nalish Type'dan bilinadi): %+v", tr)
		}
	}

	// Boshqa mijoz — balans mustaqil.
	if b, err := repo.Balance(ctx, userID+"_other"); err != nil || b != 0 {
		t.Fatalf("boshqa mijoz balansi mustaqil bo'lishi kerak: %d, %v", b, err)
	}
}

func TestWalletRepoPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("CI'da TEST_DATABASE_URL BO'LISHI SHART")
		}
		t.Skip("TEST_DATABASE_URL berilmagan — Postgres testi o'tkazib yuborildi")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	userID := fmt.Sprintf("wallettest_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx,
		`INSERT INTO users (id, phone) VALUES ($1, $2)`, userID, "+998"+userID[len(userID)-9:]); err != nil {
		t.Fatalf("test foydalanuvchi yaratilmadi: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	runWalletRepoSuite(t, NewPostgresWalletRepo(pool), userID)
}
