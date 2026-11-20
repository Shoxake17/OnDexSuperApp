package main

import (
	"errors"
	"strings"

	"chustapp/internal/users"
)

// testLoginCode — TEST_OTP test akkauntlari uchun sobit kod.
//
// Dev'da berilmasa ommaviy standart (users.TestLoginCode) ishlatiladi.
// Production'da kod MAXFIY bo'lishi shart: repo ochiq, ya'ni standart
// qiymatni hamma biladi va u bilan haqiqiy restoranning ofitsianti yoki
// kuryeri bo'lib kirish mumkin edi (audit 480).
func testLoginCode(devMode bool, raw string) (string, error) {
	code := strings.TrimSpace(raw)
	if code == "" {
		if devMode {
			return users.TestLoginCode, nil
		}
		return "", errors.New("production'da TEST_OTP_CODE (maxfiy 6 xonali kod) majburiy")
	}
	if !devMode && code == users.TestLoginCode {
		return "", errors.New("TEST_OTP_CODE ommaviy standart qiymatga teng — boshqa maxfiy kod tanlang")
	}
	return code, nil
}
