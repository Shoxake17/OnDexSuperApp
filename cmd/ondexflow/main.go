// Command ondexflow — OnDexFlow nashr vositasi (Plane ONDEXECOSY-472).
//
//	go run ./cmd/ondexflow keygen  -name customer_app
//	go run ./cmd/ondexflow pubkey  -key <pem>
//	go run ./cmd/ondexflow inspect -apk <apk>
//	go run ./cmd/ondexflow release -apk <apk> -version 0.2.16+28 -key <pem>
//	go run ./cmd/ondexflow patch   -apk <apk> -version 0.2.16+28 -number 1 -note "..." -key <pem>
//
// ┌─ XAVFSIZLIK ─────────────────────────────────────────────────────────┐
// Shaxsiy kalit FAQAT shu kompyuterda (%USERPROFILE%\.ondexflow\keys) —
// u repoga, serverga yoki CI'ga hech qachon chiqmaydi. Kalit yo'qolsa
// mavjud relizlarga yamoq chiqarib bo'lmaydi (yangi reliz kerak); kalit
// o'g'irlansa — darhol yangi kalit bilan reliz va eski ilova yozuvini
// panelda to'xtatish. Zaxira nusxani OFLAYN saqlang.
//
// Nashr faqat QORALAMA yaratadi. Foydalanuvchilarga chiqarish — admin
// panelda (OnDexFlow bo'limi), Authenticator kodi bilan.
// └──────────────────────────────────────────────────────────────────────┘
//
// Yamoq faqat Dart kodini o'zgartiradi. Vosita reliz va yangi APK ichidagi
// BARCHA boshqa fayllarni (native .so, assetlar, manifest) solishtiradi va
// farq bo'lsa yamoqni RAD ETADI — bunday o'zgarish yamoqda jimgina
// qo'llanmay qolardi. Bunday holatda yangi reliz kerak.
package main

import (
	"archive/zip"
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"chustapp/internal/modules/flow"
)

const defaultAPI = "https://api.ondex.uz"

// APK ichidagi papka -> Shorebird arch nomi (qurilma shu nom bilan so'raydi).
var abiArch = map[string]string{"arm64-v8a": "aarch64", "armeabi-v7a": "arm", "x86_64": "x86_64"}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = cmdKeygen(os.Args[2:])
	case "pubkey":
		err = cmdPubkey(os.Args[2:])
	case "inspect":
		err = cmdInspect(os.Args[2:])
	case "release":
		err = cmdRelease(os.Args[2:])
	case "patch":
		err = cmdPatch(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "XATO:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "ondexflow keygen|pubkey|inspect|release|patch — main.go boshidagi izohga qarang")
	os.Exit(2)
}

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return h
}

func defaultKey(name string) string { return filepath.Join(home(), ".ondexflow", "keys", name+".pem") }

// Reliz keshi: asosiy libapp.so va fayllar ro'yxati (yamoq shu bilan solishtiriladi).
func cacheDir(appID, version string) string {
	return filepath.Join(home(), ".ondexflow", "releases", appID, strings.ReplaceAll(version, "+", "_"))
}

// ── keygen / pubkey ──────────────────────────────────────────────────────

func cmdKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	name := fs.String("name", "", "ilova nomi (masalan customer_app)")
	_ = fs.Parse(args)
	if !regexp.MustCompile(`^[a-z_]{3,40}$`).MatchString(*name) {
		return errors.New("-name kerak (kichik harf va _)")
	}
	path := defaultKey(*name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("kalit allaqachon bor: %s (ustiga yozilmaydi — eski relizlar shu kalitga bog'langan)", path)
	}
	priv, pub, err := flow.GenerateKey()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, priv, 0o600); err != nil {
		return err
	}
	fmt.Println("Shaxsiy kalit:", path)
	fmt.Println("!!! Uni OFLAYN zaxiralang (fleshka/seyf). Repoga, serverga, chatga QO'YMANG.")
	fmt.Println()
	fmt.Println("Ochiq kalit (admin panel -> OnDexFlow -> Ilova qo'shish):")
	fmt.Println(pub)
	fmt.Println("Kalit ID:", flow.KeyID(pub))
	return nil
}

