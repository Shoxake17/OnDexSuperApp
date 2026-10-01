package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"chustapp/internal/orders"
	"chustapp/internal/payments"
	"chustapp/internal/payments/multicard"
	"chustapp/internal/payments/multicardpay"
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
//
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

			// `?ps=payme|click|uzum` — mijoz AYNAN bitta ilovani
			// tanladi. Bo'sh bo'lsa universal to'lov sahifasi ochiladi
			// (karta + hamma xizmat bir joyda).
			//
			// ⚠️ Noma'lum qiymat JIMGINA e'tiborsiz qoldirilmaydi:
			// mijoz "Payme" tanlab, boshqa sahifaga tushib qolmasin
			// (bug.md 81-band bilan bir xil mantiq).
			channel := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("ps")))
			if channel != "" && !multicard.IsAppPaymentSystem(channel) {
				httpError(w, http.StatusBadRequest,
					fmt.Errorf("to'lov xizmati qo'llanmaydi: %q", channel))
				return
			}

			// Restoran shu usulni O'ZI yoqganmi? Panelda o'chirilgan usul
			// mijoz ilovasida yashirinadi, lekin eski ilova yoki to'g'ridan-
			// to'g'ri so'rov uni baribir yuborishi mumkin — haqiqiy to'siq
			// shu yerda.
			rest, rerr := s.CatalogRepo.GetRestaurant(r.Context(), o.RestaurantID)
			if rerr != nil {
				slog.Error("to'lov: restoran topilmadi", "order", o.ID, "error", rerr)
				httpError(w, http.StatusInternalServerError,
					errors.New("restoran ma'lumotini o'qib bo'lmadi"))
				return
			}
			if !rest.AcceptsChannel(channel) {
				httpError(w, http.StatusConflict,
					errors.New("bu restoran tanlangan to'lov usulini qabul qilmaydi — boshqa usulni tanlang"))
				return
			}

			p, err := s.Payments.StartForOrder(r.Context(), o.ID, o.CustomerID,
				o.RestaurantID, "Buyurtma "+o.OrderNumber, channel, retry)
			if errors.Is(err, payments.ErrAlreadyPaid) {
				httpError(w, http.StatusConflict, err)
				return
			}
			if err != nil {
				slog.Error("to'lov boshlanmadi", "order", o.ID, "error", err)
				writeStartPaymentError(w, err)
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

	// POST /payments/multicard/callback — Multicard `callback_url`.
	//
	// ┌─ IMZO KALITI RESTORANGA BOG'LIQ ───────────────────────────────┐
	// Multicard imzoni `SHA1(uuid + invoice_id + amount + secret)` deb
	// hisoblaydi, `secret` esa HAR RESTORANNING o'ziniki (u shlyuz
	// bilan o'z shartnomasini tuzadi). Ya'ni imzoni tekshirishdan
	// OLDIN "bu qaysi restoranning to'lovi?" degan savolga javob
	// kerak — u `invoice_id` (bizning to'lov ID'si) orqali topiladi.
	//
	// Topilmasa ham javob 200 bo'ladi: 5xx qaytarsak shlyuz xabarni
	// besh marta takrorlaydi va hech narsa o'zgarmaydi.
	// └────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("POST /payments/multicard/callback", func(w http.ResponseWriter, r *http.Request) {
		if s.Payments == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		var cb multicard.Callback
		raw, ok := decodeRaw(w, r, &cb)
		if !ok {
			return
		}

		// ⚠️ Imzo ASOSIY himoya. Manba IP faqat qo'shimcha signal:
		// proksi orqasida u ishonchsiz, shuning uchun RAD ETMAYDI,
		// faqat logga yoziladi (kutilmagan manba ko'rinib tursin).
		if ip := clientIP(r); ip != "" && ip != multicard.CallbackSourceIP {
			slog.Warn("multicard callback kutilmagan manzildan",
				"ip", ip, "kutilgan", multicard.CallbackSourceIP, "invoice", cb.InvoiceID)
		}

		valid, checked := false, false
		if restaurantID, err := s.Payments.RestaurantForPayment(r.Context(), cb.InvoiceID); err == nil {
			if cl, _, cerr := s.MC.ClientFor(r.Context(), restaurantID); cerr == nil && cl != nil {
				valid, checked = cl.VerifyCallback(cb), true
			} else if cerr != nil {
				slog.Error("multicard callback: restoran kalitini olib bo'lmadi",
					"restaurant", restaurantID, "err", cerr)
			}
		}

		if err := s.Payments.HandleCallback(r.Context(), payments.CallbackData{
			PaymentID:         cb.InvoiceID,
			ProviderPaymentID: cb.UUID,
			Status:            multicardpay.MapStatus(cb.Status),
			AmountTiyin:       cb.Amount.Int64(),
			SignatureValid:    valid,
			SignatureChecked:  checked,
			Raw:               raw,
		}); err != nil {
			slog.Error("multicard callback qayta ishlanmadi",
				"invoice", cb.InvoiceID, "uuid", cb.UUID, "error", err)
			// Shlyuzga baribir 2xx: 5xx bo'lsa u xabarni besh marta
			// takrorlaydi, natija esa o'zgarmaydi. Xato bizning
			// tomonda qayd etilgan va to'lov `needs_review` bo'ladi.
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	})
}

