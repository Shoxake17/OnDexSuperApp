-- Ro'yxatdan o'tish ekrani (image/register.png) uchun: parol, email va
-- alohida ism/familiya.
--
-- KONTEKST: platformaning asosiy kirish yo'li telefon + SMS kod
-- (parolsiz) bo'lib qoladi. Parol QO'SHIMCHA yo'l sifatida qo'shildi.
-- Shu sabab `password_hash` NULL bo'lishi mumkin: mavjud
-- foydalanuvchilarda parol yo'q va ular avvalgidek SMS kod bilan
-- kirishda davom etadi.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT NOT NULL DEFAULT '';

-- Ism va familiya ALOHIDA. Mavjud `name` ustuni saqlanadi va yozishda
-- "Ism Familiya" ko'rinishida to'ldiriladi — shu sabab `name` ni
-- o'qiydigan eski kod (profil, admin panel, kuryer ilovasi) o'zgarishsiz
-- ishlashda davom etadi.
ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name  TEXT NOT NULL DEFAULT '';

-- Telefon tasdiqlangan-yo'qligi. Ro'yxatdan o'tishda akkaunt DARHOL
-- yaratiladi, lekin SMS kod tasdiqlanmaguncha `phone_verified = FALSE`
-- va kirish tokeni BERILMAYDI.
--
-- NEGA MUHIM: busiz istalgan odam BEGONA telefon raqami bilan akkaunt
-- ochib, o'sha raqam egasining ro'yxatdan o'tishini butunlay to'sib
-- qo'yardi (raqam band bo'lib qolardi).
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone_verified BOOLEAN NOT NULL DEFAULT TRUE;

-- Email unikal — LEKIN faqat bo'sh bo'lmaganlar orasida.
-- Oddiy UNIQUE ishlamaydi: parolsiz/emailsiz mavjud foydalanuvchilarda
-- email `''` va ular hammasi bir-biriga to'qnashib ketardi.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique
    ON users (lower(email)) WHERE email <> '';
