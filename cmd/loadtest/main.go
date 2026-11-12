// Buyruq loadtest — PRODUCTION'da real yuklama testi uchun AJRATILGAN test
// ma'lumotlarini yaratadi va testdan keyin ularni IZSIZ o'chiradi.
//
// ┌─ NEGA ALOHIDA VOSITA (benchseed EMAS) ─────────────────────────────────┐
// `benchseed` faqat lokal benchmark bazasiga yozadi va JWT sirini buyruq
// qatorida talab qiladi. Bu vosita esa API konteynerining ICHIDA ishlaydi
// (`docker cp` + `docker exec`): baza manzillari va sir konteynerning o'z
// muhitidan olinadi — ular hech qayerga chiqarilmaydi va hech kim ko'rmaydi.
// └────────────────────────────────────────────────────────────────────────┘
//
// ┌─ XAVFSIZLIK QOIDALARI (egasi 2026-11-12) ──────────────────────────────┐
//   - `-confirm-prod` bayrog'isiz HECH NARSA yozilmaydi;
//   - har bir yozuv `loadtest-<run>-` prefiksli ID bilan yaratiladi va
//     MANIFEST faylga yoziladi; o'chirish FAQAT manifestdagi ID'lar bo'yicha;
//   - telefonlar `+99800...` oralig'ida — "00" operator kodi mavjud emas,
//     ya'ni haqiqiy odamning raqami bilan to'qnashmaydi;
//   - test restorani kuryerlarsiz: kuryer havzasi restoran bo'yicha QAT'IY
//     (couriers.restaurant_id), demak hech bir haqiqiy kuryerga taklif bormaydi;
//     xodimi yo'q — hech kimga push/Telegram ketmaydi; to'lov faqat naqd.
// └────────────────────────────────────────────────────────────────────────┘
//
//	loadtest -mode seed    -run r20261112 -n 2000 -confirm-prod  (manifest + tokenlar)
//	loadtest -mode cleanup -run r20261112 -confirm-prod          (manifest bo'yicha o'chirish)
//	loadtest -mode verify  -run r20261112                        (qolgan yozuvlar soni)
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"chustapp/internal/catalog"
	"chustapp/internal/storage"
	"chustapp/internal/users"
)

// Manifest — test yaratgan BARCHA asosiy yozuvlar. O'chirish faqat shu ro'yxatga tayanadi.
type Manifest struct {
	Run          string    `json:"run"`
	CreatedAt    time.Time `json:"created_at"`
	RestaurantID string    `json:"restaurant_id"`
	ProductIDs   []string  `json:"product_ids"`
	UserIDs      []string  `json:"user_ids"`
	// OrderIDs — cleanup boshida to'ldiriladi (buyurtma ID'lari tasodifiy, prefikssiz):
	// chuqur tekshiruv ularni ham barcha jadvallarda qidiradi.
	OrderIDs []string `json:"order_ids,omitempty"`
}

var runRe = regexp.MustCompile(`^[a-z0-9]{3,24}$`)

// manifestFile — joriy run manifesti (main'da o'rnatiladi).
var manifestFile string

