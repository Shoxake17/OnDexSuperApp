package payments

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Muddati o'tgan to'lovni yopishdan OLDIN provayderdan so'raymiz: callback o'tib ketgan
// bo'lsa (server o'chiq, imzo xatosi, tarmoq), mijozdan pul YECHILGAN, buyurtma esa
// bekor bo'lardi va qaytarish ham bo'lmasdi.
func TestExpireStaleConfirmsPaymentPaidAtProvider(t *testing.T) {
	prov := &fakeProvider{status: StatusPaid, statusSum: 1000}
	ord := &fakeOrders{amount: 1000}
	svc, repo := newTestService(t, prov, ord, Options{TTLMinutes: 1})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", "", false)
	svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) }

	if _, err := svc.ExpireStale(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status != StatusPaid {
		t.Errorf("provayderda to'langan to'lov tasdiqlanmadi: %s", got.Status)
	}
	if ord.failed != 0 {
		t.Errorf("to'langan to'lov uchun buyurtma bekor qilindi: failed=%d", ord.failed)
	}
}

// Provayder javob bermasa pul holati NOMA'LUM: hozircha yopmaymiz, keyingi aylanishda qayta.
func TestExpireStaleKeepsPaymentWhenProviderUnreachable(t *testing.T) {
	prov := &fakeProvider{statusErr: errors.New("shlyuz ishlamayapti")}
	ord := &fakeOrders{amount: 1000}
	svc, repo := newTestService(t, prov, ord, Options{TTLMinutes: 1})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", "", false)
	svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) }

	n, err := svc.ExpireStale(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if n != 0 || got.Status != StatusPending {
		t.Errorf("javobsiz provayderda to'lov yopildi: n=%d holat=%s", n, got.Status)
	}
	if ord.failed != 0 {
		t.Errorf("buyurtma bekor qilindi: failed=%d", ord.failed)
	}
	// Provayder tiklangach keyingi aylanishda yopiladi.
	prov.statusErr = nil
	if n, _ = svc.ExpireStale(context.Background(), 10); n != 1 {
		t.Errorf("provayder tiklangach to'lov yopilmadi: n=%d", n)
	}
}

// Provayder uuid kutsa (Multicard) holat shu bilan so'raladi; bo'lmasa (Octo) bizning ID bilan.
func TestStatusRefMatchesProviderContract(t *testing.T) {
	for name, tc := range map[string]struct {
		byProviderID bool
		wantUUID     bool
	}{"multicard (uuid)": {true, true}, "octo (bizning ID)": {false, false}} {
		t.Run(name, func(t *testing.T) {
			prov := &fakeProvider{status: StatusPaid, statusSum: 1000, byProviderID: tc.byProviderID}
			svc, _ := newTestService(t, prov, &fakeOrders{amount: 1000}, Options{TTLMinutes: 1})
			p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", "", false)
			svc.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
			if _, err := svc.ExpireStale(context.Background(), 10); err != nil {
				t.Fatal(err)
			}
			want := p.ID
			if tc.wantUUID {
				want = p.ProviderPaymentID
			}
			if prov.lastStatusRef != want {
				t.Errorf("Status'ga %q berildi, kutilgan %q", prov.lastStatusRef, want)
			}
		})
	}
}
