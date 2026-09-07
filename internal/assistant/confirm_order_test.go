package assistant

import (
	"context"
	"strings"
	"testing"
)

// `confirm_order` — PUL SARFLAYDIGAN yagona ovozli yo'l
// (bug.md 11 va 105-bandlar).
//
// ┌─ NEGA BU TESTLAR ──────────────────────────────────────────────────┐
// 105-band bu amalning IKKALA TOMONDA HAM sinalmaganini ko'rsatgan
// edi. 11-band esa undan yomonroq narsani topdi: server invarianti
// ("buyurtmani odam beradi") to'g'ri edi, LEKIN ilova uni buzardi —
// signalni olib checkout tugmasini dastur bilan bosardi.
//
// Server tomonining invarianti shu yerda qulflanadi: amal
// HECH NARSA yaratmaydi va uning javobi modelga "buyurtma HALI
// berilmagan" deb aytadi. Ilova tomonidagi tuzatish
// `assistant_screen.dart` da (tasdiq dialogi).
// └────────────────────────────────────────────────────────────────────┘

// ASOSIY DA'VO: `confirm_order` serverda hech qanday holat
// o'zgartirmaydi — na buyurtma yaratadi, na bekor qiladi.
func TestConfirmOrderCreatesNothingOnServer(t *testing.T) {
	llm := &fakeLLM{replies: []Reply{
		toolCall("propose_order", `{"items":[{"product_id":"p1","qty":2}]}`),
		toolCall("confirm_order", `{}`),
		{Content: "Ekranda tasdiq oynasi chiqdi."},
	}}
	svc, cancel, ords := newSvc(llm)

	before := len(ords.list)
	if _, err := svc.Chat(context.Background(), "u1", "ha, tasdiqla", nil, nil); err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if len(ords.list) != before {
		t.Fatalf("confirm_order buyurtma YARATDI: %d -> %d", before, len(ords.list))
	}
	if len(cancel.called) != 0 {
		t.Fatalf("confirm_order buyurtma holatini o'zgartirdi: %v", cancel.called)
	}
}

// Javobdagi `note` modelga xulqni AYTADI. Bu shunchaki matn emas:
// model aynan shu matnga qarab foydalanuvchiga nima deyishini
// tanlaydi. "tasdiqlayapman" deb aytish endi NOTO'G'RI bo'lardi —
// ilova faqat oyna ochadi.
func TestConfirmOrderResultSaysOrderNotPlacedYet(t *testing.T) {
	svc, _, _ := newSvc(&fakeLLM{})

	out, err := svc.runTool(context.Background(), "u1", "confirm_order", nil, nil)
	if err != nil {
		t.Fatalf("runTool: %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("kutilmagan natija turi: %T", out)
	}
	if m["ok"] != true {
		t.Fatalf("ok emas: %v", m)
	}
	note, _ := m["note"].(string)
	if note == "" {
		t.Fatal("note bo'sh — model nima deyishini bilmaydi")
	}
	// Buyurtma HALI berilmagani aniq aytilishi kerak.
	if !strings.Contains(strings.ToLower(note), "berilmagan") {
		t.Fatalf("note buyurtma hali berilmaganini aytmaydi: %q", note)
	}
	// Va model "tasdiqladim/berildi" deb aytmasligi kerak.
	if !strings.Contains(strings.ToUpper(note), "AYTMA") {
		t.Fatalf("note modelga nima aytmaslikni ko'rsatmaydi: %q", note)
	}
}

// Tool tavsifi ham yangi xulqni aytishi kerak: model uni O'QIYDI va
// shunga qarab qaror qiladi. Avval tavsif "buyurtmani tasdiqlaydi"
// deb yozilgan edi — bu endi yolg'on bo'lardi.
func TestConfirmOrderDescriptionSaysItDoesNotPlaceOrder(t *testing.T) {
	var desc string
	for _, td := range toolDefs {
		if td.Function.Name == "confirm_order" {
			desc = td.Function.Description
		}
	}
	if desc == "" {
		t.Fatal("confirm_order tavsifi topilmadi")
	}
	if !strings.Contains(desc, "BERMAYDI") {
		t.Fatalf("tavsif buyurtma bermasligini aytmaydi: %q", desc)
	}
	if !strings.Contains(strings.ToLower(desc), "foydalanuvchi") {
		t.Fatalf("tavsif oxirgi qadamni ODAM bosishini aytmaydi: %q", desc)
	}
}

// Tizim ko'rsatmasi ham mos bo'lishi kerak — u va tool tavsifi
// bir-biriga zid bo'lsa, model qaysi biriga ishonishi noma'lum.
func TestSystemPromptDoesNotPromiseConfirmation(t *testing.T) {
	// Model "tasdiqladim" deb aytmasligi ko'rsatmada bo'lsin.
	if !strings.Contains(systemPrompt, "tasdiqladim") {
		t.Error("tizim ko'rsatmasi \"tasdiqladim\" deb aytishni taqiqlamaydi")
	}
	if !strings.Contains(systemPrompt, "TASDIQ OYNASINI") {
		t.Error("tizim ko'rsatmasi confirm_order tasdiq oynasini ochishini aytmaydi")
	}
}
