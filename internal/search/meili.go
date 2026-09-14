// Package search — MeiliSearch integratsiyasi: taom (mahsulot) nomi va
// turkumi bo'yicha tez, xato-kechiruvchan (typo-tolerant) qidiruv.
//
// ┌─ NEGA IXTIYORIY, MONGO'DAN ALOHIDA ────────────────────────────────┐
// Bu paket MANBA EMAS — u faqat Mongo'dagi mahsulotlarning izlanadigan
// nusxasini saqlaydi (id, restoran_id, nom, turkum, mavjudlik). Haqiqiy
// ma'lumot (narx, rasm, ombor va h.k.) har doim Mongo'dan o'qiladi —
// bitta manba tamoyili buzilmaydi, indeks faqat "qaysi ID'lar mos
// keladi" degan savolga javob beradi.
//
// `MEILI_HOST` bo'sh bo'lsa butun paket ishlatilmaydi va qidiruv eski
// usulga (`MongoCatalogRepo` ichidagi qo'lda substring skaneri)
// qaytadi — R2/Redis/Telegram kabi boshqa ixtiyoriy komponentlar bilan
// bir xil falsafa: bu xizmat hech qachon serverni to'xtatmaydi, faqat
// bo'lganda tezlashtiradi va xato-kechiruvchan qidiruv beradi.
//
// Rasmiy Go SDK ATAYLAB ishlatilmaydi: API juda kichik (indeks yaratish,
// hujjat qo'shish/o'chirish, qidiruv — jami olti so'rov), qo'shimcha
// bog'liqlik esa yangi ta'minot zanjiri xavfi bo'lardi. Loyihadagi
// boshqa tashqi provayder klientlari ham (masalan
// `internal/model3d/tripo.go`) xuddi shunday qo'lda yozilgan.
// └────────────────────────────────────────────────────────────────────┘
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// IndexUID — mahsulotlar uchun ishlatiladigan Meilisearch indeksi nomi.
const IndexUID = "products"

// maxResults — Search() qaytaradigan eng ko'p ID soni. `mongo_catalog.go`
// dagi `maxSearchResults` bilan bir xil chegara — natija hajmi ikkala
// yo'lda ham bir xil qoladi.
const maxResults = 100

