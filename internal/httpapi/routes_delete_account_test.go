package httpapi

import (
	"net/http"
	"testing"
	"time"

	"chustapp/internal/users"
)

// TestDeleteAccountOnlyCustomer — "Akkauntni o'chirish" faqat mijoz
// roliga ochiq. Kuryer/affitsiant/restoran/admin akkauntlari xodim
// yozuviga ergashadi va o'z hayot davri qoidasiga ega
// (`staff-module-architecture`) — bu yo'l ularga aralashmasligi shart.
func TestDeleteAccountOnlyCustomer(t *testing.T) {
	h, jwt := authzServer(t)
	for role, tok := range jwt {
		if role == users.RoleCustomer {
			continue
		}
		if w := do(t, h, "POST", "/me/delete-account", tok, `{}`); w.Code != http.StatusForbidden {
			t.Fatalf("rol %q uchun kutilgan 403, keldi %d: %s", role, w.Code, w.Body.String())
		}
	}
}

// TestDeleteAccountRevokesSession — muvaffaqiyatli o'chirishdan keyin
// AYNAN SHU token (va foydalanuvchining boshqa hech qanday tokeni)
// darhol ishlamay qolishi kerak — "chiqish" emas, akkauntning yopilishi.
func TestDeleteAccountRevokesSession(t *testing.T) {
	h, jwt := authzServer(t)
	customer := jwt[users.RoleCustomer]

	if w := do(t, h, "GET", "/me", customer, ""); w.Code != http.StatusOK {
		t.Fatalf("o'chirishdan oldin /me: %d %s", w.Code, w.Body.String())
	}

	// `revoke.Store.Revoke` belgisi soniyagacha yaxlitlanadi — aynan
	// shu soniyada chiqarilgan token yaroqli qolib ketmasligi uchun
	// (`TestLogoutRevokesEarlierTokens` dagi izohga qarang).
	nextSecond := time.Now().Truncate(time.Second).Add(time.Second)
	time.Sleep(time.Until(nextSecond) + 20*time.Millisecond)

	w := do(t, h, "POST", "/me/delete-account", customer, `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("delete-account: %d %s", w.Code, w.Body.String())
	}

	if w := do(t, h, "GET", "/me", customer, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("XAVFSIZLIK: o'chirishdan keyin eski token ishladi: %d", w.Code)
	}
	// Boshqa foydalanuvchining sessiyasi tegilmaydi.
	if w := do(t, h, "GET", "/me", jwt[users.RoleAdmin], ""); w.Code != http.StatusOK {
		t.Fatalf("boshqa foydalanuvchi sessiyasi ham bekor bo'ldi: %d", w.Code)
	}
}
