-- Buyurtmaga yetkazish manzili tafsilotlari SURATI (podyezd/qavat/
-- kvartira/domofon/izoh). Avval buyurtmada faqat delivery_lat/lng bor
-- edi va mijoz kiritgan kvartira/izoh KUYERGA UMUMAN YETIB BORMASDI —
-- ko'p qavatli uyga yetkazish amalda imkonsiz edi.
--
-- JSONB — maydonlar to'plami kelajakda o'zgarishi mumkin (masalan
-- "eshik oldiga qoldiring" bayrog'i), har biri uchun alohida ustun va
-- migratsiya qilmaslik uchun. Eski buyurtmalar uchun '{}' — ular
-- yaratilganda bu ma'lumot umuman yig'ilmagan.
ALTER TABLE orders
    ADD COLUMN IF NOT EXISTS delivery_address JSONB NOT NULL DEFAULT '{}'::jsonb;
