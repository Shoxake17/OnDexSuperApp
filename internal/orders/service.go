package orders

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"chustapp/internal/promotions"
)

var ErrNotFound = errors.New("order not found")

// ErrConflict — Save() buyurtma o'qilgandan beri boshqa so'rov tomonidan
// allaqachon o'zgartirilganini aniqlaganda qaytaradi (Order.Version orqali,
// optimistik parallel boshqaruv). Chaqiruvchi (Service) buni ko'rib, yangi
// holatni qayta o'qib, qaytadan urinadi (retryWithOptimisticLock'ga qarang) —
// shuning uchun bu xato odatiy holatda chaqiruvchiga (HTTP handler) umuman
// yetib bormaydi, faqat urinishlar soni tugasa (juda kamdan-kam, og'ir
// tortishuv holatida) chiqadi.
var ErrConflict = errors.New("buyurtma boshqa so'rov tomonidan bir vaqtda o'zgartirildi, qayta urining")

// ErrDuplicateIdempotencyKey — Save() shu mijoz uchun bu idempotency-key
// ALLAQACHON ishlatilganini (boshqa buyurtmada) DB darajasida aniqlaganda
// qaytaradi — Service.Create()dagi oldindan tekshiruv bilan bir xil
// (customer_id, idempotency_key) so'roviga parallel keladigan ikkinchi
// so'rovning ORASIDAGI tabiiy race'ga qarshi oxirgi himoya (check-then-insert
// naqshining o'zi atomik emas, shuning uchun DB unique indeks bilan
// mustahkamlangan).
var ErrDuplicateIdempotencyKey = errors.New("bu idempotency-key allaqachon ishlatilgan")

// Repository — saqlash qatlami. Hozir in-memory, keyin PostgreSQL implementatsiyasi
// shu interface'ni qanoatlantiradi va service kodi o'zgarmaydi.
type Repository interface {
	GetByID(ctx context.Context, id string) (*Order, error)
	// FindByIdempotencyKey — shu mijoz AVVALROQ xuddi shu kalit bilan
	// buyurtma yaratganmi (Create()dagi qayta yuborilgan so'rovni aniqlash
	// uchun). Topilmasa ErrNotFound.
	FindByIdempotencyKey(ctx context.Context, customerID, key string) (*Order, error)
	// Save — yangi buyurtma bo'lsa qo'shadi, mavjud bo'lsa yangilaydi.
	// MUHIM: yangilashda o.Version chaqiruvchi OXIRGI marta GetByID orqali
	// o'qigan qiymat bilan bir xil bo'lishi SHART — implementatsiya buni
	// DB darajasida (masalan `WHERE version = $eski`) tekshirishi va mos
	// kelmasa ErrConflict qaytarishi kerak (oddiy "oxirgi yozuv g'olib"
	// xatti-harakatiga yo'l qo'ymaslik uchun — ikkita parallel o'zgartirish
	// bir xil eski holatni o'qib, biri ikkinchisini jimgina bosib yubormasin).
	// Muvaffaqiyatli yozgach o.Version yangi qiymatga oshiriladi.
	Save(ctx context.Context, o *Order) error
	// ListRecent — eng so'nggi buyurtmalar (admin panel va hisobotlar uchun).
	ListRecent(ctx context.Context, limit int) ([]*Order, error)
	// HasActiveByRestaurant — restoranning yakunlanmagan buyurtmasi bormi
	// (restoranni o'chirishdan oldin tekshiriladi).
	HasActiveByRestaurant(ctx context.Context, restaurantID string) (bool, error)
	// GetActiveByCourier — kuryerning HOZIR yetkazib berayotgan (yakunlanmagan)
	// buyurtmasi (bo'lsa) — kuryer GPS joylashuvini YANGILAGANDA, buni
	// mijozga (WebSocket orqali) jonli yuborish uchun kerak. Topilmasa
	// ErrNotFound (kuryer band emas — jim o'tkazib yuboriladi, xato emas).
	GetActiveByCourier(ctx context.Context, courierID string) (*Order, error)
	// ListByRestaurant — restoranning so'nggi buyurtmalari (restoran paneli).
	ListByRestaurant(ctx context.Context, restaurantID string, limit int) ([]*Order, error)
	// ListByCustomer — mijozning o'z buyurtmalari tarixi ("Buyurtmalarim").
	ListByCustomer(ctx context.Context, customerID string, limit int) ([]*Order, error)
	// CountByCustomerAndRestaurant — mijoz shu restorandan necha marta
	// (bekor qilingan/rad etilganlarni HISOBGA OLMASDAN) buyurtma
	// bergani — promotions.TypeLoyalty'ning "kamida N marta buyurtma"
	// shartini tekshirish uchun.
	CountByCustomerAndRestaurant(ctx context.Context, customerID, restaurantID string) (int, error)
}

