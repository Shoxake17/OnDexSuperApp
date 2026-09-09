-- Stollarni zallar/zonalar bo'yicha guruhlash.
--
-- Avval bitta restoranda stol nomi (label) umuman unique edi, shuning
-- uchun "Asosiy zal · 5" va "Ayvon · 5" birga yashay olmasdi.
-- Endi unique juftlik: (restaurant_id, zone, label).
--
-- QR tokeniga TEGILMAYDI — chop etilgan kod ertasi kuni ham xuddi
-- shu tokenni o'qiydi.

ALTER TABLE restaurant_tables
    ADD COLUMN IF NOT EXISTS zone TEXT NOT NULL DEFAULT 'Asosiy zal';

DROP INDEX IF EXISTS idx_restaurant_tables_label;

CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurant_tables_zone_label
    ON restaurant_tables (restaurant_id, zone, label);
