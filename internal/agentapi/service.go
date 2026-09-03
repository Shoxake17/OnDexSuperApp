package agentapi

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
)

// ErrAwaitingUser — qoralama chegaradan oshgan, foydalanuvchi
// tasdig'i kutilmoqda. Agent buni ko'rib odamga "ilovangizda
// tasdiqlang" deb aytadi.
var ErrAwaitingUser = errors.New("foydalanuvchi ilovada tasdiqlashi kutilmoqda")

// ErrAwaitingUserNew — qoralama AYNAN SHU so'rovda tasdiq kutish
// holatiga o'tdi (ilgari `open` edi).
//
// `ErrAwaitingUser` ni O'RAYDI, ya'ni `errors.Is(err, ErrAwaitingUser)`
// avvalgidek ishlaydi. Farq faqat HTTP qatlamiga kerak: bildirishnoma
// bir marta yuborilishi kerak. Agent tasdiqni qayta-qayta yuborsa,
// foydalanuvchi har safar yangi push olmasin.
var ErrAwaitingUserNew = fmt.Errorf("%w", ErrAwaitingUser)

const (
	// linkTTL — ulanish so'rovi qancha yashaydi.
	//
	// Qisqa bo'lishi SHART: bu vaqt ichida kodni taxmin qilish
	// oynasi ochiq turadi. 10 daqiqa — telefonni qo'lga olib,
	// ilovani ochib, kodni terish uchun mo'l-ko'l.
	linkTTL = 10 * time.Minute

	// draftTTL — qoralama qancha yashaydi.
	//
	// Narxlar va aksiyalar o'zgaradi, restoran yopiladi. Eski
	// qoralamani tasdiqlash mijozga u KO'RMAGAN narxni yozib
	// qo'yardi. Tasdiqda summa baribir qayta tekshiriladi, lekin
	// muddat birinchi to'siq.
	draftTTL = 15 * time.Minute

	// maxLimitTiyin — foydalanuvchi qo'ya oladigan eng katta chegara
	// (2 000 000 so'm).
	//
	// NEGA UMUMAN TAVAN BOR: rozilik ekranida adashib nol qo'shib
	// yuborish oson, va bu xato bir marta qilinadi-yu, uzoq vaqt
	// bilinmay qoladi. Tavan xatoning narxini cheklaydi. Bu
	// summadan katta buyurtma qilish taqiqlanmagan — u shunchaki
	// ilovada tasdiqlanadi.
	maxLimitTiyin int64 = 200_000_000

	// maxExternalRefLen — sherikdagi foydalanuvchi ID uzunligi.
	maxExternalRefLen = 128

	// auditDetailMax — audit matni uzunligi. Cheklovsiz qoldirilsa
	// sherik bazani o'z matni bilan to'ldira olardi.
	auditDetailMax = 500
)

// Pricer — savatni katalog bo'yicha tekshiradi va narxlaydi.
// `*catalog.Service` shu interfeysni qanoatlantiradi.
type Pricer interface {
	PriceOrder(ctx context.Context, reqs []catalog.ItemRequest) (string, []orders.Item, error)
}

// Quoter — aksiya va chegirmalarni hisobga olgan YAKUNIY summa.
// `*orders.Service` shu interfeysni qanoatlantiradi.
//
// NEGA `orders.Service.Quote` QAYTA ISHLATILADI: u checkout ekrani
// ko'rsatadigan AYNAN o'sha hisobni bajaradi. Agar bu yerda summa
// alohida hisoblanganda, agent bir raqamni aytib, buyurtmada boshqa
// raqam chiqishi mumkin edi.
type Quoter interface {
	Quote(ctx context.Context, restaurantID string, items []orders.Item, customerID string) (*orders.QuoteResult, error)
}

// PlaceFunc — qoralamani HAQIQIY buyurtmaga aylantiradi.
//
// ┌─ NEGA CALLBACK, XIZMAT ICHIDA EMAS ────────────────────────────────┐
// Buyurtma yaratish manzilni o'qish, xizmat hududini tekshirish va
// to'lov usulini qo'yishni talab qiladi — bularning hammasi HTTP
// qatlamida, oddiy `POST /orders` bilan BIR XIL kodda bajariladi.
// Agar bu paket o'z nusxasini yozganda, ikki yo'l vaqt o'tib bir-
// biridan uzoqlashardi va agent orqali kelgan buyurtma boshqa
// qoidalar bo'yicha ishlanardi. Aynan shunday "ikkinchi yo'l"
// xatolari eng qimmatga tushadi.
// └────────────────────────────────────────────────────────────────────┘
type PlaceFunc func(ctx context.Context, d *Draft) (orderID string, err error)

// Service — agent integratsiyasining barcha qoidalari.
type Service struct {
	repo   Repository
	pricer Pricer
	quoter Quoter
	now    func() time.Time

	// locks — grant bo'yicha buyurtma joylashtirishni
	// ketma-ketlashtiradi. Sababi `ConfirmDraft` da.
	locks grantLocks
}

