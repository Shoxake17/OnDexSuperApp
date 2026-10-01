// Package multicardpay — Multicard (Rahmat) `payments.Provider`
// sifatida.
//
// ┌─ BIR BOSQICHLI TO'LOV ────────────────────────────────────────────┐
// Multicard'ning to'lov sahifasida HOLD YO'Q: mijoz to'lagan zahoti
// pul restoran hisobiga tushadi. Shuning uchun:
//
//	* yaratilgan to'lov `pending` bo'ladi, callback kelgach `paid`;
//	* `Capture` KERAK EMAS — `ErrNotSupported` qaytaradi va
//	  `payments.Service` uni faqat `held` holatdagi to'lov uchun
//	  chaqiradi, bizda esa bunday holat umuman bo'lmaydi;
//	* buyurtma rad etilsa pul QAYTARILADI (`Refund`), blok
//	  bo'shatilmaydi.
//
// Octo'ning ikki bosqichli modeli bilan farqi aynan shu. Buyurtma
// kodi o'zgarmaydi: `payments.Service` `paid` holatda ham
// `OnPaymentHeld` chaqiradi (buyurtma oshxonaga tushadi), rad
// etilganda esa `ReleaseForOrder` to'langan to'lovni qaytaradi.
// └───────────────────────────────────────────────────────────────────┘
package multicardpay

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"chustapp/internal/payments"
	"chustapp/internal/payments/mcconnect"
	"chustapp/internal/payments/multicard"
)

// Name — bazaga yoziladigan provayder nomi.
const Name = "multicard"

// Provider — restoran bo'yicha kalit oladigan to'lov provayderi.
type Provider struct {
	reg *mcconnect.Registry
}

func New(reg *mcconnect.Registry) *Provider { return &Provider{reg: reg} }

func (p *Provider) Name() string { return Name }

var (
	// ErrNotConnected — restoran to'lov tizimiga ulanmagan.
	ErrNotConnected = errors.New("restoran karta orqali to'lovni hali ulamagan")
	// ErrNoFiscalDefaults — kassada IKPU standartlari yo'q.
	ErrNoFiscalDefaults = errors.New(
		"kassada fiskal standartlar (IKPU) sozlanmagan — to'lov provayderiga murojaat qiling")
)

// client — restoran uchun klient va ulanish.
func (p *Provider) client(ctx context.Context, restaurantID string) (*multicard.Client, mcconnect.Credentials, error) {
	if p == nil || p.reg == nil {
		return nil, mcconnect.Credentials{}, payments.ErrNotConfigured
	}
	cl, creds, err := p.reg.ClientFor(ctx, restaurantID)
	if err != nil {
		return nil, creds, err
	}
	if cl == nil || !creds.Connected() {
		return nil, creds, ErrNotConnected
	}
	return cl, creds, nil
}

