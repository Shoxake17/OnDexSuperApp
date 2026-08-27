// Command bookpages — skanerlangan kitob PDF ini sahifa rasmlariga
// aylantirib R2 ga yuklaydi va kitob yozuviga bog'laydi.
//
// ┌─ NEGA API ICHIDA EMAS, ALOHIDA VOSITA ─────────────────────────────┐
// PDF ni rasmga chizish uchun render dvigateli (poppler) kerak. Uni API
// serveriga qo'shish production image'ini `distroless` dan to'liq
// Debian'ga o'tkazishni talab qilardi — ya'ni hujum yuzasini kengaytirib,
// faqat kitob o'qish uchun.
//
// Aylantirish har kitob uchun BIR MARTA bajariladi va uni admin qiladi.
// Shuning uchun u `cmd/r2upload` bilan bir xil yo'ldan boradi: alohida
// vosita, serverdan tashqarida ishlaydi, natijani bazaga yozadi.
// Server esa faqat tayyor manzillarni tarqatadi.
// └────────────────────────────────────────────────────────────────────┘
//
// Ishlatish:
//
//	go run ./cmd/bookpages -book <kitob_id>
//	go run ./cmd/bookpages -book <kitob_id> -file kitob.pdf -dpi 150
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"chustapp/internal/images"
	"chustapp/internal/storage"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// popplerImage — pdftoppm shu konteynerdan ishlatiladi.
//
// Windows'ga poppler o'rnatish shart emas: Docker allaqachon loyihaning
// talabi (Postgres/Mongo/Redis shu yerda ishlaydi).
const popplerImage = "minidocks/poppler"

// maxPages — bitta kitobdan nechta sahifa olinadi.
const maxPages = 500

// uploadWorkers — R2 ga bir vaqtda nechta yuklash.
//
// Ketma-ket yuklashda 142 sahifa bir necha daqiqa olardi. Cheksiz
// parallellik esa ulanishlarni tugatadi va R2 tezlik chegarasiga uradi.
const uploadWorkers = 6

// uploadAttempts — bitta sahifa uchun nechta urinish.
const uploadAttempts = 4

func main() {
	var (
		bookID  = flag.String("book", "", "kitob ID (majburiy)")
		pdfPath = flag.String("file", "", "lokal PDF; berilmasa kitobning pdf_url idan yuklanadi")
		dpi     = flag.Int("dpi", 150, "render zichligi")
		envFile = flag.String("env", ".env", "sozlamalar fayli")
		keep    = flag.Bool("keep", false, "vaqtinchalik rasmlarni o'chirmaslik")
	)
	flag.Parse()

	if *bookID == "" {
		log.Fatal("-book majburiy")
	}
	loadDotEnv(*envFile)

	repo, closeRepo, err := openCatalog()
	if err != nil {
		log.Fatalf("bazaga ulanmadi: %v", err)
	}
	defer closeRepo()

	ctx := context.Background()
	book, err := repo.GetBook(ctx, *bookID)
	if err != nil {
		log.Fatalf("kitob topilmadi: %v", err)
	}
	fmt.Printf("Kitob: %s (%s)\n", book.Title, book.ID)

	work, err := os.MkdirTemp("", "bookpages-")
	if err != nil {
		log.Fatalf("vaqtinchalik papka: %v", err)
	}
	if !*keep {
		defer os.RemoveAll(work)
	} else {
		fmt.Println("Vaqtinchalik papka:", work)
	}

	src := filepath.Join(work, "in.pdf")
	if *pdfPath != "" {
		if err := copyFile(*pdfPath, src); err != nil {
			log.Fatalf("PDF nusxalanmadi: %v", err)
		}
	} else {
		if book.PDFURL == "" {
			log.Fatal("kitobda pdf_url yo'q va -file ham berilmadi")
		}
		fmt.Println("PDF yuklanmoqda:", book.PDFURL)
		if err := download(book.PDFURL, src); err != nil {
			log.Fatalf("PDF yuklanmadi: %v", err)
		}
	}

	fmt.Printf("Sahifalar chizilmoqda (%d dpi)...\n", *dpi)
	pngs, err := renderPages(work, *dpi)
	if err != nil {
		log.Fatalf("render xatosi: %v", err)
	}
	fmt.Printf("  %d sahifa chizildi\n", len(pngs))
	if len(pngs) == 0 {
		log.Fatal("sahifa chiqmadi")
	}

	store, err := newStore()
	if err != nil {
		log.Fatalf("R2 sozlanmadi: %v", err)
	}

	fmt.Println("R2 ga yuklanmoqda...")
	urls, err := convertAndUpload(ctx, store, book.ID, pngs)
	if err != nil {
		log.Fatalf("yuklanmadi: %v", err)
	}

	book.Pages = urls
	// ┌─ MATN TOZALANADI ──────────────────────────────────────────────┐
	// Skanerlangan kitobda matn qatlami faqat sahifa raqamlaridan
	// iborat ("1 2 3 4 5 ..."). U qolsa maketdagi o'quvchi qaysi
	// manbani ko'rsatishini o'zi hal qila olmaydi va aynan shu axlat
	// ekranga chiqardi.
	// └────────────────────────────────────────────────────────────────┘
	book.Text = ""
	if err := repo.SaveBook(ctx, book); err != nil {
		log.Fatalf("saqlanmadi: %v", err)
	}

	fmt.Printf("\nTayyor: %d sahifa kitobga bog'landi.\n", len(urls))
	fmt.Println("Birinchi sahifa:", urls[0])
}

