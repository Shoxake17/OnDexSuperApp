package payments

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

// Payment — bitta to'lov URINISHI.
//
// ┌─ NEGA "URINISH" ──────────────────────────────────────────────────┐
// Bitta buyurtmaga bir nechta yozuv bo'lishi MUMKIN: mijoz kartani
// noto'g'ri kiritsa yoki muddat tugasa, yangi urinish yangi yozuv
// bo'ladi. Octo bir xil `shop_transaction_id` ni QAYTA ishlatishga
// ruxsat bermaydi (jonli sinovda HTTP 500 qaytardi), shuning uchun
// har urinish uchun YANGI ID kerak.
// └───────────────────────────────────────────────────────────────────┘
type Payment struct {
	ID           string `json:"id"`
	OrderID      string `json:"order_id"`
	CustomerID   string `json:"customer_id"`
	RestaurantID string `json:"restaurant_id"`
	Provider     string `json:"provider"`
	// ProviderPaymentID — provayder tomondagi ID (Octo'da
	// `octo_payment_UUID`). To'lov yaratilgunga qadar bo'sh.
	ProviderPaymentID string `json:"provider_payment_id,omitempty"`
	AmountTiyin       int64  `json:"amount_tiyin"`
	Status            Status `json:"status"`
	PayURL            string `json:"pay_url,omitempty"`

	// NeedsReview — to'lov haqida xabar keldi, lekin uni ISHONCHLI
	// tasdiqlab bo'lmadi (imzo kaliti yo'q va status API ishlamadi).
	// Bunday yozuv AVTOMATIK ravishda "to'landi" bo'lmaydi — odam
	// ko'rib chiqishi kerak.
	NeedsReview bool `json:"needs_review,omitempty"`
	// ReviewReason — nega tasdiqlab bo'lmagani (diagnostika uchun).
	ReviewReason string `json:"review_reason,omitempty"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
	// ExpiresAt — to'lov havolasining muddati.
	ExpiresAt time.Time `json:"expires_at"`
	// Raw — provayderning oxirgi xom javobi/xabari (nizolar uchun).
	Raw []byte `json:"-"`
}

// Active — to'lov hali yakunlanmagan va havolasi amal qiladi.
func (p *Payment) Active(now time.Time) bool {
	return p.Status == StatusPending && now.Before(p.ExpiresAt)
}

type Repository interface {
	Create(ctx context.Context, p *Payment) error
	Update(ctx context.Context, p *Payment) error
	GetByID(ctx context.Context, id string) (*Payment, error)
	// GetByProviderID — callback provayder ID'si bilan keladi.
	GetByProviderID(ctx context.Context, provider, providerPaymentID string) (*Payment, error)
	// ListByOrder — buyurtmaning barcha urinishlari (yangisi birinchi).
	ListByOrder(ctx context.Context, orderID string) ([]*Payment, error)
	// ListExpired — muddati o'tgan, hali yakunlanmagan to'lovlar
	// (avtomatik bekor qilish uchun).
	ListExpired(ctx context.Context, now time.Time, limit int) ([]*Payment, error)
}

// OrderSink — to'lov holati o'zgarganda buyurtmaga xabar berish.
// `payments` paketi `orders` ni IMPORT QILMAYDI (aylanma bog'liqlik
// bo'lardi), shuning uchun kerakli amallar shu tor interfeysda.
type OrderSink interface {
	// OrderAmountTiyin — buyurtmaning HAQIQIY summasi. To'lov summasi
	// mijozdan emas, AYNAN shu yerdan olinadi.
	OrderAmountTiyin(ctx context.Context, orderID string) (int64, error)
	// OnPaymentHeld — pul bloklandi (yoki yechildi): buyurtma endi
	// oshxonaga tushishi mumkin.
	OnPaymentHeld(ctx context.Context, orderID string) error
	// OnPaymentFailed — to'lov amalga oshmadi/bekor qilindi.
	OnPaymentFailed(ctx context.Context, orderID string) error
}

// Service — to'lov oqimini boshqaradi.
type Service struct {
	repo     Repository
	provider Provider
	orders   OrderSink
	idgen    func() string
	now      func() time.Time

	// returnURL — to'lovdan keyin mijoz qaytariladigan manzil.
	returnURL string
	// notifyURL — provayder callback yuboradigan TO'LIQ manzil.
	notifyURL string
	// ttlMinutes — to'lov havolasining yashash muddati.
	ttlMinutes int

	// trustCallbackWithoutProof — FAQAT dev uchun: imzo kaliti ham,
	// ishlaydigan status API ham bo'lmaganda callback'ga ISHONISH.
	// Production'da HECH QACHON yoqilmaydi (`NewService` tekshiradi).
	trustCallbackWithoutProof bool

	// onPaid — pul HAQIQATAN yechilganda (restoran bildirishnomasi).
	// Chaqiruvchi uni o'z goroutine'ida bajarishi kerak — to'lov oqimi
	// kutmaydi.
	onPaid func(p Payment)
}

// OnPaid — "pul yechildi" kuzatuvchisi (`alerts.Service.PaymentReceived`).
func (s *Service) OnPaid(fn func(p Payment)) { s.onPaid = fn }

func (s *Service) emitPaid(p *Payment) {
	if s.onPaid != nil && p != nil {
		s.onPaid(*p)
	}
}

type Options struct {
	ReturnURL  string
	NotifyURL  string
	TTLMinutes int
	// TrustCallbackWithoutProof — dev rejimida uchdan-uchiga sinash
	// uchun. Production'da yoqib bo'lmaydi.
	TrustCallbackWithoutProof bool
	// Production — true bo'lsa xavfsiz bo'lmagan sozlamalar rad etiladi.
	Production bool
}

func NewService(repo Repository, provider Provider, orders OrderSink,
	idgen func() string, opts Options) (*Service, error) {
	if repo == nil || provider == nil || orders == nil || idgen == nil {
		return nil, errors.New("payments: bog'liqliklar to'liq emas")
	}
	// ┌─ TUZATILGAN NOSOZLIK (bug.md 42-band) ────────────────────────┐
	// Avval bu yerda faqat `opts.NotifyURL == ""` tekshirilardi.
	// `PUBLIC_API_URL` prod compose'ida sanab chiqilmagani uchun
	// `os.Getenv` bo'sh satr qaytarardi va manzil shunday quriladi:
	//
	//	"" + "/payments/octo/callback"  →  "/payments/octo/callback"
	//
	// Bu NISBIY yo'l — bo'sh emas, ya'ni tekshiruvdan bemalol o'tib
	// ketardi. Octo bunday manzilga callback yubora olmaydi:
	// karta to'lovi hech qachon tasdiqlanmasdi, buyurtma `awaiting`
	// da qolib restoranga ko'rinmasdi va 30 daqiqadan keyin bekor
	// bo'lardi — mijozning puli olingan holda. Hech qanday xato
	// yoki log bu haqda ogohlantirmasdi.
	//
	// Endi manzil MUTLAQ va `https://` bilan boshlanishi shart:
	// sozlama yo'qolsa server ISHGA TUSHMAYDI, ya'ni xato deploy
	// paytida ko'rinadi, birinchi to'lovda emas.
	//
	// Dev'da `http://` ga ruxsat beriladi (lokal tunnel/emulyator).
	// └────────────────────────────────────────────────────────────────┘
	if err := validateNotifyURL(opts.NotifyURL, opts.Production); err != nil {
		return nil, err
	}
	// ┌─ XAVFSIZ BO'LMAGAN SOZLAMA PRODUCTION'GA O'TMAYDI ────────────┐
	// `TrustCallbackWithoutProof` — callback'ga dalilsiz ishonish
	// degani. Ochiq internetdan kelgan so'rov bilan buyurtmani
	// "to'landi" qilib qo'yish mumkin bo'lardi. Shuning uchun u
	// production'da server ISHGA TUSHMAY to'xtatadi — jimgina
	// o'chirib qo'yish yomonroq bo'lardi (kim yoqganini bilmay
	// qolardik).
	// └───────────────────────────────────────────────────────────────┘
	if opts.Production && opts.TrustCallbackWithoutProof {
		return nil, errors.New("payments: PAYMENTS_TRUST_CALLBACK_DEV production'da yoqib bo'lmaydi")
	}
	ttl := opts.TTLMinutes
	if ttl <= 0 {
		ttl = 30
	}
	return &Service{
		repo: repo, provider: provider, orders: orders, idgen: idgen,
		now:       time.Now,
		returnURL: opts.ReturnURL, notifyURL: opts.NotifyURL, ttlMinutes: ttl,
		trustCallbackWithoutProof: opts.TrustCallbackWithoutProof,
	}, nil
}

