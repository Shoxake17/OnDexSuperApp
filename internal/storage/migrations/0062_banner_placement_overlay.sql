-- Banner joylashuvi va ixtiyoriy matn qatlami (`internal/banners`).
--
-- placement: 'menu' (bosh sahifa), 'restaurant' (restoran sahifasi), 'store' (do'kon/qurt
-- sahifasi). Mavjud bannerlar hozirgacha restoran sahifasida ko'rinardi — ular
-- 'restaurant' bo'lib QOLADI (DEFAULT), shuning uchun hech narsa joyidan siljimaydi.
--
-- title/subtitle/button_text: rasm ustiga chiziladigan ixtiyoriy matn (rasmning o'zida
-- matn bo'lsa bo'sh qoladi). action: tugma/banner bosilganda qayerga olib borishi
-- ('' = yo'q, 'qurt' = Qurt sahifasi); restoranga havola avvalgidek `restaurant_id` da.
ALTER TABLE promo_banners
    ADD COLUMN IF NOT EXISTS placement   TEXT NOT NULL DEFAULT 'restaurant',
    ADD COLUMN IF NOT EXISTS title       TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS subtitle    TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS button_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS action      TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_promo_banners_placement
    ON promo_banners (placement, active, sort_order);