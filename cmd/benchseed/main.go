// Buyruq benchseed — YUKLAMA TESTI uchun sun'iy mijozlar va ularning tokenlarini
// yaratadi (yozish yo'lini o'lchash: `tests/load/orders.js`).
//
// Nega kerak: haqiqiy kirish OTP orqali va IP bo'yicha ~5 ta urinish bilan
// cheklangan (`otpIPLimiter`), minglab test foydalanuvchini shu yo'l bilan
// yaratib bo'lmaydi. Bu vosita FAQAT izolyatsiyalangan benchmark bazasiga
// (`scripts/bench.ps1`) yozadi va tokenni o'sha muhitning JWT siri bilan
// imzolaydi.
//
//	go run ./cmd/benchseed -db "postgres://..." -secret "..." -n 2000 -out logs/bench/tokens.json
//
// ⚠️ Production bazasiga ISHLATMANG: u sun'iy akkauntlar yaratadi.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/storage"
	"chustapp/internal/users"
)

func main() {
	dsn := flag.String("db", "", "Postgres DSN (benchmark bazasi)")
	secret := flag.String("secret", "", "JWT siri (API ishga tushirilgan sir bilan bir xil)")
	n := flag.Int("n", 1000, "mijozlar soni")
	out := flag.String("out", filepath.Join("logs", "bench", "tokens.json"), "tokenlar JSON fayli")
	flag.Parse()

	// Himoya: faqat lokal baza (benchmark konteyneri). Begona manzilga yozmaymiz.
	if !strings.Contains(*dsn, "127.0.0.1") && !strings.Contains(*dsn, "localhost") {
		fmt.Fprintln(os.Stderr, "RAD ETILDI: baza lokal emas — benchseed faqat izolyatsiyalangan benchmark bazasi uchun")
		os.Exit(2)
	}
	if *dsn == "" || *secret == "" || *n <= 0 {
		fmt.Fprintln(os.Stderr, "-db, -secret va -n majburiy")
		os.Exit(2)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "baza:", err)
		os.Exit(1)
	}
	defer pool.Close()

	repo := storage.NewPgUserRepo(pool)
	issuer := users.NewTokenIssuer(*secret, 24*time.Hour)
	tokens := make([]string, 0, *n)

	for i := 0; i < *n; i++ {
		id := fmt.Sprintf("bench-u-%05d", i)
		u, err := repo.GetByID(ctx, id)
		if err != nil {
			u = &users.User{
				ID: id, Phone: fmt.Sprintf("+99877%07d", i), // demo foydalanuvchilar (+99890...) bilan to'qnashmasin Name: "Bench Mijoz", FirstName: "Bench",
				Role: users.RoleCustomer, PhoneVerified: true, CreatedAt: time.Now(),
			}
			if err := repo.Create(ctx, u); err != nil {
				fmt.Fprintln(os.Stderr, "yaratib bo'lmadi:", id, err)
				os.Exit(1)
			}
		}
		// Yetkazish manzili xizmat hududi (Chust) ichida — buyurtma yaratish uchun shart.
		addr := users.AddressDetails{
			Lat: 41.0004 + float64(i%50)*0.0001, Lng: 71.2394 + float64(i%50)*0.0001, Text: "Chust, Bench ko'chasi",
		}
		if err := repo.UpdateAddress(ctx, id, addr); err != nil {
			fmt.Fprintln(os.Stderr, "manzil saqlanmadi:", id, err)
			os.Exit(1)
		}
		tok, err := issuer.Issue(u)
		if err != nil {
			fmt.Fprintln(os.Stderr, "token:", err)
			os.Exit(1)
		}
		tokens = append(tokens, tok)
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	raw, _ := json.Marshal(tokens)
	if err := os.WriteFile(*out, raw, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%d ta mijoz va token tayyor: %s\n", len(tokens), *out)
}
