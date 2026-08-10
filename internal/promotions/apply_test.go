package promotions

import (
	"testing"
	"time"
)

func basePromo(id string, typ Type) *Promotion {
	now := time.Now()
	return &Promotion{
		ID:            id,
		Type:          typ,
		DiscountUnit:  DiscountUnitPercent,
		DiscountValue: 20,
		StartAt:       now.Add(-time.Hour),
		EndAt:         now.Add(time.Hour),
		Active:        true,
	}
}

func TestApplyBestPercentOnOrder(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 2}}
	got := ApplyBest([]*Promotion{p}, lines, 0, time.Now())
	if got == nil || got.DiscountTiyin != 4000 {
		t.Fatalf("expected 4000 discount, got %+v", got)
	}
}

func TestApplyBestFixedAmountOnProduct(t *testing.T) {
	p := basePromo("p1", TypeFixedAmount)
	p.DiscountUnit = DiscountUnitAmount
	p.DiscountValue = 5000
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a"}
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}, {ProductID: "b", UnitPriceTiyin: 10000, Qty: 1}}
	got := ApplyBest([]*Promotion{p}, lines, 0, time.Now())
	if got == nil || got.DiscountTiyin != 5000 {
		t.Fatalf("expected 5000 discount, got %+v", got)
	}
}

func TestApplyBestFixedAmountCappedByEligibleSubtotal(t *testing.T) {
	p := basePromo("p1", TypeFixedAmount)
	p.DiscountUnit = DiscountUnitAmount
	p.DiscountValue = 50000 // more than the product costs
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a"}
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	got := ApplyBest([]*Promotion{p}, lines, 0, time.Now())
	if got == nil || got.DiscountTiyin != 10000 {
		t.Fatalf("expected discount capped at 10000, got %+v", got)
	}
}

// TestApplyBestFixedAmountAppliesPerProduct — foydalanuvchi topgan haqiqiy
// hisoblash xatosi: "5000 so'm chegirma" 2 xil mahsulotga (12000 va
// 15000 so'mlik) tayinlansa, AVVAL faqat BIR marta (5000) chegirilardi —
// HAR IKKALASIGA alohida (5000+5000=10000) chegirilishi kerak edi.
func TestApplyBestFixedAmountAppliesPerProduct(t *testing.T) {
	p := basePromo("p1", TypeFixedAmount)
	p.DiscountUnit = DiscountUnitAmount
	p.DiscountValue = 5000
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a", "b"}
	lines := []CartLine{
		{ProductID: "a", UnitPriceTiyin: 12000, Qty: 1},
		{ProductID: "b", UnitPriceTiyin: 15000, Qty: 1},
	}
	got := ApplyBest([]*Promotion{p}, lines, 0, time.Now())
	if got == nil || got.DiscountTiyin != 10000 {
		t.Fatalf("expected 10000 discount (5000 per product), got %+v", got)
	}
}

// TestApplyBestFixedAmountScalesWithQty — bitta mahsulot 2 dona olinsa,
// chegirma ham 2 baravar bo'lishi kerak (BOGO'dagi kabi miqdorga qarab).
func TestApplyBestFixedAmountScalesWithQty(t *testing.T) {
	p := basePromo("p1", TypeFixedAmount)
	p.DiscountUnit = DiscountUnitAmount
	p.DiscountValue = 5000
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a"}
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 12000, Qty: 2}}
	got := ApplyBest([]*Promotion{p}, lines, 0, time.Now())
	if got == nil || got.DiscountTiyin != 10000 {
		t.Fatalf("expected 10000 discount (5000 x 2 dona), got %+v", got)
	}
}

func TestApplyBestMaxDiscountCap(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	p.DiscountValue = 50
	p.MaxDiscountAmountTiyin = 1000
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	got := ApplyBest([]*Promotion{p}, lines, 0, time.Now())
	if got == nil || got.DiscountTiyin != 1000 {
		t.Fatalf("expected discount capped at 1000, got %+v", got)
	}
}

func TestApplyBestMinOrderAmountGate(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	p.MinOrderAmountTiyin = 50000
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	got := ApplyBest([]*Promotion{p}, lines, 0, time.Now())
	if got != nil {
		t.Fatalf("expected nil (below minimum order amount), got %+v", got)
	}
}

