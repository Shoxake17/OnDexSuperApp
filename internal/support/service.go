package support

import (
	"context"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"chustapp/internal/notify"
)

// WebSocket hodisa turlari.
const (
	EventMessage = "support_message"
	EventRead    = "support_read"
)

// Sender — `ws.Hub` (testda soxta).
type Sender interface {
	Send(key string, v any)
}

// RestaurantTopic — restoran RAHBARIYATI kanali (affitsiant obuna emas).
func RestaurantTopic(restaurantID string) string {
	return notify.Manager(notify.ModuleFood, restaurantID)
}

// AdminTopic — superadmin kanali.
func AdminTopic() string { return notify.Admin() }

type Service struct {
	store Store
	hub   Sender
	idgen func() string
	now   func() time.Time

	// locks — bitta suhbatning yozish+yuborishi QAT'IY ketma-ket: aks
	// holda seq=11 seq=10 dan oldin yetib borib, uzilishdan keyingi
	// "10 dan keyingilari" so'rovi 10 ni abadiy yo'qotardi
	// (`alerts.Service.locks` bilan bir xil sabab).
	locks [64]sync.Mutex
}

func NewService(store Store, hub Sender, idgen func() string) *Service {
	return &Service{store: store, hub: hub, idgen: idgen, now: time.Now}
}

// WithClock — vaqt manbai (testlar uchun).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

func (s *Service) lockFor(restaurantID string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(restaurantID))
	return &s.locks[h.Sum32()%uint32(len(s.locks))]
}

// Contacts — hali to'ldirilmagan bo'lsa bo'sh qiymat (xato emas).
func (s *Service) Contacts(ctx context.Context) (Contacts, error) {
	c, err := s.store.GetContacts(ctx)
	if err != nil || c == nil {
		return Contacts{}, err
	}
	return *c, nil
}

func (s *Service) UpdateContacts(ctx context.Context, in ContactsInput, by string) (Contacts, error) {
	c, err := NormalizeContacts(in)
	if err != nil {
		return Contacts{}, err
	}
	c.UpdatedAt = s.now().UTC()
	c.UpdatedBy = strings.TrimSpace(by)
	if err := s.store.SaveContacts(ctx, &c); err != nil {
		return Contacts{}, err
	}
	return c, nil
}

// Author — xabar yuboruvchi (handler tokendan to'ldiradi).
type Author struct {
	Side   Side
	UserID string
	Name   string
}

// Send — yozadi, keyin ikkala tomonga jonli yuboradi. `img` — handler
// qayta kodlagan rasm (bo'lsa matn ixtiyoriy). `created == false` — shu
// `clientID` bilan xabar avval yozilgan (qayta urinish), qayta yuborilmaydi.
func (s *Service) Send(ctx context.Context, restaurantID string, a Author, body, clientID string, img *ImageInput) (*Message, *Thread, bool, error) {
	restaurantID = strings.TrimSpace(restaurantID)
	if restaurantID == "" || !a.Side.Valid() || strings.TrimSpace(a.UserID) == "" {
		return nil, nil, false, invalid("yuboruvchi noto'g'ri")
	}
	if !ValidClientID(clientID) {
		return nil, nil, false, invalid("client_id noto'g'ri")
	}
	var text string
	var err error
	if img == nil {
		text, err = NormalizeBody(body)
	} else {
		text, err = NormalizeCaption(body)
	}
	if err != nil {
		return nil, nil, false, err
	}
	name := strings.Join(strings.Fields(a.Name), " ")
	switch {
	case a.Side == SideAdmin:
		name = AdminName
	case name == "":
		name = "Restoran"
	}
	if r := []rune(name); len(r) > 80 {
		name = string(r[:80])
	}
	now := s.now().UTC()
	m := &Message{ID: s.idgen(), RestaurantID: restaurantID, Sender: a.Side, SenderID: a.UserID,
		SenderName: name, Body: text, ClientID: clientID, CreatedAt: now}

	var att *Attachment
	if img != nil {
		if len(img.Data) == 0 || len(img.Data) > MaxAttachmentBytes || img.Width <= 0 || img.Height <= 0 {
			return nil, nil, false, invalid("rasm noto'g'ri yoki juda katta")
		}
		att = &Attachment{
			AttachmentMeta: AttachmentMeta{ID: s.idgen(), ContentType: AttachmentContentType,
				Width: img.Width, Height: img.Height, Size: len(img.Data)},
			RestaurantID: restaurantID, Data: img.Data, CreatedAt: now,
		}
		meta := att.AttachmentMeta
		m.Attachment = &meta
	}

	mu := s.lockFor(restaurantID)
	mu.Lock()
	defer mu.Unlock()
	created, err := s.store.InsertMessage(ctx, m, att)
	if err != nil {
		return nil, nil, false, err
	}
	t, err := s.store.GetThread(ctx, restaurantID)
	if err != nil {
		return nil, nil, false, err
	}
	if created && s.hub != nil {
		view := ToMessageView(m)
		s.hub.Send(RestaurantTopic(restaurantID), map[string]any{
			"type": EventMessage, "message": view, "thread": ToThreadView(t, SideRestaurant)})
		s.hub.Send(AdminTopic(), map[string]any{
			"type": EventMessage, "restaurant_id": restaurantID, "message": view, "thread": ToThreadView(t, SideAdmin)})
	}
	return m, t, created, nil
}

