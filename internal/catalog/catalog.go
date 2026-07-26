package catalog

import (
	"context"
	"errors"
)

type Restaurant struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Address string  `json:"address"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Open    bool    `json:"open"` // hozir buyurtma qabul qilyaptimi
}

type Product struct {
	ID           string `json:"id"`
	RestaurantID string `json:"restaurant_id"`
	Name         string `json:"name"`
	PriceTiyin   int64  `json:"price_tiyin"`
	Available    bool   `json:"available"`
}

var (
	ErrNotFound         = errors.New("topilmadi")
	ErrRestaurantClosed = errors.New("restoran hozir yopiq")
	ErrMixedRestaurants = errors.New("bitta buyurtmada faqat bitta restoran taomlari bo'lishi mumkin")
	ErrUnavailable      = errors.New("taom hozir mavjud emas")
)

type Repository interface {
	ListRestaurants(ctx context.Context) ([]*Restaurant, error)
	GetRestaurant(ctx context.Context, id string) (*Restaurant, error)
	SaveRestaurant(ctx context.Context, r *Restaurant) error // yangi yoki yangilash
	// DeleteRestaurant — restoran va uning barcha taomlarini o'chiradi.
	DeleteRestaurant(ctx context.Context, id string) error
	ListProducts(ctx context.Context, restaurantID string) ([]*Product, error)
	GetProductsByIDs(ctx context.Context, ids []string) ([]*Product, error)
	SaveProduct(ctx context.Context, p *Product) error
}
