package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"chustapp/internal/modules/shop"
	"chustapp/internal/qurt"
	"chustapp/internal/ratelimit"
	"chustapp/internal/users"
)

// ┌─ ONDEX POS — DO'KON / QURT / DORIXONA (POS-1, ShoxPos'dan 1:1) ───────┐
// Hamma endpoint `/merchant/shop/...`; do'kon SESSIYADAN olinadi
// (`users.entity_id`), URL'da do'kon/login YO'Q — ShoxPos'dagi
// `/:login` IDOR sinfi umuman mavjud emas. Kategoriya (qurt/store/pharmacy)
// HAR so'rovda bazadan qayta o'qiladi: admin kategoriyani o'zgartirsa
// ruxsat darhol o'zgaradi.
// Pul — tiyin (int64), miqdor — 3 xonali kasr.
// └──────────────────────────────────────────────────────────────────────┘

var shopLimiter = ratelimit.New(20, 120).Shared("shopLimiter")

func shopStatus(err error) int {
	switch {
	case errors.Is(err, shop.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, shop.ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, shop.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, shop.ErrInsufficient), errors.Is(err, shop.ErrConflict):
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

func shopFail(w http.ResponseWriter, err error) {
	code := shopStatus(err)
	switch code {
	case http.StatusInternalServerError:
		slog.Error("shop", "err", err)
		err = errors.New("server xatosi")
	case http.StatusForbidden:
		err = errors.New("bu bo'lim do'koningiz turiga ochiq emas")
	}
	httpError(w, code, err)
}

// shopStore — sessiya egasining do'koni. Faol, `store` rolli va do'konga
// bog'langan akkaunt bo'lishi SHART.
func (s *Server) shopStore(w http.ResponseWriter, r *http.Request) (shop.Store, bool) {
	if s.ShopSvc == nil || s.UserRepo == nil {
		httpError(w, http.StatusServiceUnavailable, errors.New("POS xizmati sozlanmagan"))
		return shop.Store{}, false
	}
	c := claimsFrom(r)
	if rateLimited(w, shopLimiter, c.Subject) {
		return shop.Store{}, false
	}
	u, err := s.UserRepo.GetByID(r.Context(), c.Subject)
	if err != nil || u.DeletedAt != nil || u.Role != users.RoleStore || u.EntityID == "" {
		httpError(w, http.StatusForbidden, errors.New("bu akkaunt do'konga bog'lanmagan"))
		return shop.Store{}, false
	}
	cat, err := users.ParseStoreCategory(string(u.StoreCategory))
	if err != nil {
		httpError(w, http.StatusForbidden, errors.New("do'kon turi belgilanmagan — administratorga murojaat qiling"))
		return shop.Store{}, false
	}
	return shop.Store{ID: u.EntityID, Category: string(cat)}, true
}

// shopRoute — `auth(store)` + do'konni aniqlash.
func (s *Server) shopRoute(h func(w http.ResponseWriter, r *http.Request, st shop.Store)) http.HandlerFunc {
	return s.auth([]users.Role{users.RoleStore}, func(w http.ResponseWriter, r *http.Request) {
		st, ok := s.shopStore(w, r)
		if !ok {
			return
		}
		h(w, r, st)
	})
}

func parseShopRange(r *http.Request) (from, to *time.Time, err error) {
	q := r.URL.Query()
	parse := func(v string, end bool) (*time.Time, error) {
		if v == "" {
			return nil, nil
		}
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return &t, nil
		}
		d, err := time.ParseInLocation("2006-01-02", v, tashkentLoc())
		if err != nil {
			return nil, fmt.Errorf("%w: sana formati noto'g'ri (YYYY-MM-DD)", shop.ErrInvalid)
		}
		if end {
			d = d.Add(24*time.Hour - time.Nanosecond)
		}
		return &d, nil
	}
	if from, err = parse(q.Get("from"), false); err != nil {
		return nil, nil, err
	}
	to, err = parse(q.Get("to"), true)
	return from, to, err
}

var tzOnce sync.Once
var tzLoc *time.Location

func tashkentLoc() *time.Location {
	tzOnce.Do(func() {
		l, err := time.LoadLocation("Asia/Tashkent")
		if err != nil {
			l = time.FixedZone("UZT", 5*3600)
		}
		tzLoc = l
	})
	return tzLoc
}

func (s *Server) registerShopRoutes(mux *http.ServeMux) {
	// GET /merchant/shop/me — do'kon turi va ochiq bo'limlar (sidebar shu bo'yicha).
	mux.HandleFunc("GET /merchant/shop/me", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		sections := []string{}
		for _, sec := range []shop.Section{shop.SecProducts, shop.SecSales, shop.SecDebts, shop.SecPharmacy} {
			if st.Allowed(sec) {
				sections = append(sections, string(sec))
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"store_id": st.ID, "category": st.Category,
			"shop_type": shop.ShopType(st.Category), "sections": sections})
	}))

	s.registerShopProductRoutes(mux)
	s.registerShopSalesRoutes(mux)
	s.registerShopPharmRoutes(mux)
}

