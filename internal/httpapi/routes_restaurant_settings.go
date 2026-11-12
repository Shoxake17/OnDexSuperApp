package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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

var restaurantEditableFields = []string{
	"logo_url", "cover_url", "description", "working_hours", "payment_methods",
}

// movedFields — ilgari shu yerda tahrirlangan, endi BOSHQA joyda
// boshqariladigan maydonlar va qayerdaligi.
//
// Oddiy "noma'lum maydon" deb rad etilsa, eski klient yoki panel
// foydalanuvchisi nima bo'lganini tushunmaydi. Aniq matn uni to'g'ri
// joyga yo'naltiradi.
var movedFields = map[string]string{
	"delivery_fee_tiyin": "yetkazib berish haqi endi \"Xodimlar\" bo'limida: " +
		"yetkazib beruvchining maosh usulidan (buyurtmadan maosh yoki oylik) kelib chiqadi",
	"delivery_free_from_tiyin": "bepul yetkazish chegarasi olib tashlangan: yetkazib berish haqi " +
		"endi yetkazib beruvchining maosh usulidan kelib chiqadi (\"Xodimlar\" bo'limi)",
	"multicard_store_id": "to'lov hisobi \"Moliya\" bo'limida ulanadi",
	// Qo'lda kiritish ENDI YO'Q — mijoz buyurtmadan keyin qo'ygan
	// haqiqiy bahodan hisoblanadi (`internal/ratings`). Admin ham,
	// restoran ham o'zgartira olmaydi, shuning uchun bu ikkisi
	// `restaurantLockedFields`dan shu yerga ko'chirildi.
	"rating":       "reyting endi mijoz buyurtmadan keyin qo'ygan haqiqiy bahodan avtomatik hisoblanadi",
	"rating_count": "baholar soni endi mijoz buyurtmadan keyin qo'ygan haqiqiy bahodan avtomatik hisoblanadi",
	// Xuddi reyting kabi — endi HAQIQIY buyurtma tarixidan hisoblanadi
	// (`internal/orders.ComputeETARange`, `scene_access.go` dagi
	// `withETA`): mijoz buyurtma bergan ondan yetkazilgan ongacha.
	"eta_min_minutes": "yetkazish vaqti endi haqiqiy buyurtma tarixidan avtomatik hisoblanadi",
	"eta_max_minutes": "yetkazish vaqti endi haqiqiy buyurtma tarixidan avtomatik hisoblanadi",
}

// restaurantLockedFields — faqat admin o'zgartiradigan maydonlar.
var restaurantLockedFields = map[string]bool{
	"id": true, "name": true, "kind": true, "phone": true, "address": true,
	"lat": true, "lng": true, "tags": true,
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
	// To'lov ulanishi — FAQAT ko'rish uchun. O'zgartirish
	// `POST /restaurants/{id}/finance/connect` orqali (u kalitlarni
	// shlyuzda tekshiradi). `secret` bu yerda ham, boshqa hech qayerda
	// ham qaytarilmaydi.
	MulticardStoreID string   `json:"multicard_store_id"`
	MulticardAppID   string   `json:"multicard_app_id"`
	Timezone         string   `json:"timezone"`
	Editable         []string `json:"editable"`
	Locked           []string `json:"locked"`
}

func (s *Server) restaurantSettings(ctx context.Context, rest *catalog.Restaurant) restaurantSettingsView {
	return restaurantSettingsView{
		ID:             rest.ID,
		Name:           rest.Name,
		Kind:           string(rest.Kind),
		KindTitle:      rest.Kind.Title(),
		Phone:          restaurantPhoneFor(ctx, s.UserRepo, rest.ID),
		Address:        rest.Address,
		LogoURL:        rest.LogoURL,
		CoverURL:       rest.CoverURL,
		Description:    rest.Description,
		DescriptionMax: catalog.MaxDescriptionLen,
		Open:           rest.Open,
		OpenNow:        rest.AcceptingOrdersAt(time.Now()),
		WorkingHours:   rest.WorkingHours,
		PaymentMethods: rest.EffectivePaymentMethods(),
		// Onlayn usullarni YOQISH uchun ikkalasi shart: tizimda to'lov
		// xizmati bor VA shu restoran shlyuzda kassaga ulangan. Ulanmagan
		// restoran Uzum'ni yoqib qo'ysa, mijozga baribir ko'rinmasdi
		// (`PaymentOptions`) va restoran "nega chiqmayapti?" deb qolardi.
		OnlinePaymentsAvailable: s.Payments != nil && rest.Connected(),
		MulticardStoreID:        rest.MulticardStoreID,
		MulticardAppID:          rest.MulticardAppID,
		Timezone:                "Asia/Tashkent",
		Editable:                slices.Clone(restaurantEditableFields),
		Locked:                  []string{"name", "kind", "phone", "address"},
	}
}

