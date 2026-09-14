package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chustapp/internal/images"
	"chustapp/internal/support"
	"chustapp/internal/users"
)

// ┌─ QO'LLAB-QUVVATLASH ───────────────────────────────────────────────────┐
// * Aloqa ma'lumotlari: o'qish — istalgan tizimga kirgan foydalanuvchi
//   (barcha panellar), yozish — FAQAT admin.
// * Chat: restoran FAQAT o'z suhbatiga (`EntityID` tokendan) va FAQAT
//   restoran akkaunti — affitsiant chatni ko'rmaydi. Admin suhbatlarga
//   ALOHIDA `/admin/support/*` yo'li orqali kiradi: bitta endpointni ikki
//   rolga ochish admin xabarini "restoran nomidan" yozib qo'yish xavfini
//   tug'dirardi.
// * Rasm: multipart orqali keladi, serverda QAYTA KODLANADI va faqat
//   egalik tekshiriladigan endpointdan, token bilan beriladi.
// └────────────────────────────────────────────────────────────────────────┘

const supportBodyLimit = 16 << 10

var (
	supportRestaurantIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	supportImageExt       = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
)

func supportHTTPError(w http.ResponseWriter, err error) {
	if support.IsValidation(err) {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	httpError(w, http.StatusInternalServerError, err)
}

func (s *Server) hubOnline(key string) bool { return s.Hub != nil && s.Hub.Online(key) }

func decodeSupportJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, supportBodyLimit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		httpError(w, http.StatusBadRequest, errors.New("so'rov noto'g'ri shaklda"))
		return false
	}
	return true
}

func parseSupportQuery(r *http.Request) (support.MessageQuery, error) {
	qs := r.URL.Query()
	before, err := parseSeqParam(qs.Get("before"), "before")
	if err != nil {
		return support.MessageQuery{}, err
	}
	after, err := parseSeqParam(qs.Get("after"), "after")
	if err != nil {
		return support.MessageQuery{}, err
	}
	if before > 0 && after > 0 {
		return support.MessageQuery{}, errors.New("before va after birga berilmaydi")
	}
	limit := 50
	if v := qs.Get("limit"); v != "" {
		limit, err = strconv.Atoi(v)
		if err != nil || limit < 1 || limit > support.MaxLimit {
			return support.MessageQuery{}, errors.New("limit 1 dan 100 gacha bo'lishi kerak")
		}
	}
	return support.MessageQuery{BeforeSeq: before, AfterSeq: after, Limit: limit}, nil
}

// writeSupportMessages — xabarlar (o'sish tartibida) + suhbat xulosasi.
func (s *Server) writeSupportMessages(w http.ResponseWriter, r *http.Request, restaurantID string,
	viewer support.Side, extra map[string]any) {
	q, err := parseSupportQuery(r)
	if err != nil {
		httpError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.SupportSvc.Messages(r.Context(), restaurantID, q)
	if err != nil {
		supportHTTPError(w, err)
		return
	}
	thread, err := s.SupportSvc.Thread(r.Context(), restaurantID)
	if err != nil {
		supportHTTPError(w, err)
		return
	}
	views := make([]support.MessageView, 0, len(items))
	for _, m := range items {
		views = append(views, support.ToMessageView(m))
	}
	var next int64
	if q.AfterSeq == 0 && len(items) == q.Limit {
		next = items[0].Seq
	}
	resp := map[string]any{"items": views, "next_before": next, "thread": support.ToThreadView(thread, viewer)}
	for k, v := range extra {
		resp[k] = v
	}
	writeJSON(w, http.StatusOK, resp)
}

// supportSend — tezlik chegarasi (yuboruvchi akkaunt bo'yicha), qat'iy JSON
// yoki multipart (rasm), takroriy `client_id` — 200 va o'sha xabar.
func (s *Server) supportSend(w http.ResponseWriter, r *http.Request, restaurantID string, author support.Author) {
	if ok, wait := s.supportLimiter.AllowWithWait("support:" + author.UserID); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(wait.Seconds())))))
		httpError(w, http.StatusTooManyRequests, errors.New("juda ko'p xabar yuborildi — biroz kuting"))
		return
	}
	var (
		body, clientID string
		img            *support.ImageInput
	)
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		var ok bool
		body, clientID, img, ok = s.readSupportImage(w, r)
		if !ok {
			return
		}
	} else {
		var req struct {
			Body     string `json:"body"`
			ClientID string `json:"client_id"`
		}
		if !decodeSupportJSON(w, r, &req) {
			return
		}
		body, clientID = req.Body, req.ClientID
	}
	m, thread, created, err := s.SupportSvc.Send(r.Context(), restaurantID, author, body, clientID, img)
	if err != nil {
		supportHTTPError(w, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, map[string]any{
		"message": support.ToMessageView(m),
		"thread":  support.ToThreadView(thread, author.Side),
	})
}

