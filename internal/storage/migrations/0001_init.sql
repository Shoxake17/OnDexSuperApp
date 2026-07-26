CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS couriers (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    lat        DOUBLE PRECISION NOT NULL DEFAULT 0,
    lng        DOUBLE PRECISION NOT NULL DEFAULT 0,
    available  BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id            TEXT PRIMARY KEY,
    customer_id   TEXT NOT NULL,
    restaurant_id TEXT NOT NULL,
    courier_id    TEXT REFERENCES couriers(id),
    status        TEXT NOT NULL,
    total_tiyin   BIGINT NOT NULL DEFAULT 0,
    delivery_lat  DOUBLE PRECISION NOT NULL DEFAULT 0,
    delivery_lng  DOUBLE PRECISION NOT NULL DEFAULT 0,
    -- MVP bosqichida items va history JSONB'da; alohida jadvalga keyin
    -- (hisobotlar kerak bo'lganda) ko'chiriladi
    items         JSONB NOT NULL DEFAULT '[]',
    history       JSONB NOT NULL DEFAULT '[]',
    created_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_orders_status   ON orders (status);
CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders (customer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_couriers_avail  ON couriers (available);
