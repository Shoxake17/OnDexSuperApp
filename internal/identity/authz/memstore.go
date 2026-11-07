package authz

import (
	"context"
	"regexp"
	"slices"
	"sort"
	"sync"
	"time"
)

// MemStore — xotira ombori (dev rejim, testlar). Postgres bilan BIR XIL
// shartnomaga bo'ysunadi (`authztest.RunStoreContract`), jumladan bazadagi
// CHECK va UNIQUE cheklovlariga.
type MemStore struct {
	mu      sync.RWMutex
	orgs    map[string]*Organization
	biz     map[string]*Business
	roles   map[string]*Role
	members map[string]*Membership
}

func NewMemStore() *MemStore {
	return &MemStore{orgs: map[string]*Organization{}, biz: map[string]*Business{},
		roles: map[string]*Role{}, members: map[string]*Membership{}}
}

var (
	tinRe     = regexp.MustCompile(`^[0-9]{9}$`)
	roleKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]{1,40}$`)
)

func cloneRole(r *Role) *Role {
	c := *r
	c.Permissions = slices.Clone(r.Permissions)
	return &c
}

func cloneMember(m *Membership) *Membership {
	c := *m
	c.Apps = slices.Clone(m.Apps)
	return &c
}

func (s *MemStore) CreateOrganization(_ context.Context, o *Organization) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o.ID == "" || o.Name == "" || len([]rune(o.Name)) > 200 || o.OwnerUserID == "" ||
		(o.TIN != "" && !tinRe.MatchString(o.TIN)) || !slices.Contains([]string{"", "llc", "sole", "other"}, o.LegalForm) {
		return ErrInvalid
	}
	if o.Status == "" {
		o.Status = OrgPending
	}
	if !slices.Contains([]string{OrgPending, OrgActive, OrgSuspended, OrgRejected}, o.Status) {
		return ErrInvalid
	}
	if _, ok := s.orgs[o.ID]; ok {
		return ErrConflict
	}
	for _, x := range s.orgs {
		if o.TIN != "" && x.TIN == o.TIN {
			return ErrConflict
		}
	}
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}
	c := *o
	s.orgs[o.ID] = &c
	return nil
}

func (s *MemStore) GetOrganization(_ context.Context, id string) (*Organization, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.orgs[id]
	if !ok {
		return nil, ErrNotFound
	}
	c := *o
	return &c, nil
}

// SetOrganizationStatus — faqat xotira rejimi/testlar (platform tasdiqlashi A6 da).
func (s *MemStore) SetOrganizationStatus(id, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o, ok := s.orgs[id]; ok {
		o.Status = status
	}
}

func (s *MemStore) CreateBusiness(_ context.Context, b *Business) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.ID == "" || b.Name == "" || len([]rune(b.Name)) > 200 || !b.Type.Valid() {
		return ErrInvalid
	}
	if b.Status == "" {
		b.Status = BusinessActive
	}
	if _, ok := s.orgs[b.OrgID]; !ok {
		return ErrInvalid
	}
	if _, ok := s.biz[b.ID]; ok {
		return ErrConflict
	}
	for _, x := range s.biz {
		if b.LegacyEntityID != "" && x.Type == b.Type && x.LegacyEntityID == b.LegacyEntityID {
			return ErrConflict
		}
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	c := *b
	s.biz[b.ID] = &c
	return nil
}

func (s *MemStore) GetBusiness(_ context.Context, id string) (*Business, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.biz[id]
	if !ok {
		return nil, ErrNotFound
	}
	c := *b
	return &c, nil
}

// SetBusinessStatus — faqat xotira rejimi/testlar.
func (s *MemStore) SetBusinessStatus(id, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b, ok := s.biz[id]; ok {
		b.Status = status
	}
}

func validRole(r *Role) bool {
	return r.ID != "" && r.BusinessType.Valid() && roleKeyRe.MatchString(r.Key) &&
		r.Name != "" && len([]rune(r.Name)) <= 80
}

func (s *MemStore) UpsertTemplates(_ context.Context, roles []*Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range roles {
		if !validRole(r) || r.OrgID != "" {
			return ErrInvalid
		}
		for id, x := range s.roles {
			if x.OrgID == "" && x.BusinessType == r.BusinessType && x.Key == r.Key && id != r.ID {
				return ErrConflict
			}
		}
		s.roles[r.ID] = cloneRole(r)
	}
	return nil
}

func (s *MemStore) CreateRole(_ context.Context, r *Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validRole(r) || r.OrgID == "" {
		return ErrInvalid
	}
	if _, ok := s.orgs[r.OrgID]; !ok {
		return ErrInvalid
	}
	if _, ok := s.roles[r.ID]; ok {
		return ErrConflict
	}
	for _, x := range s.roles {
		if x.OrgID == r.OrgID && x.BusinessType == r.BusinessType && x.Key == r.Key {
			return ErrConflict
		}
	}
	s.roles[r.ID] = cloneRole(r)
	return nil
}

func (s *MemStore) GetRole(_ context.Context, id string) (*Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.roles[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneRole(r), nil
}

func (s *MemStore) Roles(_ context.Context, orgID string, t BusinessType) ([]*Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Role
	for _, r := range s.roles {
		if r.BusinessType == t && (r.OrgID == "" || (orgID != "" && r.OrgID == orgID)) {
			out = append(out, cloneRole(r))
		}
	}
	sortRoles(out)
	return out, nil
}

func sortRoles(rs []*Role) {
	sort.Slice(rs, func(i, j int) bool {
		if (rs[i].OrgID == "") != (rs[j].OrgID == "") {
			return rs[i].OrgID == "" // shablonlar oldin
		}
		return rs[i].Key < rs[j].Key
	})
}

func validApps(apps []string) bool {
	for _, a := range apps {
		if !ValidApp(a) {
			return false
		}
	}
	return true
}

func validMemberStatus(st string) bool {
	return slices.Contains([]string{MemberInvited, MemberActive, MemberSuspended, MemberDismissed}, st)
}

func (s *MemStore) CreateMembership(_ context.Context, m *Membership) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m.Status == "" {
		m.Status = MemberInvited
	}
	if m.ID == "" || m.UserID == "" || !validApps(m.Apps) || !validMemberStatus(m.Status) {
		return ErrInvalid
	}
	if _, ok := s.orgs[m.OrgID]; !ok {
		return ErrInvalid
	}
	if m.IsOwner != (m.BusinessID == "" && m.RoleID == "") || (!m.IsOwner && (m.BusinessID == "" || m.RoleID == "")) {
		return ErrInvalid
	}
	if !m.IsOwner {
		b, ok := s.biz[m.BusinessID]
		if !ok || b.OrgID != m.OrgID {
			return ErrInvalid
		}
		r, ok := s.roles[m.RoleID]
		if !ok || r.BusinessType != b.Type || (r.OrgID != "" && r.OrgID != m.OrgID) {
			return ErrInvalid
		}
	}
	if _, ok := s.members[m.ID]; ok {
		return ErrConflict
	}
	for _, x := range s.members {
		switch {
		case !m.IsOwner && x.UserID == m.UserID && x.BusinessID == m.BusinessID:
			return ErrConflict
		case m.IsOwner && x.IsOwner && x.UserID == m.UserID && x.OrgID == m.OrgID:
			return ErrConflict
		case m.IsOwner && x.IsOwner && x.OrgID == m.OrgID && x.Status != MemberDismissed && m.Status != MemberDismissed:
			return ErrConflict
		}
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now()
	}
	if m.Status == MemberInvited && m.InvitedAt.IsZero() {
		m.InvitedAt = m.CreatedAt
	}
	s.members[m.ID] = cloneMember(m)
	return nil
}

func (s *MemStore) GetMembership(_ context.Context, id string) (*Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.members[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneMember(m), nil
}

func (s *MemStore) Members(_ context.Context, businessID string) ([]*Membership, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Membership
	for _, m := range s.members {
		if !m.IsOwner && m.BusinessID == businessID {
			out = append(out, cloneMember(m))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt) || (out[i].CreatedAt.Equal(out[j].CreatedAt) && out[i].ID < out[j].ID)
	})
	return out, nil
}

func (s *MemStore) UpdateMembership(_ context.Context, id string, p MembershipPatch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.members[id]
	if !ok {
		return ErrNotFound
	}
	if (p.Apps != nil && !validApps(p.Apps)) || (p.Status != "" && !validMemberStatus(p.Status)) {
		return ErrInvalid
	}
	if p.RoleID != "" {
		if m.IsOwner {
			return ErrInvalid
		}
		r, ok := s.roles[p.RoleID]
		b := s.biz[m.BusinessID]
		if !ok || b == nil || r.BusinessType != b.Type || (r.OrgID != "" && r.OrgID != m.OrgID) {
			return ErrInvalid
		}
		m.RoleID = p.RoleID
	}
	if p.Apps != nil {
		m.Apps = slices.Clone(p.Apps)
	}
	if p.Status != "" {
		m.Status = p.Status
	}
	if p.InvitedAt != nil {
		m.InvitedAt = *p.InvitedAt
	}
	return nil
}

func (s *MemStore) ActivateInvites(_ context.Context, userID string, since time.Time) ([]*Membership, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Membership
	for _, m := range s.members {
		if m.UserID == userID && m.Status == MemberInvited && m.InvitedAt.After(since) {
			m.Status = MemberActive
			out = append(out, cloneMember(m))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *MemStore) grantLocked(m *Membership, b *Business) *Grant {
	g := &Grant{Business: *b, MembershipID: m.ID, IsOwner: m.IsOwner, Apps: slices.Clone(m.Apps)}
	if o := s.orgs[b.OrgID]; o != nil {
		g.OrgStatus = o.Status
	}
	if r := s.roles[m.RoleID]; r != nil {
		g.Role = cloneRole(r)
	}
	return g
}

func (s *MemStore) Grant(_ context.Context, userID, businessID string) (*Grant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, ok := s.biz[businessID]
	if !ok {
		return nil, ErrNotFound
	}
	var staff *Membership
	for _, m := range s.members {
		if m.UserID != userID || m.Status != MemberActive {
			continue
		}
		if m.IsOwner && m.OrgID == b.OrgID {
			return s.grantLocked(m, b), nil
		}
		if !m.IsOwner && m.BusinessID == businessID {
			staff = m
		}
	}
	if staff == nil {
		return nil, ErrNotFound
	}
	return s.grantLocked(staff, b), nil
}

func (s *MemStore) Grants(_ context.Context, userID string) ([]*Grant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byBiz := map[string]*Grant{}
	for _, m := range s.members {
		if m.UserID != userID || m.Status != MemberActive {
			continue
		}
		for _, b := range s.biz {
			if (m.IsOwner && b.OrgID == m.OrgID) || (!m.IsOwner && b.ID == m.BusinessID) {
				if prev := byBiz[b.ID]; prev == nil || (m.IsOwner && !prev.IsOwner) {
					byBiz[b.ID] = s.grantLocked(m, b)
				}
			}
		}
	}
	out := make([]*Grant, 0, len(byBiz))
	for _, g := range byBiz {
		out = append(out, g)
	}
	sortGrants(out)
	return out, nil
}

func sortGrants(gs []*Grant) {
	sort.Slice(gs, func(i, j int) bool {
		if !gs[i].Business.CreatedAt.Equal(gs[j].Business.CreatedAt) {
			return gs[i].Business.CreatedAt.Before(gs[j].Business.CreatedAt)
		}
		return gs[i].Business.ID < gs[j].Business.ID
	})
}
