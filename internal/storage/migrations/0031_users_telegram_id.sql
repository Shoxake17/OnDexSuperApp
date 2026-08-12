-- Telegram Mini App (TMA) uchun kimlik bog'lanishi.
--
-- ── MUAMMO ─────────────────────────────────────────────────────────
-- Telegram Mini App ochilganda `initData` faqat TELEGRAM ID beradi.
-- Telefon raqami u yerda YO'Q va hech qachon bo'lmaydi.
--
-- OnDex'da esa kimlik — TELEFON RAQAMI (`users.phone`). SMS, Firebase
-- va Telegram orqali kirish — uchalasi ham bitta akkauntga olib
-- boradi, chunki ular bir xil raqamga tayanadi.
--
-- ── YECHIM ─────────────────────────────────────────────────────────
-- Foydalanuvchi botda kontaktini ULASHGANDA (bu allaqachon bor:
-- `AskContact`), telegram_id ↔ telefon bog'lanishi shu ustunga
-- yoziladi. Keyin Mini App faqat telegram_id bilan kelsa ham server
-- uning raqamini biladi va O'SHA akkauntni ochadi.
--
-- Natija: mijoz Mini App'da ham, mobil ilovada ham, saytda ham
-- BITTA akkauntda bo'ladi.

ALTER TABLE users ADD COLUMN IF NOT EXISTS telegram_id BIGINT;

-- ── NEGA UNIQUE ────────────────────────────────────────────────────
-- Bitta Telegram akkaunti FAQAT bitta OnDex foydalanuvchisiga
-- bog'lanishi kerak. Busiz bir odam bir nechta akkauntga bog'lanib,
-- Mini App'da qaysi biriga kirishi noaniq bo'lardi.
--
-- QISMAN indeks (`WHERE telegram_id IS NOT NULL`): NULL qiymatlar
-- indeksda umuman saqlanmaydi. Foydalanuvchilarning aksariyati
-- Telegram bog'lamaydi, shuning uchun indeks kichik bo'lib qoladi.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_telegram_id
    ON users (telegram_id) WHERE telegram_id IS NOT NULL;
