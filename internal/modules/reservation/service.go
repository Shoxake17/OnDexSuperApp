package reservation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Tashkent — bron vaqtlari shu zonada ko'rsatiladi (UTC+5, yozgi vaqt yo'q).
var Tashkent = time.FixedZone("UZT", 5*3600)

// Deps — Service bog'liqliklari. Payments/Wallet/Hours/Notifier nil bo'lishi
// mumkin (mos imkoniyat o'chiq).
type Deps struct {
	Store    Store
	Tables   Tables
	Hours    Hours
	Payments Payments
	Wallet   Wallet
	Notifier Notifier
	NewID    func() string
	Now      func() time.Time
}

// Service — bron oqimi.
type Service struct{ d Deps }

// New — modul servisi.
func New(d Deps) (*Service, error) {
	if d.Store == nil || d.Tables == nil || d.NewID == nil {
		return nil, errors.New("reservation: Store, Tables va NewID majburiy")
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Service{d: d}, nil
}

func (s *Service) now() time.Time { return s.d.Now().UTC() }

func (s *Service) notify(ctx context.Context, typ string, r *Reservation) {
	if s.d.Notifier == nil || r == nil {
		return
	}
	s.d.Notifier.Notify(context.WithoutCancel(ctx), Event{Type: typ, Reservation: *r})
}

// ── Sozlamalar ──────────────────────────────────────────────────────────

// Settings — restoran sozlamalari (yozuv bo'lmasa standart, bron o'chiq).
func (s *Service) Settings(ctx context.Context, restaurantID string) (Settings, error) {
	st, err := s.d.Store.GetSettings(ctx, restaurantID)
	if errors.Is(err, ErrNotFound) {
		return DefaultSettings(restaurantID), nil
	}
	if err != nil {
		return Settings{}, err
	}
	if st.ZoneDeposits == nil {
		st.ZoneDeposits = map[string]int64{}
	}
	return *st, nil
}

// maxDepositTiyin — kishi boshiga zaklad chegarasi (10 mln so'm): xato
// kiritilgan nol (1 000 000 000) mijozni qo'rqitmasin.
const maxDepositTiyin = 1_000_000_000

// SaveSettings — restoran paneli.
func (s *Service) SaveSettings(ctx context.Context, in Settings) (Settings, error) {
	if in.RestaurantID == "" {
		return Settings{}, ErrInvalid
	}
	if in.DepositPerPersonTiyin < 0 || in.DepositPerPersonTiyin > maxDepositTiyin ||
		in.SlotMinutes < 30 || in.SlotMinutes > 480 ||
		in.MinAdvanceMinutes < 0 || in.MinAdvanceMinutes > 1440 ||
		in.MaxDaysAhead < 1 || in.MaxDaysAhead > 90 {
		return Settings{}, fmt.Errorf("%w: chegaradan tashqari qiymat", ErrInvalid)
	}
	zones := map[string]int64{}
	for z, v := range in.ZoneDeposits {
		z = strings.TrimSpace(z)
		if z == "" || utf8.RuneCountInString(z) > 40 || v < 0 || v > maxDepositTiyin || len(zones) >= 30 {
			return Settings{}, fmt.Errorf("%w: zona zakladi", ErrInvalid)
		}
		zones[z] = v
	}
	in.ZoneDeposits = zones
	in.UpdatedAt = s.now()
	if err := s.d.Store.PutSettings(ctx, &in); err != nil {
		return Settings{}, err
	}
	return in, nil
}

// depositPerPerson — zona qiymati bo'lsa o'sha, aks holda umumiy.
func (st Settings) depositPerPerson(zone string) int64 {
	if v, ok := st.ZoneDeposits[zone]; ok && zone != "" {
		return v
	}
	return st.DepositPerPersonTiyin
}

// ── Bo'sh vaqtlar ───────────────────────────────────────────────────────

// Slot — mijozga ko'rsatiladigan bron vaqti.
type Slot struct {
	StartsAt     time.Time `json:"starts_at"`
	FreeTables   int       `json:"free_tables"`
	DepositTiyin int64     `json:"deposit_tiyin"` // eng arzon mos stol zonasi bo'yicha
}

// Availability — kun ([day] Toshkent sanasi) bo'yicha bo'sh vaqtlar.
func (s *Service) Availability(ctx context.Context, restaurantID string, day time.Time, party int) ([]Slot, error) {
	st, err := s.Settings(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	if !st.Enabled {
		return nil, ErrDisabled
	}
	if party < 1 || party > MaxPartySize {
		return nil, fmt.Errorf("%w: kishi soni", ErrInvalid)
	}
	tables, err := s.fitting(ctx, restaurantID, party, "")
	if err != nil {
		return nil, err
	}
	d := day.In(Tashkent)
	dayStart := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, Tashkent)
	dayEnd := dayStart.Add(24 * time.Hour)
	slot := time.Duration(st.SlotMinutes) * time.Minute
	busy, err := s.d.Store.ListByRestaurant(ctx, restaurantID, dayStart.Add(-slot), dayEnd.Add(slot))
	if err != nil {
		return nil, err
	}
	out := []Slot{}
	for t := dayStart; t.Before(dayEnd); t = t.Add(SlotStep) {
		if s.checkWindow(st, t) != nil {
			continue
		}
		if s.d.Hours != nil {
			open, herr := s.d.Hours.OpenBetween(ctx, restaurantID, t, t.Add(slot))
			if herr != nil || !open {
				continue
			}
		}
		free := 0
		var cheapest int64 = -1
		for _, tb := range tables {
			if overlapsAny(busy, tb.ID, t, t.Add(slot)) {
				continue
			}
			free++
			if dep := st.depositPerPerson(tb.Zone) * int64(party); cheapest < 0 || dep < cheapest {
				cheapest = dep
			}
		}
		if free > 0 {
			out = append(out, Slot{StartsAt: t.UTC(), FreeTables: free, DepositTiyin: cheapest})
		}
	}
	return out, nil
}

func overlapsAny(list []*Reservation, tableID string, start, end time.Time) bool {
	for _, r := range list {
		if r.TableID == tableID && r.Status.Active() && r.StartsAt.Before(end) && start.Before(r.EndsAt) {
			return true
		}
	}
	return false
}

// checkWindow — vaqt qoidalari: qadamga mos, eng kamida min_advance oldin,
// max_days_ahead dan uzoq emas.
func (s *Service) checkWindow(st Settings, start time.Time) error {
	now := s.now()
	if start.Unix()%int64(SlotStep/time.Second) != 0 {
		return fmt.Errorf("%w: vaqt %d daqiqalik qadamda bo'lishi kerak", ErrInvalid, int(SlotStep/time.Minute))
	}
	if start.Before(now.Add(time.Duration(st.MinAdvanceMinutes) * time.Minute)) {
		return fmt.Errorf("%w: kamida %d daqiqa oldin bron qilinadi", ErrInvalid, st.MinAdvanceMinutes)
	}
	if start.After(now.Add(time.Duration(st.MaxDaysAhead) * 24 * time.Hour)) {
		return fmt.Errorf("%w: %d kundan uzoq oldinga bron qilinmaydi", ErrInvalid, st.MaxDaysAhead)
	}
	return nil
}

// fitting — sig'imi yetadigan faol stollar, eng kichik sig'imdan (katta
// stol kichik guruhga berilib, katta guruhga joy qolmasligi oldini oladi).
// [zone] berilsa o'sha zona stollari oldinda.
func (s *Service) fitting(ctx context.Context, restaurantID string, party int, zone string) ([]Table, error) {
	all, err := s.d.Tables.ListTables(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	var out []Table
	for _, t := range all {
		if t.Active && t.Capacity >= party {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		zi, zj := zone != "" && out[i].Zone == zone, zone != "" && out[j].Zone == zone
		if zi != zj {
			return zi
		}
		if out[i].Capacity != out[j].Capacity {
			return out[i].Capacity < out[j].Capacity
		}
		return out[i].Label < out[j].Label
	})
	return out, nil
}

// ── Bron yaratish ───────────────────────────────────────────────────────

// CreateInput — mijoz so'rovi.
type CreateInput struct {
	RestaurantID string
	CustomerID   string
	StartsAt     time.Time
	PartySize    int
	Zone         string // ixtiyoriy afzallik
	Method       Method // zaklad bo'lsa: card | wallet
	Channel      string // karta: payme | click | uzum | ""
	Comment      string
}

// CreateResult — bron va (karta bo'lsa) to'lov havolasi.
type CreateResult struct {
	Reservation *Reservation `json:"reservation"`
	Pay         *PayLink     `json:"pay,omitempty"`
}

const maxComment = 300

// Create — bron. Zaklad 0 bo'lsa darhol tasdiqlanadi; wallet — darhol
// yechiladi va tasdiqlanadi; karta — `held` + to'lov havolasi (15 daq).
func (s *Service) Create(ctx context.Context, in CreateInput) (*CreateResult, error) {
	st, err := s.Settings(ctx, in.RestaurantID)
	if err != nil {
		return nil, err
	}
	if !st.Enabled {
		return nil, ErrDisabled
	}
	in.Comment = strings.TrimSpace(in.Comment)
	if in.CustomerID == "" || in.PartySize < 1 || in.PartySize > MaxPartySize ||
		utf8.RuneCountInString(in.Comment) > maxComment {
		return nil, ErrInvalid
	}
	start := in.StartsAt.UTC()
	if err := s.checkWindow(st, start); err != nil {
		return nil, err
	}
	end := start.Add(time.Duration(st.SlotMinutes) * time.Minute)
	if s.d.Hours != nil {
		open, herr := s.d.Hours.OpenBetween(ctx, in.RestaurantID, start, end)
		if herr != nil {
			return nil, herr
		}
		if !open {
			return nil, ErrClosed
		}
	}
	if err := s.checkCustomerLimit(ctx, in.CustomerID, in.RestaurantID, start, end); err != nil {
		return nil, err
	}

	tables, err := s.fitting(ctx, in.RestaurantID, in.PartySize, strings.TrimSpace(in.Zone))
	if err != nil {
		return nil, err
	}
	now := s.now()
	r := &Reservation{
		ID: s.d.NewID(), RestaurantID: in.RestaurantID, CustomerID: in.CustomerID,
		PartySize: in.PartySize, StartsAt: start, EndsAt: end, Comment: in.Comment,
		CreatedAt: now, UpdatedAt: now,
	}
	// Stol tanlash: birinchi bo'sh mos stol. Poyga bo'lsa (boshqa mijoz
	// ayni stolni oldi) baza ErrSlotTaken beradi — keyingisi sinaladi.
	placed := false
	for _, t := range tables {
		r.TableID, r.TableLabel = t.ID, t.Label
		r.DepositTiyin = st.depositPerPerson(t.Zone) * int64(in.PartySize)
		s.prepareDeposit(r, in.Method, now)
		if r.DepositTiyin > 0 && r.DepositMethod == MethodNone {
			return nil, fmt.Errorf("%w: zaklad to'lov usulini tanlang (karta yoki OnDex Wallet)", ErrInvalid)
		}
		err := s.d.Store.Insert(ctx, r)
		if errors.Is(err, ErrSlotTaken) {
			continue
		}
		if err != nil {
			return nil, err
		}
		placed = true
		break
	}
	if !placed {
		return nil, ErrNoTable
	}

	switch {
	case r.DepositTiyin == 0:
		s.notify(ctx, "reservation_new", r)
		return &CreateResult{Reservation: r}, nil
	case r.DepositMethod == MethodWallet:
		if s.d.Wallet == nil {
			s.abortHeld(ctx, r, "OnDex Wallet ulanmagan")
			return nil, ErrPaymentDisabled
		}
		if werr := s.d.Wallet.Debit(ctx, walletRef(r.ID), r.CustomerID, r.DepositTiyin); werr != nil {
			s.abortHeld(ctx, r, "wallet: "+werr.Error())
			return nil, fmt.Errorf("%w: %v", ErrWalletFunds, werr)
		}
		if cerr := s.confirmPaid(ctx, r); cerr != nil {
			return nil, cerr
		}
		return &CreateResult{Reservation: r}, nil
	default: // karta
		if s.d.Payments == nil {
			s.abortHeld(ctx, r, "karta to'lovi ulanmagan")
			return nil, ErrPaymentDisabled
		}
		link, perr := s.d.Payments.StartDeposit(ctx, r.ID, r.CustomerID, r.RestaurantID,
			fmt.Sprintf("Stol broni zakladi, %d kishi", r.PartySize), in.Channel)
		if perr != nil {
			s.abortHeld(ctx, r, "to'lov: "+perr.Error())
			return nil, perr
		}
		return &CreateResult{Reservation: r, Pay: link}, nil
	}
}

func walletRef(id string) string { return "rsv:" + id }

// prepareDeposit — zaklad bo'yicha boshlang'ich holat.
func (s *Service) prepareDeposit(r *Reservation, m Method, now time.Time) {
	if r.DepositTiyin == 0 {
		r.Status, r.DepositMethod, r.DepositState, r.HoldExpiresAt = StatusConfirmed, MethodNone, DepositNone, nil
		return
	}
	switch m {
	case MethodCard, MethodWallet:
		r.DepositMethod = m
	default:
		r.DepositMethod = MethodNone
	}
	exp := now.Add(HoldTTL)
	r.Status, r.DepositState, r.HoldExpiresAt = StatusHeld, DepositPending, &exp
}

// abortHeld — to'lov boshlanmadi: stol darhol bo'shaydi.
func (s *Service) abortHeld(ctx context.Context, r *Reservation, reason string) {
	r.Status, r.CancelledBy, r.CancelReason, r.UpdatedAt = StatusExpired, BySystem, reason, s.now()
	if r.DepositState == DepositPending {
		r.DepositState = DepositNone
	}
	if err := s.d.Store.Update(ctx, r, StatusHeld); err != nil {
		slog.Error("bron: held'ni yopib bo'lmadi", "reservation", r.ID, "err", err)
	}
}

func (s *Service) checkCustomerLimit(ctx context.Context, customerID, restaurantID string, start, end time.Time) error {
	list, err := s.d.Store.ListByCustomer(ctx, customerID, 50)
	if err != nil {
		return err
	}
	active := 0
	now := s.now()
	for _, r := range list {
		if !r.Status.Active() || r.EndsAt.Before(now) {
			continue
		}
		active++
		if r.RestaurantID == restaurantID && r.StartsAt.Before(end) && start.Before(r.EndsAt) {
			return fmt.Errorf("%w: shu vaqtga bu restoranda broningiz bor", ErrInvalid)
		}
	}
	if active >= MaxActivePerUser {
		return ErrTooMany
	}
	return nil
}

// confirmPaid — zaklad to'landi: held -> confirmed.
func (s *Service) confirmPaid(ctx context.Context, r *Reservation) error {
	r.Status, r.DepositState, r.HoldExpiresAt, r.UpdatedAt = StatusConfirmed, DepositPaid, nil, s.now()
	if err := s.d.Store.Update(ctx, r, StatusHeld); err != nil {
		return err
	}
	s.notify(ctx, "reservation_new", r)
	return nil
}

// ── To'lov hodisalari (payments.DepositSink) ────────────────────────────

// DepositAmountTiyin — to'lov summasi bazadan (klientdan emas).
func (s *Service) DepositAmountTiyin(ctx context.Context, reservationID string) (int64, error) {
	r, err := s.d.Store.Get(ctx, reservationID)
	if err != nil {
		return 0, err
	}
	if r.Status != StatusHeld || r.DepositTiyin <= 0 {
		return 0, ErrConflict
	}
	return r.DepositTiyin, nil
}

// OnDepositPaid — karta zakladi to'landi. Bron muddati o'tib ketgan va
// stol boshqaga berilgan bo'lsa — pul darhol qaytariladi.
func (s *Service) OnDepositPaid(ctx context.Context, reservationID string) error {
	r, err := s.d.Store.Get(ctx, reservationID)
	if err != nil {
		return err
	}
	switch r.Status {
	case StatusHeld:
		return s.confirmPaid(ctx, r)
	case StatusExpired:
		// Kech to'lov: stol hali bo'sh bo'lsa bron tiklanadi.
		from := r.Status
		r.Status, r.DepositState, r.HoldExpiresAt, r.CancelReason, r.CancelledBy, r.UpdatedAt =
			StatusConfirmed, DepositPaid, nil, "", "", s.now()
		uerr := s.d.Store.Update(ctx, r, from)
		if uerr == nil {
			s.notify(ctx, "reservation_new", r)
			return nil
		}
		if !errors.Is(uerr, ErrSlotTaken) {
			return uerr
		}
		slog.Warn("bron: kech to'lov, stol band — zaklad qaytariladi", "reservation", r.ID)
		r, err = s.d.Store.Get(ctx, reservationID)
		if err != nil {
			return err
		}
		// Bazada zaklad hali "yo'q" (muddati o'tganda tozalangan), pul esa
		// HAQIQATAN yechildi — qaytarish o'tkazib yuborilmasin.
		r.DepositState = DepositPaid
		return s.refundAndClose(ctx, r, StatusExpired, BySystem, "to'lov kechikdi, stol band bo'lib qoldi")
	default:
		// confirmed (takroriy xabar) yoki allaqachon bekor — o'zgarish yo'q.
		return nil
	}
}

// OnDepositFailed — to'lov amalga oshmadi: held bo'lsa stol bo'shaydi.
func (s *Service) OnDepositFailed(ctx context.Context, reservationID string) error {
	r, err := s.d.Store.Get(ctx, reservationID)
	if err != nil {
		return err
	}
	if r.Status != StatusHeld {
		return nil
	}
	r.Status, r.DepositState, r.CancelledBy, r.CancelReason, r.UpdatedAt =
		StatusExpired, DepositNone, BySystem, "zaklad to'lanmadi", s.now()
	if err := s.d.Store.Update(ctx, r, StatusHeld); err != nil {
		return err
	}
	s.notify(ctx, "reservation_expired", r)
	return nil
}

// ── Bekor qilish va holatlar ────────────────────────────────────────────

// refundAndClose — zakladni qaytaradi (karta/wallet) va bronni yopadi.
func (s *Service) refundAndClose(ctx context.Context, r *Reservation, to Status, by, reason string) error {
	from := r.Status
	if r.DepositState == DepositPaid || r.DepositState == DepositPending {
		if err := s.refund(ctx, r); err != nil {
			return err
		}
		if r.DepositState == DepositPaid {
			r.DepositState = DepositRefunded
		} else {
			r.DepositState = DepositNone
		}
	}
	r.Status, r.CancelledBy, r.CancelReason, r.HoldExpiresAt, r.UpdatedAt = to, by, reason, nil, s.now()
	return s.d.Store.Update(ctx, r, from)
}

func (s *Service) refund(ctx context.Context, r *Reservation) error {
	switch r.DepositMethod {
	case MethodWallet:
		if r.DepositState != DepositPaid {
			return nil
		}
		if s.d.Wallet == nil {
			return ErrPaymentDisabled
		}
		return s.d.Wallet.Refund(ctx, walletRef(r.ID), r.CustomerID, r.DepositTiyin)
	case MethodCard:
		if s.d.Payments == nil {
			return ErrPaymentDisabled
		}
		// Pending bo'lsa ham chaqiriladi: ochiq to'lov havolasi yopiladi.
		return s.d.Payments.RefundDeposit(ctx, r.ID)
	}
	return nil
}

// CancelByCustomer — mijoz bekor qiladi. Bron vaqtigacha 2+ soat bo'lsa
// zaklad to'liq qaytadi, aks holda restoranda qoladi.
func (s *Service) CancelByCustomer(ctx context.Context, customerID, id string) (*Reservation, error) {
	r, err := s.d.Store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.CustomerID != customerID {
		return nil, ErrNotFound // begona bron mavjudligi oshkor qilinmaydi
	}
	if r.Status != StatusHeld && r.Status != StatusConfirmed {
		return nil, ErrConflict
	}
	// Bron vaqtigacha kamida 2 soat qolgan bo'lsa — bepul bekor.
	free := !s.now().Add(FreeCancelBefore).After(r.StartsAt)
	if free || r.DepositState != DepositPaid {
		if err := s.refundAndClose(ctx, r, StatusCancelled, ByCustomer, "mijoz bekor qildi"); err != nil {
			return nil, err
		}
	} else {
		from := r.Status
		r.Status, r.DepositState, r.CancelledBy, r.CancelReason, r.UpdatedAt =
			StatusCancelled, DepositKept, ByCustomer, "mijoz kech bekor qildi (2 soatdan kam qolganda)", s.now()
		if err := s.d.Store.Update(ctx, r, from); err != nil {
			return nil, err
		}
	}
	s.notify(ctx, "reservation_cancelled", r)
	return r, nil
}

// CancelByRestaurant — restoran bekor qiladi (sabab majburiy): zaklad doim qaytadi.
func (s *Service) CancelByRestaurant(ctx context.Context, restaurantID, id, reason string) (*Reservation, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" || utf8.RuneCountInString(reason) > maxComment {
		return nil, fmt.Errorf("%w: bekor qilish sababi", ErrInvalid)
	}
	r, err := s.ownedActive(ctx, restaurantID, id)
	if err != nil {
		return nil, err
	}
	if r.Status == StatusSeated {
		return nil, ErrConflict
	}
	if err := s.refundAndClose(ctx, r, StatusCancelled, ByRestaurant, reason); err != nil {
		return nil, err
	}
	s.notify(ctx, "reservation_cancelled", r)
	return r, nil
}

func (s *Service) ownedActive(ctx context.Context, restaurantID, id string) (*Reservation, error) {
	r, err := s.d.Store.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if r.RestaurantID != restaurantID {
		return nil, ErrNotFound
	}
	if !r.Status.Active() {
		return nil, ErrConflict
	}
	return r, nil
}

// Seat — mijoz keldi: zaklad hisobdan ayriladi (applied).
func (s *Service) Seat(ctx context.Context, restaurantID, id string) (*Reservation, error) {
	r, err := s.ownedActive(ctx, restaurantID, id)
	if err != nil {
		return nil, err
	}
	if r.Status != StatusConfirmed {
		return nil, ErrConflict
	}
	// Ertaroq kelish mumkin, lekin bron kunidan oldin emas.
	if s.now().Before(r.StartsAt.Add(-3 * time.Hour)) {
		return nil, fmt.Errorf("%w: bron vaqti hali kelmagan", ErrConflict)
	}
	r.Status, r.UpdatedAt = StatusSeated, s.now()
	if r.DepositState == DepositPaid {
		r.DepositState = DepositApplied
	}
	if err := s.d.Store.Update(ctx, r, StatusConfirmed); err != nil {
		return nil, err
	}
	s.notify(ctx, "reservation_seated", r)
	return r, nil
}

// Complete — stol bo'shadi.
func (s *Service) Complete(ctx context.Context, restaurantID, id string) (*Reservation, error) {
	r, err := s.ownedActive(ctx, restaurantID, id)
	if err != nil {
		return nil, err
	}
	if r.Status != StatusSeated {
		return nil, ErrConflict
	}
	r.Status, r.UpdatedAt = StatusCompleted, s.now()
	if err := s.d.Store.Update(ctx, r, StatusSeated); err != nil {
		return nil, err
	}
	return r, nil
}

// MarkNoShow — restoran "kelmadi" deydi (15 daq kechikishdan keyin).
func (s *Service) MarkNoShow(ctx context.Context, restaurantID, id string) (*Reservation, error) {
	r, err := s.ownedActive(ctx, restaurantID, id)
	if err != nil {
		return nil, err
	}
	if r.Status != StatusConfirmed || s.now().Before(r.StartsAt.Add(NoShowAfter)) {
		return nil, fmt.Errorf("%w: 'kelmadi' faqat bron vaqtidan 15 daqiqa o'tgach", ErrConflict)
	}
	return r, s.noShow(ctx, r)
}

func (s *Service) noShow(ctx context.Context, r *Reservation) error {
	r.Status, r.UpdatedAt = StatusNoShow, s.now()
	if r.DepositState == DepositPaid {
		r.DepositState = DepositKept
	}
	if err := s.d.Store.Update(ctx, r, StatusConfirmed); err != nil {
		return err
	}
	s.notify(ctx, "reservation_no_show", r)
	return nil
}

// ── O'qish ──────────────────────────────────────────────────────────────

// Get — bitta bron (egasi tekshiruvi chaqiruvchida).
func (s *Service) Get(ctx context.Context, id string) (*Reservation, error) {
	return s.d.Store.Get(ctx, id)
}

// ForCustomer — mijozning bronlari (yangisi birinchi).
func (s *Service) ForCustomer(ctx context.Context, customerID string) ([]*Reservation, error) {
	return s.d.Store.ListByCustomer(ctx, customerID, 50)
}

// ForRestaurant — [from, to) oralig'idagi bronlar (panel).
func (s *Service) ForRestaurant(ctx context.Context, restaurantID string, from, to time.Time) ([]*Reservation, error) {
	if !to.After(from) || to.Sub(from) > 32*24*time.Hour {
		return nil, fmt.Errorf("%w: davr", ErrInvalid)
	}
	return s.d.Store.ListByRestaurant(ctx, restaurantID, from, to)
}

// ── Fon nazoratchisi ────────────────────────────────────────────────────

// Sweep — muddati o'tgan held'lar, kelmaganlar, eslatmalar, tugagan
// seated'lar. Har ~30 soniyada chaqiriladi; bir nechta nusxada xavfsiz
// (holat o'zgarishi shartli — ErrConflict o'tkazib yuboriladi).
func (s *Service) Sweep(ctx context.Context) int {
	now := s.now()
	due, err := s.d.Store.ListDue(ctx, now, 200)
	if err != nil {
		slog.Error("bron nazoratchisi: ro'yxat", "err", err)
		return 0
	}
	n := 0
	for _, r := range due {
		var err error
		switch {
		case r.Status == StatusHeld && r.HoldExpiresAt != nil && now.After(*r.HoldExpiresAt):
			err = s.refundAndClose(ctx, r, StatusExpired, BySystem, "zaklad muddatida to'lanmadi")
			if err == nil {
				s.notify(ctx, "reservation_expired", r)
			}
		case r.Status == StatusConfirmed && now.After(r.StartsAt.Add(NoShowAfter)):
			err = s.noShow(ctx, r)
		case r.Status == StatusConfirmed && r.RemindedAt == nil && !now.Before(r.StartsAt.Add(-ReminderBefore)):
			t := now
			r.RemindedAt, r.UpdatedAt = &t, now
			if err = s.d.Store.Update(ctx, r, StatusConfirmed); err == nil {
				s.notify(ctx, "reservation_reminder", r)
			}
		case r.Status == StatusSeated && now.After(r.EndsAt.Add(2*time.Hour)):
			r.Status, r.UpdatedAt = StatusCompleted, now
			err = s.d.Store.Update(ctx, r, StatusSeated)
		default:
			continue
		}
		if err != nil && !errors.Is(err, ErrConflict) {
			slog.Error("bron nazoratchisi", "reservation", r.ID, "err", err)
			continue
		}
		if err == nil {
			n++
		}
	}
	return n
}