func loadKey(path string) (pubB64 string, k *rsa.PrivateKey, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	pk, err := flow.ParsePrivateKey(b)
	if err != nil {
		return "", nil, err
	}
	return flow.PublicKeyB64(&pk.PublicKey), pk, nil
}

func cmdPubkey(args []string) error {
	fs := flag.NewFlagSet("pubkey", flag.ExitOnError)
	key := fs.String("key", defaultKey("customer_app"), "shaxsiy kalit (PEM)")
	_ = fs.Parse(args)
	pub, _, err := loadKey(*key)
	if err != nil {
		return err
	}
	fmt.Print(pub)
	return nil
}

// ── APK tahlili ──────────────────────────────────────────────────────────

type apkInfo struct {
	libapps map[string][]byte // arch -> libapp.so
	entries map[string]string // fayl -> sha256 (libapp.so va imzo fayllaridan tashqari)
	yaml    map[string]string // assets/flutter_assets/shorebird.yaml
}

func readAPK(path string) (*apkInfo, error) {
	z, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	info := &apkInfo{libapps: map[string][]byte{}, entries: map[string]string{}, yaml: map[string]string{}}
	for _, f := range z.File {
		name := f.Name
		if strings.HasPrefix(name, "META-INF/") {
			continue // imzo fayllari har build'da boshqacha
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, 512<<20))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(name, "lib/") && strings.HasSuffix(name, "/libapp.so") {
			abi := strings.TrimSuffix(strings.TrimPrefix(name, "lib/"), "/libapp.so")
			if arch, ok := abiArch[abi]; ok {
				info.libapps[arch] = b
				continue
			}
		}
		if name == "assets/flutter_assets/shorebird.yaml" {
			for _, line := range strings.Split(string(b), "\n") {
				k, v, ok := strings.Cut(line, ":")
				if ok && !strings.HasPrefix(strings.TrimSpace(k), "#") {
					info.yaml[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
				}
			}
		}
		sum := sha256.Sum256(b)
		info.entries[name] = hex.EncodeToString(sum[:])
	}
	if len(info.libapps) == 0 {
		return nil, errors.New("APK ichida libapp.so yo'q")
	}
	return info, nil
}

// checkFlowBuild — APK haqiqatan OnDexFlow uchun yig'ilganmi: Shorebird
// dvigateli (yangilagich), bizning server va bizning kalit.
func checkFlowBuild(info *apkInfo, pub, api string) error {
	if info.yaml["app_id"] == "" {
		return errors.New("shorebird.yaml APK ichida yo'q — ilova Shorebird Flutter bilan yig'ilmagan")
	}
	if want := strings.TrimRight(api, "/") + "/flow"; info.yaml["base_url"] != want {
		return fmt.Errorf("shorebird.yaml base_url = %q, kerak: %q (aks holda qurilma Shorebird bulutiga boradi)", info.yaml["base_url"], want)
	}
	if info.yaml["patch_public_key"] != pub {
		return errors.New("APK dagi patch_public_key shu kalitniki emas — build SHOREBIRD_PUBLIC_KEY bilan qilinmagan")
	}
	return nil
}

func cmdInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	apk := fs.String("apk", "", "APK fayli")
	_ = fs.Parse(args)
	info, err := readAPK(*apk)
	if err != nil {
		return err
	}
	for _, a := range sortedKeys(info.libapps) {
		fmt.Printf("libapp %-8s %8.1f MB  %s\n", a, float64(len(info.libapps[a]))/(1<<20), flow.FileRefOf(info.libapps[a]).SHA256)
	}
	keys := sortedKeys(info.yaml)
	for _, k := range keys {
		v := info.yaml[k]
		if k == "patch_public_key" && len(v) > 24 {
			v = v[:24] + "... (ID " + flow.KeyID(v) + ")"
		}
		fmt.Printf("shorebird.yaml %s: %s\n", k, v)
	}
	fmt.Println("fayllar:", len(info.entries))
	return nil
}

