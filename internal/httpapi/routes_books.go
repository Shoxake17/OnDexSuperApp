package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/users"
)

// Kafe kutubxonasi — 3D maketdagi javondan olib o'qiladigan kitoblar.
//
// ┌─ MOLIYAVIY XAVF YO'Q ──────────────────────────────────────────────┐
// Kitobda narx maydoni umuman yo'q va u savatga tushmaydi. Maket
// ichida buyurtma berish YOPILGAN (`table_menu.gd` dagi `ENABLED`
// bayrog'iga qarang), chunki stol buyurtmasi QR token talab qiladi.
//
// Kitob esa faqat o'qish uchun: eng yomon holatda foydalanuvchi
// bepul matnni ko'radi. Shuning uchun bu yo'nalishda pul yo'qotish
// ehtimoli yo'q.
// └────────────────────────────────────────────────────────────────────┘

// booksMaxTitle — nom va muallif uzunligi chegarasi.
const (
	booksMaxTitle  = 200
	booksMaxAuthor = 120
	booksMaxPages  = 500
	booksMaxURL    = 512
)

func (s *Server) registerBookRoutes(mux *http.ServeMux) {
	// Kitoblarni restoran o'zi va admin boshqaradi - stollar bilan
	// bir xil qoida.
	staff := []users.Role{users.RoleRestaurant, users.RoleAdmin}

	// GET /restaurants/{id}/books — OCHIQ ro'yxat.
	//
	// Menyu bilan bir xil qoida: kitoblar hammaga ko'rinadi va
	// avtorizatsiya talab qilmaydi. Matn bu yerda QAYTMAYDI —
	// ro'yxat uchun kerak emas va javobni shishirardi.
	mux.HandleFunc("GET /restaurants/{id}/books", func(w http.ResponseWriter, r *http.Request) {
		if !s.requireBooks(w) {
			return
		}
		restaurantID := r.PathValue("id")
		if _, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID); err != nil {
			httpError(w, http.StatusNotFound, err)
			return
		}
		list, err := s.BookRepo.ListBooks(r.Context(), restaurantID, false)
		if err != nil {
			httpError(w, http.StatusInternalServerError, err)
			return
		}
		// O'chirilgan kitob maketda ko'rinmaydi.
		out := make([]*catalog.Book, 0, len(list))
		for _, b := range list {
			if b.Active {
				out = append(out, b)
			}
		}
		writeJSON(w, http.StatusOK, out)
	})

	// GET /restaurants/{id}/books/all — BOSHQARUV ro'yxati.
	//
	// ┌─ NEGA ALOHIDA ENDPOINT KERAK BO'LDI ───────────────────────────┐
	// Admin panel avval yuqoridagi OCHIQ ro'yxatdan o'qirdi, u esa
	// o'chirilgan kitobni ATAYLAB yashiradi. Natijada "faol" tugmasi
	// bir tomonlama eshik edi: o'chirilgan kitob ro'yxatdan yo'qolardi
	// va uni qayta yoqishning yo'li qolmasdi.
	//
	// Ochiq ro'yxatga `?all=1` qo'shish ham mumkin edi, lekin unda
	// bitta endpoint ba'zan ochiq, ba'zan himoyalangan bo'lardi —
	// shunday joylarda tekshiruv tushib qolishi oson. Ikki alohida
	// yo'l: biri hamma uchun va DOIM filtrlaydi, ikkinchisi
	// xodim uchun va hech narsa yashirmaydi.
	// └────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("GET /restaurants/{id}/books/all", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !s.requireBooks(w) {
				return
			}
			// Restoran FAQAT o'z kitoblarini ko'radi; admin hammasini.
			restaurantID, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				httpError(w, http.StatusNotFound, catalog.ErrNotFound)
				return
			}
			list, err := s.BookRepo.ListBooks(r.Context(), restaurantID, false)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if list == nil {
				list = []*catalog.Book{}
			}
			writeJSON(w, http.StatusOK, list)
		}))

	// GET /books/{id} — bitta kitob, MATNI bilan.
	mux.HandleFunc("GET /books/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !s.requireBooks(w) {
			return
		}
		b, err := s.BookRepo.GetBook(r.Context(), r.PathValue("id"))
		if err != nil {
			httpError(w, http.StatusNotFound, err)
			return
		}
		if !b.Active {
			httpError(w, http.StatusNotFound, catalog.ErrNotFound)
			return
		}
		writeJSON(w, http.StatusOK, b)
	})

	// POST /restaurants/{id}/books — restoran o'z kitobini qo'shadi.
	mux.HandleFunc("POST /restaurants/{id}/books", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !s.requireBooks(w) {
				return
			}
			// Restoran FAQAT O'Z kitobini qo'sha oladi: ID tokendan
			// olinadi, yo'ldagi qiymat unga mos kelishi shart.
			restaurantID, ok := entityIDFor(claimsFrom(r), r.PathValue("id"))
			if !ok {
				httpError(w, http.StatusNotFound, catalog.ErrNotFound)
				return
			}
			var req bookRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			b := &catalog.Book{
				ID:           NewID(),
				RestaurantID: restaurantID,
				Active:       true,
				CreatedAt:    time.Now(),
			}
			if err := req.applyTo(b); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if err := s.BookRepo.SaveBook(r.Context(), b); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusCreated, b)
		}))

	// PATCH /books/{id} — qismiy yangilash.
	mux.HandleFunc("PATCH /books/{id}", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !s.requireBooks(w) {
				return
			}
			b, err := s.BookRepo.GetBook(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if _, ok := entityIDFor(claimsFrom(r), b.RestaurantID); !ok {
				httpError(w, http.StatusNotFound, catalog.ErrNotFound)
				return
			}
			var req bookRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if err := req.applyTo(b); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if err := s.BookRepo.SaveBook(r.Context(), b); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, b)
		}))

	mux.HandleFunc("DELETE /books/{id}", s.auth(staff,
		func(w http.ResponseWriter, r *http.Request) {
			if !s.requireBooks(w) {
				return
			}
			b, err := s.BookRepo.GetBook(r.Context(), r.PathValue("id"))
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			if _, ok := entityIDFor(claimsFrom(r), b.RestaurantID); !ok {
				httpError(w, http.StatusNotFound, catalog.ErrNotFound)
				return
			}
			if err := s.BookRepo.DeleteBook(r.Context(), b.ID); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
}

// requireBooks — kitob ombori ulanganmi.
//
// Ombor ixtiyoriy (faqat Mongo rejimida bor), shuning uchun uning
// yo'qligi xato emas: qolgan API ishlayveradi, kitob endpointlari
// esa buni ochiq aytadi.
func (s *Server) requireBooks(w http.ResponseWriter) bool {
	if s.BookRepo == nil {
		httpError(w, http.StatusServiceUnavailable,
			errors.New("kitoblar ombori ulanmagan"))
		return false
	}
	return true
}

// bookRequest — yaratish va yangilash uchun umumiy so'rov.
//
// ┌─ KO'RSATKICHLI MAYDONLAR ──────────────────────────────────────────┐
// PATCH da faqat YUBORILGAN maydonlar o'zgarishi kerak. Oddiy
// qiymatlar ishlatilsa, yuborilmagan maydon "bo'sh" deb tushunilib,
// mavjud matnni o'chirib yuborardi - bu xato admin panelda bir
// marta sodir bo'lgan (`routes_admin.go` dagi 3D maydonlariga qarang).
// └────────────────────────────────────────────────────────────────────┘
type bookRequest struct {
	Title    *string   `json:"title"`
	Author   *string   `json:"author"`
	CoverURL *string   `json:"cover_url"`
	PDFURL   *string   `json:"pdf_url"`
	Text     *string   `json:"text"`
	Pages    *[]string `json:"pages"`
	Active   *bool     `json:"active"`
}

func (req bookRequest) applyTo(b *catalog.Book) error {
	if req.Title != nil {
		t := strings.TrimSpace(*req.Title)
		if t == "" {
			return errors.New("nom bo'sh")
		}
		if len(t) > booksMaxTitle {
			return errors.New("nom juda uzun")
		}
		b.Title = t
	}
	if b.Title == "" {
		return errors.New("nom majburiy")
	}

	if req.Author != nil {
		a := strings.TrimSpace(*req.Author)
		if len(a) > booksMaxAuthor {
			return errors.New("muallif juda uzun")
		}
		b.Author = a
	}
	// URL maydonlarida uzunlikdan tashqari SXEMA ham tekshiriladi
	// (bug.md 24-band): avval `javascript:`, `data:` va begona domen
	// bemalol saqlanardi va mijoz ilovasida ishlatilardi.
	if req.CoverURL != nil {
		u := strings.TrimSpace(*req.CoverURL)
		if len(u) > booksMaxURL {
			return errors.New("muqova manzili juda uzun")
		}
		if !isSafeMediaURL(u) {
			return errors.New("muqova manzili yaroqsiz (faqat https:// yoki ichki yo'l)")
		}
		b.CoverURL = u
	}
	if req.PDFURL != nil {
		u := strings.TrimSpace(*req.PDFURL)
		if len(u) > booksMaxURL {
			return errors.New("PDF manzili juda uzun")
		}
		if !isSafeMediaURL(u) {
			return errors.New("PDF manzili yaroqsiz (faqat https:// yoki ichki yo'l)")
		}
		b.PDFURL = u
	}
	if req.Text != nil {
		// Chegara ataylab: matn butunlay xotiraga o'qiladi va
		// bitta javobda uzatiladi.
		if len(*req.Text) > catalog.MaxBookTextBytes {
			return errors.New("matn juda katta")
		}
		b.Text = *req.Text
	}
	if req.Pages != nil {
		if len(*req.Pages) > booksMaxPages {
			return errors.New("sahifalar juda ko'p")
		}
		pages := make([]string, 0, len(*req.Pages))
		for _, p := range *req.Pages {
			p = strings.TrimSpace(p)
			// Yaroqsiz sahifa JIMGINA tashlanadi (mavjud xulq):
			// bitta buzilgan yozuv butun kitobni saqlashni
			// to'xtatmasligi kerak. Sxema tekshiruvi ham shu
			// shartga qo'shildi (bug.md 24-band).
			if p == "" || len(p) > booksMaxURL || !isSafeMediaURL(p) {
				continue
			}
			pages = append(pages, p)
		}
		b.Pages = pages
	}
	if req.Active != nil {
		b.Active = *req.Active
	}
	// Kitobda o'qiladigan biror narsa BO'LISHI kerak. PDF ham hisobga
	// olinadi: uni yuklashda matn ajratiladi va shu yerga yoziladi,
	// lekin ajratish bo'sh natija bergan (skanerlangan) kitob ham
	// javonda muqovasi bilan turishi mumkin.
	if b.Text == "" && len(b.Pages) == 0 && b.PDFURL == "" {
		return errors.New("PDF, matn yoki sahifalar kerak")
	}
	return nil
}
