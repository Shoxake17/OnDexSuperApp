package flow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore — Postgres ombori (`flow` sxemasi, migratsiya 0079).
type PGStore struct{ pool *pgxpool.Pool }

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func mapErr(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return ErrConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (p *PGStore) CreateApp(ctx context.Context, a *App) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO flow.apps (id, name, platform, public_key, key_id, created_at, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, a.ID, a.Name, a.Platform, a.PublicKey, a.KeyID, a.CreatedAt, a.CreatedBy)
	return mapErr(err)
}

const appCols = `id, name, platform, public_key, key_id, created_at, created_by`

func scanApp(row pgx.Row) (*App, error) {
	var a App
	if err := row.Scan(&a.ID, &a.Name, &a.Platform, &a.PublicKey, &a.KeyID, &a.CreatedAt, &a.CreatedBy); err != nil {
		return nil, mapErr(err)
	}
	return &a, nil
}

func (p *PGStore) GetApp(ctx context.Context, id string) (*App, error) {
	return scanApp(p.pool.QueryRow(ctx, `SELECT `+appCols+` FROM flow.apps WHERE id = $1`, id))
}

func (p *PGStore) ListApps(ctx context.Context) ([]*App, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+appCols+` FROM flow.apps ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*App{}
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (p *PGStore) CreateRelease(ctx context.Context, r *Release) error {
	arts, err := json.Marshal(r.Artifacts)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `INSERT INTO flow.releases (id, app_id, version, platform, artifacts, entries_url, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`, r.ID, r.AppID, r.Version, r.Platform, arts, r.EntriesURL, r.CreatedAt)
	return mapErr(err)
}

const relCols = `id, app_id, version, platform, artifacts, entries_url, created_at`

func scanRelease(row pgx.Row) (*Release, error) {
	var r Release
	var arts []byte
	if err := row.Scan(&r.ID, &r.AppID, &r.Version, &r.Platform, &arts, &r.EntriesURL, &r.CreatedAt); err != nil {
		return nil, mapErr(err)
	}
	if err := json.Unmarshal(arts, &r.Artifacts); err != nil {
		return nil, fmt.Errorf("flow: reliz artefaktlari: %w", err)
	}
	return &r, nil
}

func (p *PGStore) GetRelease(ctx context.Context, id string) (*Release, error) {
	return scanRelease(p.pool.QueryRow(ctx, `SELECT `+relCols+` FROM flow.releases WHERE id = $1`, id))
}

func (p *PGStore) FindRelease(ctx context.Context, appID, version, platform string) (*Release, error) {
	return scanRelease(p.pool.QueryRow(ctx, `SELECT `+relCols+` FROM flow.releases
		WHERE app_id = $1 AND version = $2 AND platform = $3`, appID, version, platform))
}

func (p *PGStore) ListReleases(ctx context.Context, appID string) ([]*Release, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+relCols+` FROM flow.releases WHERE app_id = $1
		ORDER BY created_at DESC LIMIT 200`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Release{}
	for rows.Next() {
		r, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *PGStore) CreatePatch(ctx context.Context, x *Patch) error {
	arts, err := json.Marshal(x.Artifacts)
	if err != nil {
		return err
	}
	_, err = p.pool.Exec(ctx, `INSERT INTO flow.patches (id, release_id, app_id, version, number, channel, status,
		rollout, note, artifacts, rev, created_at, updated_at, updated_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,1,$11,$12,$13)`,
		x.ID, x.ReleaseID, x.AppID, x.Version, x.Number, x.Channel, string(x.Status), x.Rollout, x.Note,
		arts, x.CreatedAt, x.UpdatedAt, x.UpdatedBy)
	if err == nil {
		x.Rev = 1
	}
	return mapErr(err)
}

const patchCols = `id, release_id, app_id, version, number, channel, status, rollout, note, artifacts,
	downloads, installs, failures, rev, created_at, activated_at, updated_at, updated_by`

func scanPatch(row pgx.Row) (*Patch, error) {
	var x Patch
	var arts []byte
	var st string
	if err := row.Scan(&x.ID, &x.ReleaseID, &x.AppID, &x.Version, &x.Number, &x.Channel, &st, &x.Rollout,
		&x.Note, &arts, &x.Stats.Downloads, &x.Stats.Installs, &x.Stats.Failures, &x.Rev, &x.CreatedAt,
		&x.ActivatedAt, &x.UpdatedAt, &x.UpdatedBy); err != nil {
		return nil, mapErr(err)
	}
	x.Status = PatchStatus(st)
	if err := json.Unmarshal(arts, &x.Artifacts); err != nil {
		return nil, fmt.Errorf("flow: yamoq artefaktlari: %w", err)
	}
	return &x, nil
}

func (p *PGStore) GetPatch(ctx context.Context, id string) (*Patch, error) {
	return scanPatch(p.pool.QueryRow(ctx, `SELECT `+patchCols+` FROM flow.patches WHERE id = $1`, id))
}

func (p *PGStore) ListPatches(ctx context.Context, releaseID string) ([]*Patch, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+patchCols+` FROM flow.patches WHERE release_id = $1
		ORDER BY number DESC LIMIT 500`, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Patch{}
	for rows.Next() {
		x, err := scanPatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (p *PGStore) UpdatePatch(ctx context.Context, x *Patch) error {
	tag, err := p.pool.Exec(ctx, `UPDATE flow.patches SET status = $2, rollout = $3, activated_at = $4,
		updated_at = $5, updated_by = $6, rev = rev + 1 WHERE id = $1 AND rev = $7`,
		x.ID, string(x.Status), x.Rollout, x.ActivatedAt, x.UpdatedAt, x.UpdatedBy, x.Rev)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		if _, err := p.GetPatch(ctx, x.ID); err != nil {
			return err
		}
		return ErrVersionLock
	}
	x.Rev++
	return nil
}

func (p *PGStore) AddStat(ctx context.Context, releaseID string, number int, kind string) error {
	var col string
	switch kind {
	case "downloads", "installs", "failures":
		col = kind // oq ro'yxat — SQL'ga faqat shu uchtasi tushadi
	default:
		return ErrInvalid
	}
	tag, err := p.pool.Exec(ctx, `UPDATE flow.patches SET `+col+` = `+col+` + 1
		WHERE release_id = $1 AND number = $2`, releaseID, number)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
