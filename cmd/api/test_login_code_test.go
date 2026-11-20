package main

import (
	"testing"

	"chustapp/internal/users"
)

func TestTestLoginCode(t *testing.T) {
	cases := []struct {
		dev     bool
		raw     string
		want    string
		wantErr bool
	}{
		{dev: true, raw: "", want: users.TestLoginCode},
		{dev: true, raw: "123456", want: "123456"},
		{dev: false, raw: "", wantErr: true},
		{dev: false, raw: users.TestLoginCode, wantErr: true}, // ommaviy kod prod'da yo'q
		{dev: false, raw: " 482913 ", want: "482913"},
	}
	for _, c := range cases {
		got, err := testLoginCode(c.dev, c.raw)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("dev=%v raw=%q: got=%q err=%v", c.dev, c.raw, got, err)
		}
	}
}
