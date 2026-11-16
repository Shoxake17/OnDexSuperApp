// rsprobe — restoran funksional sinovi PROD API'da (egasi ruxsati 2026-11-16).
//
// `cmd/loadtest -mode seed` yaratgan test restoranida (soxta ma'lumot) butun
// oqimni o'tkazadi: bron sozlamasi, stol, bron (Wallet zaklad) + erta bekor
// (to'liq qaytadi), yaqin bron -> keldi (zaklad applied), QR stol buyurtmasi ->
// qabul/tayyor -> ofitsiant "Olaman" -> "Olib bordim", stol hisobi (zaklad
// ayrilgan) -> ofitsiant naqd yopadi, yetkazish xaritasi, hisoblar, choy
// pullari. Karta to'lovi (haqiqiy pul) sinalmaydi.
//
// Ishga tushirish (serverda, loadtest bilan bir xil konteyner usuli):
//
//	rsprobe -api http://ondex-api:8080 -run <run> -dir /data
//
// Chiqish: har qadam PASS/FAIL; birorta FAIL bo'lsa exit 1. Tokenlar
// chiqarilmaydi. Tozalash — `loadtest -mode cleanup`.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

var (
	api    string
	client = &http.Client{Timeout: 20 * time.Second}
	fails  int
)

func call(tok, method, path string, body any) (int, map[string]any, []any) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, api+path, rd)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ondex-Client", "rsprobe")
	res, err := client.Do(req)
	if err != nil {
		return 0, map[string]any{"error": err.Error()}, nil
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var m map[string]any
	var l []any
	if json.Unmarshal(raw, &m) != nil {
		_ = json.Unmarshal(raw, &l)
	}
	return res.StatusCode, m, l
}

func check(name string, ok bool, detail any) {
	if ok {
		fmt.Printf("PASS  %s\n", name)
		return
	}
	fails++
	fmt.Printf("FAIL  %s  -> %v\n", name, detail)
}

func num(m map[string]any, k string) int64 {
	if v, ok := m[k].(float64); ok {
		return int64(v)
	}
	return 0
}

func mustRead(path string, into any) {
	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil || json.Unmarshal(raw, into) != nil {
		fmt.Println("FAIL  fayl o'qilmadi:", filepath.Base(path))
		os.Exit(2)
	}
}

