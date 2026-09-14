-- Joy turlari va holati: stol bilan bir qatorda kabina, VIP xona,
-- topchan, bar stoyka, lounge, banket zali (restoran, kafe, oshxona,
-- choyxona uchun bitta model).
--
-- QR tokeniga TEGILMAYDI — chop etilgan kodlar o'zgarishsiz ishlaydi.

-- Tur. Mavjud barcha qatorlar — oddiy stol.
ALTER TABLE restaurant_tables
    ADD COLUMN IF NOT EXISTS kind TEXT NOT NULL DEFAULT 'table';

-- Sig'im (kishi). NULL ATAYLAB: mavjud stollar uchun "4 kishilik" deb
-- taxmin yozilsa, panel yolg'on ma'lumot ko'rsatardi.
ALTER TABLE restaurant_tables
    ADD COLUMN IF NOT EXISTS capacity INT;

-- Xodim "tozalanmoqda" deb belgilagan payt (NULL — belgi yo'q).
ALTER TABLE restaurant_tables
    ADD COLUMN IF NOT EXISTS cleaning_since TIMESTAMPTZ;

-- QR oxirgi marta skanerlangan payt. Kim skanerlagani SAQLANMAYDI.
ALTER TABLE restaurant_tables
    ADD COLUMN IF NOT EXISTS last_scanned_at TIMESTAMPTZ;

-- Ro'yxat `internal/tables/kind.go` bilan bir xil (test bilan qulflangan).
ALTER TABLE restaurant_tables DROP CONSTRAINT IF EXISTS restaurant_tables_kind_check;
ALTER TABLE restaurant_tables ADD CONSTRAINT restaurant_tables_kind_check
    CHECK (kind IN ('table', 'cabin', 'vip_room', 'tapchan', 'bar_counter', 'lounge', 'banquet_hall'));

ALTER TABLE restaurant_tables DROP CONSTRAINT IF EXISTS restaurant_tables_capacity_check;
ALTER TABLE restaurant_tables ADD CONSTRAINT restaurant_tables_capacity_check
    CHECK (capacity IS NULL OR capacity BETWEEN 1 AND 1000);

-- Bir zalda "Stol 1" va "Kabina 1" birga yashay oladi: unikal juftlikka
-- tur ham kiradi.
DROP INDEX IF EXISTS idx_restaurant_tables_zone_label;
CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurant_tables_zone_kind_label
    ON restaurant_tables (restaurant_id, zone, kind, label);

-- Har bir joyning oxirgi buyurtmasi (holat va "so'nggi skanerlashlar"
-- uchun) — butun restoran tarixini saralamasdan, joy bo'yicha.
CREATE INDEX IF NOT EXISTS idx_orders_table_created
    ON orders (table_id, created_at DESC)
    WHERE table_id IS NOT NULL;
