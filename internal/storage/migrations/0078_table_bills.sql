-- 0078: Stolda hisobni bo'lish (RS-5, Plane 371). Modul: internal/modules/tablebill.
--
-- Egasi qarorlari (2026-11-16): har mehmon teng ulush YOKI o'z taomlarini
-- to'laydi; aralash — qolganini ofitsiant naqd/terminal deb yopadi; ixtiyoriy
-- choy puli (0/5/10%) ofitsiantga; bron zakladi hisobdan ayriladi.
--
-- Hisob = stolning ochiq "seansi": shu stolning to'lanmagan stol buyurtmalari.
-- Ulush summasi SERVERDA hisoblanadi (klientdan olinmaydi).
CREATE SCHEMA IF NOT EXISTS tablebill;

CREATE TABLE IF NOT EXISTS tablebill.bills (
    id                     TEXT PRIMARY KEY,
    restaurant_id          TEXT        NOT NULL,
    table_id               TEXT        NOT NULL,
    table_label            TEXT        NOT NULL DEFAULT '',
    -- open | paid
    status                 TEXT        NOT NULL,
    -- Zakladi shu hisobga ayrilgan bron (bir marta).
    deposit_reservation_id TEXT        NOT NULL DEFAULT '',
    deposit_tiyin          BIGINT      NOT NULL DEFAULT 0 CHECK (deposit_tiyin >= 0),
    created_at             TIMESTAMPTZ NOT NULL,
    updated_at             TIMESTAMPTZ NOT NULL,
    closed_at              TIMESTAMPTZ,
    version                BIGINT      NOT NULL DEFAULT 1
);
-- Bitta stolda bir vaqtda faqat BITTA ochiq hisob.
CREATE UNIQUE INDEX IF NOT EXISTS bills_one_open_per_table
    ON tablebill.bills (table_id) WHERE status = 'open';

-- Hisobga kirgan buyurtmalar (buyurtma faqat bitta hisobda).
CREATE TABLE IF NOT EXISTS tablebill.bill_orders (
    bill_id  TEXT NOT NULL REFERENCES tablebill.bills(id) ON DELETE CASCADE,
    order_id TEXT NOT NULL PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS tablebill.shares (
    id           TEXT PRIMARY KEY,
    bill_id      TEXT        NOT NULL REFERENCES tablebill.bills(id) ON DELETE CASCADE,
    payer_id     TEXT        NOT NULL DEFAULT '',
    -- equal | items | staff
    kind         TEXT        NOT NULL,
    -- card | cash | terminal
    method       TEXT        NOT NULL,
    amount_tiyin BIGINT      NOT NULL CHECK (amount_tiyin > 0),
    tip_tiyin    BIGINT      NOT NULL DEFAULT 0 CHECK (tip_tiyin >= 0),
    -- items rejimida: [{"order_id":..,"line":..,"qty":..}]
    items        JSONB       NOT NULL DEFAULT '[]'::jsonb,
    -- Choy puli kimga (tayyor taomni olib borgan ofitsiant, RS-2).
    waiter_id    TEXT        NOT NULL DEFAULT '',
    waiter_name  TEXT        NOT NULL DEFAULT '',
    -- pending | paid | failed
    status       TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    paid_at      TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS shares_bill ON tablebill.shares (bill_id);

-- To'lov endi buyurtma, bron YOKI hisob ulushiga tegishli (aynan bittasi).
ALTER TABLE payments ADD COLUMN IF NOT EXISTS bill_share_id TEXT;
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_one_subject;
ALTER TABLE payments ADD CONSTRAINT payments_one_subject
    CHECK (num_nonnulls(order_id, reservation_id, bill_share_id) = 1);
CREATE INDEX IF NOT EXISTS payments_bill_share
    ON payments (bill_share_id) WHERE bill_share_id IS NOT NULL;
