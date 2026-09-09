package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"chustapp/internal/catalog"
	"chustapp/internal/orders"
	"chustapp/internal/storage"
	"chustapp/internal/tables"
	"chustapp/internal/users"
)

// Stolda ovqatlanish (QR kod) oqimining HTTP darajasidagi testlari.
//
// ┌─ NEGA AYNAN SHU DARAJADA ─────────────────────────────────────────┐
// Holat mashinasi testlari (`orders/statemachine_test.go`) sof
// funksiyani tekshiradi va handler mantig'iga — token yechish,
// restoran mosligi, dispatch to'sig'i — umuman tegmaydi. Aynan
// shunday bo'shliqda Telegram handleridagi xato qolib ketgan edi
// (`routes_auth_telegram_test.go` izohiga qarang).
// └───────────────────────────────────────────────────────────────────┘

const (
	testRestA = "rest-a"
	testRestB = "rest-b"
)

// dineInServer — ikkita restoran, har birida bitta taom va bitta stol.
func dineInServer(t *testing.T) (http.Handler, *tables.Service, string) {
	t.Helper()
	ctx := context.Background()

	userRepo := storage.NewMemoryUserRepo()
	tokens := users.NewTokenIssuer("test-secret", time.Hour)

	customer := &users.User{ID: "cust-1", Phone: "+998901234567",
		Role: users.RoleCustomer, PhoneVerified: true}
	if err := userRepo.Create(ctx, customer); err != nil {
		t.Fatal(err)
	}
	jwt, err := tokens.Issue(customer)
	if err != nil {
		t.Fatal(err)
	}

	catalogRepo := storage.NewMemoryCatalogRepo(
		[]catalog.Restaurant{
			{ID: testRestA, Name: "Restoran A", Open: true},
			{ID: testRestB, Name: "Restoran B", Open: true},
		},
		[]catalog.Product{
			{ID: "prod-a", RestaurantID: testRestA, Name: "Osh", PriceTiyin: 3500000, Available: true},
			{ID: "prod-b", RestaurantID: testRestB, Name: "Lag'mon", PriceTiyin: 3000000, Available: true},
		},
	)
	orderRepo := storage.NewMemoryOrderRepo()
	tableSvc := tables.NewService(storage.NewMemoryTableRepo())

	deps := Deps{
		UserRepo:    userRepo,
		OrderRepo:   orderRepo,
		CatalogRepo: catalogRepo,
		Tokens:      tokens,
		CatalogSvc:  catalog.NewService(catalogRepo),
		OrderSvc:    orders.NewService(orderRepo, nil, NewID, storage.NewMemoryPromotionsRepo()),
		TableSvc:    tableSvc,
		DevMode:     true,
	}
	return New(deps).Routes(nil), tableSvc, jwt
}

// ★ ASOSIY OQIM: QR token bilan buyurtma yaratiladi, manzil SO'RALMAYDI.
//
// Yetkazish oqimida `POST /orders` saqlangan manzil bo'lmasa "avval
// yetkazib berish manzilini tanlang" deb rad etadi. Stolda o'tirgan
// mijozda manzil YO'Q va bo'lishi ham shart emas.
func TestDineInOrderCreatedWithoutAddress(t *testing.T) {
	h, tableSvc, jwt := dineInServer(t)
	table, err := tableSvc.Create(context.Background(), testRestA, "5")
	if err != nil {
		t.Fatal(err)
	}

	body := `{"items":[{"product_id":"prod-a","qty":2}],` +
		`"table_token":"` + table.QRToken + `","party_size":3}`
	w := do(t, h, "POST", "/orders", jwt, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("stol buyurtmasi yaratilmadi: %d — %s", w.Code, w.Body.String())
	}

	var o orders.Order
	if err := json.Unmarshal(w.Body.Bytes(), &o); err != nil {
		t.Fatal(err)
	}
	if o.Type != orders.TypeDineIn {
		t.Errorf("tur `dine_in` bo'lishi kerak, keldi: %q", o.Type)
	}
	if o.TableLabel != "Asosiy zal · 5" {
		t.Errorf("stol nomi nusxalanmadi: %q", o.TableLabel)
	}
	if o.PartySize != 3 {
		t.Errorf("odamlar soni saqlanmadi: %d", o.PartySize)
	}
	if o.TableID != table.ID {
		t.Errorf("stol ID mos emas: %q != %q", o.TableID, table.ID)
	}
}

