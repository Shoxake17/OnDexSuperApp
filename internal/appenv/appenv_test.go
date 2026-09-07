package appenv

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Reyestrni MANBA KODI va DEPLOY konfiguratsiyasi bilan solishtiruvchi
// testlar (bug.md 42, 52, 53, 99-bandlar).
//
// ┌─ NEGA TEST, IZOH EMAS ─────────────────────────────────────────────┐
// `deploy/docker-compose.prod.yml` ning o'zida bu tuzoq haqida IKKI
// MARTA yozilgan ogohlantirish bor — va xato baribir yana ikki marta
// takrorlandi. Izoh o'qilmaydi; test yiqiladi.
// └────────────────────────────────────────────────────────────────────┘

// repoRoot — `internal/appenv` dan ikki pog'ona yuqori.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

var getenvRe = regexp.MustCompile(`os\.Getenv\("([A-Z0-9_]+)"\)`)

// stripLineComments — `//` dan keyingi matnni olib tashlaydi.
//
// Zarur: bu paketning O'Z izohida `os.Getenv("X")` namunasi bor va
// usiz test uni haqiqiy chaqiruv deb hisoblardi. Izohga chiqarilgan
// (o'chirilgan) kod ham hisobga olinmasligi kerak.
//
// Sodda usul — bu yerda yetarli: bizni faqat `os.Getenv("NOM")`
// naqshi qiziqtiradi va u satr literali ichida uchramaydi.
func stripLineComments(src string) string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "//"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}

// sourceEnvNames — `internal/` va `cmd/` dagi ishlab chiqarish
// kodida `os.Getenv("NOM")` bilan o'qilgan barcha nomlar.
func sourceEnvNames(t *testing.T) map[string][]string {
	t.Helper()
	root := repoRoot(t)
	found := map[string][]string{}

	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			for _, m := range getenvRe.FindAllStringSubmatch(stripLineComments(string(data)), -1) {
				found[m[1]] = append(found[m[1]], rel)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return found
}

func registryNames() map[string]Var {
	out := make(map[string]Var, len(Registry))
	for _, v := range Registry {
		out[v.Name] = v
	}
	return out
}

// ASOSIY QULF: kodda o'qiladigan har bir o'zgaruvchi reyestrda
// bo'lishi SHART. Yangi `os.Getenv("X")` yozgan odam reyestrni
// to'ldirmasa, shu test yiqiladi va u compose'ni ham unutmaydi.
func TestRegistryCoversEveryGetenvInSource(t *testing.T) {
	reg := registryNames()
	for name, files := range sourceEnvNames(t) {
		if _, ok := reg[name]; !ok {
			t.Errorf("%q kodda o'qiladi (%s), lekin appenv.Registry da YO'Q — "+
				"reyestrga qo'shing va deploy compose'ini ham yangilang",
				name, strings.Join(files, ", "))
		}
	}
}

// Teskari tomon: reyestrda kodda umuman ishlatilmaydigan yozuv
// qolmasin — aks holda ro'yxat asta-sekin haqiqatdan uzoqlashadi.
func TestRegistryHasNoStaleEntries(t *testing.T) {
	src := sourceEnvNames(t)
	for _, v := range Registry {
		if _, ok := src[v.Name]; !ok {
			t.Errorf("%q reyestrda bor, lekin kodda hech qayerda o'qilmaydi — "+
				"olib tashlang", v.Name)
		}
	}
}

// Har bir yozuvda oqibati yozilgan bo'lishi kerak: log xabari aynan
// shundan quriladi ("nima ishlamaydi"), ya'ni bo'sh matn xabarni
// foydasiz qiladi.
func TestEveryRegistryEntryExplainsItsEffect(t *testing.T) {
	for _, v := range Registry {
		if strings.TrimSpace(v.Effect) == "" {
			t.Errorf("%q: Effect bo'sh — yo'qolganda nima ishlamasligini yozing", v.Name)
		}
	}
	// Nomlar takrorlanmasin.
	seen := map[string]bool{}
	for _, v := range Registry {
		if seen[v.Name] {
			t.Errorf("%q reyestrda ikki marta", v.Name)
		}
		seen[v.Name] = true
	}
}

// DEPLOY QULFI: `DeployRequired` deb belgilangan har bir nom
// production compose'ining `environment:` ro'yxatida SANAB CHIQILGAN
// bo'lishi kerak.
//
// `deploy/` papkasi `.gitignore` da, ya'ni CI'da bu fayl bo'lmaydi —
// o'shanda test o'tkazib yuboriladi. Ishlab chiquvchining mashinasida
// esa u bor va aynan shu yerda xato tutiladi.
func TestProdComposeListsEveryDeployRequiredVar(t *testing.T) {
	path := filepath.Join(repoRoot(t), "deploy", "docker-compose.prod.yml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("deploy/docker-compose.prod.yml yo'q (.gitignore da) — tekshiruv o'tkazib yuborildi")
	}
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	for _, v := range Registry {
		if !v.DeployRequired {
			continue
		}
		// Compose'da o'zgaruvchi `NOM: ${NOM:-}` ko'rinishida sanaladi.
		// Faqat `${NOM}` ni izlash yetarli emas: u compose faylining
		// O'ZIDA almashtirish uchun ham ishlatiladi va konteynerga
		// o'tishini bildirmaydi — aynan shu farq 99-bandning sababi.
		if !strings.Contains(text, "\n      "+v.Name+":") {
			t.Errorf("%q `api.environment` ro'yxatida YO'Q — konteyner uni ko'rmaydi. Oqibati: %s",
				v.Name, v.Effect)
		}
	}
}

// `Missing()` haqiqatan bo'sh o'zgaruvchilarni topadimi.
func TestMissingDetectsUnsetAndBlank(t *testing.T) {
	const name = "JWT_SECRET" // reyestrdagi mavjud nom

	t.Setenv(name, "")
	if !containsVar(Missing(), name) {
		t.Fatal("bo'sh qiymat 'yo'q' deb hisoblanmadi")
	}
	t.Setenv(name, "   ")
	if !containsVar(Missing(), name) {
		t.Fatal("faqat probeldan iborat qiymat 'yo'q' deb hisoblanmadi")
	}
	t.Setenv(name, "haqiqiy-qiymat")
	if containsVar(Missing(), name) {
		t.Fatal("o'rnatilgan qiymat 'yo'q' deb hisoblandi")
	}
}

func containsVar(list []Var, name string) bool {
	for _, v := range list {
		if v.Name == name {
			return true
		}
	}
	return false
}
