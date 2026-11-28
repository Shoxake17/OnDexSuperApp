-- 0081: OnDex POS (do'kon/apteka/qurt) — ShoxPos Node.js backendining Go'dagi
-- 1:1 o'rni (Plane ONDEXECOSY-486, egasi 2026-11-28).
--
-- Modul: internal/modules/shop — O'Z SXEMASI (ADR-003). Boshqa modullar bu
-- jadvallarga SQL yozmaydi.
--
-- DO'KON = OnDex akkaunti (users.role = 'store'), `store_id` = users.entity_id.
-- Har jadvalda `store_id` bor va HAR so'rov shu bilan cheklanadi (ShoxPos'dagi
-- `login` o'rni) — begona do'kon ma'lumotiga yo'l yo'q.
--
-- PUL: tiyin (BIGINT). ShoxPos so'mni float'da saqlardi (x100 ko'chiriladi).
-- MIQDOR: NUMERIC(14,3) — tarozida sotiladigan (kg) mahsulot kasr bo'lishi mumkin.
-- pg_trgm: global bazalarda (178k) ichki qism bo'yicha qidiruv indeks bilan.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE SCHEMA IF NOT EXISTS shop;

-- Do'kon mahsulotlari (ShoxPos `products`). quantity — sotuvda, incoming — kirim.
CREATE TABLE IF NOT EXISTS shop.products (
    store_id          TEXT        NOT NULL,
    barcode           TEXT        NOT NULL CHECK (barcode ~ '^[A-Za-z0-9_-]{1,64}$'),
    name              TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 300),
    category          TEXT        NOT NULL DEFAULT '',
    price_tiyin       BIGINT      NOT NULL DEFAULT 0 CHECK (price_tiyin >= 0),
    base_price_tiyin  BIGINT      NOT NULL DEFAULT 0 CHECK (base_price_tiyin >= 0),
    quantity          NUMERIC(14,3) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    incoming_quantity NUMERIC(14,3) NOT NULL DEFAULT 0 CHECK (incoming_quantity >= 0),
    sold              NUMERIC(14,3) NOT NULL DEFAULT 0 CHECK (sold >= 0),
    -- Qurt do'koni maydonlari (mijoz ilovasidagi qurt katalogi shundan quriladi)
    image_url         TEXT        NOT NULL DEFAULT '',
    description       TEXT        NOT NULL DEFAULT '',
    composition       TEXT        NOT NULL DEFAULT '',
    weight_grams      INT         NOT NULL DEFAULT 0 CHECK (weight_grams BETWEEN 0 AND 100000),
    added_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (store_id, barcode)
);
CREATE INDEX IF NOT EXISTS shop_products_name ON shop.products (store_id, lower(name));

-- Global shtrix-kod ma'lumotnomasi (ShoxPos `products_base`, ~178k). Faqat o'qish
-- (qidiruv, skaner) — do'konlar unga yozmaydi.
CREATE TABLE IF NOT EXISTS shop.product_base (
    barcode          TEXT   PRIMARY KEY,
    name             TEXT   NOT NULL,
    category         TEXT   NOT NULL DEFAULT '',
    price_tiyin      BIGINT NOT NULL DEFAULT 0,
    base_price_tiyin BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS shop_product_base_name_trgm ON shop.product_base USING gin (lower(name) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS shop_product_base_barcode_trgm ON shop.product_base USING gin (barcode gin_trgm_ops);

-- Kategoriyalar: do'konniki (ShoxPos user.shopCategories) va do'kon turiniki
-- (ShopType.categories — bir turdagi do'konlar uchun umumiy tavsiya ro'yxati).
CREATE TABLE IF NOT EXISTS shop.store_categories (
    store_id TEXT NOT NULL,
    name     TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    PRIMARY KEY (store_id, name)
);
CREATE TABLE IF NOT EXISTS shop.type_categories (
    shop_type TEXT NOT NULL,
    name      TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 64),
    PRIMARY KEY (shop_type, name)
);

-- Sotuvlar: KUNLIK yig'indi (ShoxPos `sales`): do'kon+mahsulot+to'lov turi+kun
-- bo'yicha bitta qator; total = quantity * oxirgi narx.
CREATE TABLE IF NOT EXISTS shop.sales (
    id               TEXT        PRIMARY KEY,
    store_id         TEXT        NOT NULL,
    barcode          TEXT        NOT NULL,
    name             TEXT        NOT NULL,
    category         TEXT        NOT NULL DEFAULT '',
    quantity         NUMERIC(14,3) NOT NULL CHECK (quantity >= 0),
    price_tiyin      BIGINT      NOT NULL DEFAULT 0,
    base_price_tiyin BIGINT      NOT NULL DEFAULT 0,
    total_tiyin      BIGINT      NOT NULL DEFAULT 0,
    payment_method   TEXT        NOT NULL DEFAULT 'cash' CHECK (length(payment_method) BETWEEN 1 AND 32),
    sale_day         DATE        NOT NULL,
    sold_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (store_id, barcode, payment_method, sale_day)
);
CREATE INDEX IF NOT EXISTS shop_sales_store_time ON shop.sales (store_id, sold_at DESC);

-- Har bir sotuv qatori (ShoxPos `salesbuckets`, 30 kun saqlanadi) — "Sotilganlar".
CREATE TABLE IF NOT EXISTS shop.sale_lines (
    id               BIGSERIAL   PRIMARY KEY,
    store_id         TEXT        NOT NULL,
    barcode          TEXT        NOT NULL,
    name             TEXT        NOT NULL,
    category         TEXT        NOT NULL DEFAULT '',
    quantity         NUMERIC(14,3) NOT NULL,
    price_tiyin      BIGINT      NOT NULL DEFAULT 0,
    base_price_tiyin BIGINT      NOT NULL DEFAULT 0,
    total_tiyin      BIGINT      NOT NULL DEFAULT 0,
    payment_method   TEXT        NOT NULL DEFAULT 'cash',
    sold_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS shop_sale_lines_store_time ON shop.sale_lines (store_id, sold_at DESC);

-- Qarzlar (ShoxPos `debts`).
CREATE TABLE IF NOT EXISTS shop.debts (
    id           TEXT        PRIMARY KEY,
    store_id     TEXT        NOT NULL,
    name         TEXT        NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    phone        TEXT        NOT NULL CHECK (length(phone) BETWEEN 1 AND 32),
    amount_tiyin BIGINT      NOT NULL CHECK (amount_tiyin > 0),
    last_payment TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS shop_debts_store ON shop.debts (store_id, last_payment DESC);

-- Apteka: dorilar (ShoxPos `pharmproducts`).
CREATE TABLE IF NOT EXISTS shop.pharm_products (
    store_id              TEXT        NOT NULL,
    barcode               TEXT        NOT NULL CHECK (barcode ~ '^[A-Za-z0-9_-]{1,64}$'),
    name                  TEXT        NOT NULL,
    category              TEXT        NOT NULL DEFAULT '',
    price_tiyin           BIGINT      NOT NULL DEFAULT 0 CHECK (price_tiyin >= 0),
    base_price_tiyin      BIGINT      NOT NULL DEFAULT 0 CHECK (base_price_tiyin >= 0),
    quantity              NUMERIC(14,3) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    incoming_quantity     NUMERIC(14,3) NOT NULL DEFAULT 0 CHECK (incoming_quantity >= 0),
    sold                  NUMERIC(14,3) NOT NULL DEFAULT 0 CHECK (sold >= 0),
    dosage                TEXT        NOT NULL DEFAULT '',
    manufacturer          TEXT        NOT NULL DEFAULT '',
    expiry_date           DATE,
    batch_number          TEXT        NOT NULL DEFAULT '',
    prescription_required BOOLEAN     NOT NULL DEFAULT false,
    storage_condition     TEXT        NOT NULL DEFAULT '',
    description           TEXT        NOT NULL DEFAULT '',
    dosage_form           TEXT        NOT NULL DEFAULT '',
    shelf_number          TEXT        NOT NULL DEFAULT '',
    row_number            TEXT        NOT NULL DEFAULT '',
    added_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (store_id, barcode)
);

-- Apteka global ma'lumotnomasi (ShoxPos `pharm_base`).
CREATE TABLE IF NOT EXISTS shop.pharm_base (
    barcode               TEXT    PRIMARY KEY,
    name                  TEXT    NOT NULL,
    category              TEXT    NOT NULL DEFAULT '',
    base_price_tiyin      BIGINT  NOT NULL DEFAULT 0,
    dosage                TEXT    NOT NULL DEFAULT '',
    manufacturer          TEXT    NOT NULL DEFAULT '',
    storage_condition     TEXT    NOT NULL DEFAULT '',
    side_effects          TEXT    NOT NULL DEFAULT '',
    description           TEXT    NOT NULL DEFAULT '',
    composition           TEXT    NOT NULL DEFAULT '',
    dosage_form           TEXT    NOT NULL DEFAULT '',
    prescription_required BOOLEAN NOT NULL DEFAULT false,
    added_by_store        TEXT    NOT NULL DEFAULT '',
    added_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS shop_pharm_base_name_trgm ON shop.pharm_base USING gin (lower(name) gin_trgm_ops);
CREATE INDEX IF NOT EXISTS shop_pharm_base_barcode_trgm ON shop.pharm_base USING gin (barcode gin_trgm_ops);

-- Apteka sotuvlari (ShoxPos `pharmsales`): bitta chek = bir nechta qator.
CREATE TABLE IF NOT EXISTS shop.pharm_sales (
    id             TEXT        PRIMARY KEY,
    store_id       TEXT        NOT NULL,
    items          JSONB       NOT NULL,
    total_tiyin    BIGINT      NOT NULL CHECK (total_tiyin >= 0),
    payment_method TEXT        NOT NULL CHECK (payment_method IN ('naqd','plastik','qarz')),
    sold_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS shop_pharm_sales_store_time ON shop.pharm_sales (store_id, sold_at DESC);

-- Qurt buyurtmasi uchun ombordan ayirilgan qoldiq (ShoxPos QurtOrderLedger):
-- do'kon buyurtmani TASDIQLAGANDA yoziladi, bekor qilinsa aynan BIR MARTA qaytadi.
CREATE TABLE IF NOT EXISTS shop.qurt_order_ledger (
    order_id   TEXT        PRIMARY KEY,
    store_id   TEXT        NOT NULL,
    items      JSONB       NOT NULL,
    state      TEXT        NOT NULL CHECK (state IN ('reserved', 'restored')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
