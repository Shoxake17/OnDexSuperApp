package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"chustapp/internal/images"
	"chustapp/internal/qurt"
	"chustapp/internal/ratelimit"
	"chustapp/internal/sso"
	"chustapp/internal/users"
)

// Qurt mahsulotlari uchun limitlar. Ochiq ro'yxat IP bo'yicha; POS (server-server)
// chaqiruvlari klient serveri bo'yicha — u bitta IP, shuning uchun yuqoriroq.
var (
	qurtListLimiter     = ratelimit.New(5, 30)
	qurtMerchantLimiter = ratelimit.New(20, 60)
)

// qurtStores — hozir Qurt kategoriyasidagi FAOL do'konlar: ID -> nomi.
//
// Mijozga FAQAT shularning mahsuloti ko'rinadi: admin do'konni qurtdan boshqa
// kategoriyaga o'tkazsa yoki o'chirsa, mahsulotlar darhol yashirinadi
// (ularni o'chirish shart emas — qaytsa, o'z joyida).
func (s *Server) qurtStores(ctx context.Context) (map[string]string, error) {
	list, err := s.UserRepo.ListByRole(ctx, users.RoleStore)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, u := range list {
		if u == nil || u.EntityID == "" || u.DeletedAt != nil {
			continue
		}
		if u.StoreCategory == users.StoreCategoryQurt {
			out[u.EntityID] = u.Name
		}
	}
	return out, nil
}

func qurtIDs(m map[string]string) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}

// merchantQurtStore — POS serveri qurt do'koni nomidan chaqirganini tekshiradi.
//
// Brauzer BU YERGA kelmaydi: POS serveri klient SIRI bilan (SSO klienti kabi)
// va do'kon egasining OnDex ID'si bilan murojaat qiladi. OnDex egasi haqiqatan
// QURT kategoriyasidagi do'kon ekanini O'ZI tekshiradi (POS yuborgan
// "men qurtman" ga ishonilmaydi): kategoriya o'zgarsa, ruxsat ham darhol yo'qoladi.
//
// Sarlavhalar: X-Client-Id, X-Client-Secret, X-Store-User.
func (s *Server) merchantQurtStore(w http.ResponseWriter, r *http.Request) (*sso.Identity, bool) {
	if s.SSO == nil || s.QurtRepo == nil {
		httpError(w, http.StatusServiceUnavailable, errors.New("qurt xizmati sozlanmagan"))
		return nil, false
	}
	if rateLimited(w, qurtMerchantLimiter, clientIP(r)) {
		return nil, false
	}
	id, err := s.SSO.Lookup(r.Context(),
		r.Header.Get("X-Client-Id"), r.Header.Get("X-Client-Secret"), r.Header.Get("X-Store-User"))
	if err != nil {
		if ssoStatus(err) != http.StatusInternalServerError {
			slog.Warn("qurt: merchant so'rovi rad etildi", "ip", clientIP(r), "err", err)
		}
		httpError(w, ssoStatus(err), err)
		return nil, false
	}
	if id.Role != string(users.RoleStore) || id.StoreCategory != string(users.StoreCategoryQurt) || id.EntityID == "" {
		httpError(w, http.StatusForbidden, errors.New("bu akkaunt qurt do'koni emas"))
		return nil, false
	}
	return id, true
}

