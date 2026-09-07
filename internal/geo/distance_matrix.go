// Package geo — Google Distance Matrix API orqali HAQIQIY yo'l bo'yicha
// (transport turi/tirbandlikni hisobga olgan) ETA hisoblash. Dispatch
// matching engine shuni ishlatadi (couriers/scoring.go, couriers/dispatch.go).
package geo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LatLng struct {
	Lat float64
	Lng float64
}

// Mode — Google Distance Matrix "mode" parametri.
type Mode string

const (
	ModeDriving   Mode = "driving"
	ModeWalking   Mode = "walking"
	ModeBicycling Mode = "bicycling"
)

// Candidate — ETA so'ralishi kerak bo'lgan bitta nomzod (odatda — bitta kuryer).
type Candidate struct {
	ID       string
	Location LatLng
	Mode     Mode
}

// Result — bitta nomzod uchun natija. OK=false bo'lsa (masalan Google
// "ZERO_RESULTS" qaytarsa — yo'l topilmadi), chaqiruvchi shu nomzodni
// e'tiborsiz qoldirishi kerak.
type Result struct {
	ID       string
	Duration time.Duration
	Distance float64 // metr
	OK       bool
}

var (
	ErrNoAPIKey = errors.New("google maps api kaliti berilmagan")
)

// maxResponseBytes — tashqi javobning eng katta hajmi (1 MiB).
//
// Loyihadagi barcha tashqi HTTP javoblari uchun bir xil chegara
// (`notify/eskiz.go`, `notify/fcm.go`, `httpapi/geoclient.go`).
// Haqiqiy Distance Matrix javobi bir necha KB.
const maxResponseBytes = 1 << 20

type Client struct {
	apiKey     string
	baseURL    string // testlarda httptest.Server bilan almashtiriladi
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		baseURL:    "https://maps.googleapis.com/maps/api/distancematrix/json",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

// FetchETAs — bir nechta nomzoddan BITTA manzilgacha (odatda restoran) real
// yo'l bo'yicha ETA'ni Google Distance Matrix API orqali oladi.
//
// Nomzodlar transport turiga (Mode) qarab guruhlanadi — Google bitta
// so'rovda faqat BITTA mode qabul qiladi, shuning uchun har bir mode-guruh
// uchun alohida, lekin GURUH ICHIDA barcha nomzodlar uchun BITTA (batched,
// ko'p-origins/bitta-destination) so'rov yuboriladi. Bu ham xarajatni, ham
// kechikishni minimallashtiradi: N ta alohida so'rov o'rniga bor-yo'g'i
// nechta noyob transport turi bo'lsa, shuncha so'rov (odatda 1-3 ta).
func (c *Client) FetchETAs(ctx context.Context, dest LatLng, candidates []Candidate) ([]Result, error) {
	if c.apiKey == "" {
		return nil, ErrNoAPIKey
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	byMode := make(map[Mode][]Candidate)
	var modeOrder []Mode
	for _, cand := range candidates {
		if _, seen := byMode[cand.Mode]; !seen {
			modeOrder = append(modeOrder, cand.Mode)
		}
		byMode[cand.Mode] = append(byMode[cand.Mode], cand)
	}

	var out []Result
	for _, mode := range modeOrder {
		results, err := c.fetchGroup(ctx, dest, mode, byMode[mode])
		if err != nil {
			return nil, fmt.Errorf("mode %s: %w", mode, err)
		}
		out = append(out, results...)
	}
	return out, nil
}

func (c *Client) fetchGroup(ctx context.Context, dest LatLng, mode Mode, group []Candidate) ([]Result, error) {
	origins := make([]string, len(group))
	for i, cand := range group {
		origins[i] = fmt.Sprintf("%f,%f", cand.Location.Lat, cand.Location.Lng)
	}
	q := url.Values{}
	q.Set("origins", strings.Join(origins, "|"))
	q.Set("destinations", fmt.Sprintf("%f,%f", dest.Lat, dest.Lng))
	q.Set("mode", string(mode))
	q.Set("key", c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// ┌─ TUZATILGAN NOSOZLIK (bug.md 14-band) ────────────────────────┐
	// Bu yerda AVVAL chegarasiz `io.ReadAll(resp.Body)` turardi.
	// Loyihadagi BOSHQA barcha tashqi HTTP javoblari
	// `io.LimitReader(resp.Body, 1<<20)` bilan o'qiladi
	// (`notify/eskiz.go`, `notify/fcm.go`) — bu yagona istisno edi.
	//
	// Google javob bergani uchun amaldagi xavf past, lekin DNS yoki
	// proksi buzilgan holatda cheksiz javob xotiraga to'liq
	// yuklanardi. Bir qatorlik nomuvofiqlik.
	// └───────────────────────────────────────────────────────────────┘
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, err
	}

	var parsed distanceMatrixResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("javobni o'qib bo'lmadi: %w", err)
	}
	if parsed.Status != "OK" {
		return nil, fmt.Errorf("google distance matrix status: %s", parsed.Status)
	}
	if len(parsed.Rows) != len(group) {
		return nil, fmt.Errorf("kutilmagan javob: %d qator, %d nomzod kutilgan edi", len(parsed.Rows), len(group))
	}

	results := make([]Result, len(group))
	for i, row := range parsed.Rows {
		if len(row.Elements) == 0 {
			results[i] = Result{ID: group[i].ID}
			continue
		}
		el := row.Elements[0]
		results[i] = Result{
			ID:       group[i].ID,
			OK:       el.Status == "OK",
			Duration: time.Duration(el.Duration.Value) * time.Second,
			Distance: float64(el.Distance.Value),
		}
	}
	return results, nil
}

type distanceMatrixResponse struct {
	Status string `json:"status"`
	Rows   []struct {
		Elements []struct {
			Status   string `json:"status"`
			Duration struct {
				Value int `json:"value"` // soniya
			} `json:"duration"`
			Distance struct {
				Value int `json:"value"` // metr
			} `json:"distance"`
		} `json:"elements"`
	} `json:"rows"`
}
