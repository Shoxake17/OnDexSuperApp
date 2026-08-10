-- Sodiqlik (loyalty) aksiyasi uchun haqiqiy shart + HAQIQIY qo'llanilish
-- statistikasi (restoran panelidagi "Foydalanish"/"Savdo" ustunlari
-- endi soxta 0 emas, checkout'da real qo'llanilganda oshadi).
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS min_previous_orders BIGINT NOT NULL DEFAULT 0;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS usage_count BIGINT NOT NULL DEFAULT 0;
ALTER TABLE promotions ADD COLUMN IF NOT EXISTS sales_total_tiyin BIGINT NOT NULL DEFAULT 0;
