-- Restoran paneli "Bildirishnomalar" markazi (`internal/alerts`).
--
-- `seq` — o'suvchi tartib: jonli kanal uzilsa panel "shu raqamdan
-- keyingilari" ni so'raydi va hech narsa tushib qolmaydi.
-- `dedupe_key` — bitta hodisa (buyurtma, to'lov, eslatma) uchun bitta
-- yozuv: qayta urinish va takroriy callback ikkinchisini yaratmaydi.
CREATE TABLE IF NOT EXISTS restaurant_notifications (
    seq           BIGSERIAL PRIMARY KEY,
    id            TEXT NOT NULL UNIQUE,
    restaurant_id TEXT NOT NULL,
    kind          TEXT NOT NULL,
    category      TEXT NOT NULL CHECK (category IN
                  ('new', 'success', 'info', 'important', 'report', 'update', 'activity', 'reminder')),
    title         TEXT NOT NULL,
    body          TEXT NOT NULL DEFAULT '',
    data          JSONB NOT NULL DEFAULT '{}',
    dedupe_key    TEXT NOT NULL,
    read_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurant_notifications_dedupe
    ON restaurant_notifications (restaurant_id, dedupe_key);
CREATE INDEX IF NOT EXISTS idx_restaurant_notifications_seq
    ON restaurant_notifications (restaurant_id, seq DESC);
CREATE INDEX IF NOT EXISTS idx_restaurant_notifications_unread
    ON restaurant_notifications (restaurant_id) WHERE read_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_restaurant_notifications_created
    ON restaurant_notifications (created_at);
