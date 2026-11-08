package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Config — tekshiruv sozlamalari (flag/muhit orqali).
type Config struct {
	API, Web, POS string
	// Test akkauntlari (TEST_OTP): kod Play sharhlovchilari uchun ochiq.
	WaiterPhone, CourierPhone, TestCode string
	Timeout                             time.Duration
}

// Result — bitta tekshiruv natijasi.
type Result struct {
	Check    string
	OK       bool
	Err      string
	Duration time.Duration
}

type runner struct {
	cfg     Config
	http    *http.Client
	results []Result
}

func newRunner(cfg Config, client *http.Client) *runner {
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	return &runner{cfg: cfg, http: client}
}

// check — tekshiruvni bajaradi; xato bo'lsa BIR MARTA qayta urinadi
// (tarmoqning bir martalik uzilishi ogohlantirish bo'lmasin).
func (r *runner) check(name string, fn func() error) bool {
	start := time.Now()
	err := fn()
	if err != nil && !isPermanent(err) {
		time.Sleep(2 * time.Second)
		err = fn()
	}
	res := Result{Check: name, OK: err == nil, Duration: time.Since(start)}
	if err != nil {
		res.Err = err.Error()
	}
	r.results = append(r.results, res)
	return err == nil
}

// permanent — qayta urinish ma'nosiz (masalan kod bir martalik ishlatilgan).
type permanent struct{ error }

func isPermanent(err error) bool { _, ok := err.(permanent); return ok }

type resp struct {
	Status int
	Body   []byte
}

func (r *runner) do(method, url, bearer string, body any) (*resp, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, url, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ondex-smoke/1")
	req.Header.Set("X-Ondex-Client", "smoke")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := r.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tarmoq: %w", err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return &resp{Status: res.StatusCode, Body: b}, nil
}

// call — so'rov + kutilgan HTTP holati.
func (r *runner) call(method, url, bearer string, body any, want int) (*resp, error) {
	rs, err := r.do(method, url, bearer, body)
	if err != nil {
		return nil, err
	}
	if rs.Status != want {
		msg := ""
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(rs.Body, &e) == nil {
			msg = e.Error
		}
		return rs, fmt.Errorf("HTTP %d (kutilgan %d) %s", rs.Status, want, msg)
	}
	return rs, nil
}

// Run — barcha tekshiruvlar.
func (r *runner) Run() []Result {
	api := strings.TrimRight(r.cfg.API, "/")

	r.check("catalog", func() error {
		rs, err := r.call("GET", api+"/restaurants", "", nil, 200)
		if err != nil {
			return err
		}
		var list []map[string]any
		if json.Unmarshal(rs.Body, &list) != nil || len(list) == 0 {
			return fmt.Errorf("restoranlar ro'yxati bo'sh yoki JSON emas")
		}
		return nil
	})
	r.check("jwks", func() error {
		rs, err := r.call("GET", api+"/.well-known/jwks.json", "", nil, 200)
		if err != nil {
			return err
		}
		var j struct {
			Keys []map[string]any `json:"keys"`
		}
		if json.Unmarshal(rs.Body, &j) != nil || len(j.Keys) == 0 {
			return fmt.Errorf("kalit yo'q")
		}
		for _, k := range j.Keys {
			if _, secret := k["d"]; secret {
				return permanent{fmt.Errorf("JWKS MAXFIY kalitni oshkor qilyapti")}
			}
			if k["alg"] != "EdDSA" {
				return fmt.Errorf("alg %v", k["alg"])
			}
		}
		return nil
	})
	if r.cfg.Web != "" {
		r.check("web", func() error { _, err := r.call("GET", r.cfg.Web, "", nil, 200); return err })
	}
	if r.cfg.POS != "" {
		r.check("pos", func() error { _, err := r.call("GET", r.cfg.POS, "", nil, 200); return err })
	}
	if r.cfg.WaiterPhone != "" {
		r.staff("pro", r.cfg.WaiterPhone, "ondex-pro", "waiter", func(_ string) string { return "/waiter/me" })
	}
	if r.cfg.CourierPhone != "" {
		r.staff("go", r.cfg.CourierPhone, "ondex-go", "courier", func(entity string) string { return "/couriers/" + entity })
	}
	return r.results
}

