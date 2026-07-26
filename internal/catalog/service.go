package catalog

import (
	"context"
	"errors"
	"fmt"

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
}

func NewService(repo Repository) *Service { return &Service{repo: repo} }

// PriceOrder — buyurtma tarkibini katalog bo'yicha tekshiradi va narxlaydi.
// Qoidalar: hamma taom mavjud va bitta restoranniki, restoran ochiq bo'lishi kerak.
func (s *Service) PriceOrder(ctx context.Context, reqs []ItemRequest) (restaurantID string, items []orders.Item, err error) {
	if len(reqs) == 0 {
		return "", nil, errors.New("buyurtma bo'sh")
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
		items = append(items, orders.Item{
			ProductID:  p.ID,
			Name:       p.Name,
			Qty:        r.Qty,
			PriceTiyin: p.PriceTiyin,
		})
	}

	rest, err := s.repo.GetRestaurant(ctx, restaurantID)
	if err != nil {
		return "", nil, err
	}
	if !rest.Open {
		return "", nil, ErrRestaurantClosed
	}
	return restaurantID, items, nil
}
