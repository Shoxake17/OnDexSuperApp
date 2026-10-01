// Paket e2e — OnDex server oqimlarining TO'LIQ avtomatik sinovi.
//
// Haqiqiy HTTP serveri (`httpapi.New`) xotira repolari va soxta to'lov
// shlyuzi bilan ko'tariladi; ssenariylar mijoz, restoran, kuryer,
// ofitsiant va admin sifatida butun biznes oqimini bosib o'tadi.
//
// ┌─ UCHTA QATLAM ────────────────────────────────────────────────────┐
//  1. SSENARIYLAR (`scenario_*_test.go`) — biznes oqimi, qadam-baqadam.
//  2. INVARIANTLAR (`invariants.go`) — har javobga o'zi qo'llanadigan
//     qoidalar (pul, holat, xavfsizlik).
//  3. QAMROV RATCHETI (`main_test.go`) — har endpoint ssenariyda urilishi
//     yoki `routes_exempt.txt` da SABABI bilan turishi shart. Yangi
//     endpoint testsiz qo'shilsa suite yiqiladi.
//
// Natija: `logs/e2e/report.json` va `report.html` (qadamlar, ms, qamrov).
// └───────────────────────────────────────────────────────────────────┘
//
// Ishga tushirish: `scripts/e2e.ps1` yoki `go test ./internal/e2e/`.
// Yangi ssenariy yozish: `internal/e2e/README.md`.
package e2e

import "chustapp/internal/e2e/report"

var (
	// Rec — butun yugurish davomida yig'iladigan natijalar.
	Rec = report.NewRecorder()
	// Routes — manba koddan o'qilgan endpointlar (`TestMain` to'ldiradi).
	Routes []Route
)
