package notify

import (
	"context"
	"slices"
	"sync"
	"testing"
)

// deadPusher — "bu token endi yo'q" deb javob beradigan soxta FCM.
type deadPusher struct {
	mu   sync.Mutex
	dead []string
	seen []string
}

func (p *deadPusher) Push(ctx context.Context, tokens []string, e Event) error {
	_, err := p.PushDetailed(ctx, tokens, e)
	return err
}

func (p *deadPusher) PushDetailed(_ context.Context, tokens []string, _ Event) ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, tokens...)
	return append([]string(nil), p.dead...), nil
}

// recordingTokens — o'chirishni yozib boruvchi token do'koni.
type recordingTokens struct {
	mu      sync.Mutex
	tokens  []string
	deleted []string
}

func (f *recordingTokens) SaveToken(context.Context, string, string, string) error { return nil }

func (f *recordingTokens) DeleteToken(_ context.Context, token string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, token)
	return nil
}

func (f *recordingTokens) TokensFor(context.Context, string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.tokens...), nil
}

func (f *recordingTokens) deletedList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.deleted...)
}

// ★ FCM "UNREGISTERED" qaytargan token BAZADAN O'CHIRILADI.
//
// Avval o'lik token abadiy qolardi: dev telefonida bitta
// foydalanuvchiga 8 ta yig'ilgan va har bir bildirishnoma 8 ta keraksiz
// HTTP so'rovga aylangan edi (api.log, 2026-10-20).
func TestDeadPushTokensAreRemoved(t *testing.T) {
	pusher := &deadPusher{dead: []string{"eski-1", "eski-2"}}
	tokens := &recordingTokens{tokens: []string{"eski-1", "tirik", "eski-2"}}
	svc := newSvc(&memStore{}).WithPush(pusher, tokens)

	svc.Push(context.Background(), "u1", Event{Kind: "order_status"})

	got := tokens.deletedList()
	if len(got) != 2 {
		t.Fatalf("2 ta eskirgan token o'chirilishi kerak edi, o'chdi: %v", got)
	}
	for _, want := range []string{"eski-1", "eski-2"} {
		if !slices.Contains(got, want) {
			t.Fatalf("%q o'chirilmadi (%v)", want, got)
		}
	}
	if slices.Contains(got, "tirik") {
		t.Fatal("ishlayotgan token o'chirib yuborildi")
	}
}

// Oddiy `Pusher` (PushDetailed'siz) o'zgarishsiz ishlaydi —
// `DeadTokenPusher` ixtiyoriy interfeys.
func TestPlainPusherStillWorks(t *testing.T) {
	pusher := &fakePusher{}
	tokens := &recordingTokens{tokens: []string{"t1"}}
	svc := newSvc(&memStore{}).WithPush(pusher, tokens)

	svc.Push(context.Background(), "u1", Event{Kind: "order_status"})

	if pusher.count() != 1 {
		t.Fatal("push yuborilmadi")
	}
	if len(tokens.deletedList()) != 0 {
		t.Fatal("hech narsa o'chirilmasligi kerak edi")
	}
}

// `isUnregistered` FAQAT 404/UNREGISTERED ga "ha" deyishi SHART.
//
// Agar u oddiy 400 ni ham o'lik deb bilsa, xabarning o'zi noto'g'ri
// tuzilgan kunda (bizning xatomiz) butun bazadagi tokenlar o'chib
// ketardi.
func TestIsUnregisteredIsNarrow(t *testing.T) {
	unregistered := []byte(`{"error":{"code":404,"status":"NOT_FOUND",` +
		`"details":[{"errorCode":"UNREGISTERED"}]}}`)
	if !isUnregistered(404, unregistered) {
		t.Fatal("404 UNREGISTERED tanilmadi")
	}
	if isUnregistered(400, []byte(`{"error":{"status":"INVALID_ARGUMENT"}}`)) {
		t.Fatal("400 INVALID_ARGUMENT o'lik token deb hisoblandi")
	}
	if isUnregistered(401, unregistered) {
		t.Fatal("401 (noto'g'ri kalit) o'lik token deb hisoblandi")
	}
	if isUnregistered(404, []byte(`{"error":{"status":"NOT_FOUND"}}`)) {
		t.Fatal("sababsiz 404 o'lik token deb hisoblandi")
	}
}
