package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicCORSWorks(t *testing.T) {
	cases := []struct {
		name  string
		allow string
		want  bool
	}{
		{"wildcard", "*", true},
		{"exact", "https://ondex.uz", true},
		{"none", "", false},
		{"other", "https://evil.example", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodOptions || r.Header.Get("Origin") != "https://ondex.uz" {
					t.Errorf("preflight kutilgan edi: %s %q", r.Method, r.Header.Get("Origin"))
				}
				if c.allow != "" {
					w.Header().Set("Access-Control-Allow-Origin", c.allow)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()
			if got := publicCORSWorks(context.Background(), srv.URL+"/"); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
	if publicCORSWorks(context.Background(), "") {
		t.Fatal("bo'sh URL true bermasligi kerak")
	}
}
