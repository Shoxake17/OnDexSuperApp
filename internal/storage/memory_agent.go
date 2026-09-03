package storage

import (
	"context"
	"sort"
	"sync"
	"time"

	"chustapp/internal/agentapi"
)

// MemoryAgentRepo — agent integratsiyasining xotiradagi ombori.
//
// Ikki joyda ishlatiladi: testlarda va `DATABASE_URL` berilmagan dev
// rejimda. Postgres versiyasi bilan BIR XIL semantikaga ega bo'lishi
// shart — aks holda testlar o'tib, production'da sinardi. Shu sababli
// bu yerda ham egalik/holat tekshiruvlari o'sha tartibda bajariladi.
type MemoryAgentRepo struct {
	mu       sync.RWMutex
	partners map[string]agentapi.Partner
	// keyIndex — sha256(kalit) -> partner ID. Postgres'dagi unikal
	// indeksning o'rnini bosadi.
	keyIndex map[string]string
	links    map[string]agentapi.LinkRequest
	grants   map[string]agentapi.Grant
	drafts   map[string]agentapi.Draft
	audit    []agentapi.AuditEntry
	auditSeq int64
}

func NewMemoryAgentRepo() *MemoryAgentRepo {
	return &MemoryAgentRepo{
		partners: make(map[string]agentapi.Partner),
		keyIndex: make(map[string]string),
		links:    make(map[string]agentapi.LinkRequest),
		grants:   make(map[string]agentapi.Grant),
		drafts:   make(map[string]agentapi.Draft),
	}
}

// ── Sheriklar ──

func (r *MemoryAgentRepo) CreatePartner(_ context.Context, p *agentapi.Partner, keyHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.partners[p.ID] = *p
	r.keyIndex[keyHash] = p.ID
	return nil
}

func (r *MemoryAgentRepo) PartnerByKeyHash(_ context.Context, keyHash string) (*agentapi.Partner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.keyIndex[keyHash]
	if !ok {
		return nil, agentapi.ErrNotFound
	}
	p, ok := r.partners[id]
	if !ok {
		return nil, agentapi.ErrNotFound
	}
	return &p, nil
}

func (r *MemoryAgentRepo) PartnerByID(_ context.Context, id string) (*agentapi.Partner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.partners[id]
	if !ok {
		return nil, agentapi.ErrNotFound
	}
	return &p, nil
}

func (r *MemoryAgentRepo) ListPartners(_ context.Context) ([]*agentapi.Partner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*agentapi.Partner, 0, len(r.partners))
	for _, p := range r.partners {
		cp := p
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *MemoryAgentRepo) SetPartnerActive(_ context.Context, id string, active bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.partners[id]
	if !ok {
		return agentapi.ErrNotFound
	}
	p.Active = active
	if !active {
		now := time.Now()
		p.RevokedAt = &now
	} else {
		p.RevokedAt = nil
	}
	r.partners[id] = p
	return nil
}

// ── Ulanish so'rovlari ──

func (r *MemoryAgentRepo) CreateLink(_ context.Context, l *agentapi.LinkRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.links[l.ID] = *l
	return nil
}

func (r *MemoryAgentRepo) LinkByID(_ context.Context, id string) (*agentapi.LinkRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.links[id]
	if !ok {
		return nil, agentapi.ErrNotFound
	}
	return &l, nil
}

func (r *MemoryAgentRepo) LinkByCodeHash(_ context.Context, codeHash string) (*agentapi.LinkRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, l := range r.links {
		if l.CodeHash == codeHash {
			cp := l
			return &cp, nil
		}
	}
	return nil, agentapi.ErrNotFound
}

func (r *MemoryAgentRepo) UpdateLink(_ context.Context, l *agentapi.LinkRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.links[l.ID]; !ok {
		return agentapi.ErrNotFound
	}
	r.links[l.ID] = *l
	return nil
}

// ── Grantlar ──

func (r *MemoryAgentRepo) CreateGrant(_ context.Context, g *agentapi.Grant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.grants[g.ID] = *g
	return nil
}

func (r *MemoryAgentRepo) GrantByID(_ context.Context, id string) (*agentapi.Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.grants[id]
	if !ok {
		return nil, agentapi.ErrNotFound
	}
	return &g, nil
}

