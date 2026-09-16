// r2upload — bitta faylni Cloudflare R2 ga yuklaydigan yordamchi.
//
// ┌─ NEGA ALOHIDA VOSITA ──────────────────────────────────────────────┐
// Ba'zi fayllar API orqali kelmaydi: Book Cafe 3D sahnasi (~104 MB)
// foydalanuvchi yuklamaydi, uni ishlab chiquvchi bir marta qo'yadi.
//
// Buning uchun `aws` yoki `rclone` o'rnatish shart emas — yuklash
// mantig'i loyihada allaqachon bor (`internal/images.R2Store`) va
// shu yerda AYNAN o'sha ishlatiladi. Alohida S3 klienti yozilsa,
// imzolash va sozlash qoidalari ikki nusxada bo'lardi.
// └────────────────────────────────────────────────────────────────────┘
//
// Ishlatish:
//
//	go run ./cmd/r2upload -file <yo'l> -key <bucket ichidagi nom>
//
// Sozlamalar `.env` dagi bilan bir xil o'zgaruvchilardan olinadi:
// R2_ACCOUNT_ID, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, R2_BUCKET,
// R2_PUBLIC_URL.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"chustapp/internal/images"
	"chustapp/internal/storage"
)

func main() {
	var (
		filePath = flag.String("file", "", "yuklanadigan fayl")
		key      = flag.String("key", "", "bucket ichidagi nom (masalan bookcafe/scene.pck)")
		envFile  = flag.String("env", ".env", "sozlamalar fayli")
		// ┌─ MAKETNI RESTORANGA BOG'LASH ──────────────────────────────┐
		// Yuklangan fayl o'z-o'zidan hech qayerda ko'rinmaydi: mijoz
		// ilovasi maketni RESTORAN yozuvidan topadi.
		//
		// Ikki qadamni (yuklash va bog'lash) alohida qilish oson
		// xatoga olib keladi: fayl yangilanadi-yu, xesh eskisicha
		// qoladi va ilova "fayl buzilgan" deb rad etadi. Shu sababli
		// ikkalasi BITTA buyruqda.
		// └────────────────────────────────────────────────────────────┘
		restaurantID = flag.String("restaurant", "", "maket bog'lanadigan restoran ID")
		newName      = flag.String("name", "", "restoran nomini o'zgartirish (ixtiyoriy)")
		// Tekshirish rejimi: hech narsa yuklanmaydi va yozilmaydi,
		// faqat restoranning maket maydonlari bosib chiqariladi.
		// "API maydonni qaytarmayapti" holatida sababni ajratish
		// uchun: ma'lumot bazadami yoki yo'qmi.
		check = flag.String("check", "", "restoran maket maydonlarini ko'rish")

		// ┌─ NEGA "YUKLAMASDAN BOG'LASH" KERAK ────────────────────────────┐
		// 2026-08-27 da Docker ma'lumot diski yo'qolganda restoran
		// yozuvidagi maket maydonlari o'chdi, LEKIN R2 dagi paket
		// (221 MB) joyida qoldi. Yagona tiklash yo'li paketni qaytadan
		// yuklash edi — 15 daqiqa va butunlay keraksiz trafik.
		//
		// `-link` shu holat uchun: fayl LOKAL o'qiladi (xesh aynan o'sha
		// baytlardan hisoblanadi), yuklash esa o'tkazib yuboriladi va
		// mavjud kalit restoranga bog'lanadi.
		// └────────────────────────────────────────────────────────────────┘
		link = flag.Bool("link", false,
			"yuklamasdan bog'lash: `-key` R2 da allaqachon bor deb hisoblanadi")

		// Bucket ommaviy — xato yuklangan faylni olib tashlash yo'li
		// bo'lishi shart.
		del = flag.String("delete", "", "bucket'dan kalitni o'chirish")

		// Mijoz ilovasi relizi — saytdagi "Ilovani yuklab olish"
		// (`android_release.go`).
		androidRelease = flag.String("android-release", "", "mijoz ilovasi APK relizini joylash")
		releaseVersion = flag.String("release-version", "", "reliz versiyasi X.Y.Z+N (bo'sh — pubspec.yaml dan)")
	)
	flag.Parse()

	if *androidRelease != "" {
		loadDotEnv(*envFile)
		if err := publishAndroidRelease(*androidRelease, *releaseVersion); err != nil {
			log.Fatalf("reliz joylanmadi: %v", err)
		}
		return
	}

	if *del != "" {
		loadDotEnv(*envFile)
		store, err := newStore()
		if err != nil {
			log.Fatalf("R2 sozlanmadi: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := store.Delete(ctx, *del); err != nil {
			log.Fatalf("o'chirilmadi: %v", err)
		}
		fmt.Printf("O'chirildi: %s\n", *del)
		return
	}

	if *check != "" {
		loadDotEnv(*envFile)
		if err := showScene(*check); err != nil {
			log.Fatalf("o'qilmadi: %v", err)
		}
		return
	}

	if *filePath == "" || *key == "" {
		log.Fatal("-file va -key majburiy")
	}

	// .env topilmasa ham davom etamiz: o'zgaruvchilar muhitda
	// berilgan bo'lishi mumkin (CI shunday ishlaydi).
	loadDotEnv(*envFile)

	store, err := newStore()
	if err != nil {
		log.Fatalf("R2 sozlanmadi: %v", err)
	}

	f, err := os.Open(*filePath)
	if err != nil {
		log.Fatalf("fayl ochilmadi: %v", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		log.Fatalf("fayl o'qilmadi: %v", err)
	}

	// ┌─ XESH YUKLASHDAN OLDIN HISOBLANADI ────────────────────────────┐
	// Mijoz ilovasi faylni aynan shu qiymat bilan tekshiradi. Uni
	// yuklashdan KEYIN qayta hisoblash mumkin edi, lekin shunda
	// "qaysi nusxaning xeshi" degan savol tug'ilardi. Bu yerda esa
	// yuklanayotgan aynan shu baytlarning xeshi chiqadi.
	// └────────────────────────────────────────────────────────────────┘
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		log.Fatalf("xesh hisoblanmadi: %v", err)
	}
	digest := hex.EncodeToString(sum.Sum(nil))
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		log.Fatalf("fayl boshiga qaytilmadi: %v", err)
	}

	ctype := mime.TypeByExtension(filepath.Ext(*filePath))
	if ctype == "" {
		ctype = "application/octet-stream"
	}

	var url string
	if *link {
		url = store.PublicURL(*key)
		fmt.Printf("Bog'lanmoqda (yuklanmaydi): %s (%.1f MB)\n",
			*key, float64(info.Size())/(1024*1024))
		fmt.Println("URL     :", url)
		fmt.Println("SHA-256 :", digest)
	} else {
		fmt.Printf("Yuklanmoqda: %s → %s (%.1f MB)\n",
			*filePath, *key, float64(info.Size())/(1024*1024))

		// Katta fayl uchun vaqt kengroq: 104 MB sekin ulanishda
		// bir necha daqiqa ketishi mumkin.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		url, err = store.Upload(ctx, *key, f, info.Size(), ctype)
		if err != nil {
			log.Fatalf("yuklanmadi: %v", err)
		}

		fmt.Println("\nYuklandi.")
		fmt.Println("URL     :", url)
		fmt.Println("SHA-256 :", digest)
	}

	if *restaurantID == "" {
		fmt.Println("\nEslatma: `-restaurant <id>` berilmadi, maket hech qaysi",
			"restoranga bog'lanmadi.")
		return
	}
	if err := bindToRestaurant(*restaurantID, *newName, url, digest, info.Size()); err != nil {
		log.Fatalf("bog'lanmadi: %v", err)
	}
	fmt.Printf("\nRestoranga bog'landi: %s\n", *restaurantID)
	fmt.Println("API keshini yangilash uchun serverni qayta ishga tushiring",
		"(yoki 30 soniya kuting).")
}

