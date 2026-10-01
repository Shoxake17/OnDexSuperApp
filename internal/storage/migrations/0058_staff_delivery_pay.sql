-- Yetkazib beruvchining "buyurtmadan maosh"i (tiyin): har yetkazilgan buyurtma
-- uchun kuryerga beriladigan haq. U MIJOZNING yetkazish haqini ham belgilaydi
-- (internal/staff.DeliveryFee): oylik maoshli kuryer bo'lsa yetkazish bepul.
--
-- NULL = bu usul tanlanmagan. Faqat kuryer uchun va oylik maosh bilan BIRGA
-- bo'lmaydi: kuryer yo oylik oladi, yo buyurtmadan (xizmat qatlamida ham
-- tekshiriladi, bu yerdagi CHECK ikkinchi himoya qatlami).
ALTER TABLE staff_members
    ADD COLUMN IF NOT EXISTS delivery_pay_tiyin BIGINT
    CHECK (delivery_pay_tiyin IS NULL OR (delivery_pay_tiyin > 0 AND delivery_pay_tiyin <= 10000000));

ALTER TABLE staff_members
    DROP CONSTRAINT IF EXISTS staff_members_single_pay_mode;
ALTER TABLE staff_members
    ADD CONSTRAINT staff_members_single_pay_mode
    CHECK (delivery_pay_tiyin IS NULL OR (position = 'courier' AND monthly_salary_tiyin IS NULL));
