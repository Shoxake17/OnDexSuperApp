package couriers

import "context"

type Courier struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Available bool    `json:"available"` // online va bo'sh
	// Approved — superadmin tasdiqlagach true. Tasdiqlanmagan kuryer
	// online bo'la olmaydi va unga taklif yuborilmaydi.
	Approved bool `json:"approved"`
}

// Repository — hozir in-memory, keyin PostGIS: FindNearby SQL'da
// ORDER BY location <-> ST_MakePoint(lng, lat) LIMIT n bo'ladi.
type Repository interface {
	GetByID(ctx context.Context, id string) (*Courier, error)
	Create(ctx context.Context, c *Courier) error
	ListAll(ctx context.Context) ([]*Courier, error)
	// FindNearby — berilgan nuqtaga eng yaqin, tasdiqlangan va bo'sh kuryerlar.
	FindNearby(ctx context.Context, lat, lng float64, limit int) ([]*Courier, error)
	SetAvailable(ctx context.Context, id string, available bool) error
	SetApproved(ctx context.Context, id string, approved bool) error
}
