-- OnDex qo'llab-quvvatlash (`internal/support`).
--
-- `support_contacts` — BITTA qator (id = 1): platforma aloqa ma'lumotlari,
-- faqat admin o'zgartiradi, barcha panellarda ko'rinadi.
CREATE TABLE IF NOT EXISTS support_contacts (
    id          SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    phone       TEXT NOT NULL DEFAULT '',
    phone_hours TEXT NOT NULL DEFAULT '',
    telegram    TEXT NOT NULL DEFAULT '',
    email       TEXT NOT NULL DEFAULT '',
    email_note  TEXT NOT NULL DEFAULT '',
    updated_by  TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Restoran ↔ OnDex admin chati. `seq` — o'suvchi tartib (uzilishdan keyin
-- "shu raqamdan keyingilari"). `client_id` — qayta urinish ikkinchi xabar
-- yaratmaydi.
CREATE TABLE IF NOT EXISTS support_messages (
    seq           BIGSERIAL PRIMARY KEY,
    id            TEXT NOT NULL UNIQUE,
    restaurant_id TEXT NOT NULL,
    sender        TEXT NOT NULL CHECK (sender IN ('restaurant', 'admin')),
    sender_id     TEXT NOT NULL,
    sender_name   TEXT NOT NULL DEFAULT '',
    body          TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    client_id     TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_support_messages_client
    ON support_messages (restaurant_id, sender_id, client_id);
CREATE INDEX IF NOT EXISTS idx_support_messages_thread
    ON support_messages (restaurant_id, seq DESC);
CREATE INDEX IF NOT EXISTS idx_support_messages_unread
    ON support_messages (restaurant_id, sender, seq);

-- Suhbat xulosasi: ro'yxat va o'qilmaganlar hisobi har safar butun
-- tarixni aylanib chiqmasin.
CREATE TABLE IF NOT EXISTS support_threads (
    restaurant_id       TEXT PRIMARY KEY,
    message_count       INT NOT NULL DEFAULT 0,
    first_at            TIMESTAMPTZ NOT NULL,
    last_at             TIMESTAMPTZ NOT NULL,
    last_seq            BIGINT NOT NULL DEFAULT 0,
    restaurant_read_seq BIGINT NOT NULL DEFAULT 0,
    admin_read_seq      BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_support_threads_last ON support_threads (last_seq DESC);