// ★★ XAVFSIZLIK: "A" stolida o'tirib "B" restoranidan buyurtma qilish.
//
// Savat MIJOZDAN, stol esa IMZOLANGAN tokendan keladi. Ikkalasi bir
// xil restoranga tegishli ekani tekshirilmasa, buyurtma B'ning
// oshxonasiga tushardi, A'ning affitsianti esa uni "5-stol" deb
// ko'rardi — ya'ni bir restoran boshqasining oshxonasiga buyurtma
// yuborishi mumkin bo'lardi.
func TestDineInRejectsCartFromOtherRestaurant(t *testing.T) {
	h, tableSvc, jwt := dineInServer(t)
	tableA, err := tableSvc.Create(context.Background(), testRestA, "5")
	if err != nil {
		t.Fatal(err)
	}

	// A'ning stoli, B'ning taomi.
	body := `{"items":[{"product_id":"prod-b","qty":1}],` +
		`"table_token":"` + tableA.QRToken + `"}`
	w := do(t, h, "POST", "/orders", jwt, body)
	if w.Code == http.StatusCreated {
		t.Fatal("BEGONA restoran taomi bilan stol buyurtmasi YARATILDI — " +
			"bir restoran boshqasining oshxonasiga buyurtma yubora oladi")
	}
	if w.Code != http.StatusBadRequest {
		t.Errorf("kutilgan 400, keldi: %d — %s", w.Code, w.Body.String())
	}
}

// Yaroqsiz token — buyurtma yaratilmaydi.
func TestDineInRejectsUnknownToken(t *testing.T) {
	h, _, jwt := dineInServer(t)
	// 64 belgi (to'g'ri uzunlik), lekin mavjud emas — uzunlik
	// tekshiruvidan o'tib, bazagacha boradi.
	fake := "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
	body := `{"items":[{"product_id":"prod-a","qty":1}],"table_token":"` + fake + `"}`
	w := do(t, h, "POST", "/orders", jwt, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("yaroqsiz token qabul qilindi: %d — %s", w.Code, w.Body.String())
	}
}

// O'chirilgan stol — buyurtma qabul qilinmaydi.
//
// Restoran stolni vaqtincha o'chirganda (ta'mir, mavsumiy ayvon)
// o'sha stolning eski QR kodi bilan buyurtma kelib qolmasligi kerak.
func TestDineInRejectsInactiveTable(t *testing.T) {
	h, tableSvc, jwt := dineInServer(t)
	ctx := context.Background()
	table, err := tableSvc.Create(ctx, testRestA, "5")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tableSvc.SetActive(ctx, table.ID, false); err != nil {
		t.Fatal(err)
	}
	body := `{"items":[{"product_id":"prod-a","qty":1}],"table_token":"` + table.QRToken + `"}`
	w := do(t, h, "POST", "/orders", jwt, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("o'chirilgan stolga buyurtma qabul qilindi: %d", w.Code)
	}
}