// validateNotifyURL — callback manzili provayder murojaat qila
// oladigan MUTLAQ manzil ekanini tekshiradi.
//
// Nega alohida funksiya: bu shart pul oqimida turadi va uni sinash
// uchun butun `Service` ni qurish shart emas
// (`service_test.go: TestValidateNotifyURL`).
func validateNotifyURL(raw string, production bool) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("payments: notify_url majburiy (callback shu manzilga keladi) — " +
			"PUBLIC_API_URL o'zgaruvchisi berilganmi?")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("payments: notify_url noto'g'ri: %w", err)
	}
	// Host bo'sh bo'lsa manzil nisbiy ("/payments/octo/callback") —
	// aynan shu holat production'da oylab sezilmay turgan edi.
	if u.Host == "" {
		return fmt.Errorf("payments: notify_url MUTLAQ manzil bo'lishi kerak "+
			"(host yo'q: %q) — PUBLIC_API_URL to'liq berilganmi?", raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if production {
			return fmt.Errorf("payments: notify_url production'da https bo'lishi SHART (berilgan: %q)", raw)
		}
		return nil
	default:
		return fmt.Errorf("payments: notify_url sxemasi qo'llab-quvvatlanmaydi: %q", raw)
	}
}

// ErrAlreadyPaid — buyurtma allaqachon to'langan.
var ErrAlreadyPaid = errors.New("buyurtma allaqachon to'langan")

