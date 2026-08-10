package ws

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// TicketClaims — bilet orqasidagi identifikatsiya (WebSocket ulanishda
// hub.Serve'ga uzatiladigan kalitlar).
type TicketClaims struct {
	Subject  string
	EntityID string
}

// ticketTTL — bilet yaratilgandan keyin qancha vaqt amal qiladi. Juda
// qisqa — bilet faqat WebSocket handshake'ni AYNAN shu payt bajarish
// uchun, uzoq saqlanishi shart emas (uzoq saqlash xavfsizlik foydasini
// yo'qqa chiqarardi).
const ticketTTL = 30 * time.Second

// TicketStore — WebSocket uchun bir martalik, qisqa muddatli ulanish
// biletlari. MUAMMO: brauzer JavaScript WebSocket API'si handshake'da
// maxsus header (Authorization) qo'yishga umuman ruxsat bermaydi — shuning
// uchun ko'p ilovalar (bizniki ham avval shunday edi) uzoq muddatli JWT'ni
// to'g'ridan-to'g'ri ?token=... query orqali yuborishga majbur bo'ladi,
// bu esa token'ning server/proxy loglarida, brauzer tarixida qolib
// ketishiga olib keladi. Yechim (sanoat standarti — "WebSocket ticket"
// naqshi): klient avval oddiy HTTP so'rov bilan (Authorization header
// orqali, xavfsiz) QISQA MUDDATLI, BIR MARTALIK bilet oladi
// (POST /ws/ticket), so'ng WebSocket'ga shu bilet bilan ulanadi — asosiy
// uzoq muddatli JWT hech qachon URL'da ko'rinmaydi, va hatto bilet
// loglarda qolib ketsa ham, u 30 soniyadan keyin (yoki ishlatilgach
// darhol) foydasiz bo'lib qoladi.
type TicketStore struct {
	mu      sync.Mutex
	tickets map[string]ticketEntry
}

type ticketEntry struct {
	claims    TicketClaims
	expiresAt time.Time
}

func NewTicketStore() *TicketStore {
	return &TicketStore{tickets: make(map[string]ticketEntry)}
}

// Issue — yangi bilet yaratadi (32 tasodifiy bayt, hex — taxmin qilib
// bo'lmaydigan). Yo'lda eskirgan biletlarni ham tozalab ketadi (alohida
// fon jarayon/goroutine shart emas — bilet umri juda qisqa, xarita hech
// qachon katta bo'lib ketmaydi).
func (s *TicketStore) Issue(claims TicketClaims) string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand deyarli hech qachon xato bermaydi (OS darajasidagi
		// entropiya manbai) — agar bersa, tizim darajasida jiddiy muammo
		// bor, xavfsiz ishlay olmaymiz.
		panic("crypto/rand ishlamayapti: " + err.Error())
	}
	ticket := hex.EncodeToString(b)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for t, e := range s.tickets {
		if now.After(e.expiresAt) {
			delete(s.tickets, t)
		}
	}
	s.tickets[ticket] = ticketEntry{claims: claims, expiresAt: now.Add(ticketTTL)}
	return ticket
}

// Consume — biletni bir marta ishlatadi: topilsa VA muddati o'tmagan
// bo'lsa, DARHOL o'chiradi (bir martalik — qayta ishlatib bo'lmaydi,
// hatto to'g'ri bo'lsa ham) va claims'ni qaytaradi.
func (s *TicketStore) Consume(ticket string) (TicketClaims, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.tickets[ticket]
	if !ok {
		return TicketClaims{}, false
	}
	delete(s.tickets, ticket)
	if time.Now().After(e.expiresAt) {
		return TicketClaims{}, false
	}
	return e.claims, true
}
