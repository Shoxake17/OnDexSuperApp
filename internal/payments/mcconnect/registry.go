// Package mcconnect — restoranning to'lov shlyuzi ulanishi.
//
// ┌─ NEGA ALOHIDA PAKET ──────────────────────────────────────────────┐
// "Shu restoran uchun qaysi Multicard klienti?" degan savolga IKKI joy
// javob berishi kerak:
//
//	* HTTP qatlami — Moliya sahifasi tushum reestrini o'qiydi;
//	* to'lov qatlami — buyurtma uchun to'lov yaratadi.
//
// Javob bir xil bo'lishi SHART: bitta restoran reestrni bir kalit
// bilan, to'lovni boshqa kalit bilan qilsa, pul bir hisobga tushib,
// hisobot boshqasidan o'qilardi. Shuning uchun mantiq bitta joyda.
//
// Paket `catalog` ni IMPORT QILMAYDI: kerakli uchta maydon
// [Credentials] orqali beriladi. Shu sababli to'lov qatlami katalog
// modelidan mustaqil qoladi.
// └───────────────────────────────────────────────────────────────────┘
package mcconnect

import (
	"context"
	"errors"
	"sync"

	"chustapp/internal/payments/multicard"
	"chustapp/internal/secretstore"
)

// Credentials — restoranning saqlangan ulanishi.
type Credentials struct {
	// AppID — restoranning O'Z `application_id` si. Bo'sh bo'lsa
	// platforma kalitlari ishlatiladi (OnDex shartnomasi ostida).
	AppID string
	// SecretEnc — shifrlangan `secret` (`internal/secretstore`).
	SecretEnc string
	// StoreID — kassa. Bo'sh — ulanmagan.
	StoreID string
	// TaxMXIK / TaxPackageCode — fiskal chek uchun standart qiymatlar
	// (kassa sozlamasidan, ulash paytida o'qilgan).
	TaxMXIK        string
	TaxPackageCode string
}

// Connected — to'lov qabul qilish uchun yetarlimi.
func (c Credentials) Connected() bool { return c.StoreID != "" }

// OwnCredentials — restoran o'z shartnomasi bilan ishlaydimi.
func (c Credentials) OwnCredentials() bool { return c.AppID != "" && c.SecretEnc != "" }

// Lookup — restoran bo'yicha ulanishni topadi.
type Lookup func(ctx context.Context, restaurantID string) (Credentials, error)

var (
	// ErrNotConnected — restoran to'lov shlyuziga ulanmagan.
	ErrNotConnected = errors.New("restoran to'lov tizimiga ulanmagan")
	// ErrUnavailable — ulanish imkoniyati server tomonda o'chiq.
	ErrUnavailable = errors.New("to'lov tizimiga ulanish sozlanmagan")
)

// Registry — restoran bo'yicha Multicard klientlari.
type Registry struct {
	lookup  Lookup
	secrets *secretstore.Box
	baseURL string
	// platform — OnDex'ning o'z kalitlari bilan klient. `nil` bo'lishi
	// mumkin: o'z shartnomasi bor restoranlar baribir ishlaydi.
	platform *multicard.Client

	// clients — `application_id` bo'yicha kesh.
	//
	// Kesh SHART: klient JWT'ni ichida saqlaydi. Har so'rovda yangi
	// klient yasalsa, har bir sahifa/to'lov uchun ortiqcha
	// `POST /auth` ketardi.
	clients sync.Map // appID(string) -> *multicard.Client
}

func NewRegistry(lookup Lookup, secrets *secretstore.Box, baseURL string,
	platform *multicard.Client) *Registry {
	return &Registry{lookup: lookup, secrets: secrets, baseURL: baseURL, platform: platform}
}

// CanConnect — restoran O'Z kalitlarini ulay oladimi.
//
// Ikkalasi ham shart: manzilsiz so'rov yuborib bo'lmaydi, shifrlash
// kalitisiz esa `secret` ochiq matnda yotardi.
func (r *Registry) CanConnect() bool {
	return r != nil && r.baseURL != "" && r.secrets != nil
}

// BaseURL — shlyuz manzili.
func (r *Registry) BaseURL() string {
	if r == nil {
		return ""
	}
	return r.baseURL
}

// Credentials — restoranning saqlangan ulanishi.
func (r *Registry) Credentials(ctx context.Context, restaurantID string) (Credentials, error) {
	if r == nil || r.lookup == nil {
		return Credentials{}, ErrUnavailable
	}
	return r.lookup(ctx, restaurantID)
}

// NewClient — BERILGAN kalitlar bilan klient (keshsiz).
//
// Ulashni tekshirish uchun: kalitlar hali saqlanmagan, ya'ni ularni
// keshlash ham mumkin emas.
func (r *Registry) NewClient(appID, secret string) (*multicard.Client, error) {
	if !r.CanConnect() {
		return nil, ErrUnavailable
	}
	return multicard.New(multicard.Config{
		BaseURL: r.baseURL, ApplicationID: appID, Secret: secret,
	})
}

// Seal — `secret` ni saqlash uchun shifrlaydi.
func (r *Registry) Seal(secret string) (string, error) {
	if !r.CanConnect() {
		return "", ErrUnavailable
	}
	return r.secrets.Seal(secret)
}

// Client — shu ulanish uchun klient.
//
// Restoranning O'Z kalitlari bo'lsa — ular; bo'lmasa platforma
// klienti. Ikkalasi ham yo'q bo'lsa `nil, nil`: bu XATO emas,
// "ulanmagan" degani. Kalitlar bor-u ochib bo'lmasa — XATO
// (shifrlash kaliti almashtirilgan).
func (r *Registry) Client(c Credentials) (*multicard.Client, error) {
	if r == nil {
		return nil, ErrUnavailable
	}
	if !c.OwnCredentials() {
		return r.platform, nil
	}
	if cl, ok := r.clients.Load(c.AppID); ok {
		return cl.(*multicard.Client), nil
	}
	if r.secrets == nil {
		return nil, errors.New("to'lov kalitlarini ochib bo'lmadi: server sozlamasi yo'q " +
			"(SECRETS_ENC_KEY) — OnDex bilan bog'laning")
	}
	secret, err := r.secrets.Open(c.SecretEnc)
	if err != nil {
		// ⚠️ Sabab chaqiruvchida loglanadi; bu yerdagi matn
		// foydalanuvchiga ko'rinadi va nima qilishni aytadi.
		return nil, errors.New("saqlangan to'lov kalitini ochib bo'lmadi — " +
			"kalitlarni qaytadan kiriting")
	}
	cl, err := multicard.New(multicard.Config{
		BaseURL: r.baseURL, ApplicationID: c.AppID, Secret: secret,
	})
	if err != nil {
		return nil, errors.New("to'lov kalitlari to'liq emas — qaytadan kiriting")
	}
	actual, _ := r.clients.LoadOrStore(c.AppID, cl)
	return actual.(*multicard.Client), nil
}

// ClientFor — restoran bo'yicha klient va uning ulanishi.
func (r *Registry) ClientFor(ctx context.Context, restaurantID string) (*multicard.Client, Credentials, error) {
	c, err := r.Credentials(ctx, restaurantID)
	if err != nil {
		return nil, c, err
	}
	cl, err := r.Client(c)
	return cl, c, err
}

// Invalidate — keshdagi klientni o'chiradi.
//
// Kalit almashtirilganda SHART: aks holda keyingi so'rov ESKI sir
// bilan ketardi va shlyuz uni rad etardi.
func (r *Registry) Invalidate(appID string) {
	if r == nil || appID == "" {
		return
	}
	r.clients.Delete(appID)
}
