package main

import "testing"

func TestCheckReleaseTarget(t *testing.T) {
	for in, ok := range map[string]bool{
		"https://cdn.ondex.uz":                                true,
		"https://cdn.ondex.uz/":                               true,
		"https://pub-4ec5d645c8884863ac4649521a8a7b82.r2.dev": false,
		"":                    false,
		"http://cdn.ondex.uz": false,
	} {
		if err := checkReleaseTarget(in); (err == nil) != ok {
			t.Errorf("%q: err=%v, ok kutilgan=%v", in, err, ok)
		}
	}
}
