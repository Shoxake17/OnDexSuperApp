CREATE TABLE IF NOT EXISTS promotions (
    id                    TEXT PRIMARY KEY,
    restaurant_id         TEXT NOT NULL REFERENCES restaurants(id),
    name                  TEXT NOT NULL,
    description           TEXT NOT NULL DEFAULT '',
    type                  TEXT NOT NULL,
    discount_percent      INT NOT NULL DEFAULT 0,
    discount_amount_tiyin BIGINT NOT NULL DEFAULT 0,
    start_date            DATE NOT NULL,
    end_date              DATE NOT NULL,
    time_start            TEXT NOT NULL DEFAULT '',
    time_end              TEXT NOT NULL DEFAULT '',
    image_url             TEXT NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_promotions_restaurant ON promotions (restaurant_id);
