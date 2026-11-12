package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/payments/mcconnect"
	"chustapp/internal/payments/multicard"
	"chustapp/internal/users"
)

// RESTORAN MOLIYA BO'LIMI.
//
// ┌─ MANBA — TO'LOV SHLYUZI, BIZNING BAZA EMAS ───────────────────────┐
// Tushum raqamlari Multicard reestridan olinadi
// (`GET /payment/store/{id}/history`), bizning buyurtmalar
// jadvalidan emas. Sabab: restoran uchun HAQIQIY pul — hisobiga
// tushgan pul. Bizning bazadagi "bajarilgan buyurtma" esa to'lov
// muvaffaqiyatli o'tganini kafolatlamaydi (naqd, bekor qilingan
// to'lov, qaytarish).
//
// Ikkalasi farq qilsa, restoran shlyuz raqamiga ishonishi kerak —
// uning bankdagi puli o'sha.
// └───────────────────────────────────────────────────────────────────┘
//
// ┌─ NIMA KO'RSATILMAYDI ─────────────────────────────────────────────┐
// **Bank hisobidagi QOLDIQ** — Multicard uni bermaydi va bizda
// restoranning bank hisobiga kirish yo'q. "Hisobingizda shuncha pul
// bor" deb yozish yolg'on bo'lardi; faqat "shu davrda shuncha tushdi"
// deyish mumkin.
//
// **Xarajatlar va sof foyda** — tizimda xarajat tushunchasi yo'q.
// Ular uchun alohida daftar kerak; o'ylab topilgan raqam chizilmaydi.
// └───────────────────────────────────────────────────────────────────┘

// financeMaxDays — bitta so'rovdagi eng uzun davr.
//
// Chegara Multicard tomonidagi yukni ham, bizning javob hajmini ham
// cheklaydi: 366 kun bir yillik hisobotga yetadi.
const financeMaxDays = 366

// financeRecentLimit — "So'nggi operatsiyalar" jadvalidagi qatorlar.
const financeRecentLimit = 20

// financeTxn — mijozga ko'rsatiladigan bitta operatsiya.
//
// ⚠️ `card_pan` ATAYLAB niqoblangan holicha uzatiladi (Multicard
// shunday beradi: `860030******5959`). To'liq raqam hech qachon
// panelga chiqmaydi.
type financeTxn struct {
	UUID string `json:"uuid"`
	// Status — `success` dan boshqasi pul tushganini BILDIRMAYDI.
	Status string `json:"status"`
	// PS — to'lov tizimi (bo'sh bo'lishi mumkin).
	PS              string `json:"ps"`
	OrderRef        string `json:"order_ref"`
	AmountTiyin     int64  `json:"amount_tiyin"`
	CommissionTiyin int64  `json:"commission_tiyin"`
	At              string `json:"at"`
	RefundedAt      string `json:"refunded_at,omitempty"`
}

