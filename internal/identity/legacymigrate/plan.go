// Package legacymigrate — A1b: mavjud (eski) merchant akkauntlaridan
// tashkilot / biznes / a'zolik yozuvlarini to'ldirish (AUTH-PHASE-A.md 8-bo'lim).
//
// ┌─ QOIDALAR ────────────────────────────────────────────────────────────┐
//   - Reja SOF: bazaga tegmaydi, faqat kirishdan reja tuzadi (unit test).
//     Qo'llash — storage'da BITTA tranzaksiyada, takrorlanmaydigan (idempotent):
//     qayta ishga tushirilsa yangi narsa qo'shmaydi.
//   - restaurant/store akkaunti = biznes egasi. Ularni superadmin yaratgan,
//     ya'ni tashkilot darhol `active` (KYC keyin to'ldiriladi).
//   - waiter -> restoran xodimi (OnDexPro), courier -> restoran yoki qurt
//     do'koni kuryeri (OnDexGO). Platforma kuryerlari (to'xtatilgan) ko'chirilMAYDI.
//   - Eski akkauntlarning roli va `entity_id` si O'ZGARMAYDI: eski marshrutlar
//     ishlayveradi (A5 gacha), yangi kirish esa a'zolikka qaraydi.
//   - Avval mijoz bo'lgan xodimlar (shu akkauntda mijoz buyurtmalari bor)
//     FAQAT hisobotga chiqadi — ajratish alohida, egasi ro'yxatni ko'rgach.
//
// └───────────────────────────────────────────────────────────────────────┘
package legacymigrate

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"chustapp/internal/identity/authz"
	"chustapp/internal/users"
)

// StaffLink — `staff_members` dagi bog'lanish (akkaunt -> xodim yozuvi).
type StaffLink struct {
	MemberID     string
	RestaurantID string
	Status       string // staff.Status: active | on_leave | dismissed ...
}

// Input — eski holat (storage.LoadLegacyMerchants).
type Input struct {
	Users           []*users.User     // restaurant/store/waiter/courier, o'chirilmagan
	RestaurantNames map[string]string // restoran ID -> nom (katalog); yo'q bo'lsa akkaunt nomi
	CourierPools    map[string]string // kuryer yozuvi ID -> havza (restoran ID | "qurt:<do'kon>" | "")
	StaffByUser     map[string]StaffLink
	CustomerOrders  map[string]int // akkaunt -> mijoz sifatidagi buyurtmalar soni
	// Mavjud yozuvlar (takrorlamaslik uchun):
	ExistingBusinesses  map[string]string // "tur|legacy_id" -> business ID
	ExistingBusinessOrg map[string]string // business ID -> org ID
	ExistingMemberships map[string]bool   // "user|business" (egasi uchun "user|org:<org>")
	NewID               func() string
	Now                 time.Time
}

// Note — hisobot qatori.
type Note struct {
	UserID string
	Phone  string
	Role   users.Role
	Reason string
}

// Plan — bajariladigan yozuvlar va hisobot.
type Plan struct {
	Orgs        []*authz.Organization
	Businesses  []*authz.Business
	Memberships []*authz.Membership
	// Skipped — ataylab ko'chirilmaganlar (allaqachon bor, platforma kuryeri).
	Skipped []Note
	// Warnings — qo'lda ko'rib chiqish kerak (bog'lanish yo'q, ikkinchi egasi...).
	Warnings []Note
	// FormerCustomers — xodimga aylantirilgan mijoz akkauntlari (K4): mijoz
	// tarixi xodim akkauntida qolgan. Ajratish alohida qadam.
	FormerCustomers []Note
}

func key(t authz.BusinessType, legacy string) string { return string(t) + "|" + legacy }

// QurtPoolPrefix — qurtorders.QurtPoolPrefix bilan bir xil (import aylanasi
// bo'lmasligi uchun takrorlanmaydi: test ikkalasini solishtiradi).
const QurtPoolPrefix = "qurt:"

