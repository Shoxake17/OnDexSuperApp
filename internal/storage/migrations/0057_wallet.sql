-- OnDex Wallet — o'zgarmas (append-only) ledger. Reja: ondexwallet.md.
--
-- Balans uchun ALOHIDA ustun YO'Q — har doim
-- `SUM(amount_tiyin) WHERE user_id=?` dan hisoblanadi. Concurrency
-- himoyasi Postgres repo'da `pg_advisory_xact_lock` orqali (bir xil
-- naqsh `internal/staff` restoran bo'yicha tartib raqami berishda
-- ishlatgan, migratsiya 0045).
CREATE TABLE IF NOT EXISTS wallet_transactions (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- order_id — FK YO'Q ataylab: keshbek/spend yozuvi buyurtma
    -- ALLAQACHON mavjud bo'lgandagina yoziladi, lekin kelajakda
    -- admin_adjust kabi buyurtmasiz yozuvlar ham bo'ladi.
    order_id     TEXT NOT NULL DEFAULT '',
    -- amount_tiyin — musbat=kredit (keshbek/qaytarish), manfiy=debit
    -- (checkout'da sarflash).
    amount_tiyin BIGINT NOT NULL,
    type         TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_wallet_tx_user
    ON wallet_transactions(user_id, created_at DESC);

-- Idempotentlik: bitta buyurtmaga bitta TUR faqat BIR MARTA
-- (masalan bitta buyurtmaga ikki marta keshbek yozilmasin).
CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_tx_order_type
    ON wallet_transactions(order_id, type) WHERE order_id <> '';
