-- 0069: telefon unikalligi REALM ichida (A3c, AUTH-PHASE-A.md K3).
--
-- Bitta odam bir vaqtda mijoz (super app) VA biznes xodimi/egasi bo'la
-- oladi: ikkisi ALOHIDA akkaunt (customer va merchant realm), bitta raqam.
-- Avval raqam global unikal edi va xodim qilish mijoz akkauntining rolini
-- almashtirardi (K4) — mijoz tarixi bilan aralashardi.
--
-- ESKI yo'llar o'zgarmaydi: `GetByPhone` (eski kirish, admin yaratishi)
-- bir raqamda ikki akkaunt bo'lsa ENG ESKISINI qaytaradi — 0069 dan oldin
-- hamma raqam unikal bo'lgani uchun mavjud foydalanuvchilar uchun natija
-- aynan avvalgidek. Ikkinchi akkaunt faqat yangi v2 oqimlari (ro'yxatdan
-- o'tish, xodim taklifi) bilan paydo bo'ladi va ular realm bo'yicha qidiradi.
--
-- Yangi rol `merchant` (A3c akkauntlari) -> merchant realm. Jadval
-- `users/realm.go` `RealmForRole` bilan AYNAN bir xil (realm_pg_test.go).

CREATE OR REPLACE FUNCTION ondex_realm_for_role(r TEXT) RETURNS TEXT
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE r
    WHEN 'admin'      THEN 'platform'
    WHEN 'restaurant' THEN 'merchant'
    WHEN 'store'      THEN 'merchant'
    WHEN 'waiter'     THEN 'merchant'
    WHEN 'courier'    THEN 'merchant'
    WHEN 'merchant'   THEN 'merchant'
    ELSE 'customer'   -- noma'lum rol: eng kam huquqli maydon (fail closed)
  END
$$;

-- Global unikallik realm ichidagi unikallikdan qat'iyroq: mavjud ma'lumot
-- yangi indeksga albatta sig'adi.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_realm_phone ON users (realm, phone) WHERE phone <> '';
DROP INDEX IF EXISTS idx_users_phone_unique;
-- eski `GetByPhone` (realmsiz, eng eskisi) uchun
CREATE INDEX IF NOT EXISTS idx_users_phone_created ON users (phone, created_at) WHERE phone <> '';
