// Ovozli yo'l ko'rsatish: restoran nomi bor "yetib keldingiz" iborasi.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"chustapp/internal/orders"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
	"chustapp/internal/voice"
)

// voiceLimiter — kuryer bo'yicha: 6 ta bir zumda, keyin 5 soniyada bitta.
// Ilova bitta buyurtmaga bir marta so'raydi; chegara pullik TTS'ga
// sikl bilan urilishni to'xtatadi.
var voiceLimiter = ratelimit.New(0.2, 6)

func (s *Server) registerVoiceRoutes(mux *http.ServeMux) {
	// GET /couriers/{id}/voice/arrival?order_id=... — "Siz <restoran>
	// restoraniga yetib keldingiz" (WAV).
	//
	// ┌─ XAVFSIZLIK ──────────────────────────────────────────────────────┐
	// Matnni ilova BERMAYDI: nom kuryerga BIRIKTIRILGAN, yakunlanmagan
	// buyurtmaning restoranidan olinadi va tozalanadi (`voice.CleanName`).
	// Aks holda istalgan matnni pullik TTS'da o'qitib bo'lardi. Ibora bir
	// marta sintez qilinib bazada saqlanadi; kalit faqat serverda.
	// └───────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("GET /couriers/{id}/voice/arrival", s.auth([]users.Role{users.RoleCourier},
		func(w http.ResponseWriter, r *http.Request) {
			courierID := r.PathValue("id")
			if claimsFrom(r).EntityID != courierID {
				httpError(w, http.StatusForbidden, errors.New("boshqa kuryer nomidan so'rab bo'lmaydi"))
				return
			}
			if s.Voice == nil {
				httpError(w, http.StatusServiceUnavailable, errors.New("ovozli yo'l ko'rsatish sozlanmagan"))
				return
			}
			if !voiceLimiter.Allow(courierID) {
				httpError(w, http.StatusTooManyRequests, errors.New("juda tez-tez so'ralmoqda"))
				return
			}
			o, err := s.OrderSvc.Get(r.Context(), r.URL.Query().Get("order_id"))
			if err != nil || o.CourierID != courierID || o.IsDineIn() || o.IsTerminal() {
				httpError(w, http.StatusNotFound, orders.ErrNotFound)
				return
			}
			rest := s.restaurantFor(r.Context(), o.RestaurantID)
			if rest == nil {
				httpError(w, http.StatusNotFound, orders.ErrNotFound)
				return
			}
			text, ok := voice.ArrivalAtRestaurant(rest.Name)
			if !ok {
				httpError(w, http.StatusNotFound, errors.New("restoran nomi yo'q"))
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
			defer cancel()
			data, err := s.Voice.Clip(ctx, text)
			if err != nil {
				slog.Warn("voice: restoran iborasi tayyor emas", "restaurant", o.RestaurantID, "err", err)
				httpError(w, http.StatusServiceUnavailable, errors.New("ovoz hozircha tayyor emas — keyinroq urinib ko'ring"))
				return
			}
			w.Header().Set("Content-Type", "audio/wav")
			w.Header().Set("Content-Length", strconv.Itoa(len(data)))
			w.Header().Set("Cache-Control", "private, max-age=86400")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = w.Write(data)
		}))
}
