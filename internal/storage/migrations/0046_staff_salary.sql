-- Xodimning oylik maoshi (tiyin), ixtiyoriy. "Xodimlar" → oylik hisobot.
ALTER TABLE staff_members
    ADD COLUMN IF NOT EXISTS monthly_salary_tiyin BIGINT
    CHECK (monthly_salary_tiyin IS NULL OR (monthly_salary_tiyin > 0 AND monthly_salary_tiyin <= 100000000000));