// Attachment — faqat shu restoran suhbatidagi rasm (baytlari bilan).
func (s *Service) Attachment(ctx context.Context, restaurantID, id string) (*Attachment, error) {
	if !ValidAttachmentID(id) || strings.TrimSpace(restaurantID) == "" {
		return nil, ErrAttachmentNotFound
	}
	return s.store.GetAttachment(ctx, restaurantID, id)
}

func clampLimit(n int) int {
	if n <= 0 {
		return defaultLimit
	}
	if n > MaxLimit {
		return MaxLimit
	}
	return n
}

// Messages — HAR DOIM o'sish tartibida (eskisidan yangisiga) qaytaradi.
func (s *Service) Messages(ctx context.Context, restaurantID string, q MessageQuery) ([]*Message, error) {
	q.Limit = clampLimit(q.Limit)
	if q.BeforeSeq > 0 && q.AfterSeq > 0 {
		return nil, invalid("before va after birga berilmaydi")
	}
	list, err := s.store.ListMessages(ctx, restaurantID, q)
	if err != nil {
		return nil, err
	}
	if q.AfterSeq == 0 {
		for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
			list[i], list[j] = list[j], list[i]
		}
	}
	return list, nil
}

func (s *Service) Thread(ctx context.Context, restaurantID string) (*Thread, error) {
	return s.store.GetThread(ctx, restaurantID)
}

func (s *Service) Threads(ctx context.Context, limit int) ([]*Thread, error) {
	if limit <= 0 || limit > MaxThreads {
		limit = MaxThreads
	}
	return s.store.ListThreads(ctx, limit)
}

func (s *Service) AdminUnreadTotal(ctx context.Context) (int, error) {
	return s.store.AdminUnreadTotal(ctx)
}

// MarkRead — o'qilgan deb belgilaydi; boshqa ochiq panellar (va qarshi
// tomonning "o'qildi" belgisi) darhol yangilanadi.
func (s *Service) MarkRead(ctx context.Context, restaurantID string, side Side, upToSeq int64) (*Thread, error) {
	if !side.Valid() {
		return nil, invalid("tomon noto'g'ri")
	}
	if upToSeq <= 0 {
		return nil, invalid("up_to_seq musbat bo'lishi kerak")
	}
	mu := s.lockFor(restaurantID)
	mu.Lock()
	defer mu.Unlock()
	changed, err := s.store.MarkRead(ctx, restaurantID, side, upToSeq)
	if err != nil {
		return nil, err
	}
	t, err := s.store.GetThread(ctx, restaurantID)
	if err != nil {
		return nil, err
	}
	if changed && s.hub != nil {
		s.hub.Send(RestaurantTopic(restaurantID), map[string]any{
			"type": EventRead, "side": side, "thread": ToThreadView(t, SideRestaurant)})
		s.hub.Send(AdminTopic(), map[string]any{
			"type": EventRead, "side": side, "restaurant_id": restaurantID, "thread": ToThreadView(t, SideAdmin)})
	}
	return t, nil
}
