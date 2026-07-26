package couriers

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Dispatch oqimi:
//  1. Restoran atrofidagi eng yaqin bo'sh kuryerlar ro'yxati olinadi.
//  2. Birinchisiga taklif yuboriladi (push/WebSocket) va OfferTTL kutiladi.
//  3. Qabul qilsa — kuryer band qilinadi va order'ga biriktiriladi.
//     Rad etsa yoki vaqt tugasa — navbatdagi kuryerga o'tiladi.
//  4. Hamma rad etsa — ErrNoCourier; bu holatda admin panelga signal ketishi kerak.

var (
	ErrNoCourier      = errors.New("bo'sh kuryer topilmadi")
	ErrAlreadyRunning = errors.New("bu buyurtma uchun dispatch allaqachon ishlayapti")
)

// OfferNotifier — kuryerga taklifni yetkazadi (real hayotda: FCM push + WebSocket).
type OfferNotifier interface {
	SendOffer(courierID, orderID string, expiresIn time.Duration)
	CancelOffer(courierID, orderID string)
}

type Response struct {
	CourierID string
	Accepted  bool
}

type Dispatcher struct {
	repo     Repository
	notifier OfferNotifier
	offerTTL time.Duration // bitta kuryer taklifga necha sekund javob berishi mumkin
	maxCands int           // nechta kuryergacha urinib ko'riladi

	mu      sync.Mutex
	pending map[string]pendingOffer // orderID -> hozir taklif turgan kuryer
}

type pendingOffer struct {
	courierID string
	respCh    chan Response
}

func NewDispatcher(repo Repository, notifier OfferNotifier, offerTTL time.Duration, maxCandidates int) *Dispatcher {
	return &Dispatcher{
		repo:     repo,
		notifier: notifier,
		offerTTL: offerTTL,
		maxCands: maxCandidates,
		pending:  make(map[string]pendingOffer),
	}
}

// Dispatch — bloklanuvchi chaqiruv: kuryer topilguncha yoki kandidatlar tugaguncha ishlaydi.
// Chaqiruvchi buni goroutine'da ishga tushiradi. ctx bekor qilinsa (masalan, buyurtma
// bekor bo'lsa) dispatch to'xtaydi.
func (d *Dispatcher) Dispatch(ctx context.Context, orderID string, restaurantLat, restaurantLng float64) (string, error) {
	d.mu.Lock()
	if _, exists := d.pending[orderID]; exists {
		d.mu.Unlock()
		return "", ErrAlreadyRunning
	}
	d.mu.Unlock()

	candidates, err := d.repo.FindNearby(ctx, restaurantLat, restaurantLng, d.maxCands)
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", ErrNoCourier
	}

	for _, c := range candidates {
		accepted, err := d.offerAndWait(ctx, orderID, c.ID)
		if err != nil {
			return "", err // faqat ctx bekor bo'lganda
		}
		if accepted {
			if err := d.repo.SetAvailable(ctx, c.ID, false); err != nil {
				return "", err
			}
			slog.Info("dispatch: kuryer topildi", "order", orderID, "courier", c.ID)
			return c.ID, nil
		}
	}
	return "", ErrNoCourier
}

func (d *Dispatcher) offerAndWait(ctx context.Context, orderID, courierID string) (accepted bool, err error) {
	respCh := make(chan Response, 1)

	d.mu.Lock()
	d.pending[orderID] = pendingOffer{courierID: courierID, respCh: respCh}
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		delete(d.pending, orderID)
		d.mu.Unlock()
	}()

	d.notifier.SendOffer(courierID, orderID, d.offerTTL)
	timer := time.NewTimer(d.offerTTL)
	defer timer.Stop()

	select {
	case resp := <-respCh:
		if !resp.Accepted {
			slog.Info("dispatch: kuryer rad etdi", "order", orderID, "courier", courierID)
		}
		return resp.Accepted, nil
	case <-timer.C:
		slog.Info("dispatch: taklif vaqti tugadi", "order", orderID, "courier", courierID)
		d.notifier.CancelOffer(courierID, orderID)
		return false, nil
	case <-ctx.Done():
		d.notifier.CancelOffer(courierID, orderID)
		return false, ctx.Err()
	}
}

// HandleResponse — kuryer ilovadan "qabul qilaman / rad etaman" bosganda HTTP handler
// shu metodni chaqiradi. Eskirgan yoki begona javoblar (boshqa kuryerga tegishli taklif)
// jimgina e'tiborsiz qoldiriladi — bu race'larga qarshi asosiy himoya.
func (d *Dispatcher) HandleResponse(orderID string, resp Response) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	p, ok := d.pending[orderID]
	if !ok || p.courierID != resp.CourierID {
		return false // taklif allaqachon eskirgan
	}
	select {
	case p.respCh <- resp:
		return true
	default:
		return false
	}
}
