package mapprovider

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Mode
		ok   bool
	}{
		{"true", Google, true},
		{"TRUE", Google, true},
		{" 1 ", Google, true},
		{"false", OndexMap, true},
		{"False", OndexMap, true},
		{"0", OndexMap, true},
		{"off", OndexMap, true},
		// Bo'sh va tushunarsiz qiymat — hozirgi production (Google).
		{"", Google, false},
		{"ha", Google, false},
		{"falsee", Google, false},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("Parse(%q) = %v,%v; kutilgan %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestFromEnv(t *testing.T) {
	get := func(v string) func(string) string {
		return func(k string) string {
			if k != "GOOGLE_MAP" {
				t.Fatalf("kutilmagan kalit %q", k)
			}
			return v
		}
	}
	if FromEnv(get("false")) != OndexMap {
		t.Fatal("false → OnDexMap bo'lishi kerak")
	}
	if FromEnv(get("")) != Google || FromEnv(get("xato")) != Google {
		t.Fatal("bo'sh/noto'g'ri qiymat Google bo'lishi kerak")
	}
}