func (s *Server) registerShopProductRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /merchant/shop/products", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		list, err := s.ShopSvc.Products(r.Context(), st)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	mux.HandleFunc("GET /merchant/shop/products/incoming", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		list, err := s.ShopSvc.Incoming(r.Context(), st)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	mux.HandleFunc("GET /merchant/shop/products/search", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		q := r.URL.Query()
		list, err := s.ShopSvc.Search(r.Context(), st, q.Get("q"), q.Get("category"))
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	// GET /merchant/shop/products/scan?barcode=|name= — avval do'kon, keyin global.
	mux.HandleFunc("GET /merchant/shop/products/scan", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		q := r.URL.Query()
		res, err := s.ShopSvc.Scan(r.Context(), st, q.Get("barcode"), q.Get("name"))
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, res)
	}))

	// GET /merchant/shop/products/lookup?barcode= — OpenFoodFacts (ShoxPos 1:1).
	mux.HandleFunc("GET /merchant/shop/products/lookup", s.shopRoute(func(w http.ResponseWriter, r *http.Request, _ shop.Store) {
		bc := strings.TrimSpace(r.URL.Query().Get("barcode"))
		if !offBarcodeRe.MatchString(bc) {
			httpError(w, http.StatusBadRequest, errors.New("shtrix-kod formati noto'g'ri"))
			return
		}
		lookup := s.ProductLookup
		if lookup == nil {
			lookup = lookupOpenFoodFacts
		}
		name, err := lookup(r.Context(), bc)
		if err != nil {
			httpError(w, http.StatusBadGateway, errors.New("tashqi baza javob bermadi"))
			return
		}
		if name == "" {
			writeJSON(w, http.StatusOK, map[string]string{"source": "none", "barcode": bc, "message": "Mahsulot topilmadi"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"source": "OpenFoodFacts", "barcode": bc, "name": name})
	}))

	mux.HandleFunc("GET /merchant/shop/products/base-count", s.shopRoute(func(w http.ResponseWriter, r *http.Request, _ shop.Store) {
		n, err := s.ShopSvc.BaseCount(r.Context())
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"count": n})
	}))

	mux.HandleFunc("GET /merchant/shop/categories", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		list, err := s.ShopSvc.Categories(r.Context(), st)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	// PUT /merchant/shop/products/{barcode} — qo'shish yoki tahrirlash.
	mux.HandleFunc("PUT /merchant/shop/products/{barcode}", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		var req struct {
			Name           string  `json:"name"`
			Category       string  `json:"category"`
			PriceTiyin     int64   `json:"price_tiyin"`
			BasePriceTiyin int64   `json:"base_price_tiyin"`
			Quantity       float64 `json:"quantity"`
			WeightGrams    *int    `json:"weight_grams"`
			Description    *string `json:"description"`
			Composition    *string `json:"composition"`
			ImageURL       *string `json:"image_url"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.ImageURL != nil && *req.ImageURL != "" && !isSafeMediaURL(*req.ImageURL) {
			httpError(w, http.StatusBadRequest, errors.New("rasm manzili noto'g'ri"))
			return
		}
		p, err := s.ShopSvc.Upsert(r.Context(), st, shop.UpsertInput{Barcode: r.PathValue("barcode"), Name: req.Name,
			Category: req.Category, PriceTiyin: req.PriceTiyin, BasePriceTiyin: req.BasePriceTiyin, Quantity: req.Quantity,
			WeightGrams: req.WeightGrams, Description: req.Description, Composition: req.Composition, ImageURL: req.ImageURL})
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}))

	// PATCH /merchant/shop/products/{barcode} {field: price|base_price|quantity, value}
	mux.HandleFunc("PATCH /merchant/shop/products/{barcode}", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		var req struct {
			Field string  `json:"field"`
			Value float64 `json:"value"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		f := shop.Field(req.Field)
		if f != shop.FieldPrice && f != shop.FieldBasePrice && f != shop.FieldQuantity {
			httpError(w, http.StatusBadRequest, errors.New("field: price, base_price yoki quantity"))
			return
		}
		p, err := s.ShopSvc.UpdateField(r.Context(), st, r.PathValue("barcode"), f, req.Value)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}))

	mux.HandleFunc("DELETE /merchant/shop/products/{barcode}", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		if err := s.ShopSvc.Delete(r.Context(), st, r.PathValue("barcode")); err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	}))
}

