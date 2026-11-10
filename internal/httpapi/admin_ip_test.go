package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chustapp/internal/storage"
	"chustapp/internal/users"
)

func TestParseAdminNets(t *testing.T) {
	nets, err := ParseAdminNets(" 203.0.113.7, 10.0.0.0/8 ,2001:db8::/48,")
	if err != nil || len(nets) != 3 || nets[0].Bits() != 32 {
		t.Fatalf("%v %v", nets, err)
	}
	if n, err := ParseAdminNets(""); err != nil || len(n) != 0 {
		t.Fatalf("bo'sh: %v %v", n, err)
	}
	for _, bad := range []string{"10.0.0.0/33", "salom", "1.2.3.4/abc"} {
		if _, err := ParseAdminNets(bad); err == nil {
			t.Errorf("noto'g'ri qiymat qabul qilindi: %q", bad)
		}
	}
}

// CC-0.2: cheklov yoqilsa admin tokeni faqat ruxsat etilgan tarmoqdan;
// boshqa rollarga ta'sir qilmaydi; cheklov o'chiq — hammasi avvalgidek.
func TestAdminIPRestriction(t *testing.T) {
	repo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)
	mk := func(id string, role users.Role) string {
		u := &users.User{ID: id, Phone: "+99890000" + id[len(id)-4:], Role: role, PhoneVerified: true, CreatedAt: time.Now()}
		if err := repo.Create(t.Context(), u); err != nil {
			t.Fatal(err)
		}
		tok, err := tokens.Issue(u)
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	admin, cust := mk("u-adm-0001", users.RoleAdmin), mk("u-cus-0002", users.RoleCustomer)
	nets, _ := ParseAdminNets("203.0.113.0/24")
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) })

	try := func(s *Server, tok, ip string) int {
		r := httptest.NewRequest("GET", "/admin/x", nil)
		r.RemoteAddr = ip + ":5555"
		r.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		s.auth(nil, ok)(w, r)
		return w.Code
	}
	on := New(Deps{UserRepo: repo, Tokens: tokens, AdminAllowedNets: nets, DevMode: true})
	off := New(Deps{UserRepo: repo, Tokens: tokens, DevMode: true})

	if got := try(on, admin, "203.0.113.9"); got != 204 {
		t.Errorf("ruxsat etilgan tarmoqdan admin: %d", got)
	}
	if got := try(on, admin, "198.51.100.1"); got != 403 {
		t.Errorf("XAVFSIZLIK: begona tarmoqdan admin o'tdi: %d", got)
	}
	if got := try(on, cust, "198.51.100.1"); got != 204 {
		t.Errorf("mijozga cheklov ta'sir qilmasligi kerak: %d", got)
	}
	if got := try(off, admin, "198.51.100.1"); got != 204 {
		t.Errorf("cheklov o'chiq bo'lsa admin ochiq: %d", got)
	}
}
