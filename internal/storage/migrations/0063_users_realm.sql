-- 0063: akkaunt maydoni (realm) — ADR-002, docs/architecture/AUTH-PHASE-A.md (A1).
--
-- customer — mijozlar; merchant — biznes egalari va xodimlari; platform — OnDex egasi.
--
-- A1 BOSQICHIDA realm ROLDAN hisoblanadi va hech qanday xatti-harakat
-- o'zgarmaydi. Telefon/email/telegram unikalligi HOZIRCHA GLOBAL qoladi:
-- `UNIQUE(realm, phone)` ga o'tish A3 da, kod realmga bog'langan qidiruvga
-- o'tgandan KEYIN qilinadi (aks holda `GetByPhone` bir raqamli ikki akkauntdan
-- tasodifiyini qaytarardi).
--
-- Rol -> realm jadvali `internal/users/realm.go` `RealmForRole` bilan AYNAN
-- bir xil (Postgres testi `realm_pg_test.go` ikkalasini solishtiradi).

CREATE OR REPLACE FUNCTION ondex_realm_for_role(r TEXT) RETURNS TEXT
LANGUAGE sql IMMUTABLE AS $$
  SELECT CASE r
    WHEN 'admin'      THEN 'platform'
    WHEN 'restaurant' THEN 'merchant'
    WHEN 'store'      THEN 'merchant'
    WHEN 'waiter'     THEN 'merchant'
    WHEN 'courier'    THEN 'merchant'
    ELSE 'customer'   -- noma'lum rol: eng kam huquqli maydon (fail closed)
  END
$$;

ALTER TABLE users ADD COLUMN IF NOT EXISTS realm TEXT NOT NULL DEFAULT 'customer';

UPDATE users SET realm = ondex_realm_for_role(role) WHERE realm IS DISTINCT FROM ondex_realm_for_role(role);

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_realm_check;
ALTER TABLE users ADD CONSTRAINT users_realm_check
  CHECK (realm IN ('customer', 'merchant', 'platform'));

-- Realm rol bilan DOIM mos: qaysi kod/skript yozmasin (seed, PromoteToAdmin,
-- qo'lda SQL) — bitta manba shu trigger. A3 da realm roldan mustaqil bo'lganda
-- bu trigger olib tashlanadi.
CREATE OR REPLACE FUNCTION ondex_users_sync_realm() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  NEW.realm := ondex_realm_for_role(NEW.role);
  RETURN NEW;
END
$$;

DROP TRIGGER IF EXISTS trg_users_sync_realm ON users;
CREATE TRIGGER trg_users_sync_realm
  BEFORE INSERT OR UPDATE OF role, realm ON users
  FOR EACH ROW EXECUTE FUNCTION ondex_users_sync_realm();

CREATE INDEX IF NOT EXISTS idx_users_realm ON users (realm);