// GrantByTokenHash — bo'sh hash bilan qidiruv RAD ETILADI.
//
// Bu ehtiyot chorasi emas, HAQIQIY xavf: grant yaratilganda
// `token_hash` bir muddat bo'sh turadi (token faqat sherik uni olib
// ketganda hosil bo'ladi). Bo'sh qiymat bilan qidirishga yo'l
// qo'yilsa, hali hech kimga berilmagan grant topilib qolardi.
func (r *MemoryAgentRepo) GrantByTokenHash(_ context.Context, tokenHash string) (*agentapi.Grant, error) {
	if tokenHash == "" {
		return nil, agentapi.ErrNotFound
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, g := range r.grants {
		if g.TokenHash == tokenHash {
			cp := g
			return &cp, nil
		}
	}
	return nil, agentapi.ErrNotFound
}

func (r *MemoryAgentRepo) ActiveGrantFor(_ context.Context, partnerID, userID string) (*agentapi.Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, g := range r.grants {
		if g.PartnerID == partnerID && g.UserID == userID && g.Status == agentapi.GrantActive {
			cp := g
			return &cp, nil
		}
	}
	return nil, agentapi.ErrNotFound
}

func (r *MemoryAgentRepo) ListGrantsByUser(_ context.Context, userID string) ([]*agentapi.Grant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*agentapi.Grant, 0)
	for _, g := range r.grants {
		if g.UserID == userID {
			cp := g
			out = append(out, &cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *MemoryAgentRepo) UpdateGrant(_ context.Context, g *agentapi.Grant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.grants[g.ID]; !ok {
		return agentapi.ErrNotFound
	}
	r.grants[g.ID] = *g
	return nil
}

func (r *MemoryAgentRepo) TouchGrant(_ context.Context, id string, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	g, ok := r.grants[id]
	if !ok {
		return agentapi.ErrNotFound
	}
	g.LastUsedAt = &at
	r.grants[id] = g
	return nil
}

// ── Qoralamalar ──

func (r *MemoryAgentRepo) CreateDraft(_ context.Context, d *agentapi.Draft) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drafts[d.ID] = *d
	return nil
}

func (r *MemoryAgentRepo) DraftByID(_ context.Context, id string) (*agentapi.Draft, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.drafts[id]
	if !ok {
		return nil, agentapi.ErrNotFound
	}
	return &d, nil
}

func (r *MemoryAgentRepo) UpdateDraft(_ context.Context, d *agentapi.Draft) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.drafts[d.ID]; !ok {
		return agentapi.ErrNotFound
	}
	r.drafts[d.ID] = *d
	return nil
}

func (r *MemoryAgentRepo) ListOpenDraftsByUser(_ context.Context, userID string) ([]*agentapi.Draft, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	now := time.Now()
	out := make([]*agentapi.Draft, 0)
	for _, d := range r.drafts {
		if d.UserID != userID || d.Status != agentapi.DraftAwaitingUser {
			continue
		}
		// Muddati o'tganini ko'rsatishdan ma'no yo'q — tugma
		// bosilsa baribir rad etilardi.
		if !now.Before(d.ExpiresAt) {
			continue
		}
		cp := d
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *MemoryAgentRepo) SpentSince(_ context.Context, grantID string, since time.Time) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var sum int64
	for _, d := range r.drafts {
		if d.GrantID == grantID && d.Status == agentapi.DraftPlaced && !d.CreatedAt.Before(since) {
			sum += d.TotalTiyin
		}
	}
	return sum, nil
}

// ── Audit ──

func (r *MemoryAgentRepo) AppendAudit(_ context.Context, e *agentapi.AuditEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.auditSeq++
	e.ID = r.auditSeq
	r.audit = append(r.audit, *e)
	return nil
}

func (r *MemoryAgentRepo) ListAuditByUser(_ context.Context, userID string, limit int) ([]*agentapi.AuditEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*agentapi.AuditEntry, 0, limit)
	// Oxiridan boshlab — eng yangilari birinchi.
	for i := len(r.audit) - 1; i >= 0 && len(out) < limit; i-- {
		if r.audit[i].UserID == userID {
			cp := r.audit[i]
			out = append(out, &cp)
		}
	}
	return out, nil
}