// financeResponse — `GET /restaurants/{id}/finance`.
type financeResponse struct {
	// Connected — to'lov shlyuzi ulanganmi. `false` bo'lsa qolgan
	// maydonlar bo'sh va panel ULASH formasini ko'rsatadi.
	Connected bool   `json:"connected"`
	StoreID   string `json:"store_id,omitempty"`
	// Available — restoran ulana OLADIMI (shlyuz manzili va sirlarni
	// shifrlash kaliti sozlanganmi). `false` bo'lsa restoran o'zi hech
	// narsa qila olmaydi — bu OnDex tomonidagi sozlama.
	Available bool `json:"available"`

	// OwnCredentials — restoran O'Z shartnomasi bilan ulanganmi.
	// `false` + `connected` — OnDex shartnomasi ostida ishlayapti.
	OwnCredentials bool `json:"own_credentials"`
	// AppID — restoranning `application_id` si (sir EMAS: login kabi).
	// ⚠️ `secret` bu javobda ham, boshqa hech qayerda ham bo'lmaydi.
	AppID string `json:"app_id,omitempty"`
	// StoreTitle/AccountMasked — ulash paytida shlyuzdan olingan:
	// "nimaga ulangansiz" degan savolga so'rovsiz javob.
	StoreTitle    string `json:"store_title,omitempty"`
	AccountMasked string `json:"account_masked,omitempty"`

	From string `json:"from"`
	To   string `json:"to"`

	// TurnoverTiyin — FAQAT muvaffaqiyatli to'lovlar.
	TurnoverTiyin int64 `json:"turnover_tiyin"`
	// RefundedTiyin — qaytarilgan summa (`revert`).
	RefundedTiyin int64 `json:"refunded_tiyin"`
	// CommissionTiyin — shlyuz ushlagan komissiya (ko'rsatilgan
	// operatsiyalar bo'yicha; foiz shartnomaga bog'liq).
	CommissionTiyin int64 `json:"commission_tiyin"`
	// NetTiyin — TurnoverTiyin − RefundedTiyin. Bu "sof foyda" EMAS:
	// xarajatlar hisobga olinmagan (ular tizimda yo'q).
	NetTiyin int64 `json:"net_tiyin"`
	// Transactions — muvaffaqiyatli to'lovlar soni.
	Transactions int `json:"transactions"`

	// PrevTurnoverTiyin — xuddi shuncha kunlik OLDINGI davr, foiz
	// o'zgarishini ko'rsatish uchun.
	PrevTurnoverTiyin int64 `json:"prev_turnover_tiyin"`
	PrevTransactions  int   `json:"prev_transactions"`

	// Daily — kunlik tushum (grafik uchun), sana bo'yicha o'sish
	// tartibida.
	Daily []financeDay `json:"daily"`
	// ByMethod — to'lov tizimi bo'yicha taqsimot (faqat `success`).
	ByMethod []financeMethod `json:"by_method"`
	// Recent — so'nggi operatsiyalar (hamma holat).
	Recent []financeTxn `json:"recent"`
}

type financeDay struct {
	Date        string `json:"date"`
	AmountTiyin int64  `json:"amount_tiyin"`
	Count       int    `json:"count"`
}

type financeMethod struct {
	PS          string `json:"ps"`
	AmountTiyin int64  `json:"amount_tiyin"`
	Count       int    `json:"count"`
}

// tashkent — hisobotlar restoranning mahalliy kunida bo'linadi.
var tashkent = time.FixedZone("UTC+5", 5*60*60)

// parseFinanceRange — `from`/`to` (YYYY-MM-DD) yoki standart 7 kun.
func parseFinanceRange(r *http.Request) (from, to time.Time, err error) {
	now := time.Now().In(tashkent)
	day := func(t time.Time) time.Time {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tashkent)
	}
	to = day(now)
	from = to.AddDate(0, 0, -6)

	q := r.URL.Query()
	if s := strings.TrimSpace(q.Get("from")); s != "" {
		t, e := time.ParseInLocation("2006-01-02", s, tashkent)
		if e != nil {
			return from, to, errors.New("from — YYYY-MM-DD shaklida bo'lishi kerak")
		}
		from = t
	}
	if s := strings.TrimSpace(q.Get("to")); s != "" {
		t, e := time.ParseInLocation("2006-01-02", s, tashkent)
		if e != nil {
			return from, to, errors.New("to — YYYY-MM-DD shaklida bo'lishi kerak")
		}
		to = t
	}
	if to.Before(from) {
		return from, to, errors.New("to sanasi from dan oldin bo'lishi mumkin emas")
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > financeMaxDays {
		return from, to, errors.New("davr juda uzun — eng ko'pi bilan bir yil")
	}
	return from, to, nil
}

// dayEnd — kunning oxirgi soniyasi (reestr yopiq oraliq kutadi).
func dayEnd(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, tashkent)
}

// financeRestaurant — yo'ldagi restoranni oladi va EGALIK tekshiradi.
//
// Tartib muhim: egalik tekshiruvi restoran mavjudligidan ham, so'rov
// tanasidan ham OLDIN — aks holda javob "bu restoran bor" degan
// ma'lumotni oshkor qilardi (`TestTenancyMatrix` dagi qoida).
func (s *Server) financeRestaurant(w http.ResponseWriter, r *http.Request) (*catalog.Restaurant, bool) {
	restaurantID := r.PathValue("id")
	claims := claimsFrom(r)
	if claims.Role == users.RoleRestaurant && claims.EntityID != restaurantID {
		httpError(w, http.StatusForbidden,
			errors.New("boshqa restoran moliyasini ko'rib bo'lmaydi"))
		return nil, false
	}
	rest, err := s.CatalogRepo.GetRestaurant(r.Context(), restaurantID)
	if err != nil {
		httpError(w, http.StatusNotFound, errors.New("restoran topilmadi"))
		return nil, false
	}
	return rest, true
}

