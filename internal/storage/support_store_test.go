package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"chustapp/internal/support"
)

func TestSupportStoreMemory(t *testing.T) {
	runSupportStoreContract(t, NewMemorySupportStore())
}

// TestSupportStorePostgres — HAQIQIY Postgres ustida, VAQTINCHALIK sxemada:
// mavjud jadvallarga tegilmaydi va test oxirida sxema butunlay o'chiriladi.
func TestSupportStorePostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("CI'da TEST_DATABASE_URL BO'LISHI SHART")
		}
		t.Skip("TEST_DATABASE_URL berilmagan — Postgres testi o'tkazib yuborildi")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool: %v", err)
	}
	defer root.Close()
	schema := fmt.Sprintf("support_it_%d", time.Now().UnixNano())
	if _, err := root.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("sxema: %v", err)
	}
	defer func() {
		if _, err := root.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("sxema o'chirilmadi: %v", err)
		}
	}()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, name := range []string{"0050_support.sql", "0051_support_attachments.sql"} {
		sqlBytes, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("migratsiya %s: %v", name, err)
		}
	}
	store := NewPgSupportStore(pool)
	runSupportStoreContract(t, store)

	// Bazaning o'zi ham qoidani ushlaydi: rasmsiz bo'sh matn yozilmaydi.
	if _, err := pool.Exec(ctx, `INSERT INTO support_messages (id, restaurant_id, sender, sender_id, body, client_id)
		VALUES ('raw-1', 'rest-x', 'restaurant', 'u-x', '', 'client-raw-1')`); err == nil {
		t.Fatal("bo'sh matnli rasmsiz xabar bazaga yozildi")
	}
}