func NewService(repo Repository, pricer Pricer, quoter Quoter) *Service {
	return &Service{repo: repo, pricer: pricer, quoter: quoter, now: time.Now}
}

// ── Grant bo'yicha qulf ──

// grantLocks — grant ID bo'yicha mutex beradi.
//
// ┌─ NEGA KERAK ───────────────────────────────────────────────────────┐
// Kunlik chegara "shu paytgacha sarflangan summa" ni O'QIB, keyin
// buyurtma joylashtiradi. O'qish bilan yozish orasida boshqa so'rov
// o'tib ketsa, ikkalasi ham bir xil "hali chegaradan oshmagan" javobni
// ko'radi va ikkalasi ham o'tadi.
//
// Xarita o'sib ketmaydi: oxirgi foydalanuvchi qulfni bo'shatganda
// yozuv o'chiriladi.
// └────────────────────────────────────────────────────────────────────┘
type grantLocks struct {
	mu sync.Mutex
	m  map[string]*grantLock
}

type grantLock struct {
	mu  sync.Mutex
	ref int
}

// lock — qulfni oladi va uni bo'shatadigan funksiyani qaytaradi.
func (l *grantLocks) lock(id string) func() {
	l.mu.Lock()
	if l.m == nil {
		l.m = make(map[string]*grantLock)
	}
	e, ok := l.m[id]
	if !ok {
		e = &grantLock{}
		l.m[id] = e
	}
	e.ref++
	l.mu.Unlock()

	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		l.mu.Lock()
		if e.ref--; e.ref == 0 {
			delete(l.m, id)
		}
		l.mu.Unlock()
	}
}

// SetClock — vaqt manbaini almashtiradi.
//
// FAQAT TESTLAR uchun. Muddat tugashi (ulanish so'rovi, qoralama) bu
// tizimdagi eng muhim himoyalardan biri, uni esa haqiqiy vaqtni
// kutmasdan tekshirishning boshqa yo'li yo'q. Production kodida
// chaqirilmaydi.
func (s *Service) SetClock(fn func() time.Time) { s.now = fn }

// ── Sherik autentifikatsiyasi ──

// AuthenticatePartner — `Authorization: Bearer ondex_live_...`.
func (s *Service) AuthenticatePartner(ctx context.Context, key string) (*Partner, error) {
	key = strings.TrimSpace(key)
	// Shakl tekshiruvi — bazaga bemaqsad so'rov yubormaslik uchun.
	if !LooksLikePartnerKey(key) {
		return nil, ErrPartnerKey
	}
	p, err := s.repo.PartnerByKeyHash(ctx, HashSecret(key))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrPartnerKey
		}
		return nil, err
	}
	if !p.Active || p.RevokedAt != nil {
		return nil, ErrPartnerOff
	}
	return p, nil
}

// AuthenticateGrant — `X-OnDex-Grant: ondexg_...`.
//
// ┌─ ★ ENG MUHIM TEKSHIRUV: GRANT SHU SHERIKNIKIMI ────────────────────┐
// Grant har doim BITTA sherikka bog'langan. Bu bog'lanish
// tekshirilmasa, ikkinchi sherik (yoki o'z kaliti bor har qanday
// integratsiya) boshqa sherikning grantini ishlatib, o'ziga umuman
// ruxsat berilmagan foydalanuvchi nomidan ish qila olardi.
// └────────────────────────────────────────────────────────────────────┘
func (s *Service) AuthenticateGrant(ctx context.Context, p *Partner, token string) (*Grant, error) {
	token = strings.TrimSpace(token)
	if !LooksLikeGrantToken(token) {
		return nil, ErrGrantInvalid
	}
	g, err := s.repo.GrantByTokenHash(ctx, HashSecret(token))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrGrantInvalid
		}
		return nil, err
	}
	if g.PartnerID != p.ID {
		// Ataylab ErrGrantInvalid (aniqroq xato emas): begona sherik
		// grantning MAVJUDLIGINI ham bilmasligi kerak.
		return nil, ErrGrantInvalid
	}
	if err := g.Usable(s.now()); err != nil {
		return nil, err
	}
	// Xato oqimni to'xtatmaydi — bu faqat "oxirgi ishlatilgan"
	// ustuni, foydalanuvchiga ma'lumot uchun.
	if err := s.repo.TouchGrant(ctx, g.ID, s.now()); err != nil {
		slog.Warn("agentapi: last_used_at yangilanmadi", "grant", g.ID, "err", err)
	}
	return g, nil
}

// RequireScope — ruxsat tekshiruvi.
func (s *Service) RequireScope(g *Grant, scope string) error {
	if g == nil || !g.Allows(scope) {
		return fmt.Errorf("%w (%s)", ErrScopeDenied, ScopeLabel(scope))
	}
	return nil
}

