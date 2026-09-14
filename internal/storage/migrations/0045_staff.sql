-- "Xodimlar" bo'limi: restoran xodimlari va ularning faoliyat jurnali.
--
-- Xodim — restoranning ichki yozuvi (oshpaz, kassir, tozalovchi...).
-- Ilovasi bor lavozim (ofitsiant) uchun `user_id` — bog'langan
-- "OnDex Affitsiant" akkaunti. Restoranlar Mongo'da bo'lgani uchun
-- `restaurant_id` ga tashqi kalit YO'Q (stollar bilan bir xil).

CREATE TABLE IF NOT EXISTS staff_members (
    id            TEXT PRIMARY KEY,
    restaurant_id TEXT NOT NULL,
    number        INT  NOT NULL CHECK (number > 0),
    first_name    TEXT NOT NULL,
    last_name     TEXT NOT NULL DEFAULT '',
    phone         TEXT NOT NULL,
    position      TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active'
                  CHECK (status IN ('active', 'on_leave', 'dismissed')),
    schedule      JSONB,
    hired_on      DATE,
    note          TEXT NOT NULL DEFAULT '',
    app_access    BOOLEAN NOT NULL DEFAULT FALSE,
    user_id       TEXT REFERENCES users(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    dismissed_at  TIMESTAMPTZ
);

-- #EMP001 — restoran ichida unikal, qayta ishlatilmaydi.
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_members_number
    ON staff_members (restaurant_id, number);

-- Bitta raqam — bitta ishlayotgan xodim (ishdan bo'shaganlar tarixda qoladi).
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_members_active_phone
    ON staff_members (restaurant_id, phone) WHERE status <> 'dismissed';

-- Bitta ilova akkaunti — bitta xodim yozuvi.
CREATE UNIQUE INDEX IF NOT EXISTS idx_staff_members_user
    ON staff_members (user_id) WHERE user_id IS NOT NULL;

-- `seq` — bir amalda bir xil vaqtda yozilgan hodisalar (masalan "ishdan
-- bo'shatildi" + "ilovaga kirish yopildi") tartibi aniq bo'lsin: haftalik
-- farqlar jurnalni eng yangisidan orqaga "o'ynab" tiklanadi.
CREATE TABLE IF NOT EXISTS staff_events (
    seq           BIGSERIAL,
    id            TEXT PRIMARY KEY,
    restaurant_id TEXT NOT NULL,
    member_id     TEXT NOT NULL REFERENCES staff_members(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL,
    from_value    TEXT NOT NULL DEFAULT '',
    to_value      TEXT NOT NULL DEFAULT '',
    actor_id      TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_staff_events_restaurant_time
    ON staff_events (restaurant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_staff_events_member_time
    ON staff_events (member_id, created_at DESC);

-- Mavjud affitsiantlar — xodim yozuviga ko'chiriladi (ilovaga kirish
-- ochiq holida). Avval ular faqat `users` jadvalida edi.
INSERT INTO staff_members
    (id, restaurant_id, number, first_name, phone, position, status,
     app_access, user_id, created_at, updated_at)
SELECT md5('staff:' || u.id),
       u.entity_id,
       row_number() OVER (PARTITION BY u.entity_id ORDER BY u.created_at, u.id),
       COALESCE(NULLIF(btrim(u.name), ''), 'Ofitsiant'),
       u.phone,
       'waiter',
       'active',
       TRUE,
       u.id,
       u.created_at,
       now()
FROM users u
WHERE u.role = 'waiter' AND u.entity_id <> '' AND u.phone <> ''
ON CONFLICT DO NOTHING;
