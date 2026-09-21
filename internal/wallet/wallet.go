// Package wallet — OnDex Wallet: mijozning platforma ichidagi hisobi.
//
// Reja: F:\ChustApp\ondexwallet.md. Qisqacha qoidalar (foydalanuvchi
// tasdiqlagan, 2026-09-18):
//   - Buyurtma MUVAFFAQIYATLI yetkazilgach (StatusDelivered), agar
//     buyurtma summasi >= 50 000 so'm bo'lsa, 1% keshbek balansga
//     tushadi. To'lov usuli (naqd/karta) farqi yo'q.
//   - Balans >= 20 000 so'm bo'lgandagina sarflab bo'ladi, va FAQAT
//     buyurtmani TO'LIQ qoplasa (aralash/qisman to'lov yo'q) — karta
//     bilan ARALASHTIRILMAYDI.
//
// Ledger — o'zgarmas (append-only) yozuvlar jamlanmasi
// (`wallet_transactions`), balans HAR DOIM shu yozuvlar yig'indisidan
// hisoblanadi (alohida "balans" ustuni yo'q — Postgres implementatsiyasi
// buni foydalanuvchi bo'yicha advisory lock bilan himoya qiladi, bir xil
// naqsh `internal/staff` restoran bo'yicha tartib raqami berishda
// ishlatgan).
package wallet

import (
	"context"
	"errors"
	"time"
)

// Tranzaksiya turlari.
//
// Ochiq (string) qilib qoldirilgan — Go `enum` emas — chunki kelajakda
// fizik karta/kiosk qo'shilsa (`ondexwallet.md`, "Faza 3"), yangi turlar
// (masalan `kiosk_withdrawal`) qattiq ro'yxatni o'zgartirmasdan
// qo'shiladi.
const (
	TypeCashback    = "cashback"     // buyurtma yetkazilgach avtomatik
	TypeSpend       = "spend"        // checkout'da wallet bilan to'lash
	TypeRefund      = "refund"       // spend qilingan buyurtma rad/bekor bo'ldi
	TypeAdminAdjust = "admin_adjust" // qo'lda tuzatish (kelajakda, hozircha ishlatilmaydi)
)

const (
	// CashbackPercent — buyurtma summasidan necha foizi keshbek.
	CashbackPercent = 1
	// CashbackMinOrderTiyin — shu summadan KAM buyurtmaga keshbek
	// berilmaydi (50 000 so'm = 5 000 000 tiyin).
	CashbackMinOrderTiyin int64 = 50_000 * 100
	// SpendMinBalanceTiyin — balans shu chegaradan PAST bo'lsa, wallet
	// checkout'da to'lov usuli sifatida taklif qilinmaydi (20 000 so'm).
	SpendMinBalanceTiyin int64 = 20_000 * 100
)

var (
	// ErrInsufficientBalance — Debit: balans buyurtmani to'liq
	// qoplashga yetmaydi.
	ErrInsufficientBalance = errors.New("wallet: balans yetarli emas")
	// ErrBelowSpendThreshold — balans SpendMinBalanceTiyin dan past,
	// hali umuman sarflab bo'lmaydi (buyurtma arzon bo'lsa ham).
	ErrBelowSpendThreshold = errors.New("wallet: balans hali sarflash chegarasiga (20 000 so'm) yetmagan")
)

// Transaction — ledger'dagi BITTA o'zgarmas yozuv.
type Transaction struct {
	ID      string
	UserID  string
	OrderID string // bo'sh bo'lishi mumkin (masalan admin_adjust)
	// AmountTiyin — HAR DOIM musbat (miqdor). Yo'nalish `Type` va
	// repo metodi (Credit/Debit) orqali belgilanadi — bazada
	// Credit=+AmountTiyin, Debit=-AmountTiyin sifatida saqlanadi.
	AmountTiyin int64
	Type        string
	CreatedAt   time.Time
}

// Repository — ledger saqlash qatlami. Credit/Debit ATOMIK va
// IDEMPOTENT bo'lishi SHART: bir xil (OrderID, Type) juftligi bilan
// ikkinchi chaqiruv balansni ikkinchi marta o'zgartirmaydi
// (`applied=false` qaytadi, xato emas).
type Repository interface {
	// Credit — balansga qo'shadi.
	Credit(ctx context.Context, t *Transaction) (balanceAfter int64, applied bool, err error)
	// Debit — balansdan ayiradi. Yetarli bo'lmasa ErrInsufficientBalance.
	Debit(ctx context.Context, t *Transaction) (balanceAfter int64, applied bool, err error)
	Balance(ctx context.Context, userID string) (int64, error)
	// ListTransactions — yangisi birinchi.
	ListTransactions(ctx context.Context, userID string, limit int) ([]*Transaction, error)
}

