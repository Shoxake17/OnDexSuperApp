-- Kuryerlarni YAQINLIK bo'yicha qidirishni DB darajasiga ko'chirish.
--
-- ── MUAMMO ─────────────────────────────────────────────────────────
-- `ListAvailable()` barcha onlayn kuryerlarni MASOFADAN QAT'I NAZAR
-- qaytarardi (dispatch.go izohi: "masofadan qat'i nazar"). Keyin Go
-- tomonda HAR BIRI uchun Google Distance Matrix'ga so'rov ketardi.
--
-- Oqibatlari:
--   * har dispatch = N ta PULLIK API chaqiruvi (N = onlayn kuryerlar);
--   * shahar bo'ylab 200 kuryer bo'lsa, 20 km naridagi kuryer uchun
--     ham ETA hisoblanardi — natija baribir tashlab yuborilardi;
--   * yuk kuryerlar soniga chiziqli o'sardi.
--
-- ── YECHIM ─────────────────────────────────────────────────────────
-- `geography(Point,4326)` ustuni + GiST indeks. Nomzodlar radius
-- bo'yicha DB DARAJASIDA filtrlanadi (`ST_DWithin`), Google'ga esa
-- faqat haqiqiy nomzodlar boradi.

-- ── NEGA GENERATED (hosilaviy) USTUN ───────────────────────────────
-- `location` ni alohida yozish IKKI JOYGA YOZISH demakdir: `lat`/`lng`
-- va `location` bir-biridan ajralib qolishi mumkin (bitta UPDATE
-- unutilsa yoki xato bo'lsa). GENERATED ustun buni PRINTSIPIAL
-- imkonsiz qiladi — u har doim `lat`/`lng` dan hisoblanadi.
--
-- DIQQAT: `ST_MakePoint` argumentlari tartibi — (LNG, LAT), ya'ni
-- X, Y. Bu joyni almashtirish eng ko'p uchraydigan PostGIS xatosi:
-- kod xatosiz ishlaydi, lekin kuryerlar butunlay boshqa joyda
-- ko'rinadi. Tartib `0029_courier_geography_test` bilan qoplangan.
ALTER TABLE couriers
    ADD COLUMN IF NOT EXISTS location geography(Point, 4326)
    GENERATED ALWAYS AS (
        ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography
    ) STORED;

-- ── JOYLASHUV YANGILANGAN VAQTI ────────────────────────────────────
-- `updated_at` YARAMAYDI: u `SetApproved`, `ClaimIfAvailable`,
-- `IncrementCompletedOrders` da ham yangilanadi, ya'ni "joylashuv
-- yangimi?" degan savolga javob bera olmaydi.
--
-- Busiz: ilovasi qotib qolgan yoki tarmoqdan uzilgan kuryer hamon
-- `available` bo'lib turadi va ESKI koordinatasi bilan nomzod
-- bo'laveradi — taklif yuboriladi, javob kelmaydi, buyurtma kechikadi.
ALTER TABLE couriers
    ADD COLUMN IF NOT EXISTS location_updated_at TIMESTAMPTZ;

-- Mavjud yozuvlar uchun: hozirgi `updated_at` ni boshlang'ich deb
-- olamiz. Aks holda migratsiyadan keyin BARCHA kuryer "eskirgan"
-- bo'lib qolardi va dispatch hech kimni topa olmasdi.
UPDATE couriers
   SET location_updated_at = COALESCE(location_updated_at, updated_at, now())
 WHERE location_updated_at IS NULL;

-- ── INDEKSLAR ──────────────────────────────────────────────────────
-- GiST — `ST_DWithin` va `<->` (KNN) uchun. Indekssiz ikkalasi ham
-- to'liq jadval skanerlashga aylanadi va butun optimizatsiya
-- ma'nosiz bo'ladi.
--
-- QISMAN indeks (`WHERE available AND approved`): qidiruv HAR DOIM
-- shu shart bilan bajariladi, shuning uchun indeks faqat kerakli
-- qatorlarni saqlaydi — kichikroq va tezroq.
CREATE INDEX IF NOT EXISTS idx_couriers_location_available
    ON couriers USING GIST (location)
    WHERE available AND approved;

-- Umumiy geografik indeks — admin panel/xarita kabi shartsiz
-- so'rovlar uchun.
CREATE INDEX IF NOT EXISTS idx_couriers_location
    ON couriers USING GIST (location);
