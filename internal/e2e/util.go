package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"strings"
	"testing"
	"time"

	"chustapp/internal/users"
)

func stringsContains(s, sub string) bool { return strings.Contains(s, sub) }

// MiniPDF — matnli bir sahifali HAQIQIY PDF (xref jadvali bilan).
func MiniPDF(text string) []byte {
	var buf bytes.Buffer
	offsets := make([]int, 0, 5)
	obj := func(s string) {
		offsets = append(offsets, buf.Len())
		buf.WriteString(s)
	}
	buf.WriteString("%PDF-1.4\n")
	obj("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	obj("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")
	obj("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 200] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n")
	stream := "BT /F1 14 Tf 20 100 Td (" + text + ") Tj ET"
	obj(fmt.Sprintf("4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(stream), stream))
	obj("5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>\nendobj\n")
	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 6\n0000000000 65535 f \n")
	for _, o := range offsets {
		fmt.Fprintf(&buf, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xref)
	return buf.Bytes()
}

// emptyAddress — manzili tanlanmagan foydalanuvchi.
func emptyAddress() users.AddressDetails { return users.AddressDetails{} }

// PNG — kichik haqiqiy PNG rasm (yuklash testlari uchun).
func PNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / width), G: uint8(y * 255 / height), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// Upload — multipart so'rov (maydonlar + bitta `file`).
func (w *World) Upload(t *testing.T, a *Actor, path string, fields map[string]string, filename string, data []byte) *Resp {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	w.ctype = mw.FormDataContentType()
	defer func() { w.ctype = "" }()
	return w.Do(t, a, "POST", path, buf.Bytes())
}

// crossSecond — keyingi soniya boshigacha kutadi. Token bekor qilish
// soniya aniqligida: aynan shu soniyada chiqarilgan token ATAYLAB yaroqli
// qoladi (halol qayta-login rad etilmasin), shuning uchun "chiqishdan
// keyin token ishlamaydi" invarianti soniya chegarasidan keyin sinaladi.
func crossSecond() {
	now := time.Now()
	time.Sleep(now.Truncate(time.Second).Add(time.Second).Sub(now) + 30*time.Millisecond)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
