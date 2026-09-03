// Agent integratsiyasining XAVFSIZLIK da'volari.
//
// Bu fayldagi har bir test bitta aniq da'voni tekshiradi. Ular
// "yaxshi bo'lardi" turkumidan emas: har biri buzilsa, buzilish
// haqiqiy pul yoki begona odamning ma'lumoti bilan tugaydi.
//
// `package agentapi_test` (ichki emas, tashqi) — ataylab: shu tarzda
// testlar `storage.MemoryAgentRepo` ni ishlatadi, ya'ni xizmat
// mantig'i bilan birga OMBOR ham tekshiriladi. Ichki testda soxta
// repo yozilganda, ombordagi xato (masalan egalik shartini unutish)
// ko'rinmay qolardi.
package agentapi_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"chustapp/internal/agentapi"
	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
)

// ── Soxta bog'liqliklar ──

type fakePricer struct {
	restaurantID string
	unitTiyin    int64
	err          error
}

func (f *fakePricer) PriceOrder(_ context.Context, reqs []catalog.ItemRequest) (string, []orders.Item, error) {
	if f.err != nil {
		return "", nil, f.err
	}
	items := make([]orders.Item, 0, len(reqs))
	for _, r := range reqs {
		items = append(items, orders.Item{
			ProductID: r.ProductID, Name: "Taom " + r.ProductID,
			Qty: r.Qty, PriceTiyin: f.unitTiyin,
		})
	}
	return f.restaurantID, items, nil
}

type fakeQuoter struct{}

func (fakeQuoter) Quote(_ context.Context, _ string, items []orders.Item, _ string) (*orders.QuoteResult, error) {
	var sum int64
	for _, it := range items {
		sum += it.PriceTiyin * int64(it.Qty)
	}
	return &orders.QuoteResult{SubtotalTiyin: sum, TotalTiyin: sum}, nil
}

// ── Yordamchilar ──

type fixture struct {
	svc   *agentapi.Service
	repo  *storage.MemoryAgentRepo
	clock time.Time
}

func newFixture(t *testing.T, unitTiyin int64) *fixture {
	t.Helper()
	repo := storage.NewMemoryAgentRepo()
	svc := agentapi.NewService(repo,
		&fakePricer{restaurantID: "r1", unitTiyin: unitTiyin}, fakeQuoter{})
	f := &fixture{svc: svc, repo: repo, clock: time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)}
	svc.SetClock(func() time.Time { return f.clock })
	return f
}

func (f *fixture) partner(t *testing.T, name string, scopes ...string) (*agentapi.Partner, string) {
	t.Helper()
	if len(scopes) == 0 {
		scopes = agentapi.AllScopes
	}
	p, key, err := f.svc.RegisterPartner(context.Background(), name, "",
		agentapi.EnvLive, scopes)
	if err != nil {
		t.Fatalf("sherik yaratilmadi: %v", err)
	}
	return p, key
}

// link — to'liq ulanish oqimi: start -> foydalanuvchi tasdig'i -> poll.
func (f *fixture) link(t *testing.T, p *agentapi.Partner, userID string,
	perOrder, daily int64) (*agentapi.Grant, string) {

	t.Helper()
	ctx := context.Background()
	l, secret, code, err := f.svc.StartLink(ctx, p, "ext-"+userID, p.Scopes)
	if err != nil {
		t.Fatalf("StartLink: %v", err)
	}
	if _, err := f.svc.ApproveLink(ctx, userID, code, p.Scopes, perOrder, daily); err != nil {
		t.Fatalf("ApproveLink: %v", err)
	}
	_, token, g, err := f.svc.PollLink(ctx, p, l.ID, secret)
	if err != nil {
		t.Fatalf("PollLink: %v", err)
	}
	if token == "" {
		t.Fatal("grant tokeni berilmadi")
	}
	return g, token
}

// place — soxta buyurtma joylashtiruvchi.
func place(id string) agentapi.PlaceFunc {
	return func(context.Context, *agentapi.Draft) (string, error) { return id, nil }
}

// ── ★ Eng muhim da'vo: kalitning o'zi hech narsa bermaydi ──