func main() {
	mode := flag.String("mode", "", "seed | cleanup | verify")
	run := flag.String("run", "", "test nomi (kichik harf va raqam, 3-24): ID prefiksi loadtest-<run>-")
	n := flag.Int("n", 2000, "test mijozlari soni (seed)")
	dir := flag.String("dir", "/tmp/loadtest", "manifest va tokenlar papkasi")
	confirm := flag.Bool("confirm-prod", false, "production bazasiga yozishni tasdiqlash (seed/cleanup uchun MAJBURIY)")
	flag.Parse()

	if !runRe.MatchString(*run) {
		fail("-run noto'g'ri: faqat kichik harf/raqam, 3-24 belgi")
	}
	if (*mode == "seed" || *mode == "cleanup") && !*confirm {
		fail("-confirm-prod berilmagan — hech narsa yozilmadi")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	pool, err := pgxpool.New(ctx, mustEnv("DATABASE_URL"))
	if err != nil {
		fail("postgres: " + err.Error())
	}
	defer pool.Close()
	mclient, err := mongo.Connect(ctx, options.Client().ApplyURI(mustEnv("MONGODB_URI")))
	if err != nil {
		fail("mongo: " + err.Error())
	}
	defer func() { _ = mclient.Disconnect(context.Background()) }()
	dbName := os.Getenv("MONGODB_DB")
	if dbName == "" {
		dbName = "chustapp"
	}
	cat := storage.NewMongoCatalogRepo(mclient.Database(dbName))
	manifestPath := filepath.Join(*dir, "manifest-"+*run+".json")
	manifestFile = manifestPath

	switch *mode {
	case "seed":
		seed(ctx, pool, cat, *run, *n, *dir, manifestPath)
	case "cleanup":
		m := readManifest(manifestPath, *run)
		cleanup(ctx, pool, cat, m)
		verify(ctx, pool, cat, m)
	case "verify":
		verify(ctx, pool, cat, readManifest(manifestPath, *run))
	default:
		fail("-mode: seed | cleanup | verify")
	}
}

func seed(ctx context.Context, pool *pgxpool.Pool, cat catalog.Repository, run string, n int, dir, manifestPath string) {
	if n <= 0 || n > 20000 {
		fail("-n 1..20000 bo'lishi kerak")
	}
	if _, err := os.Stat(manifestPath); err == nil {
		fail("manifest allaqachon bor (" + manifestPath + ") — avval cleanup qiling yoki boshqa -run bering")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fail(err.Error())
	}
	prefix := "loadtest-" + run + "-"
	m := Manifest{Run: run, CreatedAt: time.Now().UTC(), RestaurantID: prefix + "rest"}
	// Manifest HAR qadamdan oldin yoziladi: yarim yo'lda yiqilsa ham cleanup nimani o'chirishni biladi.
	writeManifest(manifestPath, &m)

	r := &catalog.Restaurant{
		ID: m.RestaurantID, Name: "K6 YUKLAMA TESTI " + strings.ToUpper(run), Address: "Chust (yuklama testi)",
		Lat: 41.0004, Lng: 71.2394, Open: true, Tags: "loadtest",
		Description: "Avtomatik yuklama testi — test tugagach o'chiriladi.",
	}
	if err := cat.SaveRestaurant(ctx, r); err != nil {
		fail("restoran: " + err.Error())
	}
	for i := 0; i < 12; i++ {
		id := fmt.Sprintf("%sp%02d", prefix, i)
		m.ProductIDs = append(m.ProductIDs, id)
		writeManifest(manifestPath, &m)
		p := &catalog.Product{
			ID: id, RestaurantID: r.ID, Name: fmt.Sprintf("Test taom %02d", i),
			Category: []string{"Taomlar", "Ichimliklar", "Salatlar"}[i%3], PriceTiyin: int64(15000+i*2500) * 100,
			Available: true,
		}
		if err := cat.SaveProduct(ctx, p); err != nil {
			fail("taom: " + err.Error())
		}
	}

	repo := storage.NewPgUserRepo(pool)
	issuer := users.NewTokenIssuer(mustEnv("JWT_SECRET"), 3*time.Hour)
	tokens := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("%su%05d", prefix, i)
		m.UserIDs = append(m.UserIDs, id)
		if i%200 == 0 {
			writeManifest(manifestPath, &m)
		}
		u := &users.User{
			ID: id, Phone: fmt.Sprintf("+99800%07d", i), Name: "Yuklama Test", FirstName: "Yuklama",
			Role: users.RoleCustomer, PhoneVerified: true, CreatedAt: time.Now(),
		}
		if err := repo.Create(ctx, u); err != nil {
			writeManifest(manifestPath, &m)
			fail("mijoz " + id + ": " + err.Error())
		}
		addr := users.AddressDetails{Lat: 41.0004 + float64(i%50)*0.0001, Lng: 71.2394 + float64(i%50)*0.0001, Text: "Chust, yuklama testi"}
		if err := repo.UpdateAddress(ctx, id, addr); err != nil {
			fail("manzil " + id + ": " + err.Error())
		}
		tok, err := issuer.Issue(u)
		if err != nil {
			fail("token: " + err.Error())
		}
		tokens = append(tokens, tok)
	}
	writeManifest(manifestPath, &m)
	raw, _ := json.Marshal(tokens)
	if err := os.WriteFile(filepath.Join(dir, "tokens-"+run+".json"), raw, 0o600); err != nil {
		fail(err.Error())
	}
	fmt.Printf("SEED TAYYOR: restoran=%s taom=%d mijoz=%d manifest=%s\n", r.ID, len(m.ProductIDs), len(m.UserIDs), manifestPath)
}

// refColumns — test ID'lari bilan bog'langan bo'lishi mumkin bo'lgan ustunlar.
var refColumns = []string{"order_id", "customer_id", "user_id", "restaurant_id", "placed_by", "entity_id"}

