-- 0065: rollar va a'zolik (merchant realm) — AUTH-PHASE-A.md 3, 6-bo'limlar.
--
-- Ruxsatlar katalogi KODDA (`internal/identity/authz`), bazada emas: bazadagi rol
-- faqat katalogdagi satrlarni saqlaydi; noma'lum ruxsat kod tomonida rad etiladi.
-- Bu yerda faqat FORMAT tekshiriladi (`resource.action` ko'rinishi).
--
-- Xodim akkaunti = ALOHIDA merchant akkaunti (egasi qarori: xodim faqat
-- Telegram/SMS kodi bilan kiradi). Mijoz akkauntining roli endi almashtirilmaydi
-- (A3 dan boshlab).

CREATE TABLE IF NOT EXISTS roles (
    id             TEXT PRIMARY KEY,
    org_id         TEXT REFERENCES organizations(id),  -- NULL = tizim shabloni
    business_type  TEXT NOT NULL CHECK (business_type IN ('restaurant', 'store', 'club', 'taxi', 'hotel')),
    key            TEXT NOT NULL CHECK (key ~ '^[a-z][a-z0-9_]{1,40}$'),
    name           TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 80),
    permissions    TEXT[] NOT NULL DEFAULT '{}',
    created_by     TEXT REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- shablon kaliti tur ichida unikal; tashkilot rollari tashkilot+tur ichida
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_template_key
    ON roles (business_type, key) WHERE org_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_org_key
    ON roles (org_id, business_type, key) WHERE org_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS memberships (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users(id),
    org_id           TEXT NOT NULL REFERENCES organizations(id),
    business_id      TEXT REFERENCES businesses(id),  -- NULL = egasi: tashkilotning HAMMA biznesi
    role_id          TEXT REFERENCES roles(id),       -- NULL faqat egasi uchun
    is_owner         BOOLEAN NOT NULL DEFAULT FALSE,
    apps             TEXT[] NOT NULL DEFAULT '{}',    -- panel | pro | go
    status           TEXT NOT NULL DEFAULT 'invited'
                     CHECK (status IN ('invited', 'active', 'suspended', 'dismissed')),
    staff_member_id  TEXT NOT NULL DEFAULT '',        -- mavjud `staff` yozuviga ko'prik
    created_by       TEXT REFERENCES users(id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- egasi: biznessiz va rolsiz; xodim: biznes va rol MAJBURIY
    CHECK ((is_owner AND business_id IS NULL AND role_id IS NULL)
        OR (NOT is_owner AND business_id IS NOT NULL AND role_id IS NOT NULL)),
    CHECK (apps <@ ARRAY['panel', 'pro', 'go']::TEXT[])
);
-- bir akkaunt bir biznesda bitta a'zolik; tashkilotda bitta egalik
CREATE UNIQUE INDEX IF NOT EXISTS idx_memberships_user_business
    ON memberships (user_id, business_id) WHERE business_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_memberships_owner
    ON memberships (user_id, org_id) WHERE is_owner;
CREATE INDEX IF NOT EXISTS idx_memberships_business ON memberships (business_id) WHERE business_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_memberships_org ON memberships (org_id);

-- Bitta tashkilotning faqat bitta egasi.
CREATE UNIQUE INDEX IF NOT EXISTS idx_memberships_one_owner_per_org
    ON memberships (org_id) WHERE is_owner AND status <> 'dismissed';
