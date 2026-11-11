package main

import (
	"testing"

	"chustapp/internal/identity"
)

// PK-4: standart Android ilovalari ro'yxati to'g'ri o'qiladi (xato bo'lsa prod'da
// passkey jimgina O'CHIQ bo'lardi) va uchala ilova bitta release kaliti bilan.
func TestDefaultAndroidAppsParse(t *testing.T) {
	apps, err := identity.ParseAndroidApps(defaultAndroidApps)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"com.ondex.waiter": true, "com.ondex.courier": true, "com.ondex.customer": true}
	if len(apps) != len(want) {
		t.Fatalf("ilovalar soni: %d", len(apps))
	}
	for _, a := range apps {
		if !want[a.Package] {
			t.Fatalf("kutilmagan paket: %s", a.Package)
		}
		if a.Origin() != "android:apk-key-hash:_SpIXYUGyifIfBlNjYFp-CXxgGo76n-daM7OzhHliFU" {
			t.Fatalf("%s origin: %s", a.Package, a.Origin())
		}
	}
}
