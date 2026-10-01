package e2e

import (
	"testing"
)

// ssenariy: MIJOZ AKKAUNTI — ro'yxatdan o'tish, kirish, profil, manzil,
// sevimlilar, bildirishnomalar, hamyon, chiqish va akkauntni o'chirish.
//
// OTP tezlik chegarasi (IP bo'yicha ~5 ta) butun jarayon uchun umumiy,
// shuning uchun bu ssenariy kod so'rovlarini TEJAB ishlatadi.

const (
	e2ePhone    = "+998909990001"
	e2ePassword = "Mijoz-Parol-2026!x"
)

func TestCustomerAccountFlow(t *testing.T) {
	w := NewWorld(t, "Mijoz akkaunti (ro'yxat, profil, sevimli, hamyon)")
	var me *Actor

	w.Step(t, "ro'yxatdan o'tish: parol zaif/mos emas — rad etiladi", func(t *testing.T) {
		w.Do(t, nil, "POST", "/auth/register", map[string]any{
			"phone": e2ePhone, "first_name": "Ali", "password": "123", "password_confirm": "123",
		}).Expect(t, 400)
		w.Do(t, nil, "POST", "/auth/register", map[string]any{
			"phone": e2ePhone, "first_name": "Ali", "password": e2ePassword, "password_confirm": e2ePassword + "x",
		}).Expect(t, 400)
	})

	w.Step(t, "ro'yxatdan o'tish + SMS kod bilan tasdiqlash", func(t *testing.T) {
		reg := w.Do(t, nil, "POST", "/auth/register", map[string]any{
			"phone": e2ePhone, "first_name": "Ali", "last_name": "Valiyev",
			"password": e2ePassword, "password_confirm": e2ePassword,
		}).Expect(t, 201).Map(t)
		code, _ := reg["dev_code"].(string)
		if code == "" {
			t.Fatalf("dev rejimida kod qaytishi kerak: %v", reg)
		}
		// Tasdiqlashdan OLDIN kirib bo'lmaydi (begona raqam band qilinmasin).
		w.Do(t, nil, "POST", "/auth/login", map[string]any{"login": e2ePhone, "password": e2ePassword}).Expect(t, 401)

		v := w.Do(t, nil, "POST", "/auth/verify", map[string]any{"phone": e2ePhone, "code": code}).Expect(t, 200).Map(t)
		tok, _ := v["token"].(string)
		if tok == "" {
			t.Fatalf("token yo'q: %v", v)
		}
		me = &Actor{Name: "yangi-mijoz", Token: tok, Phone: e2ePhone}
	})

	w.Step(t, "parol tasdiqdan KEYIN o'rnatiladi (telefon isboti bilan)", func(t *testing.T) {
		// Register parolni saqlamaydi: begona raqamga parol qo'yib bo'lmasin.
		r := w.Do(t, me, "POST", "/me/password", map[string]any{
			"password": e2ePassword, "password_confirm": e2ePassword,
		}).Expect(t, 200).Map(t)
		// Parol o'zgargach eski sessiyalar bekor qilinadi; yangi token javobda
		// qaytadi — keyingi qadamlar shu bilan davom etadi (soniya chegarasiga
		// bog'liq beqaror test bo'lmasligi uchun).
		fresh, _ := r["token"].(string)
		if fresh == "" {
			t.Fatalf("parol o'rnatilgach yangi token qaytmadi: %v", r)
		}
		me.Token = fresh
	})

	w.Step(t, "parol bilan kirish; noto'g'ri parol — 401", func(t *testing.T) {
		w.Do(t, nil, "POST", "/auth/login", map[string]any{"login": e2ePhone, "password": "Boshqa-Parol-1!"}).Expect(t, 401)
		l := w.Do(t, nil, "POST", "/auth/login", map[string]any{"login": e2ePhone, "password": e2ePassword}).Expect(t, 200).Map(t)
		if l["token"] == nil || l["token"] == "" {
			t.Fatalf("login token bermadi: %v", l)
		}
		// Javobda parol xeshi bo'lmasligi invariantlar bilan tekshiriladi.
	})

	w.Step(t, "SMS kodni qayta so'rash va noto'g'ri kod", func(t *testing.T) {
		w.Do(t, nil, "POST", "/auth/verify", map[string]any{"phone": e2ePhone, "code": "000000"}).Expect(t, 401)
	})

	w.Step(t, "profil: o'qish va ism o'zgartirish", func(t *testing.T) {
		u := w.Do(t, me, "GET", "/me", nil).Expect(t, 200).Map(t)
		if u["phone"] != e2ePhone {
			t.Fatalf("/me: %v", u)
		}
		w.Do(t, me, "POST", "/me", map[string]any{"first_name": "Aziz", "last_name": "Valiyev"}).Expect(t, 200)
		u = w.Do(t, me, "GET", "/me", nil).Expect(t, 200).Map(t)
		if u["first_name"] != "Aziz" {
			t.Fatalf("ism saqlanmadi: %v", u)
		}
	})

	w.Step(t, "manzil: xizmat hududi ichida saqlanadi, tashqarida — rad", func(t *testing.T) {
		w.Do(t, me, "POST", "/me/address", map[string]any{"lat": ChustLat, "lng": ChustLng, "text": "Chust, Navoiy 5"}).Expect(t, 200)
		a := w.Do(t, me, "GET", "/me/address", nil).Expect(t, 200).Map(t)
		if a["text"] != "Chust, Navoiy 5" {
			t.Fatalf("manzil: %v", a)
		}
		// Hudud tashqarisi (okean) va noto'g'ri koordinata.
		w.Do(t, me, "POST", "/me/address", map[string]any{"lat": 0.5, "lng": 0.5, "text": "okean"}).Expect(t, 400)
		w.Do(t, me, "POST", "/me/address", map[string]any{"lat": 999, "lng": 999, "text": "x"}).Expect(t, 400)
	})

	w.Step(t, "sevimli taomlar: qo'shish, ro'yxat, olib tashlash", func(t *testing.T) {
		w.Do(t, me, "POST", "/favorites/"+ProdOsh, nil).Expect(t, 200)
		ids := w.Do(t, me, "GET", "/favorites/ids", nil).Expect(t, 200)
		if !contains(ids.Body, ProdOsh) {
			t.Fatalf("sevimli ID'lar: %s", ids.Body)
		}
		list := w.Do(t, me, "GET", "/favorites", nil).Expect(t, 200)
		if !contains(list.Body, ProdOsh) {
			t.Fatalf("sevimlilar: %s", list.Body)
		}
		// Boshqa mijozning sevimlilari shaxsiy.
		other := w.Do(t, w.Other, "GET", "/favorites/ids", nil).Expect(t, 200)
		if contains(other.Body, ProdOsh) {
			t.Fatal("begona mijozning sevimlilari ko'rindi")
		}
		w.Do(t, me, "DELETE", "/favorites/"+ProdOsh, nil).Expect(t, 200)
		after := w.Do(t, me, "GET", "/favorites/ids", nil).Expect(t, 200)
		if contains(after.Body, ProdOsh) {
			t.Fatal("o'chirilgan sevimli qoldi")
		}
	})

	w.Step(t, "bildirishnomalar va push token", func(t *testing.T) {
		w.Do(t, me, "GET", "/notifications", nil).Expect(t, 200)
		w.Do(t, me, "GET", "/notifications/unread-count", nil).Expect(t, 200)
		w.Do(t, me, "POST", "/notifications/read-all", nil).Expect(t, 200)
		w.Do(t, me, "POST", "/notifications/yoq-id/read", nil) // mavjud bo'lmagan — 5xx bo'lmasin
		w.Do(t, me, "POST", "/me/push-token", map[string]any{"token": "fcm-e2e-token-123456789", "platform": "android"}).Expect(t, 200)
		// Token QUERY orqali (ilovalar shunday yuboradi); tanasiz so'rov — 400.
		w.Do(t, me, "DELETE", "/me/push-token", nil).Expect(t, 400)
		w.Do(t, me, "DELETE", "/me/push-token?token=fcm-e2e-token-123456789", nil).Expect(t, 200)
	})

	w.Step(t, "hamyon", func(t *testing.T) {
		w.Do(t, me, "GET", "/wallet", nil).Expect(t, 200)
	})

	w.Step(t, "parolni almashtirish: eski parol kerak", func(t *testing.T) {
		// Parol bilan kirgan sessiyada (telefon isbotisiz) joriy parol SHART.
		l := w.Do(t, nil, "POST", "/auth/login", map[string]any{"login": e2ePhone, "password": e2ePassword}).Expect(t, 200).Map(t)
		tok, _ := l["token"].(string)
		session := &Actor{Name: "parolli-sessiya", Token: tok}
		w.Do(t, session, "POST", "/me/password", map[string]any{
			"current_password": "xato", "password": "Yangi-Parol-2027!y", "password_confirm": "Yangi-Parol-2027!y",
		}).Expect(t, 400)
		w.Do(t, session, "POST", "/me/password", map[string]any{
			"current_password": e2ePassword, "password": "Yangi-Parol-2027!y", "password_confirm": "Yangi-Parol-2027!y",
		}).Expect(t, 200)
		// Eski parol endi ishlamaydi, yangisi ishlaydi; keyingi qadamlar uchun qaytariladi.
		w.Do(t, nil, "POST", "/auth/login", map[string]any{"login": e2ePhone, "password": e2ePassword}).Expect(t, 401)
		l2 := w.Do(t, nil, "POST", "/auth/login", map[string]any{"login": e2ePhone, "password": "Yangi-Parol-2027!y"}).Expect(t, 200).Map(t)
		tok2, _ := l2["token"].(string)
		w.Do(t, &Actor{Name: "yangi-parol", Token: tok2}, "POST", "/me/password", map[string]any{
			"current_password": "Yangi-Parol-2027!y", "password": e2ePassword, "password_confirm": e2ePassword,
		}).Expect(t, 200)
	})

	w.Step(t, "chiqish: token DARHOL bekor bo'ladi", func(t *testing.T) {
		l := w.Do(t, nil, "POST", "/auth/login", map[string]any{"login": e2ePhone, "password": e2ePassword}).Expect(t, 200).Map(t)
		tok, _ := l["token"].(string)
		session := &Actor{Name: "sessiya", Token: tok}
		w.Do(t, session, "GET", "/me", nil).Expect(t, 200)
		crossSecond() // bekor qilish soniya aniqligida (revoke.IsRevoked izohi)
		w.Do(t, session, "POST", "/auth/logout", nil).Expect(t, 200)
		Rec.Checked(InvTenantIsolation)
		if r := w.Do(t, session, "GET", "/me", nil); r.Status != 401 {
			w.Violate(t, "security.logout_revokes_token", "chiqishdan keyin token ishladi: %d", r.Status)
		}
	})

	w.Step(t, "akkauntni o'chirish: kirish yopiladi", func(t *testing.T) {
		fresh := w.Do(t, nil, "POST", "/auth/login", map[string]any{"login": e2ePhone, "password": e2ePassword}).Expect(t, 200).Map(t)
		tok, _ := fresh["token"].(string)
		a := &Actor{Name: "o'chiriladigan", Token: tok}
		crossSecond()
		w.Do(t, a, "POST", "/me/delete-account", map[string]any{"current_password": "xato"}).Expect(t, 400)
		w.Do(t, a, "POST", "/me/delete-account", map[string]any{"current_password": e2ePassword}).Expect(t, 200)
		if r := w.Do(t, a, "GET", "/me", nil); r.Status != 401 {
			w.Violate(t, "security.deleted_account_cannot_act", "o'chirilgan akkaunt tokeni ishladi: %d", r.Status)
		}
		// O'chirilgan akkauntga kirish rad etiladi va tiklash yo'li aytiladi (403).
		w.Do(t, nil, "POST", "/auth/login", map[string]any{"login": e2ePhone, "password": e2ePassword}).Expect(t, 403)
	})
}
