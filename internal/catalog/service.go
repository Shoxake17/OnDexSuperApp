package catalog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"chustapp/internal/orders"
)

// ItemRequest — mijoz yuboradigan yagona narsa: qaysi taomdan nechta.
// Narx va nom har doim serverdagi katalogdan olinadi.
type ItemRequest struct {
	ProductID string `json:"product_id"`
	Qty       int    `json:"qty"`
}

type Service struct {
	repo Repository
	// now — soat; testlarda almashtiriladi.
	now func() time.Time
}

func NewService(repo Repository) *Service { return &Service{repo: repo, now: time.Now} }

// CheckPayment — restoran shu to'lov usulini qabul qiladimi. Buyurtma
// yaratiladigan HAR BIR yo'lda chaqiriladi: sozlama faqat panelda
// ko'rinib, serverda tekshirilmasa, u bezak bo'lib qolardi.
func (s *Service) CheckPayment(ctx context.Context, restaurantID string, m orders.PaymentMethod) error {
	rest, err := s.repo.GetRestaurant(ctx, restaurantID)
	if err != nil {
		return err
	}
	if rest.AcceptsPayment(m) {
		return nil
	}
	if m == orders.PaymentCard {
		return fmt.Errorf("%w: onlayn karta to'lovi o'chirilgan — joyida to'lang", ErrPaymentNotAccepted)
	}
	return fmt.Errorf("%w: faqat onlayn karta orqali oldindan to'lanadi", ErrPaymentNotAccepted)
}

// PriceOrder — buyurtma tarkibini katalog bo'yicha tekshiradi va narxlaydi.
// Qoidalar: hamma taom mavjud va bitta restoranniki, restoran ochiq bo'lishi kerak.
func (s *Service) PriceOrder(ctx context.Context, reqs []ItemRequest) (restaurantID string, items []orders.Item, err error) {
	if len(reqs) == 0 {
		return "", nil, errors.New("buyurtma bo'sh")
	}
	// Yuqori chegara — miqdor (`Qty`) allaqachon cheklangan edi, lekin
	// TURLAR soni emas. Chegarasiz massiv katta DB so'roviga, ortiqcha
	// xotiraga va nihoyat `subtotal` (int64) toshib ketishiga olib
	// kelishi mumkin edi. Haqiqiy savat uchun 100 tur mo'l-ko'l.
	const maxDistinctItems = 100
	if len(reqs) > maxDistinctItems {
		return "", nil, fmt.Errorf("savatda juda ko'p tur mahsulot (maksimal %d)", maxDistinctItems)
	}
	ids := make([]string, 0, len(reqs))
	for _, r := range reqs {
		if r.Qty < 1 || r.Qty > 100 {
			return "", nil, fmt.Errorf("miqdor noto'g'ri: %s x%d", r.ProductID, r.Qty)
		}
		ids = append(ids, r.ProductID)
	}
	products, err := s.repo.GetProductsByIDs(ctx, ids)
	if err != nil {
		return "", nil, err
	}
	byID := make(map[string]*Product, len(products))
	for _, p := range products {
		byID[p.ID] = p
	}

	for _, r := range reqs {
		p, ok := byID[r.ProductID]
		if !ok {
			return "", nil, fmt.Errorf("taom topilmadi: %s", r.ProductID)
		}
		if !p.Available {
			return "", nil, fmt.Errorf("%w: %s", ErrUnavailable, p.Name)
		}
		if restaurantID == "" {
			restaurantID = p.RestaurantID
		} else if restaurantID != p.RestaurantID {
			return "", nil, ErrMixedRestaurants
		}
		// `PriceTiyin` — HAR DOIM mahsulotning ASL (ro'yxat) narxi.
		// Mahsulotning o'z chegirma narxi ALOHIDA maydonda uzatiladi.
		//
		// NEGA SHUNDAY (tuzatilgan moliyaviy bug): avval bu yerga
		// to'g'ridan-to'g'ri chegirma narxi qo'yilardi. Natijada narxlash
		// qatlami (`orders.priceCart`) allaqachon chegirmali narx ustiga
		// YANA aksiya chegirmasini qo'shardi — ikkita chegirma stack
		// bo'lib, jami NOLGA tushib ketardi (haqiqiy holat: 105 850 so'm
		// savat -> 0 so'm). Endi ikkalasi ham xom holda uzatiladi va
		// narxlash qatlami ORASIDAN ENG YAXSHISINI tanlaydi, hech qachon
		// qo'shmaydi.
		//
		// Qo'shimcha foyda: `SubtotalTiyin` endi har doim ro'yxat
		// narxida hisoblanadi, ya'ni klient ko'rsatadigan "chizilgan"
		// summa bilan server summasi bir xil bo'ladi (avval ular
		// farq qilardi va chek chalkash ko'rinardi).
		discountPrice := int64(0)
		if p.DiscountPriceTiyin > 0 && p.DiscountPriceTiyin < p.PriceTiyin {
			discountPrice = p.DiscountPriceTiyin
		}
		items = append(items, orders.Item{
			ProductID:          p.ID,
			Name:               p.Name,
			Qty:                r.Qty,
			PriceTiyin:         p.PriceTiyin,
			DiscountPriceTiyin: discountPrice,
			ImageURL:           p.ImageURL,
			Category:           p.Category,
			// 3D model — AR ko'rinishi uchun (buyurtma vaqtidagi
			// nusxa, `Item.Model3DURL` izohiga qarang).
			Model3DURL: p.Model3DURL,
		})
	}

	rest, err := s.repo.GetRestaurant(ctx, restaurantID)
	if err != nil {
		return "", nil, err
	}
	if !rest.Open {
		return "", nil, ErrRestaurantClosed
	}
	// Ish vaqti — HAMMA buyurtma yo'li (ilova, stol QR, agent, yordamchi)
	// shu funksiyadan o'tadi, shuning uchun tekshiruv bitta joyda.
	if !rest.WorkingHours.IsOpenAt(s.now()) {
		return "", nil, ErrOutsideHours
	}
	return restaurantID, items, nil
}