// ── release ──────────────────────────────────────────────────────────────

func cmdRelease(args []string) error {
	fs := flag.NewFlagSet("release", flag.ExitOnError)
	apk := fs.String("apk", "", "reliz APK")
	version := fs.String("version", "", "pubspec versiyasi, masalan 0.2.16+28")
	key := fs.String("key", defaultKey("customer_app"), "shaxsiy kalit")
	api := fs.String("api", defaultAPI, "API manzili")
	_ = fs.Parse(args)
	pub, k, err := loadKey(*key)
	if err != nil {
		return err
	}
	info, err := readAPK(*apk)
	if err != nil {
		return err
	}
	if err := checkFlowBuild(info, pub, *api); err != nil {
		return err
	}
	appID := info.yaml["app_id"]
	entries, _ := json.MarshalIndent(info.entries, "", " ")
	m := flow.PublishManifest{Kind: "release", AppID: appID, Platform: flow.PlatformAndroid, Version: *version,
		Libapps: map[string]flow.FileRef{}, Entries: ptr(flow.FileRefOf(entries))}
	files := map[string][]byte{"entries": entries}
	for arch, b := range info.libapps {
		m.Libapps[arch] = flow.FileRefOf(b)
		files["libapp:"+arch] = b
	}
	// Avval mahalliy kesh: yamoq yasash uchun asos shu yerdan olinadi.
	dir := cacheDir(appID, *version)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	for arch, b := range info.libapps {
		if err := os.WriteFile(filepath.Join(dir, arch+".libapp.so"), b, 0o600); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "entries.json"), entries, 0o600); err != nil {
		return err
	}
	if err := upload(*api+"/flow/publish/releases", k, m, files); err != nil {
		return err
	}
	fmt.Printf("Reliz %s ro'yxatga olindi (ilova %s). Asos keshi: %s\n", *version, appID, dir)
	return nil
}

// ── patch ────────────────────────────────────────────────────────────────

