package e2e

import (
	"testing"
)

// ssenariy: RASM YUKLASH — logotip, muqova, chat rasmi; noto'g'ri fayllar rad etiladi.

func TestUploadsAndChatAttachments(t *testing.T) {
	w := NewWorld(t, "Rasm yuklash va chat rasmi")
	var logoURL string

	w.Step(t, "logotip yuklanadi va restoran sozlamasiga qo'yiladi", func(t *testing.T) {
		r := w.Upload(t, w.Owner, "/uploads?type=logo", nil, "logo.png", PNG(64, 64)).Expect(t, 201).Map(t)
		logoURL, _ = r["url"].(string)
		if logoURL == "" {
			t.Fatalf("url yo'q: %v", r)
		}
		w.Do(t, w.Owner, "PATCH", "/restaurants/"+RestA+"/settings", map[string]any{"logo_url": logoURL}).Expect(t, 200)
		pub := w.Do(t, w.Customer, "GET", "/restaurants/"+RestA, nil).Expect(t, 200)
		if !contains(pub.Body, logoURL) {
			t.Fatalf("logotip mijozga ko'rinmadi: %s", pub.Body)
		}
		// Begona (tashqi) manzil logotip bo'la olmaydi.
		w.Do(t, w.Owner, "PATCH", "/restaurants/"+RestA+"/settings",
			map[string]any{"logo_url": "https://evil.example/x.png"}).Expect(t, 400)
	})

	w.Step(t, "muqova va kitob muqovasi", func(t *testing.T) {
		w.Upload(t, w.Owner, "/uploads?type=cover", nil, "cover.png", PNG(240, 100)).Expect(t, 201)
		w.Upload(t, w.Owner, "/uploads/book-cover", nil, "book.png", PNG(100, 150)).Expect(t, 201)
	})

	w.Step(t, "kitob PDF: haqiqiy PDF qabul qilinadi (matnsiz bo'lsa ogohlantirish bilan)", func(t *testing.T) {
		r := w.Upload(t, w.Owner, "/uploads/book-pdf", nil, "kitob.pdf", MiniPDF("Salom dunyo")).Expect(t, 201).Map(t)
		if r["url"] == nil || r["url"] == "" {
			t.Fatalf("PDF manzili yo'q: %v", r)
		}
		if pages, _ := r["pages"].(float64); pages != 1 {
			t.Fatalf("sahifalar soni %v, 1 kutilgan: %v", r["pages"], r)
		}
		// Matn qatlami yo'q PDF (bu yerdagi minimal fayl kutubxona uchun "skanerlangan"):
		// kitob SAQLANADI, lekin admin ogohlantiriladi — jimgina bo'sh matn emas.
		if r["warning"] == nil || r["warning"] == "" {
			t.Fatalf("matnsiz PDF uchun ogohlantirish yo'q: %v", r)
		}
	})

	w.Step(t, "noto'g'ri fayllar rad etiladi (soxta rasm, noto'g'ri tur, ruxsatsiz rol)", func(t *testing.T) {
		w.Upload(t, w.Owner, "/uploads?type=logo", nil, "virus.exe", []byte("MZ....")).Expect(t, 400)
		// Kengaytma .png, ichi esa rasm emas — haqiqiy dekodlash rad etadi.
		w.Upload(t, w.Owner, "/uploads?type=logo", nil, "soxta.png", []byte("bu rasm emas")).Expect(t, 400)
		w.Upload(t, w.Owner, "/uploads/book-pdf", nil, "kitob.pdf", []byte("PDF emas")).Expect(t, 400)
		w.ExpectDenied(t, w.Customer, "POST", "/uploads", nil)
		w.ExpectDenied(t, nil, "POST", "/uploads/book-pdf", nil)
	})

	var attID string
	w.Step(t, "chat: restoran rasm yuboradi", func(t *testing.T) {
		r := w.Upload(t, w.Owner, "/restaurants/"+RestA+"/support/messages",
			map[string]string{"body": "Mana muammo", "client_id": "e2e-img-1"}, "muammo.png", PNG(80, 60)).Expect(t, 201).Map(t)
		msg, _ := r["message"].(map[string]any)
		att, _ := msg["attachment"].(map[string]any)
		attID, _ = att["id"].(string)
		if attID == "" {
			t.Fatalf("rasm biriktirilmadi: %v", r)
		}
		// client_id'siz yoki ortiqcha maydonli so'rov rad etiladi.
		w.Upload(t, w.Owner, "/restaurants/"+RestA+"/support/messages", map[string]string{"body": "x"}, "a.png", PNG(8, 8)).Expect(t, 400)
		w.Upload(t, w.Owner, "/restaurants/"+RestA+"/support/messages",
			map[string]string{"client_id": "e2e-img-2", "hack": "1"}, "a.png", PNG(8, 8)).Expect(t, 400)
	})

	w.Step(t, "rasmni faqat o'z suhbati egasi va admin oladi", func(t *testing.T) {
		got := w.Do(t, w.Owner, "GET", "/restaurants/"+RestA+"/support/attachments/"+attID, nil).Expect(t, 200)
		if len(got.Body) < 50 {
			t.Fatalf("rasm baytlari yo'q: %d bayt", len(got.Body))
		}
		w.Do(t, w.Admin, "GET", "/admin/support/threads/"+RestA+"/attachments/"+attID, nil).Expect(t, 200)
		w.ExpectDenied(t, w.OwnerB, "GET", "/restaurants/"+RestA+"/support/attachments/"+attID, nil)
		w.ExpectDenied(t, w.OwnerB, "GET", "/restaurants/"+RestB+"/support/attachments/"+attID, nil)
		w.ExpectDenied(t, w.Customer, "GET", "/admin/support/threads/"+RestA+"/attachments/"+attID, nil)
	})
}

// ssenariy: SMS ORQALI KIRISH — kod so'rash va tasdiqlash (yangi telefon avtomatik mijoz).
func TestSMSLoginFlow(t *testing.T) {
	w := NewWorld(t, "SMS orqali kirish (kod so'rash + tasdiqlash)")
	const phone = "+998909990002"

	w.Step(t, "kod so'raladi va tasdiqlanadi", func(t *testing.T) {
		r := w.Do(t, nil, "POST", "/auth/request-code", map[string]any{"phone": phone}).Expect(t, 200).Map(t)
		code, _ := r["dev_code"].(string)
		if code == "" {
			t.Fatalf("dev rejimida kod qaytishi kerak: %v", r)
		}
		w.Do(t, nil, "POST", "/auth/verify", map[string]any{"phone": phone, "code": "000000"}).Expect(t, 401)
		v := w.Do(t, nil, "POST", "/auth/verify", map[string]any{"phone": phone, "code": code}).Expect(t, 200).Map(t)
		tok, _ := v["token"].(string)
		me := &Actor{Name: "sms-mijoz", Token: tok}
		u := w.Do(t, me, "GET", "/me", nil).Expect(t, 200).Map(t)
		if u["role"] != "customer" {
			t.Fatalf("yangi akkaunt mijoz bo'lishi kerak: %v", u)
		}
		// Kod BIR MARTALIK: qayta ishlatib bo'lmaydi.
		w.Do(t, nil, "POST", "/auth/verify", map[string]any{"phone": phone, "code": code}).Expect(t, 401)
	})

	w.Step(t, "noto'g'ri raqam rad etiladi", func(t *testing.T) {
		w.Do(t, nil, "POST", "/auth/request-code", map[string]any{"phone": "123"}).Expect(t, 400)
	})
}
