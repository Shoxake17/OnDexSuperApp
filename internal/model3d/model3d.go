// Package model3d — taom rasmidan 3D model (GLB) generatsiyasi.
//
// ┌─ QATLAMLAR ───────────────────────────────────────────────────────┐
// `Generator` — TASHQI provayder bilan gaplashadigan yagona interfeys
// (hozircha Tripo, `tripo.go`). `Service` esa provayderdan MUSTAQIL
// orkestratsiya: kim so'ray oladi, holat qayerda saqlanadi, natija
// qanday tekshiriladi va qayerga yuklanadi.
//
// Provayder almashsa FAQAT `tripo.go` qayta yoziladi — `Service`,
// endpointlar, panel va mijoz ilovasi tegilmaydi.
//
// Saqlash uchun ALOHIDA kod YOZILMAGAN: `images.Store` interfeysi
// allaqachon umumiy (`Upload(ctx, key, r, size, contentType)`) va R2/
// lokal disk implementatsiyalari tayyor. Rasm va 3D model bitta yo'ldan
// yuriydi.
// └───────────────────────────────────────────────────────────────────┘
package model3d

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/images"
	"chustapp/internal/safego"
)

// Xatolar — chaqiruvchi (HTTP qatlami) ularni to'g'ri status kodga
// o'giradi, matnni o'zi yozib olmaydi.
var (
	ErrNoImage    = errors.New("taomda rasm yo'q — avval rasm yuklang")
	ErrBusy       = errors.New("bu taom uchun model allaqachon tayyorlanmoqda")
	ErrNotFound   = errors.New("taom topilmadi")
	ErrRejected   = errors.New("yuklab olingan fayl haqiqiy GLB modeli emas")
	ErrTooLarge   = errors.New("model fayli juda katta")
	ErrUnsafeURL  = errors.New("provayder qaytargan havola xavfsiz emas")
	ErrGeneration = errors.New("3D model yaratilmadi")

	// ErrNoCredit — provayder hisobida mablag' qolmagan.
	//
	// ┌─ NEGA ALOHIDA XATO ───────────────────────────────────────────┐
	// Bu SERVER xatosi EMAS — sozlama/hisob holati. Umumiy 5xx bo'lib
	// ketsa, foydalanuvchi "server xatosi, keyinroq urinib ko'ring"
	// degan yozuvni ko'rardi va sababni HECH QACHON bilmasdi: na
	// kutish yordam beradi, na qayta urinish. Restoran (yoki admin)
	// aniq nima qilish kerakligini bilishi kerak.
	// └───────────────────────────────────────────────────────────────┘
	ErrNoCredit = errors.New("3D generatsiya hisobida kredit tugagan")

	// ErrProviderBusy — provayder tezlik chegarasiga urildi.
	ErrProviderBusy = errors.New("3D xizmati hozir band — birozdan keyin urining")
)

// TaskStatus — provayderdagi vazifa holati (provayderdan mustaqil).
type TaskStatus string

const (
	TaskRunning TaskStatus = "running"
	TaskDone    TaskStatus = "done"
	TaskFailed  TaskStatus = "failed"
)

// TaskResult — `Generator.Fetch` natijasi.
type TaskResult struct {
	Status TaskStatus
	// ModelURL — FAQAT `TaskDone` da to'ladi. Bu provayder tomonidagi
	// vaqtinchalik havola; biz uni yuklab olib O'ZIMIZNING R2 ga
	// ko'chiramiz (havola muddati tugab, model yo'qolib qolmasin).
	ModelURL string
	// Err — `TaskFailed` sababi (foydalanuvchiga ko'rsatish uchun emas,
	// jurnalga yozish uchun).
	Err string
}

// Generator — tashqi 3D generatsiya xizmati.
type Generator interface {
	// Submit — rasm havolasidan yangi vazifa ochadi va vazifa ID sini
	// qaytaradi. `imageURL` — BIZNING ochiq R2 havolamiz.
	Submit(ctx context.Context, imageURL string) (taskID string, err error)
	// Fetch — vazifa holatini so'raydi.
	Fetch(ctx context.Context, taskID string) (TaskResult, error)
	// Name — jurnallar uchun provayder nomi.
	Name() string
}

// Config — xizmat sozlamalari (barchasi `.env` dan, standart qiymatlar
// bilan).
type Config struct {
	// MaxModelBytes — yuklab olinadigan GLB uchun qattiq chegara.
	// Chegarasiz provayder (yoki buzilgan javob) serverni xotira/disk
	// bo'yicha cho'ktirishi mumkin edi.
	MaxModelBytes int64
	// PollInterval / PollTimeout — vazifa holatini so'rash oralig'i va
	// umumiy kutish chegarasi. Chegara SHART: aks holda "abadiy
	// pending" holatidagi vazifa gorutinani cheksiz ushlab turardi.
	PollInterval time.Duration
	PollTimeout  time.Duration
}