func TestGrantOfOnePartnerIsUselessToAnother(t *testing.T) {
	f := newFixture(t, 1000)
	ctx := context.Background()

	shaddiy, _ := f.partner(t, "Shaddiy AI")
	begona, _ := f.partner(t, "Begona integratsiya")

	_, token := f.link(t, shaddiy, "u1", 0, 0)

	// Shaddiy uchun berilgan grant Shaddiy bilan ishlaydi.
	if _, err := f.svc.AuthenticateGrant(ctx, shaddiy, token); err != nil {
		t.Fatalf("o'z granti ishlashi kerak edi: %v", err)
	}
	// AYNAN SHU token boshqa sherik kaliti bilan ishlamasligi SHART.
	// Aks holda o'z kaliti bor har qanday integratsiya begona
	// foydalanuvchi nomidan ish qila olardi.
	if _, err := f.svc.AuthenticateGrant(ctx, begona, token); !errors.Is(err, agentapi.ErrGrantInvalid) {
		t.Fatalf("begona sherik grantni ishlata oldi (xato: %v)", err)
	}
}

func TestRevokedGrantStopsImmediately(t *testing.T) {
	f := newFixture(t, 1000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g, token := f.link(t, p, "u1", 0, 0)

	if err := f.svc.RevokeGrant(ctx, "u1", g.ID); err != nil {
		t.Fatalf("RevokeGrant: %v", err)
	}
	// Kesh yoki "amal qilish muddati" yo'q — keyingi so'rovdayoq rad.
	if _, err := f.svc.AuthenticateGrant(ctx, p, token); !errors.Is(err, agentapi.ErrGrantRevoked) {
		t.Fatalf("uzilgan grant hali ham ishlayapti (xato: %v)", err)
	}
}

func TestRevokeRejectsForeignGrant(t *testing.T) {
	f := newFixture(t, 1000)
	p, _ := f.partner(t, "Shaddiy AI")
	g, _ := f.link(t, p, "u1", 0, 0)

	// Boshqa foydalanuvchi begona grantni uza olmasligi kerak.
	if err := f.svc.RevokeGrant(context.Background(), "u2", g.ID); !errors.Is(err, agentapi.ErrNotFound) {
		t.Fatalf("begona grant uzildi (xato: %v)", err)
	}
}

// ── Ruxsatlar (scope) ──

func TestUserCanNarrowScopesButNotWiden(t *testing.T) {
	f := newFixture(t, 1000)
	ctx := context.Background()
	// Sherikning tavani — faqat o'qish.
	p, _ := f.partner(t, "Faqat o'qiydigan", agentapi.ScopeCatalogRead, agentapi.ScopeOrdersRead)

	l, secret, code, err := f.svc.StartLink(ctx, p, "ext", p.Scopes)
	if err != nil {
		t.Fatalf("StartLink: %v", err)
	}
	// Foydalanuvchi (yoki buzilgan mijoz) so'ralmagan ruxsatni
	// qo'shishga urinadi.
	g, err := f.svc.ApproveLink(ctx, "u1", code,
		[]string{agentapi.ScopeCatalogRead, agentapi.ScopeOrdersCreate}, 0, 0)
	if err != nil {
		t.Fatalf("ApproveLink: %v", err)
	}
	if g.Allows(agentapi.ScopeOrdersCreate) {
		t.Fatal("so'ralmagan ruxsat berilib qoldi")
	}
	if !g.Allows(agentapi.ScopeCatalogRead) {
		t.Fatal("so'ralgan ruxsat berilmadi")
	}
	_ = l
	_ = secret
}

func TestScopeDeniedBlocksDraft(t *testing.T) {
	f := newFixture(t, 1000)
	ctx := context.Background()
	p, _ := f.partner(t, "O'qish", agentapi.ScopeCatalogRead, agentapi.ScopeOrdersRead)
	g, _ := f.link(t, p, "u1", 100_000, 0)

	_, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if !errors.Is(err, agentapi.ErrScopeDenied) {
		t.Fatalf("ruxsatsiz qoralama yaratildi (xato: %v)", err)
	}
}

// ── Ulanish oqimi ──

func TestGrantTokenIsIssuedOnlyOnce(t *testing.T) {
	f := newFixture(t, 1000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")

	l, secret, code, err := f.svc.StartLink(ctx, p, "ext", p.Scopes)
	if err != nil {
		t.Fatalf("StartLink: %v", err)
	}
	if _, err := f.svc.ApproveLink(ctx, "u1", code, p.Scopes, 0, 0); err != nil {
		t.Fatalf("ApproveLink: %v", err)
	}
	if _, token, _, err := f.svc.PollLink(ctx, p, l.ID, secret); err != nil || token == "" {
		t.Fatalf("birinchi poll token berishi kerak edi: %v", err)
	}
	// Yozib olingan `link_secret` bilan tokenni QAYTA olib bo'lmaydi.
	if _, token, _, err := f.svc.PollLink(ctx, p, l.ID, secret); token != "" ||
		!errors.Is(err, agentapi.ErrLinkDecided) {
		t.Fatalf("token ikkinchi marta berildi (token=%q, xato=%v)", token, err)
	}
}

func TestPollRejectsWrongSecret(t *testing.T) {
	f := newFixture(t, 1000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	l, _, _, err := f.svc.StartLink(ctx, p, "ext", p.Scopes)
	if err != nil {
		t.Fatalf("StartLink: %v", err)
	}
	// `link_id` loglarga tushishi mumkin — u YOLG'IZ o'zi yetarli
	// bo'lmasligi kerak.
	if _, _, _, err := f.svc.PollLink(ctx, p, l.ID, "ondexl_"+
		"0000000000000000000000000000000000000000000000000000000000000000"); !errors.Is(err, agentapi.ErrNotFound) {
		t.Fatalf("noto'g'ri sir bilan poll o'tdi (xato: %v)", err)
	}
}

func TestExpiredLinkCannotBeApproved(t *testing.T) {
	f := newFixture(t, 1000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	_, _, code, err := f.svc.StartLink(ctx, p, "ext", p.Scopes)
	if err != nil {
		t.Fatalf("StartLink: %v", err)
	}
	f.clock = f.clock.Add(11 * time.Minute) // TTL 10 daqiqa

	if _, err := f.svc.ApproveLink(ctx, "u1", code, p.Scopes, 0, 0); !errors.Is(err, agentapi.ErrLinkExpired) {
		t.Fatalf("muddati o'tgan so'rov tasdiqlandi (xato: %v)", err)
	}
}

func TestUserCodeNormalization(t *testing.T) {
	f := newFixture(t, 1000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	_, _, code, err := f.svc.StartLink(ctx, p, "ext", p.Scopes)
	if err != nil {
		t.Fatalf("StartLink: %v", err)
	}
	// Foydalanuvchi kodni kichik harfda, chiziqchasiz, bo'shliq
	// bilan tersa ham topilishi kerak — aks holda u "kod
	// ishlamayapti" degan xulosaga kelardi.
	messy := " " + agentapi.NormalizeUserCode(code) + " "
	if _, _, err := f.svc.ResolveUserCode(ctx, messy); err != nil {
		t.Fatalf("normallashtirilgan kod topilmadi: %v", err)
	}
}

// ── Pul chegaralari ──

func TestZeroLimitAlwaysRequiresUserApproval(t *testing.T) {
	f := newFixture(t, 50_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g, _ := f.link(t, p, "u1", 0, 0) // chegara qo'yilmagan = eng qattiq

	d, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if d.Status != agentapi.DraftAwaitingUser || !d.RequiresUser {
		t.Fatalf("chegarasiz grantda tasdiq so'ralmadi: %s", d.Status)
	}
	// Agent uni O'ZI tasdiqlay olmasligi kerak.
	if _, err := f.svc.ConfirmDraft(ctx, p, g, d.ID, d.TotalTiyin,
		place("o1")); !errors.Is(err, agentapi.ErrAwaitingUser) {
		t.Fatalf("agent chegarasiz grantda o'zi tasdiqladi (xato: %v)", err)
	}
}

func TestAgentConfirmsWithinLimit(t *testing.T) {
	f := newFixture(t, 20_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g, _ := f.link(t, p, "u1", 100_000, 0)

	d, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 2}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if d.RequiresUser {
		t.Fatal("chegara ichidagi buyurtmaga tasdiq so'raldi")
	}
	got, err := f.svc.ConfirmDraft(ctx, p, g, d.ID, d.TotalTiyin, place("o1"))
	if err != nil {
		t.Fatalf("ConfirmDraft: %v", err)
	}
	if got.Status != agentapi.DraftPlaced || got.OrderID != "o1" {
		t.Fatalf("buyurtma joylashtirilmadi: %+v", got)
	}
}

func TestOverPerOrderLimitNeedsUser(t *testing.T) {
	f := newFixture(t, 200_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g, _ := f.link(t, p, "u1", 100_000, 0)

	d, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if !d.RequiresUser {
		t.Fatal("chegaradan oshgan buyurtmaga tasdiq so'ralmadi")
	}
	// Foydalanuvchi tasdiqlasa — o'shanda yaratiladi.
	got, err := f.svc.UserApproveDraft(ctx, "u1", d.ID, place("o2"))
	if err != nil {
		t.Fatalf("UserApproveDraft: %v", err)
	}
	if got.OrderID != "o2" {
		t.Fatalf("buyurtma yaratilmadi: %+v", got)
	}
}

func TestDailyLimitForcesUserApproval(t *testing.T) {
	f := newFixture(t, 60_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	// Bitta buyurtma 100k gacha, kunlik 100k.
	g, _ := f.link(t, p, "u1", 100_000, 100_000)

	first, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if first.RequiresUser {
		t.Fatal("birinchi buyurtma chegara ichida edi")
	}
	if _, err := f.svc.ConfirmDraft(ctx, p, g, first.ID, first.TotalTiyin, place("o1")); err != nil {
		t.Fatalf("ConfirmDraft: %v", err)
	}
	// Ikkinchisi kunlik tavanni yoradi (60k + 60k > 100k) — endi
	// tasdiq foydalanuvchidan so'raladi. Bu RAD ETISH emas:
	// kunlik chegara agentning NAZORATSIZ sarfini cheklaydi.
	second, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p2", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft(2): %v", err)
	}
	if !second.RequiresUser {
		t.Fatal("kunlik chegara yorilganda tasdiq so'ralmadi")
	}
}

// ── Qoralama egaligi va yaxlitligi ──

func TestDraftOfAnotherGrantCannotBeConfirmed(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g1, _ := f.link(t, p, "u1", 100_000, 0)

	// Ikkinchi foydalanuvchi, AYNI sherik.
	g2, _ := f.link(t, p, "u2", 100_000, 0)

	d, err := f.svc.CreateDraft(ctx, p, g1, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	// u1 ning qoralamasini u2 ning granti bilan tasdiqlash —
	// begona odamning nomidan buyurtma berish demakdir.
	if _, err := f.svc.ConfirmDraft(ctx, p, g2, d.ID, d.TotalTiyin,
		place("x")); !errors.Is(err, agentapi.ErrNotFound) {
		t.Fatalf("begona qoralama tasdiqlandi (xato: %v)", err)
	}
}

func TestUserCannotApproveForeignDraft(t *testing.T) {
	f := newFixture(t, 500_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g, _ := f.link(t, p, "u1", 100_000, 0)

	d, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if _, err := f.svc.UserApproveDraft(ctx, "u2", d.ID, place("x")); !errors.Is(err, agentapi.ErrNotFound) {
		t.Fatalf("begona foydalanuvchi qoralamani tasdiqladi (xato: %v)", err)
	}
}

func TestTotalMismatchBlocksOrder(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g, _ := f.link(t, p, "u1", 100_000, 0)

	d, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	// Model "esidan chiqarib" boshqa summani aytdi.
	if _, err := f.svc.ConfirmDraft(ctx, p, g, d.ID, d.TotalTiyin-1,
		place("x")); !errors.Is(err, agentapi.ErrTotalMismatch) {
		t.Fatalf("summa mos kelmasa ham buyurtma yaratildi (xato: %v)", err)
	}
}

func TestExpiredDraftCannotBeConfirmed(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g, _ := f.link(t, p, "u1", 100_000, 0)

	d, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	f.clock = f.clock.Add(16 * time.Minute) // TTL 15 daqiqa
	if _, err := f.svc.ConfirmDraft(ctx, p, g, d.ID, d.TotalTiyin,
		place("x")); !errors.Is(err, agentapi.ErrDraftExpired) {
		t.Fatalf("eskirgan qoralama tasdiqlandi (xato: %v)", err)
	}
}

// ★ Kunlik chegara QORALAMALARNI OLDINDAN TUZIB olish bilan
// aylanib o'tilmasligi kerak.
//
// `SpentSince` faqat JOYLASHTIRILGAN qoralamalarni sanaydi. Shu sababli
// agent avval bir nechta qoralama yaratsa, ularning har biri yaratilish
// paytida "chegara ichida" bo'lib ko'rinadi (hech biri hali
// joylashtirilmagan). Agar tasdiqlashda qayta tekshirilmasa, hammasini
// ketma-ket tasdiqlab kunlik tavanni bir necha barobar yorib o'tish
// mumkin edi — foydalanuvchidan bir marta ham so'ramasdan.
func TestDailyLimitCannotBeBypassedByBatchingDrafts(t *testing.T) {
	f := newFixture(t, 60_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	// Bitta buyurtma 60k gacha, kunlik 100k. Ya'ni ikkinchi buyurtma
	// kunlik tavanni yoradi.
	g, _ := f.link(t, p, "u1", 60_000, 100_000)

	// Agent IKKALA qoralamani ham OLDIN tuzadi.
	first, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft(1): %v", err)
	}
	second, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p2", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft(2): %v", err)
	}
	// Ikkalasi ham yaratilish paytida chegara ichida ko'rinadi —
	// aynan shu tuzoqning boshlanishi.
	if first.RequiresUser || second.RequiresUser {
		t.Fatal("qoralamalar yaratilishda tasdiq talab qildi — sinov ssenariysi buzildi")
	}

	// Birinchisi o'tishi kerak: 0 + 60k <= 100k.
	if _, err := f.svc.ConfirmDraft(ctx, p, g, first.ID, first.TotalTiyin, place("o1")); err != nil {
		t.Fatalf("birinchi tasdiq: %v", err)
	}

	// ★ Ikkinchisi O'TMASLIGI kerak: 60k + 60k > 100k.
	got, err := f.svc.ConfirmDraft(ctx, p, g, second.ID, second.TotalTiyin, place("o2"))
	if !errors.Is(err, agentapi.ErrAwaitingUser) {
		t.Fatalf("kunlik chegara aylanib o'tildi — ikkinchi buyurtma tasdiqsiz o'tdi (xato: %v)", err)
	}
	if got == nil || got.Status != agentapi.DraftAwaitingUser || !got.RequiresUser {
		t.Fatalf("qoralama tasdiq kutish holatiga o'tmadi: %+v", got)
	}
	// Bildirishnoma bir marta yuborilishi uchun HTTP qatlami shu
	// farqni ishlatadi.
	if !errors.Is(err, agentapi.ErrAwaitingUserNew) {
		t.Fatal("yangi o'tish belgilanmadi — foydalanuvchiga xabar yuborilmasdi")
	}

	// Endi foydalanuvchi o'zi tasdiqlasa — o'tishi KERAK. Kunlik
	// chegara odamning o'z pulini ishlatishini emas, agentning
	// nazoratsiz sarfini cheklaydi.
	done, err := f.svc.UserApproveDraft(ctx, "u1", second.ID, place("o2"))
	if err != nil {
		t.Fatalf("foydalanuvchi tasdig'i o'tmadi: %v", err)
	}
	if done.Status != agentapi.DraftPlaced || done.OrderID != "o2" {
		t.Fatalf("foydalanuvchi tasdiqlagandan keyin buyurtma yaratilmadi: %+v", done)
	}
}

// Takroriy tasdiq YANGI o'tish deb belgilanmasligi kerak — aks holda
// foydalanuvchi har bir urinishda yangi push olardi.
func TestRepeatedConfirmDoesNotRenotify(t *testing.T) {
	f := newFixture(t, 60_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g, _ := f.link(t, p, "u1", 60_000, 100_000)

	first, _ := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	second, _ := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p2", Qty: 1}}, "cash")
	if _, err := f.svc.ConfirmDraft(ctx, p, g, first.ID, first.TotalTiyin, place("o1")); err != nil {
		t.Fatalf("birinchi tasdiq: %v", err)
	}
	if _, err := f.svc.ConfirmDraft(ctx, p, g, second.ID, second.TotalTiyin,
		place("o2")); !errors.Is(err, agentapi.ErrAwaitingUserNew) {
		t.Fatalf("birinchi o'tish belgilanmadi: %v", err)
	}
	// Ikkinchi urinish: hamon tasdiq kutilmoqda, lekin bu YANGI
	// o'tish emas.
	_, err := f.svc.ConfirmDraft(ctx, p, g, second.ID, second.TotalTiyin, place("o2"))
	if !errors.Is(err, agentapi.ErrAwaitingUser) {
		t.Fatalf("takroriy tasdiq kutilmagan xato berdi: %v", err)
	}
	if errors.Is(err, agentapi.ErrAwaitingUserNew) {
		t.Fatal("takroriy tasdiq yangi o'tish deb belgilandi — foydalanuvchi takroriy push olardi")
	}
}

func TestConfirmIsIdempotent(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g, _ := f.link(t, p, "u1", 100_000, 0)

	d, _ := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if _, err := f.svc.ConfirmDraft(ctx, p, g, d.ID, d.TotalTiyin, place("o1")); err != nil {
		t.Fatalf("ConfirmDraft: %v", err)
	}
	// Tarmoq uzilib qayta yuborilgan so'rov IKKINCHI buyurtma
	// yaratmasligi kerak.
	calls := 0
	second, err := f.svc.ConfirmDraft(ctx, p, g, d.ID, d.TotalTiyin,
		func(context.Context, *agentapi.Draft) (string, error) { calls++; return "o2", nil })
	if err != nil {
		t.Fatalf("takroriy tasdiq xato berdi: %v", err)
	}
	if calls != 0 {
		t.Fatal("takroriy tasdiq YANGI buyurtma yaratdi")
	}
	if second.OrderID != "o1" {
		t.Fatalf("takroriy tasdiq boshqa buyurtma qaytardi: %s", second.OrderID)
	}
}

// ── Sherik kaliti ──

func TestDisabledPartnerIsRejected(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	p, key := f.partner(t, "Shaddiy AI")

	if _, err := f.svc.AuthenticatePartner(ctx, key); err != nil {
		t.Fatalf("yangi kalit ishlamadi: %v", err)
	}
	if err := f.svc.SetPartnerActive(ctx, p.ID, false); err != nil {
		t.Fatalf("SetPartnerActive: %v", err)
	}
	// "O'chirish tugmasi" — kalit o'g'irlangani ma'lum bo'lgandagi
	// birinchi qadam. Darhol ishlashi shart.
	if _, err := f.svc.AuthenticatePartner(ctx, key); !errors.Is(err, agentapi.ErrPartnerOff) {
		t.Fatalf("o'chirilgan sherik kaliti hali ishlayapti (xato: %v)", err)
	}
}

func TestMalformedKeysNeverReachDatabase(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	for _, bad := range []string{"", "Bearer", "ondex_live_", "boshqa_kalit",
		"ondex_live_qisqa", "ondexg_notapartnerkey"} {
		if _, err := f.svc.AuthenticatePartner(ctx, bad); !errors.Is(err, agentapi.ErrPartnerKey) {
			t.Errorf("yaroqsiz kalit %q rad etilmadi (xato: %v)", bad, err)
		}
	}
	for _, bad := range []string{"", "ondexg_", "ondex_live_xxx", "ondexg_qisqa"} {
		p, _ := f.partner(t, "P"+bad)
		if _, err := f.svc.AuthenticateGrant(ctx, p, bad); !errors.Is(err, agentapi.ErrGrantInvalid) {
			t.Errorf("yaroqsiz grant %q rad etilmadi (xato: %v)", bad, err)
		}
	}
}

// TestEmptyTokenHashNeverMatches — grant yaratilgandan keyin, sherik
// tokenni OLIB KETGUNCHA `token_hash` bo'sh turadi. Bo'sh qiymat bilan
// qidirishga yo'l qo'yilsa, hali hech kimga berilmagan grant topilib
// qolardi.
func TestEmptyTokenHashNeverMatches(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	_, _, code, err := f.svc.StartLink(ctx, p, "ext", p.Scopes)
	if err != nil {
		t.Fatalf("StartLink: %v", err)
	}
	if _, err := f.svc.ApproveLink(ctx, "u1", code, p.Scopes, 0, 0); err != nil {
		t.Fatalf("ApproveLink: %v", err)
	}
	// Grant BOR, lekin token hali berilmagan.
	if _, err := f.repo.GrantByTokenHash(ctx, ""); !errors.Is(err, agentapi.ErrNotFound) {
		t.Fatalf("bo'sh hash bilan grant topildi (xato: %v)", err)
	}
}

// TestReconnectRevokesOldGrant — foydalanuvchi qaytadan ulanganda
// eski grant qolib ketmasligi kerak: "Ulangan ilovalar" dan bittasini
// uzganda ko'rinmaydigan ikkinchisi ishlab turaverardi.
func TestReconnectRevokesOldGrant(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")

	_, oldToken := f.link(t, p, "u1", 100_000, 0)
	_, newToken := f.link(t, p, "u1", 50_000, 0)

	if _, err := f.svc.AuthenticateGrant(ctx, p, oldToken); !errors.Is(err, agentapi.ErrGrantRevoked) {
		t.Fatalf("eski grant hali ishlayapti (xato: %v)", err)
	}
	if _, err := f.svc.AuthenticateGrant(ctx, p, newToken); err != nil {
		t.Fatalf("yangi grant ishlamadi: %v", err)
	}
	list, err := f.svc.ListGrants(ctx, "u1")
	if err != nil {
		t.Fatalf("ListGrants: %v", err)
	}
	active := 0
	for _, g := range list {
		if g.Status == agentapi.GrantActive {
			active++
		}
	}
	if active != 1 {
		t.Fatalf("faol grantlar soni %d, 1 bo'lishi kerak edi", active)
	}
}

// TestLimitsAreValidated — rozilik ekranidan kelgan chegaralar.
func TestLimitsAreValidated(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")

	cases := []struct {
		name            string
		perOrder, daily int64
	}{
		{"manfiy bitta buyurtma", -1, 0},
		{"manfiy kunlik", 0, -1},
		{"tavandan katta", 999_000_000, 0},
		{"kunlik bitta buyurtmadan kichik", 100_000, 50_000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, code, err := f.svc.StartLink(ctx, p, "ext", p.Scopes)
			if err != nil {
				t.Fatalf("StartLink: %v", err)
			}
			if _, err := f.svc.ApproveLink(ctx, "u1", code, p.Scopes,
				tc.perOrder, tc.daily); err == nil {
				t.Fatal("noto'g'ri chegara qabul qilindi")
			}
		})
	}
}

// TestTestPartnerPlacesNoRealOrder — `test` kaliti butun oqimni
// oxirigacha o'tkazadi, lekin restoranga bitta ham soxta buyurtma
// tushmaydi.
func TestTestPartnerPlacesNoRealOrder(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	p, _, err := f.svc.RegisterPartner(ctx, "Sinov", "", agentapi.EnvTest, agentapi.AllScopes)
	if err != nil {
		t.Fatalf("RegisterPartner: %v", err)
	}
	g, _ := f.link(t, p, "u1", 100_000, 0)

	d, err := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	called := false
	got, err := f.svc.ConfirmDraft(ctx, p, g, d.ID, d.TotalTiyin,
		func(context.Context, *agentapi.Draft) (string, error) {
			called = true
			return "haqiqiy", nil
		})
	if err != nil {
		t.Fatalf("ConfirmDraft: %v", err)
	}
	if called {
		t.Fatal("sinov kaliti HAQIQIY buyurtma yaratdi")
	}
	if got.Status != agentapi.DraftPlaced {
		t.Fatalf("sinov oqimi yakunlanmadi: %s", got.Status)
	}
}

// TestAuditRecordsWhatAgentDid — foydalanuvchi keyin ko'ra olishi
// kerak. Audit bo'lmasa, tizim "ishonchli" deb atalolmaydi.
func TestAuditRecordsWhatAgentDid(t *testing.T) {
	f := newFixture(t, 10_000)
	ctx := context.Background()
	p, _ := f.partner(t, "Shaddiy AI")
	g, _ := f.link(t, p, "u1", 100_000, 0)

	d, _ := f.svc.CreateDraft(ctx, p, g, []catalog.ItemRequest{{ProductID: "p1", Qty: 1}}, "cash")
	if _, err := f.svc.ConfirmDraft(ctx, p, g, d.ID, d.TotalTiyin, place("o1")); err != nil {
		t.Fatalf("ConfirmDraft: %v", err)
	}
	list, err := f.svc.ListAudit(ctx, "u1", 50)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	want := map[string]bool{"grant.approved": false, "order.draft": false, "order.placed": false}
	for _, e := range list {
		if _, ok := want[e.Action]; ok {
			want[e.Action] = true
		}
	}
	for action, found := range want {
		if !found {
			t.Errorf("auditda %q yozuvi yo'q", action)
		}
	}
}
