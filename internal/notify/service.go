package notify

import (
	"context"
	"log/slog"
	"time"

	"chustapp/internal/safego"
	"chustapp/internal/ws"
)

// Service — bildirishnomaning YAGONA kirish nuqtasi.
//
// ── OQIM ───────────────────────────────────────────────────────────
//  1. DB'ga yoziladi (yo'qolmasin);
//  2. WebSocket orqali yuboriladi (ilova ochiq bo'lsa — darhol);
//  3. ilova YOPIQ bo'lsa — FCM push.
//
// ┌─ INVARIANT: HTTP SO'ROV YO'LI BLOKLANMAYDI ───────────────────────┐
// Yuborish (2 va 3) ALOHIDA goroutine'da bajariladi. Avval
// `Hub.Send` handler ichida sinxron chaqirilardi va o'lik soket
// uchun 10 soniyagacha kutardi — buyurtma holatini o'zgartirish
// so'rovi shuncha qotardi.
//
// DB yozuvi (1) esa SINXRON: u tez (bitta INSERT) va uning
// muvaffaqiyatli bo'lishi muhim — yozilmagan bildirishnoma keyin
// tiklanmaydi.
// └───────────────────────────────────────────────────────────────────┘
type Service struct {
	store  Store
	hub    *ws.Hub
	pusher Pusher
	tokens TokenStore
	idgen  func() string
	now    func() time.Time
}

func NewService(store Store, hub *ws.Hub, idgen func() string) *Service {
	return &Service{store: store, hub: hub, idgen: idgen, now: time.Now}
}

// WithPush — FCM ulanadi. Chaqirilmasa push yuborilmaydi (qolgan
// hammasi ishlashda davom etadi).
func (s *Service) WithPush(p Pusher, tokens TokenStore) *Service {
	s.pusher = p
	s.tokens = tokens
	return s
}

// Notify — SHAXSIY bildirishnoma: yoziladi + yuboriladi.
//
// `userID` bo'sh bo'lsa jimgina o'tib ketadi (masalan buyurtmada
// hali kuryer biriktirilmagan).
func (s *Service) Notify(ctx context.Context, userID string, e Event) {
	if userID == "" {
		return
	}
	n := &Notification{
		ID:        s.idgen(),
		UserID:    userID,
		Module:    e.Module,
		Kind:      e.Kind,
		Title:     e.Title,
		Body:      e.Body,
		Data:      e.Data,
		CreatedAt: s.now(),
	}
	if s.store != nil {
		if err := s.store.Save(ctx, n); err != nil {
			// Yozib bo'lmadi — yuborishga baribir urinamiz (foydalanuvchi
			// hech bo'lmasa hozir ko'rsin), lekin bu JIDDIY xato.
			slog.Error("notify: bildirishnomani saqlab bo'lmadi",
				"user", userID, "kind", e.Kind, "err", err)
		}
	}

	topic := User(userID)
	online := s.hub != nil && s.hub.Online(topic)

	// Yuborish — ALOHIDA goroutine (yuqoridagi invariant).
	//
	// `context.WithoutCancel` EMAS, yangi kontekst: chaqiruvchi HTTP
	// so'rovi tugaganda uning konteksti bekor qilinadi va push
	// yuborilmay qolardi.
	//
	// `safeGo` — recover BILAN: bu goroutine `net/http` ning panic
	// tutuvchisidan TASHQARIDA ishlaydi, ya'ni bu yerdagi har qanday
	// tutilmagan panic BUTUN jarayonni yiqitadi (bug.md 34-band).
	safeGo("notify.deliver", func() { s.deliver(userID, topic, online, n) })
}

// safeGo — `internal/safego` ustidagi qisqa yorliq. Qo'lda ochilgan
// goroutine'dagi panic butun serverni tugatadi (bug.md 34, 44-bandlar).
func safeGo(name string, fn func()) { safego.Go(name, fn) }

func (s *Service) deliver(userID, topic string, online bool, n *Notification) {
	if s.hub != nil {
		s.hub.Send(topic, map[string]any{
			"type":   "notification",
			"id":     n.ID,
			"module": n.Module,
			"kind":   n.Kind,
			"title":  n.Title,
			"body":   n.Body,
			"data":   n.Data,
		})
	}

	// Ilova OCHIQ bo'lsa push yubormaymiz — foydalanuvchi allaqachon
	// ekranda ko'rdi, ikkinchi marta bezovta qilish keraksiz.
	if online || s.pusher == nil || s.tokens == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tokens, err := s.tokens.TokensFor(ctx, userID)
	if err != nil {
		slog.Warn("notify: push tokenlarini o'qib bo'lmadi", "user", userID, "err", err)
		return
	}
	if len(tokens) == 0 {
		return
	}
	if err := s.pusher.Push(ctx, tokens, Event{
		Module: n.Module, Kind: n.Kind,
		Title: n.Title, Body: n.Body, Data: n.Data,
	}); err != nil {
		slog.Warn("notify: push yuborilmadi", "user", userID, "err", err)
	}
}

// Broadcast — MAVZUGA yuborish (shaxsiy emas): restoran xodimlari,
// buyurtmani kuzatayotganlar va h.k.
//
// DB'ga YOZILMAYDI — bu "hozir ekranda ko'rsat" turidagi jonli
// signal (menyu yangilandi, kuryer joylashuvi), tarixda saqlanishi
// kerak bo'lgan shaxsiy xabar emas.
func (s *Service) Broadcast(topic string, payload any) {
	if s.hub == nil || topic == "" {
		return
	}
	// `Hub.Send` endi o'zi bloklamaydi (navbatga qo'yadi), shuning
	// uchun bu yerda goroutine shart emas.
	s.hub.Send(topic, payload)
}

// Store/Hub — HTTP qatlami uchun (ro'yxat endpointlari).
func (s *Service) Store() Store { return s.store }
