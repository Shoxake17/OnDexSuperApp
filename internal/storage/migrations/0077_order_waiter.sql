-- 0077: Stol buyurtmasini ofitsiant OLADI (RS-2, Plane 368).
--
-- Tayyor bo'lgan stol buyurtmasini bitta ofitsiant "Olaman" bilan o'ziga
-- biriktiradi; "Olib bordim" (served) faqat o'sha ofitsiantga (yoki
-- restoranga) ochiq. Ikki ofitsiant bir vaqtda bossa — buyurtmaning
-- versiya qulfi (optimistik) bittasini o'tkazadi.
--
-- waiter_name — biriktirilgan paytdagi NUSXA (xodim keyin qayta nomlansa
-- ham tarix o'zgarmaydi; boshqa ofitsiantlar "Aziz olib bormoqda" ko'radi).
ALTER TABLE orders ADD COLUMN IF NOT EXISTS waiter_id   TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS waiter_name TEXT NOT NULL DEFAULT '';