func note(u *users.User, reason string) Note {
	return Note{UserID: u.ID, Phone: u.Phone, Role: u.Role, Reason: reason}
}

// builder — `Build` ning ichki holati.
type builder struct {
	in        Input
	p         *Plan
	biz       map[string]string // tur|legacy -> business ID (mavjud + rejadagi)
	orgOf     map[string]string // business ID -> org ID
	hasMember map[string]bool
}

// Build — reja. Kirish o'zgartirilmaydi.
func Build(in Input) *Plan {
	b := &builder{in: in, p: &Plan{}, biz: map[string]string{}, orgOf: map[string]string{}, hasMember: map[string]bool{}}
	for k, v := range in.ExistingBusinesses {
		b.biz[k] = v
	}
	for k, v := range in.ExistingBusinessOrg {
		b.orgOf[k] = v
	}
	for k, v := range in.ExistingMemberships {
		b.hasMember[k] = v
	}
	list := append([]*users.User(nil), in.Users...)
	// Barqaror tartib: eski akkaunt birinchi (bir restoranda ikki egasi
	// bo'lsa — eng eskisi egasi bo'ladi).
	sort.Slice(list, func(i, j int) bool {
		if !list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].CreatedAt.Before(list[j].CreatedAt)
		}
		return list[i].ID < list[j].ID
	})
	for _, u := range list { // 1) egalar (xodimlar ularning biznesiga bog'lanadi)
		b.owner(u)
	}
	for _, u := range list { // 2) xodimlar
		b.staff(u)
	}
	return b.p
}

func (b *builder) warn(u *users.User, reason string) {
	b.p.Warnings = append(b.p.Warnings, note(u, reason))
}
func (b *builder) skip(u *users.User, reason string) {
	b.p.Skipped = append(b.p.Skipped, note(u, reason))
}

// ownerBusiness — egasi akkauntining biznes turi va nomi; egasi emas — false.
func (b *builder) ownerBusiness(u *users.User) (authz.BusinessType, string, bool) {
	switch u.Role {
	case users.RoleRestaurant:
		name := strings.TrimSpace(b.in.RestaurantNames[u.EntityID])
		if name == "" {
			name = strings.TrimSpace(u.Name)
			if u.EntityID != "" {
				b.warn(u, "restoran nomi katalogda topilmadi — akkaunt nomi olindi")
			}
		}
		return authz.TypeRestaurant, name, true
	case users.RoleStore:
		return authz.TypeStore, strings.TrimSpace(u.Name), true
	}
	return "", "", false
}

func (b *builder) owner(u *users.User) {
	t, name, ok := b.ownerBusiness(u)
	if !ok {
		return
	}
	if u.EntityID == "" {
		b.warn(u, "entity_id bo'sh — biznesga bog'lab bo'lmaydi")
		return
	}
	if name == "" {
		name = fmt.Sprintf("%s %s", t, u.EntityID)
	}
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	k := key(t, u.EntityID)
	if _, ok := b.biz[k]; ok {
		if _, existed := b.in.ExistingBusinesses[k]; existed {
			b.skip(u, "biznes allaqachon ko'chirilgan")
		} else {
			b.warn(u, "shu biznesning IKKINCHI egasi akkaunti — egasi eng eski akkaunt, buni qo'lda hal qiling")
		}
		return
	}
	now := b.in.Now
	org := &authz.Organization{ID: b.in.NewID(), Name: name, Status: authz.OrgActive, OwnerUserID: u.ID, CreatedAt: now}
	bz := &authz.Business{ID: b.in.NewID(), OrgID: org.ID, Type: t, LegacyEntityID: u.EntityID, Name: name,
		Status: authz.BusinessActive, CreatedAt: now}
	m := &authz.Membership{ID: b.in.NewID(), UserID: u.ID, OrgID: org.ID, IsOwner: true,
		Status: authz.MemberActive, CreatedAt: now}
	b.p.Orgs, b.p.Businesses, b.p.Memberships = append(b.p.Orgs, org), append(b.p.Businesses, bz), append(b.p.Memberships, m)
	b.biz[k], b.orgOf[bz.ID] = bz.ID, org.ID
}