// readSupportImage — multipart: `file` (jpg/png/webp, ≤10 MB), ixtiyoriy
// `body` (izoh) va majburiy `client_id`. Boshqa maydon qabul qilinmaydi.
// Rasm to'liq dekodlanib QAYTA KODLANADI — fayl kengaytmasiga ishonilmaydi.
func (s *Server) readSupportImage(w http.ResponseWriter, r *http.Request) (string, string, *support.ImageInput, bool) {
	if s.UploadLimiter != nil && !s.UploadLimiter.Allow("upload:"+uploadQuotaKey(claimsFrom(r))) {
		httpError(w, http.StatusTooManyRequests, errors.New("juda ko'p rasm yuborildi — biroz kuting"))
		return "", "", nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, support.MaxUploadBytes+64<<10)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		httpError(w, http.StatusBadRequest, errors.New("rasm juda katta (maks 10 MB) yoki so'rov noto'g'ri"))
		return "", "", nil, false
	}
	defer r.MultipartForm.RemoveAll() //nolint:errcheck // vaqtinchalik fayllar

	for k, v := range r.MultipartForm.Value {
		if (k != "body" && k != "client_id") || len(v) != 1 {
			httpError(w, http.StatusBadRequest, errors.New("so'rov noto'g'ri shaklda"))
			return "", "", nil, false
		}
	}
	files := r.MultipartForm.File
	if len(files) != 1 || len(files["file"]) != 1 {
		httpError(w, http.StatusBadRequest, errors.New("bitta rasm fayli kerak (multipart 'file' maydoni)"))
		return "", "", nil, false
	}
	clientID := r.FormValue("client_id")
	if !support.ValidClientID(clientID) {
		httpError(w, http.StatusBadRequest, errors.New("client_id noto'g'ri"))
		return "", "", nil, false
	}
	header := files["file"][0]
	if !supportImageExt[strings.ToLower(filepath.Ext(header.Filename))] || header.Size <= 0 ||
		header.Size > support.MaxUploadBytes {
		httpError(w, http.StatusBadRequest, errors.New("faqat jpg, png yoki webp rasm (maks 10 MB)"))
		return "", "", nil, false
	}
	file, err := header.Open()
	if err != nil {
		httpError(w, http.StatusBadRequest, errors.New("rasmni o'qib bo'lmadi"))
		return "", "", nil, false
	}
	defer file.Close()
	data, width, height, err := images.ProcessChatImage(file)
	if err != nil {
		httpError(w, http.StatusBadRequest, errors.New("fayl haqiqiy rasm emas, buzilgan yoki o'lchami juda katta"))
		return "", "", nil, false
	}
	if len(data) > support.MaxAttachmentBytes {
		httpError(w, http.StatusBadRequest, errors.New("rasm juda katta — kichikroq rasm yuboring"))
		return "", "", nil, false
	}
	return r.FormValue("body"), clientID, &support.ImageInput{Data: data, Width: width, Height: height}, true
}

// writeSupportAttachment — rasm baytlari. Faqat egasi (handler tekshiradi),
// brauzer/klient uni boshqa turdagi kontent deb talqin qilmasin.
func (s *Server) writeSupportAttachment(w http.ResponseWriter, r *http.Request, restaurantID string) {
	a, err := s.SupportSvc.Attachment(r.Context(), restaurantID, r.PathValue("aid"))
	if errors.Is(err, support.ErrAttachmentNotFound) {
		httpError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		httpError(w, http.StatusInternalServerError, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", a.ContentType)
	h.Set("Content-Length", strconv.Itoa(len(a.Data)))
	h.Set("Cache-Control", "private, max-age=86400")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Content-Disposition", `inline; filename="rasm.webp"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(a.Data)
}

func (s *Server) supportMarkRead(w http.ResponseWriter, r *http.Request, restaurantID string, side support.Side) {
	var req struct {
		UpToSeq int64 `json:"up_to_seq"`
	}
	if !decodeSupportJSON(w, r, &req) {
		return
	}
	thread, err := s.SupportSvc.MarkRead(r.Context(), restaurantID, side, req.UpToSeq)
	if err != nil {
		supportHTTPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"thread": support.ToThreadView(thread, side)})
}

func (s *Server) supportUnavailable(w http.ResponseWriter) bool {
	if s.SupportSvc == nil {
		httpError(w, http.StatusServiceUnavailable, errors.New("qo'llab-quvvatlash xizmati sozlanmagan"))
		return true
	}
	return false
}

type supportRestaurantInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	LogoURL string `json:"logo_url"`
}

