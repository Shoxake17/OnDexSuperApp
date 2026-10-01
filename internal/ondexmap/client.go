// Package ondexmap — OnDexMap (maps.ondex.uz) `/v1` API'ga server-server mijoz.
//
// ChustApp OnDexMap'dan faqat shu paket orqali foydalanadi: qidiruv,
// koordinata → manzil va yo'l marshruti. Paket uch narsani kafolatlaydi:
//
//   - qat'iy vaqt va hajm chegarasi (osilgan yoki ulkan javob serverni
//     bo'g'maydi);
//   - ikki xil xato: ErrNotCovered ("bu nuqta bizda yo'q" — chaqiruvchi
//     Google'ga o'tadi, lekin OnDexMap SOG'LOM) va ErrUnavailable
//     (OnDexMap yiqilgan — shuningdek Google'ga o'tiladi);
//   - zanjirli to'xtatgich (circuit breaker): ketma-ket xatolardan keyin
//     qisqa muddat so'rov umuman yuborilmaydi, shunda yiqilgan xizmatni
//     har foydalanuvchi so'rovida 4 soniya kutmaymiz.
package ondexmap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultBase — production OnDexMap manzili.
	DefaultBase = "https://maps.ondex.uz"

	httpTimeout = 4 * time.Second
	// maxBody — javob hajmi chegarasi. Marshrut geometriyasi eng kattasi
	// (uzoq yo'l ~ bir necha yuz KB).
	maxBody = 8 << 20

	// Zanjirli to'xtatgich: breakerThreshold ta ketma-ket xatodan keyin
	// breakerCooldown davomida so'rov yuborilmaydi.
	breakerThreshold = 5
	breakerCooldown  = 30 * time.Second
)

var (
	// ErrNotCovered — OnDexMap javob berdi, lekin bu so'rov uchun ma'lumoti
	// yo'q (404, yoki nuqta xizmat hududidan tashqarida: 400).
	ErrNotCovered = errors.New("ondexmap: ma'lumot yo'q")
	// ErrUnavailable — tarmoq xatosi, vaqt tugashi, 5xx/429 yoki to'xtatgich ochiq.
	ErrUnavailable = errors.New("ondexmap: xizmat ishlamayapti")
)

// Point — koordinata.
type Point struct{ Lat, Lng float64 }

// Match — qidiruv natijasi (OnDexMap `/v1/search`).
type Match struct {
	ID    string   `json:"id"`
	Type  string   `json:"type"`
	Name  string   `json:"name"`
	Label string   `json:"label"`
	Near  string   `json:"near"`
	Lat   *float64 `json:"lat"`
	Lng   *float64 `json:"lng"`
	// BBox — [g'arb, janub, sharq, shimol]: koordinatasiz ko'cha/bino uchun.
	BBox *[4]float64 `json:"bbox"`
}

// Center — natijaga uchish nuqtasi: koordinata, bo'lmasa bbox markazi.
func (m Match) Center() (lat, lng float64, ok bool) {
	switch {
	case m.Lat != nil && m.Lng != nil:
		return *m.Lat, *m.Lng, true
	case m.BBox != nil:
		return (m.BBox[1] + m.BBox[3]) / 2, (m.BBox[0] + m.BBox[2]) / 2, true
	}
	return 0, 0, false
}

// Place — koordinata bo'yicha aniqlangan joy (`/v1/resolve`).
type Place struct {
	Text string `json:"text"`
}

// Route — marshrut: nuqtalar, masofa (m), vaqt (s).
type Route struct {
	Points    []Point
	DistanceM float64
	DurationS float64
}

// API — ChustApp ishlatadigan OnDexMap amallari. Testlarda soxtasi beriladi.
type API interface {
	Search(ctx context.Context, q string, near *Point, limit int) ([]Match, error)
	Resolve(ctx context.Context, lat, lng float64) (Place, error)
	Route(ctx context.Context, from, to Point) (Route, error)
}

// Client — `API` ning HTTP amalga oshirilishi.
type Client struct {
	base string
	http *http.Client
	now  func() time.Time

	mu        sync.Mutex
	failures  int
	openUntil time.Time
}

// New — `base` bo'sh bo'lsa DefaultBase.
func New(base string) *Client {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		base = DefaultBase
	}
	return &Client{base: base, http: &http.Client{Timeout: httpTimeout}, now: time.Now}
}