// cleanup — manifestdagi ID'lar va ularning buyurtmalariga tegishli HAMMA qatorlarni o'chiradi.
// Jadvallar information_schema'dan topiladi; FK tartibi noma'lum bo'lgani uchun bir necha
// o'tish qilinadi (bola jadvallar birinchi o'tishda o'chadi, ota jadvallar keyingisida).
func cleanup(ctx context.Context, pool *pgxpool.Pool, cat catalog.Repository, m *Manifest) {
	ids := append([]string{m.RestaurantID}, m.UserIDs...)
	var orderIDs []string
	rows, err := pool.Query(ctx, `SELECT id FROM orders WHERE restaurant_id = $1 OR customer_id = ANY($2)`, m.RestaurantID, m.UserIDs)
	if err != nil {
		fail("buyurtmalar ro'yxati: " + err.Error())
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			orderIDs = append(orderIDs, id)
		}
	}
	rows.Close()
	m.OrderIDs = append(m.OrderIDs, orderIDs...)
	writeManifest(manifestFile, m)
	fmt.Printf("o'chiriladi: buyurtma=%d mijoz=%d restoran=1 taom=%d\n", len(orderIDs), len(m.UserIDs), len(m.ProductIDs))

	targets := tableColumns(ctx, pool)
	for pass := 1; pass <= 6; pass++ {
		total, failed := 0, 0
		for _, tc := range targets {
			vals := ids
			if tc.col == "order_id" {
				vals = orderIDs
			}
			if len(vals) == 0 {
				continue
			}
			tag, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %q.%q WHERE %q::text = ANY($1)`, tc.schema, tc.table, tc.col), vals)
			if err != nil {
				failed++ // FK — keyingi o'tishda
				continue
			}
			total += int(tag.RowsAffected())
		}
		// Buyurtmalar va foydalanuvchilarning o'zlari (ID ustuni bo'yicha).
		if t, err := pool.Exec(ctx, `DELETE FROM orders WHERE id = ANY($1)`, orderIDs); err == nil {
			total += int(t.RowsAffected())
		} else {
			failed++
		}
		if t, err := pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, m.UserIDs); err == nil {
			total += int(t.RowsAffected())
		} else {
			failed++
		}
		fmt.Printf("o'tish %d: %d qator o'chirildi, %d jadval keyingi o'tishga qoldi\n", pass, total, failed)
		if failed == 0 && total == 0 {
			break
		}
	}
	// Chuqur o'chirish: test ID'lari kutilmagan ustun nomi bilan qolgan bo'lsa ham topiladi.
	all := append(append(append([]string{m.RestaurantID}, m.ProductIDs...), m.UserIDs...), m.OrderIDs...)
	deepDelete(ctx, pool, all)
	if err := cat.DeleteRestaurant(ctx, m.RestaurantID); err != nil && !errors.Is(err, catalog.ErrNotFound) {
		fmt.Println("OGOHLANTIRISH: restoran (Mongo) o'chmadi:", err)
	}
	for _, id := range m.ProductIDs {
		_ = cat.DeleteProduct(ctx, id)
	}
}

type tableCol struct{ schema, table, col string }

func tableColumns(ctx context.Context, pool *pgxpool.Pool) []tableCol {
	rows, err := pool.Query(ctx, `
		SELECT c.table_schema, c.table_name, c.column_name
		FROM information_schema.columns c
		JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE t.table_type = 'BASE TABLE' AND c.table_schema NOT IN ('pg_catalog','information_schema')
		  AND c.column_name = ANY($1) AND NOT (c.table_name IN ('orders','users'))
		ORDER BY c.table_name, c.column_name`, refColumns)
	if err != nil {
		fail("jadvallar: " + err.Error())
	}
	defer rows.Close()
	var out []tableCol
	for rows.Next() {
		var tc tableCol
		if err := rows.Scan(&tc.schema, &tc.table, &tc.col); err == nil {
			out = append(out, tc)
		}
	}
	return out
}

// verify — test ID'lari bilan qolgan qatorlarni sanaydi. Hammasi 0 bo'lishi SHART.
func verify(ctx context.Context, pool *pgxpool.Pool, cat catalog.Repository, m *Manifest) {
	ids := append([]string{m.RestaurantID}, m.UserIDs...)
	left := 0
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM orders WHERE restaurant_id = $1 OR customer_id = ANY($2)`, m.RestaurantID, m.UserIDs).Scan(&n)
	left += n
	if n > 0 {
		fmt.Printf("  orders: %d\n", n)
	}
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE id = ANY($1)`, m.UserIDs).Scan(&n)
	left += n
	if n > 0 {
		fmt.Printf("  users: %d\n", n)
	}
	for _, tc := range tableColumns(ctx, pool) {
		if tc.col == "order_id" {
			continue // buyurtmalar o'chgani yuqorida tekshirildi; bola qatorlar FK bilan bog'liq
		}
		if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %q.%q WHERE %q::text = ANY($1)`, tc.schema, tc.table, tc.col), ids).Scan(&n); err == nil && n > 0 {
			fmt.Printf("  %s.%s(%s): %d\n", tc.schema, tc.table, tc.col, n)
			left += n
		}
	}
	// Chuqur tekshiruv: BARCHA jadvallarning BARCHA matn ustunlarida test ID'lari
	// (restoran, taom, mijoz, buyurtma) qidiriladi — kutilmagan ustun nomlari ham qamraladi.
	all := append(append(append([]string{m.RestaurantID}, m.ProductIDs...), m.UserIDs...), m.OrderIDs...)
	left += deepScan(ctx, pool, all)
	if _, err := cat.GetRestaurant(ctx, m.RestaurantID); err == nil {
		fmt.Println("  mongo restaurants: 1")
		left++
	}
	if ps, err := cat.GetProductsByIDs(ctx, m.ProductIDs); err == nil && len(ps) > 0 {
		fmt.Printf("  mongo products: %d\n", len(ps))
		left += len(ps)
	}
	if left == 0 {
		fmt.Println("VERIFY: test ma'lumotlari QOLMADI (0)")
		return
	}
	fmt.Printf("VERIFY: %d ta yozuv QOLDI\n", left)
	os.Exit(3)
}