// Create — to'lov sahifasini yaratadi.
func (p *Provider) Create(ctx context.Context, req payments.CreateRequest) (*payments.CreateResult, error) {
	cl, creds, err := p.client(ctx, req.RestaurantID)
	if err != nil {
		return nil, err
	}
	ofd, err := fiscal(req.Items, req.AmountTiyin, creds, req.Description)
	if err != nil {
		return nil, err
	}

	// ┌─ IKKI YO'L ────────────────────────────────────────────────────┐
	// Mijoz AYNAN bitta xizmatni tanlagan bo'lsa (Payme/Click/Uzum) —
	// to'g'ridan-to'g'ri o'sha ilovaga havola. Tanlamagan bo'lsa —
	// universal sahifa (karta + hammasi bir joyda).
	// └────────────────────────────────────────────────────────────────┘
	var inv *multicard.Invoice
	var payURL string
	if ch := strings.ToLower(strings.TrimSpace(req.Channel)); ch != "" {
		if !multicard.IsAppPaymentSystem(ch) {
			return nil, fmt.Errorf("multicard: to'lov xizmati qo'llanmaydi: %q", req.Channel)
		}
		inv, err = cl.CreatePayment(ctx, multicard.DirectRequest{
			PaymentSystem: ch,
			StoreID:       creds.StoreID,
			AmountTiyin:   req.AmountTiyin,
			InvoiceID:     req.PaymentID,
			CallbackURL:   req.NotifyURL,
			ReturnURL:     req.ReturnURL,
			OFD:           ofd,
		})
		if err != nil {
			return nil, wrap(err)
		}
		payURL = inv.AppURL()
	} else {
		inv, err = cl.CreateInvoice(ctx, multicard.InvoiceRequest{
			StoreID:     creds.StoreID,
			AmountTiyin: req.AmountTiyin,
			InvoiceID:   req.PaymentID,
			CallbackURL: req.NotifyURL,
			OFD:         ofd,
			Lang:        language(req.Language),
			ReturnURL:   req.ReturnURL,
			TTLSeconds:  req.TTLMinutes * 60,
			Phone:       phone(req.CustomerPhone),
		})
		if err != nil {
			return nil, wrap(err)
		}
		payURL = inv.PayURL()
	}

	return &payments.CreateResult{
		ProviderPaymentID: inv.UUID,
		PayURL:            payURL,
		// ⚠️ `pending`, `held` EMAS: pul hali olinmagan va bloklanmagan
		// ham. Buyurtma faqat callback kelgach oshxonaga tushadi.
		Status: payments.StatusPending,
	}, nil
}

// Status — HAQIQIY holatni shlyuzdan so'raydi.
func (p *Provider) Status(ctx context.Context, restaurantID, providerPaymentID string) (*payments.StatusResult, error) {
	cl, _, err := p.client(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	info, err := cl.Payment(ctx, providerPaymentID)
	if err != nil {
		return nil, wrap(err)
	}
	res := &payments.StatusResult{
		ProviderPaymentID: info.UUID,
		Status:            MapStatus(info.Status),
	}
	if res.Status == payments.StatusPaid || res.Status == payments.StatusRefunded {
		res.PaidAmountTiyin = info.Amount()
	}
	if res.Status == payments.StatusRefunded {
		res.RefundedTiyin = info.Amount()
	}
	return res, nil
}

// Capture — bir bosqichli to'lovda KERAK EMAS.
//
// `payments.Service` uni faqat `held` holatdagi to'lov uchun
// chaqiradi; bu provayder hech qachon `held` yaratmaydi, ya'ni bu
// yerga kelish o'zi mantiqiy xato bo'lardi.
func (p *Provider) Capture(context.Context, string, string, int64) error {
	return payments.ErrNotSupported
}

// Cancel — TO'LANMAGAN to'lovni yopadi.
func (p *Provider) Cancel(ctx context.Context, restaurantID, providerPaymentID string) error {
	cl, _, err := p.client(ctx, restaurantID)
	if err != nil {
		return err
	}
	return wrap(cl.CancelInvoice(ctx, providerPaymentID))
}

// CancelsUnpaid — bu provayder TO'LANMAGAN (kutilayotgan) invoysni ham yopa
// oladi (`payments.UnpaidCanceller`). Bir bosqichli to'lovda `held` holati
// yo'q, shuning uchun bekor qilingan buyurtmaning to'lanmagan invoysi
// aynan shu yo'l bilan yopiladi.
func (p *Provider) CancelsUnpaid() bool { return true }

// Refund — to'langan pulni qaytaradi.
//
// ⚠️ Qaytarishda ham fiskal chek MAJBURIY: eski chek bekor qilinib,
// yangisi chiqariladi. Bizda qaytarish har doim TO'LIQ (buyurtma rad
// etilgan), shuning uchun bitta umumlashtirilgan satr yuboriladi.
func (p *Provider) Refund(ctx context.Context, restaurantID, providerPaymentID, _ string, amountTiyin int64) error {
	cl, creds, err := p.client(ctx, restaurantID)
	if err != nil {
		return err
	}
	ofd, err := fiscal(nil, amountTiyin, creds, "Buyurtma bekor qilindi")
	if err != nil {
		return err
	}
	return wrap(cl.RefundPartial(ctx, providerPaymentID, amountTiyin, ofd))
}

// ── Yordamchilar ────────────────────────────────────────────────────

// MapStatus — shlyuz holatini bizning holatga o'giradi.
//
// ┌─ FAQAT `success` PUL DEGANI ──────────────────────────────────────┐
// `draft`, `hold`, `billing`, `progress` — hali pul emas. Ularni
// "to'landi" deb qabul qilish buyurtmani to'lanmagan holda oshxonaga
// yuborardi (reestrda `draft` yig'indisi `success` dan katta edi).
// └───────────────────────────────────────────────────────────────────┘
func MapStatus(s string) payments.Status {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case multicard.StatusSuccess:
		return payments.StatusPaid
	case multicard.StatusRevert:
		return payments.StatusRefunded
	case multicard.StatusCancelled:
		return payments.StatusCanceled
	case multicard.StatusError:
		return payments.StatusFailed
	default:
		// `draft`, `hold`, `billing`, `progress` va noma'lum kodlar.
		return payments.StatusPending
	}
}

