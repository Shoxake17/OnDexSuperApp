package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// `POST/DELETE /admin/stores` va hamkor paroli testlari.

func TestAdminCreatesStoreAndOwnerLogsInWithPassword(t *testing.T) {
	repo := storage.NewMemoryUserRepo()
	h, jwt, _ := adminRestaurantServer(t, repo)

	w := do(t, h, "POST", "/admin/stores", jwt,
		`{"name":"Baraka do'koni","phone":"+998 90 111 22 33","category":"qurt","password":"Sinov-parol-123"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("do'kon yaratilmadi: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Store struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			Phone string `json:"phone"`
		} `json:"store"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Store.ID == "" {
		t.Fatalf("javob: %s (%v)", w.Body.String(), err)
	}

	acc, err := repo.GetByRoleEntity(context.Background(), users.RoleStore, out.Store.ID)
	if err != nil {
		t.Fatal(err)
	}
	if acc.StoreCategory != users.StoreCategoryQurt {
		t.Errorf("kategoriya saqlanmadi: %q", acc.StoreCategory)
	}
	if acc.Name != "Baraka do'koni" || acc.Phone != "+998901112233" || !acc.PhoneVerified {
		t.Errorf("akkaunt noto'g'ri: %+v", acc)
	}
	if acc.PasswordHash == "" || acc.PasswordHash == "Sinov-parol-123" {
		t.Error("parol xeshlanmagan")
	}
	if ok, _ := users.VerifyPassword(context.Background(), "Sinov-parol-123", acc.PasswordHash); !ok {
		t.Error("parol xeshi mos kelmadi")
	}
	// Javobda parol xeshi chiqmasligi kerak.
	if bytesContains(w.Body.String(), acc.PasswordHash) {
		t.Error("javobda parol xeshi bor")
	}

	// Ro'yxatda ko'rinadi
	w = do(t, h, "GET", "/admin/accounts?role=store", jwt, "")
	if w.Code != http.StatusOK || !bytesContains(w.Body.String(), out.Store.ID) {
		t.Errorf("accounts?role=store: %d %s", w.Code, w.Body.String())
	}

	// Takroriy telefon
	w = do(t, h, "POST", "/admin/stores", jwt, `{"name":"Boshqa","phone":"+998901112233","category":"qurt"}`)
	if w.Code != http.StatusConflict {
		t.Errorf("takroriy telefon: %d", w.Code)
	}

	// O'chirish
	w = do(t, h, "DELETE", "/admin/stores/"+out.Store.ID, jwt, "")
	if w.Code != http.StatusOK {
		t.Fatalf("o'chirish: %d %s", w.Code, w.Body.String())
	}
	if _, err := repo.GetByRoleEntity(context.Background(), users.RoleStore, out.Store.ID); err == nil {
		t.Error("akkaunt o'chirilmadi")
	}
	if w = do(t, h, "DELETE", "/admin/stores/"+out.Store.ID, jwt, ""); w.Code != http.StatusNotFound {
		t.Errorf("ikkinchi o'chirish: %d", w.Code)
	}
}

func TestAdminStoreValidation(t *testing.T) {
	h, jwt, _ := adminRestaurantServer(t, storage.NewMemoryUserRepo())
	for name, body := range map[string]string{
		"nomsiz":        `{"phone":"+998901112233"}`,
		"telefonsiz":    `{"name":"A","category":"qurt"}`,
		"kategoriyasiz": `{"name":"A","phone":"+998901112233"}`,
		"noma'lum":      `{"name":"A","phone":"+998901112233","category":"supermarket"}`,
		"rol nomi":      `{"name":"A","phone":"+998901112233","category":"admin"}`,
		"noto'g'ri tel": `{"name":"A","phone":"123"}`,
		"zaif parol":    `{"name":"A","phone":"+998901112233","category":"qurt","password":"123"}`,
	} {
		if w := do(t, h, "POST", "/admin/stores", jwt, body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d kutilgan 400 — %s", name, w.Code, w.Body.String())
		}
	}
}

// Parolsiz yaratilgan hamkor faqat kod bilan kiradi (avvalgi xatti-harakat).
func TestStoreWithoutPasswordIsNotVerified(t *testing.T) {
	repo := storage.NewMemoryUserRepo()
	h, jwt, _ := adminRestaurantServer(t, repo)
	w := do(t, h, "POST", "/admin/stores", jwt, `{"name":"A","phone":"+998901112244","category":"qurt"}`)
	if w.Code != http.StatusCreated {
		t.Fatal(w.Code, w.Body.String())
	}
	u, err := repo.GetByPhone(context.Background(), "+998901112244")
	if err != nil {
		t.Fatal(err)
	}
	if u.PasswordHash != "" || u.PhoneVerified {
		t.Errorf("parolsiz akkaunt tasdiqlangan bo'lmasligi kerak: %+v", u)
	}
}

// Restoran uchun ham boshlang'ich parol berish mumkin.
func TestAdminRestaurantWithPassword(t *testing.T) {
	repo := storage.NewMemoryUserRepo()
	h, jwt, _ := adminRestaurantServer(t, repo)
	w := do(t, h, "POST", "/admin/restaurants", jwt,
		`{"name":"Navo","phone":"+998901112255","category":"qurt","lat":41.0,"lng":71.2,"password":"Restoran-parol-1"}`)
	if w.Code != http.StatusCreated {
		t.Fatal(w.Code, w.Body.String())
	}
	u, err := repo.GetByPhone(context.Background(), "+998901112255")
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := users.VerifyPassword(context.Background(), "Restoran-parol-1", u.PasswordHash); !ok || !u.PhoneVerified {
		t.Errorf("restoran paroli o'rnatilmadi: %+v", u)
	}
	// Zaif parol bilan restoran YARIM yaratilmasligi kerak.
	w = do(t, h, "POST", "/admin/restaurants", jwt,
		`{"name":"Zaif","phone":"+998901112266","category":"qurt","lat":41.0,"lng":71.2,"password":"1"}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("zaif parol: %d", w.Code)
	}
}

func bytesContains(s, sub string) bool {
	return sub != "" && len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

// Kategoriyani o'zgartirish: faqat do'kon akkauntida, yopiq ro'yxatdan.
func TestAdminUpdatesStoreCategory(t *testing.T) {
	repo := storage.NewMemoryUserRepo()
	h, jwt, _ := adminRestaurantServer(t, repo)

	w := do(t, h, "POST", "/admin/stores", jwt, `{"name":"Baraka","phone":"+998901119900","category":"store"}`)
	if w.Code != http.StatusCreated {
		t.Fatal(w.Code, w.Body.String())
	}
	var out struct {
		Store struct {
			ID string `json:"id"`
		} `json:"store"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	id := out.Store.ID

	if w = do(t, h, "POST", "/admin/stores/"+id, jwt, `{"category":"qurt"}`); w.Code != http.StatusOK {
		t.Fatalf("yangilanmadi: %d %s", w.Code, w.Body.String())
	}
	acc, _ := repo.GetByRoleEntity(context.Background(), users.RoleStore, id)
	if acc.StoreCategory != users.StoreCategoryQurt || acc.Name != "Baraka" {
		t.Errorf("kategoriya yoki nom noto'g'ri: %+v", acc)
	}
	// Nom o'zgarmasa (faqat kategoriya) saqlanib qoladi; nom bilan birga ham ishlaydi
	if w = do(t, h, "POST", "/admin/stores/"+id, jwt, `{"name":"Yangi nom"}`); w.Code != http.StatusOK {
		t.Fatalf("nom: %d", w.Code)
	}
	acc, _ = repo.GetByRoleEntity(context.Background(), users.RoleStore, id)
	if acc.Name != "Yangi nom" || acc.StoreCategory != users.StoreCategoryQurt {
		t.Errorf("nom o'zgarishi kategoriyani buzdi: %+v", acc)
	}
	for name, body := range map[string]string{
		"noma'lum kategoriya": `{"category":"supermarket"}`,
		"bo'sh nom":           `{"name":"  "}`,
	} {
		if w := do(t, h, "POST", "/admin/stores/"+id, jwt, body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: %d kutilgan 400", name, w.Code)
		}
	}
	if w = do(t, h, "POST", "/admin/stores/yoq", jwt, `{"category":"qurt"}`); w.Code != http.StatusNotFound {
		t.Errorf("mavjud emas: %d", w.Code)
	}
}
