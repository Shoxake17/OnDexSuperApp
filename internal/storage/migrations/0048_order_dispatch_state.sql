-- Kuryer qidiruvi holati buyurtma yozuvida (`internal/orders/dispatch_state.go`).
--
-- Avval qidiruv holati faqat server xotirasida edi: kuryer topilmasa
-- buyurtma "Kuryer qidirilmoqda" bo'lib abadiy qolardi, server qayta
-- ishga tushsa esa tayyorlanayotgan/tayyor buyurtmalarning qidiruvi
-- umuman tiklanmasdi.
--
--   dispatch_state    — '' (yo'q/kuryer bor), 'searching', 'not_found'
--   dispatch_deadline — searching: "kuryer topilmadi" bo'ladigan payt;
--                       not_found: avtomatik bekor qilinadigan payt.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS dispatch_state TEXT NOT NULL DEFAULT ''
    CHECK (dispatch_state IN ('', 'searching', 'not_found'));
ALTER TABLE orders ADD COLUMN IF NOT EXISTS dispatch_deadline TIMESTAMPTZ;

-- Nazoratchi har 15 soniyada FAQAT kuryer kutayotgan buyurtmalarni
-- o'qiydi. Predikat `PgOrderRepo.ListAwaitingCourier` dagi so'rov bilan
-- AYNAN bir xil — aks holda rejalashtiruvchi qisman indeksni ishlatmaydi.
CREATE INDEX IF NOT EXISTS idx_orders_awaiting_courier ON orders (created_at)
    WHERE courier_id IS NULL
      AND status IN ('accepted', 'preparing', 'ready')
      AND order_type <> 'dine_in';
