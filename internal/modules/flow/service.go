package flow

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Deps — tashqi bog'liqliklar.
type Deps struct {
	Store Store
	Blobs Blobs
	NewID func() string
	Now   func() time.Time
}

type Service struct{ d Deps }

func New(d Deps) (*Service, error) {
	if d.Store == nil || d.Blobs == nil || d.NewID == nil {
		return nil, errors.New("flow: Store, Blobs va NewID shart")
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{d: d}, nil
}

// ── Kirish ma'lumotlarini tekshirish ─────────────────────────────────────

var (
	reAppID    = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	reVersion  = regexp.MustCompile(`^[0-9]{1,5}\.[0-9]{1,5}\.[0-9]{1,5}\+[0-9]{1,9}$`)
	reArch     = regexp.MustCompile(`^(aarch64|arm|x86_64)$`)
	reHash     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	reChannel  = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	reClientID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// Fayl hajmi chegaralari (xotira va R2 ni himoya qiladi).
const (
	MaxLibappBytes = 64 << 20 // asosiy libapp.so
	MaxPatchBytes  = 64 << 20 // yamoq fayli
	MaxEntries     = 4 << 20  // APK fayllar ro'yxati (JSON)
	publishSkew    = 10 * time.Minute
)

// ParsePublicKey — Shorebird formatidagi (PKCS#1 DER, base64) ochiq kalit.
// Faqat RSA >= 2048 bit qabul qilinadi.
func ParsePublicKey(b64 string) (*rsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
	if err != nil {
		return nil, fmt.Errorf("%w: ochiq kalit base64 emas", ErrInvalid)
	}
	pub, err := x509.ParsePKCS1PublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("%w: ochiq kalit PKCS#1 RSA emas", ErrInvalid)
	}
	if pub.N.BitLen() < 2048 {
		return nil, fmt.Errorf("%w: kalit kamida 2048 bit bo'lsin", ErrInvalid)
	}
	return pub, nil
}

// KeyID — ochiq kalitning qisqa barmoq izi (panelda va vositada solishtirish).
func KeyID(b64 string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(b64)))
	return hex.EncodeToString(sum[:8])
}

// VerifySignature — qurilmadagi tekshiruv bilan AYNAN bir xil
// (`signing.rs`): xabar baytlari ustiga RSASSA-PKCS1-v1_5 + SHA-256.
func VerifySignature(pub *rsa.PublicKey, message []byte, sigB64 string) error {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return ErrBadSignature
	}
	sum := sha256.Sum256(message)
	if rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig) != nil {
		return ErrBadSignature
	}
	return nil
}

// ── Ilovalar (admin) ─────────────────────────────────────────────────────