// ── Sherik ro'yxatga olish (CLI orqali) ──

// RegisterPartner — yangi integratsiya mijozi. To'liq kalitni BIR
// MARTA qaytaradi; u boshqa hech qachon tiklanmaydi.
func (s *Service) RegisterPartner(ctx context.Context, name, contact string,
	env Environment, scopes []string) (*Partner, string, error) {

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, "", errors.New("sherik nomi bo'sh")
	}
	if env != EnvLive && env != EnvTest {
		return nil, "", errors.New("environment: live yoki test")
	}
	if len(scopes) == 0 {
		return nil, "", errors.New("kamida bitta ruxsat kerak")
	}
	if err := ValidateScopes(scopes); err != nil {
		return nil, "", err
	}
	id, err := NewID()
	if err != nil {
		return nil, "", err
	}
	full, prefix, err := NewPartnerKey(env)
	if err != nil {
		return nil, "", err
	}
	p := &Partner{
		ID:          id,
		Name:        name,
		KeyPrefix:   prefix,
		Environment: env,
		Scopes:      scopes,
		Active:      true,
		Contact:     strings.TrimSpace(contact),
		CreatedAt:   s.now(),
	}
	if err := s.repo.CreatePartner(ctx, p, HashSecret(full)); err != nil {
		return nil, "", err
	}
	return p, full, nil
}

func (s *Service) ListPartners(ctx context.Context) ([]*Partner, error) {
	return s.repo.ListPartners(ctx)
}

// SetPartnerActive — integratsiyani darhol o'chirish/yoqish
// ("o'chirish tugmasi"). Kalit o'g'irlangani ma'lum bo'lsa birinchi
// qadam shu.
func (s *Service) SetPartnerActive(ctx context.Context, id string, active bool) error {
	return s.repo.SetPartnerActive(ctx, id, active)
}

// ── Ulanish oqimi ──

// StartLink — sherik "foydalanuvchi meni OnDex bilan bog'lasin"
// deydi. Qaytadi: so'rov, POLL SIRI va foydalanuvchi teradigan KOD.
//
// Ikkala sir ham FAQAT shu yerda ochiq ko'rinadi; bazaga hash yotadi.
func (s *Service) StartLink(ctx context.Context, p *Partner, externalRef string,
	wantScopes []string) (link *LinkRequest, secret, userCode string, err error) {

	if len(externalRef) > maxExternalRefLen {
		return nil, "", "", errors.New("external_ref juda uzun")
	}
	if err := ValidateScopes(wantScopes); err != nil {
		return nil, "", "", err
	}
	// Sherik o'z tavanidan ko'p so'ray olmaydi. Kesib tashlanadi —
	// so'rovni butunlay rad etish integratsiyani sinab ko'rayotgan
	// dasturchini keraksiz to'xtatardi.
	scopes := IntersectScopes(wantScopes, p.Scopes)
	if len(scopes) == 0 {
		return nil, "", "", errors.New("so'ralgan ruxsatlarning birortasi ham bu integratsiyaga berilmagan")
	}
	id, err := NewID()
	if err != nil {
		return nil, "", "", err
	}
	secret, err = NewLinkSecret()
	if err != nil {
		return nil, "", "", err
	}
	userCode, err = NewUserCode()
	if err != nil {
		return nil, "", "", err
	}
	now := s.now()
	link = &LinkRequest{
		ID:          id,
		PartnerID:   p.ID,
		SecretHash:  HashSecret(secret),
		CodeHash:    HashSecret(NormalizeUserCode(userCode)),
		ExternalRef: externalRef,
		Scopes:      scopes,
		Status:      LinkPending,
		CreatedAt:   now,
		ExpiresAt:   now.Add(linkTTL),
	}
	if err := s.repo.CreateLink(ctx, link); err != nil {
		return nil, "", "", err
	}
	s.audit(ctx, p.ID, "", "", "link.start", "kod yaratildi", true, "")
	return link, secret, userCode, nil
}

