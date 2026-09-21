-- =====================================================================
--  PRODUCTION BAZASIDAN DEMO MA'LUMOTNI OLIB TASHLASH
-- =====================================================================
--
--  NEGA KERAK
--  ----------
--  2026-08-12 gacha `SeedDemoUsers`, `SeedDemoCouriers` va
--  `SeedDemoCatalog*` `devMode` bilan himoyalanmagan edi va
--  `DATABASE_URL` mavjud bo'lsa PRODUCTION'da ham ishlardi. Kod endi
--  tuzatilgan (`cmd/api/main.go` — `if devMode`), lekin u faqat YANGI
--  qo'shilishini to'xtatadi. Allaqachon tushib qolgan qatorlar shu
--  skript bilan olib tashlanadi.
--
--  ENG MUHIMI: `u_admin` (+998900000099) — ADMIN roli bilan. Kirish
--  parolsiz, faqat OTP bilan bo'ladi va kod Telegram orqali keladi.
--  O'sha raqamni kimdir Telegramda ro'yxatdan o'tkazsa, to'g'ridan-
--  to'g'ri superadmin bo'lardi.
--
--  OLDIN NIMA QILISH KERAK
--  -----------------------
--  1. O'ZINGIZNI admin qiling, aks holda paneldan chiqib qolasiz:
--       .env ga  BOOTSTRAP_ADMIN_PHONE=+998...  qo'shing va API'ni
--       qayta ishga tushiring (yoki 2-BO'LIMdagi UPDATE ni bajaring).
--  2. ZAXIRA NUSXA oling:
--       docker compose exec -T db pg_dump -U chust chustapp > backup.sql
--
--  QANDAY ISHLATISH
--  ----------------
--  Avval FAQAT 1-BO'LIMni bajaring va natijani o'qing. Keyin
--  3-BO'LIMni tranzaksiya ichida bajarib, COMMIT dan OLDIN
--  hisobotni ko'ring.
--
--  KATALOG BU BAZADA EMAS. 0036 migratsiyasidan boshlab `restaurants`,
--  `products` va `promotions` jadvallari PostgreSQL'da UMUMAN YO'Q —
--  katalog faqat MongoDB'da. Demo restoran (r1) va uning taomlarini
--  tozalash uchun fayl oxiridagi 5-BO'LIMga qarang.
-- =====================================================================


-- Xato bo'lsa DARHOL to'xtaydi. Busiz, masalan `couriers` o'chirilishi
-- tashqi kalit sabab yiqilganda psql keyingi buyruqlarni bajaraverardi
-- va baza yarim tozalangan holatda qolardi.
\set ON_ERROR_STOP on


-- =====================================================================
--  1-BO'LIM — TEKSHIRUV (hech narsani o'zgartirmaydi)
-- =====================================================================

\echo '=== Demo foydalanuvchilar ==='
SELECT id, phone, name, role, entity_id
FROM users
WHERE id IN ('u_admin','u_rest1','u_c1','u_c2','u_c3')
ORDER BY id;

\echo '=== Demo kuryerlar ==='
SELECT id, name, approved, available FROM couriers
WHERE id IN ('c1','c2','c3') ORDER BY id;

-- Demo restoran va mahsulotlari MONGODB'da — 5-BO'LIMga qarang.

