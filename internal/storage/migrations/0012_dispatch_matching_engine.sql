-- Yandex Eats uslubidagi avtomatlashtirilgan ETA-asoslangan dispatch
-- matching engine uchun: kuryerga transport turi/reyting/tajriba, buyurtmaga
-- tayyorlash vaqti maydonlari qo'shiladi.

ALTER TABLE couriers ADD COLUMN IF NOT EXISTS vehicle_type TEXT NOT NULL DEFAULT 'moped';
ALTER TABLE couriers ADD COLUMN IF NOT EXISTS rating DOUBLE PRECISION NOT NULL DEFAULT 5.0;
ALTER TABLE couriers ADD COLUMN IF NOT EXISTS completed_orders INTEGER NOT NULL DEFAULT 0;

ALTER TABLE orders ADD COLUMN IF NOT EXISTS preparation_minutes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS ready_at TIMESTAMPTZ;
