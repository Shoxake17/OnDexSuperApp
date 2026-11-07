-- 0068: sessiyaga tashkilot (merchant) — access tokendagi `org` shu yerdan.
-- 0066 allaqachon commit qilingan, shuning uchun alohida migratsiya.
ALTER TABLE auth_sessions ADD COLUMN IF NOT EXISTS org_id TEXT NOT NULL DEFAULT '';
