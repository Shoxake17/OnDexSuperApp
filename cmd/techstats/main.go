// techstats — loyihada qaysi dasturlash tillari va texnologiyalar
// ishlatilganini va ularning ULUSHINI hisoblaydi.
//
// Ishlatish (repo ildizidan):
//
//	go run ./cmd/techstats
//
// ┌─ QANDAY HISOBLANADI ─────────────────────────────────────────────────┐
// Fayllar `git ls-files` dan olinadi — ya'ni `.gitignore` dagi hamma narsa
// (build/, node_modules/, .dart_tool/) avtomatik chetda qoladi.
//
// Ulush — BO'SH BO'LMAGAN QATORLAR bo'yicha (izohlar ham kiradi: bu
// loyihada izoh kodning bir qismi). Fayllar uch guruhga bo'linadi va
// foizlar faqat birinchisi bo'yicha hisoblanadi, aks holda natija
// yolg'on bo'lardi:
//
//  1. Loyiha kodi — jamoa yozgan kod.
//  2. Platforma shabloni — Flutter yaratgan android/ios/windows/linux/
//     macos papkalari (runner, Gradle, CMake). Asosan avtomatik.
//  3. Tashqi kod — repoga ko'chirib olingan kutubxonalar
//     (`local_packages/`).
//
// Generatsiya qilingan fayllar (`*.g.dart`, lock fayllar) va rasm/ovoz
// kabi ikkilik fayllar umuman sanalmaydi.
//
// Texnologiyalar manifestlardan o'qiladi: `go.mod`, har bir
// `pubspec.yaml`, `package.json`, `Dockerfile`, `docker-compose*.yml` va
// `.github/workflows`.
// └──────────────────────────────────────────────────────────────────────┘
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
)

// ---------- Tillar ----------

// langKind — til turi: dasturlash tili yoki ma'lumot/sozlama formati.
type langKind int

const (
	kindCode langKind = iota
	kindData
)

type lang struct {
	name string
	kind langKind
}

var byExt = map[string]lang{
	".go":     {"Go", kindCode},
	".dart":   {"Dart", kindCode},
	".ts":     {"TypeScript", kindCode},
	".tsx":    {"TypeScript", kindCode},
	".js":     {"JavaScript", kindCode},
	".mjs":    {"JavaScript", kindCode},
	".cjs":    {"JavaScript", kindCode},
	".jsx":    {"JavaScript", kindCode},
	".sql":    {"SQL", kindCode},
	".css":    {"CSS", kindCode},
	".scss":   {"CSS", kindCode},
	".html":   {"HTML", kindCode},
	".kt":     {"Kotlin", kindCode},
	".kts":    {"Kotlin (Gradle)", kindCode},
	".gradle": {"Groovy (Gradle)", kindCode},
	".java":   {"Java", kindCode},
	".swift":  {"Swift", kindCode},
	".m":      {"Objective-C", kindCode},
	".mm":     {"Objective-C", kindCode},
	".c":      {"C", kindCode},
	".h":      {"C/C++ sarlavha", kindCode},
	".cc":     {"C++", kindCode},
	".cpp":    {"C++", kindCode},
	".cmake":  {"CMake", kindCode},
	".ps1":    {"PowerShell", kindCode},
	".psm1":   {"PowerShell", kindCode},
	".sh":     {"Shell", kindCode},
	".bash":   {"Shell", kindCode},
	".bat":    {"Batch", kindCode},
	".cmd":    {"Batch", kindCode},
	".py":     {"Python", kindCode},
	".gd":     {"GDScript", kindCode},
	".rc":     {"Windows resurs", kindCode},

	".yaml":  {"YAML", kindData},
	".yml":   {"YAML", kindData},
	".json":  {"JSON", kindData},
	".arb":   {"JSON", kindData},
	".xml":   {"XML", kindData},
	".plist": {"XML", kindData},
	".toml":  {"TOML", kindData},
	".md":    {"Markdown", kindData},
	".proto": {"Protobuf", kindData},
}

