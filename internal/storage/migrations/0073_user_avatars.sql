-- 0073: akkaunt rasmi (avatar) — markazlashgan (Plane ONDEXECOSY-205).
--
-- Profil rasmi ilovalarda alohida saqlanmaydi (ShoxPos Cloudinary'dan
-- voz kechdi): rasm OnDex akkauntiga tegishli, R2'da (ImageStore), barcha
-- ilovalar OIDC userinfo `picture` orqali oladi.
--
-- Alohida jadval: `users` qatoriga va uning repo/scan kodiga tegilmaydi
-- (qo'shuvchi, eski kod e'tiborsiz qoldiradi — rollback xavfsiz).
CREATE TABLE IF NOT EXISTS user_avatars (
    user_id    TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    url        TEXT NOT NULL CHECK (url ~ '^https?://' AND length(url) <= 512),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
