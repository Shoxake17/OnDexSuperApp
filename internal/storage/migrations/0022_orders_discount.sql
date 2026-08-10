-- Aksiya (promotions) checkout'ga ulandi: buyurtma yaratilishida qaysi
-- aksiya qo'llanilgani va qancha chegirma berilgani suratga olinadi
-- (promotion o'chirilsa/o'zgarsa ham eski buyurtma tarixi o'zgarmaydi).
ALTER TABLE orders ADD COLUMN IF NOT EXISTS subtotal_tiyin BIGINT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS discount_tiyin BIGINT NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS promotion_id TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS promotion_name TEXT NOT NULL DEFAULT '';