// byName — kengaytmasiz, nomi bilan taniladigan fayllar.
var byName = map[string]lang{
	"Dockerfile":     {"Dockerfile", kindCode},
	"CMakeLists.txt": {"CMake", kindCode},
	"Makefile":       {"Makefile", kindCode},
	"Caddyfile":      {"Caddyfile", kindData},
}

func langOf(p string) (lang, bool) {
	base := path.Base(p)
	if l, ok := byName[base]; ok {
		return l, true
	}
	if strings.HasPrefix(base, "Dockerfile") {
		return byName["Dockerfile"], true
	}
	l, ok := byExt[strings.ToLower(path.Ext(p))]
	return l, ok
}

// ---------- Guruhlar ----------

type group int

const (
	groupOwn group = iota
	groupPlatform
	groupVendored
)

var platformDirRe = regexp.MustCompile(`^apps/[^/]+/(android|ios|windows|linux|macos)/`)

// generated — avtomatik yaratilgan fayllar: sanalmaydi.
func generated(p string) bool {
	base := path.Base(p)
	for _, suf := range []string{".g.dart", ".freezed.dart", ".pb.go", ".min.js", ".min.css"} {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	switch base {
	case "go.sum", "pubspec.lock", "package-lock.json", "pnpm-lock.yaml", "yarn.lock",
		"GeneratedPluginRegistrant.java", "GeneratedPluginRegistrant.swift",
		"generated_plugin_registrant.cc", "generated_plugin_registrant.h",
		"generated_plugins.cmake":
		return true
	}
	return false
}

func groupOf(p string) group {
	switch {
	case strings.HasPrefix(p, "local_packages/"):
		return groupVendored
	case platformDirRe.MatchString(p):
		return groupPlatform
	}
	return groupOwn
}

// ---------- Sanash ----------

type tally struct {
	files int
	lines int
}

type langStats map[string]*tally

func (s langStats) add(name string, lines int) {
	t := s[name]
	if t == nil {
		t = &tally{}
		s[name] = t
	}
	t.files++
	t.lines += lines
}

func (s langStats) total() (files, lines int) {
	for _, t := range s {
		files += t.files
		lines += t.lines
	}
	return
}

// nonBlankLines — bo'sh bo'lmagan qatorlar. Ikkilik fayl (NUL bayt) — -1.
func nonBlankLines(data []byte) int {
	if bytes.IndexByte(data, 0) >= 0 {
		return -1
	}
	n := 0
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) > 0 {
			n++
		}
	}
	return n
}

// ---------- Texnologiyalar ----------

// techHints — manifestdagi bog'liqlik nomidan (qism-satr) tushunarli
// texnologiya nomiga. Faqat manifestda HAQIQATAN uchragan narsa
// ko'rsatiladi — bu ro'yxat hech narsani o'zicha "bor" demaydi.
var techHints = []struct{ match, name string }{
	{"jackc/pgx", "PostgreSQL"},
	{"lib/pq", "PostgreSQL"},
	{"postgis", "PostGIS"},
	{"mongo-driver", "MongoDB"},
	{"redis", "Redis"},
	{"meilisearch", "MeiliSearch"},
	{"aws-sdk-go-v2/service/s3", "S3 / Cloudflare R2"},
	{"firebase", "Firebase"},
	{"google.golang.org/api", "Google API'lari"},
	{"gorilla/websocket", "WebSocket"},
	{"coder/websocket", "WebSocket"},
	{"web_socket_channel", "WebSocket"},
	{"golang-jwt", "JWT"},
	{"x/crypto", "Argon2 / kriptografiya"},
	{"google_maps_flutter", "Google Maps"},
	{"flutter_map", "flutter_map (OSM xaritalar)"},
	{"geolocator", "GPS (geolocator)"},
	{"posthog", "PostHog (analitika)"},
	{"url_launcher", "url_launcher"},
	{"flutter_riverpod", "Riverpod"},
	{"provider", "Provider"},
	{"shared_preferences", "shared_preferences"},
	{"flutter_secure_storage", "Secure Storage"},
	{"webview_flutter", "WebView"},
	{"next", "Next.js"},
	{"react", "React"},
	{"tailwindcss", "Tailwind CSS"},
	{"typescript", "TypeScript"},
	{"lucide-react", "Lucide ikonkalar"},
	{"model-viewer", "model-viewer (3D/AR)"},
}