// PollLink — sherik natijani so'raydi.
//
// Tasdiqlangan bo'lsa grant tokeni AYNAN SHU YERDA yaratiladi va
// BIR MARTA qaytariladi. Shu sababli ochiq token bazada hech qachon
// yotmaydi: yozilgan `link_secret` ni keyin qayta ishlatib tokenni
// takror olib bo'lmaydi (holat `consumed` ga o'tadi).
func (s *Service) PollLink(ctx context.Context, p *Partner, linkID, secret string) (
	status LinkStatus, grantToken string, g *Grant, err error) {

	l, err := s.repo.LinkByID(ctx, linkID)
	if err != nil {
		return "", "", nil, err
	}
	if l.PartnerID != p.ID {
		return "", "", nil, ErrNotFound
	}
	// Doimiy vaqtli solishtirish — hash bo'lsa ham odat sifatida.
	if subtle.ConstantTimeCompare([]byte(l.SecretHash), []byte(HashSecret(secret))) != 1 {
		return "", "", nil, ErrNotFound
	}

	now := s.now()
	if l.Status == LinkPending && !now.Before(l.ExpiresAt) {
		l.Status = LinkExpired
		l.DecidedAt = &now
		if err := s.repo.UpdateLink(ctx, l); err != nil {
			return "", "", nil, err
		}
		return LinkExpired, "", nil, ErrLinkExpired
	}

	switch l.Status {
	case LinkPending:
		return LinkPending, "", nil, ErrLinkPending
	case LinkDenied:
		return LinkDenied, "", nil, nil
	case LinkExpired:
		return LinkExpired, "", nil, ErrLinkExpired
	case LinkConsumed:
		// Token allaqachon berilgan. Uni qayta bermaymiz.
		return LinkConsumed, "", nil, ErrLinkDecided
	case LinkApproved:
		// Yagona holat: tokenni hozir yaratamiz.
	default:
		return l.Status, "", nil, ErrLinkDecided
	}

	g, err = s.repo.GrantByID(ctx, l.GrantID)
	if err != nil {
		return "", "", nil, err
	}
	if err := g.Usable(now); err != nil {
		// Foydalanuvchi tasdiqlab, keyin darhol uzib qo'ygan holat.
		return "", "", nil, err
	}
	token, err := NewGrantToken()
	if err != nil {
		return "", "", nil, err
	}
	g.TokenHash = HashSecret(token)
	if err := s.repo.UpdateGrant(ctx, g); err != nil {
		return "", "", nil, err
	}
	l.Status = LinkConsumed
	l.DecidedAt = &now
	if err := s.repo.UpdateLink(ctx, l); err != nil {
		return "", "", nil, err
	}
	s.audit(ctx, p.ID, g.ID, g.UserID, "link.consumed", "grant tokeni berildi", true, "")
	return LinkConsumed, token, g, nil
}

// ResolveUserCode — foydalanuvchi ilovada kiritgan (yoki deep link
// orqali kelgan) kod bo'yicha so'rovni topadi.
func (s *Service) ResolveUserCode(ctx context.Context, rawCode string) (*LinkRequest, *Partner, error) {
	code := NormalizeUserCode(rawCode)
	if !ValidUserCode(code) {
		return nil, nil, ErrNotFound
	}
	l, err := s.repo.LinkByCodeHash(ctx, HashSecret(code))
	if err != nil {
		return nil, nil, err
	}
	if !l.isLive(s.now()) {
		if l.Status == LinkPending {
			return nil, nil, ErrLinkExpired
		}
		return nil, nil, ErrLinkDecided
	}
	p, err := s.repo.PartnerByID(ctx, l.PartnerID)
	if err != nil {
		return nil, nil, err
	}
	if !p.Active {
		return nil, nil, ErrPartnerOff
	}
	return l, p, nil
}

