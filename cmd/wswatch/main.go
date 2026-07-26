// wswatch — dev vosita: WebSocket'ga ulanib, kelgan eventlarni chiqaradi.
// Mobil ilova qanday eventlar olishini ko'rish uchun.
//
// Ishlatish:  go run ./cmd/wswatch <token>
package main

import (
	"fmt"
	"os"

	"github.com/gorilla/websocket"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("ishlatish: wswatch <token>")
		os.Exit(1)
	}
	url := "ws://localhost:8080/ws?token=" + os.Args[1]
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
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
