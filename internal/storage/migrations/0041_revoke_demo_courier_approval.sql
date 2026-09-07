-- Demo kuryerlarning PRODUCTION'dagi tasdig'ini bekor qiladi.
--
-- ┌─ NEGA KERAK (bug.md 98-band) ─────────────────────────────────────┐
-- `0004_courier_approval.sql` migratsiyasi `c1`, `c2`, `c3` ID'li
-- kuryerlarni `approved = TRUE` qilib belgilardi. Migratsiya HAR BIR
-- muhitda bajariladi, ya'ni production bazasida ham.
--
-- Tasdiqlangan kuryer buyurtma takliflarini oladi va mijozning
-- manzilini hamda telefon raqamini ko'radi. Bu qarorni superadmin
-- qo'lda qabul qilishi kerak — migratsiya emas.
--
-- 0004 dan o'sha qator olib tashlandi (yangi bazalar uchun), lekin
-- migratsiya ALLAQACHON qo'llangan bazalarda u o'z ishini qilib
-- bo'lgan. Shu sabab bu tuzatuvchi migratsiya.
-- └───────────────────────────────────────────────────────────────────┘
--
-- ┌─ NEGA SHUNCHAKI `WHERE id IN (...)` EMAS ─────────────────────────┐
-- Haqiqiy kuryer ham `c1` ID'siga ega bo'lishi NAZARIY jihatdan
-- mumkin (ID'lar tasodifiy hex, lekin qo'lda kiritilgan yozuv ham
-- bo'lishi mumkin). Ishlab turgan kuryerni tasodifan o'chirib
-- qo'yish — real zarar.
--
-- Shuning uchun uch shart BIRGA tekshiriladi: demo ID, demo ISM va
-- HECH QANDAY buyurtma bajarmagan. Uchalasi mos kelsa — bu aniq
-- demo yozuv.
--
-- `completed_orders = 0` sharti eng muhimi: ishlab turgan kuryer
-- bu tekshiruvdan hech qachon o'tmaydi.
-- └───────────────────────────────────────────────────────────────────┘
UPDATE couriers
   SET approved = FALSE
 WHERE id IN ('c1', 'c2', 'c3')
   AND name IN ('Aziz', 'Bekzod', 'Doniyor')
   AND completed_orders = 0;