// Base — OnDexMap manzili (ilovalarga `/config/maps` orqali beriladi).
func (c *Client) Base() string { return c.base }

func (c *Client) Search(ctx context.Context, q string, near *Point, limit int) ([]Match, error) {
	v := url.Values{"q": {q}, "limit": {strconv.Itoa(limit)}}
	if near != nil {
		v.Set("lat", strconv.FormatFloat(near.Lat, 'f', 6, 64))
		v.Set("lng", strconv.FormatFloat(near.Lng, 'f', 6, 64))
	}
	var out struct {
		Results []Match `json:"results"`
	}
	if err := c.get(ctx, "/v1/search?"+v.Encode(), &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

func (c *Client) Resolve(ctx context.Context, lat, lng float64) (Place, error) {
	v := url.Values{
		"lat": {strconv.FormatFloat(lat, 'f', 6, 64)},
		"lng": {strconv.FormatFloat(lng, 'f', 6, 64)},
	}
	var p Place
	if err := c.get(ctx, "/v1/resolve?"+v.Encode(), &p); err != nil {
		return Place{}, err
	}
	if strings.TrimSpace(p.Text) == "" {
		return Place{}, ErrNotCovered
	}
	return p, nil
}

func (c *Client) Route(ctx context.Context, from, to Point) (Route, error) {
	v := url.Values{
		"from_lat": {strconv.FormatFloat(from.Lat, 'f', 6, 64)},
		"from_lng": {strconv.FormatFloat(from.Lng, 'f', 6, 64)},
		"to_lat":   {strconv.FormatFloat(to.Lat, 'f', 6, 64)},
		"to_lng":   {strconv.FormatFloat(to.Lng, 'f', 6, 64)},
	}
	var raw struct {
		DistanceM float64 `json:"distance_m"`
		DurationS float64 `json:"duration_s"`
		Geometry  struct {
			Coordinates [][]float64 `json:"coordinates"`
		} `json:"geometry"`
	}
	if err := c.get(ctx, "/v1/route?"+v.Encode(), &raw); err != nil {
		return Route{}, err
	}
	pts := make([]Point, 0, len(raw.Geometry.Coordinates))
	for _, xy := range raw.Geometry.Coordinates {
		if len(xy) >= 2 { // GeoJSON: [lng, lat]
			pts = append(pts, Point{Lat: xy[1], Lng: xy[0]})
		}
	}
	if len(pts) < 2 {
		// Bo'sh geometriya — "yo'l bor" deb aldamasdan Google'ga o'tamiz.
		return Route{}, fmt.Errorf("%w: marshrut geometriyasi bo'sh", ErrUnavailable)
	}
	return Route{Points: pts, DistanceM: raw.DistanceM, DurationS: raw.DurationS}, nil
}

// get — bitta GET so'rov: to'xtatgich, vaqt/hajm chegarasi va xatolarni
// ErrNotCovered/ErrUnavailable'ga keltirish shu yerda.
func (c *Client) get(ctx context.Context, path string, out any) error {
	if !c.allow() {
		return ErrUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil) //nolint:gosec // G704 emas: host operator sozlamasi (ONDEXMAP_API_URL), yo'l qismi kodda
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ChustApp/1.0 (+server)")
	resp, err := c.http.Do(req) //nolint:gosec // G704 emas: yuqoridagi izohga qarang
	if err != nil {
		// Chaqiruvchi o'zi bekor qilgan so'rov OnDexMap aybi emas.
		if ctx.Err() == nil {
			c.record(false)
		}
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest:
		// OnDexMap sog'lom — bu faqat "bu nuqta bizda yo'q". To'xtatgichni
		// buzmaymiz.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		c.record(true)
		return ErrNotCovered
	case resp.StatusCode != http.StatusOK:
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		c.record(false)
		return fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out); err != nil {
		c.record(false)
		return fmt.Errorf("%w: javobni o'qib bo'lmadi", ErrUnavailable)
	}
	c.record(true)
	return nil
}

// allow — to'xtatgich yopiqmi (so'rov yuborish mumkinmi).
func (c *Client) allow() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.now().Before(c.openUntil)
}

// record — natijani to'xtatgichga yozadi.
func (c *Client) record(ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ok {
		c.failures = 0
		return
	}
	c.failures++
	if c.failures >= breakerThreshold {
		c.openUntil = c.now().Add(breakerCooldown)
		c.failures = 0
	}
}
