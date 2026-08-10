-- orders.courier_id'da HECH QANDAY indeks yo'q edi (faqat customer_id va
-- status'da bor, 0001_init.sql'ga qarang). Yangi GetActiveByCourier()
-- so'rovi (kuryer GPS yuborganda — HAR 25 SONIYADA, har ONLAYN kuryer
-- uchun — internal/couriers GPS oqimiga qarang) shu ustunga qarab
-- filtrlaydi. Indekssiz bu HAR safar BUTUN orders jadvalini to'liq skan
-- qiladi (sequential scan) — jadval kattalashgan sari sekinlashib boradi.
-- Xuddi shu sabab bilan restaurant_id ham indekssiz edi (HasActiveByRestaurant/
-- ListByRestaurant) — u ham shu yerda birga tuzatildi.
CREATE INDEX IF NOT EXISTS idx_orders_courier ON orders (courier_id);
CREATE INDEX IF NOT EXISTS idx_orders_restaurant ON orders (restaurant_id);
