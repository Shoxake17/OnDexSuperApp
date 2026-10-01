// OCHIQ GET javoblar uchun HTTP kesh sarlavhalari: Cache-Control + ETag + 304.
//
// Nega: ochiq ro'yxatlar (restoranlar, menyu, aksiyalar) har mijoz ilovani
// ochganda so'raladi va javob hamma uchun bir xil. Sarlavhalar bilan
//   - brauzer/ilova shu so'rovni umuman yubormaydi (max-age ichida);
//   - Cloudflare/CDN uni chekkada javoblaydi (serverga yetib kelmaydi);
//   - muddat o'tganda ETag bilan `304 Not Modified` — tanasiz javob.
//
// ┌─ XAVFSIZLIK QOIDASI ───────────────────────────────────────────┐
// `public` FAQAT autentifikatsiyasiz (Authorization sarlavhasi YO'Q)
// va shaxsiylashtirilmagan (`for_me`) so'rovlarga beriladi. Kirgan
// foydalanuvchi javobida imzolangan maket havolasi bo'lishi mumkin
// (`scene_access.go`) — u umumiy keshga tushsa boshqalarga sizib
// chiqardi. Shuning uchun bunday javoblar `private, no-cache`
// (ETag bilan: trafik tejaladi, lekin umumiy keshga tushmaydi).
// Xato javoblar (4xx/5xx) `no-store`: vaqtinchalik xato keshda qolmasin.
//
// Vary: Origin (CORS javobni origin bo'yicha o'zgartiradi) va
// Authorization (yuqoridagi ikki xil javob kesh kalitida aralashmasin).
// └────────────────────────────────────────────────────────────────┘
package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
)

// bufferedResponse — handler javobini ushlab turadi, shunda ETag tanadan
// hisoblanadi. Ochiq ro'yxatlar kichik (KB'lar), bufer arzon.
type bufferedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedResponse) Header() http.Header         { return b.header }
func (b *bufferedResponse) WriteHeader(status int)      { b.status = status }
func (b *bufferedResponse) Write(p []byte) (int, error) { return b.body.Write(p) }

// publicCache — `next` javobiga kesh sarlavhalarini qo'shadi.
// `maxAge` soniyada; `swr` — stale-while-revalidate oynasi (0 bo'lsa yo'q).
func publicCache(maxAge, swr int, next http.HandlerFunc) http.HandlerFunc {
	public := "public, max-age=" + strconv.Itoa(maxAge)
	if swr > 0 {
		public += ", stale-while-revalidate=" + strconv.Itoa(swr)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			next(w, r)
			return
		}
		rec := &bufferedResponse{header: make(http.Header), status: http.StatusOK}
		next(rec, r)

		h := w.Header()
		for k, v := range rec.header {
			h[k] = v
		}
		h.Add("Vary", "Origin")
		h.Add("Vary", "Authorization")

		if rec.status != http.StatusOK {
			h.Set("Cache-Control", "no-store")
			w.WriteHeader(rec.status)
			w.Write(rec.body.Bytes())
			return
		}

		personal := r.Header.Get("Authorization") != "" || r.URL.Query().Get("for_me") != ""
		if personal {
			h.Set("Cache-Control", "private, no-cache")
		} else {
			h.Set("Cache-Control", public)
		}
		sum := sha256.Sum256(rec.body.Bytes())
		etag := `"` + hex.EncodeToString(sum[:12]) + `"`
		h.Set("ETag", etag)

		if etagMatches(r.Header.Get("If-None-Match"), etag) {
			h.Del("Content-Type")
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			w.Write(rec.body.Bytes())
		}
	}
}

// etagMatches — If-None-Match qiymatida (vergul bilan ro'yxat, `W/` zaif
// belgisi, `*`) bizning ETag bormi.
func etagMatches(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimPrefix(strings.TrimSpace(part), "W/")
		if part == "*" || part == etag {
			return true
		}
	}
	return false
}
