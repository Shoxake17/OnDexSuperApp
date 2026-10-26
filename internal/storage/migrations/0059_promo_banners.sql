-- Reklama/aksiya bannerlari — admin panelidan boshqariladi, mijoz
-- ilovasining restoran sahifasida ko'rsatiladi (Yandex Eats/Go
-- uslubida, `internal/banners`).
--
-- `restaurant_id` ga tashqi kalit YO'Q (`staff_members` bilan bir xil
-- sabab, 0045_staff.sql): katalog (restoranlar) MongoDB'da, bu jadval
-- esa Postgres'da — ikkalasi bir xil baza emas.
CREATE TABLE IF NOT EXISTS promo_banners (
    id            TEXT PRIMARY KEY,
    image_url     TEXT NOT NULL,
    restaurant_id TEXT NOT NULL DEFAULT '',
    active        BOOLEAN NOT NULL DEFAULT true,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Mijoz ilovasi faqat FAOL bannerlarni, tartib bo'yicha so'raydi.
CREATE INDEX IF NOT EXISTS idx_promo_banners_active_sort
    ON promo_banners (active, sort_order);
