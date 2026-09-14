package orders

import "testing"

// Bu testlar `NeedsDispatchRecovery` invariantini BUZISHGA urinadi
// (spetsifikatsiyadagi INVARIANT 7.1): "baxtli yo'l" testi bu yerda
// yetarli emas, chunki topilgan uchala xato ham aynan chekka holatlarda
// edi — stol buyurtmasi, tayyorlash vaqti nol bo'lgan buyurtma va
// restart paytida oshxonada turgan buyurtma.
func TestNeedsDispatchRecovery(t *testing.T) {
	tests := []struct {
		nom  string
		ord  Order
		want bool
	}{
		{
			// Asosiy holat: restoran qabul qilgan, kuryer hali yo'q.
			nom:  "yetkazish, qabul qilingan, kuryersiz",
			ord:  Order{Type: TypeDelivery, Status: StatusAccepted, PreparationMinutes: 20},
			want: true,
		},
		{
			// Bo'sh Type = delivery (Normalized). Eski buyurtmalar va
			// xotira omboridagi yozuvlar shunday bo'ladi.
			nom:  "turi bo'sh (= yetkazish) ham tiklanadi",
			ord:  Order{Status: StatusAccepted, PreparationMinutes: 20},
			want: true,
		},
		{
			// 1-XATO: stol buyurtmasiga kuryer UMUMAN kerak emas.
			nom:  "STOL buyurtmasi hech qachon dispatch qilinmaydi",
			ord:  Order{Type: TypeDineIn, Status: StatusReady, PreparationMinutes: 20},
			want: false,
		},
		{
			// 2-XATO: tayyorlash vaqti dispatch uchun SHART EMAS.
			nom:  "tayyorlash vaqti nol bo'lsa ham tiklanadi",
			ord:  Order{Type: TypeDelivery, Status: StatusAccepted, PreparationMinutes: 0},
			want: true,
		},
		{
			// 3-XATO: restart paytida oshxonada turgan buyurtma abadiy
			// "Kuryer qidirilmoqda" bo'lib qolardi.
			nom:  "tayyorlanmoqda holatida ham tiklanadi",
			ord:  Order{Type: TypeDelivery, Status: StatusPreparing, DispatchState: DispatchSearching},
			want: true,
		},
		{
			nom:  "tayyor, qidirilmoqda — tiklanadi",
			ord:  Order{Type: TypeDelivery, Status: StatusReady, DispatchState: DispatchSearching},
			want: true,
		},
		{
			// Migratsiyadan oldingi yozuv: holat maydoni bo'sh.
			nom:  "tayyor, holat maydoni bo'sh (eski yozuv) — tiklanadi",
			ord:  Order{Type: TypeDelivery, Status: StatusReady},
			want: true,
		},
		{
			// "Kuryer topilmadi" — qidiruv ATAYLAB to'xtatilgan, restoran
			// qarorini kutadi. Restart uni jimgina qayta boshlamasligi kerak.
			nom:  "kuryer topilmadi holatida qayta boshlanmaydi",
			ord:  Order{Type: TypeDelivery, Status: StatusReady, DispatchState: DispatchNotFound},
			want: false,
		},
		{
			// Kuryer allaqachon biriktirilgan — qayta qidirish
			// ikkinchi kuryerni yuborardi.
			nom:  "kuryer bor bo'lsa qayta qidirilmaydi",
			ord:  Order{Type: TypeDelivery, Status: StatusReady, CourierID: "c1"},
			want: false,
		},
		{
			nom:  "hali qabul qilinmagan",
			ord:  Order{Type: TypeDelivery, Status: StatusCreated, PreparationMinutes: 20},
			want: false,
		},
		{
			nom:  "bekor qilingan buyurtma tiklanmaydi",
			ord:  Order{Type: TypeDelivery, Status: StatusCancelled, PreparationMinutes: 20},
			want: false,
		},
		{
			nom:  "yetkazilgan buyurtma tiklanmaydi",
			ord:  Order{Type: TypeDelivery, Status: StatusDelivered, CourierID: "c1"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.nom, func(t *testing.T) {
			if got := tt.ord.NeedsDispatchRecovery(); got != tt.want {
				t.Fatalf("NeedsDispatchRecovery() = %v, kutilgan %v", got, tt.want)
			}
		})
	}
}

// Tiklash sharti dispatch'ni ISHGA TUSHIRUVCHI shartdan qat'iyroq
// bo'lmasligi kerak: `routes_orders.go` da restoran qabul qilganda
// dispatch FAQAT `!IsDineIn()` bo'yicha filtrlanadi, tayyorlash vaqti
// tekshirilmaydi. Ikkalasi ajralib ketgani uchun xato paydo bo'lgan
// edi — bu test ularni birga ushlab turadi.
func TestRecoveryMirrorsDispatchTrigger(t *testing.T) {
	// `routes_orders.go` dagi shartning aynan nusxasi.
	dispatchTriggered := func(o *Order) bool { return !o.IsDineIn() }

	for _, o := range []Order{
		{Type: TypeDelivery, Status: StatusAccepted, PreparationMinutes: 0},
		{Type: TypeDelivery, Status: StatusAccepted, PreparationMinutes: 15},
		{Status: StatusAccepted, PreparationMinutes: 0},
	} {
		if dispatchTriggered(&o) && !o.NeedsDispatchRecovery() {
			t.Fatalf("qabul qilishda dispatch boshlanadi, lekin restartdan keyin tiklanmaydi: %+v", o)
		}
	}

	// Teskarisi ham: stol buyurtmasi ikkala yo'lda ham rad etilishi kerak.
	dineIn := Order{Type: TypeDineIn, Status: StatusAccepted, PreparationMinutes: 15}
	if dispatchTriggered(&dineIn) || dineIn.NeedsDispatchRecovery() {
		t.Fatal("stol buyurtmasi dispatch qilinmasligi kerak")
	}
}