// deepDelete — barcha matn ustunlarida test ID'lari bor qatorlarni o'chiradi (FK uchun bir necha o'tish).
func deepDelete(ctx context.Context, pool *pgxpool.Pool, ids []string) {
	cols := textColumns(ctx, pool)
	for pass := 1; pass <= 6; pass++ {
		total, failed := 0, 0
		for _, tc := range cols {
			// audit_log — faqat qo'shiladi (trigger DELETE'ni taqiqlaydi, ataylab).
			// Tekshiruvda (deepScan) u baribir sanaladi.
			if tc.table == "audit_log" {
				continue
			}
			tag, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %q.%q WHERE %q::text = ANY($1)`, tc.schema, tc.table, tc.col), ids)
			if err != nil {
				failed++
				if pass == 1 {
					fmt.Printf("  [chuqur xato] %s.%s(%s): %v\n", tc.schema, tc.table, tc.col, err)
				}
				continue
			}
			total += int(tag.RowsAffected())
		}
		fmt.Printf("chuqur o'tish %d: %d qator, %d xato\n", pass, total, failed)
		if total == 0 {
			return
		}
	}
}

func textColumns(ctx context.Context, pool *pgxpool.Pool) []tableCol {
	rows, err := pool.Query(ctx, `
		SELECT c.table_schema, c.table_name, c.column_name
		FROM information_schema.columns c
		JOIN information_schema.tables t ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE t.table_type = 'BASE TABLE' AND c.table_schema NOT IN ('pg_catalog','information_schema')
		  AND c.data_type IN ('text','character varying','uuid')`)
	if err != nil {
		fail("ustunlar: " + err.Error())
	}
	defer rows.Close()
	var cols []tableCol
	for rows.Next() {
		var tc tableCol
		if err := rows.Scan(&tc.schema, &tc.table, &tc.col); err == nil {
			cols = append(cols, tc)
		}
	}
	return cols
}

func deepScan(ctx context.Context, pool *pgxpool.Pool, ids []string) int {
	cols := textColumns(ctx, pool)
	found := 0
	for _, tc := range cols {
		var n int
		err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %q.%q WHERE %q::text = ANY($1)`, tc.schema, tc.table, tc.col), ids).Scan(&n)
		if err != nil {
			// Tekshirib bo'lmagan ustun — "toza" deb hisoblanmaydi, ko'rsatiladi.
			fmt.Printf("  [chuqur TEKSHIRILMADI] %s.%s(%s): %v\n", tc.schema, tc.table, tc.col, err)
			found++
			continue
		}
		if n > 0 {
			fmt.Printf("  [chuqur] %s.%s(%s): %d\n", tc.schema, tc.table, tc.col, n)
			found += n
		}
	}
	fmt.Printf("chuqur tekshiruv: %d ustun ko'rildi\n", len(cols))
	return found
}

func readManifest(path, run string) *Manifest {
	raw, err := os.ReadFile(filepath.Clean(path)) // yo'l operator bergan -dir ostida
	if err != nil {
		fail("manifest o'qilmadi: " + err.Error())
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil || m.Run != run {
		fail("manifest buzilgan yoki boshqa run")
	}
	// Ikkinchi qulf: manifestdagi HAR bir ID prefiksga mos bo'lishi shart.
	prefix := "loadtest-" + run + "-"
	for _, id := range append(append([]string{m.RestaurantID}, m.ProductIDs...), m.UserIDs...) {
		if !strings.HasPrefix(id, prefix) {
			fail("manifestda prefikssiz ID bor: " + id + " — o'chirish rad etildi")
		}
	}
	return &m
}

func writeManifest(path string, m *Manifest) {
	raw, _ := json.MarshalIndent(m, "", " ")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		fail("manifest yozilmadi: " + err.Error())
	}
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		fail(k + " muhitda yo'q")
	}
	return v
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "loadtest:", msg)
	os.Exit(2)
}
