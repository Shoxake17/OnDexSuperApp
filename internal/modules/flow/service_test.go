package flow

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type memBlobs struct{ data map[string][]byte }

func (b *memBlobs) Put(_ context.Context, key string, data []byte, _ string) (string, error) {
	b.data[key] = append([]byte(nil), data...)
	return "https://cdn.test/" + key, nil
}

type fixture struct {
	svc   *Service
	blobs *memBlobs
	key   *rsa.PrivateKey
	app   *App
	now   time.Time
}

var idSeq atomic.Int64

func newFixture(t *testing.T) *fixture {
	t.Helper()
	privPEM, pub, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	k, err := ParsePrivateKey(privPEM)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{blobs: &memBlobs{data: map[string][]byte{}}, key: k, now: time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)}
	f.svc, err = New(Deps{Store: NewMemStore(), Blobs: f.blobs,
		NewID: func() string { return fmt.Sprintf("id%d", idSeq.Add(1)) },
		Now:   func() time.Time { return f.now }})
	if err != nil {
		t.Fatal(err)
	}
	f.app, err = f.svc.CreateApp(context.Background(), "", "OnDex", PlatformAndroid, pub, "admin1")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func hexSum(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

func (f *fixture) release(t *testing.T, version string) *Release {
	t.Helper()
	lib := []byte("base-libapp-" + version)
	ent := []byte(`{"lib/arm64-v8a/libflutter.so":"x"}`)
	m := PublishManifest{Kind: "release", AppID: f.app.ID, Platform: PlatformAndroid, Version: version,
		Libapps: map[string]FileRef{"aarch64": FileRefOf(lib)}, Entries: ptr(FileRefOf(ent))}
	raw, sig, err := SignManifest(f.key, m, f.now)
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.svc.PublishRelease(context.Background(), raw, sig, Files{"libapp:aarch64": lib, "entries": ent})
	if err != nil {
		t.Fatalf("reliz: %v", err)
	}
	return r
}

func (f *fixture) patchManifest(t *testing.T, version string, n int, body []byte) (PublishManifest, Files) {
	t.Helper()
	newLib := []byte(fmt.Sprintf("new-libapp-%d", n))
	h := hexSum(newLib)
	sig, err := Sign(f.key, []byte(h))
	if err != nil {
		t.Fatal(err)
	}
	return PublishManifest{Kind: "patch", AppID: f.app.ID, Platform: PlatformAndroid, Version: version, Number: n,
		Patches: map[string]PatchFileRef{"aarch64": {FileRef: FileRefOf(body), Hash: h, Signature: sig}}}, Files{"patch:aarch64": body}
}

func (f *fixture) patch(t *testing.T, version string, n int) *Patch {
	t.Helper()
	m, files := f.patchManifest(t, version, n, []byte(fmt.Sprintf("diff-%d", n)))
	raw, sig, err := SignManifest(f.key, m, f.now)
	if err != nil {
		t.Fatal(err)
	}
	p, err := f.svc.PublishPatch(context.Background(), raw, sig, files)
	if err != nil {
		t.Fatalf("yamoq %d: %v", n, err)
	}
	return p
}

func ptr[T any](v T) *T { return &v }

func (f *fixture) check(t *testing.T, version, client string, cur *int) CheckResponse {
	t.Helper()
	r, err := f.svc.Check(context.Background(), CheckRequest{AppID: f.app.ID, ReleaseVersion: version,
		Platform: PlatformAndroid, Arch: "aarch64", ClientID: client, CurrentPatchNumber: cur})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDraftIsNeverOffered(t *testing.T) {
	f := newFixture(t)
	f.release(t, "0.2.16+28")
	f.patch(t, "0.2.16+28", 1)
	if r := f.check(t, "0.2.16+28", "c1", nil); r.PatchAvailable {
		t.Fatal("qoralama yamoq qurilmaga berildi")
	}
}

func TestActivateOfferRollbackFlow(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.release(t, "0.2.16+28")
	p := f.patch(t, "0.2.16+28", 1)
	p, err := f.svc.SetPatchState(ctx, p.ID, PatchActive, 100, p.Rev, "admin1")
	if err != nil {
		t.Fatal(err)
	}
	r := f.check(t, "0.2.16+28", "c1", nil)
	if !r.PatchAvailable || r.Patch.Number != 1 || r.Patch.HashSignature == nil {
		t.Fatalf("faol yamoq berilmadi: %+v", r)
	}
	// Qurilma imzoni o'zi tekshiradi — shu yerda ham tekshiramiz.
	pub, _ := ParsePublicKey(f.app.PublicKey)
	if err := VerifySignature(pub, []byte(r.Patch.Hash), *r.Patch.HashSignature); err != nil {
		t.Fatal("berilgan imzo yaroqsiz")
	}
	// Eskirgan rev — rad.
	if _, err := f.svc.SetPatchState(ctx, p.ID, PatchPaused, 0, p.Rev-1, "admin1"); !errors.Is(err, ErrVersionLock) {
		t.Fatalf("eski rev o'tib ketdi: %v", err)
	}
	p, err = f.svc.SetPatchState(ctx, p.ID, PatchRolledBack, 0, p.Rev, "admin1")
	if err != nil {
		t.Fatal(err)
	}
	r = f.check(t, "0.2.16+28", "c1", ptr(1))
	if r.PatchAvailable || len(r.RolledBackPatchNumbers) != 1 || r.RolledBackPatchNumbers[0] != 1 {
		t.Fatalf("orqaga qaytarish qurilmaga yetmadi: %+v", r)
	}
	if _, err := f.svc.SetPatchState(ctx, p.ID, PatchActive, 100, p.Rev, "admin1"); !errors.Is(err, ErrTransition) {
		t.Fatal("qaytarilgan yamoq qayta yoqildi")
	}
}

func TestNoDowngradeWhenNewerPaused(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.release(t, "1.0.0+1")
	p1 := f.patch(t, "1.0.0+1", 1)
	p2 := f.patch(t, "1.0.0+1", 2)
	if _, err := f.svc.SetPatchState(ctx, p1.ID, PatchActive, 100, p1.Rev, "a"); err != nil {
		t.Fatal(err)
	}
	p2, _ = f.svc.SetPatchState(ctx, p2.ID, PatchActive, 100, p2.Rev, "a")
	if _, err := f.svc.SetPatchState(ctx, p2.ID, PatchPaused, 0, p2.Rev, "a"); err != nil {
		t.Fatal(err)
	}
	// 2-yamoqdagi qurilmaga 1-yamoq taklif QILINMAYDI.
	if r := f.check(t, "1.0.0+1", "c", ptr(2)); r.PatchAvailable {
		t.Fatalf("qurilma eski yamoqqa tushirildi: %+v", r.Patch)
	}
	// Yamoqsiz qurilma 1-ni oladi.
	if r := f.check(t, "1.0.0+1", "c", nil); !r.PatchAvailable || r.Patch.Number != 1 {
		t.Fatalf("yamoqsiz qurilma: %+v", r)
	}
}

func TestRolloutIsStableAndProportional(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.release(t, "1.0.0+1")
	p := f.patch(t, "1.0.0+1", 1)
	p, _ = f.svc.SetPatchState(ctx, p.ID, PatchActive, 25, p.Rev, "a")
	in := 0
	got := map[string]bool{}
	for i := 0; i < 2000; i++ {
		c := fmt.Sprintf("client%d", i)
		if f.check(t, "1.0.0+1", c, nil).PatchAvailable {
			in++
			got[c] = true
		}
	}
	if in < 400 || in > 600 {
		t.Fatalf("25%% rollout: %d/2000", in)
	}
	// Foiz oshirilganda avvalgi qurilmalar chiqib ketmaydi.
	if _, err := f.svc.SetPatchState(ctx, p.ID, PatchActive, 60, p.Rev, "a"); err != nil {
		t.Fatal(err)
	}
	for c := range got {
		if !f.check(t, "1.0.0+1", c, nil).PatchAvailable {
			t.Fatalf("%s 60%% da chiqib ketdi", c)
		}
	}
}

func TestPublishRejectsForgeries(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.release(t, "1.0.0+1")

	// 1) Boshqa kalit bilan imzolangan manifest.
	other, _, _ := GenerateKey()
	ok, _ := ParsePrivateKey(other)
	m, files := f.patchManifest(t, "1.0.0+1", 1, []byte("d"))
	raw, sig, _ := SignManifest(ok, m, f.now)
	if _, err := f.svc.PublishPatch(ctx, raw, sig, files); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("begona kalit: %v", err)
	}
	// 2) To'g'ri manifest, lekin fayl almashtirilgan.
	raw, sig, _ = SignManifest(f.key, m, f.now)
	if _, err := f.svc.PublishPatch(ctx, raw, sig, Files{"patch:aarch64": []byte("X")}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("almashtirilgan fayl: %v", err)
	}
	// 3) Yamoq xeshi imzosi begona kalitniki (qurilma rad etardi — server ham).
	bad := m
	pr := bad.Patches["aarch64"]
	pr.Signature, _ = Sign(ok, []byte(pr.Hash))
	bad.Patches = map[string]PatchFileRef{"aarch64": pr}
	raw, sig, _ = SignManifest(f.key, bad, f.now)
	if _, err := f.svc.PublishPatch(ctx, raw, sig, files); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("begona yamoq imzosi: %v", err)
	}
	// 4) Eskirgan imzo (replay).
	raw, sig, _ = SignManifest(f.key, m, f.now.Add(-time.Hour))
	if _, err := f.svc.PublishPatch(ctx, raw, sig, files); !errors.Is(err, ErrStale) {
		t.Fatalf("eski so'rov: %v", err)
	}
	// 5) Manifestni imzodan keyin o'zgartirish.
	raw, sig, _ = SignManifest(f.key, m, f.now)
	raw[len(raw)-2] ^= 1
	if _, err := f.svc.PublishPatch(ctx, raw, sig, files); err == nil {
		t.Fatal("buzilgan manifest qabul qilindi")
	}
	// 6) Raqam ketma-ketligi: 1 dan boshqasi rad.
	m3, f3 := f.patchManifest(t, "1.0.0+1", 3, []byte("d3"))
	raw, sig, _ = SignManifest(f.key, m3, f.now)
	if _, err := f.svc.PublishPatch(ctx, raw, sig, f3); !errors.Is(err, ErrConflict) {
		t.Fatalf("raqam sakrashi: %v", err)
	}
	// Hech narsa saqlanmagan bo'lishi kerak.
	rel, _ := f.svc.d.Store.FindRelease(ctx, f.app.ID, "1.0.0+1", PlatformAndroid)
	if ps, _ := f.svc.ListPatches(ctx, rel.ID); len(ps) != 0 {
		t.Fatalf("soxta yamoq saqlangan: %d", len(ps))
	}
}

