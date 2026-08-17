-- Kuryer akkaunti o'chirilganda yozuvni ro'yxatlardan olib tashlash.
--
-- NEGA `DELETE` EMAS: `orders.courier_id` shu jadvalga FOREIGN KEY,
-- ya'ni bir marta yetkazgan kuryerni haqiqatan o'chirish yo baza
-- cheklovini buzadi, yo tarixiy buyurtmalarni birga olib ketadi.
-- Batafsil sabab: `internal/couriers/courier.go` dagi `SoftDelete`.
--
-- Shaxsiy ma'lumot (ism) o'chirish paytida TOZALANADI — bu ustun
-- faqat "ro'yxatlarda ko'rsatma" belgisi.

ALTER TABLE couriers ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Faol kuryerlar bo'yicha so'rovlar (dispatch, superadmin ro'yxati)
-- endi `deleted_at IS NULL` shartini qo'shadi — qisman indeks shu
-- so'rovlarni tez saqlaydi.
CREATE INDEX IF NOT EXISTS idx_couriers_not_deleted
    ON couriers (available) WHERE deleted_at IS NULL;
