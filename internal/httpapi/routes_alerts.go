package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chustapp/internal/alerts"
	"chustapp/internal/users"
)

// ┌─ "BILDIRISHNOMALAR" MARKAZI ───────────────────────────────────────────┐
// Restoran FAQAT o'z bildirishnomalarini ko'radi va o'qilgan qiladi
// (`EntityID` tokendan). Affitsiant, kuryer, mijoz kira olmaydi — jonli
// kanal ham faqat restoran akkauntiga ochiladi (`managerTopicIfRestaurant`).
// Platforma yangilanishini faqat admin yuboradi.
// └────────────────────────────────────────────────────────────────────────┘

// managerTopicIfRestaurant — rahbariyat kanaliga obuna FAQAT restoran
// akkauntiga. Affitsiant bir xil `EntityID` ga ega, lekin to'lov
// summalari va xodimlar o'zgarishini ko'rmasligi kerak.
func managerTopicIfRestaurant(role, entityID string) []string {
	if role != string(users.RoleRestaurant) || entityID == "" {
		return nil
	}
	return []string{alerts.Topic(entityID)}
}

const alertsBodyLimit = 16 << 10

var alertIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func alertsHTTPError(w http.ResponseWriter, err error) {
	if alerts.IsValidation(err) {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	httpError(w, http.StatusInternalServerError, err)
}

func parseSeqParam(v, name string) (int64, error) {
	if v == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return 0, errors.New(name + " noto'g'ri")
	}
	return n, nil
}