// maskAccount — hisob raqamining oxirgi to'rt raqami.
//
// To'liq raqam SAQLANMAYDI ham, LOGGA yozilmaydi ham: u bizga kerak
// emas, restoran esa oxirgi raqamlar bilan to'g'ri hisobga ulanganini
// tekshira oladi.
func maskAccount(no string) string {
	no = strings.TrimSpace(no)
	if no == "" {
		return ""
	}
	if len(no) <= 4 {
		return strings.Repeat("•", len(no))
	}
	return "••••" + no[len(no)-4:]
}

// writeFinanceConnectError — ulash paytidagi xato.
//
// Shlyuz 4xx bersa sabab foydalanuvchida: kalitlar yoki kassa raqami.
// Ikkalasini AJRATIB aytish mumkin emas (shlyuz `/auth` dan keyin
// ham, kassa so'rovida ham 4xx beradi), shuning uchun matn ikkalasini
// ham tekshirishni aytadi — "noma'lum xato" dan foydali.
func writeFinanceConnectError(w http.ResponseWriter, storeID string, err error) {
	// ⚠️ ASL XATONI YOZIB QO'YAMIZ. `httpError` 5xx javoblarida
	// tafsilotni yashiradi va faqat BIZNING umumiy matnimizni logga
	// tushiradi — ya'ni "dial tcp: i/o timeout" kabi haqiqiy sabab
	// butunlay yo'qolardi. Aynan shu sababdan shlyuz sinov stendi
	// ishdan chiqqanda log "to'lov tizimiga ulanib bo'lmadi" dan
	// boshqa hech narsa ko'rsatmadi.
	slog.Error("multicard: ulash xatosi", "store", storeID, "err", err)
	if errors.Is(err, multicard.ErrAuth) {
		httpError(w, http.StatusUnprocessableEntity, errors.New(
			"application_id yoki secret noto'g'ri — to'lov provayderi bergan "+
				"kalitlarni tekshiring"))
		return
	}
	var apiErr *multicard.APIError
	if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
		httpError(w, http.StatusUnprocessableEntity, fmt.Errorf(
			"kalitlar qabul qilindi, lekin %q kassasi topilmadi — raqamni tekshiring",
			storeID))
		return
	}
	httpError(w, http.StatusBadGateway,
		errors.New("to'lov tizimiga ulanib bo'lmadi — keyinroq urinib ko'ring"))
}

// canConnectFinance — restoran O'Z kalitlari bilan ulana oladimi.
func (s *Server) canConnectFinance() bool { return s.MC.CanConnect() }

// mcCredentials — katalogdagi restorandan ulanish ma'lumoti.
//
// `mcconnect` paketi `catalog` ni bilmaydi (to'lov qatlami katalog
// modelidan mustaqil bo'lishi uchun), shuning uchun o'girish shu
// yerda — HTTP qatlamida.
func mcCredentials(rest *catalog.Restaurant) mcconnect.Credentials {
	if rest == nil {
		return mcconnect.Credentials{}
	}
	return mcconnect.Credentials{
		AppID:          rest.MulticardAppID,
		SecretEnc:      rest.MulticardSecretEnc,
		StoreID:        rest.MulticardStoreID,
		TaxMXIK:        rest.MulticardTaxMXIK,
		TaxPackageCode: rest.MulticardTaxPackageCode,
	}
}

// multicardFor — shu restoran uchun ishlatiladigan shlyuz klienti.
//
// Ikkalasi ham yo'q bo'lsa `nil, nil`: bu XATO emas, "ulanmagan".
// Kalitlar bor-u ochib bo'lmasa — XATO (kalit almashtirilgan).
func (s *Server) multicardFor(rest *catalog.Restaurant) (*multicard.Client, error) {
	c, err := s.MC.Client(mcCredentials(rest))
	if err != nil && rest != nil {
		slog.Error("to'lov kalitini ochib bo'lmadi",
			"restaurant", rest.ID, "app_id", rest.MulticardAppID, "err", err)
	}
	return c, err
}

// financeConnectRequest — ulash formasi.
type financeConnectRequest struct {
	ApplicationID string `json:"application_id"`
	Secret        string `json:"secret"`
	StoreID       string `json:"store_id"`
}

