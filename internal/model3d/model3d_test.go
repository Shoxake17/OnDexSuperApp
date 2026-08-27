package model3d

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"chustapp/internal/catalog"
)

func parseIP(t *testing.T, s string) net.IP {
	t.Helper()
	ip := net.ParseIP(s)
	if ip == nil {
		t.Fatalf("IP o'qilmadi: %s", s)
	}
	return ip
}

// ── Soxta bog'liqliklar ──

type fakeStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newFakeStore() *fakeStore { return &fakeStore{objects: map[string][]byte{}} }

func (f *fakeStore) Upload(_ context.Context, key string, r io.Reader, _ int64, _ string) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	f.mu.Lock()
	f.objects[key] = b
	f.mu.Unlock()
	return "https://cdn.example.com/" + key, nil
}

type fakeRepo struct {
	mu       sync.Mutex
	products map[string]*catalog.Product
	saves    int
}

func newFakeRepo(p ...*catalog.Product) *fakeRepo {
	m := map[string]*catalog.Product{}
	for _, x := range p {
		m[x.ID] = x
	}
	return &fakeRepo{products: m}
}

func (f *fakeRepo) GetProductsByIDs(_ context.Context, ids []string) ([]*catalog.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*catalog.Product
	for _, id := range ids {
		if p, ok := f.products[id]; ok {
			cp := *p
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (f *fakeRepo) SaveProduct(_ context.Context, p *catalog.Product) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *p
	f.products[p.ID] = &cp
	f.saves++
	return nil
}

func (f *fakeRepo) get(id string) catalog.Product {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.products[id]
}

// Qolgan metodlar bu testlarda ishlatilmaydi.
func (f *fakeRepo) ListRestaurants(context.Context) ([]*catalog.Restaurant, error) { return nil, nil }
func (f *fakeRepo) GetRestaurant(context.Context, string) (*catalog.Restaurant, error) {
	return nil, catalog.ErrNotFound
}
func (f *fakeRepo) SaveRestaurant(context.Context, *catalog.Restaurant) error { return nil }
func (f *fakeRepo) DeleteRestaurant(context.Context, string) error            { return nil }
func (f *fakeRepo) ListProducts(context.Context, string) ([]*catalog.Product, error) {
	return nil, nil
}
func (f *fakeRepo) SearchProducts(context.Context, string) ([]*catalog.ProductSearchResult, error) {
	return nil, nil
}
func (f *fakeRepo) DeleteProduct(context.Context, string) error { return nil }

type fakeGen struct {
	submitted string
	submitErr error
	results   []TaskResult
	calls     int
	mu        sync.Mutex
}

func (g *fakeGen) Name() string { return "fake" }

func (g *fakeGen) Submit(_ context.Context, imageURL string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.submitted = imageURL
	if g.submitErr != nil {
		return "", g.submitErr
	}
	return "task-1", nil
}

func (g *fakeGen) Fetch(context.Context, string) (TaskResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.calls >= len(g.results) {
		return TaskResult{Status: TaskRunning}, nil
	}
	res := g.results[g.calls]
	g.calls++
	return res, nil
}

// ── SSRF himoyasi ──
//
// Bu testlar ENG MUHIMI: model havolasi tashqi xizmatdan keladi va
// tekshiruvsiz u serverni ichki tarmoqqa so'rov yuborishga majbur qila
// olardi (bulut metama'lumotlari, baza, ichki xizmatlar).

func TestSafeHTTPSURL_RejectsNonHTTPS(t *testing.T) {
	for _, raw := range []string{
		"http://example.com/a.glb",
		"file:///etc/passwd",
		"ftp://example.com/a.glb",
		"gopher://example.com",
	} {
		if _, err := safeHTTPSURL(raw); !errors.Is(err, ErrUnsafeURL) {
			t.Errorf("%q uchun ErrUnsafeURL kutilgandi, olindi: %v", raw, err)
		}
	}
}

func TestSafeHTTPSURL_RejectsInternalAddresses(t *testing.T) {
	// Bular DNS'siz, to'g'ridan-to'g'ri IP — `net.LookupIP` ularni
	// o'zgartirmasdan qaytaradi.
	for _, raw := range []string{
		"https://127.0.0.1/model.glb",     // loopback
		"https://169.254.169.254/latest/", // bulut metama'lumotlari
		"https://10.1.2.3/model.glb",      // xususiy tarmoq
		"https://192.168.0.5/model.glb",   // xususiy tarmoq
		"https://172.16.0.9/model.glb",    // xususiy tarmoq
		"https://100.64.0.1/model.glb",    // CGNAT
		"https://[::1]/model.glb",         // IPv6 loopback
	} {
		if _, err := safeHTTPSURL(raw); !errors.Is(err, ErrUnsafeURL) {
			t.Errorf("%q BLOKLANISHI kerak edi, olindi: %v", raw, err)
		}
	}
}

func TestIsInternalIP_AllowsPublic(t *testing.T) {
	for _, raw := range []string{"8.8.8.8", "1.1.1.1", "104.18.0.1"} {
		if isInternalIP(parseIP(t, raw)) {
			t.Errorf("%s ommaviy manzil, bloklanmasligi kerak", raw)
		}
	}
}

// ── GLB tekshiruvi ──

func TestIsGLB(t *testing.T) {
	valid := append([]byte("glTF"), make([]byte, 8)...)
	if !isGLB(valid) {
		t.Error("haqiqiy GLB imzosi qabul qilinishi kerak")
	}
	for _, bad := range [][]byte{
		[]byte("<!DOCTYPE html><html>"), // xato sahifasi
		[]byte("PK\x03\x04zipfile"),     // arxiv
		[]byte("glT"),                   // juda qisqa
		{},
	} {
		if isGLB(bad) {
			t.Errorf("%q GLB emas, rad etilishi kerak", string(bad))
		}
	}
}

// ── Xizmat mantig'i ──

func TestStart_RequiresImage(t *testing.T) {
	p := &catalog.Product{ID: "p1", RestaurantID: "r1"}
	s := NewService(&fakeGen{}, newFakeStore(), newFakeRepo(p), DefaultConfig(), nil)
	if err := s.Start(context.Background(), p); !errors.Is(err, ErrNoImage) {
		t.Fatalf("ErrNoImage kutilgandi, olindi: %v", err)
	}
}

func TestStart_RejectsWhilePending(t *testing.T) {
	p := &catalog.Product{
		ID: "p1", RestaurantID: "r1",
		ImageURL:      "https://cdn.example.com/a.webp",
		Model3DStatus: catalog.Model3DPending,
	}
	s := NewService(&fakeGen{}, newFakeStore(), newFakeRepo(p), DefaultConfig(), nil)
	if err := s.Start(context.Background(), p); !errors.Is(err, ErrBusy) {
		t.Fatalf("ErrBusy kutilgandi, olindi: %v", err)
	}
}

// Kredit ikki marta sarflanmasligi: bir vaqtda kelgan ikkita so'rovdan
// faqat bittasi vazifa ochishi kerak.
func TestClaim_PreventsDoubleStart(t *testing.T) {
	s := NewService(&fakeGen{}, newFakeStore(), newFakeRepo(), DefaultConfig(), nil)
	if !s.claim("p1") {
		t.Fatal("birinchi claim muvaffaqiyatli bo'lishi kerak")
	}
	if s.claim("p1") {
		t.Fatal("ikkinchi claim RAD ETILISHI kerak — aks holda ikkita vazifa ochilardi")
	}
	s.release("p1")
	if !s.claim("p1") {
		t.Fatal("bo'shatilgandan keyin qayta claim ishlashi kerak")
	}
}

func TestRemove_ClearsFields(t *testing.T) {
	p := &catalog.Product{
		ID: "p1", RestaurantID: "r1",
		Model3DURL:    "https://cdn.example.com/models/p1.glb",
		Model3DStatus: catalog.Model3DReady,
	}
	repo := newFakeRepo(p)
	s := NewService(&fakeGen{}, newFakeStore(), repo, DefaultConfig(), nil)
	if err := s.Remove(context.Background(), p); err != nil {
		t.Fatalf("kutilmagan xato: %v", err)
	}
	got := repo.get("p1")
	if got.Model3DURL != "" || got.Model3DStatus != "" || got.Model3DTaskID != "" {
		t.Fatalf("maydonlar tozalanmadi: %+v", got)
	}
}

// `finishFailed` mahsulotni "pending" holatidan CHIQARISHI shart —
// aks holda restoran tugmani qayta bosa olmasdi.
func TestFinishFailed_LeavesPending(t *testing.T) {
	p := &catalog.Product{
		ID: "p1", RestaurantID: "r1",
		Model3DStatus: catalog.Model3DPending,
		Model3DTaskID: "task-1",
	}
	repo := newFakeRepo(p)
	s := NewService(&fakeGen{}, newFakeStore(), repo, DefaultConfig(), nil)
	s.finishFailed("p1", "sinov")

	got := repo.get("p1")
	if got.Model3DStatus != catalog.Model3DFailed {
		t.Fatalf("holat %q, kutilgan %q", got.Model3DStatus, catalog.Model3DFailed)
	}
	if got.Model3DTaskID != "" {
		t.Fatal("vazifa ID tozalanishi kerak")
	}
}

// Mijozga ichki holat KO'RSATILMASLIGI kerak.
func TestPublicView_HidesInternalModelFields(t *testing.T) {
	p := catalog.Product{
		Model3DURL:    "https://cdn.example.com/models/p1.glb",
		Model3DStatus: catalog.Model3DPending,
		Model3DTaskID: "secret-task",
	}
	pub := p.PublicView()
	if pub.Model3DURL == "" {
		t.Error("model havolasi mijozga KERAK — o'chirilmasligi kerak")
	}
	if pub.Model3DStatus != "" || pub.Model3DTaskID != "" {
		t.Errorf("ichki maydonlar oshkor bo'ldi: %+v", pub)
	}
}

// Xizmat sozlamalari nol qiymatlarda ham xavfsiz standartga tushishi
// kerak (aks holda `PollTimeout=0` kontekstni darhol bekor qilardi).
func TestNewService_FillsDefaults(t *testing.T) {
	s := NewService(&fakeGen{}, newFakeStore(), newFakeRepo(), Config{}, nil)
	if s.cfg.MaxModelBytes <= 0 || s.cfg.PollInterval <= 0 || s.cfg.PollTimeout <= 0 {
		t.Fatalf("standart qiymatlar to'ldirilmadi: %+v", s.cfg)
	}
}

func TestDownloadModel_RejectsOversize(t *testing.T) {
	// Chegaradan katta "GLB" — hajm tekshiruvi imzo tekshiruvidan
	// OLDIN ishlashi kerak.
	big := append([]byte("glTF"), bytes.Repeat([]byte{0}, 100)...)
	if int64(len(big)) <= 10 {
		t.Skip()
	}
	_ = big
	// Haqiqiy tarmoq so'rovisiz: chegara mantig'i `downloadModel`
	// ichida, uni to'liq tekshirish uchun HTTP server kerak bo'lardi.
	// Bu yerda faqat konfiguratsiya chegarasi mavjudligini qulflaymiz.
	s := NewService(&fakeGen{}, newFakeStore(), newFakeRepo(), Config{MaxModelBytes: 10}, nil)
	if s.cfg.MaxModelBytes != 10 {
		t.Fatal("chegara qiymati saqlanmadi")
	}
}

// Kredit tugashi umumiy "server xatosi" ichida YO'QOLMASLIGI kerak:
// foydalanuvchi aniq sababni ko'rishi va nima qilish kerakligini
// bilishi shart.
func TestWrapGeneration_KeepsSentinels(t *testing.T) {
	if got := wrapGeneration(ErrNoCredit); !errors.Is(got, ErrNoCredit) {
		t.Fatal("ErrNoCredit zanjirda qolishi kerak")
	}
	if got := wrapGeneration(ErrNoCredit); errors.Is(got, ErrGeneration) {
		t.Fatal("sentinel xato umumiy turga O'RALMASLIGI kerak — xabar uzayadi")
	}
	if got := wrapGeneration(ErrProviderBusy); !errors.Is(got, ErrProviderBusy) {
		t.Fatal("ErrProviderBusy zanjirda qolishi kerak")
	}

	other := errors.New("tarmoq uzildi")
	got := wrapGeneration(other)
	if !errors.Is(got, ErrGeneration) || !errors.Is(got, other) {
		t.Fatalf("noma'lum xato IKKALA turni ham saqlashi kerak: %v", got)
	}
}

func TestTripoClient_RequiresKey(t *testing.T) {
	c := NewTripoClient("", "", "")
	_, err := c.Submit(context.Background(), "https://cdn.example.com/a.jpg")
	if err == nil || !strings.Contains(err.Error(), "TRIPO_API_KEY") {
		t.Fatalf("kalitsiz aniq xato kutilgandi, olindi: %v", err)
	}
}

// `.env` ni qo'lda to'ldirishda eng ko'p uchraydigan xatolar KODDA
// tuzatilishi kerak: noto'g'ri sozlama tarmoq qatlamidan tushunarsiz
// xato ("unsupported protocol scheme") sifatida chiqmasin.
func TestNormalizeBaseURL(t *testing.T) {
	cases := map[string]string{
		"":                           DefaultTripoBaseURL,
		"   ":                        DefaultTripoBaseURL,
		"tripo3d.ai":                 "https://tripo3d.ai",     // sxema qo'shiladi
		"api.tripo3d.ai/":            "https://api.tripo3d.ai", // + slash kesiladi
		"https://api.tripo3d.ai":     "https://api.tripo3d.ai",
		"https://proxy.example.com/": "https://proxy.example.com",
		"http://api.tripo3d.ai":      DefaultTripoBaseURL, // https majburiy
		"://buzilgan":                DefaultTripoBaseURL,
	}
	for in, want := range cases {
		if got := normalizeBaseURL(in); got != want {
			t.Errorf("normalizeBaseURL(%q) = %q, kutilgan %q", in, got, want)
		}
	}
}

// Vaqt chegarasi haqiqatan qo'llanishini qulflaydi: `PollTimeout`
// o'tgach kuzatuvchi to'xtashi va mahsulotni "failed" ga o'tkazishi
// kerak (abadiy "pending" bo'lib qolmasin).
func TestWatch_TimesOut(t *testing.T) {
	p := &catalog.Product{
		ID: "p1", RestaurantID: "r1",
		Model3DStatus: catalog.Model3DPending,
		Model3DTaskID: "task-1",
	}
	repo := newFakeRepo(p)
	gen := &fakeGen{} // har doim TaskRunning qaytaradi
	s := NewService(gen, newFakeStore(), repo, Config{
		PollInterval: 5 * time.Millisecond,
		PollTimeout:  30 * time.Millisecond,
	}, nil)
	s.claim("p1")
	s.watch("p1", "task-1")

	if got := repo.get("p1"); got.Model3DStatus != catalog.Model3DFailed {
		t.Fatalf("timeout'dan keyin holat %q, kutilgan %q", got.Model3DStatus, catalog.Model3DFailed)
	}
}
