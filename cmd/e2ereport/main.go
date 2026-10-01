// Buyruq e2ereport — E2E oqim testlarining JSON hisobotini HTML ga aylantiradi.
//
//	go run ./cmd/e2ereport                       # logs/e2e/report.json -> report.html
//	go run ./cmd/e2ereport -in x.json -out x.html
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"chustapp/internal/e2e/report"
)

func main() {
	in := flag.String("in", filepath.Join("logs", "e2e", "report.json"), "JSON hisobot")
	out := flag.String("out", "", "HTML chiqishi (standart: JSON yonida .html)")
	flag.Parse()

	rep, err := report.Load(*in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hisobotni o'qib bo'lmadi:", err)
		os.Exit(1)
	}
	dst := *out
	if dst == "" {
		dst = strings.TrimSuffix(*in, filepath.Ext(*in)) + ".html"
	}
	if err := os.WriteFile(dst, []byte(report.HTML(rep)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "yozib bo'lmadi:", err)
		os.Exit(1)
	}
	fmt.Println(dst)
}