// ProductDoc — Meilisearch'dagi bitta mahsulot yozuvi. Faqat QIDIRISH
// uchun kerakli maydonlar: narx, rasm, ombor va boshqa hamma narsa har
// doim Mongo'dan olinadi (indeks natijasi faqat ID beradi, `PublicView`
// himoyasi ham shu sabab bu yerga umuman kirmaydi).
type ProductDoc struct {
	ID           string `json:"id"`
	RestaurantID string `json:"restaurant_id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	Available    bool   `json:"available"`
}

// Client — self-hosted Meilisearch'ga minimal HTTP klient.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("meilisearch bilan aloqa yo'q (%s ishga tushganmi?): %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("meilisearch %s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

// EnsureIndex — indeksni yaratadi (bor bo'lsa tegilmaydi) va
// qidiriladigan/filtrlanadigan maydonlarni sozlaydi. Server ishga
// tushganda BIR MARTA chaqiriladi.
//
// Sozlash so'rovlari Meilisearch'da ASINXRON navbatga tushadi (task),
// lekin bu yerda natija kutilmaydi: ular tez (bir necha o'n
// millisekund) bajariladi va keyingi hujjat qo'shish/qidirish
// so'rovlari baribir navbat tartibida ishlanadi — muvaffaqiyatsizlik
// xavfi yo'q, faqat ishga tushishda bir zumlik kechikish bo'lishi
// mumkin (amaliy ahamiyatsiz).
func (c *Client) EnsureIndex(ctx context.Context) error {
	var existing struct {
		UID string `json:"uid"`
	}
	if err := c.do(ctx, http.MethodGet, "/indexes/"+IndexUID, nil, &existing); err != nil {
		if createErr := c.do(ctx, http.MethodPost, "/indexes", map[string]string{
			"uid":        IndexUID,
			"primaryKey": "id",
		}, nil); createErr != nil {
			return fmt.Errorf("indeks yaratilmadi: %w", createErr)
		}
	}
	if err := c.do(ctx, http.MethodPut, "/indexes/"+IndexUID+"/settings/searchable-attributes",
		[]string{"name", "category"}, nil); err != nil {
		return fmt.Errorf("qidiriladigan maydonlar sozlanmadi: %w", err)
	}
	if err := c.do(ctx, http.MethodPut, "/indexes/"+IndexUID+"/settings/filterable-attributes",
		[]string{"available", "restaurant_id"}, nil); err != nil {
		return fmt.Errorf("filtr maydonlari sozlanmadi: %w", err)
	}
	if err := c.do(ctx, http.MethodPut, "/indexes/"+IndexUID+"/settings/synonyms",
		synonymSettings(), nil); err != nil {
		return fmt.Errorf("sinonimlar sozlanmadi: %w", err)
	}
	return nil
}

// synonymGroups — talaffuzga asoslangan yozuv variantlari (translit).
//
// ┌─ NEGA BULAR TYPO TOLERANCE BILAN TUZATILMAYDI ─────────────────────┐
// Meilisearch'ning xato-kechirish qoidasi 5 harfdan qisqa so'zga HECH
// QANDAY moslik bermaydi ("Cola"/"Kola" — 4 harf), va 2 ta harf farq
// qiladigan so'zlarga ham (masalan "Coca"/"Koka" — ikkalasi ham C→K,
// c→k) faqat 9+ harfli so'zlarda ishlaydi. Bundan tashqari bu —
// klaviaturada adashib yozilgan XATO emas, balki inglizcha nomning
// o'zbekcha-ruscha TALAFFUZ bo'yicha yozilishi — edit-distance
// algoritmi bunday holatlar uchun mo'ljallanmagan.
//
// Shuning uchun aniq lug'at kerak. Har qator — bitta tushuncha uchun
// BARCHA yozilish variantlari; `synonymSettings()` ularni ikki
// yo'nalishli (har biri qolganlarining sinonimi) Meilisearch shakliga
// aylantiradi — katalogda qaysi yozilishi ishlatilishidan qat'i nazar
// ishlaydi.
//
// Yangi variant qo'shish uchun shu ro'yxatga bitta qator yetadi —
// boshqa hech narsa o'zgartirish shart emas.
// └────────────────────────────────────────────────────────────────────┘
var synonymGroups = [][]string{
	{"coca cola", "koka kola", "кока кола", "кока-кола"},
	{"cola", "kola", "кола"},
	{"cappuccino", "kapuchino", "капучино"},
	{"mojito", "moxito", "мохито"},
	{"pizza", "pitsa", "пицца"},
	{"latte", "latte", "латте"},
	{"espresso", "эспрессо"},
	{"sprite", "sprayt", "спрайт"},
	{"fanta", "фанта"},
	{"burger", "burger", "бургер"},
	{"hot dog", "hotdog", "hot-dog", "хот-дог"},
	{"lagman", "lag'mon", "лагман"},
}

// synonymSettings — `synonymGroups`ni Meilisearch'ning "har so'z/ibora
// -> qolgan hammasi" shakliga aylantiradi.
func synonymSettings() map[string][]string {
	out := make(map[string][]string, len(synonymGroups)*2)
	for _, group := range synonymGroups {
		for i, word := range group {
			var rest []string
			for j, w := range group {
				if i != j {
					rest = append(rest, w)
				}
			}
			out[word] = rest
		}
	}
	return out
}

// IndexProducts — hujjatlarni qo'shadi yoki (ID mos kelsa) yangilaydi.
// Bitta mahsulot ham, to'liq qayta indekslash ham shu metoddan o'tadi.
func (c *Client) IndexProducts(ctx context.Context, docs []ProductDoc) error {
	if len(docs) == 0 {
		return nil
	}
	return c.do(ctx, http.MethodPost, "/indexes/"+IndexUID+"/documents", docs, nil)
}

// DeleteProduct — bitta mahsulotni indeksdan o'chiradi.
func (c *Client) DeleteProduct(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/indexes/"+IndexUID+"/documents/"+escapeID(id), nil, nil)
}

// DeleteByRestaurant — restoran o'chirilganda uning BARCHA taomlarini
// indeksdan o'chiradi (`restaurant_id` filtrlanadigan maydon sifatida
// `EnsureIndex`da sozlangan).
func (c *Client) DeleteByRestaurant(ctx context.Context, restaurantID string) error {
	filter := fmt.Sprintf("restaurant_id = %s", quoteFilterValue(restaurantID))
	return c.do(ctx, http.MethodPost, "/indexes/"+IndexUID+"/documents/delete", map[string]string{
		"filter": filter,
	}, nil)
}

// Search — nomi/turkumi so'rovga mos, MAVJUD (available=true)
// mahsulotlarning ID ro'yxatini, moslik darajasi bo'yicha saralangan
// holda qaytaradi. Haqiqiy mahsulot ma'lumoti chaqiruvchi tomonidan
// Mongo'dan (har doim yangi holatda) olinadi.
func (c *Client) Search(ctx context.Context, query string) ([]string, error) {
	var resp struct {
		Hits []struct {
			ID string `json:"id"`
		} `json:"hits"`
	}
	body := map[string]any{
		"q":                    query,
		"limit":                maxResults,
		"filter":               "available = true",
		"attributesToRetrieve": []string{"id"},
	}
	if err := c.do(ctx, http.MethodPost, "/indexes/"+IndexUID+"/search", body, &resp); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		ids = append(ids, h.ID)
	}
	return ids, nil
}

// escapeID — hujjat ID'ini URL yo'lida xavfsiz ishlatish uchun.
// Mahsulot ID'lari serverda generatsiya qilinadi (foydalanuvchi erkin
// matni emas), lekin himoya bepul va aniq.
func escapeID(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			// Kutilmagan belgi — indeks so'rovi hech qachon buzuq yo'lga
			// tushmasligi uchun butunlay tashlab yuboriladi (ID baribir
			// topilmaydi, xato emas, shunchaki bo'sh natija).
		}
	}
	return b.String()
}

// quoteFilterValue — Meilisearch filtr ifodasiga xavfsiz qo'yish uchun
// qiymatni tirnoqqa oladi (ichidagi tirnoq/backslash escape qilinadi).
func quoteFilterValue(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return `"` + v + `"`
}