func TestDuplicateReleaseAndUnknownRelease(t *testing.T) {
	f := newFixture(t)
	f.release(t, "1.0.0+1")
	lib := []byte("x")
	ent := []byte("{}")
	m := PublishManifest{Kind: "release", AppID: f.app.ID, Platform: PlatformAndroid, Version: "1.0.0+1",
		Libapps: map[string]FileRef{"aarch64": FileRefOf(lib)}, Entries: ptr(FileRefOf(ent))}
	raw, sig, _ := SignManifest(f.key, m, f.now)
	if _, err := f.svc.PublishRelease(context.Background(), raw, sig, Files{"libapp:aarch64": lib, "entries": ent}); !errors.Is(err, ErrConflict) {
		t.Fatalf("takroriy reliz: %v", err)
	}
	if r := f.check(t, "9.9.9+9", "c", nil); r.PatchAvailable {
		t.Fatal("noma'lum relizga yamoq")
	}
}

func TestReportCountsOnly(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.release(t, "1.0.0+1")
	p := f.patch(t, "1.0.0+1", 1)
	for _, typ := range []string{EventDownload, EventInstall, EventInstallFailure, "__boshqa__"} {
		if err := f.svc.Report(ctx, Event{AppID: f.app.ID, Type: typ, PatchNumber: 1, Platform: PlatformAndroid,
			ReleaseVersion: "1.0.0+1", Arch: "aarch64", ClientID: "c"}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := f.svc.d.Store.GetPatch(ctx, p.ID)
	if got.Stats != (PatchStats{Downloads: 1, Installs: 1, Failures: 1}) {
		t.Fatalf("statistika: %+v", got.Stats)
	}
	// Hodisalar holatni o'zgartirmaydi (avto-pauza YO'Q — hujum yuzasi).
	if got.Status != PatchDraft {
		t.Fatal("hodisa holatni o'zgartirdi")
	}
}

func TestCreateAppRejectsWeakKey(t *testing.T) {
	f := newFixture(t)
	if _, err := f.svc.CreateApp(context.Background(), "", "X", PlatformAndroid, "bm90LWEta2V5", "a"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("yaroqsiz kalit: %v", err)
	}
}