// Service — wallet mantig'i.
type Service struct {
	repo  Repository
	idgen func() string
	now   func() time.Time
}

func NewService(repo Repository, idgen func() string) *Service {
	return &Service{repo: repo, idgen: idgen, now: time.Now}
}

func (s *Service) Balance(ctx context.Context, userID string) (int64, error) {
	return s.repo.Balance(ctx, userID)
}

func (s *Service) ListTransactions(ctx context.Context, userID string, limit int) ([]*Transaction, error) {
	return s.repo.ListTransactions(ctx, userID, limit)
}

// CanSpend — checkout'da wallet'ni to'lov usuli sifatida
// ko'rsatish/ko'rsatmaslikni klient (yoki HTTP qatlami) shu bilan
// hal qiladi: balans HAM 20 000 so'm chegarasidan, HAM buyurtma
// summasidan katta-teng bo'lishi kerak.
func (s *Service) CanSpend(ctx context.Context, userID string, orderTotalTiyin int64) (bool, error) {
	balance, err := s.repo.Balance(ctx, userID)
	if err != nil {
		return false, err
	}
	return balance >= SpendMinBalanceTiyin && balance >= orderTotalTiyin, nil
}

// DebitForOrder — checkout: mijoz wallet'ni TO'LIQ to'lov usuli
// sifatida tanladi. `orders.Service.CreateExpecting` chaqiradi,
// buyurtma ALLAQACHON bazaga yozilgandan KEYIN (order_id FK/ledger
// yozuvi mantiqan mavjud buyurtmaga ishora qilishi uchun) — agar bu
// yerda xato qaytsa, chaqiruvchi buyurtmani DARHOL bekor qilishi
// SHART (pul yechilmagan holda "to'langan" buyurtma qolmasligi uchun).
func (s *Service) DebitForOrder(ctx context.Context, orderID, userID string, amountTiyin int64) error {
	if amountTiyin <= 0 {
		return errors.New("wallet: summa musbat bo'lishi kerak")
	}
	balance, err := s.repo.Balance(ctx, userID)
	if err != nil {
		return err
	}
	if balance < SpendMinBalanceTiyin {
		return ErrBelowSpendThreshold
	}
	_, _, err = s.repo.Debit(ctx, &Transaction{
		ID: s.idgen(), UserID: userID, OrderID: orderID,
		AmountTiyin: amountTiyin, Type: TypeSpend, CreatedAt: s.now(),
	})
	return err
}

// RefundForOrder — wallet bilan to'langan buyurtma rad/bekor qilinganda
// (`orders.Service.settleWalletPayment`) yechilgan summani qaytaradi.
func (s *Service) RefundForOrder(ctx context.Context, orderID, userID string, amountTiyin int64) error {
	if amountTiyin <= 0 {
		return nil
	}
	_, _, err := s.repo.Credit(ctx, &Transaction{
		ID: s.idgen(), UserID: userID, OrderID: orderID,
		AmountTiyin: amountTiyin, Type: TypeRefund, CreatedAt: s.now(),
	})
	return err
}

// CreditCashback — buyurtma MUVAFFAQIYATLI yetkazilgach
// (`orders.Service.Transition`, StatusDelivered) chaqiriladi.
//
// Chegaradan past buyurtmada JIM qaytadi (xato EMAS) — chaqiruvchi
// (orders.Service) har buyurtmada shartsiz chaqiradi, "1%" va
// "50 000 so'm" qoidasi FAQAT shu yerda, bitta joyda yashaydi.
func (s *Service) CreditCashback(ctx context.Context, orderID, userID string, orderTotalTiyin int64) error {
	if orderTotalTiyin < CashbackMinOrderTiyin {
		return nil
	}
	amount := orderTotalTiyin * CashbackPercent / 100
	if amount <= 0 {
		return nil
	}
	_, _, err := s.repo.Credit(ctx, &Transaction{
		ID: s.idgen(), UserID: userID, OrderID: orderID,
		AmountTiyin: amount, Type: TypeCashback, CreatedAt: s.now(),
	})
	return err
}