-- ┌─ BUGINI DIQQAT BILAN O'QING ────────────────────────────────────────┐
-- Agar quyidagi sonlar 0 DAN KATTA bo'lsa, demo yozuvlarga HAQIQIY
-- buyurtmalar bog'langan. `orders.courier_id` da tashqi kalit bor
-- (kaskadsiz), ya'ni kuryerni o'chirish YIQILADI.
--
-- `orders.restaurant_id` boshqa BAZADAGI katalogga ishora qiladi, ya'ni
-- tashqi kalit texnik jihatdan ham mumkin emas. Restoran Mongo'dan
-- o'chirilsa buyurtmalar "osilib" qoladi (mavjud bo'lmagan id).
-- Shuning uchun buyurtma bo'lsa restoranni O'CHIRMANG — 5-BO'LIMdagi
-- yumshoq yechimni (open: false) ishlating.
-- └─────────────────────────────────────────────────────────────────────┘
\echo '=== BLOKLOVCHI bogliqliklar ==='
SELECT 'demo kuryerga bog''langan buyurtmalar' AS nima, count(*) AS soni
FROM orders WHERE courier_id IN ('c1','c2','c3')
UNION ALL
SELECT 'demo restoranning buyurtmalari', count(*)
FROM orders WHERE restaurant_id = 'r1'
UNION ALL
SELECT 'demo foydalanuvchilarning buyurtmalari', count(*)
FROM orders WHERE customer_id IN ('u_admin','u_rest1','u_c1','u_c2','u_c3')
UNION ALL
SELECT 'demo restoranning stollari', count(*)
FROM restaurant_tables WHERE restaurant_id = 'r1';

\echo '=== Sizda boshqa (haqiqiy) admin bormi? ==='
-- Bu 0 bo'lsa TO'XTANG: `u_admin` ni o'chirsangiz tizimda umuman
-- superadmin qolmaydi va panelga hech kim kira olmaydi.
SELECT count(*) AS haqiqiy_adminlar
FROM users WHERE role = 'admin' AND id <> 'u_admin';


