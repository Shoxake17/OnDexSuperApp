-- Restoranning O'Z kuryerlari (2026-09-15).
--
-- `restaurant_id` bo'sh ('') — OnDex platforma kuryeri (o'zi ro'yxatdan
-- o'tgan). To'ldirilgan — restoran "Xodimlar" bo'limida qo'shgan va
-- "OnDex Kuryer" ilovasiga kirish ochgan yetkazib beruvchi
-- (`staff.UserAccounts`).
--
-- Dispatch havuzi QAT'IY ajratilgan (`couriers.Repository.ListAvailableNear`):
-- restoran buyurtmasi faqat shu restoran kuryerlariga taklif qilinadi,
-- restoran kuryeri boshqa restoranning buyurtmasini HECH QACHON ko'rmaydi.
--
-- Restoranlar Mongo'da — tashqi kalit YO'Q (`staff_members` bilan bir xil).
-- Mavjud yozuvlar '' oladi, ya'ni platforma kuryeri bo'lib qoladi.

ALTER TABLE couriers ADD COLUMN IF NOT EXISTS restaurant_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_couriers_restaurant
    ON couriers (restaurant_id) WHERE deleted_at IS NULL;