// financeConnectResponse — ulash natijasi.
//
// ⚠️ Kiritilgan `secret` QAYTARILMAYDI. Restoran to'g'ri kalit
// kiritganini shlyuzdan kelgan ma'lumot bilan tekshiradi: kassa nomi,
// tashkilot, STIR va PUL TUSHADIGAN HISOB.
type financeConnectResponse struct {
	StoreID       string `json:"store_id"`
	StoreTitle    string `json:"store_title"`
	MerchantName  string `json:"merchant_name"`
	MerchantTIN   string `json:"merchant_tin"`
	AccountMasked string `json:"account_masked"`
	ContractNum   string `json:"contract_num"`
	// FeePercent — shlyuz komissiyasi (shartnomaga bog'liq, kodda
	// qadalmaydi). Bo'sh bo'lishi mumkin.
	FeePercent string `json:"fee_percent"`
}

func (s *Server) registerFinanceRoutes(mux *http.ServeMux) {
	roles := []users.Role{users.RoleRestaurant, users.RoleAdmin}

	// POST /restaurants/{id}/finance/connect
	//
	// ┌─ RESTORAN O'Z HISOBINI ULAYDI ─────────────────────────────────┐
	// Multicard'da do'kon/merchant YARATISH API'si yo'q (hujjatning 59
	// sahifasi tekshirildi): restoran shartnomani o'zi tuzadi va
	// `application_id` + `secret` + kassa raqamini oladi. Bu forma
	// aynan shularni qabul qiladi.
	//
	// Saqlashdan OLDIN ikki tekshiruv (ikkalasi ham shart):
	//   1. `POST /auth` — kalitlar haqiqiymi;
	//   2. `GET /payment/store/{id}` — kassa shu kalitlarga tegishlimi.
	// Faqat birinchisi bo'lsa, xato kassa raqami keyinroq, Moliya
	// sahifasi ochilganda bilinardi (aynan shunday bo'lgan edi).
	//
	// Javob PUL QAYERGA tushishini ko'rsatadi — restoran ulashni
	// tasdiqlashdan oldin o'z rekvizitlarini ko'rishi kerak.
	// └────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("POST /restaurants/{id}/finance/connect", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := s.financeRestaurant(w, r)
			if !ok {
				return
			}
			var req financeConnectRequest
			if !decodeJSON(w, r, &req) {
				return
			}
			appID := strings.TrimSpace(req.ApplicationID)
			secret := strings.TrimSpace(req.Secret)
			storeID := strings.TrimSpace(req.StoreID)

			if err := catalog.ValidateStoreID(storeID); err != nil || storeID == "" {
				httpError(w, http.StatusBadRequest,
					errors.New("kassa (do'kon) raqamini kiriting"))
				return
			}
			if err := catalog.ValidateAppID(appID); err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}
			if secret == "" || len(secret) > catalog.MaxSecretLen {
				httpError(w, http.StatusBadRequest,
					errors.New("secret kiritilmagan yoki juda uzun"))
				return
			}
			if !s.canConnectFinance() {
				httpError(w, http.StatusServiceUnavailable, errors.New(
					"to'lov tizimiga ulanish OnDex tomonida hali sozlanmagan — "+
						"OnDex bilan bog'laning"))
				return
			}

			client, err := s.MC.NewClient(appID, secret)
			if err != nil {
				httpError(w, http.StatusBadRequest, errors.New("kalitlar to'liq emas"))
				return
			}
			// Tekshiruv: kalitlar + kassa. Xatoni bir xil yo'l bilan
			// ishlaymiz (4xx — foydalanuvchi xatosi, 5xx — shlyuz).
			info, err := client.Store(r.Context(), storeID)
			if err != nil {
				writeFinanceConnectError(w, storeID, err)
				return
			}

			enc, err := s.MC.Seal(secret)
			if err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			claims := claimsFrom(r)
			// ⚠️ AUDIT: pul manzili o'zgaryapti. Eski va yangi
			// qiymatlar yoziladi — `secret` esa HECH QACHON.
			slog.Warn("to'lov hisobi ulandi",
				"restaurant", rest.ID,
				"eski_app", rest.MulticardAppID, "eski_store", rest.MulticardStoreID,
				"yangi_app", appID, "yangi_store", storeID,
				"hisob", maskAccount(info.MerchantAccount.AccountNo),
				"by", claims.Subject, "role", claims.Role)

			rest.MulticardAppID = appID
			rest.MulticardSecretEnc = enc
			rest.MulticardStoreID = storeID
			rest.MulticardStoreTitle = info.Title
			rest.MulticardAccountMasked = maskAccount(info.MerchantAccount.AccountNo)
			// Fiskal standartlar kassadan olinadi: karta orqali to'lov
			// yaratishda `ofd` majburiy va usiz to'lov ishlamaydi.
			rest.MulticardTaxMXIK = info.TaxMXIK
			rest.MulticardTaxPackageCode = info.TaxPackageCode
			if err := s.CatalogRepo.SaveRestaurant(r.Context(), rest); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			// Eski klient keshda qolib ketmasin: kalit almashgan bo'lsa
			// keyingi so'rov ESKI sir bilan ketardi.
			s.MC.Invalidate(appID)
			s.dropRestaurantsCache(r.Context())

			writeJSON(w, http.StatusOK, financeConnectResponse{
				StoreID:       storeID,
				StoreTitle:    info.Title,
				MerchantName:  info.Merchant.Name,
				MerchantTIN:   info.Merchant.TIN,
				AccountMasked: rest.MulticardAccountMasked,
				ContractNum:   info.Contract.Num.String(),
				FeePercent:    info.Contract.Fee.Down.String(),
			})
		}))

	// DELETE /restaurants/{id}/finance/connect — ulanishni uzish.
	//
	// Kalitlar o'chiriladi, ya'ni OnDex endi bu restoranning to'lov
	// hisobiga umuman kira olmaydi. Restoran o'zi qaytarib ulay oladi.
	mux.HandleFunc("DELETE /restaurants/{id}/finance/connect", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := s.financeRestaurant(w, r)
			if !ok {
				return
			}
			claims := claimsFrom(r)
			slog.Warn("to'lov hisobi uzildi",
				"restaurant", rest.ID, "app", rest.MulticardAppID,
				"store", rest.MulticardStoreID,
				"by", claims.Subject, "role", claims.Role)

			oldApp := rest.MulticardAppID
			rest.MulticardAppID = ""
			rest.MulticardSecretEnc = ""
			rest.MulticardStoreID = ""
			rest.MulticardStoreTitle = ""
			rest.MulticardAccountMasked = ""
			if err := s.CatalogRepo.SaveRestaurant(r.Context(), rest); err != nil {
				httpError(w, http.StatusInternalServerError, err)
				return
			}
			if oldApp != "" {
				s.MC.Invalidate(oldApp)
			}
			s.dropRestaurantsCache(r.Context())
			w.WriteHeader(http.StatusNoContent)
		}))

	// GET /restaurants/{id}/finance?from=YYYY-MM-DD&to=YYYY-MM-DD
	//
	// ┌─ XAVFSIZLIK ───────────────────────────────────────────────────┐
	//  * Faqat restoran xodimi (O'Z restorani) va admin. Affitsiant
	//    ATAYLAB yo'q — tushum moliyaviy ma'lumot (`routes_stats.go`
	//    dagi o'sha qoida).
	//  * Egalik tekshiruvi 400/503 dan OLDIN (`TestTenancyMatrix`).
	//  * Javobda `application_id`/`secret` HECH QACHON bo'lmaydi.
	// └────────────────────────────────────────────────────────────────┘
	mux.HandleFunc("GET /restaurants/{id}/finance", s.auth(roles,
		func(w http.ResponseWriter, r *http.Request) {
			rest, ok := s.financeRestaurant(w, r)
			if !ok {
				return
			}
			from, to, err := parseFinanceRange(r)
			if err != nil {
				httpError(w, http.StatusBadRequest, err)
				return
			}

			out := financeResponse{
				Available:      s.canConnectFinance(),
				StoreID:        rest.MulticardStoreID,
				OwnCredentials: rest.MulticardAppID != "",
				AppID:          rest.MulticardAppID,
				StoreTitle:     rest.MulticardStoreTitle,
				AccountMasked:  rest.MulticardAccountMasked,
				From:           from.Format("2006-01-02"),
				To:             to.Format("2006-01-02"),
				Daily:          []financeDay{},
				ByMethod:       []financeMethod{},
				Recent:         []financeTxn{},
			}
			mc, cerr := s.multicardFor(rest)
			out.Connected = cerr == nil && mc != nil && rest.MulticardStoreID != ""
			if !out.Connected {
				// Ulanmagan holat XATO EMAS: panel ulash formasini
				// ko'rsatadi. 503 qaytarilsa u "nosozlik" bo'lib
				// ko'rinardi.
				//
				// ⚠️ ISTISNO: kalitlar SAQLANGAN, lekin ochib
				// bo'lmayapti (`SECRETS_ENC_KEY` almashtirilgan yoki
				// yo'qolgan). Buni "ulanmagan" deb ko'rsatish yolg'on
				// bo'lardi — restoran qayta ulashga urinib, sababini
				// bilmasdan qolardi.
				if cerr != nil && rest.MulticardAppID != "" {
					httpError(w, http.StatusServiceUnavailable, cerr)
					return
				}
				writeJSON(w, http.StatusOK, out)
				return
			}

			// ┌─ KESH (30 soniya) ──────────────────────────────────────┐
			// Ma'lumot TASHQI shlyuzdan keladi (bizning bazadan emas):
			// 7 kunlik hisobot ~0.8 s, 30 kunlik ~4–5 s oladi, boshqa
			// sahifalar esa 0–70 ms. Sahifaga har kirganda shuncha
			// kutish "faqat Moliya aylanyapti" degan sezgini beradi.
			//
			// Kalit restoran, KALITLAR va kassani o'z ichiga oladi:
			// restoran hisobni almashtirsa eski hisobning raqamlari
			// ko'rinib qolmasligi uchun (ulash/uzish alohida kalit
			// beradi, eski yozuv o'zi 30 soniyada o'ladi).
			//
			// Faqat MUVAFFAQIYATLI javob keshlanadi — shlyuz xatosi
			// keshga tushsa, u tiklangandan keyin ham 30 soniya xato
			// ko'rinardi.
			// └──────────────────────────────────────────────────────────┘
			cacheKey := fmt.Sprintf("cache:finance:%s:%s:%s:%s:%s",
				rest.ID, rest.MulticardAppID, rest.MulticardStoreID, out.From, out.To)
			var cached financeResponse
			if s.Cache.GetJSON(r.Context(), cacheKey, &cached) {
				writeJSON(w, http.StatusOK, cached)
				return
			}

			days := int(to.Sub(from).Hours()/24) + 1
			// Oldingi davr — XUDDI shuncha kun, foiz o'zgarishi uchun.
			// U joriy davr BILAN BIR VAQTDA olinadi: ketma-ket bo'lsa
			// kutish ikki barobar bo'lardi.
			prevTo := from.AddDate(0, 0, -1)
			prevCh := make(chan *financeAgg, 1) // bufer: handler erta chiqsa goroutine osilmasin
			go func() {
				p, perr := s.financePeriod(r, mc, rest.MulticardStoreID, prevTo.AddDate(0, 0, -(days-1)), prevTo)
				if perr != nil {
					p = nil
				}
				prevCh <- p
			}()

			cur, err := s.financePeriod(r, mc, rest.MulticardStoreID, from, to)
			if err != nil {
				writeFinanceError(w, rest.MulticardStoreID, err)
				return
			}
			// ⚠️ Oldingi davr xatosi butun sahifani yiqitmaydi: u
			// faqat foiz ko'rsatkichi uchun. Joriy davr esa majburiy.
			if prev := <-prevCh; prev != nil {
				out.PrevTurnoverTiyin = prev.turnover
				out.PrevTransactions = prev.count
			}

			out.TurnoverTiyin = cur.turnover
			out.RefundedTiyin = cur.refunded
			out.CommissionTiyin = cur.commission
			out.NetTiyin = cur.turnover - cur.refunded
			out.Transactions = cur.count
			out.Daily = cur.daily
			out.ByMethod = cur.byMethod
			out.Recent = cur.recent
			s.Cache.SetJSON(r.Context(), cacheKey, out, financeCacheTTL)
			writeJSON(w, http.StatusOK, out)
		}))
}

