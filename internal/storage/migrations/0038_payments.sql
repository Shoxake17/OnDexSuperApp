-- Karta orqali to'lov (Octo) — to'lov urinishlari va buyurtmadagi holat.
--
-- ┌─ NEGA ALOHIDA JADVAL, BUYURTMADA MAYDON EMAS ─────────────────────┐
-- Bitta buyurtmaga BIR NECHTA urinish bo'lishi mumkin: mijoz kartani
-- noto'g'ri kiritsa yoki muddat tugasa, yangi urinish boshlanadi.
-- Octo bir xil `shop_transaction_id` ni qayta ishlatishga ruxsat
-- bermaydi (jonli sinovda HTTP 500 qaytardi), ya'ni har urinish uchun
-- yangi ID kerak. Buyurtmada esa faqat YAKUNIY holat saqlanadi.
-- └───────────────────────────────────────────────────────────────────┘

-- Buyurtmadagi to'lov holati. BO'SH qiymat = naqd (eski buyurtmalar
-- ham shunday qoladi va to'lov tekshiruviga umuman tushmaydi).
ALTER TABLE orders ADD COLUMN IF NOT EXISTS payment_method TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS payment_state TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS payments (
    id                  TEXT PRIMARY KEY,
    order_id            TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    customer_id         TEXT NOT NULL DEFAULT '',
    restaurant_id       TEXT NOT NULL DEFAULT '',
    provider            TEXT NOT NULL,
    -- Provayder tomondagi ID (Octo'da `octo_payment_UUID`). To'lov
    -- yaratilgunga qadar bo'sh bo'ladi.
    provider_payment_id TEXT NOT NULL DEFAULT '',
    amount_tiyin        BIGINT NOT NULL,
    status              TEXT NOT NULL,
    pay_url             TEXT NOT NULL DEFAULT '',
    -- To'lov haqida xabar keldi, lekin ISHONCHLI tasdiqlab bo'lmadi —
    -- odam ko'rib chiqishi kerak (avtomatik "to'landi" bo'lmaydi).
    needs_review        BOOLEAN NOT NULL DEFAULT FALSE,
    review_reason       TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    paid_at             TIMESTAMPTZ,
    expires_at          TIMESTAMPTZ NOT NULL,
    -- Provayderning xom javobi — nizo chiqqanda yagona dalil.
    raw                 JSONB
);

CREATE INDEX IF NOT EXISTS idx_payments_order ON payments(order_id);

-- Callback provayder ID'si bilan keladi va u YAGONA bo'lishi shart:
-- ikkita yozuv bir xil provayder to'loviga ishora qilsa, qaysi
-- buyurtmani to'langan deb belgilashni bilib bo'lmasdi.
CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_provider_id
    ON payments(provider, provider_payment_id)
    WHERE provider_payment_id <> '';

-- Muddati o'tgan, hali yakunlanmagan to'lovlarni tez topish uchun
-- (fon vazifasi ularni yopadi va buyurtmani bekor qiladi).
CREATE INDEX IF NOT EXISTS idx_payments_expiry
    ON payments(expires_at) WHERE status = 'pending';
