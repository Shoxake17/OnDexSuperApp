package images

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// fakePNGHeader — faqat PNG signature + IHDR chunk'dan iborat "rasm":
// haqiqiy piksel ma'lumoti (IDAT) UMUMAN yo'q, shuning uchun fayl hajmi
// bir necha o'nlab bayt, lekin sarlavhada e'lon qilingan o'lcham istalgancha
// katta bo'lishi mumkin — aynan "decompression bomb" hujum shaklini
// taqlid qiladi (kichik fayl, ulkan e'lon qilingan piksel o'lchami).
// image.DecodeConfig() PNG uchun faqat IHDR'ni o'qiganidan keyin qaytadi —
// IDAT/IEND shart emas — shuning uchun bu "yarim" fayl DecodeConfig uchun
// yetarli, lekin haqiqiy image.Decode() (IDAT yo'qligi sababli) baribir
// xato bilan tugaydi (bizga farqi yo'q — decodeImageSafely o'lcham
// tekshiruvidan O'TKAZMASLIGI kerak, image.Decode()gacha yetib bormasligi).
func fakePNGHeader(width, height uint32) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})

	data := make([]byte, 13)
	binary.BigEndian.PutUint32(data[0:4], width)
	binary.BigEndian.PutUint32(data[4:8], height)
	data[8] = 8  // bit depth
	data[9] = 2  // color type: truecolor
	data[10] = 0 // compression
	data[11] = 0 // filter
	data[12] = 0 // interlace

	chunkType := []byte("IHDR")
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(data)))
	buf.Write(lenBuf[:])
	buf.Write(chunkType)
	buf.Write(data)

	crc := crc32.ChecksumIEEE(append(chunkType, data...))
	var crcBuf [4]byte
	binary.BigEndian.PutUint32(crcBuf[:], crc)
	buf.Write(crcBuf[:])

	return buf.Bytes()
}

func TestDecodeImageSafely_RejectsDecompressionBomb(t *testing.T) {
	huge := fakePNGHeader(30000, 30000) // 900 mln piksel, fayl o'zi ~50 bayt
	if len(huge) > 200 {
		t.Fatalf("test faylining o'zi kutilganidan katta: %d bayt", len(huge))
	}
	_, err := decodeImageSafely(bytes.NewReader(huge))
	if err == nil {
		t.Fatal("30000x30000 rasm rad etilishi kerak edi, lekin qabul qilindi")
	}
	if !strings.Contains(err.Error(), "juda katta") {
		t.Fatalf("xato matni o'lcham chegarasi haqida bo'lishi kerak edi, olindi: %v", err)
	}
}

func TestDecodeImageSafely_AllowsNormalImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("test rasmini kodlashda xato: %v", err)
	}

	decoded, err := decodeImageSafely(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("oddiy 200x100 rasm rad etilmasligi kerak edi: %v", err)
	}
	b := decoded.Bounds()
	if b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("dekodlangan o'lcham noto'g'ri: %dx%d", b.Dx(), b.Dy())
	}
}

func TestProcessProductImage_RejectsDecompressionBomb(t *testing.T) {
	huge := fakePNGHeader(20000, 20000)
	if _, err := ProcessProductImage(bytes.NewReader(huge)); err == nil {
		t.Fatal("ProcessProductImage 20000x20000 rasmni rad etishi kerak edi")
	}
}
