package orders

import "testing"

// Bu testlar `NeedsDispatchRecovery` invariantini BUZISHGA urinadi
// (spetsifikatsiyadagi INVARIANT 7.1): "baxtli yo'l" testi bu yerda
// yetarli emas, chunki topilgan ikkala xato ham aynan chekka holatlarda
// edi — stol buyurtmasi va tayyorlash vaqti nol bo'lgan buyurtma.
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
			// Ilgari tiklash uni ham dispatch qilardi va kuryerlarni
			// mavjud bo'lmagan yetkazish uchun bezovta qilardi.
			nom:  "STOL buyurtmasi hech qachon dispatch qilinmaydi",
			ord:  Order{Type: TypeDineIn, Status: StatusAccepted, PreparationMinutes: 20},
			want: false,
		},
		{
			// 2-XATO: tayyorlash vaqti dispatch uchun SHART EMAS —
			// u faqat ETA'ni moslashtiradi. Vaqtni saqlash xato
			// bergan buyurtma ham tiklanishi kerak, aks holda u
			// abadiy kuryersiz qoladi.
			nom:  "tayyorlash vaqti nol bo'lsa ham tiklanadi",
			ord:  Order{Type: TypeDelivery, Status: StatusAccepted, PreparationMinutes: 0},
			want: true,
		},
		{
			// Kuryer allaqachon biriktirilgan — qayta qidirish
			// ikkinchi kuryerni yuborardi.
			nom:  "kuryer bor bo'lsa qayta qidirilmaydi",
			ord:  Order{Type: TypeDelivery, Status: StatusAccepted, CourierID: "c1", PreparationMinutes: 20},
			want: false,
		},
		{
			nom:  "hali qabul qilinmagan",
			ord:  Order{Type: TypeDelivery, Status: StatusCreated, PreparationMinutes: 20},
			want: false,
		},
		{
			// `preparing` dan boshlab dispatch allaqachon ishlagan
			// bo'lishi kerak; bu yerda qayta boshlash noto'g'ri.
			nom:  "tayyorlanmoqda holatida qayta boshlanmaydi",
			ord:  Order{Type: TypeDelivery, Status: StatusPreparing, PreparationMinutes: 20},
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
	// `routes_orders.go:403` dagi shartning aynan nusxasi.
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
