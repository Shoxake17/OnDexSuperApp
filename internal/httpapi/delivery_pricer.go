package httpapi

import (
	"context"

	"chustapp/internal/staff"
)

// staffDeliveryPricer — `orders.DeliveryPricer` ni xodimlar ustiga
// bog'laydi.
//
// ┌─ NEGA KO'PRIK KERAK ──────────────────────────────────────────────┐
// `orders` paketi `staff` ni import qilmaydi (buyurtma xizmati xodimlar
// tuzilmasini bilishi shart emas) va shunday qolishi kerak. Shu sabab
// bog'lash HTTP qatlamida — ikkala paketni ham shu yer ko'radi.
// └───────────────────────────────────────────────────────────────────┘
//
// Qoidaning O'ZI bu yerda EMAS, `staff.DeliveryFee` da: "buyurtmadan
// maosh" bo'lsa shu summa, oylik maosh bo'lsa BEPUL. Bu tur faqat
// restoranning xodimlarini topib beradi va stol buyurtmasini ajratadi.
type staffDeliveryPricer struct {
	repo staff.Repository
}

// NewDeliveryPricer — `orders.Service.WithDelivery` uchun.
func NewDeliveryPricer(repo staff.Repository) staffDeliveryPricer {
	return staffDeliveryPricer{repo: repo}
}

// DeliveryFee — `orders.DeliveryPricer`.
//
// STOL buyurtmasida haq har doim 0: ofitsiant stolga olib boradi, kuryer
// ishtirok etmaydi. Bu tekshiruv xodimlar ro'yxatiga murojaatdan OLDIN —
// stol buyurtmasi uchun ortiqcha so'rov ketmasin.
//
// Ro'yxatni o'qib bo'lmasa haq 0 EMAS, XATO qaytadi: jimgina "bepul"
// deb hisoblansa mijoz bir narxni ko'rib, boshqasini to'lashi (yoki
// restoran haqsiz yetkazib berishi) mumkin edi.
func (p staffDeliveryPricer) DeliveryFee(ctx context.Context, restaurantID string, _ int64, dineIn bool) (int64, error) {
	if dineIn {
		return 0, nil
	}
	members, err := p.repo.List(ctx, restaurantID)
	if err != nil {
		return 0, err
	}
	return staff.DeliveryFee(members), nil
}
