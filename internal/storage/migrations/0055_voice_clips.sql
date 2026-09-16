-- Ovozli yo'l ko'rsatish: serverda bir marta sintez qilingan iboralar
-- (masalan "Siz Book Cafe restoraniga yetib keldingiz"), internal/voice.
-- Kalit — ovoz va matnning sha256 si; birinchi yozuv qoladi.
CREATE TABLE IF NOT EXISTS voice_clips (
    key        TEXT PRIMARY KEY,
    text       TEXT NOT NULL,
    audio      BYTEA NOT NULL CHECK (octet_length(audio) <= 4194304),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