// renderPages — pdftoppm ni Docker orqali chaqiradi.
func renderPages(work string, dpi int) ([]string, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, errors.New("docker topilmadi — render shu orqali bajariladi")
	}
	// Docker Windows yo'lini `/` bilan kutadi.
	mount := strings.ReplaceAll(work, `\`, "/") + ":/data"

	cmd := exec.Command("docker", "run", "--rm",
		"-v", mount, popplerImage,
		"pdftoppm", "-png", "-r", fmt.Sprint(dpi),
		"-l", fmt.Sprint(maxPages),
		"/data/in.pdf", "/data/page")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}

	entries, err := filepath.Glob(filepath.Join(work, "page-*.png"))
	if err != nil {
		return nil, err
	}
	// `pdftoppm` nomlarni raqam bilan to'ldiradi (page-001.png), ya'ni
	// alifbo tartibi = sahifa tartibi. Baribir aniq tartiblaymiz:
	// 1000 dan oshgan kitobda to'ldirish kengayadi.
	sort.Strings(entries)
	return entries, nil
}

// convertAndUpload — PNG larni WebP ga o'girib R2 ga yuklaydi.
func convertAndUpload(ctx context.Context, store *images.R2Store,
	bookID string, pngs []string) ([]string, error) {

	urls := make([]string, len(pngs))
	errs := make([]error, len(pngs))

	var wg sync.WaitGroup
	sem := make(chan struct{}, uploadWorkers)
	var done int
	var mu sync.Mutex

	for i, path := range pngs {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			f, err := os.Open(path)
			if err != nil {
				errs[i] = err
				return
			}
			data, err := images.ProcessPageImage(f)
			f.Close()
			if err != nil {
				errs[i] = fmt.Errorf("%s: %w", filepath.Base(path), err)
				return
			}
			// Kalitda kitob ID si bor: sahifalar aralashmaydi va kitob
			// o'chirilganda ularni topish oson.
			key := fmt.Sprintf("book-pages/%s/%04d.webp", bookID, i+1)
			url, err := uploadWithRetry(ctx, store, key, data)
			if err != nil {
				errs[i] = fmt.Errorf("%d-sahifa: %w", i+1, err)
				return
			}
			urls[i] = url

			mu.Lock()
			done++
			if done%20 == 0 || done == len(pngs) {
				fmt.Printf("  %d / %d\n", done, len(pngs))
			}
			mu.Unlock()
		}(i, path)
	}
	wg.Wait()

	for _, e := range errs {
		if e != nil {
			return nil, e
		}
	}
	return urls, nil
}

// uploadWithRetry — bitta sahifani qayta urinishlar bilan yuklaydi.
//
// ┌─ NEGA KERAK ───────────────────────────────────────────────────────┐
// 142 sahifa = 142 ta tarmoq amali. Beqaror Wi-Fi'da ulardan bittasi
// uzilishi deyarli muqarrar: birinchi urinishda 140-sahifada
// "An existing connection was forcibly closed" chiqdi va butun ish
// bekor bo'ldi — 140 ta muvaffaqiyatli yuklash ham behuda ketdi.
//
// AWS SDK ning o'z qayta urinishi (3 marta) yetmadi, chunki u faqat
// bitta so'rov ichida ishlaydi va uzilish oynasi undan uzunroq edi.
// └────────────────────────────────────────────────────────────────────┘
func uploadWithRetry(ctx context.Context, store *images.R2Store,
	key string, data []byte) (string, error) {

	var lastErr error
	for attempt := 1; attempt <= uploadAttempts; attempt++ {
		url, err := store.Upload(ctx, key, bytes.NewReader(data),
			int64(len(data)), "image/webp")
		if err == nil {
			return url, nil
		}
		lastErr = err
		if attempt < uploadAttempts {
			// Ortib boruvchi kutish: tarmoq tiklanishiga vaqt beradi
			// va uzilish paytida serverni yana urib turmaydi.
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}
	return "", lastErr
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func download(url, dst string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// openCatalog — `cmd/r2upload` dagi bilan bir xil.
func openCatalog() (*storage.MongoCatalogRepo, func(), error) {
	uri := os.Getenv("MONGODB_URI")
	dbName := os.Getenv("MONGODB_DB")
	if uri == "" || dbName == "" {
		return nil, nil, fmt.Errorf("MONGODB_URI/MONGODB_DB to'ldirilmagan")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, err
	}
	stop := func() { _ = client.Disconnect(context.Background()) }
	return storage.NewMongoCatalogRepo(client.Database(dbName)), stop, nil
}

func newStore() (*images.R2Store, error) {
	bucket := os.Getenv("R2_BUCKET")
	if bucket == "" {
		return nil, fmt.Errorf("R2_BUCKET berilmagan")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return images.NewR2Store(ctx,
		os.Getenv("R2_ACCOUNT_ID"),
		os.Getenv("R2_ACCESS_KEY_ID"),
		os.Getenv("R2_SECRET_ACCESS_KEY"),
		bucket,
		os.Getenv("R2_PUBLIC_URL"))
}

// loadDotEnv — `.env` ni o'qiydi (qatorma-qator, `cmd/r2upload` bilan bir xil).
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
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
			_ = os.Setenv(k, v)
		}
	}
}