// ApproveLink — foydalanuvchi ilovada "Ruxsat beraman" dedi.
//
// `scopes` — foydalanuvchi TANLAGAN ruxsatlar. U sherik
// so'raganidan KO'PINI bera olmaydi (kesiladi), lekin KAMINI
// berishi mumkin — masalan buyurtma berishga ruxsat bermay, faqat
// holatni ko'rishga ruxsat berishi.
func (s *Service) ApproveLink(ctx context.Context, userID, rawCode string,
	scopes []string, perOrderLimit, dailyLimit int64) (*Grant, error) {

	if strings.TrimSpace(userID) == "" {
		return nil, errors.New("user_id bo'sh")
	}
	if perOrderLimit < 0 || dailyLimit < 0 {
		return nil, errors.New("chegara manfiy bo'lishi mumkin emas")
	}
	if perOrderLimit > maxLimitTiyin || dailyLimit > maxLimitTiyin {
		return nil, fmt.Errorf("chegara juda katta (maksimal %d so'm)", maxLimitTiyin/100)
	}
	// Kunlik chegara har bir buyurtma chegarasidan kichik bo'lishi
	// mantiqsiz — birinchi buyurtmayoq kunlik tavanni yorardi va
	// foydalanuvchi nima uchun tasdiq so'ralayotganini tushunmasdi.
	if dailyLimit > 0 && perOrderLimit > dailyLimit {
		return nil, errors.New("kunlik chegara bitta buyurtma chegarasidan kichik bo'lmasligi kerak")
	}
	if err := ValidateScopes(scopes); err != nil {
		return nil, err
	}
	l, p, err := s.ResolveUserCode(ctx, rawCode)
	if err != nil {
		return nil, err
	}
	granted := IntersectScopes(scopes, l.Scopes)
	if len(granted) == 0 {
		return nil, errors.New("kamida bitta ruxsat tanlang")
	}
	now := s.now()

	// ┌─ ESKI GRANT AVVAL UZILADI ─────────────────────────────────┐
	// Bazada (partner_id, user_id) bo'yicha FAOL grant yagona
	// bo'lishi shart (qisman unikal indeks). Foydalanuvchi qaytadan
	// ulanayotganda eskisi jimgina qolib ketsa, "Ulangan ilovalar"
	// dan bittasini uzganda ikkinchisi ishlab turaverardi.
	// └────────────────────────────────────────────────────────────┘
	if old, err := s.repo.ActiveGrantFor(ctx, p.ID, userID); err == nil {
		old.Status = GrantRevoked
		old.RevokedAt = &now
		if err := s.repo.UpdateGrant(ctx, old); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	id, err := NewID()
	if err != nil {
		return nil, err
	}
	g := &Grant{
		ID:                 id,
		PartnerID:          p.ID,
		PartnerName:        p.Name,
		UserID:             userID,
		Scopes:             granted,
		Status:             GrantActive,
		PerOrderLimitTiyin: perOrderLimit,
		DailyLimitTiyin:    dailyLimit,
		CreatedAt:          now,
	}
	if err := s.repo.CreateGrant(ctx, g); err != nil {
		return nil, err
	}
	l.Status = LinkApproved
	l.UserID = userID
	l.GrantID = g.ID
	l.DecidedAt = &now
	if err := s.repo.UpdateLink(ctx, l); err != nil {
		return nil, err
	}
	s.audit(ctx, p.ID, g.ID, userID, "grant.approved",
		"ruxsatlar: "+JoinScopes(granted), true, "")
	return g, nil
}

// DenyLink — foydalanuvchi rad etdi.
func (s *Service) DenyLink(ctx context.Context, userID, rawCode string) error {
	l, p, err := s.ResolveUserCode(ctx, rawCode)
	if err != nil {
		return err
	}
	now := s.now()
	l.Status = LinkDenied
	l.UserID = userID
	l.DecidedAt = &now
	if err := s.repo.UpdateLink(ctx, l); err != nil {
		return err
	}
	s.audit(ctx, p.ID, "", userID, "grant.denied", "", true, "")
	return nil
}

// ── Grantlarni boshqarish (foydalanuvchi tomoni) ──

func (s *Service) ListGrants(ctx context.Context, userID string) ([]*Grant, error) {
	return s.repo.ListGrantsByUser(ctx, userID)
}

// RevokeGrant — "Ulangan ilovalar" dagi uzish tugmasi.
//
// Bu YAGONA joy emas, lekin eng muhimi: bekor qilingan grant bilan
// keyingi so'rov darhol 401 oladi (tokenlar JWT emas — har so'rovda
// bazadan tekshiriladi, ya'ni "bekor qilingan, lekin hali amal
// qiladigan" oyna UMUMAN yo'q).
func (s *Service) RevokeGrant(ctx context.Context, userID, grantID string) error {
	g, err := s.repo.GrantByID(ctx, grantID)
	if err != nil {
		return err
	}
	// Egalik tekshiruvi — begona grantni uzib bo'lmaydi.
	if g.UserID != userID {
		return ErrNotFound
	}
	if g.Status == GrantRevoked {
		return nil // idempotent
	}
	now := s.now()
	g.Status = GrantRevoked
	g.RevokedAt = &now
	if err := s.repo.UpdateGrant(ctx, g); err != nil {
		return err
	}
	s.audit(ctx, g.PartnerID, g.ID, userID, "grant.revoked", "foydalanuvchi uzdi", true, "")
	return nil
}

func (s *Service) ListAudit(ctx context.Context, userID string, limit int) ([]*AuditEntry, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.repo.ListAuditByUser(ctx, userID, limit)
}

func (s *Service) ListOpenDrafts(ctx context.Context, userID string) ([]*Draft, error) {
	return s.repo.ListOpenDraftsByUser(ctx, userID)
}

// ── Buyurtma qoralamasi ──

// CreateDraft — agent savatni tuzdi; server uni narxlaydi va
// tasdiqlash rejimini aniqlaydi.
func (s *Service) CreateDraft(ctx context.Context, p *Partner, g *Grant,
	items []catalog.ItemRequest, paymentMethod string) (*Draft, error) {

	if err := s.RequireScope(g, ScopeOrdersCreate); err != nil {
		return nil, err
	}
	restaurantID, priced, err := s.pricer.PriceOrder(ctx, items)
	if err != nil {
		return nil, err
	}
	quote, err := s.quoter.Quote(ctx, restaurantID, priced, g.UserID)
	if err != nil {
		return nil, err
	}

	now := s.now()
	// ┌─ TASDIQLASH REJIMI ────────────────────────────────────────┐
	// Uchta holatning BIRORTASIDA ham agent o'zi yakunlay olmaydi:
	//   * chegara umuman qo'yilmagan (0)   — eng qattiq, standart;
	//   * summa bitta buyurtma chegarasidan katta;
	//   * kunlik chegara shu buyurtma bilan yoriladi.
	//
	// Uchinchisi RAD ETISH emas, TASDIQ so'rash: kunlik chegara
	// agentning NAZORATSIZ sarfini cheklaydi, odamning o'z pulini
	// ishlatishini emas.
	// └────────────────────────────────────────────────────────────┘
	requiresUser := g.PerOrderLimitTiyin <= 0 || quote.TotalTiyin > g.PerOrderLimitTiyin
	if !requiresUser {
		// Bu yerdagi tekshiruv agentga ERTA signal beradi ("bu
		// buyurtma tasdiq talab qiladi"), lekin YAKUNIY emas —
		// haqiqiy qaror `ConfirmDraft` da qabul qilinadi. Sababi
		// o'sha yerda tushuntirilgan.
		over, err := s.exceedsDaily(ctx, g, quote.TotalTiyin, now)
		if err != nil {
			return nil, err
		}
		requiresUser = over
	}

	id, err := NewID()
	if err != nil {
		return nil, err
	}
	d := &Draft{
		ID:            id,
		GrantID:       g.ID,
		UserID:        g.UserID,
		PartnerID:     p.ID,
		PartnerName:   p.Name,
		RestaurantID:  restaurantID,
		Items:         toDraftItems(priced),
		SubtotalTiyin: quote.SubtotalTiyin,
		DiscountTiyin: quote.DiscountTiyin,
		TotalTiyin:    quote.TotalTiyin,
		PaymentMethod: paymentMethod,
		Status:        DraftOpen,
		RequiresUser:  requiresUser,
		CreatedAt:     now,
		ExpiresAt:     now.Add(draftTTL),
	}
	if requiresUser {
		d.Status = DraftAwaitingUser
	}
	if err := s.repo.CreateDraft(ctx, d); err != nil {
		return nil, err
	}
	s.audit(ctx, p.ID, g.ID, g.UserID, "order.draft",
		fmt.Sprintf("%d tiyin, %d tur", d.TotalTiyin, len(d.Items)), true, "")
	return d, nil
}

// ConfirmDraft — agent qoralamani tasdiqlaydi.
//
// `expectedTotal` — agent O'ZI ko'rgan summa. Mos kelmasa buyurtma
// yaratilmaydi: bu til modelining "esidan chiqargan" yoki eskirgan
// holatda ishlayotganini ushlaydigan arzon, lekin kuchli tekshiruv.
func (s *Service) ConfirmDraft(ctx context.Context, p *Partner, g *Grant,
	draftID string, expectedTotal int64, place PlaceFunc) (*Draft, error) {

	if err := s.RequireScope(g, ScopeOrdersCreate); err != nil {
		return nil, err
	}
	// ┌─ ★ SHU GRANT BO'YICHA KETMA-KET ───────────────────────────┐
	// O'qish → chegara tekshiruvi → joylashtirish ketma-ketligi
	// bo'linmasligi kerak. Bo'linsa, bir vaqtda kelgan ikki tasdiq
	// ikkalasi ham "chegara ichida" degan javobni ko'radi.
	// └────────────────────────────────────────────────────────────┘
	unlock := s.locks.lock(g.ID)
	defer unlock()

	d, err := s.repo.DraftByID(ctx, draftID)
	if err != nil {
		return nil, err
	}
	// ★ Egalik: qoralama AYNAN shu grantniki bo'lishi shart.
	// Busiz bitta sherik boshqa foydalanuvchining qoralamasini
	// tasdiqlab yuborardi.
	if d.GrantID != g.ID {
		return nil, ErrNotFound
	}
	switch d.Status {
	case DraftPlaced:
		// Idempotent: tarmoq uzilib qayta yuborilgan so'rov yangi
		// buyurtma YARATMAYDI.
		return d, nil
	case DraftAwaitingUser:
		return d, ErrAwaitingUser
	case DraftOpen:
		// Davom etamiz.
	default:
		return nil, ErrDraftDecided
	}
	now := s.now()
	if !now.Before(d.ExpiresAt) {
		d.Status = DraftExpired
		d.DecidedAt = &now
		if err := s.repo.UpdateDraft(ctx, d); err != nil {
			return nil, err
		}
		return nil, ErrDraftExpired
	}
	if expectedTotal != d.TotalTiyin {
		return nil, fmt.Errorf("%w: qoralamada %d tiyin", ErrTotalMismatch, d.TotalTiyin)
	}

	// ┌─ ★ KUNLIK CHEGARA AYNAN SHU YERDA HAL QILINADI ────────────┐
	// Yaratishdagi tekshiruv YETARLI EMAS edi: `SpentSince` faqat
	// JOYLASHTIRILGAN qoralamalarni sanaydi, ya'ni agent avval bir
	// nechta qoralama yaratib (har biri "chegara ichida" ko'rinadi),
	// keyin hammasini ketma-ket tasdiqlab kunlik tavanni bir necha
	// barobar yorib o'ta olardi.
	//
	// Bu RAD ETISH emas, tasdiq so'rash: kunlik chegara agentning
	// NAZORATSIZ sarfini cheklaydi, odamning o'z pulini ishlatishini
	// emas — `CreateDraft` dagi mantiq bilan bir xil.
	// └────────────────────────────────────────────────────────────┘
	over, err := s.exceedsDaily(ctx, g, d.TotalTiyin, now)
	if err != nil {
		return nil, err
	}
	if over {
		d.Status = DraftAwaitingUser
		d.RequiresUser = true
		if err := s.repo.UpdateDraft(ctx, d); err != nil {
			return nil, err
		}
		s.audit(ctx, p.ID, g.ID, g.UserID, "order.limit",
			fmt.Sprintf("kunlik chegara: %d tiyin tasdiqqa yuborildi", d.TotalTiyin),
			false, "")
		return d, ErrAwaitingUserNew
	}
	return s.place(ctx, p, g, d, place)
}

// exceedsDaily — shu summa qo'shilsa kunlik chegara yoriladimi.
//
// Chegara 0 bo'lsa — cheklov yo'q (`false`). Sarf faqat HAQIQATAN
// joylashtirilgan qoralamalar bo'yicha hisoblanadi.
func (s *Service) exceedsDaily(ctx context.Context, g *Grant, total int64,
	now time.Time) (bool, error) {

	if g.DailyLimitTiyin <= 0 {
		return false, nil
	}
	spent, err := s.repo.SpentSince(ctx, g.ID, startOfDay(now))
	if err != nil {
		return false, err
	}
	return spent+total > g.DailyLimitTiyin, nil
}

// UserApproveDraft — foydalanuvchi ilovada tasdiqladi (chegaradan
// oshgan buyurtma).
func (s *Service) UserApproveDraft(ctx context.Context, userID, draftID string,
	place PlaceFunc) (*Draft, error) {

	d, err := s.repo.DraftByID(ctx, draftID)
	if err != nil {
		return nil, err
	}
	if d.UserID != userID {
		return nil, ErrNotFound
	}
	// Agent yo'li bilan BIR XIL qulf: sarf hisobi ikkala yo'ldan ham
	// o'zgaradi, ya'ni ular bir-birining o'qishini bo'lmasligi kerak.
	//
	// Kunlik chegara bu yerda QAYTA tekshirilmaydi — foydalanuvchining
	// o'zi tasdiqlayapti, chegara esa aynan uni so'rash uchun bor.
	unlock := s.locks.lock(d.GrantID)
	defer unlock()

	// Qulfni kutib turgan vaqtda agent uni tasdiqlagan bo'lishi
	// mumkin — holatni QAYTA o'qiymiz, aks holda quyidagi tekshiruvlar
	// eskirgan nusxa ustida ishlardi.
	if d, err = s.repo.DraftByID(ctx, draftID); err != nil {
		return nil, err
	}
	if d.Status == DraftPlaced {
		return d, nil
	}
	if d.Status != DraftAwaitingUser {
		return nil, ErrDraftDecided
	}
	now := s.now()
	if !now.Before(d.ExpiresAt) {
		d.Status = DraftExpired
		d.DecidedAt = &now
		if err := s.repo.UpdateDraft(ctx, d); err != nil {
			return nil, err
		}
		return nil, ErrDraftExpired
	}
	g, err := s.repo.GrantByID(ctx, d.GrantID)
	if err != nil {
		return nil, err
	}
	// Grant oradan uzilgan bo'lishi mumkin — o'shanda tasdiq ham
	// ishlamasligi kerak.
	if err := g.Usable(now); err != nil {
		return nil, err
	}
	p, err := s.repo.PartnerByID(ctx, g.PartnerID)
	if err != nil {
		return nil, err
	}
	return s.place(ctx, p, g, d, place)
}

// UserRejectDraft — foydalanuvchi rad etdi.
func (s *Service) UserRejectDraft(ctx context.Context, userID, draftID string) error {
	d, err := s.repo.DraftByID(ctx, draftID)
	if err != nil {
		return err
	}
	if d.UserID != userID {
		return ErrNotFound
	}
	if d.Status != DraftAwaitingUser && d.Status != DraftOpen {
		return ErrDraftDecided
	}
	now := s.now()
	d.Status = DraftRejected
	d.DecidedAt = &now
	if err := s.repo.UpdateDraft(ctx, d); err != nil {
		return err
	}
	s.audit(ctx, d.PartnerID, d.GrantID, userID, "order.rejected", "foydalanuvchi rad etdi", true, "")
	return nil
}

// place — qoralamani buyurtmaga aylantirishning YAGONA yo'li.
//
// Sinov (`test`) kaliti bu yerda to'xtaydi: qoralama "joylashtirildi"
// deb belgilanadi, lekin HAQIQIY buyurtma yaratilmaydi. Shu sababli
// integratsiyani ishlab chiquvchi butun oqimni oxirigacha sinab
// ko'radi va restoranga bitta ham soxta buyurtma tushmaydi.
func (s *Service) place(ctx context.Context, p *Partner, g *Grant, d *Draft,
	place PlaceFunc) (*Draft, error) {

	now := s.now()
	orderID := ""
	if p.Environment == EnvTest {
		orderID = "test-" + d.ID
	} else {
		var err error
		if orderID, err = place(ctx, d); err != nil {
			s.audit(ctx, p.ID, g.ID, g.UserID, "order.place", err.Error(), false, "")
			return nil, err
		}
	}
	d.Status = DraftPlaced
	d.OrderID = orderID
	d.DecidedAt = &now
	if err := s.repo.UpdateDraft(ctx, d); err != nil {
		// Buyurtma YARATILGAN, lekin qoralama holati yozilmadi.
		// Takroriy tasdiq yangi buyurtma yaratmaydi (buyurtmalarda
		// idempotentlik kaliti qoralama ID'sidan olinadi), shuning
		// uchun bu holat xavfsiz — lekin ko'rinmay qolmasligi kerak.
		slog.Error("agentapi: qoralama holati yozilmadi",
			"draft", d.ID, "order", orderID, "err", err)
		return nil, err
	}
	s.audit(ctx, p.ID, g.ID, g.UserID, "order.placed",
		fmt.Sprintf("buyurtma %s, %d tiyin", orderID, d.TotalTiyin), true, "")
	return d, nil
}

// IdempotencyKey — qoralamadan olinadigan buyurtma kaliti.
//
// Bu funksiya HTTP qatlamida ishlatiladi. Kalit qoralama ID'siga
// bog'langani uchun bitta qoralama HECH QACHON ikkita buyurtma
// yaratmaydi — hatto ikki so'rov bir vaqtda kelib qolsa ham (baza
// darajasidagi unikal indeks ikkinchisini ushlaydi va birinchisining
// buyurtmasini qaytaradi).
func IdempotencyKey(draftID string) string { return "agent:" + draftID }

// ── Yordamchilar ──

func toDraftItems(items []orders.Item) []DraftItem {
	out := make([]DraftItem, 0, len(items))
	for _, it := range items {
		out = append(out, DraftItem{
			ProductID:  it.ProductID,
			Name:       it.Name,
			Qty:        it.Qty,
			PriceTiyin: it.PriceTiyin,
		})
	}
	return out
}

// ToItemRequests — qoralamani qayta narxlash uchun.
func (d *Draft) ToItemRequests() []catalog.ItemRequest {
	out := make([]catalog.ItemRequest, 0, len(d.Items))
	for _, it := range d.Items {
		out = append(out, catalog.ItemRequest{ProductID: it.ProductID, Qty: it.Qty})
	}
	return out
}

// trimRunes — matnni BELGILAR bo'yicha qisqartiradi.
//
// ┌─ NEGA BAYT BO'YICHA EMAS ──────────────────────────────────────────┐
// `s[:n]` ko'p baytli belgini o'rtasidan bo'lib, yaroqsiz UTF-8 hosil
// qiladi. Xato matnlarimizda "—" (3 bayt) ko'p uchraydi. Postgres
// bunday qatorni RAD ETADI, natijada audit yozuvi jimgina yo'qolardi —
// aynan xato uzun bo'lgan holatlarda, ya'ni u eng kerakli paytda.
// └────────────────────────────────────────────────────────────────────┘
func trimRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// startOfDay — kunlik chegara uchun sanoq boshlanishi (server
// mintaqasi bo'yicha; O'zbekistonda bitta mintaqa).
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// audit — qayd yozadi. Xato oqimni TO'XTATMAYDI (log qilinadi):
// audit yozilmagani uchun foydalanuvchining buyurtmasini bekor
// qilish mutanosib emas.
func (s *Service) audit(ctx context.Context, partnerID, grantID, userID,
	action, detail string, ok bool, ip string) {

	detail = trimRunes(detail, auditDetailMax)
	e := &AuditEntry{
		PartnerID: partnerID,
		GrantID:   grantID,
		UserID:    userID,
		Action:    action,
		Detail:    detail,
		OK:        ok,
		IP:        ip,
		CreatedAt: s.now(),
	}
	if err := s.repo.AppendAudit(ctx, e); err != nil {
		slog.Warn("agentapi: audit yozilmadi", "action", action, "err", err)
	}
}

// Audit — HTTP qatlami uchun ochiq o'ram (so'rov IP'si bilan).
func (s *Service) Audit(ctx context.Context, p *Partner, g *Grant, action, detail string, ok bool, ip string) {
	partnerID, grantID, userID := "", "", ""
	if p != nil {
		partnerID = p.ID
	}
	if g != nil {
		grantID, userID = g.ID, g.UserID
	}
	s.audit(ctx, partnerID, grantID, userID, action, detail, ok, ip)
}
