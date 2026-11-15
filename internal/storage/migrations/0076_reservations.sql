-- 0076: Stol broni + zaklad (RS-3, Plane 369; ENG-1 booking dvigateli).
--
-- Modul: internal/modules/reservation — O'Z SXEMASI (ADR-003). Boshqa
-- modullar bu jadvallarga SQL yozmaydi, faqat modul API'si orqali.
-- Stol va foydalanuvchiga FK ATAYLAB yo'q (modul chegarasi): ular modul
-- API'si orqali tekshiriladi.
--
-- ┌─ IKKI KISHI BIR STOLNI BIR VAQTDA BAND QILA OLMAYDI ──────────────────┐
-- Kafolat ilova kodida emas, BAZADA: EXCLUDE (gist) — bir stolning faol
-- bronlari (held/confirmed/seated) vaqt oralig'i bo'yicha kesishmaydi.
-- Parallel so'rovlarning ikkinchisi 23P01 bilan rad etiladi.
-- └──────────────────────────────────────────────────────────────────────┘
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE SCHEMA IF NOT EXISTS reservation;

-- Restoranning bron sozlamalari (restoran panelida tahrirlanadi).
CREATE TABLE IF NOT EXISTS reservation.settings (
    restaurant_id            TEXT PRIMARY KEY,
    enabled                  BOOLEAN NOT NULL DEFAULT FALSE,
    -- Zaklad KISHI BOSHIGA (egasi qarori 2026-11-15), tiyinda. 0 = zakladsiz.
    deposit_per_person_tiyin BIGINT  NOT NULL DEFAULT 0 CHECK (deposit_per_person_tiyin >= 0),
    -- Zona bo'yicha farq ({"VIP": 10000000}); yo'q zona = umumiy qiymat.
    zone_deposits            JSONB   NOT NULL DEFAULT '{}'::jsonb,
    -- Bitta bron davomiyligi (stol shuncha vaqt band).
    slot_minutes             INT     NOT NULL DEFAULT 120 CHECK (slot_minutes BETWEEN 30 AND 480),
    -- Eng kamida qancha oldin bron qilinadi va necha kun oldinga.
    min_advance_minutes      INT     NOT NULL DEFAULT 60 CHECK (min_advance_minutes BETWEEN 0 AND 1440),
    max_days_ahead           INT     NOT NULL DEFAULT 30 CHECK (max_days_ahead BETWEEN 1 AND 90),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS reservation.reservations (
    id              TEXT PRIMARY KEY,
    restaurant_id   TEXT        NOT NULL,
    table_id        TEXT        NOT NULL,
    -- Buyurtma vaqtidagi NUSXA (stol keyin qayta nomlansa ham tarix o'zgarmaydi).
    table_label     TEXT        NOT NULL DEFAULT '',
    customer_id     TEXT        NOT NULL,
    party_size      INT         NOT NULL CHECK (party_size BETWEEN 1 AND 50),
    starts_at       TIMESTAMPTZ NOT NULL,
    ends_at         TIMESTAMPTZ NOT NULL,
    -- held -> confirmed -> seated -> completed; cancelled / no_show / expired.
    status          TEXT        NOT NULL,
    deposit_tiyin   BIGINT      NOT NULL DEFAULT 0 CHECK (deposit_tiyin >= 0),
    -- card | wallet | '' (zakladsiz)
    deposit_method  TEXT        NOT NULL DEFAULT '',
    -- '' | pending | paid | refunded | kept | applied
    deposit_state   TEXT        NOT NULL DEFAULT '',
    comment         TEXT        NOT NULL DEFAULT '',
    -- held holatida to'lov kutish muddati (o'tsa — expired, stol bo'shaydi).
    hold_expires_at TIMESTAMPTZ,
    cancel_reason   TEXT        NOT NULL DEFAULT '',
    -- customer | restaurant | system
    cancelled_by    TEXT        NOT NULL DEFAULT '',
    reminded_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL,
    CHECK (ends_at > starts_at),
    CONSTRAINT reservations_no_overlap EXCLUDE USING gist (
        table_id WITH =,
        tstzrange(starts_at, ends_at, '[)') WITH &&
    ) WHERE (status IN ('held', 'confirmed', 'seated'))
);

CREATE INDEX IF NOT EXISTS reservations_restaurant_day
    ON reservation.reservations (restaurant_id, starts_at);
CREATE INDEX IF NOT EXISTS reservations_customer
    ON reservation.reservations (customer_id, starts_at DESC);
-- Fon nazoratchilari: muddati o'tgan hold va kelmaganlar.
CREATE INDEX IF NOT EXISTS reservations_active_due
    ON reservation.reservations (status, starts_at)
    WHERE status IN ('held', 'confirmed');

-- ── To'lovlar: zaklad ham to'lov ─────────────────────────────────────────
-- To'lov endi buyurtmaga YOKI bronga tegishli (aynan bittasiga).
-- Eski kod faqat order_id'ni yozadi — CHECK uni buzmaydi.
ALTER TABLE payments ALTER COLUMN order_id DROP NOT NULL;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS reservation_id TEXT;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'payments_one_subject') THEN
        ALTER TABLE payments ADD CONSTRAINT payments_one_subject
            CHECK (num_nonnulls(order_id, reservation_id) = 1);
    END IF;
END $$;
CREATE INDEX IF NOT EXISTS payments_reservation
    ON payments (reservation_id) WHERE reservation_id IS NOT NULL;