// openCatalog — Mongo katalog omboriga ulanadi.
//
// Ulanish ikki joyda kerak (ko'rish va yozish), shuning uchun u bir
// marta yozilgan: takrorlansa sozlama qoidalari ikki nusxada bo'lardi.
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

// showScene — restoranning maket maydonlarini bosib chiqaradi.
func showScene(id string) error {
	repo, disconnect, err := openCatalog()
	if err != nil {
		return err
	}
	defer disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rest, err := repo.GetRestaurant(ctx, id)
	if err != nil {
		return err
	}

	fmt.Printf("ID      : %s\n", rest.ID)
	fmt.Printf("Nomi    : %s\n", rest.Name)
	fmt.Printf("URL     : %q\n", rest.Scene3DURL)
	fmt.Printf("SHA-256 : %q\n", rest.Scene3DSHA256)
	fmt.Printf("Hajm    : %d\n", rest.Scene3DBytes)
	return nil
}

// bindToRestaurant — maketni restoran yozuviga yozadi.
//
// To'g'ridan-to'g'ri bazaga yoziladi, admin API orqali emas: bu bir
// martalik ma'muriy amal va u uchun Telegram OTP bilan token olish
// ortiqcha bo'lardi. Tekshiruvlar (manzil R2 da, xesh to'g'ri) API
// tomonda ham bor va u yerda qoladi.
func bindToRestaurant(id, name, url, digest string, size int64) error {
	repo, disconnect, err := openCatalog()
	if err != nil {
		return err
	}
	defer disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	rest, err := repo.GetRestaurant(ctx, id)
	if err != nil {
		return fmt.Errorf("restoran topilmadi: %w", err)
	}

	if name != "" {
		fmt.Printf("Nomi: %q → %q\n", rest.Name, name)
		rest.Name = name
	}
	rest.Scene3DURL = url
	rest.Scene3DSHA256 = digest
	rest.Scene3DBytes = size

	return repo.SaveRestaurant(ctx, rest)
}

// loadDotEnv — `.env` faylni o'qiydi (bor bo'lsa).
//
// Parser `cmd/api/main.go` dagi bilan bir xil naqshda: tizim
// muhitida allaqachon bor qiymat USTUN turadi, ya'ni CI o'z
// sozlamasini bermoqchi bo'lsa fayl unga xalaqit bermaydi.
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

func newStore() (*images.R2Store, error) {
	bucket := os.Getenv("R2_BUCKET")
	account := os.Getenv("R2_ACCOUNT_ID")
	access := os.Getenv("R2_ACCESS_KEY_ID")
	secret := os.Getenv("R2_SECRET_ACCESS_KEY")
	public := os.Getenv("R2_PUBLIC_URL")

	var missing []string
	for name, v := range map[string]string{
		"R2_BUCKET":            bucket,
		"R2_ACCOUNT_ID":        account,
		"R2_ACCESS_KEY_ID":     access,
		"R2_SECRET_ACCESS_KEY": secret,
		"R2_PUBLIC_URL":        public,
	} {
		if v == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("to'ldirilmagan: %s", strings.Join(missing, ", "))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return images.NewR2Store(ctx, account, access, secret, bucket, public)
}