// Notifier — holat o'zgarganda tashqi dunyoga xabar (push, WebSocket, restoran paneli).
type Notifier interface {
	// OrderCreated — yangi buyurtma tushdi (restoran paneliga jonli boradi).
	OrderCreated(o *Order)
	OrderStatusChanged(o *Order, from Status)
}

type Service struct {
	repo           Repository
	notifier       Notifier
	idgen          func() string
	now            func() time.Time
	promotionsRepo promotions.Repository // nil = aksiyalar qo'llanilmaydi (masalan testlarda)
}

func NewService(repo Repository, notifier Notifier, idgen func() string, promotionsRepo promotions.Repository) *Service {
	return &Service{repo: repo, notifier: notifier, idgen: idgen, now: time.Now, promotionsRepo: promotionsRepo}
}

// priceCart — Create() VA Quote() ikkalasi ham AYNAN shu funksiyani
// chaqiradi, shuning uchun mijozga checkout'dan oldin ko'rsatilgan
// narx bilan buyurtma yaratilganda haqiqatan yozilgan narx HAR DOIM
// bir xil manbadan kelib chiqadi (ikki xil hisoblash yo'q).
func (s *Service) priceCart(ctx context.Context, restaurantID string, items []Item, customerID string) (subtotal, discount int64, applied *promotions.AppliedDiscount, err error) {
	for _, it := range items {
		subtotal += it.PriceTiyin * int64(it.Qty)
	}
	if s.promotionsRepo == nil {
		return subtotal, 0, nil, nil
	}
	promos, err := s.promotionsRepo.ListByRestaurant(ctx, restaurantID)
	if err != nil {
		return subtotal, 0, nil, err
	}
	var previousOrders int
	if customerID != "" {
		previousOrders, err = s.repo.CountByCustomerAndRestaurant(ctx, customerID, restaurantID)
		if err != nil {
			return subtotal, 0, nil, err
		}
	}
	lines := make([]promotions.CartLine, len(items))
	for i, it := range items {
		lines[i] = promotions.CartLine{
			ProductID: it.ProductID, Category: it.Category, UnitPriceTiyin: it.PriceTiyin, Qty: it.Qty,
		}
	}
	applied = promotions.ApplyBest(promos, lines, previousOrders, s.now())
	if applied != nil {
		discount = applied.DiscountTiyin
	}

	// ---- Mahsulot chegirmasi VS aksiya: ENG YAXSHISI, hech qachon ikkalasi ----
	//
	// Tizimda ikkita mustaqil chegirma mexanizmi bor:
	//   1) mahsulotning o'z chegirma narxi (`DiscountPriceTiyin`) —
	//      restoran panelida belgilanadi;
	//   2) aksiya (`promotions.ApplyBest`).
	//
	// Avval ular QO'SHILARDI: narxlash chegirma narxidan boshlanib,
	// ustiga aksiya chegirmasi ayirilardi. Haqiqiy holatda bu jamini
	// NOLGA tushirdi (105 850 so'mlik savat -> 0 so'm, ya'ni bepul
	// buyurtma). Endi ikkalasi RAQOBATCHI nomzod: qaysi biri mijozga
	// ko'proq foyda bersa, o'sha BITTASI qo'llanadi.
	//
	// Bu `ApplyBest` ning o'z falsafasiga ham mos — u allaqachon bir
	// nechta aksiyadan faqat bittasini tanlaydi (stacking yo'q).
	var productDiscount int64
	for _, it := range items {
		if it.DiscountPriceTiyin > 0 && it.DiscountPriceTiyin < it.PriceTiyin {
			productDiscount += (it.PriceTiyin - it.DiscountPriceTiyin) * int64(it.Qty)
		}
	}
	if productDiscount > discount {
		// Mahsulot chegirmasi yutdi — aksiya UMUMAN qo'llanmaydi
		// (`applied = nil`), shuning uchun chekda aksiya nomi
		// ko'rsatilmaydi va statistikasi ham oshirilmaydi.
		discount = productDiscount
		applied = nil
	}

	// Oxirgi himoya: chegirma hech qachon jamidan oshmaydi. `capDiscount`
	// buni aksiya uchun allaqachon qiladi, lekin mahsulot chegirmasi
	// boshqa yo'ldan keladi — invariant bitta joyda kafolatlanishi kerak.
	if discount > subtotal {
		discount = subtotal
	}
	return subtotal, discount, applied, nil
}

