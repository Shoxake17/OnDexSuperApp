package revoke

import (
	"context"
	"testing"
	"time"
)

func TestRevokeInvalidatesOlderTokens(t *testing.T) {
	// Redis'siz (nil) ham to'liq ishlashi kerak — bekor qilish RAM'da.
	s := New(nil, time.Hour)
	ctx := context.Background()

	old := time.Now().Add(-time.Minute) // bekor qilishdan OLDIN chiqarilgan
	if s.IsRevoked("u1", old) {
		t.Fatal("hali hech narsa bekor qilinmagan")
	}

	s.Revoke(ctx, "u1")

	if !s.IsRevoked("u1", old) {
		t.Error("eski token bekor qilinishi kerak edi")
	}
	// Bekor qilingandan KEYIN chiqarilgan token (qayta login) yaroqli.
	if s.IsRevoked("u1", time.Now().Add(time.Second)) {
		t.Error("yangi token yaroqli qolishi kerak edi")
	}
	// Boshqa foydalanuvchiga ta'sir qilmaydi.
	if s.IsRevoked("u2", old) {
		t.Error("boshqa foydalanuvchi tegilmasligi kerak edi")
	}
}

func TestRevokeIgnoresEmptyUser(t *testing.T) {
	s := New(nil, time.Hour)
	s.Revoke(context.Background(), "")
	if s.IsRevoked("", time.Now().Add(-time.Hour)) {
		t.Error("bo'sh ID uchun belgi qo'yilmasligi kerak edi")
	}
}
