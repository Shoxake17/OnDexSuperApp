-- order_number: mijoz/restoran/kuryerga ko'rsatiladigan qisqa, ketma-ket
-- raqam (masalan "#1042") — ichki tasodifiy hex ID o'rniga.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS order_number BIGSERIAL;

-- pickup_code / pickup_code_at: restoran kuryerga og'zaki aytadigan 4 xonali
-- tasdiqlash kodi — kuryer haqiqatan restoranga borganini isbotlash uchun.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS pickup_code TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS pickup_code_at TIMESTAMPTZ;