// writeStartPaymentError — to'lov boshlanmaganda mijozga NIMA
// QILISHNI aytadi.
//
// ┌─ "KEYINROQ URINIB KO'RING" HAR DOIM TO'G'RI EMAS ─────────────────┐
// Avval bu yerda HAR QANDAY xato 502 va "to'lov tizimiga ulanib
// bo'lmadi, birozdan keyin urinib ko'ring" ga aylanardi. Jonli
// sinovda sabab butunlay boshqa edi: restoran kassasida fiskal
// standart (IKPU) yo'q. Qayta urinish bunday xatoni HECH QACHON
// tuzatmaydi — mijoz besh marta bosdi va har safar o'sha javobni
// oldi (logda ko'rinib turibdi).
//
// Shuning uchun endi uch toifa:
//   - sozlama yetishmaydi (ulanmagan, IKPU yo'q) → 409, aniq matn:
//     mijoz NAQD ni tanlashi mumkin;
//   - shlyuz so'rovni rad etdi (4xx) → 422;
//   - shlyuz javob bermadi / 5xx → 502, "keyinroq urinib ko'ring".
//
// 409 va 422 — 5xx EMAS, ya'ni matn `httpError` tomonidan
// yashirilmaydi va mijozga yetib boradi.
// └───────────────────────────────────────────────────────────────────┘
func writeStartPaymentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, multicardpay.ErrNotConnected):
		httpError(w, http.StatusConflict, errors.New(
			"bu restoran karta orqali to'lovni hali ulamagan — "+
				"naqd pulni tanlang"))
		return
	case errors.Is(err, multicardpay.ErrNoFiscalDefaults):
		httpError(w, http.StatusConflict, errors.New(
			"restoranning to'lov kassasi to'liq sozlanmagan — "+
				"hozircha naqd pulni tanlang"))
		return
	case errors.Is(err, payments.ErrNotConfigured):
		httpError(w, http.StatusConflict,
			errors.New("karta orqali to'lov sozlanmagan — naqd pulni tanlang"))
		return
	}
	// Shlyuz javob berdi, lekin so'rovni rad etdi: qayta urinish
	// yordam bermaydi.
	var provErr *payments.ProviderError
	if errors.As(err, &provErr) && provErr.Code >= 400 && provErr.Code < 500 {
		httpError(w, http.StatusUnprocessableEntity, errors.New(
			"to'lov tizimi so'rovni rad etdi — restoran bilan bog'laning "+
				"yoki naqd pulni tanlang"))
		return
	}
	// Qolgani — haqiqatan vaqtinchalik nosozlik.
	httpError(w, http.StatusBadGateway,
		errors.New("to'lov tizimiga ulanib bo'lmadi, birozdan keyin urinib ko'ring"))
}

// paymentMethodFromRequest — so'rovdagi to'lov usulini o'qiydi.
//
// ┌─ TUZATILGAN NOSOZLIK (bug.md 81-band, SERVER TOMONI) ──────────────┐
// Avval bu funksiya noma'lum qiymatni JIMGINA naqd deb qabul
// qilardi. Pul tomonidan bu xavfsiz edi (buyurtma "to'langan" bo'lib
// qolmasdi), lekin u AYNAN 81-bandni yashirardi:
//
//	klient `"payme"` yuboradi → server `cash` yozadi → buyurtma
//	oshxonaga tushadi → kuryer eshik oldida naqd pul so'raydi.
//
// Ilova tomonidagi xato server logida ham, javobda ham ko'rinmasdi.
//
// Endi noma'lum qiymat XATO bilan rad etiladi: klientdagi nosozlik
// birinchi so'rovdayoq chiqadi. Bo'sh qiymat esa avvalgidek naqd —
// eski klientlar `payment_method` ni umuman yubormaydi.
// └────────────────────────────────────────────────────────────────────┘
func paymentMethodFromRequest(v string) (orders.PaymentMethod, error) {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "":
		// Ko'rsatilmagan — naqd (eski klientlar bilan moslik).
		return orders.PaymentCash, nil
	case string(orders.PaymentCash):
		return orders.PaymentCash, nil
	case string(orders.PaymentCard):
		return orders.PaymentCard, nil
	case string(orders.PaymentWallet):
		return orders.PaymentWallet, nil
	default:
		return "", fmt.Errorf("to'lov usuli qo'llab-quvvatlanmaydi: %q (faqat %q, %q yoki %q)",
			v, orders.PaymentCash, orders.PaymentCard, orders.PaymentWallet)
	}
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
