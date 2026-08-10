-- Aksiyalar modeli kengaytirildi (image/aksiyaqosh.png namunasiga mos):
-- chegirma endi umumiy qiymat+birlik, minimal/maksimal cheklovlar,
-- to'liq sana+vaqt oralig'i, cheksiz muddat va qo'lda Faol/To'xtatilgan
-- bayrog'i, qo'llanilish doirasi (mahsulot/buyurtma/turkum) qo'shildi.
ALTER TABLE promotions DROP COLUMN IF EXISTS discount_percent;
ALTER TABLE promotions DROP COLUMN IF EXISTS discount_amount_tiyin;
ALTER TABLE promotions DROP COLUMN IF EXISTS start_date;
ALTER TABLE promotions DROP COLUMN IF EXISTS end_date;
ALTER TABLE promotions DROP COLUMN IF EXISTS time_start;
ALTER TABLE promotions DROP COLUMN IF EXISTS time_end;

ALTER TABLE promotions ADD COLUMN IF NOT EXISTS discount_unit TEXT NOT NULL DEFAULT 'percent';
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS discount_value BIGINT NOT NULL DEFAULT 0;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS min_order_amount_tiyin BIGINT NOT NULL DEFAULT 0;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS max_discount_amount_tiyin BIGINT NOT NULL DEFAULT 0;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS start_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS end_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS indefinite BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS applies_to_products BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS applies_to_orders BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS applies_to_categories BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS target_product_ids TEXT[] NOT NULL DEFAULT '{}';
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS target_categories TEXT[] NOT NULL DEFAULT '{}';
