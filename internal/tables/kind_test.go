package tables

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestParseKind(t *testing.T) {
	if k, err := ParseKind(""); err != nil || k != KindTable {
		t.Fatalf("bo'sh tur stol bo'lishi kerak: %q %v", k, err)
	}
	if k, err := ParseKind("  VIP_Room "); err != nil || k != KindVIPRoom {
		t.Fatalf("registr/bo'shliq: %q %v", k, err)
	}
	for _, bad := range []string{"sauna", "table;DROP", "стол"} {
		if _, err := ParseKind(bad); !errors.Is(err, ErrUnknownKind) {
			t.Errorf("%q qabul qilindi", bad)
		}
	}
	seen := map[string]bool{}
	for _, k := range Kinds() {
		if k.Title == "" || seen[strings.ToLower(k.Title)] {
			t.Errorf("nom bo'sh yoki takror: %+v", k)
		}
		seen[strings.ToLower(k.Title)] = true
	}
	if Kind("").Title() != "Stol" || Kind("nomalum").Title() != "Joy" {
		t.Fatal("Title standart qiymatlari")
	}
}

// Turlar ro'yxati uch joyda: Go, Postgres CHECK va restoran paneli.
// Bittasiga qo'shib boshqasini unutish — panel yubora olmaydigan yoki
// baza rad etadigan tur. Shuning uchun ular shu yerda solishtiriladi.
func TestKindsMatchMigrationAndPanel(t *testing.T) {
	var goKinds []string
	for _, k := range Kinds() {
		goKinds = append(goKinds, string(k.Kind))
	}

	sql, err := os.ReadFile("../storage/migrations/0044_table_kinds.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`CHECK \(kind IN \(([^)]*)\)\)`).FindSubmatch(sql)
	if m == nil {
		t.Fatal("migratsiyada kind CHECK topilmadi")
	}
	var sqlKinds []string
	for _, q := range regexp.MustCompile(`'([a-z_]+)'`).FindAllSubmatch(m[1], -1) {
		sqlKinds = append(sqlKinds, string(q[1]))
	}
	if strings.Join(sqlKinds, ",") != strings.Join(goKinds, ",") {
		t.Errorf("migratsiya: %v, Go: %v", sqlKinds, goKinds)
	}

	dart, err := os.ReadFile("../../apps/restaurant_panel/lib/pages/tables/table_models.dart")
	if err != nil {
		t.Fatal(err)
	}
	var panel []string
	panelTitles := map[string]string{}
	for _, d := range regexp.MustCompile(`TableKindInfo\(\s*'([a-z_]+)',\s*'([^']+)'`).FindAllSubmatch(dart, -1) {
		panel = append(panel, string(d[1]))
		panelTitles[string(d[1])] = string(d[2])
	}
	if strings.Join(panel, ",") != strings.Join(goKinds, ",") {
		t.Errorf("panel: %v, Go: %v", panel, goKinds)
	}
	for _, k := range Kinds() {
		if panelTitles[string(k.Kind)] != k.Title {
			t.Errorf("%s nomi: panel %q, Go %q", k.Kind, panelTitles[string(k.Kind)], k.Title)
		}
	}
}
