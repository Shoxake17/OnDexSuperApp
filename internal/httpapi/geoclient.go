package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Tashqi geo xizmatlariga (Google, Yandex, 2GIS) so'rov yuborishning
// YAGONA yo'li.
//
// ┌─ TUZATILGAN NOSOZLIK (bug.md 21-band) ─────────────────────────────┐
// Oltita joyda `http.DefaultClient.Do(req)` chaqirilardi va uch xato
// har birida takrorlangan edi:
//
//  1. **TIMEOUT YO'Q.** `http.DefaultClient` ning `Timeout` maydoni
//     NOL — ya'ni cheksiz kutadi. Google/Yandex/2GIS sekin javob
//     bersa yoki ulanish osilib qolsa, goroutine va mijoz ulanishi
//     CHEKSIZ band bo'lardi. Bir necha o'nlab bunday so'rov serverni
//     bo'g'ardi.
//
//  2. **XATO E'TIBORSIZ QOLDIRILARDI:**
//     `req, _ := http.NewRequestWithContext(...)` — `req` nil bo'lsa
//     keyingi qator nil-pointer panic berardi.
//
//  3. **JAVOB HAJMI CHEKLANMAGAN:** `json.NewDecoder(resp.Body)` —
//     loyihaning boshqa joylarida `io.LimitReader(..., 1<<20)`
//     ishlatiladi, bu yerda yo'q edi.
//
// Uchalasi ham shu faylda, BIR MARTA hal qilingan.
//
// So'rov konteksti ham uzatiladi: mijoz ulanishni uzsa, tashqi
// so'rov ham darhol bekor qilinadi (`Timeout` dan tashqari ikkinchi
// chegara).
// └────────────────────────────────────────────────────────────────────┘

// geoHTTPTimeout — tashqi geo xizmatlari uchun umumiy chegara.
//
// 10 soniya ATAYLAB: Google Geocoding odatda 200-500 ms javob
// beradi, ya'ni 10s — "aniq nosozlik" chegarasi, oddiy kutish emas.
// Undan uzunroq qiymat mijoz ekranida uzoq muzlashga aylanadi.
const geoHTTPTimeout = 10 * time.Second

// geoMaxResponseBytes — tashqi javobning eng katta hajmi (1 MiB).
// Haqiqiy javoblar bir necha KB; bu chegara buzilgan yoki zararli
// javob xotirani to'ldirishining oldini oladi.
const geoMaxResponseBytes = 1 << 20

// geoClient — TIMEOUT bilan. Paket darajasida bitta nusxa: `http.Client`
// goroutine-xavfsiz va ulanishlarni qayta ishlatadi (har so'rovga yangi
// klient yaratish TCP/TLS ulanishlarini isrof qilardi).
var geoClient = &http.Client{Timeout: geoHTTPTimeout}

// fetchGeoJSON — tashqi geo xizmatidan JSON oladi va `out` ga yozadi.
//
// Xatolar birlashtirilgan: chaqiruvchi ularni odatda 502 (Bad Gateway)
// bilan qaytaradi — sabab bizda emas, tashqi xizmatda.
func fetchGeoJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // G704 emas: URL faqat kod ichidagi o'zgarmas geo xizmat hostlaridan quriladi
	if err != nil {
		// Avval bu xato `_` bilan tashlanardi va `req` nil bo'lsa
		// keyingi qator panic berardi.
		return fmt.Errorf("geo so'rovini qurib bo'lmadi: %w", err)
	}
	resp, err := geoClient.Do(req) //nolint:gosec // G704 emas: yuqoridagi izohga qarang
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Hajm chegarasi: buzilgan yoki juda katta javob xotirani
	// to'ldirmasin.
	if err := json.NewDecoder(io.LimitReader(resp.Body, geoMaxResponseBytes)).
		Decode(out); err != nil {
		return fmt.Errorf("geo javobini o'qib bo'lmadi: %w", err)
	}
	return nil
}
