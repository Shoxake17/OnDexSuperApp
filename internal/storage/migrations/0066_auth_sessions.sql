-- 0066: sessiyalar va refresh tokenlar — AUTH-PHASE-A.md 4-bo'lim.
--
-- Refresh token OCHIQ holda HECH QACHON saqlanmaydi — faqat sha256 xeshi.
-- Har ishlatilganda yangisi beriladi (rotatsiya). Almashtirilgan (eski) refresh
-- qayta kelsa — shu oiladagi BARCHA sessiyalar bekor qilinadi (o'g'irlik belgisi).
--
-- Muddatlar (egasi tasdiqlagan): panel 7 kun harakatsiz / 30 kun mutlaq;
-- OnDexPro/GO 14 / 60 kun; admin 30 daqiqa / 12 soat. Qiymatlarni KOD qo'yadi,
-- baza faqat ularni saqlaydi va mutlaq muddat idle'dan oldin kelmasligini
-- tekshiradi.

CREATE TABLE IF NOT EXISTS auth_sessions (
    id               TEXT PRIMARY KEY,
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    realm            TEXT NOT NULL CHECK (realm IN ('customer', 'merchant', 'platform')),
    client_id        TEXT NOT NULL CHECK (client_id ~ '^[a-z][a-z0-9-]{1,40}$'),
    family_id        TEXT NOT NULL,
    refresh_hash     BYTEA NOT NULL UNIQUE CHECK (length(refresh_hash) = 32),
    -- oldingi refresh xeshi: qayta ishlatishni aniqlash uchun (faqat bitta qadam)
    prev_refresh_hash BYTEA UNIQUE CHECK (prev_refresh_hash IS NULL OR length(prev_refresh_hash) = 32),
    amr              TEXT[] NOT NULL DEFAULT '{}',   -- pwd | otp | totp
    mfa_at           TIMESTAMPTZ,                    -- oxirgi TOTP (step-up uchun)
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    idle_expires_at  TIMESTAMPTZ NOT NULL,
    abs_expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at       TIMESTAMPTZ,
    revoke_reason    TEXT NOT NULL DEFAULT '',
    ip_hash          BYTEA,                          -- HMAC(pepper, ip): ochiq IP saqlanmaydi
    user_agent       TEXT NOT NULL DEFAULT '' CHECK (length(user_agent) <= 300),
    device_name      TEXT NOT NULL DEFAULT '' CHECK (length(device_name) <= 100),
    CHECK (idle_expires_at <= abs_expires_at),
    CHECK (abs_expires_at > created_at)
);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_user ON auth_sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_auth_sessions_family ON auth_sessions (family_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry ON auth_sessions (abs_expires_at);
