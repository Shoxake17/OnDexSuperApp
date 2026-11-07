package identity

import (
	"context"
	"log/slog"
	"time"
)

// Audit natijalari (`audit_log.result` CHECK bilan bir xil).
const (
	ResultOK     = "ok"
	ResultDenied = "denied"
	ResultFailed = "failed"
)

// Event — audit jurnali yozuvi (`audit_log`, faqat qo'shiladi).
//
// TOKEN, KOD, PAROL HECH QACHON YOZILMAYDI — faqat identifikatorlar va
// sabablar. IP ochiq saqlanmaydi (HMAC xesh).
type Event struct {
	At          time.Time
	ActorUserID string
	Realm       string
	OrgID       string
	BusinessID  string
	Action      string // masalan "session.create"
	Target      string
	Result      string
	IPHash      []byte
	UserAgent   string
	Details     map[string]any
}

// Auditor — jurnalga yozuvchi. Yozib bo'lmasa xato QAYTARMAYDI (asosiy
// amalni to'xtatmaydi), lekin logga yozadi — implementatsiya ichida.
type Auditor interface {
	Record(ctx context.Context, e Event)
}

// NopAuditor — jurnal ulanmagan holat (faqat testlar).
type NopAuditor struct{}

func (NopAuditor) Record(context.Context, Event) {}

// LogAuditor — Postgres bo'lmagan (xotira) rejim uchun: yozuvni slog'ga.
type LogAuditor struct{}

func (LogAuditor) Record(_ context.Context, e Event) {
	slog.Info("audit", "action", e.Action, "result", e.Result, "actor", e.ActorUserID,
		"realm", e.Realm, "target", e.Target)
}