func (s *Server) registerQurtRoutes(mux *http.ServeMux) {
	const maxUploadSize = 5 << 20 // 5MB
	allowedImageExt := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

	// ── Mijoz ilovasi (ochiq) ──────────────────────────────────────────

	// GET /qurt/products?category=&q=&limit=&offset= — sotuvdagi qurt mahsulotlari.
	// Ulgurji narx va POS kaliti javobda YO'Q.
	mux.HandleFunc("GET /qurt/products", func(w http.ResponseWriter, r *http.Request) {
		if s.QurtRepo == nil {
			httpError(w, http.StatusServiceUnavailable, errors.New("qurt xizmati sozlanmagan"))
			return
		}
		if rateLimited(w, qurtListLimiter, clientIP(r)) {
			return
		}
		stores, err := s.qurtStores(r.Context())
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 || limit > 100 {
			limit = 50
		}
		offset, _ := strconv.Atoi(q.Get("offset"))
		if offset < 0 || offset > 10_000 {
			offset = 0
		}
		query := strings.TrimSpace(q.Get("q"))
		if len([]rune(query)) > 64 {
			query = string([]rune(query)[:64])
		}
		items, err := s.QurtRepo.List(r.Context(), qurt.Filter{
			StoreIDs: qurtIDs(stores),
			Category: strings.TrimSpace(q.Get("category")),
			Query:    query, Limit: limit, Offset: offset,
		})
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		if items == nil {
			items = []*qurt.Product{}
		}
		for _, p := range items {
			p.StoreName = stores[p.StoreID]
		}
		w.Header().Set("Cache-Control", "public, max-age=15")
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	// GET /qurt/categories — sotuvdagi mahsulotlar kategoriyalari.
	mux.HandleFunc("GET /qurt/categories", func(w http.ResponseWriter, r *http.Request) {
		if s.QurtRepo == nil {
			httpError(w, http.StatusServiceUnavailable, errors.New("qurt xizmati sozlanmagan"))
			return
		}
		if rateLimited(w, qurtListLimiter, clientIP(r)) {
			return
		}
		stores, err := s.qurtStores(r.Context())
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		cats, err := s.QurtRepo.Categories(r.Context(), qurtIDs(stores))
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		if cats == nil {
			cats = []string{}
		}
		w.Header().Set("Cache-Control", "public, max-age=15")
		writeJSON(w, http.StatusOK, map[string]any{"categories": cats})
	})

	// ── POS serveri (server-server, klient siri bilan) ─────────────────

	// PUT /merchant/qurt/products/{external_id} — mahsulotni yaratish/yangilash.
	mux.HandleFunc("PUT /merchant/qurt/products/{external_id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.merchantQurtStore(w, r)
		if !ok {
			return
		}
		var req struct {
			Name           string `json:"name"`
			Description    string `json:"description"`
			Composition    string `json:"composition"`
			Category       string `json:"category"`
			PriceTiyin     int64  `json:"price_tiyin"`
			WholesaleTiyin int64  `json:"wholesale_tiyin"`
			WeightGrams    int    `json:"weight_grams"`
			ImageURL       string `json:"image_url"`
			Available      bool   `json:"available"`
			// Stock — POS qoldig'i (dona). Eski POS yubormaydi: u holda mavjud qoldiq saqlanadi.
			Stock *int `json:"stock"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		p := &qurt.Product{
			StoreID: id.EntityID, ExternalID: r.PathValue("external_id"),
			Name: req.Name, Description: req.Description, Composition: req.Composition,
			Category: req.Category, PriceTiyin: req.PriceTiyin, WholesaleTiyin: req.WholesaleTiyin,
			WeightGrams: req.WeightGrams, ImageURL: req.ImageURL, Available: req.Available,
		}
		if req.Stock != nil {
			p.Stock, p.StockSet = *req.Stock, true
		}
		if err := p.Normalize(); err != nil {
			httpError(w, http.StatusBadRequest, err)
			return
		}
		if !isSafeMediaURL(p.ImageURL) {
			httpError(w, http.StatusBadRequest, errors.New("rasm manzili noto'g'ri"))
			return
		}
		saved, err := s.QurtRepo.Upsert(r.Context(), p)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"id": saved.ID})
	})

	// DELETE /merchant/qurt/products/{external_id}
	mux.HandleFunc("DELETE /merchant/qurt/products/{external_id}", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.merchantQurtStore(w, r)
		if !ok {
			return
		}
		ext := r.PathValue("external_id")
		if !qurt.ValidExternalID(ext) {
			httpError(w, http.StatusBadRequest, errors.New("mahsulot kaliti noto'g'ri"))
			return
		}
		if err := s.QurtRepo.Delete(r.Context(), id.EntityID, ext); err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	})

	// POST /merchant/qurt/uploads — mahsulot rasmi (multipart `file`). Rasm
	// OnDex omboriga (R2/disk) tushadi: mijoz ilovasi shu manzildan oladi.
	mux.HandleFunc("POST /merchant/qurt/uploads", func(w http.ResponseWriter, r *http.Request) {
		id, ok := s.merchantQurtStore(w, r)
		if !ok {
			return
		}
		if s.ImageStore == nil {
			httpError(w, http.StatusServiceUnavailable, errors.New("rasm ombori sozlanmagan"))
			return
		}
		if s.UploadLimiter != nil && !s.UploadLimiter.Allow("upload:qurt:"+id.EntityID) {
			httpError(w, http.StatusTooManyRequests,
				errors.New("juda ko'p yuklash — biroz kuting va qayta urining"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			httpError(w, http.StatusBadRequest, errors.New("fayl juda katta (maks 5MB) yoki formati noto'g'ri"))
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			httpError(w, http.StatusBadRequest, errors.New("fayl topilmadi (multipart 'file' maydoni kerak)"))
			return
		}
		defer file.Close()
		if !allowedImageExt[strings.ToLower(filepath.Ext(header.Filename))] {
			httpError(w, http.StatusBadRequest, errors.New("faqat jpg, png, webp rasm formatlari qabul qilinadi"))
			return
		}
		webpBytes, err := images.ProcessQurtProductImage(file)
		if err != nil {
			httpError(w, http.StatusBadRequest, errors.New("fayl haqiqiy rasm emas yoki buzilgan"))
			return
		}
		key := "qurt/" + NewID() + ".webp"
		url, err := uploadImmutable(r.Context(), s.ImageStore, key, webpBytes, "image/webp")
		if err != nil {
			slog.Error("qurt rasmini yuklashda xato", "err", err)
			httpError(w, http.StatusInternalServerError, errors.New("rasm saqlashda xato"))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"url": url})
	})
}
