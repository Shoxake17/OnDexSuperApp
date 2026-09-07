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

// ErrTotalChanged — mijozga KO'RSATILGAN summa bilan hozirgi haqiqiy
// summa mos kelmadi, shuning uchun buyurtma yaratilmadi.
//
// ┌─ NEGA KERAK ───────────────────────────────────────────────────────┐
// Ekranda summa ko'rsatilishi bilan tugma bosilishi orasida vaqt
// o'tadi. O'sha oraliqda aksiya tugashi, restoran narxni tahrirlashi
// yoki chegirma bekor bo'lishi mumkin. Tekshiruvsiz mijoz BOSHQA
// summaga rozi bo'lgan buyurtmani olardi va buni faqat chekda
// ko'rardi.
//
// Ayniqsa AI yordamchisi uchun muhim: u yerda savatni odam emas, til
// modeli tuzadi va foydalanuvchi faqat yakuniy raqamga qarab
// tasdiqlaydi — o'sha raqam buzilmasligi kerak.
//
// Tekshiruv IXTIYORIY: `expectedTotalTiyin <= 0` berilsa o'tkazib
// yuboriladi (eski chaqiruvlar o'zgarmaydi).
// └────────────────────────────────────────────────────────────────────┘
var ErrTotalChanged = errors.New("narx o'zgardi")

// TotalChangedError — `ErrTotalChanged` ning YANGI SUMMA bilan
// birgalikdagi shakli.
//
// Faqat matn qaytarilsa HTTP qatlami yangi raqamni matndan ajratib
// olishga majbur bo'lardi. Bu tur uni tayyor beradi, ya'ni ilova
// darhol "yangi summa — shuncha, tasdiqlaysizmi?" deb ko'rsata oladi.
type TotalChangedError struct {
	ExpectedTiyin int64
	ActualTiyin   int64
}

func (e *TotalChangedError) Error() string {
	return fmt.Sprintf("narx o'zgardi: ko'rsatilgan %d tiyin, hozirgi %d tiyin",
		e.ExpectedTiyin, e.ActualTiyin)
}

// Unwrap — `errors.Is(err, ErrTotalChanged)` ishlashi uchun.
func (e *TotalChangedError) Unwrap() error { return ErrTotalChanged }

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
	// HasActiveByCustomer — mijozning yakunlanmagan buyurtmasi bormi
	// (akkauntni o'chirishdan oldin tekshiriladi).
	//
	// ┌─ NEGA ALOHIDA METOD (bug.md 27-band) ─────────────────────────┐
	// Avval bu tekshiruv `ListByCustomer(ctx, id, 20)` bilan
	// bajarilardi va izohda "faol buyurtma har doim shular orasida
	// bo'ladi" deb yozilgandi. Bu NOTO'G'RI: faol mijozda
	// yakunlanmagan buyurtma eng yangi 20 tadan pastda qolishi
	// mumkin — o'shanda akkaunt o'chiriladi va buyurtma EGASIZ
	// qoladi.
	//
	// Kuryer uchun to'g'ri naqsh allaqachon bor
	// (`GetActiveByCourier` — bazadan aynan shu savol), mijoz uchun
	// qo'llanmagandi.
	// └───────────────────────────────────────────────────────────────┘
	HasActiveByCustomer(ctx context.Context, customerID string) (bool, error)
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

// PaymentGateway — buyurtma holati o'zgarganda pulni tasdiqlash yoki
// bo'shatish. `orders` paketi to'lov provayderini BILMAYDI — bu tor
// interfeys `internal/payments` da amalga oshiriladi.
type PaymentGateway interface {
	// CaptureForOrder — bloklangan pulni yechadi (restoran qabul qildi).
	CaptureForOrder(ctx context.Context, orderID string, amountTiyin int64) error
	// ReleaseForOrder — blokni bo'shatadi yoki pulni qaytaradi
	// (buyurtma rad etildi/bekor qilindi).
	ReleaseForOrder(ctx context.Context, orderID string) error
}

type Service struct {
	repo           Repository
	notifier       Notifier
	idgen          func() string
	now            func() time.Time
	promotionsRepo promotions.Repository // nil = aksiyalar qo'llanilmaydi (masalan testlarda)
	payments       PaymentGateway        // nil = karta to'lovi ulanmagan
}

