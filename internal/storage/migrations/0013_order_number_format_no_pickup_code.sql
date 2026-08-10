-- Eski `order_number` (BIGSERIAL, faqat ichki ketma-ketlik manbai) endi
-- `order_seq` deb ataladi — ketma-ketlik generatori (sequence) o'zgarishsiz
-- qoladi, faqat ustun nomi o'zgaradi.
ALTER TABLE orders RENAME COLUMN order_number TO order_seq;

-- Foydalanuvchiga (mijoz/restoran/kuryer) ko'rsatiladigan YANGI raqam:
-- "DDMMYY-0000001" formatida (masalan "300726-0000123") — order_seq
-- asosida INSERT paytida bitta atomik amalda avtomatik hisoblanadi
-- (qo'shimcha so'rov shart emas). Kuryer restoranda buyurtmani olib
-- ketishda shu raqamning OXIRGI 4 xonasini xodimga og'zaki aytadi.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS order_number TEXT;
UPDATE orders SET order_number = to_char(created_at, 'DDMMYY') || '-' || lpad(order_seq::text, 7, '0')
	WHERE order_number IS NULL;
ALTER TABLE orders ALTER COLUMN order_number SET NOT NULL;
ALTER TABLE orders ALTER COLUMN order_number SET DEFAULT (
	to_char(now(), 'DDMMYY') || '-' || lpad(nextval(pg_get_serial_sequence('orders', 'order_seq'))::text, 7, '0')
);

-- Restoran/kuryer o'rtasidagi alohida 4 xonali TASDIQLASH kodi endi shart
-- emas — kuryer buyurtma raqamining oxirgi 4 xonasini og'zaki aytish
-- orqali o'zi to'g'ridan-to'g'ri "oldim" deb belgilaydi (dastur darajasida
-- kod tekshirilmaydi, foydalanuvchi ataylab tanlagan kamroq ishqalanishli
-- yechim).
ALTER TABLE orders DROP COLUMN IF EXISTS pickup_code;
ALTER TABLE orders DROP COLUMN IF EXISTS pickup_code_at;