func (s *Server) registerShopSalesRoutes(mux *http.ServeMux) {
	// POST /merchant/shop/sales {items: [...], payment_method}
	mux.HandleFunc("POST /merchant/shop/sales", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		var req struct {
			Items         []shop.SellLine `json:"items"`
			PaymentMethod string          `json:"payment_method"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		saved, err := s.ShopSvc.Sell(r.Context(), st, req.Items, req.PaymentMethod)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"sales": saved})
	}))

	mux.HandleFunc("GET /merchant/shop/sales", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		from, to, err := parseShopRange(r)
		if err != nil {
			shopFail(w, err)
			return
		}
		list, err := s.ShopSvc.Sales(r.Context(), st, from, to)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	mux.HandleFunc("DELETE /merchant/shop/sales/{id}", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		if err := s.ShopSvc.DeleteSale(r.Context(), st, r.PathValue("id")); err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	}))

	// GET /merchant/shop/sold-lines?from=&to= — "Sotilganlar" (30 kun).
	mux.HandleFunc("GET /merchant/shop/sold-lines", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		from, to, err := parseShopRange(r)
		if err != nil {
			shopFail(w, err)
			return
		}
		list, err := s.ShopSvc.SoldLines(r.Context(), st, from, to)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	mux.HandleFunc("GET /merchant/shop/debts", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		list, err := s.ShopSvc.Debts(r.Context(), st)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	mux.HandleFunc("POST /merchant/shop/debts", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		var req struct {
			Name        string `json:"name"`
			Phone       string `json:"phone"`
			AmountTiyin int64  `json:"amount_tiyin"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		d, err := s.ShopSvc.AddDebt(r.Context(), st, req.Name, req.Phone, req.AmountTiyin)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, d)
	}))

	mux.HandleFunc("POST /merchant/shop/debts/{id}/pay", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		var req struct {
			PaidTiyin int64 `json:"paid_tiyin"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		full, d, err := s.ShopSvc.PayDebt(r.Context(), st, r.PathValue("id"), req.PaidTiyin)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"fully_paid": full, "debt": d})
	}))
}

func (s *Server) registerShopPharmRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /merchant/shop/pharm", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		list, err := s.ShopSvc.PharmProducts(r.Context(), st)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	mux.HandleFunc("GET /merchant/shop/pharm/search", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		q := r.URL.Query()
		list, err := s.ShopSvc.PharmSearch(r.Context(), st, q.Get("q"), q.Get("category"))
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	mux.HandleFunc("GET /merchant/shop/pharm/info", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		p, err := s.ShopSvc.PharmInfo(r.Context(), st, r.URL.Query().Get("q"))
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}))

	mux.HandleFunc("GET /merchant/shop/pharm/base", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		list, err := s.ShopSvc.PharmBaseList(r.Context(), st)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	mux.HandleFunc("GET /merchant/shop/pharm/base-count", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		n, err := s.ShopSvc.PharmBaseCount(r.Context(), st)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]int{"count": n})
	}))

	mux.HandleFunc("POST /merchant/shop/pharm/base", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		var req shop.PharmBase
		if !decodeJSON(w, r, &req) {
			return
		}
		b, err := s.ShopSvc.AddPharmBase(r.Context(), st, req)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, b)
	}))

	// POST /merchant/shop/pharm — dori qo'shish (qoldiq va kirim += miqdor).
	mux.HandleFunc("POST /merchant/shop/pharm", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		var req struct {
			Barcode              string  `json:"barcode"`
			Name                 string  `json:"name"`
			Category             string  `json:"category"`
			PriceTiyin           int64   `json:"price_tiyin"`
			BasePriceTiyin       int64   `json:"base_price_tiyin"`
			Quantity             float64 `json:"quantity"`
			Dosage               string  `json:"dosage"`
			Manufacturer         string  `json:"manufacturer"`
			ExpiryDate           string  `json:"expiry_date"`
			BatchNumber          string  `json:"batch_number"`
			PrescriptionRequired bool    `json:"prescription_required"`
			StorageCondition     string  `json:"storage_condition"`
			Description          string  `json:"description"`
			DosageForm           string  `json:"dosage_form"`
			ShelfNumber          string  `json:"shelf_number"`
			RowNumber            string  `json:"row_number"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		exp, err := parseExpiry(req.ExpiryDate)
		if err != nil {
			shopFail(w, err)
			return
		}
		p, err := s.ShopSvc.AddPharm(r.Context(), st, shop.PharmAddInput{Barcode: req.Barcode, Name: req.Name,
			Category: req.Category, PriceTiyin: req.PriceTiyin, BasePriceTiyin: req.BasePriceTiyin, Quantity: req.Quantity,
			Dosage: req.Dosage, Manufacturer: req.Manufacturer, BatchNumber: req.BatchNumber,
			StorageCondition: req.StorageCondition, Description: req.Description, DosageForm: req.DosageForm,
			ShelfNumber: req.ShelfNumber, RowNumber: req.RowNumber, ExpiryDate: exp,
			PrescriptionRequired: req.PrescriptionRequired})
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}))

	// PATCH /merchant/shop/pharm/{barcode} — faqat yuborilgan maydonlar o'zgaradi.
	mux.HandleFunc("PATCH /merchant/shop/pharm/{barcode}", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		var req struct {
			Name           *string         `json:"name"`
			Category       *string         `json:"category"`
			Dosage         *string         `json:"dosage"`
			Manufacturer   *string         `json:"manufacturer"`
			ShelfNumber    *string         `json:"shelf_number"`
			RowNumber      *string         `json:"row_number"`
			Description    *string         `json:"description"`
			PriceTiyin     *int64          `json:"price_tiyin"`
			BasePriceTiyin *int64          `json:"base_price_tiyin"`
			Quantity       *float64        `json:"quantity"`
			ExpiryDate     json.RawMessage `json:"expiry_date"` // yo'q = o'zgarmaydi, null = tozalash
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		u := shop.PharmInfoUpdate{Name: req.Name, Category: req.Category, Dosage: req.Dosage,
			Manufacturer: req.Manufacturer, ShelfNumber: req.ShelfNumber, RowNumber: req.RowNumber,
			Description: req.Description, PriceTiyin: req.PriceTiyin, BasePriceTiyin: req.BasePriceTiyin, Quantity: req.Quantity}
		if len(req.ExpiryDate) > 0 {
			var v *string
			if err := json.Unmarshal(req.ExpiryDate, &v); err != nil {
				httpError(w, http.StatusBadRequest, errors.New("expiry_date: YYYY-MM-DD yoki null"))
				return
			}
			if v == nil || *v == "" {
				u.ClearExpiry = true
			} else {
				exp, err := parseExpiry(*v)
				if err != nil {
					shopFail(w, err)
					return
				}
				u.ExpiryDate = exp
			}
		}
		p, err := s.ShopSvc.UpdatePharmInfo(r.Context(), st, r.PathValue("barcode"), u)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, p)
	}))

	mux.HandleFunc("DELETE /merchant/shop/pharm/{barcode}", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		if err := s.ShopSvc.DeletePharm(r.Context(), st, r.PathValue("barcode")); err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	}))

	mux.HandleFunc("POST /merchant/shop/pharm/sales", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		var req struct {
			Items         []shop.PharmSellLine `json:"items"`
			PaymentMethod string               `json:"payment_method"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		sale, err := s.ShopSvc.SellPharm(r.Context(), st, req.Items, req.PaymentMethod)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, sale)
	}))

	mux.HandleFunc("GET /merchant/shop/pharm/sales", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		list, err := s.ShopSvc.PharmSales(r.Context(), st)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, nonNilSlice(list))
	}))

	mux.HandleFunc("GET /merchant/shop/pharm/stats", s.shopRoute(func(w http.ResponseWriter, r *http.Request, st shop.Store) {
		n, sum, err := s.ShopSvc.PharmStats(r.Context(), st)
		if err != nil {
			shopFail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"sales_count": n, "total_tiyin": sum})
	}))
}

func parseExpiry(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, fmt.Errorf("%w: yaroqlilik sanasi YYYY-MM-DD bo'lishi kerak", shop.ErrInvalid)
	}
	return &t, nil
}

func nonNilSlice[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}

// ── OpenFoodFacts (ShoxPos /products/lookup) ──

var offBarcodeRe = regexp.MustCompile(`^[0-9A-Za-z]{4,32}$`)

var offClient = &http.Client{Timeout: 5 * time.Second,
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// lookupOpenFoodFacts — mahsulot nomi; topilmasa "". Shtrix-kod regex bilan
// cheklangan, manzil qat'iy — SSRF yo'q.
func lookupOpenFoodFacts(ctx context.Context, barcode string) (string, error) {
	if !offBarcodeRe.MatchString(barcode) {
		return "", errors.New("shtrix-kod formati noto'g'ri")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, //nolint:gosec // G704 emas: host qat'iy, shtrix-kod [0-9A-Za-z]{4,32}
		"https://world.openfoodfacts.org/api/v0/product/"+barcode+".json", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "OnDexPOS/1.0")
	res, err := offClient.Do(req) //nolint:gosec // G704 emas: yuqoridagi izohga qarang
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openfoodfacts %d", res.StatusCode)
	}
	var body struct {
		Status  int `json:"status"`
		Product struct {
			Name string `json:"product_name"`
		} `json:"product"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(&body); err != nil {
		return "", err
	}
	if body.Status != 1 {
		return "", nil
	}
	return strings.TrimSpace(body.Product.Name), nil
}

// ── Qurt katalogi sinxroni (ShoxPos qurtSync o'rnida, jarayon ichida) ──

// ShopQurtSync — shop mahsuloti o'zgarganda mijoz qurt katalogini yangilaydi.
// Xato bo'lsa kalit navbatda qoladi va `Retry` qayta urinadi; server har
// yoqilganda `ResyncAll` hammasini solishtiradi (ShoxPos markAllForResync).
type ShopQurtSync struct {
	Repo    qurt.Repository
	Shop    *shop.Service
	Stores  func(ctx context.Context) ([]string, error) // qurt do'konlari entity_id
	mu      sync.Mutex
	pending map[string]bool // storeID \x00 barcode
}

// Push — bitta mahsulot holati (o'chirilgan yoki to'liq bo'lmagan — katalogdan
// olib tashlanmaydi, faqat nashr qilinmaydi; ShoxPos toPayload bilan 1:1).
func (q *ShopQurtSync) Push(ctx context.Context, storeID string, p *shop.Product, deleted bool) {
	if err := q.push(ctx, storeID, p, deleted); err != nil {
		slog.Warn("qurt katalogi sinxroni", "store", storeID, "barcode", p.Barcode, "err", err)
		q.mu.Lock()
		if q.pending == nil {
			q.pending = map[string]bool{}
		}
		q.pending[storeID+"\x00"+p.Barcode] = true
		q.mu.Unlock()
	}
}

func (q *ShopQurtSync) push(ctx context.Context, storeID string, p *shop.Product, deleted bool) error {
	if deleted {
		return q.Repo.Delete(ctx, storeID, p.Barcode)
	}
	if p.Name == "" || p.Category == "" || p.PriceTiyin <= 0 || p.WeightGrams < 1 {
		return nil // to'liq emas — hali nashr qilinmaydi
	}
	stock := int(p.Quantity)
	if stock > 1000000 {
		stock = 1000000
	}
	if stock < 0 {
		stock = 0
	}
	qp := &qurt.Product{StoreID: storeID, ExternalID: p.Barcode, Name: p.Name, Description: p.Description,
		Composition: p.Composition, Category: p.Category, PriceTiyin: p.PriceTiyin, WholesaleTiyin: p.BasePriceTiyin,
		WeightGrams: p.WeightGrams, ImageURL: p.ImageURL, Available: p.Quantity > 0, Stock: stock, StockSet: true}
	if err := qp.Normalize(); err != nil {
		slog.Warn("qurt katalogi rad etdi", "store", storeID, "barcode", p.Barcode, "err", err)
		return nil // qayta urinish foydasiz (ShoxPos: 400 -> synced)
	}
	_, err := q.Repo.Upsert(ctx, qp)
	return err
}

// Retry — navbatdagi muvaffaqiyatsiz kalitlarni qayta yuboradi.
func (q *ShopQurtSync) Retry(ctx context.Context) {
	q.mu.Lock()
	keys := make([]string, 0, len(q.pending))
	for k := range q.pending {
		keys = append(keys, k)
	}
	q.pending = map[string]bool{}
	q.mu.Unlock()
	for _, k := range keys {
		storeID, barcode, _ := strings.Cut(k, "\x00")
		p, err := q.Shop.QurtProductOf(ctx, storeID, barcode)
		switch {
		case errors.Is(err, shop.ErrNotFound):
			q.Push(ctx, storeID, &shop.Product{Barcode: barcode}, true)
		case err != nil:
			q.mu.Lock()
			q.pending[k] = true // baza vaqtincha xato — keyingi safar
			q.mu.Unlock()
		default:
			q.Push(ctx, storeID, p, false)
		}
	}
}

// ResyncAll — barcha qurt do'konlari mahsulotlarini katalog bilan solishtiradi.
func (q *ShopQurtSync) ResyncAll(ctx context.Context) {
	stores, err := q.Stores(ctx)
	if err != nil {
		slog.Warn("qurt resync: do'konlar", "err", err)
		return
	}
	n := 0
	for _, id := range stores {
		list, err := q.Shop.QurtProducts(ctx, id)
		if err != nil {
			slog.Warn("qurt resync", "store", id, "err", err)
			continue
		}
		for i := range list {
			q.Push(ctx, id, &list[i], false)
			n++
		}
	}
	if n > 0 {
		slog.Info("qurt katalogi qayta solishtirildi", "mahsulot", n)
	}
}

// ShopQurtStoreIDs — faol qurt do'konlari (ResyncAll uchun).
func (s *Server) ShopQurtStoreIDs(ctx context.Context) ([]string, error) {
	m, err := s.qurtStores(ctx)
	if err != nil {
		return nil, err
	}
	return qurtIDs(m), nil
}