// CreateApp — ilovani ulaydi. [id] bo'sh bo'lsa yangi UUID; berilsa (mavjud
// `shorebird.yaml` dagi app_id) — UUID formatida bo'lishi shart.
func (s *Service) CreateApp(ctx context.Context, id, name, platform, publicKey, by string) (*App, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if id == "" {
		id = newUUID()
	} else if !reAppID.MatchString(id) {
		return nil, fmt.Errorf("%w: app_id UUID formatida bo'lsin", ErrInvalid)
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 80 {
		return nil, fmt.Errorf("%w: nom 1..80 belgi", ErrInvalid)
	}
	if platform != PlatformAndroid {
		return nil, fmt.Errorf("%w: hozircha faqat android", ErrInvalid)
	}
	publicKey = strings.TrimSpace(publicKey)
	if _, err := ParsePublicKey(publicKey); err != nil {
		return nil, err
	}
	a := &App{ID: id, Name: name, Platform: platform, PublicKey: publicKey,
		KeyID: KeyID(publicKey), CreatedAt: s.d.Now().UTC(), CreatedBy: by}
	if err := s.d.Store.CreateApp(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

func (s *Service) ListApps(ctx context.Context) ([]*App, error) { return s.d.Store.ListApps(ctx) }

func (s *Service) ListReleases(ctx context.Context, appID string) ([]*Release, error) {
	if _, err := s.d.Store.GetApp(ctx, appID); err != nil {
		return nil, err
	}
	return s.d.Store.ListReleases(ctx, appID)
}

func (s *Service) ListPatches(ctx context.Context, releaseID string) ([]*Patch, error) {
	if _, err := s.d.Store.GetRelease(ctx, releaseID); err != nil {
		return nil, err
	}
	return s.d.Store.ListPatches(ctx, releaseID)
}

// SetPatchState — panel amali. Ruxsat etilgan o'tishlar:
//
//	draft  -> active (rollout 1..100)
//	active -> active (rollout o'zgaradi) | paused | rolled_back
//	paused -> active | rolled_back
//	rolled_back — yakuniy (qayta yoqib bo'lmaydi: yangi yamoq chiqariladi)
func (s *Service) SetPatchState(ctx context.Context, id string, status PatchStatus, rollout, rev int, by string) (*Patch, error) {
	p, err := s.d.Store.GetPatch(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Rev != rev {
		return nil, ErrVersionLock
	}
	switch status {
	case PatchActive:
		if p.Status == PatchRolledBack {
			return nil, ErrTransition
		}
		if rollout < 1 || rollout > 100 {
			return nil, fmt.Errorf("%w: foiz 1..100", ErrInvalid)
		}
		if p.ActivatedAt == nil {
			t := s.d.Now().UTC()
			p.ActivatedAt = &t
		}
		p.Rollout = rollout
	case PatchPaused:
		if p.Status != PatchActive {
			return nil, ErrTransition
		}
	case PatchRolledBack:
		if p.Status != PatchActive && p.Status != PatchPaused {
			return nil, ErrTransition
		}
	default:
		return nil, ErrTransition
	}
	p.Status = status
	p.UpdatedAt = s.d.Now().UTC()
	p.UpdatedBy = by
	if err := s.d.Store.UpdatePatch(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ── Qurilma: yamoq tekshiruvi ────────────────────────────────────────────

func (s *Service) Check(ctx context.Context, req CheckRequest) (CheckResponse, error) {
	none := CheckResponse{RolledBackPatchNumbers: []int{}}
	if !reAppID.MatchString(req.AppID) || !reArch.MatchString(req.Arch) ||
		!reClientID.MatchString(req.ClientID) || len(req.ReleaseVersion) > 128 {
		return none, ErrInvalid
	}
	ch := req.Channel
	if ch == "" {
		ch = "stable"
	}
	if !reChannel.MatchString(ch) {
		return none, ErrInvalid
	}
	rel, err := s.d.Store.FindRelease(ctx, req.AppID, req.ReleaseVersion, req.Platform)
	if errors.Is(err, ErrNotFound) {
		return none, nil // noma'lum reliz — oddiy "yamoq yo'q"
	}
	if err != nil {
		return none, err
	}
	patches, err := s.d.Store.ListPatches(ctx, rel.ID)
	if err != nil {
		return none, err
	}
	sort.Slice(patches, func(i, j int) bool { return patches[i].Number > patches[j].Number })
	var best *Patch
	var bestArt *PatchArtifact
	for _, p := range patches {
		if p.Channel != ch {
			continue
		}
		if p.Status == PatchRolledBack {
			none.RolledBackPatchNumbers = append(none.RolledBackPatchNumbers, p.Number)
			continue
		}
		if best != nil || p.Status != PatchActive {
			continue
		}
		// Qurilmani ESKIROQ yamoqqa tushirmaymiz (yangisi pauzada bo'lsa ham).
		if req.CurrentPatchNumber != nil && p.Number <= *req.CurrentPatchNumber {
			continue
		}
		if !inRollout(req.ClientID, p.ID, p.Rollout) {
			continue
		}
		for i := range p.Artifacts {
			if p.Artifacts[i].Arch == req.Arch {
				best, bestArt = p, &p.Artifacts[i]
				break
			}
		}
	}
	sort.Ints(none.RolledBackPatchNumbers)
	if best == nil {
		return none, nil
	}
	sig := bestArt.Signature
	none.PatchAvailable = true
	none.Patch = &CheckPatch{Number: best.Number, Hash: bestArt.Hash, DownloadURL: bestArt.URL, HashSignature: &sig}
	return none, nil
}

// inRollout — qurilma barqaror (har so'rovda bir xil) chelakka tushadi:
// foiz oshirilganda avval kirganlar chiqib ketmaydi.
func inRollout(clientID, patchID string, percent int) bool {
	if percent >= 100 {
		return true
	}
	if percent <= 0 {
		return false
	}
	sum := sha256.Sum256([]byte(patchID + ":" + clientID))
	return int(binary.BigEndian.Uint32(sum[:4])%100) < percent
}

// Report — qurilma hodisasi. Faqat hisoblagich; client_id SAQLANMAYDI.
func (s *Service) Report(ctx context.Context, e Event) error {
	if !reAppID.MatchString(e.AppID) || e.PatchNumber < 1 || len(e.ReleaseVersion) > 128 {
		return ErrInvalid
	}
	var kind string
	switch e.Type {
	case EventDownload:
		kind = "downloads"
	case EventInstall:
		kind = "installs"
	case EventInstallFailure, EventUpdateFailure:
		kind = "failures"
	default:
		return nil // noma'lum tur — jim e'tiborsiz (yangilagich yangi tur qo'shsa yiqilmasin)
	}
	rel, err := s.d.Store.FindRelease(ctx, e.AppID, e.ReleaseVersion, e.Platform)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	err = s.d.Store.AddStat(ctx, rel.ID, e.PatchNumber, kind)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

// ── Nashr (vosita -> server, imzolangan) ─────────────────────────────────

// PublishManifest — vosita imzolaydigan hujjat. Imzo JSON baytlari
// ustiga (`Signed.Manifest` aynan shu baytlar), fayllar esa manifestdagi
// SHA-256 orqali imzoga bog'lanadi.
type PublishManifest struct {
	Kind     string `json:"kind"` // "release" | "patch"
	AppID    string `json:"app_id"`
	Platform string `json:"platform"`
	Version  string `json:"release_version"`
	IssuedAt int64  `json:"issued_at"` // unix soniya
	// Release:
	Libapps map[string]FileRef `json:"libapps,omitempty"` // arch -> libapp.so
	Entries *FileRef           `json:"entries,omitempty"`
	// Patch:
	Number  int                     `json:"number,omitempty"`
	Channel string                  `json:"channel,omitempty"`
	Note    string                  `json:"note,omitempty"`
	Patches map[string]PatchFileRef `json:"patches,omitempty"` // arch -> yamoq
}

type FileRef struct {
	SHA256 string `json:"sha256"` // yuklangan fayl baytlarining xeshi
	Size   int64  `json:"size"`
}

type PatchFileRef struct {
	FileRef
	Hash      string `json:"hash"`      // yamoqdan keyingi libapp.so xeshi
	Signature string `json:"signature"` // hash ustiga imzo (qurilma tekshiradi)
}

// Files — yuklangan fayllar: kalit "libapp:<arch>", "patch:<arch>", "entries".
type Files map[string][]byte

// verifyPublish — imzo, vaqt, ilova va fayl xeshlarini tekshiradi.
func (s *Service) verifyPublish(ctx context.Context, raw []byte, sigB64 string) (*PublishManifest, *App, *rsa.PublicKey, error) {
	if len(raw) == 0 || len(raw) > 64<<10 {
		return nil, nil, nil, ErrInvalid
	}
	var m PublishManifest
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, nil, nil, fmt.Errorf("%w: manifest", ErrInvalid)
	}
	if !reAppID.MatchString(m.AppID) || m.Platform != PlatformAndroid || !reVersion.MatchString(m.Version) {
		return nil, nil, nil, fmt.Errorf("%w: app_id/platform/versiya", ErrInvalid)
	}
	app, err := s.d.Store.GetApp(ctx, m.AppID)
	if err != nil {
		return nil, nil, nil, err
	}
	pub, err := ParsePublicKey(app.PublicKey)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := VerifySignature(pub, raw, sigB64); err != nil {
		return nil, nil, nil, err
	}
	// Imzo to'g'ri bo'lgandan KEYIN vaqt: eski imzolangan so'rovni qayta
	// yuborish (replay) 10 daqiqadan keyin ishlamaydi; undan oldin esa
	// takroriy yaratish noyoblik cheklovida (ErrConflict) to'xtaydi.
	now := s.d.Now()
	if d := now.Sub(time.Unix(m.IssuedAt, 0)); d > publishSkew || d < -publishSkew {
		return nil, nil, nil, ErrStale
	}
	return &m, app, pub, nil
}

func checkFile(files Files, key string, ref FileRef, max int64) ([]byte, error) {
	b, ok := files[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s fayli yo'q", ErrInvalid, key)
	}
	if int64(len(b)) != ref.Size || ref.Size <= 0 || ref.Size > max {
		return nil, fmt.Errorf("%w: %s hajmi", ErrInvalid, key)
	}
	sum := sha256.Sum256(b)
	if hex.EncodeToString(sum[:]) != ref.SHA256 {
		return nil, fmt.Errorf("%w: %s xeshi manifestga mos emas", ErrInvalid, key)
	}
	return b, nil
}

func keyVersion(v string) string { return strings.ReplaceAll(v, "+", "_") }

// PublishRelease — yangi reliz (asosiy libapp.so lar). Faqat yozuv; hech
// kimga hech narsa yuborilmaydi.
func (s *Service) PublishRelease(ctx context.Context, raw []byte, sig string, files Files) (*Release, error) {
	m, app, _, err := s.verifyPublish(ctx, raw, sig)
	if err != nil {
		return nil, err
	}
	if m.Kind != "release" || len(m.Libapps) == 0 || len(m.Libapps) > 4 || m.Entries == nil {
		return nil, fmt.Errorf("%w: reliz manifesti", ErrInvalid)
	}
	if len(files) != len(m.Libapps)+1 {
		return nil, fmt.Errorf("%w: ortiqcha yoki yetishmayotgan fayl", ErrInvalid)
	}
	if _, err := s.d.Store.FindRelease(ctx, app.ID, m.Version, m.Platform); err == nil {
		return nil, ErrConflict
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	base := fmt.Sprintf("flow/%s/%s/%s", app.ID, keyVersion(m.Version), m.Platform)
	entries, err := checkFile(files, "entries", *m.Entries, MaxEntries)
	if err != nil {
		return nil, err
	}
	rel := &Release{ID: s.d.NewID(), AppID: app.ID, Version: m.Version, Platform: m.Platform, CreatedAt: s.d.Now().UTC()}
	archs := sortedKeys(m.Libapps)
	for _, arch := range archs {
		ref := m.Libapps[arch]
		if !reArch.MatchString(arch) {
			return nil, fmt.Errorf("%w: arch", ErrInvalid)
		}
		b, err := checkFile(files, "libapp:"+arch, ref, MaxLibappBytes)
		if err != nil {
			return nil, err
		}
		url, err := s.d.Blobs.Put(ctx, fmt.Sprintf("%s/release/%s/libapp.so", base, arch), b, "application/octet-stream")
		if err != nil {
			return nil, err
		}
		rel.Artifacts = append(rel.Artifacts, ReleaseArtifact{Arch: arch, Hash: ref.SHA256, Size: ref.Size, URL: url})
	}
	if rel.EntriesURL, err = s.d.Blobs.Put(ctx, base+"/release/entries.json", entries, "application/json"); err != nil {
		return nil, err
	}
	if err := s.d.Store.CreateRelease(ctx, rel); err != nil {
		return nil, err
	}
	return rel, nil
}

// PublishPatch — yangi yamoq QORALAMA holatida. Har yamoq imzosi qurilma
// bilan bir xil tekshiriladi: imzosiz yoki boshqa kalitli yamoq umuman
// saqlanmaydi.
func (s *Service) PublishPatch(ctx context.Context, raw []byte, sig string, files Files) (*Patch, error) {
	m, app, pub, err := s.verifyPublish(ctx, raw, sig)
	if err != nil {
		return nil, err
	}
	if m.Kind != "patch" || m.Number < 1 || m.Number > 100000 || len(m.Patches) == 0 || len(m.Patches) > 4 {
		return nil, fmt.Errorf("%w: yamoq manifesti", ErrInvalid)
	}
	if len(files) != len(m.Patches) {
		return nil, fmt.Errorf("%w: ortiqcha yoki yetishmayotgan fayl", ErrInvalid)
	}
	ch := m.Channel
	if ch == "" {
		ch = "stable"
	}
	if !reChannel.MatchString(ch) || len(m.Note) > 500 {
		return nil, fmt.Errorf("%w: kanal/izoh", ErrInvalid)
	}
	rel, err := s.d.Store.FindRelease(ctx, app.ID, m.Version, m.Platform)
	if err != nil {
		return nil, err
	}
	existing, err := s.d.Store.ListPatches(ctx, rel.ID)
	if err != nil {
		return nil, err
	}
	maxN := 0
	for _, p := range existing {
		if p.Number > maxN {
			maxN = p.Number
		}
	}
	if m.Number != maxN+1 {
		return nil, fmt.Errorf("%w: keyingi yamoq raqami %d bo'lishi kerak", ErrConflict, maxN+1)
	}
	relArchs := map[string]bool{}
	for _, a := range rel.Artifacts {
		relArchs[a.Arch] = true
	}
	base := fmt.Sprintf("flow/%s/%s/%s/patches/%d", app.ID, keyVersion(m.Version), m.Platform, m.Number)
	p := &Patch{ID: s.d.NewID(), ReleaseID: rel.ID, AppID: app.ID, Version: m.Version, Number: m.Number,
		Channel: ch, Status: PatchDraft, Note: strings.TrimSpace(m.Note), CreatedAt: s.d.Now().UTC(), UpdatedAt: s.d.Now().UTC()}
	for _, arch := range sortedKeys(m.Patches) {
		ref := m.Patches[arch]
		if !relArchs[arch] {
			return nil, fmt.Errorf("%w: relizda %s arxitekturasi yo'q", ErrInvalid, arch)
		}
		if !reHash.MatchString(ref.Hash) {
			return nil, fmt.Errorf("%w: hash", ErrInvalid)
		}
		if err := VerifySignature(pub, []byte(ref.Hash), ref.Signature); err != nil {
			return nil, err
		}
		b, err := checkFile(files, "patch:"+arch, ref.FileRef, MaxPatchBytes)
		if err != nil {
			return nil, err
		}
		url, err := s.d.Blobs.Put(ctx, fmt.Sprintf("%s/%s.patch", base, arch), b, "application/octet-stream")
		if err != nil {
			return nil, err
		}
		p.Artifacts = append(p.Artifacts, PatchArtifact{Arch: arch, Hash: ref.Hash, Signature: ref.Signature, Size: ref.Size, URL: url})
	}
	if err := s.d.Store.CreatePatch(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// newUUID — app_id Shorebird formatida (UUID v4, crypto/rand).
func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand yiqilsa — tizim yaroqsiz
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	x := hex.EncodeToString(b[:])
	return x[0:8] + "-" + x[8:12] + "-" + x[12:16] + "-" + x[16:20] + "-" + x[20:32]
}
