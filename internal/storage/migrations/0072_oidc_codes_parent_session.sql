-- 0072: OnDex OIDC provayderi (A4c, Plane ONDEXECOSY-199/201).
--
-- Egasi qarori (2026-11-08): Ory Hydra EMAS — o'z OIDC'miz mavjud markaziy
-- sessiya xizmati (`auth_sessions`) ustida. Sessiya markazi BITTA qoladi.
--
-- 1) parent_id — OnDex kirish sahifasidagi SSO sessiyasi (client `ondex-sso`)
--    va undan ochilgan ilova sessiyalari (masalan `merchant-web`). SSO sessiyasi
--    yopilsa (chiqish, parol almashishi) bolalari ham yopiladi va har biriga
--    back-channel logout yuboriladi.
-- 2) oauth_codes — Authorization Code (RFC 6749 + PKCE RFC 7636). Kod OCHIQ
--    holda saqlanmaydi (faqat sha256), umri 60 s, BIR MARTA ishlatiladi
--    (`used_at`). Qayta kelsa — u bilan ochilgan sessiya bekor qilinadi
--    (`issued_session_id`, RFC 6749 4.1.2).
-- Ikkalasi ham qo'shuvchi: eski kod ularni e'tiborsiz qoldiradi (rollback xavfsiz).

ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS parent_id TEXT
    REFERENCES auth_sessions(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_auth_sessions_parent ON auth_sessions (parent_id) WHERE parent_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS oauth_codes (
    code_hash         BYTEA PRIMARY KEY CHECK (length(code_hash) = 32),
    client_id         TEXT NOT NULL CHECK (client_id ~ '^[a-z][a-z0-9-]{1,40}$'),
    redirect_uri      TEXT NOT NULL CHECK (length(redirect_uri) <= 512),
    code_challenge    TEXT NOT NULL CHECK (code_challenge ~ '^[A-Za-z0-9_-]{43}$'),
    nonce             TEXT NOT NULL DEFAULT '' CHECK (length(nonce) <= 256),
    scope             TEXT NOT NULL DEFAULT '' CHECK (length(scope) <= 256),
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_session_id TEXT NOT NULL REFERENCES auth_sessions(id) ON DELETE CASCADE,
    org_id            TEXT NOT NULL DEFAULT '',
    amr               TEXT[] NOT NULL DEFAULT '{}',
    auth_time         TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at        TIMESTAMPTZ NOT NULL,
    used_at           TIMESTAMPTZ,
    issued_session_id TEXT,
    CHECK (expires_at > created_at)
);
CREATE INDEX IF NOT EXISTS idx_oauth_codes_expiry ON oauth_codes (expires_at);
