-- Email bilan ro'yxatdan o'tish uchun telefon IXTIYORIY bo'ldi.
--
-- `users.phone` da UNIQUE cheklov bor edi va u NOT NULL. Telefonsiz
-- (faqat email bilan) yaratilgan akkauntlarda telefon bo'sh satr
-- bo'ladi — oddiy UNIQUE bilan IKKINCHI shunday akkaunt yaratib
-- bo'lmasdi (hammasi `''` bo'lib bir-biriga to'qnashardi).
--
-- Yechim: unikallik faqat BO'SH BO'LMAGAN telefonlar orasida
-- talab qilinadi (qisman indeks) — email uchun ishlatilgan naqshning
-- aynan o'zi (migratsiya 0026).
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_phone_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_unique
    ON users (phone) WHERE phone <> '';
