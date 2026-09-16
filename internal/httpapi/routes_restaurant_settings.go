package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/users"
)

// ┌─ RESTORAN SOZLAMALARI: KIM NIMANI O'ZGARTIRADI ───────────────────────┐
// Restoran xodimi FAQAT o'z brendi va ish tartibini boshqaradi: logo,
// muqova, tavsif, ish vaqti, to'lov usullari.
//
// Nomi, turi, manzili, joylashuvi va akkaunt telefoni — FAQAT OnDex
// administratori (`POST /admin/restaurants/{id}`): ular shartnoma, mijoz
// ilovasidagi qidiruv, xarita va kuryer taqsimotiga bog'liq. Masalan
// manzilni o'zgartirish kuryerlarni boshqa joyga yuborib qo'yardi.
//
// Qoida SERVERDA: ruxsat etilgan maydonlar ro'yxati ("allowlist"). Yopiq
// maydon so'rovda kelsa 403 — jimgina e'tiborsiz qoldirilmaydi, aks holda
// klient "saqlandi" deb o'ylab qolardi.
// └───────────────────────────────────────────────────────────────────────┘

var restaurantEditableFields = []string{"logo_url", "cover_url", "description", "working_hours", "payment_methods"}

// restaurantLockedFields — faqat admin o'zgartiradigan maydonlar.
var restaurantLockedFields = map[string]bool{
	"id": true, "name": true, "kind": true, "phone": true, "address": true,
	"lat": true, "lng": true, "tags": true,
	"rating": true, "rating_count": true, "eta_min_minutes": true, "eta_max_minutes": true,
	"scene_3d_url": true, "scene_3d_sha256": true, "scene_3d_bytes": true,
}

type restaurantSettingsView struct {
	ID                      string                 `json:"id"`
	Name                    string                 `json:"name"`
	Kind                    string                 `json:"kind"`
	KindTitle               string                 `json:"kind_title"`
	Phone                   string                 `json:"phone"`
	Address                 string                 `json:"address"`
	LogoURL                 string                 `json:"logo_url"`
	CoverURL                string                 `json:"cover_url"`
	Description             string                 `json:"description"`
	DescriptionMax          int                    `json:"description_max"`
	Open                    bool                   `json:"open"`
	OpenNow                 bool                   `json:"open_now"`
	WorkingHours            *catalog.WorkingHours  `json:"working_hours"`
	PaymentMethods          catalog.PaymentMethods `json:"payment_methods"`
	OnlinePaymentsAvailable bool                   `json:"online_payments_available"`
	Timezone                string                 `json:"timezone"`
	Editable                []string               `json:"editable"`
	Locked                  []string               `json:"locked"`
}

func (s *Server) restaurantSettings(ctx context.Context, rest *catalog.Restaurant) restaurantSettingsView {
	return restaurantSettingsView{
		ID:                      rest.ID,
		Name:                    rest.Name,
		Kind:                    string(rest.Kind),
		KindTitle:               rest.Kind.Title(),
		Phone:                   restaurantPhoneFor(ctx, s.UserRepo, rest.ID),
		Address:                 rest.Address,
		LogoURL:                 rest.LogoURL,
		CoverURL:                rest.CoverURL,
		Description:             rest.Description,
		DescriptionMax:          catalog.MaxDescriptionLen,
		Open:                    rest.Open,
		OpenNow:                 rest.AcceptingOrdersAt(time.Now()),
		WorkingHours:            rest.WorkingHours,
		PaymentMethods:          rest.EffectivePaymentMethods(),
		OnlinePaymentsAvailable: s.Payments != nil,
		Timezone:                "Asia/Tashkent",
		Editable:                slices.Clone(restaurantEditableFields),
		Locked:                  []string{"name", "kind", "phone", "address"},
	}
}

// withOpenState — ochiq restoran javobiga "hozir buyurtma qabul qiladimi",
// sababi va holat o'zgaradigan payt qo'shiladi (`Restaurant.WithOpenState`).
// Keshdan KEYIN hisoblanadi: vaqtga bog'liq qiymat keshda eskirmasin.
func withOpenState(v catalog.Restaurant) catalog.Restaurant {
	return v.WithOpenState(time.Now())
}