type techSet map[string]map[string]bool // texnologiya -> qaysi manifestlarda

func (t techSet) mark(dep, where string) {
	for _, h := range techHints {
		if strings.Contains(dep, h.match) {
			if t[h.name] == nil {
				t[h.name] = map[string]bool{}
			}
			t[h.name][where] = true
		}
	}
}

// goModDeps — `go.mod` dagi TO'G'RIDAN-TO'G'RI bog'liqliklar.
func goModDeps(data []byte) []string {
	var deps []string
	inBlock := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "require ("):
			inBlock = true
			continue
		case inBlock && line == ")":
			inBlock = false
			continue
		}
		if strings.Contains(line, "// indirect") {
			continue
		}
		if inBlock && line != "" && !strings.HasPrefix(line, "//") {
			deps = append(deps, strings.Fields(line)[0])
		} else if strings.HasPrefix(line, "require ") {
			if f := strings.Fields(line); len(f) >= 2 {
				deps = append(deps, f[1])
			}
		}
	}
	return deps
}

// pubspecDeps — `dependencies:` bloki nomlari (dev_dependencies emas).
// YAML kutubxonasisiz: blok ikki bo'shliq bilan boshlangan kalitlar.
func pubspecDeps(data []byte) []string {
	var deps []string
	in := false
	for _, raw := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if raw == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			continue
		}
		if !strings.HasPrefix(raw, " ") {
			in = strings.HasPrefix(raw, "dependencies:")
			continue
		}
		if in && strings.HasPrefix(raw, "  ") && !strings.HasPrefix(raw, "   ") {
			if name, _, ok := strings.Cut(strings.TrimSpace(raw), ":"); ok {
				deps = append(deps, name)
			}
		}
	}
	return deps
}

func packageJSONDeps(data []byte) []string {
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return nil
	}
	var deps []string
	for k := range pkg.Dependencies {
		deps = append(deps, k)
	}
	for k := range pkg.DevDependencies {
		deps = append(deps, k)
	}
	return deps
}

var (
	composeImageRe = regexp.MustCompile(`(?m)^\s*image:\s*["']?([^\s"'#]+)`)
	dockerFromRe   = regexp.MustCompile(`(?mi)^\s*FROM\s+(\S+)`)
)

// imageName — "postgis/postgis:16-3.4" -> "postgis/postgis".
func imageName(ref string) string {
	if i := strings.LastIndex(ref, "@"); i > 0 {
		ref = ref[:i]
	}
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		ref = ref[:i]
	}
	return ref
}

// ---------- Asosiy ----------

// scan — bitta o'tishda yig'ilgan natijalar.
type scan struct {
	files                                int
	stats                                map[group]langStats
	skippedGen, skippedBin, skippedOther int
	techs                                techSet
	goDeps, dartDeps, npmDeps            []string
	dartApps                             map[string]int
	images                               map[string]bool
	workflows                            int
}

func main() {
	out, err := exec.Command("git", "ls-files", "-z").Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "git ls-files ishlamadi (repo ildizidan ishga tushiring):", err)
		os.Exit(1)
	}
	files := strings.Split(strings.TrimRight(string(out), "\x00"), "\x00")

	// `os.Root` — yo'llar git'dan kelsa ham, o'qish repo ildizidan
	// tashqariga (`../`, belgili havola) chiqa olmaydi.
	root, err := os.OpenRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "repo ildizini ochib bo'lmadi:", err)
		os.Exit(1)
	}
	defer root.Close()

	s := &scan{
		files:    len(files),
		stats:    map[group]langStats{groupOwn: {}, groupPlatform: {}, groupVendored: {}},
		techs:    techSet{},
		dartApps: map[string]int{},
		images:   map[string]bool{},
	}
	for _, p := range files {
		if p == "" {
			continue
		}
		data, err := root.ReadFile(p)
		if err != nil {
			continue // o'chirilgan, lekin hali indeksdagi fayl
		}
		// Manifestlar — texnologiyalar uchun; tashqi kod hisobga olinmaydi.
		if groupOf(p) != groupVendored {
			s.manifest(p, data)
		}
		s.count(p, data)
	}
	s.markImages()
	s.report()
}

