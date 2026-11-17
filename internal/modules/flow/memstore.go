package flow

import (
	"context"
	"sort"
	"sync"
)

// MemStore — test va bazasiz rejim uchun.
type MemStore struct {
	mu       sync.Mutex
	apps     map[string]*App
	releases map[string]*Release
	patches  map[string]*Patch
}

func NewMemStore() *MemStore {
	return &MemStore{apps: map[string]*App{}, releases: map[string]*Release{}, patches: map[string]*Patch{}}
}

func cpApp(a *App) *App { c := *a; return &c }
func cpRelease(r *Release) *Release {
	c := *r
	c.Artifacts = append([]ReleaseArtifact(nil), r.Artifacts...)
	return &c
}
func cpPatch(p *Patch) *Patch {
	c := *p
	c.Artifacts = append([]PatchArtifact(nil), p.Artifacts...)
	if p.ActivatedAt != nil {
		t := *p.ActivatedAt
		c.ActivatedAt = &t
	}
	return &c
}

func (m *MemStore) CreateApp(_ context.Context, a *App) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.apps[a.ID]; ok {
		return ErrConflict
	}
	for _, x := range m.apps {
		if x.Name == a.Name {
			return ErrConflict
		}
	}
	m.apps[a.ID] = cpApp(a)
	return nil
}

func (m *MemStore) GetApp(_ context.Context, id string) (*App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.apps[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cpApp(a), nil
}

func (m *MemStore) ListApps(context.Context) ([]*App, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*App{}
	for _, a := range m.apps {
		out = append(out, cpApp(a))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (m *MemStore) CreateRelease(_ context.Context, r *Release) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.releases {
		if x.AppID == r.AppID && x.Version == r.Version && x.Platform == r.Platform {
			return ErrConflict
		}
	}
	m.releases[r.ID] = cpRelease(r)
	return nil
}

func (m *MemStore) GetRelease(_ context.Context, id string) (*Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.releases[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cpRelease(r), nil
}

func (m *MemStore) FindRelease(_ context.Context, appID, version, platform string) (*Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.releases {
		if r.AppID == appID && r.Version == version && r.Platform == platform {
			return cpRelease(r), nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemStore) ListReleases(_ context.Context, appID string) ([]*Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*Release{}
	for _, r := range m.releases {
		if r.AppID == appID {
			out = append(out, cpRelease(r))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (m *MemStore) CreatePatch(_ context.Context, p *Patch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.patches {
		if x.ReleaseID == p.ReleaseID && x.Number == p.Number {
			return ErrConflict
		}
	}
	c := cpPatch(p)
	c.Rev = 1
	p.Rev = 1
	m.patches[p.ID] = c
	return nil
}

func (m *MemStore) GetPatch(_ context.Context, id string) (*Patch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.patches[id]
	if !ok {
		return nil, ErrNotFound
	}
	return cpPatch(p), nil
}

func (m *MemStore) ListPatches(_ context.Context, releaseID string) ([]*Patch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []*Patch{}
	for _, p := range m.patches {
		if p.ReleaseID == releaseID {
			out = append(out, cpPatch(p))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number > out[j].Number })
	return out, nil
}

func (m *MemStore) UpdatePatch(_ context.Context, p *Patch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cur, ok := m.patches[p.ID]
	if !ok {
		return ErrNotFound
	}
	if cur.Rev != p.Rev {
		return ErrVersionLock
	}
	c := cpPatch(p)
	c.Stats = cur.Stats
	c.Rev = cur.Rev + 1
	p.Rev = c.Rev
	m.patches[p.ID] = c
	return nil
}

func (m *MemStore) AddStat(_ context.Context, releaseID string, number int, kind string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.patches {
		if p.ReleaseID == releaseID && p.Number == number {
			switch kind {
			case "downloads":
				p.Stats.Downloads++
			case "installs":
				p.Stats.Installs++
			case "failures":
				p.Stats.Failures++
			}
			return nil
		}
	}
	return ErrNotFound
}