func TestApplyBestBOGO(t *testing.T) {
	p := basePromo("p1", TypeBOGO)
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a"}
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 5000, Qty: 5}} // 2 free units (5/2=2)
	got := ApplyBest([]*Promotion{p}, lines, 0, time.Now())
	if got == nil || got.DiscountTiyin != 10000 {
		t.Fatalf("expected 10000 discount (2 free units), got %+v", got)
	}
}

func TestApplyBestBOGOOddQtyNoFreeUnit(t *testing.T) {
	p := basePromo("p1", TypeBOGO)
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a"}
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 5000, Qty: 1}}
	got := ApplyBest([]*Promotion{p}, lines, 0, time.Now())
	if got != nil {
		t.Fatalf("expected nil (qty=1 gives 0 free units), got %+v", got)
	}
}

func TestApplyBestBundleRequiresAllProducts(t *testing.T) {
	p := basePromo("p1", TypeBundle)
	p.AppliesToProducts = true
	p.TargetProductIDs = []string{"a", "b"}
	// Only "a" present -> no discount.
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	if got := ApplyBest([]*Promotion{p}, lines, 0, time.Now()); got != nil {
		t.Fatalf("expected nil (missing product b), got %+v", got)
	}
	// Both present -> discount over the combined eligible subtotal.
	lines = append(lines, CartLine{ProductID: "b", UnitPriceTiyin: 10000, Qty: 1})
	got := ApplyBest([]*Promotion{p}, lines, 0, time.Now())
	if got == nil || got.DiscountTiyin != 4000 {
		t.Fatalf("expected 4000 discount, got %+v", got)
	}
}

func TestApplyBestLoyaltyGatedByPreviousOrders(t *testing.T) {
	p := basePromo("p1", TypeLoyalty)
	p.AppliesToOrders = true
	p.MinPreviousOrders = 5
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	if got := ApplyBest([]*Promotion{p}, lines, 2, time.Now()); got != nil {
		t.Fatalf("expected nil (only 2 previous orders, need 5), got %+v", got)
	}
	got := ApplyBest([]*Promotion{p}, lines, 5, time.Now())
	if got == nil || got.DiscountTiyin != 2000 {
		t.Fatalf("expected 2000 discount once threshold met, got %+v", got)
	}
}

func TestApplyBestFreeDeliveryNeverApplied(t *testing.T) {
	p := basePromo("p1", TypeFreeDelivery)
	p.AppliesToOrders = true
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	if got := ApplyBest([]*Promotion{p}, lines, 0, time.Now()); got != nil {
		t.Fatalf("expected nil (free_delivery never discounts price), got %+v", got)
	}
}

func TestApplyBestPicksHighestDiscount(t *testing.T) {
	small := basePromo("small", TypePercent)
	small.AppliesToOrders = true
	small.DiscountValue = 10

	big := basePromo("big", TypePercent)
	big.AppliesToOrders = true
	big.DiscountValue = 30

	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	got := ApplyBest([]*Promotion{small, big}, lines, 0, time.Now())
	if got == nil || got.Promotion.ID != "big" {
		t.Fatalf("expected 'big' promotion to win, got %+v", got)
	}
}

func TestApplyBestIgnoresInactive(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	p.Active = false
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	if got := ApplyBest([]*Promotion{p}, lines, 0, time.Now()); got != nil {
		t.Fatalf("expected nil (paused promotion), got %+v", got)
	}
}

func TestApplyBestIgnoresExpired(t *testing.T) {
	p := basePromo("p1", TypePercent)
	p.AppliesToOrders = true
	p.StartAt = time.Now().Add(-48 * time.Hour)
	p.EndAt = time.Now().Add(-24 * time.Hour)
	lines := []CartLine{{ProductID: "a", UnitPriceTiyin: 10000, Qty: 1}}
	if got := ApplyBest([]*Promotion{p}, lines, 0, time.Now()); got != nil {
		t.Fatalf("expected nil (expired promotion), got %+v", got)
	}
}