// DefaultConfig — ishlab chiqarish uchun mo'ljallangan standartlar.
func DefaultConfig() Config {
	return Config{
		MaxModelBytes: 25 << 20, // 25MB — teksturali GLB uchun mo'l-ko'l
		PollInterval:  5 * time.Second,
		PollTimeout:   10 * time.Minute,
	}
}

// Service — 3D generatsiya orkestratsiyasi.
type Service struct {
	gen   Generator
	store images.Store
	repo  catalog.Repository
	cfg   Config

	// onUpdate — mahsulot holati o'zgarganda chaqiriladi (WebSocket
	// orqali panelga jonli xabar berish uchun). `nil` bo'lishi mumkin:
	// paket `notify`/`httpapi` ga BOG'LANMAYDI, shunchaki callback
	// chaqiradi — qatlamlar bir-biriga kirib ketmasin.
	onUpdate func(p *catalog.Product)

	// active — hozir ishlanayotgan mahsulotlar. Baza holatidan TASHQARI
	// qo'shimcha himoya: ikki so'rov bir vaqtda kelsa, ikkinchisi
	// baza yozuvini ko'rishga ulgurmasdan ikkinchi vazifa ochib
	// yuborardi (ya'ni kredit ikki marta sarflanardi).
	mu     sync.Mutex
	active map[string]bool
}

func NewService(gen Generator, store images.Store, repo catalog.Repository, cfg Config, onUpdate func(*catalog.Product)) *Service {
	if cfg.MaxModelBytes <= 0 {
		cfg.MaxModelBytes = DefaultConfig().MaxModelBytes
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = DefaultConfig().PollInterval
	}
	if cfg.PollTimeout <= 0 {
		cfg.PollTimeout = DefaultConfig().PollTimeout
	}
	return &Service{
		gen: gen, store: store, repo: repo, cfg: cfg,
		onUpdate: onUpdate,
		active:   map[string]bool{},
	}
}

// Start — mahsulot uchun 3D generatsiyani boshlaydi.
//
// Egalik tekshiruvi bu yerda EMAS (HTTP qatlamida) — bu funksiya
// mahsulot obyektini tayyor holda oladi.
//
// Chaqiruv TEZ qaytadi: provayderga so'rov yuboriladi, vazifa ID
// saqlanadi va kuzatish fon vazifasiga o'tadi. Panel javobni kutib
// qotib turmaydi.
func (s *Service) Start(ctx context.Context, p *catalog.Product) error {
	if strings.TrimSpace(p.ImageURL) == "" {
		return ErrNoImage
	}
	if p.Model3DStatus == catalog.Model3DPending {
		return ErrBusy
	}
	if !s.claim(p.ID) {
		return ErrBusy
	}

	// Tashqi xizmat WebP havolasini qabul qilmaydi — JPEG nusxa
	// tayyorlanadi (`source.go`).
	srcURL, err := s.sourceImageURL(ctx, p)
	if err != nil {
		s.release(p.ID)
		return wrapGeneration(err)
	}

	taskID, err := s.gen.Submit(ctx, srcURL)
	if err != nil {
		s.release(p.ID)
		return wrapGeneration(err)
	}

	p.Model3DStatus = catalog.Model3DPending
	p.Model3DTaskID = taskID
	if err := s.repo.SaveProduct(ctx, p); err != nil {
		// Vazifa provayderda ochilgan, lekin biz uni eslab qola
		// olmadik — kuzatib bo'lmaydi. Qulfni bo'shatamiz va xato
		// qaytaramiz; restoran qayta urinishi mumkin.
		s.release(p.ID)
		return err
	}
	s.notify(p)

	// safego — recover bilan (bug.md 44-band): bu goroutine
	// HTTP so'rovidan tashqarida ishlaydi, ya'ni undagi panic butun
	// jarayonni yiqitardi.
	safego.Go("model3d.watch:"+p.ID, func() { s.watch(p.ID, taskID) })
	return nil
}

