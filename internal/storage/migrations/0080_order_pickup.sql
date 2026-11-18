-- 0080: Olib ketish buyurtmasi (Borish, Plane 473/474).
--
-- order_type = 'pickup': kuryer, manzil, yetkazish haqi yo'q; mijoz oldindan
-- to'laydi va restoranga o'zi keladi. Olish kodi SAQLANMAYDI (server sirdan
-- HMAC bilan hisoblanadi - httpapi/pickup_code.go).
--
-- Yangi terminal holat 'not_picked_up' ustunda CHECK yo'qligi uchun alohida
-- o'zgarish talab qilmaydi (holatlar ro'yxati orders.terminalStatuses da).
ALTER TABLE orders ADD COLUMN IF NOT EXISTS pickup_at             TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS pickup_reminded_at    TIMESTAMPTZ;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS pickup_discount_tiyin BIGINT NOT NULL DEFAULT 0;

-- Sweep: tayyor, olinmagan olib ketish buyurtmalari (eslatma / 2 soatlik yakun).
-- Haqiqiy tayyor bo'lgan vaqt history'dan olinadi (ready_at - TAXMINIY vaqt).
CREATE INDEX IF NOT EXISTS orders_pickup_ready_idx ON orders (created_at)
    WHERE order_type = 'pickup' AND status = 'ready';

-- Yangi terminal holat ('not_picked_up') qo'shildi: kuryerning faol
-- buyurtmalari indeksi (0054) predikati storage.activeOrderStatusFilter bilan
-- HARFMA-HARF mos bo'lishi shart, aks holda rejalashtiruvchi uni ishlatmaydi.
-- 0054 qayta ishlamaydi (migratsiyalar nomi bo'yicha) - shu yerda qayta quriladi.
-- Test: TestActiveCourierIndexMatchesQuery.
DROP INDEX IF EXISTS idx_orders_courier_active;
CREATE INDEX IF NOT EXISTS idx_orders_courier_active
    ON orders (courier_id, created_at DESC)
    WHERE status NOT IN ('delivered', 'served', 'rejected', 'cancelled', 'not_picked_up');
