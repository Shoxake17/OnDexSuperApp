package orders

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrNotFound = errors.New("order not found")

// Repository — saqlash qatlami. Hozir in-memory, keyin PostgreSQL implementatsiyasi
// shu interface'ni qanoatlantiradi va service kodi o'zgarmaydi.
type Repository interface {
	GetByID(ctx context.Context, id string) (*Order, error)
	Save(ctx context.Context, o *Order) error
	// ListRecent — eng so'nggi buyurtmalar (admin panel va hisobotlar uchun).
	ListRecent(ctx context.Context, limit int) ([]*Order, error)
	// HasActiveByRestaurant — restoranning yakunlanmagan buyurtmasi bormi
	// (restoranni o'chirishdan oldin tekshiriladi).
	HasActiveByRestaurant(ctx context.Context, restaurantID string) (bool, error)
}

// Notifier — holat o'zgarganda tashqi dunyoga xabar (push, WebSocket, restoran paneli).
type Notifier interface {
	OrderStatusChanged(o *Order, from Status)
}

type Service struct {
	repo     Repository
	notifier Notifier
	idgen    func() string
	now      func() time.Time
}

func NewService(repo Repository, notifier Notifier, idgen func() string) *Service {
	return &Service{repo: repo, notifier: notifier, idgen: idgen, now: time.Now}
}

func (s *Service) Create(ctx context.Context, o *Order) (*Order, error) {
	if o.CustomerID == "" || o.RestaurantID == "" || len(o.Items) == 0 {
		return nil, errors.New("customer_id, restaurant_id va items majburiy")
	}
	o.ID = s.idgen()
	o.Status = StatusCreated
	o.CreatedAt = s.now()
	o.UpdatedAt = o.CreatedAt
	o.TotalTiyin = 0
	for _, it := range o.Items {
		o.TotalTiyin += it.PriceTiyin * int64(it.Qty)
	}
	if err := s.repo.Save(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Order, error) {
	return s.repo.GetByID(ctx, id)
}

// ChangeStatus — yagona holat o'zgartirish nuqtasi. Hamma handler shu orqali o'tadi,
// shuning uchun state machine'ni chetlab o'tib bo'lmaydi.
func (s *Service) ChangeStatus(ctx context.Context, orderID string, to Status, by Actor) (*Order, error) {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if err := ValidateTransition(o.Status, to, by); err != nil {
		return nil, err
	}
	if to == StatusPickedUp && o.CourierID == "" {
		return nil, fmt.Errorf("buyurtmaga kuryer biriktirilmagan")
	}
	from := o.Status
	o.Status = to
	o.UpdatedAt = s.now()
	o.History = append(o.History, StatusChange{From: from, To: to, By: by, At: o.UpdatedAt})
	if err := s.repo.Save(ctx, o); err != nil {
		return nil, err
	}
	if s.notifier != nil {
		s.notifier.OrderStatusChanged(o, from)
	}
	return o, nil
}

// AssignCourier — dispatcher muvaffaqiyatli yakunlanganda chaqiriladi.
func (s *Service) AssignCourier(ctx context.Context, orderID, courierID string) (*Order, error) {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if o.IsTerminal() {
		return nil, fmt.Errorf("terminal holatdagi buyurtmaga kuryer biriktirib bo'lmaydi")
	}
	if o.CourierID != "" {
		return nil, fmt.Errorf("buyurtmada allaqachon kuryer bor: %s", o.CourierID)
	}
	o.CourierID = courierID
	o.UpdatedAt = s.now()
	if err := s.repo.Save(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}