// financeCacheTTL — hisobot javobining kesh muddati.
//
// 30 soniya: restoran yangi tushgan to'lovni ko'rishi uchun yetarlicha
// qisqa, sahifalar orasida o'tib yurganda esa har safar shlyuzga
// bormaslik uchun yetarlicha uzun.
const financeCacheTTL = 30 * time.Second

// financeAgg — bitta davr bo'yicha yig'ilgan ko'rsatkichlar.
type financeAgg struct {
	turnover   int64
	refunded   int64
	commission int64
	count      int
	daily      []financeDay
	byMethod   []financeMethod
	recent     []financeTxn
}

// financePeriod — reestrni o'qib, davr ko'rsatkichlarini yig'adi.
//
// ┌─ NEGA `stat` EMAS, `list` BO'YICHA ───────────────────────────────┐
// Javobdagi `stat` holat×tizim bo'yicha jamlanma beradi, lekin KUNLIK
// taqsimot bermaydi — grafik uchun yozuvlarning o'zi kerak. Shuning
// uchun sahifalab o'qiymiz va o'zimiz yig'amiz.
// └───────────────────────────────────────────────────────────────────┘
func (s *Server) financePeriod(r *http.Request, mc *multicard.Client, storeID string, from, to time.Time) (*financeAgg, error) {
	agg := &financeAgg{daily: []financeDay{}, byMethod: []financeMethod{}, recent: []financeTxn{}}
	perDay := map[string]*financeDay{}
	perPS := map[string]*financeMethod{}

	txns, err := s.financeTransactions(r.Context(), mc, storeID, from, to)
	if err != nil {
		return nil, err
	}
	{
		for _, t := range txns {
			// So'nggi operatsiyalar — HAMMA holat (restoran bekor
			// qilingan to'lovni ham ko'rishi kerak).
			if len(agg.recent) < financeRecentLimit {
				agg.recent = append(agg.recent, financeTxn{
					UUID: t.UUID, Status: t.Status, PS: t.PS,
					OrderRef:        t.StoreInvoiceID,
					AmountTiyin:     t.AmountTiyin.Int64(),
					CommissionTiyin: t.CommissionTiyin.Int64(),
					At:              firstNonEmpty(t.PaymentTime, t.RefundTime),
					RefundedAt:      t.RefundTime,
				})
			}
			switch t.Status {
			case multicard.StatusSuccess:
				agg.turnover += t.AmountTiyin.Int64()
				agg.commission += t.CommissionTiyin.Int64()
				agg.count++
				if d := dateOf(t.PaymentTime); d != "" {
					x := perDay[d]
					if x == nil {
						x = &financeDay{Date: d}
						perDay[d] = x
					}
					x.AmountTiyin += t.AmountTiyin.Int64()
					x.Count++
				}
				ps := t.PS
				if ps == "" {
					ps = "boshqa"
				}
				m := perPS[ps]
				if m == nil {
					m = &financeMethod{PS: ps}
					perPS[ps] = m
				}
				m.AmountTiyin += t.AmountTiyin.Int64()
				m.Count++
			case multicard.StatusRevert:
				agg.refunded += t.AmountTiyin.Int64()
			}
		}
	}

	// Grafikda BO'SH kunlar ham turishi kerak — aks holda uch kunlik
	// ma'lumot yetti kunlik davrda "uzluksiz" bo'lib ko'rinardi.
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		if x := perDay[key]; x != nil {
			agg.daily = append(agg.daily, *x)
		} else {
			agg.daily = append(agg.daily, financeDay{Date: key})
		}
	}
	for _, m := range perPS {
		agg.byMethod = append(agg.byMethod, *m)
	}
	sort.Slice(agg.byMethod, func(i, j int) bool {
		return agg.byMethod[i].AmountTiyin > agg.byMethod[j].AmountTiyin
	})
	return agg, nil
}