// manifest — fayl manifest bo'lsa, undan bog'liqliklarni yig'adi.
func (s *scan) manifest(p string, data []byte) {
	base, dir := path.Base(p), path.Dir(p)
	switch {
	case base == "go.mod":
		s.goDeps = goModDeps(data)
		s.markAll(s.goDeps, "go.mod")
	case base == "pubspec.yaml":
		deps := pubspecDeps(data)
		s.dartApps[dir] = len(deps)
		s.dartDeps = append(s.dartDeps, deps...)
		s.markAll(deps, dir)
	case base == "package.json" && !strings.Contains(p, "node_modules/"):
		deps := packageJSONDeps(data)
		s.npmDeps = append(s.npmDeps, deps...)
		s.markAll(deps, dir)
	case strings.HasPrefix(base, "docker-compose") && (strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml")):
		s.addImages(composeImageRe, data)
	case strings.HasPrefix(base, "Dockerfile"):
		s.addImages(dockerFromRe, data)
	case strings.HasPrefix(p, ".github/workflows/"):
		s.workflows++
	}
}

func (s *scan) markAll(deps []string, where string) {
	for _, d := range deps {
		s.techs.mark(d, where)
	}
}

func (s *scan) addImages(re *regexp.Regexp, data []byte) {
	for _, m := range re.FindAllSubmatch(data, -1) {
		s.images[imageName(string(m[1]))] = true
	}
}

// count — faylni tegishli guruh va til bo'yicha sanaydi.
func (s *scan) count(p string, data []byte) {
	if generated(p) {
		s.skippedGen++
		return
	}
	l, ok := langOf(p)
	if !ok {
		s.skippedOther++
		return
	}
	n := nonBlankLines(data)
	if n < 0 {
		s.skippedBin++
		return
	}
	s.stats[groupOf(p)].add(l.name, n)
}

// imageTechs — konteyner image nomidan texnologiya (masalan postgis, redis).
var imageTechs = []struct{ match, name string }{
	{"mongo", "MongoDB"}, {"postgres", "PostgreSQL"}, {"caddy", "Caddy"},
	{"golang", "Go (build image)"}, {"node", "Node.js"}, {"distroless", "Distroless"},
}

func (s *scan) markImages() {
	for img := range s.images {
		s.techs.mark(img, "Docker")
		for _, h := range imageTechs {
			if !strings.Contains(img, h.match) {
				continue
			}
			if s.techs[h.name] == nil {
				s.techs[h.name] = map[string]bool{}
			}
			s.techs[h.name]["Docker"] = true
		}
	}
}

// kindOf — til nomidan uning turi (dasturlash tili yoki ma'lumot formati).
func kindOf() map[string]langKind {
	m := map[string]langKind{}
	for _, l := range byExt {
		m[l.name] = l.kind
	}
	for _, l := range byName {
		m[l.name] = l.kind
	}
	return m
}

func (s *scan) report() {
	kinds := kindOf()
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println(" ChustApp (OnDex) — tillar va texnologiyalar")
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Printf(" git'dagi fayllar: %d  (sanalmadi: generatsiya %d, ikkilik %d, rasm/ovoz/boshqa %d)\n\n",
		s.files, s.skippedGen, s.skippedBin, s.skippedOther)

	printTable("1. LOYIHA KODI — dasturlash tillari (jamoa yozgan kod)", s.stats[groupOwn], kinds, kindCode)
	printTable("2. LOYIHA — sozlama va ma'lumot formatlari", s.stats[groupOwn], kinds, kindData)

	pf, pl := s.stats[groupPlatform].total()
	vf, vl := s.stats[groupVendored].total()
	fmt.Println("3. FOIZLARGA KIRITILMAGAN")
	fmt.Printf("   Flutter platforma shabloni (android/ios/windows...): %d fayl, %d qator — %s\n",
		pf, pl, topLangs(s.stats[groupPlatform], 4))
	fmt.Printf("   Tashqi kod (local_packages/):                     %d fayl, %d qator — %s\n\n",
		vf, vl, topLangs(s.stats[groupVendored], 3))

	s.printTechs()

	_, ownCode := sumKind(s.stats[groupOwn], kinds, kindCode)
	fmt.Printf("\n Jami loyiha kodi: %d qator (bo'sh qatorlarsiz, izohlar bilan)\n", ownCode)
}

func (s *scan) printTechs() {
	fmt.Println("4. TEXNOLOGIYALAR (manifestlardan)")
	fmt.Printf("   Go bog'liqliklari (to'g'ridan-to'g'ri):  %d\n", len(s.goDeps))
	fmt.Printf("   Flutter paketlari (jami, %d manifest):   %d\n", len(s.dartApps), len(s.dartDeps))
	fmt.Printf("   npm paketlari:                           %d\n", len(s.npmDeps))
	fmt.Printf("   GitHub Actions workflow'lari:            %d\n\n", s.workflows)

	// Ko'p joyda ishlatilgani yuqorida.
	names := make([]string, 0, len(s.techs))
	for n := range s.techs {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if a, b := len(s.techs[names[i]]), len(s.techs[names[j]]); a != b {
			return a > b
		}
		return names[i] < names[j]
	})
	for _, n := range names {
		fmt.Printf("   • %-30s %s\n", n, whereList(s.techs[n]))
	}

	if len(s.images) > 0 {
		fmt.Printf("\n   Docker image'lari: %s\n", whereList(s.images))
	}
}

