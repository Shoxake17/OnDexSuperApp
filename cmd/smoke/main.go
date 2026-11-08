// smoke — production'ni foydalanuvchi o'rniga tekshiradigan dastur
// (Plane ONDEXECOSY-168).
//
// Har ishga tushishda haqiqiy so'rovlar bilan: katalog (Postgres+Mongo+Redis
// zanjiri), JWKS (maxfiy kalit oshkor emasligi), web va ShoxPos sahifalari,
// OnDexPro va OnDexGO xodimlarining TO'LIQ sessiya hayot sikli (test
// akkauntlari, TEST_OTP): kod -> markaziy sessiya -> eski marshrut ->
// refresh -> chiqish -> eski token o'lik.
//
// Natija Prometheus textfile formatida (stdout yoki -out fayl, atomik
// yoziladi). Alertmanager yiqilgan tekshiruvni Telegram'ga yuboradi.
//
//	go run ./cmd/smoke -api https://api.ondex.uz -web https://ondex.uz -pos https://merchant.ondex.uz \
//	   -waiter +998999999999 -courier +998888888888 -out /var/lib/node_exporter/textfile/ondex_smoke.prom
//
// Chiqish kodi: 0 — hammasi o'tdi, 1 — kamida bitta yiqildi (CI uchun).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	cfg := Config{}
	flag.StringVar(&cfg.API, "api", "https://api.ondex.uz", "API manzili")
	flag.StringVar(&cfg.Web, "web", "", "web sahifa (bo'sh — tekshirilmaydi)")
	flag.StringVar(&cfg.POS, "pos", "", "ShoxPos web (bo'sh — tekshirilmaydi)")
	flag.StringVar(&cfg.WaiterPhone, "waiter", "", "OnDexPro test raqami (TEST_OTP)")
	flag.StringVar(&cfg.CourierPhone, "courier", "", "OnDexGO test raqami (TEST_OTP)")
	flag.StringVar(&cfg.TestCode, "code", "666666", "TEST_OTP sobit kodi")
	flag.DurationVar(&cfg.Timeout, "timeout", 15*time.Second, "har so'rov muddati")
	out := flag.String("out", "", "textfile yo'li (bo'sh — stdout)")
	flag.Parse()

	results := newRunner(cfg, nil).Run()
	metrics := Metrics(results, time.Now())
	failed := 0
	for _, r := range results {
		status := "OK  "
		if !r.OK {
			status, failed = "FAIL", failed+1
		}
		fmt.Fprintf(os.Stderr, "%s %-12s %6dms %s\n", status, r.Check, r.Duration.Milliseconds(), sanitize(r.Err))
	}
	if *out == "" {
		fmt.Print(metrics)
	} else if err := writeAtomic(*out, metrics); err != nil {
		fmt.Fprintln(os.Stderr, "metrika yozilmadi:", err)
		os.Exit(2)
	}
	if failed > 0 {
		os.Exit(1)
	}
}

// writeAtomic — node_exporter yarim yozilgan faylni o'qimasin.
func writeAtomic(path, data string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".smoke-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.WriteString(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