const (
	financePageSize = 200
	// financeMaxPages — himoya chegarasi: reestr kutilmaganda juda katta
	// bo'lsa server cheksiz aylanmasin.
	financeMaxPages = 25
	// financeParallel — bir davr ichida bir vaqtda ketadigan so'rovlar.
	//
	// Chegara SHART: 25 sahifani birdan yuborish shlyuzni bitta mijoz
	// bilan to'ldirib, boshqa restoranlar so'rovini sekinlashtirardi.
	// Joriy va oldingi davr birga ishlagani uchun jami 2×4 = 8.
	financeParallel = 4
)

// financeTransactions — davrdagi barcha yozuvlar, reestr TARTIBIDA.
//
// ┌─ NEGA PARALLEL ───────────────────────────────────────────────────┐
// Reestr sahifalab beriladi (200 tadan). 1455 yozuvli hafta — 8
// sahifa; ketma-ket olinsa har biri o'z kutishini qo'shadi.
//
// Birinchi sahifa ketma-ket olinadi (jami son shundan ma'lum), qolgani
// esa `financeParallel` tadan parallel. Natija sahifa TARTIBIDA
// yig'iladi: "so'nggi operatsiyalar" reestrning birinchi yozuvlari va
// ular tartib bilan turishi kerak.
// └───────────────────────────────────────────────────────────────────┘
func (s *Server) financeTransactions(ctx context.Context, mc *multicard.Client,
	storeID string, from, to time.Time) ([]multicard.Transaction, error) {
	fetch := func(ctx context.Context, page int) (*multicard.History, error) {
		return mc.StoreHistory(ctx, multicard.HistoryQuery{
			StoreID: storeID,
			From:    from,
			To:      dayEnd(to),
			Offset:  page * financePageSize,
			Limit:   financePageSize,
		})
	}

	first, err := fetch(ctx, 0)
	if err != nil {
		return nil, err
	}
	pages := min((first.Pagination.Total+financePageSize-1)/financePageSize, financeMaxPages)
	if pages <= 1 || len(first.List) == 0 {
		return first.List, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([][]multicard.Transaction, pages)
	results[0] = first.List

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		sem      = make(chan struct{}, financeParallel)
	)
	for page := 1; page < pages; page++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(page int) {
			defer wg.Done()
			defer func() { <-sem }()
			h, err := fetch(ctx, page)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
					cancel() // qolganlari behuda kutmasin
				}
				return
			}
			results[page] = h.List
		}(page)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	var all []multicard.Transaction
	for _, list := range results {
		all = append(all, list...)
	}
	return all, nil
}