// ★★ QR TOKEN ABADIY: stol tahrirlangandan keyin ham O'ZGARMAYDI.
//
// QR kod ilovada emas, MENYU VARAQASIGA chop etilgan va stolda
// turadi (foydalanuvchi qarori). Token o'zgarsa, butun zaldagi
// varaqalar bir zumda ishlamay qolardi va hammasini qayta chop
// etish kerak bo'lardi.
//
// Bu test aynan shu kafolatni qo'riqlaydi: nom o'zgartirilsa ham,
// stol yopilib qayta ochilsa ham — QR ISHLAYVERADI.
func TestQRTokenNeverChanges(t *testing.T) {
	h, tableSvc, jwt := dineInServer(t)
	ctx := context.Background()
	table, err := tableSvc.Create(ctx, testRestA, "5")
	if err != nil {
		t.Fatal(err)
	}
	original := table.QRToken

	// Nomini o'zgartiramiz.
	renamed, err := tableSvc.Rename(ctx, table.ID, "5-A")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.QRToken != original {
		t.Fatalf("nom o'zgartirilgach QR token O'ZGARDI:\neski: %s\nyangi: %s",
			original, renamed.QRToken)
	}

	// Vaqtincha yopib, qayta ochamiz.
	if _, err := tableSvc.SetActive(ctx, table.ID, false); err != nil {
		t.Fatal(err)
	}
	reopened, err := tableSvc.SetActive(ctx, table.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.QRToken != original {
		t.Fatalf("yopib-ochilgach QR token O'ZGARDI: %s", reopened.QRToken)
	}

	// Va eng muhimi: BOSHIDAGI token hamon buyurtma yaratadi.
	body := `{"items":[{"product_id":"prod-a","qty":1}],"table_token":"` + original + `"}`
	w := do(t, h, "POST", "/orders", jwt, body)
	if w.Code != http.StatusCreated {
		t.Fatalf("chop etilgan QR token ishlamay qoldi: %d — %s",
			w.Code, w.Body.String())
	}
}

// ★★ QR AYNAN O'Z STOLINI ochishi kerak — boshqasini emas.
//
// Ikkita stol yaratilib, ikkalasining tokeni ham sinaladi: har biri
// FAQAT o'z stolini qaytarishi shart. Agar token→stol bog'lanishi
// biror joyda chalkashsa (masalan indeks noto'g'ri yangilansa),
// mijoz 5-stolda o'tirib buyurtmasi 7-stolga tushardi va affitsiant
// uni boshqa odamga olib borardi.
func TestQRTokenResolvesToItsOwnTable(t *testing.T) {
	_, tableSvc, _ := dineInServer(t)
	ctx := context.Background()

	t5, err := tableSvc.Create(ctx, testRestA, "5")
	if err != nil {
		t.Fatal(err)
	}
	t7, err := tableSvc.Create(ctx, testRestA, "7")
	if err != nil {
		t.Fatal(err)
	}
	if t5.QRToken == t7.QRToken {
		t.Fatal("ikki stolga BIR XIL token berildi")
	}

	got5, err := tableSvc.Resolve(ctx, t5.QRToken)
	if err != nil {
		t.Fatal(err)
	}
	if got5.ID != t5.ID || got5.Label != "5" {
		t.Fatalf("5-stol tokeni boshqa stolni ochdi: %q", got5.Label)
	}

	got7, err := tableSvc.Resolve(ctx, t7.QRToken)
	if err != nil {
		t.Fatal(err)
	}
	if got7.ID != t7.ID || got7.Label != "7" {
		t.Fatalf("7-stol tokeni boshqa stolni ochdi: %q", got7.Label)
	}
}

// ★★ Bir restoran tokeni BOSHQA restoranga olib bormasligi kerak.
//
// Token butun tizim bo'ylab unikal, shuning uchun u restoranni ham
// bir qiymatli aniqlaydi. Bu test o'sha bog'lanishni qo'riqlaydi.
func TestQRTokenBoundToItsRestaurant(t *testing.T) {
	_, tableSvc, _ := dineInServer(t)
	ctx := context.Background()

	aTable, err := tableSvc.Create(ctx, testRestA, "1")
	if err != nil {
		t.Fatal(err)
	}
	bTable, err := tableSvc.Create(ctx, testRestB, "1") // AYNI nom, boshqa restoran
	if err != nil {
		t.Fatal(err)
	}

	gotA, err := tableSvc.Resolve(ctx, aTable.QRToken)
	if err != nil {
		t.Fatal(err)
	}
	if gotA.RestaurantID != testRestA {
		t.Fatalf("A restorani tokeni %q restoraniga olib bordi", gotA.RestaurantID)
	}
	gotB, err := tableSvc.Resolve(ctx, bTable.QRToken)
	if err != nil {
		t.Fatal(err)
	}
	if gotB.RestaurantID != testRestB {
		t.Fatalf("B restorani tokeni %q restoraniga olib bordi", gotB.RestaurantID)
	}
}

// Odamlar soni chegaradan oshsa rad etiladi (panel/ilova ekranlarini
// buzib yuboradigan qiymatlar).
func TestDineInRejectsAbsurdPartySize(t *testing.T) {
	h, tableSvc, jwt := dineInServer(t)
	table, err := tableSvc.Create(context.Background(), testRestA, "5")
	if err != nil {
		t.Fatal(err)
	}
	body := `{"items":[{"product_id":"prod-a","qty":1}],` +
		`"table_token":"` + table.QRToken + `","party_size":2000000}`
	w := do(t, h, "POST", "/orders", jwt, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("aql bovar qilmas odamlar soni qabul qilindi: %d", w.Code)
	}
}

// Yetkazish buyurtmasi ESKICHA ishlashda davom etishi kerak:
// `table_token` bo'lmasa manzil MAJBURIY.
func TestDeliveryStillRequiresAddress(t *testing.T) {
	h, _, jwt := dineInServer(t)
	body := `{"items":[{"product_id":"prod-a","qty":1}]}`
	w := do(t, h, "POST", "/orders", jwt, body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("manzilsiz YETKAZISH buyurtmasi qabul qilindi: %d — %s",
			w.Code, w.Body.String())
	}
}
