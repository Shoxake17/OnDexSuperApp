-- 0079: OnDexFlow — o'z OTA yangilanish tizimi (Plane ONDEXECOSY-472).
--
-- Modul: internal/modules/flow — O'Z SXEMASI (ADR-003). Boshqa modullar
-- bu jadvallarga SQL yozmaydi.
--
-- Fayllarning o'zi (asosiy libapp.so, yamoqlar) R2 da; bu yerda faqat
-- metama'lumot, xesh va imzolar. Imzo shaxsiy kalitsiz yasab bo'lmaydi —
-- baza buzilsa ham qurilma soxta yamoqni rad etadi.
CREATE SCHEMA IF NOT EXISTS flow;

CREATE TABLE IF NOT EXISTS flow.apps (
    id          TEXT PRIMARY KEY,                 -- shorebird.yaml app_id (UUID)
    name        TEXT NOT NULL UNIQUE,
    platform    TEXT NOT NULL CHECK (platform IN ('android')),
    public_key  TEXT NOT NULL,                    -- PKCS#1 DER base64
    key_id      TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by  TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS flow.releases (
    id          TEXT PRIMARY KEY,
    app_id      TEXT NOT NULL REFERENCES flow.apps(id),
    version     TEXT NOT NULL,
    platform    TEXT NOT NULL,
    artifacts   JSONB NOT NULL DEFAULT '[]'::jsonb,
    entries_url TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (app_id, version, platform)
);

CREATE TABLE IF NOT EXISTS flow.patches (
    id           TEXT PRIMARY KEY,
    release_id   TEXT NOT NULL REFERENCES flow.releases(id),
    app_id       TEXT NOT NULL,
    version      TEXT NOT NULL,
    number       INT  NOT NULL CHECK (number >= 1),
    channel      TEXT NOT NULL DEFAULT 'stable',
    status       TEXT NOT NULL CHECK (status IN ('draft','active','paused','rolled_back')),
    rollout      INT  NOT NULL DEFAULT 0 CHECK (rollout BETWEEN 0 AND 100),
    note         TEXT NOT NULL DEFAULT '',
    artifacts    JSONB NOT NULL DEFAULT '[]'::jsonb,
    downloads    BIGINT NOT NULL DEFAULT 0,
    installs     BIGINT NOT NULL DEFAULT 0,
    failures     BIGINT NOT NULL DEFAULT 0,
    rev          INT  NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at TIMESTAMPTZ,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by   TEXT NOT NULL DEFAULT '',
    UNIQUE (release_id, number)
);

CREATE INDEX IF NOT EXISTS patches_release_idx ON flow.patches (release_id, number DESC);