// dateOf — "2026-09-30 14:36:31" → "2026-09-30".
func dateOf(s string) string {
	if len(s) < 10 {
		return ""
	}
	return s[:10]
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// writeFinanceError — shlyuz xatosini HTTP javobiga aylantiradi.
//
// ┌─ IKKI XIL NOSOZLIK, IKKI XIL JAVOB ───────────────────────────────┐
// **Shlyuz 4xx qaytardi** — u ISHLAYAPTI va so'rovimizni rad etdi.
// Amalda buning yagona sababi bor: do'kon raqami noto'g'ri. Restoran
// buni O'ZI tuzata oladi, ya'ni bu "server nosozligi" emas. Shuning
// uchun 422 va ANIQ matn: qaysi raqam tanilmadi.
//
// Nega bu muhim: sinovda `multicard_store_id` ga "65" kiritildi,
// shlyuz `400 ERROR_FIELDS «Касса» неверно` qaytardi, biz esa 502
// berdik. `httpError` 5xx tafsilotini yashirgani uchun panelda
// "server xatosi, keyinroq urinib ko'ring" chiqdi — foydalanuvchi
// o'z xatosini tuzatishning iloji YO'Q edi va kutishdan boshqa chora
// qolmasdi.
//
// **Shlyuz javob bermadi yoki 5xx berdi** — bu haqiqatan tashqi
// nosozlik: 502. Bu holda tafsilot MIJOZGA BORMAYDI (`httpError` 5xx
// ni yashiradi), faqat logga tushadi — shuning uchun uni yozish
// xavfsiz.
// └───────────────────────────────────────────────────────────────────┘
func writeFinanceError(w http.ResponseWriter, storeID string, err error) {
	// Asl xato — logga (`writeFinanceConnectError` dagi o'sha sabab).
	slog.Error("multicard: reestr xatosi", "store", storeID, "err", err)
	var apiErr *multicard.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Status >= 400 && apiErr.Status < 500 {
			// ⚠️ Shlyuzning O'Z matni javobga QO'YILMAYDI: u ruscha va
			// ichki maydon nomlarini oshkor qiladi. Faqat bizning matn
			// va foydalanuvchining o'zi kiritgan raqam.
			httpError(w, http.StatusUnprocessableEntity, fmt.Errorf(
				"to'lov tizimi %q do'konini tanimadi — raqamni tekshiring "+
					"(uni to'lov provayderi beradi)", storeID))
			return
		}
		httpError(w, http.StatusBadGateway, errors.New(
			"to'lov tizimi javob bermadi: "+apiErr.Details))
		return
	}
	httpError(w, http.StatusBadGateway,
		errors.New("to'lov tizimiga ulanib bo'lmadi — keyinroq urinib ko'ring"))
}
