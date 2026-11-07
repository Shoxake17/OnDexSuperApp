-- 0070: xodim taklifi muddati (A3c, AUTH-PHASE-A.md 5-bo'lim).
--
-- Taklif = `status = 'invited'` a'zolik. Xodim o'z telefoniga kelgan kod bilan
-- BIRINCHI marta kirganda (raqam egaligi isbotlanadi) taklif faollashadi —
-- faqat `invited_at` dan 24 soat o'tmagan bo'lsa. Eskirgan taklifni egasi
-- qayta yuboradi (`invited_at` yangilanadi).

ALTER TABLE memberships ADD COLUMN IF NOT EXISTS invited_at TIMESTAMPTZ;
UPDATE memberships SET invited_at = created_at WHERE status = 'invited' AND invited_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_memberships_user_invited ON memberships (user_id) WHERE status = 'invited';
