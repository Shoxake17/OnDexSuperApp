package httpapi

import (
	"net/http"
	"testing"
	"time"

	"chustapp/internal/users"
)

// TestLogoutRevokesEarlierTokens — `POST /auth/logout` shu foydalanuvchining
// OLDINROQ chiqarilgan tokenini bekor qiladi, boshqa foydalanuvchiga tegmaydi.
//
// Bu `TestAuthorizationMatrix` har endpoint uchun YANGI fikstura olishining
// sababini ham aniq ko'rsatadi: logout tokenlar chiqarilgan soniyadan KEYINGI
// soniyada sodir bo'lsa, o'sha foydalanuvchining keyingi barcha so'rovlari
// 401 oladi. Umumiy fikstura bilan matritsa shu sabab mashina tezligiga
// bog'liq holda yiqilardi (CI'da `-race` bilan).
//
// Keyingi soniyani ATAYLAB kutamiz — aks holda test ham o'sha tasodifga
// bog'liq bo'lib qolardi (bekor qilish belgisi soniyagacha yaxlitlanadi va
// aynan o'sha soniyada chiqarilgan token yaroqli qoladi — `revoke.Revoke`).
func TestLogoutRevokesEarlierTokens(t *testing.T) {
	h, jwt := authzServer(t)
	customer := jwt[users.RoleCustomer]
	if w := do(t, h, "GET", "/me", customer, ""); w.Code != http.StatusOK {
		t.Fatalf("logoutdan oldin /me: %d %s", w.Code, w.Body.String())
	}

	nextSecond := time.Now().Truncate(time.Second).Add(time.Second)
	time.Sleep(time.Until(nextSecond) + 20*time.Millisecond)

	if w := do(t, h, "POST", "/auth/logout", customer, `{}`); w.Code != http.StatusOK {
		t.Fatalf("logout: %d %s", w.Code, w.Body.String())
	}
	if w := do(t, h, "GET", "/me", customer, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("XAVFSIZLIK: chiqishdan keyin eski token ishladi: %d", w.Code)
	}
	if w := do(t, h, "POST", "/ws/ticket", customer, `{}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("XAVFSIZLIK: chiqishdan keyin eski token bilan WS bilet olindi: %d", w.Code)
	}
	// Boshqa foydalanuvchining sessiyasi tegilmaydi.
	if w := do(t, h, "GET", "/me", jwt[users.RoleAdmin], ""); w.Code != http.StatusOK {
		t.Fatalf("boshqa foydalanuvchi sessiyasi ham bekor bo'ldi: %d", w.Code)
	}
}