func main() {
	flag.StringVar(&api, "api", "http://ondex-api:8080", "API manzili")
	run := flag.String("run", "", "loadtest run nomi")
	dir := flag.String("dir", "/data", "seed fayllari papkasi")
	flag.Parse()
	var customers []string
	mustRead(filepath.Join(*dir, "tokens-"+*run+".json"), &customers)
	var staff map[string]string
	mustRead(filepath.Join(*dir, "staff-"+*run+".json"), &staff)
	if len(customers) < 2 {
		fmt.Println("FAIL  kamida 2 mijoz kerak")
		os.Exit(2)
	}
	owner, waiter, c0, c1 := staff["owner"], staff["waiter"], customers[0], customers[1]
	rid := "loadtest-" + *run + "-rest"
	tash := time.FixedZone("UZT", 5*3600)

	// 1. Bron sozlamasi: kishi boshiga 10 000 so'm, 2 soat, istalgan payt.
	st, _, _ := call(owner, "PUT", "/restaurants/"+rid+"/reservation-settings", map[string]any{
		"enabled": true, "deposit_per_person_tiyin": 1_000_000, "slot_minutes": 120,
		"min_advance_minutes": 0, "max_days_ahead": 7})
	check("restoran bron sozlamasini yoqadi", st == 200, st)

	// 2. Stol.
	st, tb, _ := call(owner, "POST", "/restaurants/"+rid+"/tables", map[string]any{
		"label": "1", "zone": "Zal", "kind": "table", "capacity": 4})
	tableToken, _ := tb["qr_token"].(string)
	check("stol yaratildi (QR token bilan)", st == 201 && tableToken != "", st)

	// 3. Ertangi bron (Wallet) + erta bekor -> zaklad to'liq qaytadi.
	tomorrow := time.Now().In(tash).Add(24 * time.Hour)
	st, av, _ := call(c0, "GET", fmt.Sprintf("/restaurants/%s/reservations/availability?date=%s&party=2",
		rid, tomorrow.Format("2006-01-02")), nil)
	slots, _ := av["slots"].([]any)
	check("bo'sh vaqtlar ko'rinadi", st == 200 && len(slots) > 0, st)
	if len(slots) > 0 {
		at := slots[len(slots)/2].(map[string]any)["starts_at"]
		st, res, _ := call(c0, "POST", "/restaurants/"+rid+"/reservations", map[string]any{
			"starts_at": at, "party_size": 2, "method": "wallet", "comment": "rsprobe"})
		r, _ := res["reservation"].(map[string]any)
		check("ertangi bron Wallet zakladi bilan tasdiqlandi", st == 201 && r != nil && r["status"] == "confirmed" &&
			r["deposit_state"] == "paid" && num(r, "deposit_tiyin") == 2_000_000, res)
		if r != nil {
			st, c, _ := call(c0, "POST", "/me/reservations/"+r["id"].(string)+"/cancel", nil)
			check("erta bekor qilish — zaklad to'liq qaytdi", st == 200 && c["deposit_state"] == "refunded", c)
		}
	}

	// 4. Yaqin bron (hozirgi kun) -> restoran "Keldi" -> zaklad hisobdan ayriladi.
	today := time.Now().In(tash)
	st, av, _ = call(c1, "GET", fmt.Sprintf("/restaurants/%s/reservations/availability?date=%s&party=2",
		rid, today.Format("2006-01-02")), nil)
	slots, _ = av["slots"].([]any)
	if st == 200 && len(slots) == 0 {
		// Kech tunda bugungi vaqtlar tugagan — ertangi BIRINCHI vaqt (yarim
		// tundan keyin, "Keldi" uchun 3 soatlik oyna ichida bo'lishi mumkin).
		st, av, _ = call(c1, "GET", fmt.Sprintf("/restaurants/%s/reservations/availability?date=%s&party=2",
			rid, today.Add(24*time.Hour).Format("2006-01-02")), nil)
		slots, _ = av["slots"].([]any)
	}
	var seatedID string
	if st == 200 && len(slots) > 0 {
		at := slots[0].(map[string]any)["starts_at"]
		if ts, err := time.Parse(time.RFC3339, fmt.Sprint(at)); err == nil && time.Until(ts) > 3*time.Hour {
			fmt.Println("SKIP  eng yaqin bron vaqti 3 soatdan uzoq — Keldi sinalmaydi:", ts.In(tash).Format("15:04"))
		}
		st, res, _ := call(c1, "POST", "/restaurants/"+rid+"/reservations", map[string]any{
			"starts_at": at, "party_size": 2, "method": "wallet"})
		if r, ok := res["reservation"].(map[string]any); ok && st == 201 {
			seatedID = r["id"].(string)
			st, s, _ := call(owner, "POST", "/reservations/"+seatedID+"/seat", nil)
			check("mehmon keldi — zaklad 'hisobdan ayiring'", st == 200 && s["deposit_state"] == "applied", s)
		} else {
			check("bugungi bron", false, res)
		}
	} else {
		check("bugungi bo'sh vaqt (kech bo'lsa yo'q bo'lishi mumkin)", false, fmt.Sprintf("status=%d slots=%d", st, len(slots)))
	}
	from := url.QueryEscape(today.Add(-24 * time.Hour).UTC().Format(time.RFC3339))
	to := url.QueryEscape(today.Add(48 * time.Hour).UTC().Format(time.RFC3339))
	st, _, list := call(owner, "GET", "/restaurants/"+rid+"/reservations?from="+from+"&to="+to, nil)
	check("panel bronlar ro'yxati", st == 200 && len(list) >= 2, len(list))

	// 5. QR stol buyurtmasi -> tayyor -> ofitsiant Olaman -> Olib bordim.
	st, o, _ := call(c1, "POST", "/orders", map[string]any{
		"items": []map[string]any{{"product_id": "loadtest-" + *run + "-p00", "qty": 2}}, "table_token": tableToken,
		"party_size": 2, "payment_method": "cash", "idempotency_key": "rsprobe-" + *run})
	orderID, _ := o["id"].(string)
	check("mijoz QR orqali stolga buyurtma berdi", st == 201 && o["type"] == "dine_in", st)
	for _, step := range []map[string]any{{"to": "accepted", "preparation_minutes": 5}, {"to": "preparing"}, {"to": "ready"}} {
		st, _, _ = call(owner, "POST", "/orders/"+orderID+"/transition", step)
		check("restoran: "+step["to"].(string), st == 200, st)
	}
	st, cl, _ := call(waiter, "POST", "/waiter/orders/"+orderID+"/claim", nil)
	check("ofitsiant 'Olaman'", st == 200 && cl["mine"] == true, cl)
	st, _, _ = call(waiter, "POST", "/orders/"+orderID+"/transition", map[string]any{"to": "served"})
	check("ofitsiant 'Olib bordim'", st == 200, st)

	// 6. Stol hisobi: zaklad ayrilgan, ofitsiant naqd yopadi.
	st, bill, _ := call(c1, "GET", "/orders/"+orderID+"/bill", nil)
	b, _ := bill["bill"].(map[string]any)
	total, due := num(bill, "total_tiyin"), num(bill, "due_tiyin")
	check("mehmon hisobni ko'radi", st == 200 && b != nil && total == 3_000_000, bill)
	if seatedID != "" {
		check("bron zakladi hisobdan ayrildi", b != nil && num(b, "deposit_tiyin") == 2_000_000 && due == 1_000_000, bill)
	}
	if b != nil {
		billID := b["id"].(string)
		st, _, bl := call(waiter, "GET", "/waiter/bills", nil)
		check("ofitsiant ochiq hisobni ko'radi", st == 200 && len(bl) == 1, len(bl))
		st, cv, _ := call(waiter, "POST", "/bills/"+billID+"/close", map[string]any{"method": "cash"})
		cb, _ := cv["bill"].(map[string]any)
		check("ofitsiant naqd yopdi — hisob to'landi", st == 200 && cb != nil && cb["status"] == "paid", cv)
	}

	// 7. Panel: xarita, hisoblar, choy pullari; bronni yakunlash.
	st, _, _ = call(owner, "GET", "/restaurants/"+rid+"/delivery-map", nil)
	check("yetkazish xaritasi", st == 200, st)
	st, _, _ = call(owner, "GET", "/restaurants/"+rid+"/bills", nil)
	check("panel: ochiq hisoblar", st == 200, st)
	st, _, _ = call(owner, "GET", "/restaurants/"+rid+"/tips?from="+from+"&to="+to, nil)
	check("panel: choy pullari", st == 200, st)
	if seatedID != "" {
		st, _, _ = call(owner, "POST", "/reservations/"+seatedID+"/complete", nil)
		check("bron yakunlandi (stol bo'shadi)", st == 200, st)
	}
	st, _, _ = call(c0, "GET", "/restaurants/"+rid+"/delivery-map", nil)
	check("mijoz restoran xaritasini ko'ra olmaydi", st == 403 || st == 404, st)

	if fails > 0 {
		fmt.Printf("NATIJA: %d ta FAIL\n", fails)
		os.Exit(1)
	}
	fmt.Println("NATIJA: hammasi PASS")
}