// newlyEnabledOnline — `pm` da YANGIDAN yoqilayotgan onlayn usullarni
// tekshiradi. Yoqib bo'lmaydigan bo'lsa (usul nomi, sabab) qaytaradi.
//
// Ikki sabab:
//   - tizimda to'lov xizmati umuman yo'q (`s.Payments == nil`);
//   - restoran shlyuzda kassaga ULANMAGAN (faqat Rahmat muhitida —
//     `s.MC` sozlangan bo'lsa): ulanmagan restoran yoqsa ham mijozga
//     ko'rinmaydi (`PaymentOptions`), restoran esa "nega chiqmayapti?"
//     deb qoladi. Aniq xabar uni Moliya bo'limiga yo'naltiradi.
func (s *Server) newlyEnabledOnline(rest *catalog.Restaurant, pm catalog.PaymentMethods) (string, string) {
	old := rest.EffectivePaymentMethods()
	type method struct {
		title   string
		on, was bool
	}
	for _, m := range []method{
		{"Bank kartasi", pm.CardOnline, old.CardOnline},
		{"Payme", pm.Payme, old.Payme},
		{"Click", pm.Click, old.Click},
		{"Uzum", pm.Uzum, old.Uzum},
	} {
		if !m.on || m.was {
			continue // o'chirilmoqda yoki allaqachon yoqilgan edi
		}
		if s.Payments == nil {
			return m.title, "onlayn to'lov tizimda hali ulanmagan — uni yoqib bo'lmaydi"
		}
		if s.MC.CanConnect() && !rest.Connected() {
			return m.title, "avval \"Moliya\" bo'limida to'lov hisobini ulang — shundan keyin yoqiladi"
		}
	}
	return "", ""
}

// withOpenState — ochiq restoran javobiga "hozir buyurtma qabul qiladimi",
// sababi va holat o'zgaradigan payt qo'shiladi (`Restaurant.WithOpenState`).
// Keshdan KEYIN hisoblanadi: vaqtga bog'liq qiymat keshda eskirmasin.
//
// Shu yerda mijozga ko'rinadigan TO'LOV USULLARI ham qo'shiladi
// (`payment_options`): ochiq javob qaerdan chiqmasin, mijoz ilovasi
// bir xil ma'lumotni oladi — restoran yoqqan VA shlyuzga ulangan usullar.
func withOpenState(v catalog.Restaurant) catalog.Restaurant {
	return v.WithOpenState(time.Now()).WithPaymentOptions()
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
	return hasMediaBase(s, folder)
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
				if where, moved := movedFields[k]; moved {
					httpError(w, http.StatusBadRequest, fmt.Errorf("%q bu yerda o'zgartirilmaydi: %s", k, where))
					return
				}
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
				// Faqat YANGI yoqilayotgan usullar tekshiriladi — oldin
				// yoqilgan, keyin tizim o'chgan bo'lsa, restoran uni
				// baribir o'chira olishi kerak.
				if name, bad := s.newlyEnabledOnline(rest, pm); bad != "" {
					httpError(w, http.StatusBadRequest, fmt.Errorf("%s: %s", name, bad))
					return
				}
				rest.PaymentMethods = &pm
				changed = append(changed, "payment_methods")
			}

			// ┌─ TO'LOV SHLYUZI SOZLAMALAR ORQALI O'ZGARMAYDI ─────────┐
			// `multicard_store_id` ATAYLAB bu yerda emas. Pul manzilini
			// belgilaydigan yagona eshik —
			// `POST /restaurants/{id}/finance/connect`: u kalitlarni va
			// kassani SAQLASHDAN OLDIN shlyuzda tekshiradi va pul
			// tushadigan hisobni javobda ko'rsatadi.
			//
			// Bu yerda ham ruxsat berilsa, tekshirilmagan raqam
			// yozilardi va xato faqat Moliya sahifasi ochilganda
			// bilinardi — aynan shunday bo'lgan edi ("65" kiritildi,
			// xato bir kundan keyin chiqdi).
			// └────────────────────────────────────────────────────────┘

			if len(changed) > 0 {
				if err := s.CatalogRepo.SaveRestaurant(r.Context(), rest); err != nil {
					httpError(w, http.StatusInternalServerError, err)
					return
				}
				s.dropRestaurantsCache(r.Context())
				// Audit: kim, qaysi restoranning, qaysi maydonlarini.
				// Qiymatlarning o'zi yozilmaydi (tavsif uzun, logda kerak emas).
				slog.Info("restoran sozlamalari o'zgartirildi",
					"restaurant", rest.ID, "by", claims.Subject, "role", claims.Role, "fields", changed)
			}
			writeJSON(w, http.StatusOK, s.restaurantSettings(r.Context(), rest))
		}))
}
