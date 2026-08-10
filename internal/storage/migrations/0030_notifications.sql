-- Bildirishnomalar va push tokenlari.
--
-- ── NEGA SAQLANADI ─────────────────────────────────────────────────
-- Avval bildirishnoma FAQAT WebSocket orqali ketardi va soket o'lik
-- bo'lsa IZSIZ yo'qolardi: kuryer taklifni ko'rmasdi, mijoz buyurtma
-- holati o'zgarganini bilmasdi va buni aniqlashning yo'li yo'q edi.
--
-- Endi avval yoziladi, keyin yuboriladi.

CREATE TABLE IF NOT EXISTS notifications (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- module/kind — ATAYLAB oddiy satr, enum EMAS.
    -- Yangi modul (do'kon, taxi, klub) qo'shilganda MIGRATSIYA
    -- KERAK BO'LMASLIGI uchun.
    module     TEXT NOT NULL,
    kind       TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    body       TEXT NOT NULL DEFAULT '',
    -- data — ilovada kerakli ekranga o'tish uchun (deep link).
    data       JSONB NOT NULL DEFAULT '{}'::jsonb,
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Asosiy so'rov: "shu foydalanuvchining eng yangi N tasi".
CREATE INDEX IF NOT EXISTS idx_notifications_user_created
    ON notifications (user_id, created_at DESC);

-- O'qilmaganlar sanog'i (qo'ng'iroq belgisidagi raqam) — QISMAN
-- indeks: o'qilganlar indeksda umuman saqlanmaydi, ya'ni u kichik
-- bo'lib qoladi.
CREATE INDEX IF NOT EXISTS idx_notifications_unread
    ON notifications (user_id) WHERE read_at IS NULL;

-- ── PUSH TOKENLARI ─────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS device_tokens (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform   TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- NEGA token PRIMARY KEY: bitta qurilmada ikki hisob almashsa, token
-- YANGI egasiga o'tishi kerak — aks holda chiqib ketgan foydalanuvchi
-- push olishda davom etardi (o'zga odamning buyurtmasi haqida).
CREATE INDEX IF NOT EXISTS idx_device_tokens_user
    ON device_tokens (user_id);