// staff — xodim to'liq hayot sikli: kod -> markaziy sessiya -> eski
// marshrut (ko'prik) -> refresh -> chiqish -> eski token o'lik.
func (r *runner) staff(p, phone, client, role string, route func(entity string) string) {
	api := strings.TrimRight(r.cfg.API, "/")
	var access, refresh, entity string

	if !r.check(p+".login", func() error {
		if _, err := r.call("POST", api+"/auth/request-code", "", map[string]any{"phone": phone}, 200); err != nil {
			return fmt.Errorf("kod so'rash: %w", err)
		}
		rs, err := r.call("POST", api+"/v2/auth/merchant/otp/verify", "", map[string]any{
			"phone": phone, "code": r.cfg.TestCode, "client_id": client, "device_name": "smoke"}, 200)
		if err != nil {
			return permanent{fmt.Errorf("kirish: %w", err)} // kod ishlatildi — qayta urinish yangi kod talab qiladi
		}
		var t struct {
			Access  string `json:"access_token"`
			Refresh string `json:"refresh_token"`
			MFA     bool   `json:"mfa_required"`
		}
		if json.Unmarshal(rs.Body, &t) != nil || t.Access == "" || t.Refresh == "" || t.MFA {
			return permanent{fmt.Errorf("token juftligi kelmadi")}
		}
		access, refresh = t.Access, t.Refresh
		return nil
	}) {
		return
	}
	r.check(p+".bridge", func() error {
		rs, err := r.call("GET", api+"/me", access, nil, 200)
		if err != nil {
			return err
		}
		var u struct {
			Role     string `json:"role"`
			EntityID string `json:"entity_id"`
		}
		_ = json.Unmarshal(rs.Body, &u)
		if u.Role != role {
			return permanent{fmt.Errorf("rol %q, kutilgan %q", u.Role, role)}
		}
		entity = u.EntityID
		_, err = r.call("GET", api+route(entity), access, nil, 200)
		return err
	})
	r.check(p+".refresh", func() error {
		rs, err := r.call("POST", api+"/v2/auth/token/refresh", "", map[string]any{"refresh_token": refresh}, 200)
		if err != nil {
			return permanent{err}
		}
		var t struct {
			Access  string `json:"access_token"`
			Refresh string `json:"refresh_token"`
		}
		if json.Unmarshal(rs.Body, &t) != nil || t.Access == "" || t.Refresh == refresh {
			return permanent{fmt.Errorf("refresh yangi juftlik bermadi")}
		}
		access, refresh = t.Access, t.Refresh
		_, err = r.call("GET", api+"/me", access, nil, 200)
		return err
	})
	r.check(p+".logout", func() error {
		if _, err := r.call("POST", api+"/v2/auth/logout", access, nil, 204); err != nil {
			return permanent{err}
		}
		_, err := r.call("GET", api+"/me", access, nil, 401)
		if err != nil {
			return permanent{fmt.Errorf("chiqishdan keyin token hali ishlayapti: %w", err)}
		}
		return nil
	})
}

var labelUnsafe = regexp.MustCompile(`[^\p{L}\p{N} .,:;()/_%+\-']`)

// sanitize — Prometheus yorlig'i uchun qisqa, xavfsiz matn (token/raqam sizmasin).
func sanitize(s string) string {
	s = regexp.MustCompile(`(ort_|omfa_|eyJ)[A-Za-z0-9_\-.]+`).ReplaceAllString(s, "<token>")
	s = regexp.MustCompile(`\+?998\d{9}`).ReplaceAllString(s, "<tel>")
	s = labelUnsafe.ReplaceAllString(s, " ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:120])
	}
	return strings.TrimSpace(s)
}

// Metrics — node_exporter textfile formati.
func Metrics(results []Result, now time.Time) string {
	var b strings.Builder
	b.WriteString("# HELP ondex_smoke_check_success 1 = tekshiruv o'tdi, 0 = yiqildi (error yorlig'ida sabab).\n")
	b.WriteString("# TYPE ondex_smoke_check_success gauge\n")
	sorted := append([]Result(nil), results...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Check < sorted[j].Check })
	failed := 0
	for _, r := range sorted {
		v := 1
		if !r.OK {
			v, failed = 0, failed+1
		}
		fmt.Fprintf(&b, "ondex_smoke_check_success{check=%q,error=%q} %d\n", r.Check, sanitize(r.Err), v)
	}
	b.WriteString("# HELP ondex_smoke_check_duration_seconds Tekshiruv davomiyligi.\n# TYPE ondex_smoke_check_duration_seconds gauge\n")
	for _, r := range sorted {
		fmt.Fprintf(&b, "ondex_smoke_check_duration_seconds{check=%q} %.3f\n", r.Check, r.Duration.Seconds())
	}
	fmt.Fprintf(&b, "# HELP ondex_smoke_failed_checks Yiqilgan tekshiruvlar soni.\n# TYPE ondex_smoke_failed_checks gauge\nondex_smoke_failed_checks %d\n", failed)
	fmt.Fprintf(&b, "# HELP ondex_smoke_last_run_timestamp_seconds Oxirgi ishga tushish.\n# TYPE ondex_smoke_last_run_timestamp_seconds gauge\nondex_smoke_last_run_timestamp_seconds %d\n", now.Unix())
	return b.String()
}
