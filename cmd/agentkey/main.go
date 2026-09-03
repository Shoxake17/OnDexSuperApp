// agentkey — integratsiya sheriklari uchun API kalitlarini boshqarish.
//
// ┌─ NEGA CLI, ADMIN PANELDAGI TUGMA EMAS ─────────────────────────────┐
// Sherik kaliti — butun integratsiyaning ildiz siri. Uni HTTP orqali
// yaratish yo'li ochilsa, o'sha endpoint darhol eng qimmatli nishonga
// aylanardi: admin sessiyasi o'g'irlansa, hujumchi o'ziga yangi
// "sherik" yasab, keyin foydalanuvchilardan rozilik so'rab yurardi.
//
// Bu yerda esa kalit yaratish uchun SERVERGA kirish kerak — ya'ni
// huquq allaqachon boshqa (ancha qattiq) himoya bilan cheklangan.
// Aynan shu sabab `BOOTSTRAP_ADMIN_PHONE` ham `.env` orqali beriladi.
// └────────────────────────────────────────────────────────────────────┘
//
// Ishlatish:
//
//	go run ./cmd/agentkey list
//	go run ./cmd/agentkey create -name "Shaddiy AI" -env live \
//	    -scopes catalog:read,profile:read,orders:read,orders:create,orders:cancel
//	go run ./cmd/agentkey disable -id <partner_id>
//	go run ./cmd/agentkey enable  -id <partner_id>
//
// `DATABASE_URL` `.env` dan yoki muhitdan olinadi.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/agentapi"
	"chustapp/internal/storage"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	loadDotEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		fail("DATABASE_URL yo'q — .env faylni tekshiring")
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fail("PostgreSQL konfiguratsiya xatosi: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fail("PostgreSQL'ga ulanib bo'lmadi: %v", err)
	}
	// Migratsiya shu yerda ham qo'llanadi: kalitni serverdan OLDIN
	// yaratmoqchi bo'lgan holat normal (integratsiyani oldindan
	// tayyorlash).
	if err := storage.Migrate(ctx, pool); err != nil {
		fail("migratsiya xatosi: %v", err)
	}

	// Pricer/Quoter kerak emas — bu vosita faqat sheriklar bilan
	// ishlaydi. `nil` berilishi xavfsiz: ular faqat qoralama
	// yo'lida chaqiriladi.
	svc := agentapi.NewService(storage.NewPgAgentRepo(pool), nil, nil)

	switch os.Args[1] {
	case "create":
		cmdCreate(ctx, svc, os.Args[2:])
	case "list":
		cmdList(ctx, svc)
	case "disable":
		cmdSetActive(ctx, svc, os.Args[2:], false)
	case "enable":
		cmdSetActive(ctx, svc, os.Args[2:], true)
	default:
		usage()
		os.Exit(2)
	}
}

func cmdCreate(ctx context.Context, svc *agentapi.Service, args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("name", "", "sherik nomi (foydalanuvchi rozilik ekranida KO'RADI)")
	contact := fs.String("contact", "", "aloqa (email/telegram) — ixtiyoriy")
	env := fs.String("env", "test", "live yoki test")
	scopes := fs.String("scopes", strings.Join(agentapi.AllScopes, ","),
		"vergul bilan: "+strings.Join(agentapi.AllScopes, ","))
	fs.Parse(args)

	if strings.TrimSpace(*name) == "" {
		fail("-name majburiy")
	}
	p, full, err := svc.RegisterPartner(ctx, *name, *contact,
		agentapi.Environment(*env), agentapi.ParseScopes(*scopes))
	if err != nil {
		fail("%v", err)
	}

	fmt.Println()
	fmt.Println("  Sherik yaratildi")
	fmt.Println("  ────────────────────────────────────────────────")
	fmt.Printf("  ID:      %s\n", p.ID)
	fmt.Printf("  Nom:     %s\n", p.Name)
	fmt.Printf("  Muhit:   %s\n", p.Environment)
	fmt.Printf("  Ruxsat:  %s\n", strings.Join(p.Scopes, ", "))
	fmt.Println()
	fmt.Println("  API KALIT (BIR MARTA ko'rsatiladi, saqlab qo'ying):")
	fmt.Println()
	fmt.Printf("      %s\n", full)
	fmt.Println()
	// Bu ogohlantirish ATAYLAB bu yerda: kalit qayerda turishi
	// kerakligini aytmasak, u eng oson joyga — mijoz kodiga yoki
	// git'ga tushadi.
	fmt.Println("  ⚠  Kalit FAQAT sherikning SERVERIDA (.env) turadi.")
	fmt.Println("     Brauzer/mobil kodga yozilmaydi — API brauzerdan")
	fmt.Println("     kelgan so'rovni umuman rad etadi.")
	fmt.Println("  ⚠  Bu kalit o'zi hech kimning akkauntiga kira olmaydi:")
	fmt.Println("     har bir amal uchun foydalanuvchi granti ham kerak.")
	fmt.Println()
}

func cmdList(ctx context.Context, svc *agentapi.Service) {
	list, err := svc.ListPartners(ctx)
	if err != nil {
		fail("%v", err)
	}
	if len(list) == 0 {
		fmt.Println("Sheriklar yo'q.")
		return
	}
	fmt.Printf("%-34s %-20s %-6s %-8s %s\n", "ID", "NOM", "MUHIT", "HOLAT", "KALIT PREFIKSI")
	for _, p := range list {
		state := "faol"
		if !p.Active {
			state = "o'chiq"
		}
		fmt.Printf("%-34s %-20s %-6s %-8s %s\n",
			p.ID, trunc(p.Name, 20), p.Environment, state, p.KeyPrefix)
	}
}

func cmdSetActive(ctx context.Context, svc *agentapi.Service, args []string, active bool) {
	fs := flag.NewFlagSet("set", flag.ExitOnError)
	id := fs.String("id", "", "sherik ID'si (`list` dan)")
	fs.Parse(args)
	if strings.TrimSpace(*id) == "" {
		fail("-id majburiy")
	}
	if err := svc.SetPartnerActive(ctx, *id, active); err != nil {
		fail("%v", err)
	}
	if active {
		fmt.Println("Sherik yoqildi.")
		return
	}
	// O'chirish DARHOL ta'sir qiladi: sherik kaliti bilan keladigan
	// har qanday so'rov shu zahoti rad etiladi. Grantlar esa
	// tegilmaydi — sherik qayta yoqilsa, foydalanuvchilar qaytadan
	// rozilik berishi shart bo'lmaydi.
	fmt.Println("Sherik o'chirildi — barcha so'rovlari darhol rad etiladi.")
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func usage() {
	fmt.Println(`agentkey — AI agent integratsiyasi kalitlari

  agentkey create -name "Shaddiy AI" -env live [-scopes ...] [-contact ...]
  agentkey list
  agentkey disable -id <partner_id>
  agentkey enable  -id <partner_id>`)
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "xato: "+format+"\n", a...)
	os.Exit(1)
}

// loadDotEnv — `cmd/api` dagi bilan bir xil, sodda o'qigich.
// Nusxa ataylab: bu vosita `main` paketiga bog'lanmaydi va yagona
// kerak bo'ladigan qiymat `DATABASE_URL`.
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}