// QuoteResult — mijoz ilovasi checkout'dan OLDIN (masalan savatga
// mahsulot qo'shilganda/o'chirilganda) chaqiradigan HAQIQIY narxlash
// natijasi — real buyurtma yaratmaydi, faqat oldindan ko'rsatadi.
type QuoteResult struct {
	SubtotalTiyin int64  `json:"subtotal_tiyin"`
	DiscountTiyin int64  `json:"discount_tiyin"`
	TotalTiyin    int64  `json:"total_tiyin"`
	PromotionID   string `json:"promotion_id,omitempty"`
	PromotionName string `json:"promotion_name,omitempty"`
}

// Quote — priceCart'ning ochiq (public) o'rovi, HTTP handler uchun.
func (s *Service) Quote(ctx context.Context, restaurantID string, items []Item, customerID string) (*QuoteResult, error) {
	subtotal, discount, applied, err := s.priceCart(ctx, restaurantID, items, customerID)
	if err != nil {
		return nil, err
	}
	res := &QuoteResult{SubtotalTiyin: subtotal, DiscountTiyin: discount, TotalTiyin: subtotal - discount}
	if applied != nil {
		res.PromotionID = applied.Promotion.ID
		res.PromotionName = applied.Promotion.Name
	}
	return res, nil
}

