package payments

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Buyurtma bekor qilinganda TO'LANMAGAN invoys provayderda yopilishi shart:
// aks holda mijoz havolani keyin to'lasa, pul yechiladi, buyurtma esa yo'q.
// (Multicard sandbox'dagi haqiqiy sinov bu bo'shliqni ochgan: bekor qilish
// endpointi noto'g'ri edi va to'lanmagan invoys hech qachon yopilmasdi.)
func TestReleaseClosesUnpaidInvoiceWhenProviderSupportsIt(t *testing.T) {
	prov := &fakeProvider{unpaid: true}
	svc, repo := newTestService(t, prov, &fakeOrders{amount: 1000}, Options{})
	p, err := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ReleaseForOrder(context.Background(), "order-1"); err != nil {
		t.Fatal(err)
	}
	if prov.canceled != 1 {
		t.Errorf("provayderda yopilmadi: cancel=%d", prov.canceled)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status != StatusCanceled {
		t.Errorf("holat: %s, kutilgan canceled", got.Status)
	}
}

// Belgi bo'lmasa (Octo) kutilayotgan to'lovga TEGILMAYDI: xatti-harakat o'zgarmaydi.
func TestReleaseLeavesPendingAloneWithoutSupport(t *testing.T) {
	prov := &fakeProvider{} // unpaid=false
	svc, repo := newTestService(t, prov, &fakeOrders{amount: 1000}, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", "", false)
	if err := svc.ReleaseForOrder(context.Background(), "order-1"); err != nil {
		t.Fatal(err)
	}
	if prov.canceled != 0 {
		t.Errorf("qo'llamaydigan provayderda cancel chaqirildi: %d", prov.canceled)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status != StatusPending {
		t.Errorf("holat: %s, kutilgan pending", got.Status)
	}
}

// Yopib bo'lmasa (masalan invoys shu daqiqada to'lanib bo'lgan) buyurtmani
// bekor qilish TO'XTAMAYDI va holat o'zgarmaydi: to'lov xabari keladi.
func TestReleaseUnpaidCancelFailureDoesNotBlock(t *testing.T) {
	prov := &fakeProvider{unpaid: true, cancelErr: errors.New("multicard: invoys allaqachon to'langan")}
	svc, repo := newTestService(t, prov, &fakeOrders{amount: 1000}, Options{})
	p, _ := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", "", false)
	if err := svc.ReleaseForOrder(context.Background(), "order-1"); err != nil {
		t.Fatalf("yopish xatosi bekor qilishni to'xtatdi: %v", err)
	}
	got, _ := repo.GetByID(context.Background(), p.ID)
	if got.Status != StatusPending {
		t.Errorf("holat: %s, kutilgan pending (to'lov xabari keladi)", got.Status)
	}
}

// Mijoz qayta urinsa eski invoys ham yopiladi: ikkalasini to'lab, pul ikki
// marta yechilmasin.
func TestRetryClosesOldInvoiceAtProvider(t *testing.T) {
	for name, tc := range map[string]struct {
		unpaid bool
		want   int
	}{"multicard": {true, 1}, "octo": {false, 0}} {
		t.Run(name, func(t *testing.T) {
			prov := &fakeProvider{unpaid: tc.unpaid}
			svc, _ := newTestService(t, prov, &fakeOrders{amount: 1000}, Options{})
			_, _ = svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", "", false)
			svc.now = func() time.Time { return time.Now().Add(2 * minRetryAge) }
			if _, err := svc.StartForOrder(context.Background(), "order-1", "c", "r", "d", "", true); err != nil {
				t.Fatal(err)
			}
			if prov.canceled != tc.want {
				t.Errorf("eski invoys uchun cancel=%d, kutilgan %d", prov.canceled, tc.want)
			}
		})
	}
}