// ErrCallbackMismatch — callback ichidagi ikki identifikator BOSHQA-BOSHQA
// to'lovga ishora qilyapti.
//
// ┌─ NEGA BU ALOHIDA XATO ────────────────────────────────────────────┐
// Octo callback'ida ikkita identifikator keladi:
//
//	shop_transaction_id — BIZNING to'lov ID'imiz
//	octo_payment_UUID   — OCTO tomondagi ID
//
// Imzo esa FAQAT `octo_payment_UUID` + `status` ustidan hisoblanadi
// (`octo.VerifyCallbackSignature`). Ya'ni yaroqli imzo "shu UUID shu
// holatda" degan faktni isbotlaydi va BIZNING qaysi to'lovimiz haqida
// ekanini UMUMAN aytmaydi.
//
// Ikkalasi bir to'lovga tegishli ekani tekshirilmasa, hujum juda oson
// bo'lardi: hujumchi o'zining 1 000 so'mlik to'lovini haqiqatan qilib,
// undan yaroqli (UUID, status, signature) uchligini oladi, so'ng katta
// buyurtma yaratib callback'ni QO'LDA yuboradi — o'z UUID/imzosi bilan,
// lekin BOSHQA `shop_transaction_id` bilan. Imzo to'g'ri, to'lov esa
// butunlay boshqasi.
//
// Shuning uchun bog'lanish buzilganda xabar butunlay rad etiladi.
// └───────────────────────────────────────────────────────────────────┘
var ErrCallbackMismatch = errors.New("callback identifikatorlari bir-biriga mos emas")

// ReturnURL — to'lov tugagach provayder mijozni qaytaradigan manzil.
//
// Mobil ilova to'lov sahifasini O'Z ICHIDAGI WebView'da ochadi va shu
// manzilga o'tilganini ko'rib oynani yopadi. Manzilni ilovaga qattiq
// yozib qo'ymaymiz — u serverning sozlamasi va dev/prod'da farq qiladi.
func (s *Service) ReturnURL() string { return s.returnURL }