func cmdPatch(args []string) error {
	fs := flag.NewFlagSet("patch", flag.ExitOnError)
	apk := fs.String("apk", "", "yangi APK (AYNAN shu versiya bilan yig'ilgan)")
	version := fs.String("version", "", "qaysi relizga, masalan 0.2.16+28")
	number := fs.Int("number", 0, "yamoq raqami (1, 2, ...)")
	note := fs.String("note", "", "izoh (panelda ko'rinadi)")
	key := fs.String("key", defaultKey("customer_app"), "shaxsiy kalit")
	api := fs.String("api", defaultAPI, "API manzili")
	patchTool := fs.String("patch-tool", filepath.Join(home(), ".shorebird", "bin", "cache", "artifacts", "patch", "patch.exe"), "bidiff vositasi")
	_ = fs.Parse(args)
	if *number < 1 {
		return errors.New("-number kerak")
	}
	pub, k, err := loadKey(*key)
	if err != nil {
		return err
	}
	info, err := readAPK(*apk)
	if err != nil {
		return err
	}
	if err := checkFlowBuild(info, pub, *api); err != nil {
		return err
	}
	appID := info.yaml["app_id"]
	dir := cacheDir(appID, *version)
	baseEntriesRaw, err := os.ReadFile(filepath.Join(dir, "entries.json"))
	if err != nil {
		return fmt.Errorf("reliz %s keshi yo'q (%s) — reliz shu kompyuterdan chiqarilganmi?", *version, dir)
	}
	var baseEntries map[string]string
	if err := json.Unmarshal(baseEntriesRaw, &baseEntries); err != nil {
		return err
	}
	// Native/asset/manifest o'zgarishi — yamoq bilan YETKAZIB BO'LMAYDI.
	var changed []string
	for name, h := range info.entries {
		if baseEntries[name] != h {
			changed = append(changed, name)
		}
	}
	for name := range baseEntries {
		if _, ok := info.entries[name]; !ok {
			changed = append(changed, name+" (o'chirilgan)")
		}
	}
	if len(changed) > 0 {
		sort.Strings(changed)
		if len(changed) > 15 {
			changed = append(changed[:15], "...")
		}
		return fmt.Errorf("Dart'dan tashqari o'zgarish bor — yamoq bilan bo'lmaydi, YANGI RELIZ kerak:\n  %s", strings.Join(changed, "\n  "))
	}
	tmp, err := os.MkdirTemp("", "ondexflow-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	m := flow.PublishManifest{Kind: "patch", AppID: appID, Platform: flow.PlatformAndroid, Version: *version,
		Number: *number, Note: *note, Patches: map[string]flow.PatchFileRef{}}
	files := map[string][]byte{}
	for arch, newLib := range info.libapps {
		base, err := os.ReadFile(filepath.Join(dir, arch+".libapp.so"))
		if err != nil {
			return fmt.Errorf("%s asosi keshda yo'q: %w", arch, err)
		}
		if bytes.Equal(base, newLib) {
			return fmt.Errorf("%s: Dart kodi relizdan farq qilmaydi — yamoqqa hojat yo'q", arch)
		}
		newPath := filepath.Join(tmp, arch+".new.so")
		basePath := filepath.Join(tmp, arch+".base.so")
		outPath := filepath.Join(tmp, arch+".patch")
		// arch qat'iy ro'yxatdan (abiArch qiymatlari) — yo'l tashqariga chiqmaydi.
		if err := os.WriteFile(newPath, newLib, 0o600); err != nil {
			return err
		}
		if err := os.WriteFile(basePath, base, 0o600); err != nil { //nolint:gosec // G703: tmp + qat'iy arch nomi
			return err
		}
		cmd := exec.Command(*patchTool, basePath, newPath, outPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("bidiff: %v: %s", err, out)
		}
		diff, err := os.ReadFile(outPath)
		if err != nil {
			return err
		}
		h := flow.FileRefOf(newLib).SHA256 // qurilma yamoqdan keyin shu xeshni kutadi
		sig, err := flow.Sign(k, []byte(h))
		if err != nil {
			return err
		}
		m.Patches[arch] = flow.PatchFileRef{FileRef: flow.FileRefOf(diff), Hash: h, Signature: sig}
		files["patch:"+arch] = diff
		fmt.Printf("%s: libapp %.1f MB -> yamoq %.1f KB\n", arch, float64(len(newLib))/(1<<20), float64(len(diff))/1024)
	}
	if err := upload(*api+"/flow/publish/patches", k, m, files); err != nil {
		return err
	}
	fmt.Printf("Yamoq #%d (%s) QORALAMA sifatida yuklandi. Chiqarish: admin panel -> OnDexFlow.\n", *number, *version)
	return nil
}

// ── tarmoq ───────────────────────────────────────────────────────────────

func upload(url string, k *rsa.PrivateKey, m flow.PublishManifest, files map[string][]byte) error {
	raw, sig, err := flow.SignManifest(k, m, time.Now())
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("manifest", string(raw))
	_ = mw.WriteField("signature", sig)
	for _, name := range sortedKeys(files) {
		fw, err := mw.CreateFormFile(name, "blob")
		if err != nil {
			return err
		}
		if _, err := fw.Write(files[name]); err != nil {
			return err
		}
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("User-Agent", "ondexflow/1")
	res, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if res.StatusCode != http.StatusCreated {
		return fmt.Errorf("server %d: %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func ptr[T any](v T) *T { return &v }
