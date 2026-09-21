package httpapi

import (
	"context"
	"sort"
	"strings"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/stats"
)

// Superadmin statistikasi: BARCHA restoranlar bo'yicha yig'indilar.
//
// Davr (`?period=`): `today`, `7d`, `30d` (standart), `all`. Kunlar
// `stats.Location` (+05:00) bo'yicha — restoran statistikasi bilan bir xil.

const (
	periodToday = "today"
	period7d    = "7d"
	period30d   = "30d"
	periodAll   = "all"

	// dailyChartDays — grafikdagi kunlar soni (davrdan mustaqil).
	dailyChartDays = 14
)

// statsPeriod — `?period=` qiymatini [from, to) oralig'iga aylantiradi.
// Noma'lum qiymat standart (30 kun) ga tushadi — xato emas: eski/begona
// klient baribir ma'noli javob oladi.
func statsPeriod(raw string, now time.Time) (period string, from, to time.Time) {
	today := time.Date(now.In(stats.Location).Year(), now.In(stats.Location).Month(),
		now.In(stats.Location).Day(), 0, 0, 0, 0, stats.Location)
	to = today.AddDate(0, 0, 1)
	switch strings.TrimSpace(raw) {
	case periodToday:
		return periodToday, today, to
	case period7d:
		return period7d, today.AddDate(0, 0, -6), to
	case periodAll:
		// 2000-yil: platforma bundan oldin bo'lmagan. Nol vaqt (0001) ba'zi
		// drayverlarda muammo chiqaradi.
		return periodAll, time.Date(2000, 1, 1, 0, 0, 0, 0, stats.Location), to
	default:
		return period30d, today.AddDate(0, 0, -29), to
	}
}

type platformTotalsJSON struct {
	Orders         int   `json:"orders"`
	New            int   `json:"new"`
	InProgress     int   `json:"in_progress"`
	Accepted       int   `json:"accepted"`
	Completed      int   `json:"completed"`
	Cancelled      int   `json:"cancelled"`
	RevenueTiyin   int64 `json:"revenue_tiyin"`
	AvgCheckTiyin  int64 `json:"avg_check_tiyin"`
	CompletionRate int   `json:"completion_rate"` // foiz, 0..100 (bajarilgan / (bajarilgan + bekor))
}

type platformRestaurantJSON struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	LogoURL string `json:"logo_url"`
	Open    bool   `json:"open"`
	// Deleted — restoran katalogdan o'chirilgan, buyurtmalar tarixi qolgan.
	Deleted bool `json:"deleted,omitempty"`
	platformTotalsJSON
}

func toTotalsJSON(t stats.RestaurantTotals) platformTotalsJSON {
	out := platformTotalsJSON{
		Orders:       t.Orders,
		New:          t.New,
		InProgress:   t.InProgress,
		Accepted:     t.Accepted(),
		Completed:    t.Completed,
		Cancelled:    t.Cancelled,
		RevenueTiyin: t.RevenueTiyin,
	}
	if t.Completed > 0 {
		out.AvgCheckTiyin = t.RevenueTiyin / int64(t.Completed)
	}
	if done := t.Completed + t.Cancelled; done > 0 {
		out.CompletionRate = t.Completed * 100 / done
	}
	return out
}

// platformStats — javobning yangi qismi (jadval, jami, grafik, holatlar).
// Ombor `stats.PlatformSource` ni bilmasa, bo'sh (nol) natija qaytadi.
func (s *Server) platformStats(ctx context.Context, rawPeriod string, now time.Time,
	restaurants []*catalog.Restaurant) (map[string]any, error) {
	period, from, to := statsPeriod(rawPeriod, now)
	today := time.Date(now.In(stats.Location).Year(), now.In(stats.Location).Month(),
		now.In(stats.Location).Day(), 0, 0, 0, 0, stats.Location)
	q := stats.PlatformQuery{
		From: from, To: to,
		DailyFrom: today.AddDate(0, 0, -(dailyChartDays - 1)),
		DailyDays: dailyChartDays,
	}

	var data stats.PlatformData
	if src, ok := s.OrderRepo.(stats.PlatformSource); ok {
		var err error
		if data, err = src.PlatformStats(ctx, q); err != nil {
			return nil, err
		}
	} else {
		data = stats.PlatformData{Statuses: map[string]int{}, Daily: stats.NewDaily(q.DailyFrom, q.DailyDays)}
	}

	byID := make(map[string]stats.RestaurantTotals, len(data.Restaurants))
	for _, t := range data.Restaurants {
		byID[t.RestaurantID] = t
	}

	var total stats.RestaurantTotals
	rows := make([]platformRestaurantJSON, 0, len(restaurants)+1)
	seen := make(map[string]bool, len(restaurants))
	openCount := 0
	for _, r := range restaurants {
		seen[r.ID] = true
		t := byID[r.ID]
		total.Add(t)
		if r.Open {
			openCount++
		}
		rows = append(rows, platformRestaurantJSON{
			ID: r.ID, Name: r.Name, LogoURL: r.LogoURL, Open: r.Open,
			platformTotalsJSON: toTotalsJSON(t),
		})
	}
	// O'chirilgan restoranlarning buyurtmalari tarixda qoladi: ular jamiga
	// kiradi va jadvalda alohida qator bo'ladi — jadval yig'indisi kartalar
	// bilan mos tushsin.
	var orphan stats.RestaurantTotals
	for id, t := range byID {
		if !seen[id] {
			orphan.Add(t)
		}
	}
	if orphan.Orders > 0 {
		total.Add(orphan)
		rows = append(rows, platformRestaurantJSON{
			Deleted: true, platformTotalsJSON: toTotalsJSON(orphan),
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.RevenueTiyin != b.RevenueTiyin {
			return a.RevenueTiyin > b.RevenueTiyin
		}
		if a.Orders != b.Orders {
			return a.Orders > b.Orders
		}
		return a.Name < b.Name
	})

	statuses := data.Statuses
	if statuses == nil {
		statuses = map[string]int{}
	}
	fromOut := ""
	if period != periodAll {
		fromOut = from.Format("2006-01-02")
	}
	return map[string]any{
		"period":           period,
		"from":             fromOut,
		"to":               to.AddDate(0, 0, -1).Format("2006-01-02"),
		"totals":           toTotalsJSON(total),
		"restaurants":      rows,
		"statuses":         statuses,
		"daily":            data.Daily,
		"restaurants_open": openCount,
	}, nil
}
