package e2e

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"testing"
	"time"

	"chustapp/internal/modules/flow"
)

// OnDexFlow (Plane ONDEXECOSY-472): admin ilovani ulaydi -> vosita
// imzolangan reliz va yamoq nashr qiladi (QORALAMA) -> qurilma ko'rmaydi ->
// admin panelda faollashtiradi -> qurilma imzoli yamoqni oladi -> hodisa
// hisoblanadi -> orqaga qaytarish qurilmaga yetadi. Soxta nashr rad etiladi.
func TestFlowOTA(t *testing.T) {
	w := NewWorld(t, "OnDexFlow: o'z OTA yangilanishi")

	privPEM, pub, err := flow.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	key, _ := flow.ParsePrivateKey(privPEM)

	// ── Admin endpointlari himoyalangan
	w.Do(t, nil, "GET", "/admin/flow/apps", nil).Expect(t, 401)
	w.Do(t, w.Customer, "GET", "/admin/flow/apps", nil).Expect(t, 403)
	w.Do(t, w.Admin, "GET", "/admin/flow/apps", nil).Expect(t, 200)

	// ── Ilovani ulash: sababsiz 428, sabab bilan 201
	body := map[string]any{"name": "OnDex", "platform": "android", "public_key": pub}
	w.Do(t, w.Admin, "POST", "/admin/flow/apps", body).Expect(t, 428)
	var app flow.App
	w.Do(t, w.Admin.WithReason("e2e: ilovani ulash"), "POST", "/admin/flow/apps", body).Expect(t, 201).JSON(t, &app)
	if app.ID == "" || app.KeyID == "" {
		t.Fatalf("ilova: %+v", app)
	}

	const ver = "0.2.16+28"
	multi := func(path string, m flow.PublishManifest, k interface{}, files map[string][]byte) *Resp {
		t.Helper()
		raw, sig, err := flow.SignManifest(key, m, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		if kk, ok := k.(string); ok && kk == "begona" {
			other, _, _ := flow.GenerateKey()
			ok2, _ := flow.ParsePrivateKey(other)
			sig, _ = flow.Sign(ok2, raw)
		}
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("manifest", string(raw))
		_ = mw.WriteField("signature", sig)
		for name, b := range files {
			fw, _ := mw.CreateFormFile(name, "f.bin")
			_, _ = fw.Write(b)
		}
		_ = mw.Close()
		w.ctype = mw.FormDataContentType()
		defer func() { w.ctype = "" }()
		return w.Do(t, nil, "POST", path, buf.Bytes())
	}

	// ── Reliz nashri (imzolangan)
	lib := bytes.Repeat([]byte("L"), 4096)
	ent := []byte(`{"lib/arm64-v8a/libflutter.so":"aa"}`)
	relM := flow.PublishManifest{Kind: "release", AppID: app.ID, Platform: "android", Version: ver,
		Libapps: map[string]flow.FileRef{"aarch64": flow.FileRefOf(lib)}, Entries: ptrRef(flow.FileRefOf(ent))}
	multi("/flow/publish/releases", relM, nil, map[string][]byte{"libapp:aarch64": lib, "entries": ent}).Expect(t, 201)
	multi("/flow/publish/releases", relM, nil, map[string][]byte{"libapp:aarch64": lib, "entries": ent}).Expect(t, 409)

	// ── Yamoq nashri — QORALAMA, qurilma ko'rmaydi
	newLib := bytes.Repeat([]byte("N"), 4096)
	h := fmt.Sprintf("%x", sha256.Sum256(newLib))
	hsig, _ := flow.Sign(key, []byte(h))
	diff := []byte("bidiff-zstd")
	patchM := flow.PublishManifest{Kind: "patch", AppID: app.ID, Platform: "android", Version: ver, Number: 1, Note: "e2e",
		Patches: map[string]flow.PatchFileRef{"aarch64": {FileRef: flow.FileRefOf(diff), Hash: h, Signature: hsig}}}
	multi("/flow/publish/patches", patchM, "begona", map[string][]byte{"patch:aarch64": diff}).Expect(t, 401)
	multi("/flow/publish/patches", patchM, nil, map[string][]byte{"patch:aarch64": []byte("almashtirilgan")}).Expect(t, 400)
	var patch flow.Patch
	multi("/flow/publish/patches", patchM, nil, map[string][]byte{"patch:aarch64": diff}).Expect(t, 201).JSON(t, &patch)
	if patch.Status != flow.PatchDraft {
		t.Fatalf("yamoq holati: %s", patch.Status)
	}

	check := func() flow.CheckResponse {
		t.Helper()
		var r flow.CheckResponse
		w.Do(t, nil, "POST", "/flow/api/v1/patches/check", map[string]any{
			"app_id": app.ID, "channel": "stable", "release_version": ver, "platform": "android",
			"arch": "aarch64", "client_id": "qurilma-1",
		}).Expect(t, 200).JSON(t, &r)
		return r
	}
	if r := check(); r.PatchAvailable {
		t.Fatal("qoralama qurilmaga berildi")
	}
	w.Do(t, nil, "POST", "/flow/api/v1/patches/check", map[string]any{"app_id": "x"}).Expect(t, 400)

	// ── Admin panel: reliz/yamoq ro'yxati va faollashtirish
	var rels struct {
		Releases []flow.Release `json:"releases"`
	}
	w.Do(t, w.Admin, "GET", "/admin/flow/apps/"+app.ID+"/releases", nil).Expect(t, 200).JSON(t, &rels)
	if len(rels.Releases) != 1 {
		t.Fatalf("relizlar: %d", len(rels.Releases))
	}
	statePath := "/admin/flow/patches/" + patch.ID + "/state"
	w.Do(t, w.Admin, "POST", statePath, map[string]any{"status": "active", "rollout_percent": 100, "rev": patch.Rev}).Expect(t, 428)
	w.Do(t, w.Admin.WithReason("e2e: eski rev"), "POST", statePath, map[string]any{"status": "active", "rollout_percent": 100, "rev": patch.Rev + 5}).Expect(t, 409)
	w.Do(t, w.Admin.WithReason("e2e: chiqarish"), "POST", statePath, map[string]any{"status": "active", "rollout_percent": 100, "rev": patch.Rev}).Expect(t, 200).JSON(t, &patch)

	// ── Qurilma imzoli yamoqni oladi, hisobot yuboradi
	r := check()
	if !r.PatchAvailable || r.Patch.Number != 1 || r.Patch.Hash != h || r.Patch.HashSignature == nil {
		t.Fatalf("yamoq berilmadi: %+v", r)
	}
	pk, _ := flow.ParsePublicKey(pub)
	if err := flow.VerifySignature(pk, []byte(r.Patch.Hash), *r.Patch.HashSignature); err != nil {
		t.Fatal("qurilmaga yaroqsiz imzo berildi")
	}
	w.Do(t, nil, "POST", "/flow/api/v1/patches/events", map[string]any{"event": map[string]any{
		"app_id": app.ID, "arch": "aarch64", "client_id": "qurilma-1", "type": "__patch_install__",
		"patch_number": 1, "platform": "android", "release_version": ver, "timestamp": time.Now().Unix(),
	}}).Expect(t, 201)
	var ps struct {
		Patches []flow.Patch `json:"patches"`
	}
	w.Do(t, w.Admin, "GET", "/admin/flow/releases/"+rels.Releases[0].ID+"/patches", nil).Expect(t, 200).JSON(t, &ps)
	if len(ps.Patches) != 1 || ps.Patches[0].Stats.Installs != 1 {
		t.Fatalf("statistika: %+v", ps.Patches)
	}

	// ── Orqaga qaytarish qurilmaga yetadi
	w.Do(t, w.Admin.WithReason("e2e: qaytarish"), "POST", statePath, map[string]any{"status": "rolled_back", "rev": patch.Rev}).Expect(t, 200)
	r = check()
	if r.PatchAvailable || len(r.RolledBackPatchNumbers) != 1 {
		t.Fatalf("qaytarish: %+v", r)
	}
	raw, _ := json.Marshal(r)
	if !bytes.Contains(raw, []byte(`"rolled_back_patch_numbers":[1]`)) {
		t.Fatalf("javob shakli: %s", raw)
	}
}

func ptrRef(r flow.FileRef) *flow.FileRef { return &r }
