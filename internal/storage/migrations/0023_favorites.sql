CREATE TABLE IF NOT EXISTS favorites (
    customer_id TEXT NOT NULL,
    product_id  TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (customer_id, product_id)
);

CREATE INDEX IF NOT EXISTS idx_favorites_customer ON favorites (customer_id, created_at DESC);