// WithPayments — karta to'lovini ulaydi. Alohida setter: mavjud
// `NewService` chaqiruvlari (va o'nlab testlar) o'zgarmasin.
func (s *Service) WithPayments(p PaymentGateway) *Service {
	s.payments = p
	return s
}

func NewService(repo Repository, notifier Notifier, idgen func() string, promotionsRepo promotions.Repository) *Service {
	return &Service{repo: repo, notifier: notifier, idgen: idgen, now: time.Now, promotionsRepo: promotionsRepo}
}

// priceCart — Create() VA Quote() ikkalasi ham AYNAN shu funksiyani
// chaqiradi, shuning uchun mijozga checkout'dan oldin ko'rsatilgan
// narx bilan buyurtma yaratilganda haqiqatan yozilgan narx HAR DOIM
// bir xil manbadan kelib chiqadi (ikki xil hisoblash yo'q).
// lineDiscounts — chegirmaning savat qatorlari bo'yicha taqsimoti
// (`items` bilan bir xil uzunlik va tartib). Yig'indisi HAR DOIM
// `discount` ga teng, ya'ni mijoz ekranda ko'rgan qator narxlari
// pastdagi jamiga aniq qo'shiladi.
func (s *Service) priceCart(ctx context.Context, restaurantID string, items []Item, customerID string) (subtotal, discount int64, applied promotions.Result, lineDiscounts []int64, err error) {
	lineDiscounts = make([]int64, len(items))
	for _, it := range items {
		subtotal += it.PriceTiyin * int64(it.Qty)
	}
	if s.promotionsRepo == nil {
		return subtotal, 0, applied, lineDiscounts, nil
	}
	promos, err := s.promotionsRepo.ListByRestaurant(ctx, restaurantID)
	if err != nil {
		return subtotal, 0, applied, lineDiscounts, err
	}
	var previousOrders int
	if customerID != "" {
		previousOrders, err = s.repo.CountByCustomerAndRestaurant(ctx, customerID, restaurantID)
		if err != nil {
			return subtotal, 0, applied, lineDiscounts, err
		}
	}
	lines := make([]promotions.CartLine, len(items))
	for i, it := range items {
		lines[i] = promotions.CartLine{
			ProductID: it.ProductID, Category: it.Category, UnitPriceTiyin: it.PriceTiyin, Qty: it.Qty,
		}
	}
	// ---- Chegirma: HAR QATOR o'zining eng yaxshisini oladi ----
	//
	// Tizimda ikkita mustaqil chegirma mexanizmi bor:
	//   1) mahsulotning o'z chegirma narxi (`DiscountPriceTiyin`) —
	//      restoran panelidagi eski maydon, endi faqat eski yozuvlarda;
	//   2) aksiyalar (`promotions.Apply`).
	//
	// Mahsulot chegirmasi har qator uchun "kafolatlangan minimum"
	// (baseline) bo'lib beriladi, aksiyalar esa faqat undan foydaliroq
	// qatorlarda yutadi. Bitta qatorda ikkitasi hech qachon
	// QO'SHILMAYDI, lekin turli qatorlar turli manbadan chegirma oladi —
	// batafsil izoh `promotions.Apply` da.
	productLineDiscounts := make([]int64, len(items))
	for i, it := range items {
		if it.DiscountPriceTiyin > 0 && it.DiscountPriceTiyin < it.PriceTiyin {
			productLineDiscounts[i] = (it.PriceTiyin - it.DiscountPriceTiyin) * int64(it.Qty)
		}
	}

	applied = promotions.Apply(promos, lines, productLineDiscounts, previousOrders, s.now())
	discount = applied.DiscountTiyin
	copy(lineDiscounts, applied.LineDiscounts)

	// Oxirgi himoya: chegirma hech qachon jamidan oshmaydi. `capDiscount`
	// buni aksiya uchun allaqachon qiladi, lekin mahsulot chegirmasi
	// boshqa yo'ldan keladi — invariant bitta joyda kafolatlanishi kerak.
	if discount > subtotal {
		discount = subtotal
		lineDiscounts = promotions.Spread(discount, lineDiscounts)
	}
	return subtotal, discount, applied, lineDiscounts, nil
}

