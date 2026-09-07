package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"chustapp/internal/delivery"
	"chustapp/internal/users"
)

func (s *Server) registerMeRoutes(mux *http.ServeMux) {
	// POST /me — mijoz o'z ismi/familiyasini saqlaydi.
	//
	// Bu endpoint AVVAL UMUMAN YO'Q EDI: `users.name` ustuni bor edi-yu,
	// mijoz ilovasida uni to'ldirishning hech qanday yo'li yo'q edi va
	// profil har doim "Mijoz" deb ko'rsatardi.
	mux.HandleFunc("POST /me", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				FirstName string `json:"first_name"`
				LastName  string `json:"last_name"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if err := s.AuthSvc.UpdateName(r.Context(), claimsFrom(r).Subject,
				req.FirstName, req.LastName); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			u, err := s.UserRepo.GetByID(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, u)
		}))

	// POST /me/password — parol o'rnatish/o'zgartirish.
	//
	// JORIY PAROL: akkauntda parol allaqachon bo'lsa, `current_password`
	// SHART. Avval bu tekshiruv yo'q edi va o'g'irlangan token (masalan
	// qulfsiz qolgan telefon) egasiga parolni jimgina almashtirib,
	// pastdagi `Revoke` orqali HAQIQIY egani tizimdan chiqarib yuborish
	// imkonini berardi. Parol hali yo'q bo'lsa (SMS bilan kirgan yoki
	// endigina raqamini tasdiqlagan foydalanuvchi) so'ralmaydi —
	// ro'yxatdan o'tish oqimi aynan shu holatdan foydalanadi.
	//
	// SESSIYALAR: parol o'zgargach BARCHA eski sessiyalar BEKOR
	// QILINADI. Sabab: parol o'zgartirishning asosiy holati —
	// "akkauntim buzilgan deb o'ylayapman". Eski token tirik qolsa,
	// hujumchi 30 kun davomida kira olardi va parol o'zgarishi
	// ma'nosiz bo'lardi.
	//
	// YANGI TOKEN: bekor qilish chaqiruvchining O'Z tokenini ham
	// yaroqsiz qiladi, shuning uchun javobda darhol yangi token
	// beriladi. Busiz foydalanuvchi parolni o'zgartirgani uchun
	// "jazolanib", tizimdan chiqarib yuborilardi.
	mux.HandleFunc("POST /me/password", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				CurrentPassword string `json:"current_password"`
				Password        string `json:"password"`
				PasswordConfirm string `json:"password_confirm"`
			}
			if !decodeJSON(w, r, &req) {
				return
			}
			c := claimsFrom(r)
			uid := c.Subject
			// SMS kod bilan HOZIRGINA kirgan bo'lsa joriy parol
			// so'ralmaydi — parolini unutgan odam uni ayta olmaydi,
			// SMS esa egalikni isbotlaydi. Muddat 15 daqiqa
			// (`users.PhoneProofWindow`), ya'ni o'g'irlangan token
			// bu imtiyozni uzoq saqlab qolmaydi.
			if err := s.AuthSvc.SetPassword(r.Context(), uid,
				req.CurrentPassword, req.Password, req.PasswordConfirm,
				c.HasFreshPhoneProof(time.Now())); err != nil {
				respondAuthError(w, err, http.StatusBadRequest)
				return
			}
			s.Revoked.Revoke(r.Context(), uid)
			u, err := s.UserRepo.GetByID(r.Context(), uid)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			token, err := s.Tokens.Issue(u)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"password_set": true, "token": token})
		}))

	mux.HandleFunc("GET /me", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			u, err := s.UserRepo.GetByID(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, u)
		}))

	// GET /me/address — mijozning xaritadan tanlagan yetkazib berish
	// manzilini qaytaradi (hali tanlanmagan bo'lsa bo'sh/0 qiymatlar).
	mux.HandleFunc("GET /me/address", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			u, err := s.UserRepo.GetByID(r.Context(), claimsFrom(r).Subject)
			if err != nil {
				httpError(w, http.StatusNotFound, err)
				return
			}
			writeJSON(w, http.StatusOK, u.Address)
		}))

	// POST /me/address — xarita ekranida "Tayyor" bosilganda manzil
	// tafsilotlari shu foydalanuvchi akkauntiga saqlanadi. Faqat mijoz
	// (RoleCustomer) emas, ISTALGAN login qilgan foydalanuvchiga ochiq —
	// aks holda avval kuryer/boshqa rolga o'tgan akkaunt (bir xil telefon
	// raqami customer_app'da ham ishlatilaveradi) 403 bilan bloklanib
	// qolar edi, garchi bu shunchaki shaxsiy manzilni saqlash bo'lsa ham.
	mux.HandleFunc("POST /me/address", s.auth(nil,
		func(w http.ResponseWriter, r *http.Request) {
			var addr users.AddressDetails
			if err := json.NewDecoder(r.Body).Decode(&addr); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			// Matn maydonlarining uzunligi (bug.md 26-band): avval
			// chegara umuman yo'q edi va profilga ~1 MB matn saqlash
			// mumkin edi — u keyin panelga, kuryerga va chekka
			// chiqardi.
			if err := addr.Validate(); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			// Koordinata tekshiruvi + hudud tekshiruvi bitta chaqiruvda
			// (`CheckPoint`). Hudud tekshiruvi buyurtma bosqichiga
			// yetmasdan, manzil saqlanayotgan paytdayoq aytilgani ma'qul.
			switch err := delivery.CheckPoint(addr.Lat, addr.Lng); {
			case errors.Is(err, delivery.ErrBadCoords):
				httpError(w, http.StatusBadRequest, errors.New("lat/lng noto'g'ri"))
				return
			case errors.Is(err, delivery.ErrOutsideArea):
				httpError(w, http.StatusBadRequest,
					errors.New("bu manzilga hozircha yetkazmaymiz — faqat Chust shahri"))
				return
			}
			if err := s.UserRepo.UpdateAddress(r.Context(), claimsFrom(r).Subject, addr); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			writeJSON(w, http.StatusOK, addr)
		}))

	// ---------- Buyurtmalar (token talab qilinadi) ----------

	// POST /orders — istalgan login qilgan foydalanuvchi (faqat mijoz
	// bilan cheklanmagan — Yandex Eats/Wolt'dagi kabi kuryer yoki restoran
	// xodimi ham shaxsan o'zi uchun taom buyurtma qila oladi; bir xil
	// telefon raqami avval kuryer bo'lib ro'yxatdan o'tgan bo'lsa, endi u
	// customer_app'da 403 bilan bloklanib qolmasligi kerak). Buyurtma
	// beruvchi FAQAT product_id + qty yuboradi; narx, nom va restoran
	// katalogdan aniqlanadi (narxni soxtalashtirib bo'lmaydi).
}
