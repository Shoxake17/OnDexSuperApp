package couriers

import (
	"context"
	"log/slog"
)

// maxMissedOffers — ketma-ket shuncha taklifga JAVOB BERMAGAN (na qabul, na
// rad) kuryer liniyadan avtomatik chiqariladi.
//
// ┌─ NEGA (2026-09-15) ───────────────────────────────────────────────┐
// Ilovasi yopiq kuryer endi smena davomida push bilan uyg'otiladi
// (`closedAppLocationMaxAge`). Liniyadan chiqishni unutib uyga ketgan
// kuryerning telefoni esa har buyurtmada kechasi ham jiringlab, to'lqin
// joyini behuda egallab turardi. Yandex/Uber kabi: bir necha taklif
// javobsiz qolsa — kuryer liniyadan chiqariladi va bu haqda xabar oladi.
// Rad etish ham JAVOB — hisobni nolga qaytaradi.
// └───────────────────────────────────────────────────────────────────┘
const maxMissedOffers = 3

// AutoOfflineNotifier — ixtiyoriy: `OfferNotifier` shuni ham bajarsa, kuryer
// liniyadan chiqarilgani haqida xabar oladi (jonli kanal + push).
type AutoOfflineNotifier interface {
	CourierAutoOffline(courierID string, missed int)
}

// resetMissed — kuryer taklifga javob berdi.
func (d *Dispatcher) resetMissed(courierID string) {
	d.mu.Lock()
	delete(d.missed, courierID)
	d.mu.Unlock()
}

// recordMissed — taklif muddati kuryer javobisiz tugadi.
func (d *Dispatcher) recordMissed(ctx context.Context, courierID string) {
	d.mu.Lock()
	d.missed[courierID]++
	n := d.missed[courierID]
	if n >= maxMissedOffers {
		delete(d.missed, courierID)
	}
	d.mu.Unlock()
	if n < maxMissedOffers {
		return
	}

	if err := d.repo.SetAvailable(ctx, courierID, false); err != nil {
		slog.Warn("dispatch: javobsiz kuryerni liniyadan chiqarib bo'lmadi",
			"courier", courierID, "err", err)
		return
	}
	slog.Warn("dispatch: kuryer ketma-ket takliflarga javob bermadi — liniyadan chiqarildi",
		"courier", courierID, "javobsiz", n)
	if an, ok := d.notifier.(AutoOfflineNotifier); ok {
		an.CourierAutoOffline(courierID, n)
	}
}