func runSupportStoreContract(t *testing.T, store support.Store) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	if c, err := store.GetContacts(ctx); err != nil || c != nil {
		t.Fatalf("bo'sh aloqa: %+v %v", c, err)
	}
	want := support.Contacts{Phone: "+998901234567", PhoneHours: "09:00 – 22:00", Telegram: "ondex_support",
		Email: "support@ondex.uz", EmailNote: "24/7", UpdatedBy: "u-admin", UpdatedAt: now}
	if err := store.SaveContacts(ctx, &want); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetContacts(ctx)
	if err != nil || got == nil || got.Phone != want.Phone || got.Telegram != want.Telegram || got.Email != want.Email ||
		got.PhoneHours != want.PhoneHours || got.EmailNote != want.EmailNote || !got.UpdatedAt.Equal(now) {
		t.Fatalf("aloqa: %+v %v", got, err)
	}
	cleared := support.Contacts{UpdatedAt: now.Add(time.Minute), UpdatedBy: "u-admin"}
	if err := store.SaveContacts(ctx, &cleared); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetContacts(ctx); got == nil || got.Phone != "" || got.Email != "" {
		t.Fatalf("tozalash: %+v", got)
	}

	at := func(sec int) time.Time { return now.Add(time.Duration(sec) * time.Second) }
	insert := func(id, rid string, side support.Side, senderID, clientID, body string, when time.Time,
		att *support.Attachment, wantNew bool) *support.Message {
		t.Helper()
		m := &support.Message{ID: id, RestaurantID: rid, Sender: side, SenderID: senderID, SenderName: "N",
			Body: body, ClientID: clientID, CreatedAt: when}
		created, err := store.InsertMessage(ctx, m, att)
		if err != nil || created != wantNew {
			t.Fatalf("insert %s: created=%v err=%v", id, created, err)
		}
		return m
	}
	image := func(id, rid string, data string) *support.Attachment {
		return &support.Attachment{
			AttachmentMeta: support.AttachmentMeta{ID: id, ContentType: support.AttachmentContentType,
				Width: 640, Height: 480, Size: len(data)},
			RestaurantID: rid, Data: []byte(data), CreatedAt: now,
		}
	}

	m1 := insert("m1", "rest-a", support.SideRestaurant, "u-a", "client-0001", "Salom", at(1), nil, true)
	dup := insert("m1-dup", "rest-a", support.SideRestaurant, "u-a", "client-0001", "Boshqa matn", at(2), nil, false)
	if dup.ID != m1.ID || dup.Body != "Salom" || dup.Seq != m1.Seq || dup.Attachment != nil {
		t.Fatalf("takror mavjud xabarni qaytarmadi: %+v", dup)
	}
	m2 := insert("m2", "rest-a", support.SideAdmin, "u-admin", "client-0001", "Javob", at(3), nil, true)
	m3 := insert("m3", "rest-b", support.SideRestaurant, "u-b", "client-0001", "B dan", at(4), nil, true)
	m4 := insert("m4", "rest-a", support.SideRestaurant, "u-a", "client-0002", "Yana savol", at(5), nil, true)
	m5 := insert("m5", "rest-a", support.SideRestaurant, "u-a", "client-0003", "", at(6), image("att-1", "rest-a", "RIFF-one-WEBP"), true)
	if m5.Attachment == nil || m5.Attachment.ID != "att-1" || m5.Attachment.Size != len("RIFF-one-WEBP") {
		t.Fatalf("rasm metama'lumoti: %+v", m5.Attachment)
	}
	// Takroriy rasmli xabar YANGI rasm yozmaydi.
	dupImg := insert("m5-dup", "rest-a", support.SideRestaurant, "u-a", "client-0003", "", at(7),
		image("att-2", "rest-a", "RIFF-two-WEBP"), false)
	if dupImg.ID != m5.ID || dupImg.Attachment == nil || dupImg.Attachment.ID != "att-1" {
		t.Fatalf("takroriy rasm: %+v", dupImg)
	}
	if _, err := store.GetAttachment(ctx, "rest-a", "att-2"); !errors.Is(err, support.ErrAttachmentNotFound) {
		t.Fatalf("yetim rasm qoldi: %v", err)
	}
	if !(m1.Seq < m2.Seq && m2.Seq < m3.Seq && m3.Seq < m4.Seq && m4.Seq < m5.Seq) {
		t.Fatalf("seq o'smadi: %d %d %d %d %d", m1.Seq, m2.Seq, m3.Seq, m4.Seq, m5.Seq)
	}

	a, err := store.GetAttachment(ctx, "rest-a", "att-1")
	if err != nil || string(a.Data) != "RIFF-one-WEBP" || a.Width != 640 || a.ContentType != support.AttachmentContentType {
		t.Fatalf("rasm: %+v %v", a, err)
	}
	if _, err := store.GetAttachment(ctx, "rest-b", "att-1"); !errors.Is(err, support.ErrAttachmentNotFound) {
		t.Fatalf("XAVFSIZLIK: begona restoran rasmi berildi: %v", err)
	}

	th, err := store.GetThread(ctx, "rest-a")
	if err != nil || th.MessageCount != 4 || th.LastSeq != m5.Seq || th.LastSender != support.SideRestaurant ||
		th.LastBody != "" || !th.LastHasImage || th.RestaurantReadSeq != m5.Seq || th.AdminReadSeq != m2.Seq ||
		th.UnreadRestaurant != 0 || th.UnreadAdmin != 2 || !th.FirstAt.Equal(at(1)) || !th.LastAt.Equal(at(6)) {
		t.Fatalf("suhbat: %+v %v", th, err)
	}
	if none, err := store.GetThread(ctx, "rest-z"); err != nil || none.MessageCount != 0 || none.RestaurantID != "rest-z" {
		t.Fatalf("yo'q suhbat: %+v %v", none, err)
	}

	ids := func(list []*support.Message) string {
		s := ""
		for _, m := range list {
			s += m.ID + ","
		}
		return s
	}
	latest, _ := store.ListMessages(ctx, "rest-a", support.MessageQuery{Limit: 2})
	if ids(latest) != "m5,m4," || latest[0].Attachment == nil || latest[0].Attachment.Height != 480 || latest[1].Attachment != nil {
		t.Fatalf("eng yangilari: %s", ids(latest))
	}
	if list, _ := store.ListMessages(ctx, "rest-a", support.MessageQuery{BeforeSeq: m2.Seq, Limit: 10}); ids(list) != "m1," {
		t.Fatalf("eskiroq: %s", ids(list))
	}
	if list, _ := store.ListMessages(ctx, "rest-a", support.MessageQuery{AfterSeq: m1.Seq, Limit: 10}); ids(list) != "m2,m4,m5," {
		t.Fatalf("keyingilari: %s", ids(list))
	}

	threads, err := store.ListThreads(ctx, 10)
	if err != nil || len(threads) != 2 || threads[0].RestaurantID != "rest-a" || threads[1].RestaurantID != "rest-b" ||
		threads[1].UnreadAdmin != 1 || !threads[0].LastHasImage || threads[1].LastHasImage {
		t.Fatalf("suhbatlar: %+v %v", threads, err)
	}
	if n, err := store.AdminUnreadTotal(ctx); err != nil || n != 3 {
		t.Fatalf("admin o'qilmaganlari: %d %v", n, err)
	}

	if changed, err := store.MarkRead(ctx, "rest-a", support.SideAdmin, 1<<40); err != nil || !changed {
		t.Fatalf("o'qish: %v %v", changed, err)
	}
	if th, _ := store.GetThread(ctx, "rest-a"); th.AdminReadSeq != m5.Seq || th.UnreadAdmin != 0 {
		t.Fatalf("o'qish oxirgi xabargacha qisqarmadi: %+v", th)
	}
	if changed, _ := store.MarkRead(ctx, "rest-a", support.SideAdmin, m1.Seq); changed {
		t.Fatal("o'qish belgisi orqaga ketdi")
	}
	if changed, err := store.MarkRead(ctx, "rest-z", support.SideRestaurant, 5); changed || err != nil {
		t.Fatalf("yo'q suhbat o'qildi: %v %v", changed, err)
	}
	if n, _ := store.AdminUnreadTotal(ctx); n != 1 {
		t.Fatalf("admin o'qilmaganlari (keyin): %d", n)
	}
}