type adminSupportThread struct {
	support.ThreadView
	Restaurant       supportRestaurantInfo `json:"restaurant"`
	RestaurantOnline bool                  `json:"restaurant_online"`
}

func (s *Server) registerSupportRoutes(mux *http.ServeMux) {
	// GET /support/contacts — barcha panellar (admin kiritgan aloqa ma'lumotlari).
	mux.HandleFunc("GET /support/contacts", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			if s.supportUnavailable(w) {
				return
			}
			c, err := s.SupportSvc.Contacts(r.Context())
			if err != nil {
				supportHTTPError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, support.ToContactsView(c))
		}))

	// PUT /admin/support/contacts  {"phone","phone_hours","telegram","email","email_note"}
	mux.HandleFunc("PUT /admin/support/contacts", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			if s.supportUnavailable(w) {
				return
			}
			var in support.ContactsInput
			if !decodeSupportJSON(w, r, &in) {
				return
			}
			by := claimsFrom(r).Subject
			c, err := s.SupportSvc.UpdateContacts(r.Context(), in, by)
			if err != nil {
				supportHTTPError(w, err)
				return
			}
			slog.Info("qo'llab-quvvatlash aloqa ma'lumotlari yangilandi", "by", by)
			writeJSON(w, http.StatusOK, support.ToContactsView(c))
		}))

	// ── Restoran tomoni ──

	restaurantOnly := []users.Role{users.RoleRestaurant}
	own := func(w http.ResponseWriter, r *http.Request) (supportRestaurantInfo, bool) {
		c := claimsFrom(r)
		restaurantID, ok := entityIDFor(c, r.PathValue("id"))
		if !ok || c.Role != users.RoleRestaurant {
			httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
			return supportRestaurantInfo{}, false
		}
		if s.supportUnavailable(w) {
			return supportRestaurantInfo{}, false
		}
		rest, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID)
		if err != nil {
			httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
			return supportRestaurantInfo{}, false
		}
		return supportRestaurantInfo{ID: restaurantID, Name: rest.Name, Address: rest.Address, LogoURL: rest.LogoURL}, true
	}

	// GET /restaurants/{id}/support/messages?before=&after=&limit=
	mux.HandleFunc("GET /restaurants/{id}/support/messages", s.auth(restaurantOnly,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := own(w, r)
			if !ok {
				return
			}
			s.writeSupportMessages(w, r, rest.ID, support.SideRestaurant,
				map[string]any{"support_online": s.hubOnline(support.AdminTopic())})
		}))

	// GET /restaurants/{id}/support/summary — yon menyu rozetkasi.
	mux.HandleFunc("GET /restaurants/{id}/support/summary", s.auth(restaurantOnly,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := own(w, r)
			if !ok {
				return
			}
			thread, err := s.SupportSvc.Thread(r.Context(), rest.ID)
			if err != nil {
				supportHTTPError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"thread":         support.ToThreadView(thread, support.SideRestaurant),
				"support_online": s.hubOnline(support.AdminTopic()),
			})
		}))

	// POST /restaurants/{id}/support/messages
	//   JSON: {"body":"...","client_id":"..."}
	//   multipart: file + client_id (+ ixtiyoriy body) — rasm
	mux.HandleFunc("POST /restaurants/{id}/support/messages", s.auth(restaurantOnly,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := own(w, r)
			if !ok {
				return
			}
			s.supportSend(w, r, rest.ID, support.Author{
				Side: support.SideRestaurant, UserID: claimsFrom(r).Subject, Name: rest.Name})
		}))

	// GET /restaurants/{id}/support/attachments/{aid} — o'z suhbatidagi rasm.
	mux.HandleFunc("GET /restaurants/{id}/support/attachments/{aid}", s.auth(restaurantOnly,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := own(w, r)
			if !ok {
				return
			}
			s.writeSupportAttachment(w, r, rest.ID)
		}))

	// POST /restaurants/{id}/support/read  {"up_to_seq": 42}
	mux.HandleFunc("POST /restaurants/{id}/support/read", s.auth(restaurantOnly,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := own(w, r)
			if !ok {
				return
			}
			s.supportMarkRead(w, r, rest.ID, support.SideRestaurant)
		}))

	// ── Admin tomoni ──

	adminOnly := []users.Role{users.RoleAdmin}
	thread := func(w http.ResponseWriter, r *http.Request) (supportRestaurantInfo, bool) {
		if s.supportUnavailable(w) {
			return supportRestaurantInfo{}, false
		}
		restaurantID := r.PathValue("rid")
		if !supportRestaurantIDRe.MatchString(restaurantID) {
			httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
			return supportRestaurantInfo{}, false
		}
		rest, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID)
		if err != nil {
			httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
			return supportRestaurantInfo{}, false
		}
		return supportRestaurantInfo{ID: restaurantID, Name: rest.Name, Address: rest.Address, LogoURL: rest.LogoURL}, true
	}

	// GET /admin/support/threads — barcha suhbatlar, oxirgi xabar bo'yicha.
	mux.HandleFunc("GET /admin/support/threads", s.auth(adminOnly,
		func(w http.ResponseWriter, r *http.Request) {
			if s.supportUnavailable(w) {
				return
			}
			threads, err := s.SupportSvc.Threads(r.Context(), support.MaxThreads)
			if err != nil {
				supportHTTPError(w, err)
				return
			}
			restaurants, err := s.CatalogRepo.ListRestaurants(r.Context())
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			byID := make(map[string]supportRestaurantInfo, len(restaurants))
			for _, rest := range restaurants {
				byID[rest.ID] = supportRestaurantInfo{ID: rest.ID, Name: rest.Name, Address: rest.Address, LogoURL: rest.LogoURL}
			}
			items := make([]adminSupportThread, 0, len(threads))
			total := 0
			for _, t := range threads {
				info, found := byID[t.RestaurantID]
				if !found {
					info = supportRestaurantInfo{ID: t.RestaurantID, Name: "O'chirilgan restoran"}
				}
				total += t.UnreadAdmin
				items = append(items, adminSupportThread{
					ThreadView: support.ToThreadView(t, support.SideAdmin), Restaurant: info,
					RestaurantOnline: found && s.hubOnline(support.RestaurantTopic(t.RestaurantID)),
				})
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": items, "unread_total": total})
		}))

	// GET /admin/support/summary — navigatsiya rozetkasi.
	mux.HandleFunc("GET /admin/support/summary", s.auth(adminOnly,
		func(w http.ResponseWriter, r *http.Request) {
			if s.supportUnavailable(w) {
				return
			}
			n, err := s.SupportSvc.AdminUnreadTotal(r.Context())
			if err != nil {
				supportHTTPError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"unread": n})
		}))

	// GET /admin/support/threads/{rid}/messages?before=&after=&limit=
	mux.HandleFunc("GET /admin/support/threads/{rid}/messages", s.auth(adminOnly,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := thread(w, r)
			if !ok {
				return
			}
			s.writeSupportMessages(w, r, rest.ID, support.SideAdmin, map[string]any{
				"restaurant":        rest,
				"restaurant_online": s.hubOnline(support.RestaurantTopic(rest.ID)),
			})
		}))

	// POST /admin/support/threads/{rid}/messages — JSON yoki multipart (rasm).
	mux.HandleFunc("POST /admin/support/threads/{rid}/messages", s.auth(adminOnly,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := thread(w, r)
			if !ok {
				return
			}
			s.supportSend(w, r, rest.ID, support.Author{Side: support.SideAdmin, UserID: claimsFrom(r).Subject})
		}))

	// GET /admin/support/threads/{rid}/attachments/{aid}
	mux.HandleFunc("GET /admin/support/threads/{rid}/attachments/{aid}", s.auth(adminOnly,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := thread(w, r)
			if !ok {
				return
			}
			s.writeSupportAttachment(w, r, rest.ID)
		}))

	// POST /admin/support/threads/{rid}/read  {"up_to_seq": 42}
	mux.HandleFunc("POST /admin/support/threads/{rid}/read", s.auth(adminOnly,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := thread(w, r)
			if !ok {
				return
			}
			s.supportMarkRead(w, r, rest.ID, support.SideAdmin)
		}))
}
