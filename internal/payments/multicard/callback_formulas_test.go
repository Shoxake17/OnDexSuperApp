package multicard

import (
	"crypto/md5"  //nolint:gosec // test: shlyuz algoritmini mustaqil hisoblaydi
	"crypto/sha1" //nolint:gosec // test: shlyuz algoritmini mustaqil hisoblaydi
	"encoding/hex"
	"encoding/json"
	"strconv"
	"testing"
)

// Multicard IKKI xil callback yuboradi (hujjat): SUCCESS (store_id bor, status yo'q,
// imzo MD5) va WEBHOOK (status bor, imzo SHA1). Haqiqiy sandbox sinovi faqat SHA1
// tekshirilganini ochdi: har bir haqiqiy SUCCESS callback rad etilardi.
func TestVerifyCallbackBothFormulas(t *testing.T) {
	c, err := New(Config{BaseURL: "https://dev-mesh.multicard.uz", ApplicationID: "app", Secret: "sir"})
	if err != nil {
		t.Fatal(err)
	}
	md5hex := func(s string) string { h := md5.Sum([]byte(s)); return hex.EncodeToString(h[:]) }   //nolint:gosec
	sha1hex := func(s string) string { h := sha1.Sum([]byte(s)); return hex.EncodeToString(h[:]) } //nolint:gosec
	amount := int64(3_500_000)
	a := strconv.FormatInt(amount, 10)

	// SUCCESS: MD5(store_id + invoice_id + amount + secret), `status` yo'q.
	success := Callback{StoreID: "6", UUID: "u-1", InvoiceID: "inv-1", Amount: Tiyin(amount)}
	success.Sign = md5hex("6" + "inv-1" + a + "sir")
	if !c.VerifyCallback(success) {
		t.Error("to'g'ri SUCCESS (MD5) imzosi rad etildi")
	}
	if !success.IsSuccessCallback() {
		t.Error("store_id bor, status yo'q xabar SUCCESS deb tanilmadi")
	}

	// WEBHOOK: SHA1(uuid + invoice_id + amount + secret), `status` bor.
	hook := Callback{UUID: "u-1", InvoiceID: "inv-1", Amount: Tiyin(amount), Status: "success"}
	hook.Sign = sha1hex("u-1" + "inv-1" + a + "sir")
	if !c.VerifyCallback(hook) {
		t.Error("to'g'ri WEBHOOK (SHA1) imzosi rad etildi")
	}
	if hook.IsSuccessCallback() {
		t.Error("status bor xabar SUCCESS deb tanildi")
	}

	// Buzilgani rad etiladi (ikkala turda).
	bad := map[string]Callback{
		"success: summa o'zgardi":   {StoreID: "6", UUID: "u-1", InvoiceID: "inv-1", Amount: 1, Sign: success.Sign},
		"success: invoys boshqa":    {StoreID: "6", UUID: "u-1", InvoiceID: "inv-2", Amount: Tiyin(amount), Sign: success.Sign},
		"success: kassa boshqa":     {StoreID: "7", UUID: "u-1", InvoiceID: "inv-1", Amount: Tiyin(amount), Sign: success.Sign},
		"success: imzo bo'sh":       {StoreID: "6", UUID: "u-1", InvoiceID: "inv-1", Amount: Tiyin(amount)},
		"webhook: summa o'zgardi":   {UUID: "u-1", InvoiceID: "inv-1", Amount: 1, Status: "success", Sign: hook.Sign},
		"webhook: MD5 imzosi bilan": {UUID: "u-1", InvoiceID: "inv-1", Amount: Tiyin(amount), Status: "success", Sign: md5hex("u-1" + "inv-1" + a + "sir")},
	}
	for name, cb := range bad {
		if c.VerifyCallback(cb) {
			t.Errorf("%s: buzilgan callback QABUL qilindi", name)
		}
	}
}

// Hujjatdagi HAQIQIY namunalar tahlil qilinadi: `store_id` raqam, `refund_time` null.
func TestCallbackDocExamplesParse(t *testing.T) {
	var success Callback
	if err := json.Unmarshal([]byte(`{"store_id":6,"amount":20000,"invoice_id":"2024864028760",
	  "billing_id":"20241214242009869794410864028760","payment_time":"2024-12-14 14:36:31",
	  "phone":"998930601725","card_pan":"860030******5959","ps":"uzcard",
	  "card_token":"6225f3c93f7a880142782fa4","uuid":"e60d8ebc-b9fe-11ef-b159-005056b4367d",
	  "receipt_url":"https://dev-checkout.multicard.uz/check/e60d8ebc","sign":"553b4292b0f1d8e0e18e6daeb3af3761"}`), &success); err != nil {
		t.Fatal(err)
	}
	if success.StoreID != "6" || success.Amount.Int64() != 20000 || !success.IsSuccessCallback() {
		t.Errorf("SUCCESS namunasi noto'g'ri tahlil qilindi: %+v", success)
	}

	var hook Callback
	if err := json.Unmarshal([]byte(`{"uuid":"e60d8ebc","amount":200000,"invoice_id":"test","status":"progress",
	  "billing_id":"1","payment_time":"2024-12-14 14:36:31","refund_time":null,"phone":"998930601725",
	  "card_pan":"860030******5959","ps":"uzcard","card_token":"x","receipt_url":null,"sign":"5fe1"}`), &hook); err != nil {
		t.Fatal(err)
	}
	if hook.Status != "progress" || hook.StoreID != "" || hook.IsSuccessCallback() {
		t.Errorf("WEBHOOK namunasi noto'g'ri tahlil qilindi: %+v", hook)
	}

	// store_id satr bo'lib kelsa ham o'qiladi.
	var asString Callback
	if err := json.Unmarshal([]byte(`{"store_id":"6","invoice_id":"x","amount":1}`), &asString); err != nil || asString.StoreID != "6" {
		t.Errorf("satr store_id: %v %+v", err, asString)
	}
}