// fiscal — chek satrlarini shlyuz shakliga o'giradi.
//
// Satrlar berilmagan bo'lsa (qaytarish) bitta umumiy satr yasaladi.
func fiscal(items []payments.Item, amountTiyin int64, creds mcconnect.Credentials, fallbackName string) ([]multicard.OFDItem, error) {
	if creds.TaxMXIK == "" || creds.TaxPackageCode == "" {
		return nil, ErrNoFiscalDefaults
	}
	if len(items) == 0 {
		name := strings.TrimSpace(fallbackName)
		if name == "" {
			name = "Buyurtma"
		}
		return []multicard.OFDItem{{
			Name: name, Qty: 1, Price: amountTiyin, Total: amountTiyin,
			MXIK: creds.TaxMXIK, PackageCode: creds.TaxPackageCode,
		}}, nil
	}
	out := make([]multicard.OFDItem, 0, len(items))
	for _, it := range items {
		name := strings.TrimSpace(it.Name)
		if name == "" {
			name = "Taom"
		}
		qty := it.Qty
		if qty <= 0 {
			qty = 1
		}
		out = append(out, multicard.OFDItem{
			Name: name, Qty: qty,
			Price: it.UnitPriceTiyin, Total: it.TotalTiyin,
			MXIK: creds.TaxMXIK, PackageCode: creds.TaxPackageCode,
		})
	}
	return out, nil
}

// language — shlyuz qabul qiladigan til kodi.
func language(l string) string {
	switch strings.ToLower(strings.TrimSpace(l)) {
	case "ru":
		return "ru"
	case "en":
		return "en"
	default:
		return "uz"
	}
}

// phone — shlyuz `998XXXXXXXXX` kutadi, bizda `+998...` saqlanadi.
// Mos kelmasa umuman yuborilmaydi: noto'g'ri raqamga SMS ketmasin.
func phone(p string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, p)
	if len(digits) == 12 && strings.HasPrefix(digits, "998") {
		return digits
	}
	return ""
}

// wrap — shlyuz xatosini `payments` qatlamiga tarjima qiladi.
//
// Chaqiruvchi "qayta urinish mumkin" (tarmoq) bilan "provayder rad
// etdi" (4xx) ni ajrata olishi kerak.
func wrap(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *multicard.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Code == "ERROR_NOT_FOUND" {
			return payments.ErrNotFound
		}
		return &payments.ProviderError{Code: apiErr.Status, Message: apiErr.Details}
	}
	if errors.Is(err, multicard.ErrNotConfigured) {
		return payments.ErrNotConfigured
	}
	return fmt.Errorf("multicard: %w", err)
}