func (s *Service) Create(ctx context.Context, o *Order) (*Order, error) {
	if o.CustomerID == "" || o.RestaurantID == "" || len(o.Items) == 0 {
		return nil, errors.New("customer_id, restaurant_id va items majburiy")
	}
	// Idempotentlik: klient shu mijoz uchun avval AYNAN shu kalit bilan
	// buyurtma yaratgan bo'lsa (masalan tarmoq uzilib qayta yuborilgan
	// so'rov), YANGI buyurtma yaratmasdan ESKISINI qaytaramiz. Bu
	// tekshiruv o'zi atomik emas (check-then-insert) — shu sabab pastda
	// Save() DB darajasidagi unique indeks orqali topgan ziddiyatni ham
	// alohida ushlaymiz (parallel ikkita so'rov bir vaqtda shu tekshiruvdan
	// muvaffaqiyatli o'tib ketishi mumkin).
	if o.IdempotencyKey != "" {
		existing, err := s.repo.FindByIdempotencyKey(ctx, o.CustomerID, o.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return nil, err
		}
	}
	o.ID = s.idgen()
	o.Status = StatusCreated
	o.CreatedAt = s.now()
	o.UpdatedAt = o.CreatedAt
	o.Version = 1

	subtotal, discount, applied, err := s.priceCart(ctx, o.RestaurantID, o.Items, o.CustomerID)
	if err != nil {
		return nil, err
	}
	o.SubtotalTiyin = subtotal
	o.DiscountTiyin = discount
	o.TotalTiyin = subtotal - discount
	if applied != nil {
		o.PromotionID = applied.Promotion.ID
		o.PromotionName = applied.Promotion.Name
	}

	// BEPUL buyurtma hech qachon jimgina yaratilmaydi.
	//
	// Chegirma qoidalari to'g'ri bo'lsa ham, noto'g'ri KIRITILGAN
	// ma'lumot (masalan 18 000 so'mlik mahsulotga 50 so'm chegirma
	// narxi — panelda raqam kiritish xatosi) jamini nolga tushirishi
	// mumkin. Bunday buyurtma restoran uchun sof zarar, shuning uchun
	// u yaratilmaydi va xato ANIQ aytiladi — mijozga "0 so'm" ko'rsatib
	// jim o'tkazib yuborilmaydi.
	if o.TotalTiyin <= 0 {
		return nil, fmt.Errorf("buyurtma jami noto'g'ri (%d tiyin) — chegirma sozlamalarida xato bo'lishi mumkin, restoran bilan bog'laning", o.TotalTiyin)
	}

	if err := s.repo.Save(ctx, o); err != nil {
		if errors.Is(err, ErrDuplicateIdempotencyKey) && o.IdempotencyKey != "" {
			// Race: boshqa parallel so'rov BIZDAN oldin xuddi shu kalit
			// bilan yozib ulgurdi — endi uning natijasini qaytaramiz
			// (dublikat buyurtma yaratmaymiz).
			return s.repo.FindByIdempotencyKey(ctx, o.CustomerID, o.IdempotencyKey)
		}
		return nil, err
	}
	// Aksiya HAQIQATDA qo'llanilgan buyurtma muvaffaqiyatli saqlangach —
	// "Foydalanish"/"Savdo" statistikasini oshiramiz. Bu buyurtma
	// saqlashning o'zidan KEYIN, xato bo'lsa ham buyurtma yaratilishini
	// bloklamaydi (faqat statistikaga ta'sir qiladi, log yozib qo'ya
	// qolamiz).
	if applied != nil && s.promotionsRepo != nil {
		if err := s.promotionsRepo.IncrementUsage(ctx, applied.Promotion.ID, discount); err != nil {
			slog.Error("aksiya statistikasini oshirib bo'lmadi", "promotion_id", applied.Promotion.ID, "error", err)
		}
	}
	if s.notifier != nil {
		s.notifier.OrderCreated(o)
	}
	return o, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Order, error) {
	return s.repo.GetByID(ctx, id)
}

// maxOptimisticRetries — Save() ErrConflict qaytarganda (boshqa so'rov
// bizdan oldin yozib ulgurgan) nechta marta yangi holatni qayta o'qib
// urinamiz. Bu — optimistik parallel boshqaruvning STANDART qo'llanilishi
// (qulflash o'rniga): ziddiyat kamdan-kam, shuning uchun qayta urinish
// arzon va tez; agar 5 martadan keyin ham ziddiyat davom etsa (amalda
// deyarli imkonsiz — bitta buyurtmaga shu qadar zich parallel yozuv),
// ErrConflict chaqiruvchiga (HTTP 409) chiqariladi.
const maxOptimisticRetries = 5

