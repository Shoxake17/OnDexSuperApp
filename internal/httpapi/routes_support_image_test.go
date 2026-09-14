package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chustapp/internal/support"
)

// Chatga rasm: multipart, serverda qayta kodlash, egalik bo'yicha berish,
// begona fayllarni rad etish va umumiy 1 MB chegaradan FAQAT shu yo'lning
// istisnosi.

func testPNG(t *testing.T, w, h int, noise bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(7))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.NRGBA{R: uint8(x * 200 / w), G: uint8(y * 200 / h), B: 120, A: 255}
			if noise {
				c.R += uint8(rng.Intn(16))
				c.G += uint8(rng.Intn(16))
				c.B += uint8(rng.Intn(16))
			}
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func postMultipart(t *testing.T, h http.Handler, path, jwt string, fields map[string]string, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	if filename != "" {
		fw, err := mw.CreateFormFile("file", filename)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", path, &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	if jwt != "" {
		r.Header.Set("Authorization", "Bearer "+jwt)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

type imageMsgResp struct {
	Message support.MessageView `json:"message"`
	Thread  support.ThreadView  `json:"thread"`
}

func decodeImageResp(t *testing.T, w *httptest.ResponseRecorder, want int) imageMsgResp {
	t.Helper()
	if w.Code != want {
		t.Fatalf("kutilgan %d, keldi %d — %s", want, w.Code, w.Body.String())
	}
	var r imageMsgResp
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestSupportImageMessages(t *testing.T) {
	h, _, jwt := supportServer(t)
	restPath := "/restaurants/" + testRestA + "/support/messages"

	// Katta (>1 MB) ekran rasmi o'tadi va uzun tomoni 1600 px ga kichrayadi.
	big := testPNG(t, 2000, 800, true)
	if len(big) <= 1<<20 {
		t.Fatalf("test rasmi 1 MB dan kichik chiqdi: %d", len(big))
	}
	first := decodeImageResp(t, postMultipart(t, h, restPath, jwt["a"],
		map[string]string{"body": " Printer xatosi ", "client_id": "image-000001"}, "ekran.PNG", big), http.StatusCreated)
	a := first.Message.Attachment
	if a == nil || a.Width != 1600 || a.Height != 640 || a.ContentType != "image/webp" || a.Size <= 0 ||
		first.Message.Body != "Printer xatosi" {
		t.Fatalf("rasmli xabar: %+v", first.Message)
	}

	// Qayta urinish — o'sha xabar, yangi rasm yozilmaydi.
	again := decodeImageResp(t, postMultipart(t, h, restPath, jwt["a"],
		map[string]string{"body": "Printer xatosi", "client_id": "image-000001"}, "ekran.png", big), http.StatusOK)
	if again.Message.ID != first.Message.ID || again.Message.Attachment == nil || again.Message.Attachment.ID != a.ID {
		t.Fatalf("takror: %+v", again.Message)
	}

	// Faqat rasm (izohsiz).
	only := decodeImageResp(t, postMultipart(t, h, restPath, jwt["a"],
		map[string]string{"client_id": "image-000002"}, "rasm.jpg", testPNG(t, 300, 200, false)), http.StatusCreated)
	if only.Message.Body != "" || only.Message.Attachment == nil || !only.Thread.LastHasImage || only.Thread.LastBody != "Rasm" {
		t.Fatalf("izohsiz rasm: %+v %+v", only.Message, only.Thread)
	}

	// Egasi rasmni oladi: WebP, himoya sarlavhalari, baytlar javobdagi hajmga teng.
	imgPath := "/restaurants/" + testRestA + "/support/attachments/" + a.ID
	w := do(t, h, "GET", imgPath, jwt["a"], "")
	body := w.Body.Bytes()
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "image/webp" ||
		w.Header().Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(w.Header().Get("Cache-Control"), "private") ||
		len(body) < 12 || string(body[:4]) != "RIFF" || string(body[8:12]) != "WEBP" || len(body) != a.Size {
		t.Fatalf("rasm olish: %d %v (%d bayt)", w.Code, w.Header(), len(body))
	}
	if bytes.Contains(body, []byte("tEXt")) || bytes.Contains(body, []byte("IHDR")) {
		t.Fatal("asl PNG baytlari saqlanib qolgan — qayta kodlanmagan")
	}
	if w := do(t, h, "GET", "/admin/support/threads/"+testRestA+"/attachments/"+a.ID, jwt["admin"], ""); w.Code != http.StatusOK {
		t.Fatalf("admin rasmni ololmadi: %d", w.Code)
	}
	for _, c := range []struct {
		path, key string
		want      int
	}{
		{"/restaurants/" + testRestB + "/support/attachments/" + a.ID, "b", http.StatusNotFound},
		{imgPath, "b", http.StatusNotFound},
		{imgPath, "waiter", http.StatusForbidden},
		{imgPath, "customer", http.StatusForbidden},
		{imgPath, "admin", http.StatusForbidden},
		{imgPath, "", http.StatusUnauthorized},
		{"/admin/support/threads/" + testRestB + "/attachments/" + a.ID, "admin", http.StatusNotFound},
		{"/admin/support/threads/" + testRestA + "/attachments/" + a.ID, "a", http.StatusForbidden},
		{"/restaurants/" + testRestA + "/support/attachments/..%2F..%2Fx", "a", http.StatusNotFound},
	} {
		if w := do(t, h, "GET", c.path, jwt[c.key], ""); w.Code != c.want {
			t.Errorf("XAVFSIZLIK: %s %s: kutilgan %d, keldi %d", c.key, c.path, c.want, w.Code)
		}
	}

	// Admin ham rasm yubora oladi.
	adminImg := decodeImageResp(t, postMultipart(t, h, "/admin/support/threads/"+testRestA+"/messages", jwt["admin"],
		map[string]string{"client_id": "admin-img-01", "body": "Mana namuna"}, "namuna.png", testPNG(t, 64, 64, false)), http.StatusCreated)
	if adminImg.Message.Sender != support.SideAdmin || adminImg.Message.Attachment == nil {
		t.Fatalf("admin rasmi: %+v", adminImg.Message)
	}

}

// Rad etish holatlari ALOHIDA serverda: har server o'z tezlik chegarasiga ega
// (akkaunt bo'yicha 10 ta bir zumda), aks holda test himoyaning o'ziga
// (429) urilib, 400 ni tekshira olmasdi.
func TestSupportImageRejections(t *testing.T) {
	h, _, jwt := supportServer(t)
	restPath := "/restaurants/" + testRestA + "/support/messages"
	small := testPNG(t, 40, 30, false)
	for _, c := range []struct {
		name     string
		fields   map[string]string
		filename string
		data     []byte
	}{
		{"matn .png nomi bilan", map[string]string{"client_id": "bad-000001"}, "rasm.png", []byte("bu rasm emas, oddiy matn")},
		{"exe", map[string]string{"client_id": "bad-000002"}, "dastur.exe", small},
		{"svg", map[string]string{"client_id": "bad-000003"}, "logo.svg", []byte("<svg onload=alert(1)></svg>")},
		{"html .jpg nomi bilan", map[string]string{"client_id": "bad-000004"}, "rasm.jpg", []byte("<html><script>x</script>")},
		{"fayl yo'q", map[string]string{"client_id": "bad-000005", "body": "salom"}, "", nil},
		{"begona maydon", map[string]string{"client_id": "bad-000006", "sender": "admin"}, "rasm.png", small},
		{"client_id yo'q", map[string]string{}, "rasm.png", small},
		{"juda katta", map[string]string{"client_id": "bad-000007"}, "katta.png", bytes.Repeat([]byte{0x89}, support.MaxUploadBytes+1)},
	} {
		if w := postMultipart(t, h, restPath, jwt["a"], c.fields, c.filename, c.data); w.Code != http.StatusBadRequest {
			t.Errorf("%s: kutilgan 400, keldi %d %s", c.name, w.Code, w.Body.String())
		}
	}
	// Affitsiant rasm yubora olmaydi.
	if w := postMultipart(t, h, restPath, jwt["waiter"], map[string]string{"client_id": "waiter-img-1"}, "r.png", small); w.Code != http.StatusForbidden {
		t.Errorf("XAVFSIZLIK: affitsiant rasm yubordi: %d", w.Code)
	}

	// Xuddi shu yo'ldagi JSON umumiy 1 MB chegarada QOLADI.
	huge := `{"body":"` + strings.Repeat("a", 2<<20) + `","client_id":"json-big-01"}`
	if w := do(t, h, "POST", restPath, jwt["a"], huge); w.Code != http.StatusBadRequest {
		t.Errorf("katta JSON o'tib ketdi: %d", w.Code)
	}
}

func TestSupportImageUploadExemptionIsNarrow(t *testing.T) {
	for path, want := range map[string]bool{
		"/restaurants/rest-a/support/messages":          true,
		"/admin/support/threads/rest-a/messages":        true,
		"/restaurants/a/b/support/messages":             false,
		"/restaurants//support/messages":                false,
		"/restaurants/rest-a/support/messages/x":        false,
		"/restaurants/rest-a/support/read":              false,
		"/admin/support/threads/a/../b/messages":        false,
		"/restaurants/rest-a/products/support/messages": false,
	} {
		r := httptest.NewRequest("POST", path, nil)
		r.Header.Set("Content-Type", "multipart/form-data; boundary=x")
		if got := isSupportImageUpload(r); got != want {
			t.Errorf("%s: %v, kutilgan %v", path, got, want)
		}
	}
	r := httptest.NewRequest("POST", "/restaurants/rest-a/support/messages", nil)
	r.Header.Set("Content-Type", "application/json")
	if isSupportImageUpload(r) {
		t.Error("JSON so'rov istisnoga tushdi")
	}
	r = httptest.NewRequest("GET", "/restaurants/rest-a/support/messages", nil)
	r.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	if isSupportImageUpload(r) {
		t.Error("GET istisnoga tushdi")
	}
}
