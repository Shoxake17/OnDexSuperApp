-- Restoran kartasidagi ikkita ko'rsatkich (mijoz super-app bosh sahifasi).
--
-- HAMMASI 0 = "ma'lumot yo'q" degani, "yomon" emas. Mijoz tomonida 0
-- bo'lganda chip UMUMAN chizilmaydi — soxta "0.0 ★" yoki "0 daqiqa"
-- ko'rsatilmaydi.
--
-- REYTING HOZIRCHA QO'LDA kiritiladi (admin panel). Haqiqiy baholash
-- tizimi qurilganda (ROADMAP 3-band) shu ustunlar buyurtmalardan
-- hisoblanadigan qiymat bilan to'ldiriladi va qo'lda kiritish olib
-- tashlanadi — shuning uchun ustun nomlari o'sha kelajakka ham mos.
ALTER TABLE restaurants
    ADD COLUMN IF NOT EXISTS rating DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS rating_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS eta_min_minutes INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS eta_max_minutes INTEGER NOT NULL DEFAULT 0;

-- Ma'noga zid qiymatlar bazaga umuman tushmasin: tekshiruv faqat
-- handler'da bo'lsa, boshqa yo'l (migratsiya, qo'lda SQL, kelajakdagi
-- import) uni chetlab o'tardi.
ALTER TABLE restaurants
    ADD CONSTRAINT restaurants_rating_range CHECK (rating >= 0 AND rating <= 5),
    ADD CONSTRAINT restaurants_rating_count_positive CHECK (rating_count >= 0),
    ADD CONSTRAINT restaurants_eta_positive
        CHECK (eta_min_minutes >= 0 AND eta_max_minutes >= 0),
    ADD CONSTRAINT restaurants_eta_order
        CHECK (eta_max_minutes >= eta_min_minutes);