// watch — vazifani tugaguncha kuzatadi. ALOHIDA kontekst bilan:
// HTTP so'rovi allaqachon tugagan, uning konteksti bekor qilingan.
func (s *Service) watch(productID, taskID string) {
	defer s.release(productID)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("3D generatsiya fonida panic", "product", productID, "panic", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.PollTimeout)
	defer cancel()

	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.finishFailed(productID, "kutish vaqti tugadi")
			return
		case <-ticker.C:
		}

		res, err := s.gen.Fetch(ctx, taskID)
		if err != nil {
			// Tarmoq xatosi — vazifa hali tirik bo'lishi mumkin,
			// keyingi urinishda qayta so'raymiz. Faqat kontekst
			// tugaganda to'xtaymiz (yuqoridagi shox).
			slog.Warn("3D vazifa holatini so'rashda xato",
				"provayder", s.gen.Name(), "task", taskID, "err", err)
			continue
		}

		switch res.Status {
		case TaskRunning:
			continue
		case TaskFailed:
			s.finishFailed(productID, res.Err)
			return
		case TaskDone:
			if err := s.storeModel(ctx, productID, res.ModelURL); err != nil {
				slog.Error("3D modelni saqlashda xato",
					"product", productID, "err", err)
				s.finishFailed(productID, err.Error())
			}
			return
		}
	}
}

// storeModel — provayder havolasidan GLB ni yuklab oladi, TEKSHIRADI va
// o'zimizning omborga ko'chiradi.
//
// ┌─ NEGA O'ZIMIZGA KO'CHIRAMIZ ──────────────────────────────────────┐
// Provayder havolasi vaqtinchalik (muddati tugaydi) va uning ishlashi
// bizning nazoratimizda emas. Mijoz ilovasi esa modelni oylar davomida
// ochishi mumkin. Shuning uchun GLB R2 ga ko'chiriladi — rasmlar bilan
// bir xil mantiq.
// └───────────────────────────────────────────────────────────────────┘
func (s *Service) storeModel(ctx context.Context, productID, providerURL string) error {
	data, err := downloadModel(ctx, providerURL, s.cfg.MaxModelBytes)
	if err != nil {
		return err
	}

	key := "models/" + productID + "-" + shortStamp() + ".glb"
	url, err := s.store.Upload(ctx, key, bytes.NewReader(data), int64(len(data)), "model/gltf-binary")
	if err != nil {
		return err
	}

	p, err := s.product(ctx, productID)
	if err != nil {
		return err
	}
	p.Model3DURL = url
	p.Model3DStatus = catalog.Model3DReady
	p.Model3DTaskID = ""
	if err := s.repo.SaveProduct(ctx, p); err != nil {
		return err
	}
	slog.Info("3D model tayyor", "product", productID, "hajm", len(data))
	s.notify(p)
	return nil
}

func (s *Service) finishFailed(productID, reason string) {
	slog.Warn("3D model yaratilmadi", "product", productID, "sabab", reason)
	// Alohida kontekst: chaqiruvchi konteksti allaqachon tugagan
	// bo'lishi mumkin (timeout shoxi), lekin holatni baribir yozishimiz
	// kerak — aks holda mahsulot abadiy "pending" bo'lib qolardi.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	p, err := s.product(ctx, productID)
	if err != nil {
		return
	}
	p.Model3DStatus = catalog.Model3DFailed
	p.Model3DTaskID = ""
	if err := s.repo.SaveProduct(ctx, p); err != nil {
		slog.Error("3D holatini saqlashda xato", "product", productID, "err", err)
		return
	}
	s.notify(p)
}

// Remove — modelni mahsulotdan uzadi.
//
// R2 dagi fayl ATAYLAB o'chirilmaydi: mijoz ilovasida ochiq turgan
// ekranda model birdan yo'qolib qolmasin va o'chirish xatosi
// mahsulotni yangilashga to'sqinlik qilmasin. Yetim fayllar arzon
// (obyekt ombori), yo'qolgan model esa — buzilgan ekran.
func (s *Service) Remove(ctx context.Context, p *catalog.Product) error {
	p.Model3DURL = ""
	p.Model3DStatus = ""
	p.Model3DTaskID = ""
	if err := s.repo.SaveProduct(ctx, p); err != nil {
		return err
	}
	s.notify(p)
	return nil
}