// staffTarget — xodim qaysi biznesga, qaysi rol va ilova bilan; xodim emas
// yoki ko'chirilmaydi — false (sabab hisobotga yozilgan).
func (b *builder) staffTarget(u *users.User) (t authz.BusinessType, legacy, tplKey, app string, ok bool) {
	switch u.Role {
	case users.RoleWaiter:
		return authz.TypeRestaurant, u.EntityID, "waiter", authz.AppPro, true
	case users.RoleCourier:
		pool, found := b.in.CourierPools[u.EntityID]
		switch {
		case !found:
			b.warn(u, "kuryer yozuvi topilmadi")
		case pool == "":
			b.skip(u, "platforma kuryeri (to'xtatilgan) — ko'chirilmaydi")
		case strings.HasPrefix(pool, QurtPoolPrefix):
			return authz.TypeStore, strings.TrimPrefix(pool, QurtPoolPrefix), "courier", authz.AppGo, true
		default:
			return authz.TypeRestaurant, pool, "courier", authz.AppGo, true
		}
	}
	return "", "", "", "", false
}

func (b *builder) staff(u *users.User) {
	t, legacy, tplKey, app, ok := b.staffTarget(u)
	if !ok {
		return
	}
	if n := b.in.CustomerOrders[u.ID]; n > 0 {
		b.p.FormerCustomers = append(b.p.FormerCustomers,
			note(u, fmt.Sprintf("akkauntda %d ta mijoz buyurtmasi bor (avval mijoz bo'lgan)", n)))
	}
	bizID, found := b.biz[key(t, legacy)]
	if !found || legacy == "" {
		b.warn(u, fmt.Sprintf("%s %q biznesi topilmadi (egasi akkaunti yo'q)", t, legacy))
		return
	}
	if b.hasMember[u.ID+"|"+bizID] {
		b.skip(u, "a'zolik allaqachon bor")
		return
	}
	status, memberID := authz.MemberActive, ""
	if l, ok := b.in.StaffByUser[u.ID]; ok {
		memberID = l.MemberID
		if l.Status != "active" {
			status = authz.MemberSuspended
		}
	}
	b.p.Memberships = append(b.p.Memberships, &authz.Membership{ID: b.in.NewID(), UserID: u.ID, OrgID: b.orgOf[bizID],
		BusinessID: bizID, RoleID: authz.Template{Type: t, Key: tplKey}.ID(), Apps: []string{app},
		Status: status, StaffMemberID: memberID, CreatedAt: b.in.Now})
	b.hasMember[u.ID+"|"+bizID] = true
}

// Summary — inson o'qiydigan hisobot.
func (p *Plan) Summary() string {
	var sb strings.Builder
	owners, staff := 0, 0
	for _, m := range p.Memberships {
		if m.IsOwner {
			owners++
		} else {
			staff++
		}
	}
	fmt.Fprintf(&sb, "Yangi tashkilotlar: %d\nYangi bizneslar:    %d\nEgalik a'zoliklari: %d\nXodim a'zoliklari:  %d\n",
		len(p.Orgs), len(p.Businesses), owners, staff)
	section := func(title string, ns []Note) {
		fmt.Fprintf(&sb, "\n%s: %d\n", title, len(ns))
		for _, n := range ns {
			fmt.Fprintf(&sb, "  - %s %s (%s): %s\n", n.UserID, n.Phone, n.Role, n.Reason)
		}
	}
	section("O'tkazib yuborildi", p.Skipped)
	section("OGOHLANTIRISH (qo'lda ko'ring)", p.Warnings)
	section("Avval mijoz bo'lgan xodimlar (ajratish keyin)", p.FormerCustomers)
	return sb.String()
}
