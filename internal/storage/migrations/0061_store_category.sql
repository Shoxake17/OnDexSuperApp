-- Do'kon akkaunti kategoriyasi (users.role = 'store'): qurt | store | pharmacy.
-- Boshqa rollar uchun bo'sh qoladi. Kategoriya POS (ShoxPos) panelida qaysi
-- sahifalar ochiq ekanligini belgilaydi (OnDex faqat qiymatni saqlaydi va beradi).
ALTER TABLE users ADD COLUMN IF NOT EXISTS store_category TEXT NOT NULL DEFAULT '';

-- Kategoriya joriy etilishidan oldin yaratilgan do'konlar — oddiy "do'kon".
UPDATE users SET store_category = 'store' WHERE role = 'store' AND store_category = '';
