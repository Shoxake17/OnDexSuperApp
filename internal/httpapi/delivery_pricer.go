package httpapi

import (
	"context"

	"chustapp/internal/catalog"
)

// catalogDeliveryPricer — `orders.DeliveryPricer` ni katalog ustiga
// bog'laydi.
//
// ┌─ NEGA KO'PRIK KERAK ──────────────────────────────────────────────┐
// `orders` paketi `catalog` ni import qilmaydi (buyurtma xizmati menyu
// tuzilmasini bilishi shart emas) va shunday qolishi kerak. Shu sabab
// bog'lash HTTP qatlamida — ikkala paketni ham shu yer ko'radi.
// └───────────────────────────────────────────────────────────────────┘
//
// Qoidaning O'ZI bu yerda EMAS, `catalog.EffectiveDeliveryFee` da:
// bepul chegarasi, stol buyurtmasi va 0 = bepul — hammasi bitta
// joyda. Bu tur faqat restoranni topib beradi.
type catalogDeliveryPricer struct {
	repo catalog.Repository
}

// NewDeliveryPricer — `orders.Service.WithDelivery` uchun.
func NewDeliveryPricer(repo catalog.Repository) catalogDeliveryPricer {
	return catalogDeliveryPricer{repo: repo}
}

// DeliveryFee — `orders.DeliveryPricer`.
//
// Restoran topilmasa haq 0 EMAS, XATO qaytadi: topilmagan restoran
// uchun buyurtma baribir yaratilmaydi, lekin bu yerda jimgina 0
// qaytarilsa sabab yashirinib, keyin "nega haq olinmadi?" degan
// tushunarsiz holat chiqardi.
func (p catalogDeliveryPricer) DeliveryFee(ctx context.Context, restaurantID string, orderTotalTiyin int64, dineIn bool) (int64, error) {
	rest, err := p.repo.GetRestaurant(ctx, restaurantID)
	if err != nil {
		return 0, err
	}
	return rest.EffectiveDeliveryFee(orderTotalTiyin, dineIn), nil
}
