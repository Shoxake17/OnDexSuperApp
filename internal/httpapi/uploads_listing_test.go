package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// Audit 480: lokal disk rejimida /uploads/ papka ro'yxatini bermasligi kerak.
func TestNoDirListing(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "books"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "books", "a.pdf"), []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := http.StripPrefix("/uploads/", noDirListing(http.FileServer(http.Dir(dir))))
	cases := map[string]int{
		"/uploads/":            http.StatusNotFound,
		"/uploads/books/":      http.StatusNotFound,
		"/uploads/books/a.pdf": http.StatusOK,
	}
	for path, want := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("%s: %d, kutilgan %d", path, rec.Code, want)
		}
	}
}