// ResumePending — server ishga tushganda tugallanmagan vazifalarni
// davom ettiradi.
//
// ┌─ NEGA SHART ──────────────────────────────────────────────────────┐
// Generatsiya bir necha daqiqa davom etadi. Shu orada server qayta
// ishga tushsa (deploy, qulash) kuzatuvchi gorutina yo'qoladi va
// mahsulot ABADIY "pending" holatida qolardi — restoran tugmani qayta
// bosa olmasdi ("allaqachon tayyorlanmoqda" xatosi).
//
// Aynan shu naqsh buyurtmalar dispatch'ida ham bor
// (`orders.NeedsDispatchRecovery`).
// └───────────────────────────────────────────────────────────────────┘
func (s *Service) ResumePending(ctx context.Context) {
	restaurants, err := s.repo.ListRestaurants(ctx)
	if err != nil {
		slog.Warn("3D: restoranlar ro'yxati olinmadi, davom ettirish o'tkazib yuborildi", "err", err)
		return
	}
	for _, rest := range restaurants {
		products, err := s.repo.ListProducts(ctx, rest.ID)
		if err != nil {
			continue
		}
		for _, p := range products {
			if p.Model3DStatus != catalog.Model3DPending {
				continue
			}
			if p.Model3DTaskID == "" {
				// Vazifa ID yo'q — kuzatib bo'lmaydi, "failed" ga
				// o'tkazamiz (restoran qayta urinadi).
				s.finishFailed(p.ID, "server qayta ishga tushdi, vazifa ID yo'qolgan")
				continue
			}
			if !s.claim(p.ID) {
				continue
			}
			slog.Info("3D vazifa davom ettirilmoqda", "product", p.ID)
			taskID := p.Model3DTaskID
			safego.Go("model3d.watch:"+p.ID, func() { s.watch(p.ID, taskID) })
		}
	}
}

// ── Yordamchilar ──

func (s *Service) product(ctx context.Context, id string) (*catalog.Product, error) {
	list, err := s.repo.GetProductsByIDs(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return list[0], nil
}

func (s *Service) claim(productID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active[productID] {
		return false
	}
	s.active[productID] = true
	return true
}

func (s *Service) release(productID string) {
	s.mu.Lock()
	delete(s.active, productID)
	s.mu.Unlock()
}

// SetOnUpdate — holat o'zgarishi kuzatuvchisini o'rnatadi.
//
// ┌─ NEGA KONSTRUKTORDA EMAS ─────────────────────────────────────────┐
// Kuzatuvchi WebSocket kanaliga xabar yuboradi va menyu keshini
// tozalaydi — ikkalasi ham `httpapi` paketining ICHKI bilimlari
// (kanal nomi, kesh kaliti). Ular `main` ga eksport qilinsa,
// hozircha ichki bo'lgan tafsilotlar butun loyihaga ochilardi.
//
// Shuning uchun `main` xizmatni kuzatuvchisiz quradi, `httpapi.New`
// esa uni o'zi ulaydi.
// └───────────────────────────────────────────────────────────────────┘
func (s *Service) SetOnUpdate(fn func(*catalog.Product)) { s.onUpdate = fn }

func (s *Service) notify(p *catalog.Product) {
	if s.onUpdate != nil {
		s.onUpdate(p)
	}
}

// wrapGeneration — noma'lum xatoni umumiy turga o'raydi.
//
// TANILGAN sentinel xatolar (kredit tugagan, xizmat band) O'ZGARISHSIZ
// qaytariladi: ularning matni foydalanuvchi uchun yozilgan va HTTP
// qatlami ular uchun aniq status kodi tanlaydi. Ustiga umumiy
// "3D model yaratilmadi" prefiksini qo'shish faqat xabarni
// uzaytirardi.
func wrapGeneration(err error) error {
	if errors.Is(err, ErrNoCredit) || errors.Is(err, ErrProviderBusy) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrGeneration, err)
}

func shortStamp() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// downloadModel — havolani XAVFSIZLIK tekshiruvidan o'tkazib yuklaydi.
//
// Uch qatlam:
//  1. `safeHTTPSURL` — SSRF himoyasi (ichki tarmoqqa so'rov yuborilmasin);
//  2. `io.LimitReader` — hajm chegarasi;
//  3. GLB imzosi — kelgan bayt haqiqatan 3D model ekanini tekshiradi.
func downloadModel(ctx context.Context, rawURL string, maxBytes int64) ([]byte, error) {
	u, err := safeHTTPSURL(rawURL)
	if err != nil {
		return nil, err
	}

	req, err := newRequest(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("model yuklab olinmadi: HTTP %d", resp.StatusCode)
	}

	// maxBytes+1 — chegaradan OSHGANINI aniqlash uchun (aniq maxBytes
	// o'qilsa, fayl kattaroq bo'lishi ham mumkin edi).
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrTooLarge
	}
	if !isGLB(data) {
		return nil, ErrRejected
	}
	return data, nil
}

// isGLB — binary glTF imzosi: "glTF" + versiya.
//
// Bu shunchaki kengaytmaga ishonmaslik: provayder (yoki o'rtadagi
// hujumchi) boshqa fayl bersa, uni R2 ga qo'yib, mijozlarga tarqatgan
// bo'lardik.
func isGLB(b []byte) bool {
	return len(b) >= 12 && bytes.Equal(b[0:4], []byte("glTF"))
}
