-- Foydalanuvchi qaysi mijoz dasturidan kirganini qayd etish.
--
-- ┌─ NEGA KERAK ──────────────────────────────────────────────────────┐
-- Superadmin panelida "mijoz qaysi qurilmadan foydalanyapti" degan
-- savolga javob beradigan yagona manba YO'Q edi. `device_tokens`
-- (migration 0030) da `platform` bor, lekin u FAQAT push tokeni
-- ro'yxatdan o'tgan qurilmalarni biladi — ya'ni Telegram Mini App
-- va brauzerdagi foydalanuvchilar u yerda UMUMAN ko'rinmaydi
-- (ularda FCM tokeni bo'lmaydi).
--
-- Bu jadval esa har bir autentifikatsiyalangan so'rovdan (`middleware.go`
-- dagi `X-Ondex-Client` sarlavhasi) to'ldiriladi, ya'ni TMA ham,
-- mobil ilova ham, panel ham bir xil ko'rinadi.
-- └───────────────────────────────────────────────────────────────────┘
--
-- MAXFIYLIK: bu yerda faqat PLATFORMA va ILOVA VERSIYASI saqlanadi.
-- IP manzil, qurilma modeli yoki boshqa barmoq izi ATAYLAB
-- saqlanmaydi — admin panelidagi vazifa uchun kerak emas, saqlash esa
-- keraksiz shaxsiy ma'lumot yig'ish bo'lardi.

CREATE TABLE IF NOT EXISTS user_devices (
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- platform — server tomonda ROʻYXATDAN tekshiriladi
    -- (`users.NormalizePlatform`): mijoz yuborgan ixtiyoriy satr bu
    -- yerga tushmaydi.
    platform    TEXT NOT NULL,
    app_version TEXT NOT NULL DEFAULT '',
    first_seen  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Kalit (user, platform): bitta odam ham telefonda, ham TMA'da
    -- bo'lishi MUMKIN va admin ikkalasini ham ko'rishi kerak. Lekin
    -- har kirish uchun yangi qator yozilmaydi — mavjudi yangilanadi.
    PRIMARY KEY (user_id, platform)
);

-- "Oxirgi 24 soatda faol" kabi saralashlar uchun.
CREATE INDEX IF NOT EXISTS idx_user_devices_last_seen
    ON user_devices (last_seen DESC);
