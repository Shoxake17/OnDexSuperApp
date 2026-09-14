-- Affitsiant o'zi kiritgan stol buyurtmasi (`POST /waiter/orders`).
--
-- Bunday buyurtmada mijoz akkaunti YO'Q (mehmon stolda og'zaki buyurtma
-- beradi), shuning uchun `customer_id` bo'sh qoladi va buyurtmani kim
-- kiritgani shu ustunda saqlanadi. Faqat INSERT'da yoziladi — keyin
-- o'zgarmaydi (`PgOrderRepo.Save`).
ALTER TABLE orders ADD COLUMN IF NOT EXISTS placed_by TEXT NOT NULL DEFAULT '';