// minRetryAge — "qayta urinish" tugmasi shu vaqtdan yosh urinishga
// yangi tranzaksiya ochmaydi. Mijoz tugmani ikki marta bosib yuborsa
// yoki sahifa qayta yuklansa, Octo'da keraksiz tranzaksiyalar
// to'planib qolmasligi uchun.
const minRetryAge = 60 * time.Second

// StartForOrder — buyurtma uchun to'lov boshlaydi va to'lov sahifasi
// havolasini qaytaradi.
//
// ┌─ SUMMA KLIENTDAN OLINMAYDI ───────────────────────────────────────┐
// Summa `OrderSink.OrderAmountTiyin` orqali BUYURTMADAN olinadi.
// Klient yuborgan qiymatga ishonilsa, mijoz 100 000 so'mlik savatni
// 1 000 so'mga "to'lay" olardi.
// └───────────────────────────────────────────────────────────────────┘
//
// Takroriy chaqirish xavfsiz: hali amal qilayotgan urinish bo'lsa,
// YANGI to'lov yaratilmaydi va o'sha havola qaytariladi (provayder
// bir xil tranzaksiya ID'sini qayta ishlatishga ruxsat bermaydi).
//
// ┌─ `retry` NIMA UCHUN KERAK ────────────────────────────────────────┐
// Bitta to'lov urinishi bank tomonda "o'lishi" mumkin: OTP kodini uch
// marta xato kiritish yoki SMS ni ko'p marta qayta so'rash tranzaksiyani
// bekor qiladi ("takroriy SMS xabarlarining maksimal soni"). O'sha
// havolani qayta ochish YORDAM BERMAYDI — bank uni qabul qilmaydi va
// mijoz tuzoqqa tushib qoladi.
//
// `retry=true` bo'lsa eski urinish yopiladi va YANGI tranzaksiya
// ochiladi. Juda tez-tez bosishdan himoya: urinish `minRetryAge` dan
// yosh bo'lsa yangisi yaratilmaydi (aks holda har bosishda Octo'da
// yangi tranzaksiya paydo bo'lardi).
// └───────────────────────────────────────────────────────────────────┘
func (s *Service) StartForOrder(ctx context.Context, orderID, customerID, restaurantID, description string, retry bool) (*Payment, error) {
	existing, err := s.repo.ListByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for _, p := range existing {
		switch {
		case p.Status == StatusPaid || p.Status == StatusHeld:
			return nil, ErrAlreadyPaid
		case p.Active(now) && p.PayURL != "":
			if !retry || now.Sub(p.CreatedAt) < minRetryAge {
				return p, nil
			}
			// Eski urinish yopiladi. Provayder tomonda u o'z muddati
			// bilan o'ladi; mijoz uni baribir to'lay olsa, kechikkan
			// callback qabul qilinadi (`applyStatus` izohiga qarang).
			p.Status = StatusFailed
			p.ReviewReason = "mijoz qayta urindi — yangi to'lov ochildi"
			p.UpdatedAt = now
			if uerr := s.repo.Update(ctx, p); uerr != nil {
				return nil, uerr
			}
		}
	}

	amount, err := s.orders.OrderAmountTiyin(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if amount <= 0 {
		return nil, errors.New("to'lov summasi noto'g'ri")
	}

	p := &Payment{
		ID:           s.idgen(),
		OrderID:      orderID,
		CustomerID:   customerID,
		RestaurantID: restaurantID,
		Provider:     s.provider.Name(),
		AmountTiyin:  amount,
		Status:       StatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
		ExpiresAt:    now.Add(time.Duration(s.ttlMinutes) * time.Minute),
	}
	// Yozuv provayderga so'rov YUBORISHDAN OLDIN saqlanadi: aks holda
	// so'rov ketib, javob yo'lda yo'qolsa (tarmoq uzilishi), bizda
	// hech qanday iz qolmasdi va mijoz to'lagan pulni hech kim
	// bog'lay olmasdi.
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}

	res, err := s.provider.Create(ctx, CreateRequest{
		PaymentID:   p.ID,
		AmountTiyin: amount,
		Description: description,
		// IKKI BOSQICHLI: pul bloklanadi, restoran qabul qilgandan
		// keyin yechiladi.
		Hold:       true,
		ReturnURL:  s.returnURL,
		NotifyURL:  s.notifyURL,
		TTLMinutes: s.ttlMinutes,
		Language:   "uz",
		CustomerID: customerID,
	})
	if err != nil {
		p.Status = StatusFailed
		p.ReviewReason = err.Error()
		p.UpdatedAt = s.now()
		if uerr := s.repo.Update(ctx, p); uerr != nil {
			slog.Error("to'lov holatini saqlab bo'lmadi", "payment", p.ID, "error", uerr)
		}
		return nil, err
	}

	p.ProviderPaymentID = res.ProviderPaymentID
	p.PayURL = res.PayURL
	p.UpdatedAt = s.now()
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// CallbackData — provayderdan kelgan xabarning MOSLASHTIRILGAN
// ko'rinishi (provayderga xos maydonlar HTTP qatlamida ajratiladi).
type CallbackData struct {
	PaymentID         string // bizning ID (shop_transaction_id)
	ProviderPaymentID string
	Status            Status
	AmountTiyin       int64
	// SignatureValid/SignatureChecked — imzo tekshiruvi natijasi.
	SignatureValid   bool
	SignatureChecked bool
	Raw              []byte
}

// HandleCallback — provayder xabarini qayta ishlaydi.
//
// ┌─ ISHONCH ZANJIRI ─────────────────────────────────────────────────┐
// Callback ochiq internetdan keladi — uni istalgan odam yubora oladi.
// Shuning uchun to'lov FAQAT quyidagi dalillardan biri bo'lganda
// tasdiqlanadi:
//
//  1. imzo tekshirildi va TO'G'RI (`unique_key` sozlangan bo'lsa);
//  2. yoki provayderning O'Z API'sidan holat so'raldi va u tasdiqladi.
//
// Ikkalasi ham bo'lmasa — to'lov `NeedsReview` bilan belgilanadi va
// buyurtma OSHXONAGA TUSHMAYDI. Bu holat jimgina o'tkazib
// yuborilmaydi: log'da xato darajasida yoziladi.
// └───────────────────────────────────────────────────────────────────┘
func (s *Service) HandleCallback(ctx context.Context, cb CallbackData) error {
	p, err := s.findPayment(ctx, cb)
	if err != nil {
		return err
	}

	status := cb.Status
	amount := cb.AmountTiyin
	proof := ""

	// Imzo FAQAT u aynan shu to'lovga bog'langan bo'lsa dalil bo'ladi
	// (`callbackIsBound` izohiga qarang). Bog'lanmagan bo'lsa quyidagi
	// `default` shoxiga tushadi va dalil provayder API'sidan so'raladi.
	if cb.SignatureChecked && cb.SignatureValid && !callbackIsBound(p, cb) {
		slog.Warn("to'lov: imzo to'g'ri, lekin to'lov provayder ID'siga bog'lanmagan — API so'raladi",
			"payment", p.ID, "order", p.OrderID,
			"callbackdagi_uuid", cb.ProviderPaymentID)
	}

	switch {
	case cb.SignatureChecked && cb.SignatureValid && callbackIsBound(p, cb):
		proof = "imzo"
	case cb.SignatureChecked && !cb.SignatureValid:
		// Imzo tekshirildi va NOTO'G'RI — bu soxta xabar. Boshqa
		// dalil qidirilmaydi.
		slog.Error("to'lov: callback imzosi NOTO'G'RI — e'tiborga olinmadi",
			"payment", p.ID, "order", p.OrderID)
		return errors.New("imzo noto'g'ri")
	default:
		// Imzo tekshirilmadi (kalit yo'q) — provayderdan so'raymiz.
		st, serr := s.provider.Status(ctx, p.ID)
		switch {
		case serr == nil:
			proof = "status-api"
			status = st.Status
			if st.PaidAmountTiyin > 0 {
				amount = st.PaidAmountTiyin
			}
		case s.trustCallbackWithoutProof:
			// FAQAT dev: uchdan-uchiga sinash uchun.
			proof = "dev-trust"
			slog.Warn("to'lov: callback DALILSIZ qabul qilindi (faqat dev rejim)",
				"payment", p.ID, "order", p.OrderID, "status_api_error", serr)
		default:
			p.NeedsReview = true
			p.ReviewReason = fmt.Sprintf("tasdiqlab bo'lmadi: imzo kaliti yo'q, status API xatosi: %v", serr)
			p.UpdatedAt = s.now()
			p.Raw = cb.Raw
			if uerr := s.repo.Update(ctx, p); uerr != nil {
				return uerr
			}
			slog.Error("to'lov: TASDIQLAB BO'LMADI — buyurtma kutmoqda",
				"payment", p.ID, "order", p.OrderID, "callback_status", cb.Status,
				"status_api_error", serr)
			return nil
		}
	}

	// Summa tekshiruvi: kam to'langan buyurtma oshxonaga tushmaydi.
	if (status == StatusHeld || status == StatusPaid) && amount > 0 && amount < p.AmountTiyin {
		p.NeedsReview = true
		p.ReviewReason = fmt.Sprintf("summa kam: %d tiyin, kutilgan %d", amount, p.AmountTiyin)
		p.UpdatedAt = s.now()
		p.Raw = cb.Raw
		if uerr := s.repo.Update(ctx, p); uerr != nil {
			return uerr
		}
		slog.Error("to'lov: SUMMA MOS EMAS", "payment", p.ID, "order", p.OrderID,
			"kelgan", amount, "kutilgan", p.AmountTiyin)
		return nil
	}

	return s.applyStatus(ctx, p, status, cb.Raw, proof)
}

func (s *Service) findPayment(ctx context.Context, cb CallbackData) (*Payment, error) {
	if cb.PaymentID != "" {
		if p, err := s.repo.GetByID(ctx, cb.PaymentID); err == nil {
			// ★ BOG'LANISH TEKSHIRUVI (ErrCallbackMismatch izohiga qarang).
			// Ikkala identifikator ham ma'lum bo'lsa, ular AYNAN bir
			// to'lovni ko'rsatishi shart.
			if cb.ProviderPaymentID != "" && p.ProviderPaymentID != "" &&
				!strings.EqualFold(p.ProviderPaymentID, cb.ProviderPaymentID) {
				slog.Error("to'lov: callback IDENTIFIKATORLARI MOS EMAS — rad etildi",
					"payment", p.ID, "order", p.OrderID,
					"bizdagi_uuid", p.ProviderPaymentID,
					"callbackdagi_uuid", cb.ProviderPaymentID)
				return nil, ErrCallbackMismatch
			}
			return p, nil
		}
	}
	if cb.ProviderPaymentID != "" {
		return s.repo.GetByProviderID(ctx, s.provider.Name(), cb.ProviderPaymentID)
	}
	return nil, ErrNotFound
}

// callbackIsBound — callback'dagi imzo AYNAN shu to'lovga tegishlimi.
//
// Imzo `octo_payment_UUID` ustidan hisoblanadi, shuning uchun u faqat
// yozuvimiz o'sha UUID'ga BOG'LANGAN bo'lsa dalil bo'la oladi.
//
// `p.ProviderPaymentID` bo'sh bo'lishi mumkin: `StartForOrder` avval
// yozuvni yaratadi, provayder javobini olgach esa UUID'ni yozadi
// (`repo.Create` va `repo.Update` orasidagi qisqa oyna). Aynan o'sha
// oynada kelgan callback uchun imzoga TAYANIB BO'LMAYDI — u boshqa
// to'lovning imzosi bo'lishi mumkin va biz buni tekshira olmaymiz.
// Bunday holatda dalil provayderning O'Z API'sidan so'raladi (u bizning
// `p.ID` bo'yicha javob beradi, ya'ni chalkashish imkoni yo'q).
func callbackIsBound(p *Payment, cb CallbackData) bool {
	if cb.ProviderPaymentID == "" {
		// Provayder ID umuman berilmagan — imzo ham bo'lishi mumkin emas.
		return false
	}
	return strings.EqualFold(p.ProviderPaymentID, cb.ProviderPaymentID)
}

// applyStatus — holatni saqlaydi va buyurtmaga xabar beradi.
// IDEMPOTENT: bir xil xabar necha marta kelsa ham buyurtma bir marta
// o'zgaradi (Octo javob olmaguncha callback'ni takrorlaydi — jonli
// sinovda har ~50 soniyada).
func (s *Service) applyStatus(ctx context.Context, p *Payment, status Status, raw []byte, proof string) error {
	if p.Status == status {
		return nil // takroriy xabar — hech narsa o'zgarmaydi
	}
	// ┌─ QAYSI HOLAT HAQIQATAN YAKUNIY ───────────────────────────────┐
	// Faqat PUL HARAKATLANGAN holatlar (`paid`, `refunded`) orqaga
	// qaytmaydi — kechikkan eski xabar to'langan buyurtmani
	// "kutilmoqda" ga tushirib yubormasligi kerak.
	//
	// `failed`/`canceled` esa BIZNING mahalliy qarorimiz (masalan mijoz
	// qayta urinib, eski urinishni yopdik). Agar mijoz baribir eski
	// havolani to'lasa, provayderdan "pul bloklandi" xabari keladi va
	// unga ISHONISH KERAK: pul haqiqatan provayderda turibdi. Aks holda
	// mijozning puli bloklanib, buyurtmasi esa to'lanmagan bo'lib
	// qolardi.
	// └───────────────────────────────────────────────────────────────┘
	if (p.Status == StatusPaid || p.Status == StatusRefunded) && status != StatusRefunded {
		slog.Warn("to'lov: yakuniy holatdan keyin xabar keldi, e'tiborga olinmadi",
			"payment", p.ID, "hozirgi", p.Status, "kelgan", status)
		return nil
	}

	prev := p.Status
	p.Status = status
	p.UpdatedAt = s.now()
	p.NeedsReview = false
	p.ReviewReason = ""
	if len(raw) > 0 {
		p.Raw = raw
	}
	if status == StatusPaid && p.PaidAt == nil {
		t := s.now()
		p.PaidAt = &t
	}
	if err := s.repo.Update(ctx, p); err != nil {
		return err
	}
	slog.Info("to'lov holati o'zgardi", "payment", p.ID, "order", p.OrderID,
		"dan", prev, "ga", status, "dalil", proof)
	if status == StatusPaid {
		s.emitPaid(p)
	}

	switch status {
	case StatusHeld, StatusPaid:
		return s.orders.OnPaymentHeld(ctx, p.OrderID)
	case StatusCanceled, StatusFailed:
		return s.orders.OnPaymentFailed(ctx, p.OrderID)
	}
	return nil
}

// CaptureForOrder — restoran buyurtmani QABUL QILGANDA chaqiriladi:
// bloklangan pul haqiqatan yechiladi.
func (s *Service) CaptureForOrder(ctx context.Context, orderID string, amountTiyin int64) error {
	p, err := s.heldPayment(ctx, orderID)
	if err != nil || p == nil {
		return err
	}
	if amountTiyin <= 0 || amountTiyin > p.AmountTiyin {
		amountTiyin = p.AmountTiyin
	}
	if err := s.provider.Capture(ctx, p.ProviderPaymentID, amountTiyin); err != nil {
		return fmt.Errorf("to'lovni tasdiqlab bo'lmadi: %w", err)
	}
	t := s.now()
	p.Status = StatusPaid
	p.PaidAt = &t
	p.UpdatedAt = t
	if err := s.repo.Update(ctx, p); err != nil {
		return err
	}
	s.emitPaid(p)
	// ┌─ ORTIQCHA BLOKLAR BO'SHATILADI ───────────────────────────────┐
	// Mijoz qayta urinib, ikkala havolani ham to'lagan bo'lishi mumkin
	// (eski va yangi). Bitta buyurtma uchun bitta to'lov yechiladi,
	// qolgan bloklar esa DARHOL bo'shatiladi — aks holda mijozning
	// puli 7-30 kun bloklanib qolardi.
	// └───────────────────────────────────────────────────────────────┘
	others, err := s.repo.ListByOrder(ctx, orderID)
	if err != nil {
		slog.Warn("ortiqcha to'lov bloklarini o'qib bo'lmadi", "order", orderID, "err", err)
		return nil //nolint:nilerr // pul yechildi: ortiqcha bloklarni bo'shatish asosiy natijani bekor qilmaydi
	}
	for _, other := range others {
		if other.ID == p.ID || other.Status != StatusHeld || other.ProviderPaymentID == "" {
			continue
		}
		if cerr := s.provider.Cancel(ctx, other.ProviderPaymentID); cerr != nil {
			slog.Error("ortiqcha blokni bo'shatib bo'lmadi",
				"payment", other.ID, "order", orderID, "error", cerr)
			continue
		}
		other.Status = StatusCanceled
		other.ReviewReason = "ortiqcha urinish — blok bo'shatildi"
		other.UpdatedAt = s.now()
		if uerr := s.repo.Update(ctx, other); uerr != nil {
			slog.Error("ortiqcha to'lovni saqlab bo'lmadi", "payment", other.ID, "error", uerr)
		}
	}
	return nil
}

// ReleaseForOrder — buyurtma rad etilganda/bekor qilinganda: blok
// bo'shatiladi (pul mijozda qoladi) yoki allaqachon yechilgan bo'lsa
// qaytariladi.
func (s *Service) ReleaseForOrder(ctx context.Context, orderID string) error {
	list, err := s.repo.ListByOrder(ctx, orderID)
	if err != nil {
		return err
	}
	for _, p := range list {
		switch p.Status {
		case StatusHeld:
			if err := s.provider.Cancel(ctx, p.ProviderPaymentID); err != nil {
				return fmt.Errorf("blokni bo'shatib bo'lmadi: %w", err)
			}
			p.Status = StatusCanceled
		case StatusPaid:
			if err := s.provider.Refund(ctx, p.ProviderPaymentID, s.idgen(), p.AmountTiyin); err != nil {
				return fmt.Errorf("pulni qaytarib bo'lmadi: %w", err)
			}
			p.Status = StatusRefunded
		default:
			continue
		}
		p.UpdatedAt = s.now()
		if err := s.repo.Update(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// heldPayment — buyurtmaning bloklangan to'lovi (bo'lmasa nil, nil).
func (s *Service) heldPayment(ctx context.Context, orderID string) (*Payment, error) {
	list, err := s.repo.ListByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	for _, p := range list {
		if p.Status == StatusHeld && p.ProviderPaymentID != "" {
			return p, nil
		}
	}
	return nil, nil
}

// ListByOrder — buyurtmaning to'lov urinishlari (HTTP qatlami uchun).
func (s *Service) ListByOrder(ctx context.Context, orderID string) ([]*Payment, error) {
	return s.repo.ListByOrder(ctx, orderID)
}

// ExpireStale — muddati o'tgan, to'lanmagan urinishlarni yopadi va
// buyurtmaga xabar beradi. Fon vazifasi sifatida davriy chaqiriladi.
func (s *Service) ExpireStale(ctx context.Context, limit int) (int, error) {
	now := s.now()
	list, err := s.repo.ListExpired(ctx, now, limit)
	if err != nil {
		return 0, err
	}
	var n int
	for _, p := range list {
		p.Status = StatusFailed
		p.ReviewReason = "to'lov muddati tugadi"
		p.UpdatedAt = now
		if err := s.repo.Update(ctx, p); err != nil {
			return n, err
		}
		if err := s.orders.OnPaymentFailed(ctx, p.OrderID); err != nil {
			slog.Error("muddati o'tgan to'lov: buyurtmani yangilab bo'lmadi",
				"order", p.OrderID, "error", err)
		}
		n++
	}
	return n, nil
}
