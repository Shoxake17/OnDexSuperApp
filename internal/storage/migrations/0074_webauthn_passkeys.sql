-- 0074: passkey (WebAuthn / FIDO2) — fishingga chidamli ikkinchi bosqich (AUD-4).
--
-- Faqat OCHIQ kalit saqlanadi (sir emas): baza nusxasi o'g'irlansa ham u
-- bilan kirib bo'lmaydi. `data` — go-webauthn `Credential` JSON'i (ochiq
-- kalit, imzo hisoblagichi, flaglar). Alohida jadvallar: eski kod
-- e'tiborsiz qoldiradi — rollback xavfsiz.
CREATE TABLE IF NOT EXISTS webauthn_credentials (
    id            TEXT PRIMARY KEY CHECK (length(id) BETWEEN 16 AND 1400), -- credential ID, base64url, GLOBAL noyob
    user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    data          JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_webauthn_user ON webauthn_credentials (user_id);

-- Bir martalik challenge: imzolangan javobni qayta yuborish (replay) rad
-- etiladi. Muddati o'tganlari yozish paytida tozalanadi.
CREATE TABLE IF NOT EXISTS webauthn_used_challenges (
    hash        BYTEA PRIMARY KEY CHECK (length(hash) = 32),
    expires_at  TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_webauthn_used_exp ON webauthn_used_challenges (expires_at);
