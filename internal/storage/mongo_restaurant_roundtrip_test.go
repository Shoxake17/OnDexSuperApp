package storage

import (
	"reflect"
	"testing"
	"time"

	"chustapp/internal/catalog"
)

// MONGO HUJJATI RESTORANNING HAMMA MAYDONINI SAQLASHI KERAK.
//
// ┌─ NEGA BU TEST YOZILDI ────────────────────────────────────────────┐
// `restaurantDoc` va `toDomain` — QO'LDA yozilgan ro'yxatlar. Domenga
// yangi maydon qo'shilganda ularni yangilash UNUTILSA, maydon
// jimgina yo'qoladi: `SaveRestaurant` xatosiz ishlaydi, `GET` esa nol
// qaytaradi.
//
// Aynan shu 2026-09-30 da sodir bo'ldi — `delivery_fee_tiyin`
// qo'shilgach, PATCH 200 qaytarar, lekin qiymat saqlanmasdi. Uch
// qatlamdagi 11 ta test buni O'TKAZIB YUBORDI, chunki hammasi xotira
// repo'sini ishlatadi (u strukturani butunlay nusxalaydi).
// └───────────────────────────────────────────────────────────────────┘
//
// Test hujjatga BOG'LIQ emas: u domen strukturasini refleksiya bilan
// to'ldiradi, ya'ni yangi maydon qo'shilishi bilan o'zi qamrab oladi.

// computedFields — ATAYLAB saqlanmaydigan maydonlar
// (`Restaurant.WithOpenState` har so'rovda qaytadan hisoblaydi).
var computedFields = map[string]bool{
	"OpenNow": true, "ClosedReason": true, "OpenChangesAt": true,
	// Mijozga ko'rinadigan to'lov usullari (`WithPaymentOptions`).
	"PaymentOptions": true,
}

// fillRestaurant — har bir maydonga NOLDAN FARQLI qiymat qo'yadi.
//
// Nol qiymat yaramaydi: maydon tushib qolsa ham natija nol bo'lardi va
// test o'tib ketardi.
func fillRestaurant(t *testing.T) catalog.Restaurant {
	t.Helper()
	var r catalog.Restaurant
	v := reflect.ValueOf(&r).Elem()
	typ := v.Type()

	for i := range typ.NumField() {
		f := typ.Field(i)
		if computedFields[f.Name] || !v.Field(i).CanSet() {
			continue
		}
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.String:
			fv.SetString("qiymat-" + f.Name)
		case reflect.Bool:
			fv.SetBool(true)
		case reflect.Int, reflect.Int32, reflect.Int64:
			fv.SetInt(int64(1000 + i))
		case reflect.Float64, reflect.Float32:
			fv.SetFloat(float64(i) + 0.5)
		case reflect.Ptr:
			// Ma'lum ikkita ko'rsatkich maydon — qo'lda.
			switch f.Name {
			case "WorkingHours":
				fv.Set(reflect.ValueOf(&catalog.WorkingHours{
					Days: []catalog.DayHours{{Day: 1, Enabled: true, Open: "09:00", Close: "23:00"}},
				}))
			case "PaymentMethods":
				// Har bir bayroq YOQILGAN: yangi usul (Payme/Click/Uzum)
				// Mongo'ga yozilmay qolsa, nol qiymat bilan qaytib, test
				// buni ko'rmay o'tib ketmasligi uchun.
				fv.Set(reflect.ValueOf(&catalog.PaymentMethods{
					Cash: true, CardTerminal: true, CardOnline: true,
					Payme: true, Click: true, Uzum: true,
				}))
			default:
				t.Fatalf("YANGI ko'rsatkich maydon %q — bu testni kengaytiring "+
					"va `restaurantDoc`/`toDomain` da ham hisobga oling", f.Name)
			}
		default:
			t.Fatalf("YANGI turdagi maydon %q (%s) — bu testni kengaytiring "+
				"va `restaurantDoc`/`toDomain` da ham hisobga oling", f.Name, fv.Kind())
		}
	}
	return r
}

func TestMongoRestaurantRoundTripKeepsEveryField(t *testing.T) {
	want := fillRestaurant(t)

	got := restaurantDoc(&want).toDomain()

	wv, gv := reflect.ValueOf(want), reflect.ValueOf(*got)
	typ := wv.Type()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		if computedFields[name] {
			continue
		}
		w, g := wv.Field(i).Interface(), gv.Field(i).Interface()
		if !reflect.DeepEqual(w, g) {
			t.Errorf("%s: Mongo hujjatidan qaytmadi — saqlangan %v, qaytgan %v\n"+
				"  `restaurantDoc` VA `toDomain` ikkalasiga ham qo'shing "+
				"(`mongo_catalog.go`)", name, w, g)
		}
	}
}

// Hisoblanadigan maydonlar SAQLANMASLIGI kerak — ular har so'rovda
// qaytadan hisoblanadi va bazadagi eski qiymat chalg'itardi.
func TestMongoRestaurantDropsComputedFields(t *testing.T) {
	now := time.Now()
	yes := true
	r := catalog.Restaurant{
		ID: "r1", OpenNow: &yes, ClosedReason: "hours", OpenChangesAt: &now,
	}
	got := restaurantDoc(&r).toDomain()
	if got.OpenNow != nil || got.ClosedReason != "" || got.OpenChangesAt != nil {
		t.Errorf("hisoblanadigan maydonlar saqlanib qolgan: %+v", got)
	}
}