func (s *Server) registerAlertRoutes(mux *http.ServeMux) {
	roles := []users.Role{users.RoleRestaurant, users.RoleAdmin}

	// own — egalik (HAMMA narsadan oldin), xizmat, restoran mavjudligi.
	own := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		restaurantID, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
		if !ok {
			httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
			return "", false
		}
		if s.AlertsSvc == nil {
			httpError(w, http.StatusServiceUnavailable, errors.New("bildirishnomalar xizmati sozlanmagan"))
			return "", false
		}
		if _, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID); err != nil {
			httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
			return "", false
		}
		return restaurantID, true
	}

	// GET /restaurants/{id}/notifications
	//   ?category=new&period=today&q=to'lov&before=120&limit=30
	//   ?after=118 — jonli kanal uzilganda o'tkazib yuborilganlar (o'sish tartibida).
	mux.HandleFunc("GET /restaurants/{id}/notifications", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := own(w, r)
			if !ok {
				return
			}
			qs := r.URL.Query()
			category, err := alerts.ParseCategory(qs.Get("category"))
			if err != nil {
				alertsHTTPError(w, err)
				return
			}
			since, err := alerts.PeriodSince(qs.Get("period"), time.Now())
			if err != nil {
				alertsHTTPError(w, err)
				return
			}
			before, err := parseSeqParam(qs.Get("before"), "before")
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			after, err := parseSeqParam(qs.Get("after"), "after")
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			limit := 30
			if v := qs.Get("limit"); v != "" {
				limit, err = strconv.Atoi(v)
				if err != nil || limit < 1 || limit > alerts.MaxLimit {
					httpError(w, http.StatusBadRequest, errors.New("limit 1 dan 100 gacha bo'lishi kerak"))
					return
				}
			}
			items, err := s.AlertsSvc.List(r.Context(), restaurantID, alerts.Query{
				Category: category, Since: since, Search: qs.Get("q"),
				BeforeSeq: before, AfterSeq: after, UnreadOnly: qs.Get("unread") == "1", Limit: limit,
			})
			if err != nil {
				alertsHTTPError(w, err)
				return
			}
			counts, err := s.AlertsSvc.Counts(r.Context(), restaurantID, since)
			if err != nil {
				alertsHTTPError(w, err)
				return
			}
			unread, latest, err := s.AlertsSvc.Summary(r.Context(), restaurantID)
			if err != nil {
				alertsHTTPError(w, err)
				return
			}
			views := make([]alerts.View, 0, len(items))
			for _, n := range items {
				views = append(views, alerts.ToView(n))
			}
			var next int64
			if after == 0 && len(items) == limit {
				next = items[len(items)-1].Seq
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"items":       views,
				"next_before": next,
				"counts":      counts,
				"unread":      unread,
				"latest_seq":  latest,
				"categories":  alerts.Categories(),
			})
		}))

	// GET /restaurants/{id}/notifications/summary — qo'ng'iroq belgisi.
	mux.HandleFunc("GET /restaurants/{id}/notifications/summary", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := own(w, r)
			if !ok {
				return
			}
			unread, latest, err := s.AlertsSvc.Summary(r.Context(), restaurantID)
			if err != nil {
				alertsHTTPError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"unread": unread, "latest_seq": latest})
		}))

	// POST /restaurants/{id}/notifications/{nid}/read
	mux.HandleFunc("POST /restaurants/{id}/notifications/{nid}/read", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := own(w, r)
			if !ok {
				return
			}
			id := r.PathValue("nid")
			if !alertIDRe.MatchString(id) {
				httpError(w, http.StatusNotFound, errors.New("bildirishnoma topilmadi"))
				return
			}
			unread, err := s.AlertsSvc.MarkRead(r.Context(), restaurantID, id)
			if err != nil {
				alertsHTTPError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"unread": unread})
		}))

	// POST /restaurants/{id}/notifications/read-all  {"up_to_seq": 120}
	mux.HandleFunc("POST /restaurants/{id}/notifications/read-all", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			restaurantID, ok := own(w, r)
			if !ok {
				return
			}
			var req struct {
				UpToSeq int64 `json:"up_to_seq"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, alertsBodyLimit))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("so'rov noto'g'ri shaklda"))
				return
			}
			updated, unread, err := s.AlertsSvc.MarkAllRead(r.Context(), restaurantID, req.UpToSeq)
			if err != nil {
				alertsHTTPError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"updated": updated, "unread": unread})
		}))

	// POST /admin/restaurant-notifications  {"restaurant_id":"", "title":"...", "body":"..."}
	// Platforma yangilanishi: `restaurant_id` bo'sh — barcha restoranlarga.
	mux.HandleFunc("POST /admin/restaurant-notifications", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			if s.AlertsSvc == nil {
				httpError(w, http.StatusServiceUnavailable, errors.New("bildirishnomalar xizmati sozlanmagan"))
				return
			}
			var req struct {
				RestaurantID string `json:"restaurant_id"`
				Title        string `json:"title"`
				Body         string `json:"body"`
			}
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, alertsBodyLimit))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("so'rov noto'g'ri shaklda"))
				return
			}
			title, err := alerts.ValidateText(req.Title, alerts.MaxTitleLen, true, "Sarlavha")
			if err != nil {
				alertsHTTPError(w, err)
				return
			}
			body, err := alerts.ValidateText(req.Body, alerts.MaxBodyLen, false, "Matn")
			if err != nil {
				alertsHTTPError(w, err)
				return
			}
			var targets []string
			if id := strings.TrimSpace(req.RestaurantID); id != "" {
				if _, err := s.CatalogRepo.GetRestaurant(r.Context(), id); err != nil {
					httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
					return
				}
				targets = []string{id}
			} else {
				list, err := s.CatalogRepo.ListRestaurants(r.Context())
				if err != nil {
					httpError(w, http.StatusInternalServerError, err)
					return
				}
				for _, rest := range list {
					targets = append(targets, rest.ID)
				}
			}
			broadcastID := NewID()
			sent := 0
			for _, rid := range targets {
				n, err := s.AlertsSvc.Publish(r.Context(), rid, alerts.Input{
					Kind: alerts.KindPlatformUpdate, Category: alerts.CategoryUpdate,
					Title: title, Body: body, DedupeKey: "platform_update:" + broadcastID,
				})
				if err != nil {
					slog.Error("platforma yangilanishi yuborilmadi", "restaurant", rid, "err", err)
					continue
				}
				if n != nil {
					sent++
				}
			}
			slog.Info("platforma yangilanishi yuborildi", "broadcast", broadcastID, "sent", sent,
				"by", claimsFrom(r).Subject)
			writeJSON(w, http.StatusCreated, map[string]any{"sent": sent})
		}))
}
