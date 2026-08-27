-- Buyurtmaga bir vaqtda BIR NECHTA aksiya tushishi mumkin bo'lib qoldi
-- (har savat qatori o'zining eng foydali chegirmasini oladi), ustiga
-- mahsulotning o'z chegirma narxi ham qo'shilishi mumkin. `promotion_id`
-- esa bitta — eng ko'p hissa qo'shgan aksiya.
--
-- Shu ustun o'sha aksiyaning AYNAN O'ZI bergan summani saqlaydi: chekda
-- aksiya nomi faqat u butun chegirmaga teng bo'lganda ko'rsatiladi,
-- aks holda "Chegirma" deb yoziladi (mijozga noto'g'ri manba
-- ko'rsatilmasin).
ALTER TABLE orders ADD COLUMN IF NOT EXISTS promotion_discount_tiyin BIGINT NOT NULL DEFAULT 0;
