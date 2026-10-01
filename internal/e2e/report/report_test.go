package report

import (
	"encoding/json"
	"strings"
	"testing"
)

// Bo'sh ro'yxatlar JSON'da `[]` bo'lishi shart. Aks holda hisobotni o'qiydigan
// skript (CI xulosasi: python `len(r['violations'])`) `null` da yiqiladi -
// va aynan HAMMA NARSA YASHIL bo'lganda (buzilish yo'q), ya'ni eng yaxshi
// holatda CI qizil bo'lardi.
func TestBuildEmitsEmptyArraysNotNull(t *testing.T) {
	raw, err := json.Marshal(NewRecorder().Build(Coverage{}, 250, "abc"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"steps", "calls", "violations", "routes", "duplicates"} {
		if got := strings.TrimSpace(string(m[k])); got != "[]" {
			t.Errorf("%s = %s, kutilgan []", k, got)
		}
	}
	var cov map[string]json.RawMessage
	if err := json.Unmarshal(m["coverage"], &cov); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"gated_only", "uncovered", "exempted"} {
		if got := strings.TrimSpace(string(cov[k])); got != "[]" {
			t.Errorf("coverage.%s = %s, kutilgan []", k, got)
		}
	}
}
