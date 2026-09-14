-- Qo'llab-quvvatlash chatidagi rasmlar (`internal/support`).
--
-- Rasm OMMAVIY bucket'ga EMAS, shu jadvalga yoziladi: ekran rasmida
-- mijoz telefoni, buyurtma va to'lov ma'lumotlari bo'lishi mumkin. U faqat
-- suhbat egasiga (o'sha restoran) va adminga, token bilan, alohida
-- endpoint orqali beriladi. Server rasmni QAYTA KODLAYDI (WebP, uzun tomoni
-- 1600 px) — hajm kichik, metama'lumot (EXIF/GPS) yo'q.
CREATE TABLE IF NOT EXISTS support_attachments (
    id            TEXT PRIMARY KEY,
    restaurant_id TEXT NOT NULL,
    content_type  TEXT NOT NULL CHECK (content_type IN ('image/webp')),
    width         INT NOT NULL CHECK (width BETWEEN 1 AND 1600),
    height        INT NOT NULL CHECK (height BETWEEN 1 AND 1600),
    size_bytes    INT NOT NULL CHECK (size_bytes BETWEEN 1 AND 3145728),
    data          BYTEA NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_attachments_restaurant ON support_attachments (restaurant_id);

ALTER TABLE support_messages
    ADD COLUMN IF NOT EXISTS attachment_id TEXT REFERENCES support_attachments (id);

-- Bitta rasm — bitta xabar.
CREATE UNIQUE INDEX IF NOT EXISTS idx_support_messages_attachment
    ON support_messages (attachment_id) WHERE attachment_id IS NOT NULL;

-- Faqat rasmdan iborat xabarda matn bo'sh bo'lishi mumkin.
ALTER TABLE support_messages DROP CONSTRAINT IF EXISTS support_messages_body_check;
ALTER TABLE support_messages ADD CONSTRAINT support_messages_body_check
    CHECK (char_length(body) <= 2000 AND (char_length(body) >= 1 OR attachment_id IS NOT NULL));
