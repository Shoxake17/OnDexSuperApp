package users

// Realm — akkaunt maydoni (ADR-002, docs/architecture/IDENTITY.md).
//
// ┌─ NEGA KERAK ──────────────────────────────────────────────────────────┐
// OnDex'da uchta BIR-BIRIDAN AJRATILGAN akkaunt maydoni bor:
//   - customer — mijozlar (super app, web);
//   - merchant — biznes egalari va ularning xodimlari (Merchant panel,
//     OnDexPro, OnDexGO);
//   - platform — OnDex egasi (admin).
// Bir realm akkaunti boshqa realm ilovasiga kira OLMAYDI.
// └───────────────────────────────────────────────────────────────────────┘
//
// A1 BOSQICHI (hozir): realm ROLDAN avtomatik hisoblanadi va bazaga yoziladi
// (`RealmForRole`). Hech qanday xatti-harakat o'zgarmaydi — ustun keyingi
// bosqichlar (A3: realmga bog'langan kirish va `UNIQUE(realm, phone)`) uchun
// tayyorlanadi.
type Realm string

const (
	RealmCustomer Realm = "customer"
	RealmMerchant Realm = "merchant"
	RealmPlatform Realm = "platform"
)

// Valid — ma'lum realmmi.
func (r Realm) Valid() bool {
	switch r {
	case RealmCustomer, RealmMerchant, RealmPlatform:
		return true
	}
	return false
}

// RealmForRole — rol qaysi realmga tegishli.
//
// Bu jadval migratsiya 0063 dagi `CASE` bilan AYNAN bir xil bo'lishi SHART
// (`realm_test.go` ikkalasini solishtiradi). Noma'lum rol — `customer`:
// eng kam huquqli maydon (fail closed), merchant yoki platform'ga hech qachon
// tasodifan tushmaydi.
func RealmForRole(role Role) Realm {
	switch role {
	case RoleAdmin:
		return RealmPlatform
	case RoleRestaurant, RoleStore, RoleWaiter, RoleCourier:
		return RealmMerchant
	}
	return RealmCustomer
}
