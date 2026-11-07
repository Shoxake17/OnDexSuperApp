-- 0067: ikki bosqichli tasdiq (TOTP) va audit jurnali — AUTH-PHASE-A.md 3, 9-bo'limlar.
--
-- Egasi qarori: biznes EGASI uchun TOTP MAJBURIY; platform (admin) uchun ham.
-- TOTP maxfiy kaliti `secretstore` bilan (AES-256-GCM, kalit `SECRETS_ENC_KEY`
-- bazadan TASHQARIDA) shifrlanadi — baza nusxasi o'zi kodni bermaydi.

CREATE TABLE IF NOT EXISTS mfa_factors (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type            TEXT NOT NULL CHECK (type IN ('totp')),
    secret_enc      TEXT NOT NULL CHECK (secret_enc <> ''),
    confirmed_at    TIMESTAMPTZ,          -- NULL = ulanmoqda (hali birinchi kod kiritilmagan)
    last_used_step  BIGINT NOT NULL DEFAULT 0,  -- bir kodni ikki marta ishlatishga qarshi
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- bir akkauntda bitta tasdiqlangan TOTP
CREATE UNIQUE INDEX IF NOT EXISTS idx_mfa_one_totp
    ON mfa_factors (user_id) WHERE type = 'totp' AND confirmed_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS mfa_recovery_codes (
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  BYTEA NOT NULL CHECK (length(code_hash) = 32),
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, code_hash)
);

-- Audit jurnali — FAQAT QO'SHILADI. O'zgartirish va o'chirish trigger bilan
-- taqiqlangan (ilova bitta DB foydalanuvchisi bilan ishlagani uchun GRANT
-- yetarli emas). Eski yozuvlarni arxivlash faqat migratsiya orqali, triggerni
-- ataylab o'chirib.
CREATE TABLE IF NOT EXISTS audit_log (
    id             BIGSERIAL PRIMARY KEY,
    at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_user_id  TEXT NOT NULL DEFAULT '',
    realm          TEXT NOT NULL DEFAULT '',
    org_id         TEXT NOT NULL DEFAULT '',
    business_id    TEXT NOT NULL DEFAULT '',
    action         TEXT NOT NULL CHECK (action ~ '^[a-z][a-z0-9_.]{2,60}$'),
    target         TEXT NOT NULL DEFAULT '' CHECK (length(target) <= 200),
    result         TEXT NOT NULL CHECK (result IN ('ok', 'denied', 'failed')),
    ip_hash        BYTEA,
    user_agent     TEXT NOT NULL DEFAULT '' CHECK (length(user_agent) <= 300),
    details        JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit_log (actor_user_id, at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_org ON audit_log (org_id, at DESC) WHERE org_id <> '';
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_log (action, at DESC);

CREATE OR REPLACE FUNCTION ondex_audit_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'audit_log faqat qo''shiladi (% taqiqlangan)', TG_OP;
END
$$;

DROP TRIGGER IF EXISTS trg_audit_append_only ON audit_log;
CREATE TRIGGER trg_audit_append_only
  BEFORE UPDATE OR DELETE ON audit_log
  FOR EACH STATEMENT EXECUTE FUNCTION ondex_audit_append_only();

-- TRUNCATE alohida trigger turini talab qiladi (yuqoridagisi uni USHLAMAYDI).
DROP TRIGGER IF EXISTS trg_audit_no_truncate ON audit_log;
CREATE TRIGGER trg_audit_no_truncate
  BEFORE TRUNCATE ON audit_log
  FOR EACH STATEMENT EXECUTE FUNCTION ondex_audit_append_only();
