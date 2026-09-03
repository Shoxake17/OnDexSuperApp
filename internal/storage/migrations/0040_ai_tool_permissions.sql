-- Ilova ichidagi yordamchi (Shaddiy) uchun AMAL RUXSATLARI.
--
-- ┌─ NEGA "O'CHIRILGANLAR" SAQLANADI, "YOQILGANLAR" EMAS ──────────────┐
-- Sukut bo'yicha barcha amallar yoqilgan. Agar ro'yxatda YOQILGANLAR
-- saqlansa, keyinchalik yangi amal qo'shilganda u barcha eski
-- foydalanuvchilarda JIMGINA o'chiq bo'lib qolardi va sabab hech
-- qayerda ko'rinmasdi.
--
-- O'CHIRILGANLAR saqlansa esa yangi amal hammada ishlaydi, foydalanuvchi
-- esa xohlasa o'chiradi — ya'ni ro'yxat foydalanuvchining AYNIQSA
-- bildirgan qarorlarini saqlaydi, sukutni emas.
-- └────────────────────────────────────────────────────────────────────┘
--
-- Vergul bilan ajratilgan amal nomlari (masalan "cancel_order,my_orders").
-- Bo'sh satr — hech narsa o'chirilmagan.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS ai_disabled_tools TEXT NOT NULL DEFAULT '';
