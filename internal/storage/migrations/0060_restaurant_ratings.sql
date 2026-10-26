CREATE TABLE IF NOT EXISTS restaurant_ratings (
    id            TEXT PRIMARY KEY,
    order_id      TEXT NOT NULL UNIQUE,
    restaurant_id TEXT NOT NULL DEFAULT '',
    customer_id   TEXT NOT NULL DEFAULT '',
    stars         INTEGER NOT NULL CHECK (stars BETWEEN 1 AND 5),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_restaurant_ratings_restaurant ON restaurant_ratings (restaurant_id);