// isOwnUploadURL — manzil AYNAN bizning yuklash endpointimiz bergan rasm
// (`POST /uploads?type=logo|cover`): lokal `/uploads/<papka>/...webp` yoki
// `R2_PUBLIC_URL/<papka>/...webp`.
//
// Admin tashqi https manzilni ham qo'ya oladi (`isSafeMediaURL`), restoran
// xodimi esa YO'Q: aks holda u mijozlar ilovasiga begona serverdan rasm
// yuklatib, ularning IP manzillarini yig'ishi yoki kontentni istalgan
// payt almashtirishi mumkin edi.
func isOwnUploadURL(raw, folder string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return true // rasmni olib tashlash
	}
	if strings.ContainsAny(s, "?#\\ ") || strings.Contains(s, "..") ||
		!strings.HasSuffix(s, ".webp") {
		return false
	}
	if strings.HasPrefix(s, "/uploads/"+folder+"/") {
		return true
	}
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("R2_PUBLIC_URL")), "/")
	return strings.HasPrefix(base, "https://") && strings.HasPrefix(s, base+"/"+folder+"/")
}

func decodeStrict(raw json.RawMessage, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func (s *Server) registerRestaurantSettingsRoutes(mux *http.ServeMux) {
	staff := []users.Role{users.RoleRestaurant, users.RoleAdmin}

	// load — egalik (HAMMA narsadan oldin), keyin restoran.
	load := func(w http.ResponseWriter, r *http.Request) (*catalog.Restaurant, bool) {
		id := r.PathValue("id")
		claims := claimsFrom(r)
		if claims.Role == users.RoleRestaurant && claims.EntityID != id {
			httpError(w, http.StatusForbidden, errors.New("boshqa restoran sozlamalarini ko'rib bo'lmaydi"))
			return nil, false
		}
		rest, err := s.CatalogRepo.GetRestaurant(r.Context(), id)
		if err != nil {
			if errors.Is(err, catalog.ErrNotFound) {
				httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
			} else {
				httpError(w, http.StatusInternalServerError, err)
			}
			return nil, false
		}
		return rest, true
	}

	// GET /restaurants/{id}/settings — "Restoran sozlamalari" sahifasi.
	mux.HandleFunc("GET /restaurants/{id}/settings", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := load(w, r)
			if !ok {
				return
			}
			writeJSON(w, http.StatusOK, s.restaurantSettings(r.Context(), rest))
		}))

	// PATCH /restaurants/{id}/settings — faqat `restaurantEditableFields`.
	//
	//	{"description":"...","logo_url":"/uploads/logos/x.webp",
	//	 "working_hours":{"days":[{"day":1,"enabled":true,"open":"08:00","close":"23:00"},...]},
	//	 "payment_methods":{"cash":true,"card_terminal":true,"card_online":false}}
	//
	// Hamma o'zgarish tekshirilgach BIRDANIGA saqlanadi: bitta maydon xato
	// bo'lsa hech narsa yozilmaydi.
	mux.HandleFunc("PATCH /restaurants/{id}/settings", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			claims := claimsFrom(r)
			id := r.PathValue("id")
			if claims.Role == users.RoleRestaurant && claims.EntityID != id {
				httpError(w, http.StatusForbidden, errors.New("boshqa restoran sozlamalarini o'zgartirib bo'lmaydi"))
				return
			}
			var raw map[string]json.RawMessage
			if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("so'rov JSON obyekt bo'lishi kerak"))
				return
			}
			if len(raw) == 0 {
				httpError(w, http.StatusBadRequest, errors.New("o'zgartiriladigan maydon yo'q"))
				return
			}
			keys := make([]string, 0, len(raw))
			for k := range raw {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if restaurantLockedFields[k] {
					httpError(w, http.StatusForbidden,
						fmt.Errorf("%q maydonini faqat OnDex administratori o'zgartira oladi", k))
					return
				}
			}
			for _, k := range keys {
				if !slices.Contains(restaurantEditableFields, k) {
					httpError(w, http.StatusBadRequest, fmt.Errorf("noma'lum maydon: %q", k))
					return
				}
			}

			rest, ok := load(w, r)
			if !ok {
				return
			}
			var changed []string

			if v, ok := raw["description"]; ok {
				var text string
				if err := json.Unmarshal(v, &text); err != nil {
					httpError(w, http.StatusBadRequest, errors.New("description matn bo'lishi kerak"))
					return
				}
				norm, err := catalog.NormalizeDescription(text)
				if err != nil {
					httpError(w, http.StatusBadRequest, err)
					return
				}
				if norm != rest.Description {
					rest.Description = norm
					changed = append(changed, "description")
				}
			}

			for _, f := range []struct {
				key, folder string
				target      *string
			}{
				{"logo_url", "logos", &rest.LogoURL},
				{"cover_url", "covers", &rest.CoverURL},
			} {
				v, ok := raw[f.key]
				if !ok {
					continue
				}
				var u string
				if err := json.Unmarshal(v, &u); err != nil {
					httpError(w, http.StatusBadRequest, fmt.Errorf("%s matn bo'lishi kerak", f.key))
					return
				}
				u = strings.TrimSpace(u)
				if u == *f.target {
					continue
				}
				if !isOwnUploadURL(u, f.folder) {
					httpError(w, http.StatusBadRequest,
						fmt.Errorf("%s — faqat OnDex'ga yuklangan rasm bo'lishi mumkin", f.key))
					return
				}
				*f.target = u
				changed = append(changed, f.key)
			}

			if v, ok := raw["working_hours"]; ok {
				if string(bytes.TrimSpace(v)) == "null" {
					if rest.WorkingHours != nil {
						rest.WorkingHours = nil
						changed = append(changed, "working_hours")
					}
				} else {
					var wh catalog.WorkingHours
					if err := decodeStrict(v, &wh); err != nil {
						httpError(w, http.StatusBadRequest, errors.New("working_hours noto'g'ri shaklda"))
						return
					}
					norm, err := wh.Validate()
					if err != nil {
						httpError(w, http.StatusBadRequest, err)
						return
					}
					rest.WorkingHours = &norm
					changed = append(changed, "working_hours")
				}
			}

			if v, ok := raw["payment_methods"]; ok {
				var pm catalog.PaymentMethods
				if err := decodeStrict(v, &pm); err != nil {
					httpError(w, http.StatusBadRequest, errors.New("payment_methods noto'g'ri shaklda"))
					return
				}
				// OnDex Wallet — platforma qoidasi: o'chirishga urinish jimgina
				// e'tiborsiz qoldirilmaydi, aniq rad etiladi.
				var wallet struct {
					On *bool `json:"ondex_wallet"`
				}
				if json.Unmarshal(v, &wallet) == nil && wallet.On != nil && !*wallet.On {
					httpError(w, http.StatusForbidden, catalog.ErrWalletAlwaysOn)
					return
				}
				pm.OnDexWallet = true
				if err := pm.Validate(); err != nil {
					httpError(w, http.StatusBadRequest, err)
					return
				}
				// Ulanmagan to'lov tizimini YANGIDAN yoqib bo'lmaydi: mijoz
				// uni tanlab, har safar "mavjud emas" xatosini olardi.
				if pm.CardOnline && s.Payments == nil && !rest.EffectivePaymentMethods().CardOnline {
					httpError(w, http.StatusBadRequest,
						errors.New("onlayn karta to'lovi tizimda hali ulanmagan — uni yoqib bo'lmaydi"))
					return
				}
				rest.PaymentMethods = &pm
				changed = append(changed, "payment_methods")
			}

			if len(changed) > 0 {
				if err := s.CatalogRepo.SaveRestaurant(r.Context(), rest); err != nil {
					httpError(w, http.StatusInternalServerError, err)
					return
				}
				s.Cache.Del(r.Context(), restaurantsCacheKey)
				// Audit: kim, qaysi restoranning, qaysi maydonlarini.
				// Qiymatlarning o'zi yozilmaydi (tavsif uzun, logda kerak emas).
				slog.Info("restoran sozlamalari o'zgartirildi",
					"restaurant", rest.ID, "by", claims.Subject, "role", claims.Role, "fields", changed)
			}
			writeJSON(w, http.StatusOK, s.restaurantSettings(r.Context(), rest))
		}))
}