func sumKind(s langStats, kindOf map[string]langKind, k langKind) (files, lines int) {
	for name, t := range s {
		if kindOf[name] == k {
			files += t.files
			lines += t.lines
		}
	}
	return
}

func printTable(title string, s langStats, kindOf map[string]langKind, k langKind) {
	type row struct {
		name string
		t    *tally
	}
	var rows []row
	for name, t := range s {
		if kindOf[name] == k {
			rows = append(rows, row{name, t})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].t.lines > rows[j].t.lines })
	_, total := sumKind(s, kindOf, k)

	fmt.Println(title)
	fmt.Printf("   %-18s %7s %9s %7s  %s\n", "Til", "Fayl", "Qator", "Ulush", "")
	for _, r := range rows {
		pct := 0.0
		if total > 0 {
			pct = float64(r.t.lines) * 100 / float64(total)
		}
		fmt.Printf("   %-18s %7d %9d %6.1f%%  %s\n", r.name, r.t.files, r.t.lines, pct, bar(pct))
	}
	fmt.Println()
}

// bar — ulush chizig'i (har belgi = 2%).
func bar(pct float64) string {
	n := int(pct/2 + 0.5)
	if n == 0 && pct > 0 {
		return "▏"
	}
	return strings.Repeat("█", n)
}

func topLangs(s langStats, n int) string {
	type kv struct {
		name  string
		lines int
	}
	var list []kv
	for name, t := range s {
		list = append(list, kv{name, t.lines})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].lines > list[j].lines })
	var parts []string
	for i, x := range list {
		if i == n {
			break
		}
		parts = append(parts, fmt.Sprintf("%s %d", x.name, x.lines))
	}
	if len(parts) == 0 {
		return "yo'q"
	}
	return strings.Join(parts, ", ")
}

func whereList(w map[string]bool) string {
	list := make([]string, 0, len(w))
	for k := range w {
		list = append(list, k)
	}
	sort.Strings(list)
	return strings.Join(list, ", ")
}
