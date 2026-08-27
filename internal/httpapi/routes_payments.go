package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"chustapp/internal/orders"
	"chustapp/internal/payments"
	"chustapp/internal/payments/octo"
)

// Karta orqali to'lov (Octo) — HTTP qatlami.
//
// ┌─ ISHONCH CHEGARASI ───────────────────────────────────────────────┐
// Bu fayldagi ikkita endpoint butunlay boshqa ishonch darajasida:
//
//	POST /orders/{id}/pay   — MIJOZ chaqiradi (token bilan, egalik
//	                          tekshiriladi). Summa so'rovdan OLINMAYDI.
//	POST /payments/octo/callback — PROVAYDER chaqiradi. Token yo'q va
//	                          bo'lishi ham mumkin emas, shuning uchun
//	                          bu yerga kelgan MA'LUMOTGA ISHONILMAYDI:
//	                          har xabar imzo yoki provayder API'si
//	                          orqali tasdiqlanadi (payments.Service).
// └───────────────────────────────────────────────────────────────────┘
func (s *Server) registerPaymentRoutes(mux *http.ServeMux) {
	// POST /orders/{id}/pay — buyurtma uchun to'lov boshlaydi va
	// to'lov sahifasi havolasini qaytaradi.
	mux.HandleFunc("POST /orders/{id}/pay", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.Payments == nil {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("karta orqali to'lov sozlanmagan"))
				return
			}
			orderID := r.PathValue("id")
			o, err := s.OrderRepo.GetByID(r.Context(), orderID)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			// EGALIK: faqat o'z buyurtmasi uchun to'lov boshlash mumkin.
			// Busiz begona buyurtma ID'sini kiritib, o'sha buyurtmaga
			// to'lov havolasi yaratib olish mumkin bo'lardi.
			if o.CustomerID != claimsFrom(r).Subject {
				httpError(w, http.StatusForbidden, errors.New("bu buyurtma sizniki emas"))
				return
			}
			if !o.PaymentMethod.RequiresPrepayment() {
				httpError(w, http.StatusBadRequest,
					errors.New("bu buyurtma karta orqali to'lash uchun yaratilmagan"))
				return
			}
			if o.IsTerminal() {
				httpError(w, http.StatusBadRequest, errors.New("buyurtma yakunlangan"))
				return
			}

			// `?retry=1` — mijoz "qayta urinish" bosdi. Oldingi urinish
			// bank tomonda o'lgan bo'lishi mumkin (OTP kodini uch marta
			// xato kiritish yoki SMS ni ko'p marta so'rash tranzaksiyani
			// bekor qiladi), o'sha havolani qayta ochish esa yordam
			// bermaydi — YANGI tranzaksiya kerak.
			retry := r.URL.Query().Get("retry") == "1"

			p, err := s.Payments.StartForOrder(r.Context(), o.ID, o.CustomerID,
				o.RestaurantID, "Buyurtma "+o.OrderNumber, retry)
			if errors.Is(err, payments.ErrAlreadyPaid) {
				httpError(w, http.StatusConflict, err)
				return
			}
			if err != nil {
				// Provayder xatosi mijozga XOM holda ko'rsatilmaydi —
				// unda ichki tafsilotlar bo'lishi mumkin.
				slog.Error("to'lov boshlanmadi", "order", o.ID, "error", err)
				httpError(w, http.StatusBadGateway,
					errors.New("to'lov tizimiga ulanib bo'lmadi, birozdan keyin urinib ko'ring"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"payment_id": p.ID,
				"pay_url":    p.PayURL,
				"status":     p.Status,
				"expires_at": p.ExpiresAt,
				// Mobil ilova to'lov sahifasini o'z ichidagi WebView'da
				// ochadi va shu manzilga o'tilganda oynani yopadi.
				"return_url": s.Payments.ReturnURL(),
			})
		}))

	// GET /orders/{id}/payment — mijoz ilovasi to'lov holatini shu
	// yerdan kuzatadi (to'lov sahifasidan qaytgach).
	mux.HandleFunc("GET /orders/{id}/payment", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.Payments == nil {
				httpError(w, http.StatusServiceUnavailable,
					errors.New("karta orqali to'lov sozlanmagan"))
				return
			}
			orderID := r.PathValue("id")
			o, err := s.OrderRepo.GetByID(r.Context(), orderID)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if o.CustomerID != claimsFrom(r).Subject {
				httpError(w, http.StatusForbidden, errors.New("bu buyurtma sizniki emas"))
				return
			}
			list, err := s.Payments.ListByOrder(r.Context(), orderID)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			// Faqat mijozga kerakli maydonlar: provayderning xom javobi
			// va ichki izohlar (`review_reason`) chiqarilmaydi.
			out := make([]map[string]any, 0, len(list))
			for _, p := range list {
				out = append(out, map[string]any{
					"id":           p.ID,
					"status":       p.Status,
					"amount_tiyin": p.AmountTiyin,
					"pay_url":      p.PayURL,
					"expires_at":   p.ExpiresAt,
					"created_at":   p.CreatedAt,
				})
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"order_id":      o.ID,
				"payment_state": o.PaymentState,
				"attempts":      out,
			})
		}))

	// POST /payments/octo/callback — Octo `notify_url`.
	//
	// AUTENTIFIKATSIYA YO'Q va bo'lishi ham mumkin emas: chaqiruvchi —
	// Octo serveri, bizning tokenimiz unda yo'q. Himoya boshqacha
	// quriladi (payments.Service.HandleCallback izohiga qarang):
	// imzo yoki provayder API'si orqali tasdiqlanmagan xabar buyurtmani
	// O'ZGARTIRMAYDI.
	mux.HandleFunc("POST /payments/octo/callback", func(w http.ResponseWriter, r *http.Request) {
		if s.Payments == nil || s.OctoClient == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		var cb octo.Callback
		raw, ok := decodeRaw(w, r, &cb)
		if !ok {
			return
		}

		valid, checked := s.OctoClient.VerifyCallbackSignature(cb)
		data := payments.CallbackData{
			PaymentID:         cb.ShopTransactionID,
			ProviderPaymentID: cb.OctoPaymentUUID,
			Status:            octo.MapStatus(cb.Status),
			SignatureValid:    valid,
			SignatureChecked:  checked,
			Raw:               raw,
		}
		if sum := strings.TrimSpace(cb.TotalSum.String()); sum != "" {
			if tiyin, err := payments.TiyinFromDecimal(sum); err == nil {
				data.AmountTiyin = tiyin
			}
		}

		if err := s.Payments.HandleCallback(r.Context(), data); err != nil {
			slog.Error("octo callback qayta ishlanmadi",
				"payment", cb.ShopTransactionID, "error", err)
			// Provayderga 200 qaytaramiz: 5xx bo'lsa u xabarni
			// cheksiz takrorlaydi. Xato bizning tomonda qayd etilgan
			// va to'lov `needs_review` bo'lib qoladi.
		}

		// ┌─ IKKI BOSQICHLI TO'LOV: JAVOBDAGI QAROR ──────────────────┐
		// Octo pul bloklangach "summani tasdiqlaysizmi?" deb so'raydi
		// va javob kutadi. Biz BU YERDA hech narsani tasdiqlamaymiz:
		// pul faqat RESTORAN buyurtmani qabul qilgandan keyin
		// yechiladi (`orders.Service.settlePayment` -> `set_accept`).
		//
		// Bo'sh javob "hozircha qaror yo'q" degani. Octo xabarni
		// takrorlaydi (jonli sinovda ~50 soniyada bir marta) — bu
		// bizga xalaqit bermaydi, chunki qayta ishlash IDEMPOTENT.
		// └───────────────────────────────────────────────────────────┘
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
}

// paymentMethodFromRequest — so'rovdagi to'lov usulini xavfsiz o'qiydi.
// Noma'lum qiymat NAQD deb qaraladi: xato yozuv tufayli buyurtma
// "to'langan" holatga tushib qolmasligi kerak.
func paymentMethodFromRequest(v string) orders.PaymentMethod {
	if strings.TrimSpace(strings.ToLower(v)) == string(orders.PaymentCard) {
		return orders.PaymentCard
	}
	return orders.PaymentCash
}

// setPaymentMethod — buyurtmaga to'lov usulini va boshlang'ich holatni
// qo'yadi. Kartada holat DARHOL "kutilmoqda" bo'ladi — aynan shu
// qiymat buyurtmani restorandan yashiradi va harakatlanishini
// to'xtatadi (`orders.Order.AwaitingPayment`).
func setPaymentMethod(o *orders.Order, m orders.PaymentMethod) {
	o.PaymentMethod = m
	if m.RequiresPrepayment() {
		o.PaymentState = orders.PaymentAwaiting
	}
}
