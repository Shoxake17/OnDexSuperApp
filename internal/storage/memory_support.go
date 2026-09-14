package storage

import (
	"context"
	"sort"
	"sync"

	"chustapp/internal/support"
)

// MemorySupportStore — DATABASE_URL berilmaganda (dev va testlar). Postgres
// bilan bir xil qoidalar: o'suvchi `seq`, (restoran, yuboruvchi, client_id)
// unikal, takroriy xabar rasm yozmaydi, o'qish belgisi faqat oldinga suriladi.
type MemorySupportStore struct {
	mu          sync.RWMutex
	contacts    *support.Contacts
	seq         int64
	messages    []support.Message
	threads     map[string]*support.Thread
	attachments map[string]support.Attachment
}

func NewMemorySupportStore() *MemorySupportStore {
	return &MemorySupportStore{threads: map[string]*support.Thread{}, attachments: map[string]support.Attachment{}}
}

func cloneSupportMessage(m support.Message) support.Message {
	if m.Attachment != nil {
		a := *m.Attachment
		m.Attachment = &a
	}
	return m
}

func (r *MemorySupportStore) GetContacts(_ context.Context) (*support.Contacts, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.contacts == nil {
		return nil, nil
	}
	c := *r.contacts
	return &c, nil
}

func (r *MemorySupportStore) SaveContacts(_ context.Context, c *support.Contacts) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := *c
	r.contacts = &v
	return nil
}

func (r *MemorySupportStore) InsertMessage(_ context.Context, m *support.Message, att *support.Attachment) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, x := range r.messages {
		if x.RestaurantID == m.RestaurantID && x.SenderID == m.SenderID && x.ClientID == m.ClientID {
			*m = cloneSupportMessage(x)
			return false, nil
		}
	}
	if att != nil {
		a := *att
		a.Data = append([]byte(nil), att.Data...)
		r.attachments[a.ID] = a
		meta := a.AttachmentMeta
		m.Attachment = &meta
	}
	r.seq++
	m.Seq = r.seq
	r.messages = append(r.messages, cloneSupportMessage(*m))
	t := r.threads[m.RestaurantID]
	if t == nil {
		t = &support.Thread{RestaurantID: m.RestaurantID, FirstAt: m.CreatedAt}
		r.threads[m.RestaurantID] = t
	}
	t.MessageCount++
	if m.Seq > t.LastSeq {
		t.LastSeq, t.LastAt = m.Seq, m.CreatedAt
	}
	if m.Sender == support.SideAdmin {
		t.AdminReadSeq = max(t.AdminReadSeq, m.Seq)
	} else {
		t.RestaurantReadSeq = max(t.RestaurantReadSeq, m.Seq)
	}
	return true, nil
}

func (r *MemorySupportStore) ListMessages(_ context.Context, restaurantID string, q support.MessageQuery) ([]*support.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []*support.Message
	for _, x := range r.messages {
		if x.RestaurantID != restaurantID || (q.BeforeSeq > 0 && x.Seq >= q.BeforeSeq) ||
			(q.AfterSeq > 0 && x.Seq <= q.AfterSeq) {
			continue
		}
		c := cloneSupportMessage(x)
		out = append(out, &c)
	}
	if q.AfterSeq > 0 {
		sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	} else {
		sort.Slice(out, func(i, j int) bool { return out[i].Seq > out[j].Seq })
	}
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func (r *MemorySupportStore) GetAttachment(_ context.Context, restaurantID, id string) (*support.Attachment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.attachments[id]
	if !ok || a.RestaurantID != restaurantID {
		return nil, support.ErrAttachmentNotFound
	}
	a.Data = append([]byte(nil), a.Data...)
	return &a, nil
}

// threadLocked — xulosa nusxasi, oxirgi xabar va o'qilmaganlar bilan.
func (r *MemorySupportStore) threadLocked(restaurantID string) *support.Thread {
	t := r.threads[restaurantID]
	if t == nil {
		return &support.Thread{RestaurantID: restaurantID}
	}
	c := *t
	for _, x := range r.messages {
		if x.RestaurantID != restaurantID {
			continue
		}
		if x.Seq == c.LastSeq {
			c.LastSender, c.LastBody, c.LastHasImage = x.Sender, x.Body, x.Attachment != nil
		}
		if x.Sender == support.SideAdmin && x.Seq > c.RestaurantReadSeq {
			c.UnreadRestaurant++
		}
		if x.Sender == support.SideRestaurant && x.Seq > c.AdminReadSeq {
			c.UnreadAdmin++
		}
	}
	return &c
}

func (r *MemorySupportStore) GetThread(_ context.Context, restaurantID string) (*support.Thread, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.threadLocked(restaurantID), nil
}

func (r *MemorySupportStore) ListThreads(_ context.Context, limit int) ([]*support.Thread, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*support.Thread, 0, len(r.threads))
	for id := range r.threads {
		out = append(out, r.threadLocked(id))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeq > out[j].LastSeq })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *MemorySupportStore) MarkRead(_ context.Context, restaurantID string, side support.Side, upToSeq int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := r.threads[restaurantID]
	if t == nil {
		return false, nil
	}
	target := min(upToSeq, t.LastSeq)
	ptr := &t.RestaurantReadSeq
	if side == support.SideAdmin {
		ptr = &t.AdminReadSeq
	}
	if target <= *ptr {
		return false, nil
	}
	*ptr = target
	return true, nil
}

func (r *MemorySupportStore) AdminUnreadTotal(_ context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n := 0
	for _, x := range r.messages {
		if t := r.threads[x.RestaurantID]; t != nil && x.Sender == support.SideRestaurant && x.Seq > t.AdminReadSeq {
			n++
		}
	}
	return n, nil
}
