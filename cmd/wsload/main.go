// Buyruq wsload — WebSocket ulanishlar YUKLAMA TESTI: N ta bir vaqtdagi
// ulanishni ochib, ushlab turadi va uzilganlarini sanaydi.
//
//	go run ./cmd/wsload -url ws://localhost:18081/ws -tokens logs/bench/tokens.json -n 10000 -hold 60s
//
// Tokenlar `cmd/benchseed` dan. Tokenlar soni N dan kam bo'lsa, ular
// aylanma ishlatiladi (bitta foydalanuvchi bir nechta ulanish ochadi).
//
// ⚠️ FAQAT lokal server: begona manzil rad etiladi.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	target := flag.String("url", "ws://localhost:18081/ws", "WebSocket manzili")
	tokensFile := flag.String("tokens", "logs/bench/tokens.json", "tokenlar JSON fayli")
	n := flag.Int("n", 1000, "ulanishlar soni")
	rate := flag.Int("rate", 500, "sekundiga nechta yangi ulanish")
	hold := flag.Duration("hold", 30*time.Second, "hammasi ulangach ushlab turish muddati")
	flag.Parse()

	u, err := url.Parse(*target)
	if err != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") {
		fmt.Fprintln(os.Stderr, "RAD ETILDI: faqat lokal server (production'ga yuklama yubormang)")
		os.Exit(2)
	}
	raw, err := os.ReadFile(*tokensFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tokenlar:", err)
		os.Exit(2)
	}
	var tokens []string
	if json.Unmarshal(raw, &tokens) != nil || len(tokens) == 0 {
		fmt.Fprintln(os.Stderr, "tokenlar fayli bo'sh yoki buzuq")
		os.Exit(2)
	}

	var (
		mu        sync.Mutex
		lat       []float64
		failed    atomic.Int64
		dropped   atomic.Int64
		alive     atomic.Int64
		messages  atomic.Int64
		firstFail atomic.Value
		wg        sync.WaitGroup
		stop      = make(chan struct{})
	)
	dialer := websocket.Dialer{HandshakeTimeout: 20 * time.Second}
	tick := time.NewTicker(time.Second / time.Duration(*rate))
	defer tick.Stop()

	start := time.Now()
	for i := 0; i < *n; i++ {
		<-tick.C
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			hdr := http.Header{"Authorization": {"Bearer " + tokens[i%len(tokens)]}}
			t0 := time.Now()
			c, _, err := dialer.Dial(*target, hdr)
			if err != nil {
				failed.Add(1)
				firstFail.CompareAndSwap(nil, err.Error())
				return
			}
			mu.Lock()
			lat = append(lat, float64(time.Since(t0).Microseconds())/1000)
			mu.Unlock()
			alive.Add(1)
			defer c.Close()
			go func() { <-stop; c.Close() }()
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					select {
					case <-stop:
					default:
						dropped.Add(1)
						alive.Add(-1)
					}
					return
				}
				messages.Add(1)
			}
		}(i)
	}
	fmt.Printf("ulanish bosqichi tugadi: %.1f s, ulangan %d, xato %d\n",
		time.Since(start).Seconds(), alive.Load(), failed.Load())
	fmt.Printf("HOLD_START alive=%d\n", alive.Load())
	time.Sleep(*hold)
	fmt.Printf("HOLD_END alive=%d\n", alive.Load())

	close(stop)
	wg.Wait()
	sort.Float64s(lat)
	pct := func(p float64) float64 {
		if len(lat) == 0 {
			return 0
		}
		return lat[int(float64(len(lat)-1)*p)]
	}
	ff, _ := firstFail.Load().(string)
	fmt.Printf("\nWEBSOCKET YUKLAMA: maqsad %d | ulandi %d | ulana olmadi %d | ushlab turishda uzildi %d\n",
		*n, len(lat), failed.Load(), dropped.Load())
	fmt.Printf("ulanish vaqti ms: p50 %.1f | p95 %.1f | p99 %.1f | max %.1f | olingan xabarlar %d\n",
		pct(.5), pct(.95), pct(.99), pct(1), messages.Load())
	if ff != "" {
		fmt.Println("birinchi xato:", ff)
	}
	if failed.Load() > 0 || dropped.Load() > 0 {
		os.Exit(1)
	}
}
