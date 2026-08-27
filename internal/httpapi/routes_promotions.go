package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/promotions"
	"chustapp/internal/users"
)

func (s *Server) registerPromotionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /restaurants/{id}/active-promotions", func(w http.ResponseWriter, r *http.Request) {
		restaurantID := r.PathValue("id")
		list, err := s.PromotionsRepo.ListByRestaurant(r.Context(), restaurantID)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		now := time.Now()
		out := make([]map[string]any, 0, len(list))
		for _, p := range list {
			if p.ComputeStatus(now) != promotions.StatusActive {
				continue
			}
			// ┌─ NEGA HAMMA CHEKLOV MAYDONI KERAK ────────────────────┐
			// Klient (menyu kartochkasi) shu ma'lumot asosida
			// chegirmali narxni CHIZADI. Maydon yuborilmasa, u
			// cheklovni bilmay to'liq chegirmani ko'rsatadi va
			// checkout'dagi HAQIQIY summa boshqacha chiqadi —
			// `max_discount_amount_tiyin` aynan shu sababdan
			// yetishmasdi ("30%, lekin ko'pi bilan 50 000 so'm"
			// aksiyasida menyu to'liq 30% ni ko'rsatardi).
			//
			// `discount_unit` — EffectiveUnit(), ya'ni turga zid
			// bo'lgan eski yozuv ham klientga to'g'ri birlikda
			// boradi.
			// └───────────────────────────────────────────────────────┘
			out = append(out, map[string]any{
				"id":                        p.ID,
				"name":                      p.Name,
				"description":               p.Description,
				"type":                      p.Type,
				"discount_unit":             p.EffectiveUnit(),
				"discount_value":            p.DiscountValue,
				"min_order_amount_tiyin":    p.MinOrderAmountTiyin,
				"max_discount_amount_tiyin": p.MaxDiscountAmountTiyin,
				"applies_to_products":       p.AppliesToProducts,
				"applies_to_orders":         p.AppliesToOrders,
				"applies_to_categories":     p.AppliesToCategories,
				"target_product_ids":        p.TargetProductIDs,
				"target_categories":         p.TargetCategories,
				"min_previous_orders":       p.MinPreviousOrders,
			})
		}
		writeJSON(w, http.StatusOK, out)
	})

	// GET /restaurants/{id}/promotions
	mux.HandleFunc("GET /restaurants/{id}/promotions", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			claims := claimsFrom(r)
			if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran aksiyalarini ko'rib bo'lmaydi"))
				return
			}
			list, err := s.PromotionsRepo.ListByRestaurant(r.Context(), restaurantID)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, list)
		}))

	// resolvePromotionConflicts — `incoming` bilan BIR XIL mahsulot/turkumga
	// tegishli, HOZIR faol boshqa aksiyalarni avtomatik moslashtiradi (ikkita
	// aksiya bir xil mahsulotga sababsiz "yashirincha" tortishmasin degan
	// qat'iy qoida). Ikki xil holat farqlanadi:
	//   - TO'LIQ to'qnashuv (masalan eskisi ham xuddi shu mahsulotlarga, yoki
	//     biri "Buyurtmalar" — butun savat) — eskisi BUTUNLAY to'xtatiladi.
	//   - QISMAN to'qnashuv (eskisi N ta mahsulotga, yangisi shulardan
	//     ba'zilariga tegishli) — FAQAT to'qnashgan mahsulot/turkumlar
	//     eskisining ro'yxatidan olib tashlanadi, qolganlariga eskisi davom
	//     etadi (ma'lumot yo'qolmaydi). Aralash mahsulot<->turkum holati
	//     ATAYLAB hisobga olinmaydi — kamdan-kam uchraydi.
	// Qaytadi: to'liq to'xtatilgan va qisman moslashtirilgan aksiya nomlari.
	resolvePromotionConflicts := func(ctx context.Context, restaurantID string, incoming *promotions.Promotion) (stopped, adjusted []string) {
		if !incoming.Active {
			return nil, nil
		}
		existing, err := s.PromotionsRepo.ListByRestaurant(ctx, restaurantID)
		if err != nil {
			return nil, nil
		}
		now := time.Now()
		for _, other := range existing {
			if other.ID == incoming.ID || !other.Active {
				continue
			}
			if other.ComputeStatus(now) != promotions.StatusActive {
				continue
			}
			if incoming.AppliesToOrders || other.AppliesToOrders {
				other.Active = false
				if err := s.PromotionsRepo.Save(ctx, other); err == nil {
					stopped = append(stopped, other.Name)
				}
				continue
			}

			changed := false
			if incoming.AppliesToProducts && other.AppliesToProducts && len(other.TargetProductIDs) > 0 {
				newSet := make(map[string]bool, len(incoming.TargetProductIDs))
				for _, id := range incoming.TargetProductIDs {
					newSet[id] = true
				}
				kept := other.TargetProductIDs[:0:0]
				for _, id := range other.TargetProductIDs {
					if newSet[id] {
						changed = true
						continue
					}
					kept = append(kept, id)
				}
				other.TargetProductIDs = kept
			}
			if incoming.AppliesToCategories && other.AppliesToCategories && len(other.TargetCategories) > 0 {
				newSet := make(map[string]bool, len(incoming.TargetCategories))
				for _, c := range incoming.TargetCategories {
					newSet[c] = true
				}
				kept := other.TargetCategories[:0:0]
				for _, c := range other.TargetCategories {
					if newSet[c] {
						changed = true
						continue
					}
					kept = append(kept, c)
				}
				other.TargetCategories = kept
			}
			if !changed {
				continue
			}
			other.AppliesToProducts = other.AppliesToProducts && len(other.TargetProductIDs) > 0
			other.AppliesToCategories = other.AppliesToCategories && len(other.TargetCategories) > 0
			if err := s.PromotionsRepo.Save(ctx, other); err != nil {
				continue
			}
			if !other.AppliesToProducts && !other.AppliesToCategories && !other.AppliesToOrders {
				stopped = append(stopped, other.Name)
			} else {
				adjusted = append(adjusted, other.Name)
			}
		}
		return stopped, adjusted
	}

	// POST /restaurants/{id}/promotions — yaratish yoki tahrirlash (id
	// berilsa yangilaydi, aks holda yangi yaratadi — products bilan bir xil naqsh).
	mux.HandleFunc("POST /restaurants/{id}/promotions", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			claims := claimsFrom(r)
			if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran aksiyalarini o'zgartirib bo'lmaydi"))
				return
			}
			if _, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID); err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			var req struct {
				ID                     string   `json:"id"`
				Name                   string   `json:"name"`
				Description            string   `json:"description"`
				Type                   string   `json:"type"`
				DiscountUnit           string   `json:"discount_unit"`
				DiscountValue          int64    `json:"discount_value"`
				MinOrderAmountTiyin    int64    `json:"min_order_amount_tiyin"`
				MaxDiscountAmountTiyin int64    `json:"max_discount_amount_tiyin"`
				StartAt                string   `json:"start_at"`
				EndAt                  string   `json:"end_at"`
				Indefinite             bool     `json:"indefinite"`
				Active                 bool     `json:"active"`
				AppliesToProducts      bool     `json:"applies_to_products"`
				AppliesToOrders        bool     `json:"applies_to_orders"`
				AppliesToCategories    bool     `json:"applies_to_categories"`
				TargetProductIDs       []string `json:"target_product_ids"`
				TargetCategories       []string `json:"target_categories"`
				MinPreviousOrders      int64    `json:"min_previous_orders"`
				ImageURL               string   `json:"image_url"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			req.Name = strings.TrimSpace(req.Name)
			req.Description = strings.TrimSpace(req.Description)
			if req.Name == "" || len(req.Name) > promotions.MaxNameLength {
				httpError(w, http.StatusBadRequest, errors.New("nomi majburiy va juda uzun bo'lmasligi kerak"))
				return
			}
			if len(req.Description) > promotions.MaxDescriptionLength {
				httpError(w, http.StatusBadRequest, errors.New("tavsif juda uzun"))
				return
			}
			typ := promotions.Type(req.Type)
			if !promotions.IsValidType(typ) {
				httpError(w, http.StatusBadRequest, errors.New("turi noto'g'ri"))
				return
			}
			unit := promotions.DiscountUnit(req.DiscountUnit)
			if unit != promotions.DiscountUnitPercent && unit != promotions.DiscountUnitAmount {
				httpError(w, http.StatusBadRequest, errors.New("discount_unit noto'g'ri (percent yoki amount)"))
				return
			}
			// ┌─ TUR VA BIRLIK ZID BO'LMASLIGI KERAK ─────────────────┐
			// "Foiz orqali chegirma" turi + `amount` birligi kabi
			// juftlik AVVAL ruxsat etilardi va server bilan klient uni
			// BOSHQACHA o'qirdi: menyuda "-20%" ko'rinib, haqiqatda 20
			// tiyin chegirma berilardi. Endi bunday yozuv umuman
			// yaratilmaydi (mavjud eskilarini `EffectiveUnit()`
			// xavfsiz o'qiydi).
			//
			// Jimgina TO'G'RILAB qo'yilmaydi — kiritilgan raqamning
			// MA'NOSI o'zgarib ketardi (20% mi, 20 tiyinmi?), shuning
			// uchun restoran o'zi hal qilishi uchun xato qaytariladi.
			// └───────────────────────────────────────────────────────┘
			if typ == promotions.TypePercent && unit != promotions.DiscountUnitPercent {
				httpError(w, http.StatusBadRequest, errors.New("foiz orqali chegirma uchun birlik % bo'lishi kerak"))
				return
			}
			if typ == promotions.TypeFixedAmount && unit != promotions.DiscountUnitAmount {
				httpError(w, http.StatusBadRequest, errors.New("summa orqali chegirma uchun birlik so'm bo'lishi kerak"))
				return
			}
			if unit == promotions.DiscountUnitPercent {
				if req.DiscountValue < 1 || req.DiscountValue > 100 {
					httpError(w, http.StatusBadRequest, errors.New("chegirma foizi 1-100 oralig'ida bo'lishi kerak"))
					return
				}
			} else if req.DiscountValue <= 0 {
				httpError(w, http.StatusBadRequest, errors.New("chegirma summasi musbat bo'lishi kerak"))
				return
			}
			if req.MinOrderAmountTiyin < 0 || req.MaxDiscountAmountTiyin < 0 {
				httpError(w, http.StatusBadRequest, errors.New("minimal/maksimal summalar manfiy bo'lishi mumkin emas"))
				return
			}
			if req.MinPreviousOrders < 0 {
				httpError(w, http.StatusBadRequest, errors.New("oldingi buyurtmalar soni manfiy bo'lishi mumkin emas"))
				return
			}
			startAt, err := time.Parse(time.RFC3339, req.StartAt)
			if err != nil {
				httpError(w, http.StatusBadRequest, errors.New("start_at noto'g'ri"))
				return
			}
			var endAt time.Time
			if !req.Indefinite {
				endAt, err = time.Parse(time.RFC3339, req.EndAt)
				if err != nil {
					httpError(w, http.StatusBadRequest, errors.New("end_at noto'g'ri"))
					return
				}
				if endAt.Before(startAt) {
					httpError(w, http.StatusBadRequest, errors.New("end_at start_at'dan oldin bo'lishi mumkin emas"))
					return
				}
			} else {
				endAt = startAt.AddDate(100, 0, 0)
			}
			if !req.AppliesToProducts && !req.AppliesToOrders && !req.AppliesToCategories {
				httpError(w, http.StatusBadRequest, errors.New("kamida bitta qo'llanilish joyini tanlang"))
				return
			}
			if req.AppliesToProducts && len(req.TargetProductIDs) == 0 {
				httpError(w, http.StatusBadRequest, errors.New("mahsulotlar tanlanmagan"))
				return
			}
			if req.AppliesToCategories && len(req.TargetCategories) == 0 {
				httpError(w, http.StatusBadRequest, errors.New("turkumlar tanlanmagan"))
				return
			}
			if !req.AppliesToProducts {
				req.TargetProductIDs = nil
			} else {
				// Tanlangan mahsulotlar HAQIQATDA shu restoranga tegishli
				// ekanini tekshiramiz — boshqa restoran mahsulot ID'sini
				// qo'yish orqali chetlab o'tishning oldini olish uchun.
				products, err := s.CatalogRepo.GetProductsByIDs(r.Context(), req.TargetProductIDs)
				if err != nil {
					httpError(w, http.StatusInternalServerError, err)
					return
				}
				if len(products) != len(req.TargetProductIDs) {
					httpError(w, http.StatusBadRequest, errors.New("tanlangan mahsulotlardan biri topilmadi"))
					return
				}
				for _, p := range products {
					if p.RestaurantID != restaurantID {
						httpError(w, http.StatusForbidden, errors.New("boshqa restoran mahsulotini tanlab bo'lmaydi"))
						return
					}
				}
			}
			if !req.AppliesToCategories {
				req.TargetCategories = nil
			} else {
				// Turkumlar platformaning STANDART ro'yxatidan
				// (`catalog.PredefinedCategories`) bo'lishi shart.
				//
				// Avval bu ro'yxat UMUMAN tekshirilmasdi (faqat bo'sh
				// emasligi): turkum — oddiy matn, shuning uchun
				// restoran paneli (yoki to'g'ridan-to'g'ri API
				// chaqiruvi) istalgan qatorni, istalgan miqdorda
				// yuborishi mumkin edi. Natijada (a) hech qachon
				// qo'llanmaydigan "o'lik" aksiya yaratilardi —
				// mahsulotning `Category` maydoni shu ro'yxatdan
				// bo'lgani uchun erkin matn hech qachon mos kelmasdi
				// — va restoran nega ishlamayotganini tushunmasdi,
				// (b) chegarasiz massiv bazaga bemalol yozilardi.
				//
				// Menyudagi MAVJUD turkumlarga qarab tekshirmaymiz:
				// restoran hali mahsulot qo'shmagan turkum uchun ham
				// oldindan aksiya tayyorlab qo'yishi mumkin bo'lishi
				// kerak.
				known := make(map[string]bool, len(catalog.PredefinedCategories))
				for _, c := range catalog.PredefinedCategories {
					known[c] = true
				}
				seen := make(map[string]bool, len(req.TargetCategories))
				cleaned := make([]string, 0, len(req.TargetCategories))
				for _, c := range req.TargetCategories {
					c = strings.TrimSpace(c)
					if c == "" || seen[c] {
						continue // bo'sh va takroriy qiymatlar tashlanadi
					}
					if !known[c] {
						httpError(w, http.StatusBadRequest,
							fmt.Errorf("noma'lum turkum: %s", c))
						return
					}
					seen[c] = true
					cleaned = append(cleaned, c)
				}
				if len(cleaned) == 0 {
					httpError(w, http.StatusBadRequest, errors.New("turkumlar tanlanmagan"))
					return
				}
				req.TargetCategories = cleaned
			}

			p := &promotions.Promotion{
				ID:                     req.ID,
				RestaurantID:           restaurantID,
				Name:                   req.Name,
				Description:            req.Description,
				Type:                   typ,
				DiscountUnit:           unit,
				DiscountValue:          req.DiscountValue,
				MinOrderAmountTiyin:    req.MinOrderAmountTiyin,
				MaxDiscountAmountTiyin: req.MaxDiscountAmountTiyin,
				StartAt:                startAt,
				EndAt:                  endAt,
				Indefinite:             req.Indefinite,
				Active:                 req.Active,
				AppliesToProducts:      req.AppliesToProducts,
				AppliesToOrders:        req.AppliesToOrders,
				AppliesToCategories:    req.AppliesToCategories,
				TargetProductIDs:       req.TargetProductIDs,
				TargetCategories:       req.TargetCategories,
				MinPreviousOrders:      req.MinPreviousOrders,
				ImageURL:               req.ImageURL,
				CreatedAt:              time.Now(),
			}
			if p.ID == "" {
				p.ID = NewID()
			} else if existing, err := s.PromotionsRepo.GetByID(r.Context(), p.ID); err == nil {
				if existing.RestaurantID != restaurantID {
					httpError(w, http.StatusForbidden, errors.New("bu aksiya sizning restoraningizga tegishli emas"))
					return
				}
				p.CreatedAt = existing.CreatedAt
				// Haqiqiy qo'llanilish statistikasi (Mongo ReplaceOne butun
				// hujjatni almashtiradi) tahrirlashda TASODIFAN 0'ga
				// qaytib ketmasligi uchun eskisidan ko'chiriladi.
				p.UsageCount = existing.UsageCount
				p.SalesTotalTiyin = existing.SalesTotalTiyin
			}
			if err := s.PromotionsRepo.Save(r.Context(), p); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			// Yangi aksiya bilan bir xil mahsulot/turkumga qarab turgan
			// boshqa faol aksiyalar avtomatik moslashtiriladi — ikkita
			// aksiya bir xil mahsulotga sababsiz "yashirincha" tortishmasin
			// (avval ApplyBest ENG KO'P chegirma beruvchisini jimgina
			// tanlardi, lekin eskisi "faol" bo'lib ro'yxatda qolib,
			// chalkashtirar edi). resolvePromotionConflicts()ga qarang.
			stoppedNames, adjustedNames := resolvePromotionConflicts(r.Context(), restaurantID, p)
			s.Hub.Send(restaurantTopic(restaurantID), map[string]any{"type": "promotions_updated"})
			writeJSON(w, http.StatusCreated, savePromotionResponse{
				Promotion:              p,
				StoppedPromotionNames:  stoppedNames,
				AdjustedPromotionNames: adjustedNames,
			})
		}))

	// DELETE /restaurants/{id}/promotions/{promoId}
	mux.HandleFunc("DELETE /restaurants/{id}/promotions/{promoId}", s.auth([]users.Role{users.RoleRestaurant, users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID := r.PathValue("id")
			promoID := r.PathValue("promoId")
			claims := claimsFrom(r)
			if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran aksiyalarini o'zgartirib bo'lmaydi"))
				return
			}
			existing, err := s.PromotionsRepo.GetByID(r.Context(), promoID)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if existing.RestaurantID != restaurantID {
				httpError(w, http.StatusForbidden, errors.New("bu aksiya sizning restoraningizga tegishli emas"))
				return
			}
			if err := s.PromotionsRepo.Delete(r.Context(), promoID); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			s.Hub.Send(restaurantTopic(restaurantID), map[string]any{"type": "promotions_updated"})
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		}))

	// ---------- Auth (ochiq endpoint'lar) ----------

	// POST /auth/request-code  {"phone":"+998901234567"}
}