// QuoteResult — mijoz ilovasi checkout'dan OLDIN (masalan savatga
// mahsulot qo'shilganda/o'chirilganda) chaqiradigan HAQIQIY narxlash
// natijasi — real buyurtma yaratmaydi, faqat oldindan ko'rsatadi.
type QuoteResult struct {
	SubtotalTiyin int64 `json:"subtotal_tiyin"`
	DiscountTiyin int64 `json:"discount_tiyin"`
	TotalTiyin    int64 `json:"total_tiyin"`
	// PromotionID/PromotionName — ENG KO'P hissa qo'shgan aksiya. Savatga
	// bir vaqtda bir nechta aksiya tushishi mumkin (har qator o'zining
	// eng yaxshisini oladi), to'liq ro'yxat — `Promotions`.
	PromotionID   string `json:"promotion_id,omitempty"`
	PromotionName string `json:"promotion_name,omitempty"`
	// PromotionDiscountTiyin — AYNAN SHU (asosiy) aksiya bergan summa.
	//
	// Klient hisob qatorini shunga qarab nomlaydi: shu bitta aksiya
	// chegirmaning HAMMASINI bergan bo'lsa "Aksiya: <nom>", aks holda
	// (bir nechta aksiya yoki mahsulot chegirmasi aralashgan) oddiy
	// "Chegirma" — aks holda boshqa manbalardan kelgan summa ham bitta
	// aksiya nomi ostida ko'rsatilib, mijozga noto'g'ri ma'lumot
	// berilardi.
	PromotionDiscountTiyin int64 `json:"promotion_discount_tiyin,omitempty"`
	// Promotions — savatga tushgan BARCHA aksiyalar va har birining
	// hissasi (hissasi bo'yicha kamayish tartibida).
	Promotions []QuotePromotion `json:"promotions,omitempty"`
	// Lines — HAR BIR savat qatorining yakuniy narxi. Klient savat va
	// rasmiylashtirish ekranlarida AYNAN shu qiymatlarni chizadi:
	// `sum(Lines[].TotalTiyin) == TotalTiyin` (invariant).
	//
	// Avval qator narxlari klientda alohida taxmin qilinardi va
	// serverning jamiga mos kelmasdi — savat ekranida qatorlar
	// yig'indisi bilan pastdagi jami har xil bo'lardi.
	Lines []QuoteLine `json:"lines"`
}

// QuotePromotion — savatga qo'llangan bitta aksiya va uning hissasi.
type QuotePromotion struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DiscountTiyin int64  `json:"discount_tiyin"`
}

// QuoteLine — savatdagi bitta taom uchun yakuniy hisob.
type QuoteLine struct {
	ProductID      string `json:"product_id"`
	Qty            int    `json:"qty"`
	UnitPriceTiyin int64  `json:"unit_price_tiyin"` // ASL (chizilgan) narx
	// SubtotalTiyin — UnitPriceTiyin * Qty (chegirmasiz).
	SubtotalTiyin int64 `json:"subtotal_tiyin"`
	// DiscountTiyin — shu qatorga tegishli JAMI chegirma.
	DiscountTiyin int64 `json:"discount_tiyin"`
	// TotalTiyin — SubtotalTiyin - DiscountTiyin, ya'ni mijoz shu taom
	// uchun haqiqatda to'laydigan summa.
	TotalTiyin int64 `json:"total_tiyin"`
}

// Quote — priceCart'ning ochiq (public) o'rovi, HTTP handler uchun.
func (s *Service) Quote(ctx context.Context, restaurantID string, items []Item, customerID string) (*QuoteResult, error) {
	subtotal, discount, applied, lineDiscounts, err := s.priceCart(ctx, restaurantID, items, customerID)
	if err != nil {
		return nil, err
	}
	res := &QuoteResult{
		SubtotalTiyin: subtotal,
		DiscountTiyin: discount,
		TotalTiyin:    subtotal - discount,
		Lines:         make([]QuoteLine, 0, len(items)),
	}
	for i, it := range items {
		lineSubtotal := it.PriceTiyin * int64(it.Qty)
		res.Lines = append(res.Lines, QuoteLine{
			ProductID:      it.ProductID,
			Qty:            it.Qty,
			UnitPriceTiyin: it.PriceTiyin,
			SubtotalTiyin:  lineSubtotal,
			DiscountTiyin:  lineDiscounts[i],
			TotalTiyin:     lineSubtotal - lineDiscounts[i],
		})
	}
	for _, ap := range applied.Promotions {
		res.Promotions = append(res.Promotions, QuotePromotion{
			ID: ap.Promotion.ID, Name: ap.Promotion.Name, DiscountTiyin: ap.DiscountTiyin,
		})
	}
	if primary := applied.Primary(); primary != nil {
		res.PromotionID = primary.Promotion.ID
		res.PromotionName = primary.Promotion.Name
		res.PromotionDiscountTiyin = primary.DiscountTiyin
	}
	return res, nil
}

