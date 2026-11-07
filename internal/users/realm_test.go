package users

import "testing"

func TestRealmForRole(t *testing.T) {
	cases := map[Role]Realm{
		RoleCustomer:   RealmCustomer,
		RoleAdmin:      RealmPlatform,
		RoleRestaurant: RealmMerchant,
		RoleStore:      RealmMerchant,
		RoleWaiter:     RealmMerchant,
		RoleCourier:    RealmMerchant,
		RoleMerchant:   RealmMerchant,
		// noma'lum va bo'sh rol — eng kam huquqli maydon (fail closed)
		Role(""):         RealmCustomer,
		Role("superman"): RealmCustomer,
	}
	for role, want := range cases {
		if got := RealmForRole(role); got != want {
			t.Errorf("RealmForRole(%q) = %q, kutilgan %q", role, got, want)
		}
	}
}

// Har bir ma'lum rol aniq bir realmga tushishi kerak — yangi rol qo'shilib,
// realm jadvali unutilsa, u jimgina `customer` bo'lib qolmasin.
func TestRealmForRole_AllKnownRolesMapped(t *testing.T) {
	known := []Role{RoleCustomer, RoleCourier, RoleRestaurant, RoleWaiter, RoleAdmin, RoleStore, RoleMerchant}
	for _, r := range known {
		if !RealmForRole(r).Valid() {
			t.Errorf("%q yaroqsiz realm oldi", r)
		}
	}
	if RealmForRole(RoleCustomer) != RealmCustomer {
		t.Fatal("mijoz customer realmda bo'lishi shart")
	}
	for _, r := range []Role{RoleCourier, RoleRestaurant, RoleWaiter, RoleAdmin, RoleStore, RoleMerchant} {
		if RealmForRole(r) == RealmCustomer {
			t.Errorf("%q mijoz realmiga tushib qoldi — realm.go va 0063 migratsiyasini yangilang", r)
		}
	}
}

func TestRealmValid(t *testing.T) {
	for _, r := range []Realm{RealmCustomer, RealmMerchant, RealmPlatform} {
		if !r.Valid() {
			t.Errorf("%q valid bo'lishi kerak", r)
		}
	}
	for _, r := range []Realm{"", "admin", "Customer"} {
		if r.Valid() {
			t.Errorf("%q valid bo'lmasligi kerak", r)
		}
	}
}
