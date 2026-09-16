package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"chustapp/internal/images"
)

// ┌─ ILOVANI SAYTDAN YUKLAB OLISH (2026-09-16) ─────────────────────────┐
// ondex.uz dagi "Ilovani yuklab olish" tugmasi `/download/android` ga olib
// boradi (`apps/web/app/download/android/route.ts`). U R2 dagi
// `releases/android/latest.json` manifestini o'qib, eng so'nggi APK ga
// yo'naltiradi.
//
// APK saytning o'zida TURMAYDI: ~100 MB fayl git'ga ham (`*.apk`
// gitignore'da), Docker image'ga ham kirmaydi. R2 da tursa yangi reliz
// saytni qayta deploy qilmasdan chiqadi.
//
//	go run ./cmd/r2upload -android-release F:\OndexProd\apk\customer_app-0.2.3_15.apk
//
// Versiya `apps/customer_app/pubspec.yaml` dan olinadi (`-release-version`
// bilan aniq berish mumkin). Har reliz O'ZGARMAS kalit bilan joylanadi —
// avval tarqatilgan havolalar buzilmaydi; manifest esa oxirgisini ko'rsatadi.
// └────────────────────────────────────────────────────────────────────┘

const (
	androidReleasePrefix = "releases/android/"
	androidManifestKey   = androidReleasePrefix + "latest.json"
	// maxAPKBytes — tasodifan noto'g'ri (masalan butun papka arxivi)
	// fayl yuklanmasin.
	maxAPKBytes = 300 << 20
)

var releaseVersionRe = regexp.MustCompile(`^(\d+\.\d+\.\d+)\+(\d+)$`)

// androidManifest — `releases/android/latest.json`.
type androidManifest struct {
	Version     string `json:"version"`
	Build       int    `json:"build"`
	URL         string `json:"url"`
	SizeBytes   int64  `json:"size_bytes"`
	SHA256      string `json:"sha256"`
	PublishedAt string `json:"published_at"`
}

// parseReleaseVersion — "0.2.3+15" → ("0.2.3", 15).
func parseReleaseVersion(v string) (string, int, error) {
	m := releaseVersionRe.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return "", 0, fmt.Errorf("versiya %q — X.Y.Z+N shaklida bo'lishi kerak", v)
	}
	build, err := strconv.Atoi(m[2])
	if err != nil || build <= 0 {
		return "", 0, fmt.Errorf("build raqami noto'g'ri: %q", m[2])
	}
	return m[1], build, nil
}

// pubspecVersion — `pubspec.yaml` dagi `version:` qiymati.
func pubspecVersion(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // loyiha ichidagi ma'lum fayl
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile(`(?m)^version:\s*(\S+)\s*$`).FindSubmatch(data)
	if m == nil {
		return "", fmt.Errorf("%s: `version:` topilmadi", path)
	}
	return string(m[1]), nil
}

// checkAPK — fayl haqiqatan Android paketi: ZIP va ichida AndroidManifest.xml
// hamda classes.dex bor. Imzo va prod manzili `build_mobile.ps1` da
// tekshiriladi.
func checkAPK(path string) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("APK emas (ZIP ochilmadi): %w", err)
	}
	defer zr.Close()
	var manifest, dex bool
	for _, f := range zr.File {
		switch f.Name {
		case "AndroidManifest.xml":
			manifest = true
		case "classes.dex":
			dex = true
		}
	}
	if !manifest || !dex {
		return errors.New("APK emas: AndroidManifest.xml yoki classes.dex yo'q")
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // ishlab chiquvchi bergan fayl
	if err != nil {
		return "", err
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// publishAndroidRelease — APK va manifestni R2 ga joylaydi.
func publishAndroidRelease(apkPath, version string) error {
	if !strings.EqualFold(filepath.Ext(apkPath), ".apk") {
		return errors.New("fayl .apk bo'lishi kerak")
	}
	if strings.TrimSpace(version) == "" {
		v, err := pubspecVersion(filepath.Join("apps", "customer_app", "pubspec.yaml"))
		if err != nil {
			return fmt.Errorf("versiya aniqlanmadi (-release-version bering): %w", err)
		}
		version = v
	}
	semver, build, err := parseReleaseVersion(version)
	if err != nil {
		return err
	}

	info, err := os.Stat(apkPath)
	if err != nil {
		return err
	}
	if info.Size() <= 0 || info.Size() > maxAPKBytes {
		return fmt.Errorf("APK hajmi g'alati: %d bayt", info.Size())
	}
	if err := checkAPK(apkPath); err != nil {
		return err
	}
	digest, err := fileSHA256(apkPath)
	if err != nil {
		return err
	}

	store, err := newStore()
	if err != nil {
		return fmt.Errorf("R2 sozlanmadi: %w", err)
	}

	f, err := os.Open(apkPath) //nolint:gosec // yuqorida tekshirilgan fayl
	if err != nil {
		return err
	}
	defer f.Close()

	key := fmt.Sprintf("%sondex-%s-%d.apk", androidReleasePrefix, semver, build)
	fmt.Printf("Yuklanmoqda: %s → %s (%.1f MB)\n", apkPath, key, float64(info.Size())/(1<<20))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	url, err := store.UploadWithOptions(ctx, key, f, info.Size(), images.UploadOptions{
		ContentType:        "application/vnd.android.package-archive",
		ContentDisposition: fmt.Sprintf(`attachment; filename="OnDex-%s.apk"`, semver),
		CacheControl:       "public, max-age=31536000, immutable",
	})
	if err != nil {
		return fmt.Errorf("APK yuklanmadi: %w", err)
	}

	body, err := json.MarshalIndent(androidManifest{
		Version: semver, Build: build, URL: url, SizeBytes: info.Size(), SHA256: digest,
		PublishedAt: time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	// Manifest APK'dan KEYIN yoziladi: yarim yo'lda uzilsa sayt eski,
	// ishlaydigan relizni ko'rsatishda davom etadi.
	if _, err := store.UploadWithOptions(ctx, androidManifestKey, bytes.NewReader(body), int64(len(body)), images.UploadOptions{
		ContentType:  "application/json",
		CacheControl: "public, max-age=60",
	}); err != nil {
		return fmt.Errorf("manifest yozilmadi (APK joylandi, lekin sayt uni ko'rmaydi): %w", err)
	}

	fmt.Println("\nReliz joylandi.")
	fmt.Println("Versiya :", semver, "build", build)
	fmt.Println("URL     :", url)
	fmt.Println("SHA-256 :", digest)
	fmt.Println("Sayt 1 daqiqa ichida yangi APK ni bera boshlaydi (/download/android).")
	return nil
}
