package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"

	"chustapp/internal/catalog"
)

// YETKAZIB BERISH NARXI â€” RESTORANNING O'Z SOZLAMASI.
//
// Restoran o'z kuryeri bilan ishlaydi va narxni o'zi belgilaydi; OnDex
// bu puldan hech narsa olmaydi, ya'ni bu shartnomaga tegmaydigan,
// kundalik sozlama va restoran xodimiga ochiq bo'lishi kerak.

// deliveryFeeResp â€” javobning FAQAT shu testga keraklik qismi.
type deliveryFeeResp struct {
	DeliveryFeeTiyin      int64    `json:"delivery_fee_tiyin"`
	DeliveryFreeFromTiyin int64    `json:"delivery_free_from_tiyin"`
	DeliveryFeeMax        int64    `json:"delivery_fee_max_tiyin"`
	Editable              []string `json:"editable"`
}

func getDeliveryFee(t *testing.T, h http.Handler, token string) deliveryFeeResp {
	t.Helper()
	w := do(t, h, "GET", settingsPathA, token, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET settings: %d â€” %s", w.Code, w.Body.String())
	}
	var s deliveryFeeResp
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDeliveryFeeSettings(t *testing.T) {
	h, _, jwt := settingsServer(t)
	token := jwt["a"]

	// Standart holat â€” BEPUL (0), "belgilanmagan" emas. Mavjud
	// restoranlarda hech narsa o'zgarmaydi.
	got := getDeliveryFee(t, h, token)
	if got.DeliveryFeeTiyin != 0 || got.DeliveryFreeFromTiyin != 0 {
		t.Errorf("standart holat bepul bo'lishi kerak: %+v", got)
	}
	if got.DeliveryFeeMax != catalog.MaxDeliveryFeeTiyin {
		t.Errorf("panel chegarani bilishi kerak: %d", got.DeliveryFeeMax)
	}
	// Maydonlar restoran xodimiga OCHIQ.
	var editable bool
	for _, f := range got.Editable {
		if f == "delivery_fee_tiyin" {
			editable = true
		}
	}
	if !editable {
		t.Errorf("delivery_fee_tiyin tahrirlanadigan maydonlar ro'yxatida yo'q: %v", got.Editable)
	}

	// Saqlash.
	w := do(t, h, "PATCH", settingsPathA, token,
		`{"delivery_fee_tiyin":800000,"delivery_free_from_tiyin":10000000}`)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH: %d â€” %s", w.Code, w.Body.String())
	}
	got = getDeliveryFee(t, h, token)
	if got.DeliveryFeeTiyin != 800_000 || got.DeliveryFreeFromTiyin != 10_000_000 {
		t.Errorf("saqlanmadi: %+v", got)
	}

	// â”Œâ”€ FAQAT BITTA MAYDON YUBORILSA â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”
	// Ikkinchisi SAQLANGAN qiymatida qoladi va tekshiruv ikkalasiga
	// birdan qo'llanadi. Alohida tekshirilsa "bepul chegarasi bor,
	// narx yo'q" kabi ma'nosiz juftlik yozilib qolardi.
	// â””â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”˜
	w = do(t, h, "PATCH", settingsPathA, token, `{"delivery_fee_tiyin":600000}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bitta maydon PATCH: %d â€” %s", w.Code, w.Body.String())
	}
	got = getDeliveryFee(t, h, token)
	if got.DeliveryFeeTiyin != 600_000 {
		t.Errorf("narx yangilanmadi: %d", got.DeliveryFeeTiyin)
	}
	if got.DeliveryFreeFromTiyin != 10_000_000 {
		t.Errorf("tegilmagan maydon o'zgarib ketdi: %d", got.DeliveryFreeFromTiyin)
	}

	// Nolga qaytarish â€” BEPUL qilish. Bu "o'zgarish yo'q" deb
	// o'tkazib yuborilmasligi kerak.
	w = do(t, h, "PATCH", settingsPathA, token, `{"delivery_fee_tiyin":0}`)
	if w.Code != http.StatusOK {
		t.Fatalf("bepul qilish: %d â€” %s", w.Code, w.Body.String())
	}
	if got = getDeliveryFee(t, h, token); got.DeliveryFeeTiyin != 0 {
		t.Errorf("bepul qilinmadi: %d", got.DeliveryFeeTiyin)
	}
}

func TestDeliveryFeeSettingsValidation(t *testing.T) {
	h, _, jwt := settingsServer(t)
	token := jwt["a"]

	for _, c := range []struct {
		name, body string
	}{
		{"manfiy narx", `{"delivery_fee_tiyin":-1}`},
		{"manfiy chegara", `{"delivery_free_from_tiyin":-1}`},
		// Panelda nol ortiqcha yozilib qolgan holat.
		{"juda katta", `{"delivery_fee_tiyin":100000000}`},
		{"son emas", `{"delivery_fee_tiyin":"8000"}`},
	} {
		w := do(t, h, "PATCH", settingsPathA, token, c.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: kod %d, 400 kutilgan â€” %s", c.name, w.Code, w.Body.String())
		}
	}

	// Rad etilgan so'rovdan keyin hech narsa saqlanmagan bo'lishi kerak.
	if got := getDeliveryFee(t, h, token); got.DeliveryFeeTiyin != 0 {
		t.Errorf("xato so'rov qiymatni o'zgartirib yubordi: %d", got.DeliveryFeeTiyin)
	}
}
