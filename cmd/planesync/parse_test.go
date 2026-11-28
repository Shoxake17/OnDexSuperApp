package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestRefs(t *testing.T) {
	cases := []struct {
		name          string
		c             Commit
		mention, done []int
	}{
		{"faqat eslatma", Commit{Subject: "feat(shop): x (ONDEXECOSY-489)"}, []int{489}, nil},
		{"tugadi tanada", Commit{Subject: "fix: y (ONDEXECOSY-12)", Body: "Tugadi: ONDEXECOSY-12, ONDEXECOSY-13"}, []int{12, 13}, []int{12, 13}},
		{"closes inglizcha", Commit{Subject: "fix: z", Body: "Closes ONDEXECOSY-7 and ONDEXECOSY-8"}, []int{7, 8}, []int{7, 8}},
		{"yopadi va", Commit{Subject: "x", Body: "yopadi ONDEXECOSY-3 va ONDEXECOSY-4"}, []int{3, 4}, []int{3, 4}},
		{"takror bitta", Commit{Subject: "ONDEXECOSY-5 ONDEXECOSY-5"}, []int{5}, nil},
		{"hech narsa", Commit{Subject: "chore: deps"}, nil, nil},
		{"tugadi so'zi boshqa joyda", Commit{Subject: "ish tugadimi? (ONDEXECOSY-9)"}, []int{9}, nil},
	}
	for _, c := range cases {
		m, d := Refs(c.c)
		if !reflect.DeepEqual(m, c.mention) || !reflect.DeepEqual(d, c.done) {
			t.Errorf("%s: mention=%v done=%v, kutilgan %v %v", c.name, m, d, c.mention, c.done)
		}
	}
}

func TestParseGitLog(t *testing.T) {
	out := "aaa\x1ffeat: one\x1fbody 1\nline\x1e\nbbb\x1ffix: two\x1f\x1e\n"
	cs := ParseGitLog(out)
	if len(cs) != 2 || cs[0].SHA != "aaa" || cs[0].Body != "body 1\nline" || cs[1].Subject != "fix: two" {
		t.Fatalf("noto'g'ri: %+v", cs)
	}
}

func TestGroupErrors(t *testing.T) {
	logs := strings.Join([]string{
		`time=2026-11-28T10:00:00Z level=INFO msg="ok"`,
		`time=2026-11-28T10:00:01Z level=ERROR msg="buyurtma saqlanmadi" order=a1b2c3d4e5f6 err="pq: timeout after 5000ms" phone=+998901234567`,
		`time=2026-11-28T10:00:02Z level=ERROR msg="buyurtma saqlanmadi" order=ffeeddccbbaa err="pq: timeout after 3000ms"`,
		`time=2026-11-28T10:00:03Z level=ERROR msg="push yuborilmadi" err="token=abcdef secret"`,
		`panic: runtime error: index out of range [3] with length 2`,
	}, "\n")
	gs := GroupErrors(logs)
	if len(gs) != 3 {
		t.Fatalf("3 guruh kutilgan, %d: %+v", len(gs), gs)
	}
	if gs[0].Count != 2 || !strings.Contains(gs[0].Title, "timeout after Nms") {
		t.Errorf("guruhlash/normallash: %+v", gs[0])
	}
	if strings.Contains(gs[0].Sample, "901234567") {
		t.Errorf("telefon niqoblanmadi: %s", gs[0].Sample)
	}
	if strings.Contains(gs[1].Title, "abcdef") || strings.Contains(gs[1].Sample, "abcdef") {
		t.Errorf("token niqoblanmadi: %+v", gs[1])
	}
	if !strings.HasPrefix(gs[2].Title, "panic:") {
		t.Errorf("panic tutilmadi: %+v", gs[2])
	}
}

func TestMask(t *testing.T) {
	in := `user=a@b.uz ip=10.1.2.3 Authorization: Bearer eyJhbGciOi password=Qwerty1`
	got := Mask(in)
	for _, leak := range []string{"a@b.uz", "10.1.2.3", "eyJhbGciOi", "Qwerty1"} {
		if strings.Contains(got, leak) {
			t.Errorf("%q chiqib qoldi: %s", leak, got)
		}
	}
}
