-- 0064: tashkilot va bizneslar (merchant realm) — AUTH-PHASE-A.md 3-bo'lim.
--
-- Organization — yuridik shaxs (MCHJ / YaTT), egasi merchant akkaunti.
-- Business — tashkilotdagi bitta biznes (restoran, do'kon, klub, taksi, mehmonxona).
--
-- A1: jadvallar BO'SH yaratiladi; mavjud restoran/do'konlardan to'ldirish
-- alohida buyruq bilan (`cmd/identitymigrate`, avval --dry-run).
--
-- KYC (egasi qarori): STIR + guvohnoma rasmi, tasdiqlashni FAQAT platform (OnDex
-- egasi) qiladi. Tasdiqlanmagan tashkilot bizneslari mijozlarga KO'RINMAYDI.

CREATE TABLE IF NOT EXISTS organizations (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    legal_form     TEXT NOT NULL DEFAULT '' CHECK (legal_form IN ('', 'llc', 'sole', 'other')),
    tin            TEXT NOT NULL DEFAULT '' CHECK (tin = '' OR tin ~ '^[0-9]{9}$'),  -- STIR: 9 raqam
    status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'active', 'suspended', 'rejected')),
    owner_user_id  TEXT NOT NULL REFERENCES users(id),
    kyc_doc_key    TEXT NOT NULL DEFAULT '',  -- guvohnoma rasmi: YOPIQ R2 kaliti (ochiq URL EMAS)
    status_reason  TEXT NOT NULL DEFAULT '',
    verified_at    TIMESTAMPTZ,
    verified_by    TEXT REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- faol tashkilot har doim kim tasdiqlaganini biladi
    CHECK (status <> 'active' OR verified_at IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_organizations_owner ON organizations (owner_user_id);
CREATE INDEX IF NOT EXISTS idx_organizations_status ON organizations (status);
-- bitta STIR bitta tashkilot (bo'sh STIR — hali kiritilmagan, cheklanmaydi)
CREATE UNIQUE INDEX IF NOT EXISTS idx_organizations_tin ON organizations (tin) WHERE tin <> '';

CREATE TABLE IF NOT EXISTS businesses (
    id                TEXT PRIMARY KEY,
    org_id            TEXT NOT NULL REFERENCES organizations(id),
    type              TEXT NOT NULL CHECK (type IN ('restaurant', 'store', 'club', 'taxi', 'hotel')),
    -- hozirgi tizimdagi obyekt ID'si (restoran — Mongo katalogi, do'kon — users.entity_id);
    -- eski marshrutlar ishlashda davom etishi uchun ko'prik
    legacy_entity_id  TEXT NOT NULL DEFAULT '',
    name              TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    status            TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'closed')),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_businesses_org ON businesses (org_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_businesses_legacy
    ON businesses (type, legacy_entity_id) WHERE legacy_entity_id <> '';