-- =====================================================================
--  2-BO'LIM — O'ZINGIZNI ADMIN QILISH (BOOTSTRAP_ADMIN_PHONE o'rniga)
-- =====================================================================
--  Raqamni O'ZINGIZNIKIGA almashtiring. Foydalanuvchi allaqachon
--  ro'yxatdan o'tgan bo'lishi kerak (ilovadan OTP bilan kirgan).
--
-- UPDATE users SET role = 'admin', entity_id = ''
-- WHERE phone = '+998900000000';   -- <-- O'Z RAQAMINGIZ


-- =====================================================================
--  3-BO'LIM — O'CHIRISH (tranzaksiya ichida)
-- =====================================================================
--  1-BO'LIMdagi bloklovchi sonlar HAMMASI 0 bo'lsa shu bo'limni
--  bajaring. Aks holda 4-BO'LIMga o'ting.

BEGIN;

-- Katalog (restaurants/products/promotions) bu bazada YO'Q — ular
-- MongoDB'da, 5-BO'LIMga qarang. Bu yerda faqat Postgres egalik
-- qiladigan yozuvlar: sevimlilar va stollar katalogga MANTIQIY ishora
-- qiladi (tashqi kalitsiz), shuning uchun tartib muhim emas.
DELETE FROM favorites          WHERE product_id IN ('p1','p2','p3');
DELETE FROM restaurant_tables  WHERE restaurant_id = 'r1';

-- Kuryerlar: `orders.courier_id` tashqi kaliti sabab, bog'langan
-- buyurtma bo'lsa bu qator XATO beradi va butun tranzaksiya
-- to'xtaydi — bu ATAYLAB, jimgina ma'lumot buzilgandan ko'ra yaxshi.
DELETE FROM couriers WHERE id IN ('c1','c2','c3');

-- Foydalanuvchilar: `notifications` va `device_tokens` da
-- ON DELETE CASCADE bor, ular o'zi tozalanadi.
DELETE FROM users WHERE id IN ('u_admin','u_rest1','u_c1','u_c2','u_c3');

-- ── Yakuniy tekshiruv: hammasi 0 bo'lishi kerak ──
\echo '=== COMMIT dan oldingi holat (hammasi 0 bolsin) ==='
SELECT 'qolgan demo user'  AS nima, count(*) AS soni FROM users       WHERE id IN ('u_admin','u_rest1','u_c1','u_c2','u_c3')
UNION ALL
SELECT 'qolgan demo kuryer', count(*) FROM couriers    WHERE id IN ('c1','c2','c3')
UNION ALL
SELECT 'qolgan demo stol',   count(*) FROM restaurant_tables WHERE restaurant_id = 'r1'
UNION ALL
SELECT 'qolgan demo sevimli', count(*) FROM favorites  WHERE product_id IN ('p1','p2','p3');

\echo '=== Admin qoldimi? (0 BOLMASIN!) ==='
SELECT count(*) AS adminlar FROM users WHERE role = 'admin';

-- ┌─ FAIL-SAFE: BU YERDA ROLLBACK TURADI ──────────────────────────────┐
-- Fayl `COMMIT` siz tugasa xavfsiz deb o'ylash XATO: `psql` normal
-- chiqishda ochiq tranzaksiyani COMMIT qiladi. Ya'ni `psql -f` bilan
-- ishga tushirilsa o'chirish jimgina yozilib ketardi.
--
-- Shuning uchun standart holat — ROLLBACK. Skriptni bemalol
-- ishlatib, yuqoridagi hisobotni ko'ring: hech narsa o'zgarmaydi.
-- Natija to'g'ri bo'lsa, pastdagi ROLLBACK ni COMMIT ga almashtirib
-- QAYTA ishga tushiring.
-- └─────────────────────────────────────────────────────────────────────┘
ROLLBACK;
-- COMMIT;   <-- ishonch hosil qilgach yuqoridagi ROLLBACK o'rniga shu


-- =====================================================================
--  4-BO'LIM — BUYURTMALAR BOG'LANGAN BO'LSA (yumshoq yechim)
-- =====================================================================
--  Buyurtma tarixini yo'qotmaslik uchun o'chirmasdan NEYTRALLASH.
--  Xavfsizlik nuqtai nazaridan asosiysi — `u_admin` dan admin rolini
--  olib tashlash.
--
-- BEGIN;
-- -- Superadmin huquqini olib tashlaymiz (akkaunt qoladi, lekin
-- -- hech qanday imtiyozi bo'lmaydi).
-- UPDATE users SET role = 'customer', entity_id = '' WHERE id = 'u_admin';
-- -- Demo restoran akkaunti ham o'z restoraniga kira olmasin.
-- UPDATE users SET role = 'customer', entity_id = '' WHERE id = 'u_rest1';
-- -- Demo kuryerlar dispatch nomzodi bo'lmasin.
-- UPDATE couriers SET approved = FALSE, available = FALSE
--  WHERE id IN ('c1','c2','c3');
-- COMMIT;
--
--  Demo restoranni mijozlarga ko'rsatmaslik — Mongo'da, 5-BO'LIMda.


-- =====================================================================
--  5-BO'LIM — MONGODB (KATALOG SHU YERDA)
-- =====================================================================
--  Katalog — restoranlar, menyu va aksiyalar — FAQAT MongoDB'da
--  (0036 migratsiyasidan keyin PostgreSQL'da bu jadvallar yo'q).
--
--  DIQQAT: `r1` demo ID'si HAQIQIY restoran tomonidan qayta
--  ishlatilgan bo'lishi mumkin. Bu mashinada `r1` = "Avigo" — ya'ni
--  ID'ga qarab emas, NOMGA qarab tekshiring, aks holda haqiqiy
--  restoranni o'chirib yuborasiz.
--
--  Lokal:  docker compose exec -T mongo mongosh chustapp
--  Prod:   docker compose exec -T mongo mongosh -u "$MONGO_USER" \
--            -p "$MONGO_PASSWORD" --authenticationDatabase admin chustapp
--
--  1) Avval KO'RING:
--
--    db.restaurants.find({ _id: "r1" }, { name: 1 })
--    db.products.countDocuments({ restaurant_id: "r1" })
--    db.promotions.countDocuments({ restaurant_id: "r1" })
--
--  2) Nomi "Chust Osh Markazi" (demo seed) EKANINI tasdiqlagach:
--
--    db.promotions.deleteMany({ restaurant_id: "r1" })
--    db.products.deleteMany({ restaurant_id: "r1" })
--    db.restaurants.deleteOne({ _id: "r1", name: "Chust Osh Markazi" })
--
--  3) Buyurtma tarixi bog'langan bo'lsa (1-BO'LIMdagi son > 0),
--     o'chirmang — mijozlarga ko'rinmaydigan qiling:
--
--    db.restaurants.updateOne({ _id: "r1" }, { $set: { open: false } })