func (s *Service) Create(ctx context.Context, o *Order) (*Order, error) {
	return s.CreateExpecting(ctx, o, 0)
}

// CreateExpecting — Create, lekin mijozga KO'RSATILGAN summani ham
// tekshiradi.
//
// `expectedTotalTiyin` > 0 bo'lsa va hisoblangan jami undan farq qilsa,
// buyurtma YARATILMAYDI va `ErrTotalChanged` qaytadi. 0 berilsa
// tekshiruv yo'q — `Create` aynan shunday chaqiradi.
//
// Tekshiruv narx hisoblangandan KEYIN, `Save` dan OLDIN turadi: shu
// sababli mos kelmagan holatda bazada hech qanday iz qolmaydi.
//
// Idempotentlik tekshiruvi bundan OLDIN ishlaydi — ya'ni allaqachon
// yaratilgan buyurtma narx o'zgargani uchun "yo'qolib qolmaydi",
// eskisi o'z holicha qaytariladi.
func (s *Service) CreateExpecting(ctx context.Context, o *Order,
	expectedTotalTiyin int64) (*Order, error) {

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

	subtotal, discount, applied, _, err := s.priceCart(ctx, o.RestaurantID, o.Items, o.CustomerID)
	if err != nil {
		return nil, err
	}
	o.SubtotalTiyin = subtotal
	o.DiscountTiyin = discount
	o.TotalTiyin = subtotal - discount
	// Buyurtma yozuvida ENG KO'P hissa qo'shgan aksiya saqlanadi (bir
	// nechta aksiya tushgan bo'lsa ham) va uning AYNAN O'ZI bergan summa
	// — chek shu ikkisiga qarab rostgo'y yoziladi: nom faqat chegirmaning
	// hammasi o'sha aksiyadan bo'lganda ko'rsatiladi.
	if primary := applied.Primary(); primary != nil {
		o.PromotionID = primary.Promotion.ID
		o.PromotionName = primary.Promotion.Name
		o.PromotionDiscountTiyin = primary.DiscountTiyin
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

	// Mijoz ko'rgan summa hamon o'z kuchidami. Farq bo'lsa buyurtma
	// yaratilmaydi — chaqiruvchi yangi summani ko'rsatib qayta
	// so'raydi (`ErrTotalChanged` matnida ikkala raqam ham bor).
	if expectedTotalTiyin > 0 && o.TotalTiyin != expectedTotalTiyin {
		return nil, &TotalChangedError{
			ExpectedTiyin: expectedTotalTiyin,
			ActualTiyin:   o.TotalTiyin,
		}
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
	// HAR BIR qo'llangan aksiya alohida hisoblanadi va har biriga AYNAN
	// O'ZI bergan summa yoziladi — savatning jami chegirmasi emas (aks
	// holda mahsulot chegirmalari va boshqa aksiyalarning ulushi ham shu
	// aksiya "savdosi" bo'lib ko'rinardi).
	if s.promotionsRepo != nil {
		for _, ap := range applied.Promotions {
			if err := s.promotionsRepo.IncrementUsage(ctx, ap.Promotion.ID, ap.DiscountTiyin); err != nil {
				slog.Error("aksiya statistikasini oshirib bo'lmadi",
					"promotion_id", ap.Promotion.ID, "error", err)
			}
		}
	}
	// ┌─ TO'LANMAGAN BUYURTMA OSHXONAGA TUSHMAYDI ────────────────────┐
	// Karta to'lovida restoran buyurtmani pul BLOKLANGANDAN keyingina
	// ko'radi (`OnPaymentHeld` bildirishnomani o'sha yerda yuboradi).
	// Aks holda mijoz to'lamay chiqib ketsa, oshxona taomni allaqachon
	// tayyorlab qo'ygan bo'lardi.
	// └───────────────────────────────────────────────────────────────┘
	if o.AwaitingPayment() {
		slog.Info("buyurtma to'lov kutmoqda — restoranga hali yuborilmadi",
			"order", o.ID, "total_tiyin", o.TotalTiyin)
		return o, nil
	}
	if s.notifier != nil {
		s.notifier.OrderCreated(o)
	}
	return o, nil
}

// ═══════════════════════════════════════════════════════════════════
// TO'LOV BILAN BOG'LIQ AMALLAR (payments.OrderSink)
// ═══════════════════════════════════════════════════════════════════

// OrderAmountTiyin — to'lanadigan summa AYNAN buyurtmadan olinadi
// (klient yuborgan qiymatga hech qachon ishonilmaydi).
func (s *Service) OrderAmountTiyin(ctx context.Context, orderID string) (int64, error) {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return 0, err
	}
	return o.TotalTiyin, nil
}

// OnPaymentHeld — pul bloklandi (yoki yechildi). Buyurtma endi
// restoranga ko'rinadi va bildirishnoma AYNAN shu yerda yuboriladi.
//
// Idempotent: to'lov tizimi xabarni takrorlashi mumkin, lekin
// bildirishnoma faqat BIR MARTA (holat haqiqatan o'zgarganda) ketadi.
func (s *Service) OnPaymentHeld(ctx context.Context, orderID string) error {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if o.PaymentState.Settled() {
		return nil // allaqachon qayd etilgan
	}
	o.PaymentState = PaymentHeld
	o.UpdatedAt = s.now()
	if err := s.repo.Save(ctx, o); err != nil {
		return err
	}
	if s.notifier != nil {
		s.notifier.OrderCreated(o)
	}
	slog.Info("to'lov bloklandi — buyurtma restoranga yuborildi", "order", o.ID)
	return nil
}

// OnPaymentFailed — to'lov amalga oshmadi yoki muddati tugadi.
// Buyurtma bekor qilinadi: u hech qachon oshxonaga tushmagan, shuning
// uchun hech kim zarar ko'rmaydi.
func (s *Service) OnPaymentFailed(ctx context.Context, orderID string) error {
	o, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if o.PaymentState.Settled() || o.IsTerminal() {
		return nil
	}
	o.PaymentState = PaymentFailed
	o.Status = StatusCancelled
	o.UpdatedAt = s.now()
	o.History = append(o.History, StatusChange{
		From: StatusCreated, To: StatusCancelled, By: ActorSystem, At: o.UpdatedAt,
	})
	if err := s.repo.Save(ctx, o); err != nil {
		return err
	}
	slog.Info("to'lov amalga oshmadi — buyurtma bekor qilindi", "order", o.ID)
	return nil
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
		// ┌─ TO'LANMAGAN BUYURTMA HARAKATLANMAYDI ────────────────────┐
		// Bu — karta to'lovining YAGONA chokepoint'i. Holat
		// o'zgartirishning hamma yo'li shu funksiyadan o'tgani uchun,
		// kelajakda yangi handler qo'shilsa ham to'lanmagan buyurtmani
		// oshxonaga surib yubora olmaydi.
		//
		// Bekor qilish ISTISNO: mijoz fikridan qaytsa yoki to'lov
		// muddati tugasa, buyurtma yopilishi kerak.
		// └───────────────────────────────────────────────────────────┘
		if o.AwaitingPayment() && to != StatusCancelled {
			return nil, fmt.Errorf("buyurtma to'lovi hali tasdiqlanmagan (%s)", o.PaymentState)
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
		s.settlePayment(ctx, o, to)
		if s.notifier != nil {
			s.notifier.OrderStatusChanged(o, from)
		}
		return o, nil
	}
	return nil, ErrConflict
}

// settlePayment — karta to'lovini buyurtma holatiga moslaydi:
//
//	qabul qilindi        -> bloklangan pul YECHILADI (capture)
//	rad etildi/bekor     -> blok BO'SHATILADI (yoki qaytariladi)
//
// Xato buyurtma o'tishini BLOKLAMAYDI (holat allaqachon saqlangan) —
// u log'ga yoziladi. Sabab: restoran "Qabul qildim" bosganda to'lov
// provayderining vaqtincha nosozligi tufayli oshxona to'xtab qolmasligi
// kerak; yechilmagan blok esa keyin qo'lda yoki takroriy urinishda
// tugallanadi va 30 kundan keyin AVTOMATIK bo'shaydi (pul mijozda).
func (s *Service) settlePayment(ctx context.Context, o *Order, to Status) {
	if s.payments == nil || !o.PaymentMethod.RequiresPrepayment() {
		return
	}
	switch to {
	case StatusAccepted:
		if o.PaymentState != PaymentHeld {
			return
		}
		if err := s.payments.CaptureForOrder(ctx, o.ID, o.TotalTiyin); err != nil {
			slog.Error("to'lovni yechib bo'lmadi", "order", o.ID, "error", err)
			return
		}
		// PUL ALLAQACHON YECHILGAN — holat SAQLANISHI shart
		// (bug.md 35-band).
		s.persistPaymentState(ctx, o, PaymentPaid)
	case StatusRejected, StatusCancelled:
		if !o.PaymentState.Settled() {
			return
		}
		if err := s.payments.ReleaseForOrder(ctx, o.ID); err != nil {
			slog.Error("to'lovni bo'shatib bo'lmadi", "order", o.ID, "error", err)
			return
		}
		next := PaymentFailed
		if o.PaymentState == PaymentPaid {
			next = PaymentRefunded
		}
		s.persistPaymentState(ctx, o, next)
	}
}

// persistPaymentState — to'lov holatini QAT'IY saqlaydi.
//
// ┌─ TUZATILGAN NOSOZLIK (bug.md 35-band) ─────────────────────────────┐
// Avval bu joyda oddiy `s.repo.Save(ctx, o)` turardi va xato FAQAT
// logga yozilardi. `Save` esa optimistik qulf bilan ishlaydi, ya'ni
// boshqa so'rov bizdan oldin yozib ulgursa `ErrConflict` qaytaradi —
// qayta urinish esa YO'Q edi.
//
// Natija: PROVAYDERDA PUL YECHILGAN, bazada esa `PaymentState`
// `held` bo'lib qolardi. Buyurtma keyin bekor qilinsa,
// `ReleaseForOrder` `StatusPaid` emas, `StatusHeld` yo'lidan borib
// ALLAQACHON YECHILGAN to'lovni "bo'shatishga" urinardi.
//
// Endi konfliktda buyurtma QAYTA O'QILADI va holat yangi versiyaga
// qo'yiladi (loyihaning boshqa joylaridagi `maxOptimisticRetries`
// naqshi). Hamma urinish yiqilsa — `slog.Error` "QO'LDA tekshirish
// kerak" izohi bilan: bu pul masalasi, jimgina qolishi mumkin emas.
//
// Kontekst ATAYLAB yangi: chaqiruvchi HTTP so'rovi tugagan bo'lishi
// mumkin, holat esa baribir yozilishi shart.
// └────────────────────────────────────────────────────────────────────┘
func (s *Service) persistPaymentState(ctx context.Context, o *Order, state PaymentState) {
	if err := ctx.Err(); err != nil {
		// So'rov konteksti allaqachon bekor qilingan — o'z muddatimiz.
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
	}

	o.PaymentState = state
	for attempt := 0; attempt < maxOptimisticRetries; attempt++ {
		err := s.repo.Save(ctx, o)
		if err == nil {
			return
		}
		if !errors.Is(err, ErrConflict) {
			slog.Error("to'lov holatini saqlab bo'lmadi (qayta urinilmaydi)",
				"order", o.ID, "holat", state, "error", err)
			return
		}
		// Konflikt — boshqa so'rov yozib ulgurdi. Yangi versiyani
		// o'qib, holatni QAYTA qo'yamiz.
		fresh, getErr := s.repo.GetByID(ctx, o.ID)
		if getErr != nil {
			slog.Error("to'lov holatini saqlash uchun buyurtma qayta o'qilmadi",
				"order", o.ID, "holat", state, "error", getErr)
			return
		}
		fresh.PaymentState = state
		*o = *fresh
	}
	slog.Error("PUL HARAKATLANDI, LEKIN HOLAT SAQLANMADI — QO'LDA tekshirish kerak",
		"order", o.ID, "kutilgan_holat", state, "urinishlar", maxOptimisticRetries)
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
