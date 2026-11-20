// wswatch — dev vosita: WebSocket'ga ulanib, kelgan eventlarni chiqaradi.
// Mobil ilova qanday eventlar olishini ko'rish uchun.
//
// Ishlatish:  go run ./cmd/wswatch <token> [ws://localhost:8080/ws]
//
// bug.md 64: avval token `?token=` so'rov parametrida yuborilardi — server
// uni hech qachon qabul qilmagan (faqat `Authorization: Bearer` yoki
// `?ticket=`), ya'ni vosita umuman ulanmasdi. Endi mobil ilovalar kabi
// sarlavha orqali yuboriladi.
package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("ishlatish: wswatch <token> [ws://localhost:8080/ws]")
		os.Exit(1)
	}
	url := "ws://localhost:8080/ws"
	if len(os.Args) > 2 {
		url = os.Args[2]
	}
	h := http.Header{}
	h.Set("Authorization", "Bearer "+os.Args[1])
	conn, resp, err := websocket.DefaultDialer.Dial(url, h)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		fmt.Println("ulanish xatosi:", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Println("ulandi, eventlar kutilmoqda...")
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("uzildi:", err)
			return
		}
		fmt.Println("event:", string(msg))
	}
}
