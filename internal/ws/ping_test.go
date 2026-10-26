package ws

import "testing"

// Klientning tiriklik so'rovi TANILISHI, boshqa hech narsa esa
// tanilmasligi kerak.
//
// Avval server klient xabarlarini umuman o'qimasdan tashlardi va
// klient "yarim ochiq" ulanishni (soket ochiq, lekin serverdan hech
// narsa kelmaydi) aniqlay olmasdi.
func TestIsClientPing(t *testing.T) {
	ok := [][]byte{
		[]byte(`{"type":"ping"}`),
		[]byte(`{ "type" : "ping" }`),
		[]byte(`{"type":"ping","seq":12}`),
	}
	for _, m := range ok {
		if !isClientPing(m) {
			t.Fatalf("tiriklik so'rovi tanilmadi: %s", m)
		}
	}

	notPing := [][]byte{
		[]byte(`{"type":"new_order","title":"ping keldi"}`),
		[]byte(`{"type":"pong"}`),
		[]byte(`ping`),
		[]byte(``),
		[]byte(`{"type":`),
		[]byte(`[1,2,3]`),
	}
	for _, m := range notPing {
		if isClientPing(m) {
			t.Fatalf("tiriklik so'rovi emas, lekin tanildi: %s", m)
		}
	}
}
