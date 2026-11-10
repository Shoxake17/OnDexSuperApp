package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"chustapp/internal/users"
)

// partnerPasswordHash — superadmin hamkor (restoran, do'kon) akkaunti uchun
// kiritgan BOSHLANG'ICH parolni xeshlaydi. Parol bo'sh bo'lsa ("", nil):
// akkaunt faqat SMS/Telegram kod bilan kiradi (avvalgi xatti-harakat).
//
// ┌─ NEGA PhoneVerified=true ──────────────────────────────────────────┐
// `LoginWithPassword` tasdiqlanmagan telefonni rad etadi. Hamkor akkaunti
// SUPERADMIN tomonidan yaratiladi, ya'ni raqamni admin kafolatlaydi —
// aks holda hamkor parol bilan birinchi marta kira olmasdi (parolni
// o'rnatish uchun avval kod bilan kirishi shart bo'lardi).
//
// Xavf: admin raqamni xato yozsa, akkaunt begona raqamga bog'lanadi.
// Lekin u akkauntga parolni FAQAT admin biladi, haqiqiy egasi esa kod
// bilan kirsa ham baribir shu akkauntga tushadi (yangi akkaunt emas),
// va `/me/password` bilan parolni almashtirsa admin paroli o'chadi.
// └────────────────────────────────────────────────────────────────────┘
func partnerPasswordHash(ctx context.Context, password string) (string, error) {
	if password == "" {
		return "", nil
	}
	if err := users.ValidatePassword(password); err != nil {
		return "", err
	}
	return users.HashPassword(ctx, password)
}

// registerAdminStoreRoutes — superadmin do'konlarni ro'yxatdan o'tkazadi.
//
// Restoran kabi: do'konlar o'zi ro'yxatdan o'tmaydi, akkauntni faqat
// superadmin yaratadi. Do'kon ma'lumotlari (nom) akkauntning o'zida
// turadi: kassa/POS (alohida xizmat) tovar va sotuvlarni o'zida saqlaydi,
// OnDex esa FAQAT kimlikni beradi (`internal/sso`).
func (s *Server) registerAdminStoreRoutes(mux *http.ServeMux) {
	// POST /admin/stores — do'kon + unga kirish akkaunti bir amalda.
	mux.HandleFunc("POST /admin/stores", s.auth([]users.Role{users.RoleAdmin}, s.adminAction("store.create", false,
		func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Name     string `json:"name"`     // do'kon nomi (majburiy)
				Phone    string `json:"phone"`    // akkaunt telefoni (majburiy)
				Category string `json:"category"` // qurt | store | pharmacy (majburiy)
				Password string `json:"password"` // boshlang'ich parol (ixtiyoriy)
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			name := strings.TrimSpace(req.Name)
			if name == "" || strings.TrimSpace(req.Phone) == "" {
				httpError(w, http.StatusBadRequest, errors.New("name va phone majburiy"))
				return
			}
			if len([]rune(name)) > 120 {
				httpError(w, http.StatusBadRequest, errors.New("do'kon nomi juda uzun"))
				return
			}
			// Kategoriya MAJBURIY va yopiq ro'yxatdan: u POS panelida qaysi
			// sahifalar ochiq ekanligini belgilaydi.
			category, err := users.ParseStoreCategory(req.Category)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			phone, err := users.NormalizePhone(req.Phone)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if _, err := s.UserRepo.GetByPhone(r.Context(), phone); err == nil {
				httpError(w, http.StatusConflict, errors.New("bu telefon raqam allaqachon ro'yxatda"))
				return
			}
			hash, err := partnerPasswordHash(r.Context(), req.Password)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			account := users.User{
				ID: NewID(), Phone: phone, Name: name,
				Role: users.RoleStore, EntityID: NewID(), StoreCategory: category,
				PasswordHash: hash, PhoneVerified: hash != "",
				CreatedAt: time.Now(),
			}
			if err := s.UserRepo.Create(r.Context(), &account); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			slog.Info("do'kon akkaunti yaratildi", "store", account.EntityID, "by", claimsFrom(r).Subject)
			writeJSON(w, http.StatusCreated, map[string]any{
				"store": map[string]string{"id": account.EntityID, "name": name, "phone": phone,
					"category": string(category)},
				"account": account,
			})
		})))

	// POST /admin/stores/{id} — do'kon nomi va/yoki kategoriyasini o'zgartirish.
	//
	// Kategoriya POS panelida qaysi sahifa/API ochiq ekanligini belgilaydi.
	// O'zgarish POS (ShoxPos) ga u tokenini yangilaganda yetib boradi
	// (`POST /sso/identity`), ya'ni qayta kirish shart emas.
	mux.HandleFunc("POST /admin/stores/{id}", s.auth([]users.Role{users.RoleAdmin},
		func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			var req struct {
				Name     *string `json:"name"`
				Category *string `json:"category"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			acc, err := s.UserRepo.GetByRoleEntity(r.Context(), users.RoleStore, id)
			if err != nil {
				httpError(w, http.StatusNotFound, errors.New("do'kon topilmadi"))
				return
			}
			name, category := acc.Name, acc.StoreCategory
			if req.Name != nil {
				name = strings.TrimSpace(*req.Name)
				if name == "" || len([]rune(name)) > 120 {
					httpError(w, http.StatusBadRequest, errors.New("do'kon nomi bo'sh yoki juda uzun"))
					return
				}
			}
			if req.Category != nil {
				c, err := users.ParseStoreCategory(*req.Category)
				if err != nil {
					httpError(w, http.StatusBadRequest, err)
					return
				}
				category = c
			}
			if err := s.UserRepo.SetStoreProfile(r.Context(), acc.ID, name, category); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			slog.Info("do'kon yangilandi", "store", id, "category", category, "by", claimsFrom(r).Subject)
			writeJSON(w, http.StatusOK, map[string]string{
				"id": id, "name": name, "category": string(category),
			})
		}))

	// DELETE /admin/stores/{id} — do'kon akkauntini o'chiradi va tokenlarini
	// bekor qiladi (restoranni o'chirishdagi kabi).
	mux.HandleFunc("DELETE /admin/stores/{id}", s.auth([]users.Role{users.RoleAdmin}, s.adminAction("store.delete", true,
		func(w http.ResponseWriter, r *http.Request) {
			id := r.PathValue("id")
			if _, err := s.UserRepo.GetByRoleEntity(r.Context(), users.RoleStore, id); err != nil {
				httpError(w, http.StatusNotFound, errors.New("do'kon topilmadi"))
				return
			}
			deleted, err := s.UserRepo.DeleteByRoleEntity(r.Context(), users.RoleStore, id)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			for _, uid := range deleted {
				s.Revoked.Revoke(r.Context(), uid)
			}
			// Do'konning qurt mahsulotlari ham mijozdan olib tashlanadi.
			if s.QurtRepo != nil {
				if err := s.QurtRepo.DeleteByStore(r.Context(), id); err != nil {
					slog.Error("do'kon mahsulotlarini o'chirishda xato", "store", id, "err", err)
				}
			}
			slog.Info("do'kon o'chirildi", "store", id, "by", claimsFrom(r).Subject)
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		})))
}
