-- Stolda ovqatlanish (dine_in): QR kod orqali buyurtma berish.
--
-- ── OQIM ───────────────────────────────────────────────────────────
-- Mijoz stoldagi QR kodni skanerlaydi → Telegram Mini App ochiladi →
-- o'sha restoranning menyusi ko'rinadi → savat → buyurtma → restoran
-- paneli (jonli) → qabul qilindi → tayyorlanmoqda → tayyor →
-- affitsiant ilovasiga push.
--
-- To'lov ilovada EMAS: mijoz affitsiantga naqd/karta bilan to'laydi
-- (foydalanuvchi qarori, 2026-08-12).

-- ── Stollar ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS restaurant_tables (
    id            TEXT PRIMARY KEY,
    restaurant_id TEXT        NOT NULL,
    -- label — mijoz va affitsiant ko'radigan nom: "5", "VIP-2",
    -- "Ayvon 3". Raqam EMAS, matn: restoranlar stollarini har xil
    -- nomlaydi va "12-stol"ni butun songa siqish keyin cheklov bo'lardi.
    label         TEXT        NOT NULL,
    -- qr_token — QR kod ichidagi SIR. Stol ID'sining o'zi emas:
    --
    -- ID ketma-ket yoki taxmin qilinadigan bo'lsa, istalgan odam
    -- boshqa stol nomidan buyurtma berardi. Token esa 32 baytlik
    -- tasodifiy qiymat — taxmin qilib bo'lmaydi.
    --
    -- Token QR chop etilganda YARATILADI va qayta yaratish mumkin
    -- (stol QR'i o'g'irlansa/nusxalansa — yangi token, eski QR
    -- darhol ishlamay qoladi).
    qr_token      TEXT        NOT NULL,
    -- active — stol vaqtincha ishlatilmayotgan bo'lsa (ta'mir,
    -- mavsumiy ayvon). O'CHIRISH o'rniga: o'chirilsa, o'sha stolga
    -- bog'langan eski buyurtmalar tarixi "yetim" qolardi.
    active        BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Token BUTUN TIZIM bo'ylab unikal: token bo'yicha qidirganda
-- restoran ID'si oldindan ma'lum bo'lmaydi (QR faqat tokenni beradi).
CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurant_tables_token
    ON restaurant_tables (qr_token);

-- Bitta restoranda bir xil nomli ikkita stol bo'lmasin — aks holda
-- affitsiant "5-stol" degan ikkita buyurtmani ajrata olmasdi.
CREATE UNIQUE INDEX IF NOT EXISTS idx_restaurant_tables_label
    ON restaurant_tables (restaurant_id, label);

CREATE INDEX IF NOT EXISTS idx_restaurant_tables_restaurant
    ON restaurant_tables (restaurant_id);

-- ── Buyurtmaga tur ─────────────────────────────────────────────────
--
-- DEFAULT 'delivery' + NOT NULL: mavjud barcha qatorlar avtomatik
-- to'ldiriladi. Busiz eski buyurtmalar NULL turda qolib, holat
-- mashinasidan o'ta olmasdi.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS order_type TEXT NOT NULL DEFAULT 'delivery';

-- table_id — REFERENCES qo'yilmadi ATAYLAB: stol o'chirilsa buyurtma
-- tarixi yo'qolmasligi kerak. `table_label` allaqachon nusxa saqlaydi,
-- shuning uchun stol yo'q bo'lsa ham buyurtma to'liq o'qiladi.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS table_id    TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS table_label TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS party_size  INT;

-- Restoran paneli "hozirgi stol buyurtmalari"ni shu indeks bilan
-- oladi (eng tez-tez so'raladigan so'rov).
CREATE INDEX IF NOT EXISTS idx_orders_dine_in
    ON orders (restaurant_id, status)
    WHERE order_type = 'dine_in';