// ChangeStatus — yagona holat o'zgartirish nuqtasi. Hamma handler shu orqali o'tadi,
// shuning uchun state machine'ni chetlab o'tib bo'lmaydi.
//
// Optimistik parallel boshqaruv (Order.Version, [[ErrConflict]]) bilan
// himoyalangan: agar boshqa so'rov (masalan admin "cancel") bizning
// GetByID va Save orasida buyurtmani o'zgartirib ulgursa, Save ErrConflict
// qaytaradi va biz YANGI holatni qaytadan o'qib, ValidateTransition'ni
// SHU YANGI holatga nisbatan qayta tekshirib, qayta urinamiz — shu tufayli
// "eski o'qilgan holat asosida noto'g'ri o'tish ruxsat berilib qo'yish"
// (masalan bekor qilingan buyurtmaga picked_up qo'llanishi) mumkin emas.
func (s *Service) ChangeStatus(ctx context.Context, orderID string, to Status, by Actor) (*Order, error) {
	for attempt := 0; attempt < maxOptimisticRetries; attempt++ {
		o, err := s.repo.GetByID(ctx, orderID)
		if err != nil {
			return nil, err
		}
		if err := ValidateTransition(o.Type, o.Status, to, by); err != nil {
			return nil, err
		}
		if to == StatusPickedUp && o.CourierID == "" {
			return nil, fmt.Errorf("buyurtmaga kuryer biriktirilmagan")
		}
		from := o.Status
		o.Status = to
		o.UpdatedAt = s.now()
		o.History = append(o.History, StatusChange{From: from, To: to, By: by, At: o.UpdatedAt})
		err = s.repo.Save(ctx, o)
		if errors.Is(err, ErrConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if s.notifier != nil {
			s.notifier.OrderStatusChanged(o, from)
		}
		return o, nil
	}
	return nil, ErrConflict
}

// SetPreparationTime — restoran "Qabul qilindi" bosgan payt kiritgan
// taxminiy tayyorlash vaqtini saqlaydi. ReadyAt = hozir + minutes — dispatch
// matching engine kuryerning restoranga ETA'sini AYNAN shu vaqtga
// moslashtirish uchun ishlatadi (na juda erta, na juda kech kelsin).
func (s *Service) SetPreparationTime(ctx context.Context, orderID string, minutes int) (*Order, error) {
	if minutes <= 0 {
		return nil, errors.New("tayyorlash vaqti (daqiqada) musbat bo'lishi kerak")
	}
	// Yuqori chegara: avval faqat `<= 0` rad etilardi. Ulkan qiymat
	// (masalan 999999999999) `time.Duration(minutes) * time.Minute`
	// hisobida int64 toshib ketishiga va natijada MA'NOSIZ (hatto
	// o'tmishdagi) `ReadyAt` qiymatiga olib kelardi — dispatch esa
	// kuryer ETA'sini aynan shu vaqtga moslashtiradi. 4 soat — eng
	// sekin taom uchun ham mo'l-ko'l.
	const maxPreparationMinutes = 240
	if minutes > maxPreparationMinutes {
		return nil, fmt.Errorf("tayyorlash vaqti juda katta (maksimal %d daqiqa)", maxPreparationMinutes)
	}
	for attempt := 0; attempt < maxOptimisticRetries; attempt++ {
		o, err := s.repo.GetByID(ctx, orderID)
		if err != nil {
			return nil, err
		}
		readyAt := s.now().Add(time.Duration(minutes) * time.Minute)
		o.PreparationMinutes = minutes
		o.ReadyAt = &readyAt
		err = s.repo.Save(ctx, o)
		if errors.Is(err, ErrConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return o, nil
	}
	return nil, ErrConflict
}

// AssignCourier — dispatcher muvaffaqiyatli yakunlanganda chaqiriladi.
func (s *Service) AssignCourier(ctx context.Context, orderID, courierID string) (*Order, error) {
	for attempt := 0; attempt < maxOptimisticRetries; attempt++ {
		o, err := s.repo.GetByID(ctx, orderID)
		if err != nil {
			return nil, err
		}
		if o.IsTerminal() {
			return nil, fmt.Errorf("terminal holatdagi buyurtmaga kuryer biriktirib bo'lmaydi")
		}
		if o.CourierID != "" {
			return nil, fmt.Errorf("buyurtmada allaqachon kuryer bor: %s", o.CourierID)
		}
		o.CourierID = courierID
		o.UpdatedAt = s.now()
		err = s.repo.Save(ctx, o)
		if errors.Is(err, ErrConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return o, nil
	}
	return nil, ErrConflict
}
