package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestParseReleaseVersion(t *testing.T) {
	v, b, err := parseReleaseVersion("0.2.3+15")
	if err != nil || v != "0.2.3" || b != 15 {
		t.Fatalf("0.2.3+15: %q %d %v", v, b, err)
	}
	for _, bad := range []string{"", "0.2.3", "0.2+1", "v0.2.3+1", "0.2.3+0", "0.2.3+x", "0.2.3-dev+4"} {
		if _, _, err := parseReleaseVersion(bad); err == nil {
			t.Errorf("%q rad etilishi kerak edi", bad)
		}
	}
}

func writeZip(t *testing.T, names ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.apk")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("x"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return path
}

func TestCheckAPK(t *testing.T) {
	if err := checkAPK(writeZip(t, "AndroidManifest.xml", "classes.dex", "res/a.png")); err != nil {
		t.Fatalf("to'g'ri APK: %v", err)
	}
	if err := checkAPK(writeZip(t, "readme.txt")); err == nil {
		t.Fatal("manifestsiz arxiv rad etilishi kerak edi")
	}
	notZip := filepath.Join(t.TempDir(), "x.apk")
	_ = os.WriteFile(notZip, []byte("not a zip"), 0o600)
	if err := checkAPK(notZip); err == nil {
		t.Fatal("ZIP bo'lmagan fayl rad etilishi kerak edi")
	}
}

func TestPubspecVersion(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pubspec.yaml")
	_ = os.WriteFile(p, []byte("name: x\nversion: 0.2.2+14\n\nenvironment:\n"), 0o600)
	v, err := pubspecVersion(p)
	if err != nil || v != "0.2.2+14" {
		t.Fatalf("%q %v", v, err)
	}
}
