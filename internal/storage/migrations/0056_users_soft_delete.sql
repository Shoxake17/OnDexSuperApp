-- Mijozning "Akkauntni o'chirish" — YUMSHOQ o'chirish.
--
-- NEGA `couriers.deleted_at` (migration 0035) BILAN BIR XIL NAQSH:
-- foydalanuvchi akkauntini o'chirganda MA'LUMOTI (buyurtmalar,
-- sevimlilar, manzil) SAQLANIB QOLISHI kerak — faqat kirish yopiladi.
-- Agar keyin O'SHA telefon/email bilan qayta tasdiqlansa (SMS/Telegram/
-- Google), akkaunt TIRILADI va eski ma'lumot qaytadan ko'rinadi
-- (`internal/users/service.go` dagi reaktivatsiya mantig'i).
--
-- Bu Google Play / App Store'ning "hisobni o'chirish" talabi bilan
-- ham mos: platforma "ma'lumotni butunlay o'chirish" emas, "kirishni
-- to'xtatish + tiklash imkoni" variantini tanladi.
--
-- QATTIQ o'chirish (`users.Delete`, superadmin panel) BUTUNLAY
-- BOSHQA yo'l — u haqiqatan tozalaydi va bu ustunga tegishli emas.

ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Superadmin ro'yxatida "o'chirilgan" akkauntlarni ajratish uchun.
CREATE INDEX IF NOT EXISTS idx_users_deleted_at
    ON users (id) WHERE deleted_at IS NOT NULL;
