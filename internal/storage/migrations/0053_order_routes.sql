-- Yetkazish yo'li (A — restoran, B — mijoz manzili) buyurtmaga biriktirilib
-- saqlanadi: mijoz buyurtmani olgandan keyin ham kuzatuv sahifasida o'sha
-- xarita ko'rinib turadi (internal/tracking).
--
-- Bir buyurtmaga bitta yozuv; birinchisi qoladi (ON CONFLICT DO NOTHING) —
-- tarix keyingi hisob-kitob bilan o'zgarmaydi.
CREATE TABLE IF NOT EXISTS order_routes (
    order_id         TEXT PRIMARY KEY,
    origin_lat       DOUBLE PRECISION NOT NULL,
    origin_lng       DOUBLE PRECISION NOT NULL,
    dest_lat         DOUBLE PRECISION NOT NULL,
    dest_lng         DOUBLE PRECISION NOT NULL,
    points           JSONB NOT NULL,
    distance_meters  INTEGER NOT NULL CHECK (distance_meters >= 0),
    duration_seconds INTEGER NOT NULL CHECK (duration_seconds >= 0),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
