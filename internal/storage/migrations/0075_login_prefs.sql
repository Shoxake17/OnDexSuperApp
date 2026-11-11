-- 0075: kirish sozlamalari — passkey asosiy, parol IXTIYORIY (Plane 317).
--
-- Yozuv yo'q = parol bilan kirish O'CHIQ (yangi akkauntlar passkey yoki
-- telefon kodi + Authenticator bilan kiradi). Parolni egasi akkaunt
-- sahifasida (passport) o'zi yoqadi.
--
-- O'tish: hozir PAROL bilan kirayotgan merchant akkauntlari (Windows panel,
-- admin/restoran ilovalari) uzilmasin — ular uchun YOQIQ yozuv qo'yiladi;
-- egasi xohlasa akkaunt sahifasida o'chiradi.
--
-- Alohida jadval: eski kod uni e'tiborsiz qoldiradi — rollback xavfsiz.
CREATE TABLE IF NOT EXISTS merchant_login_prefs (
    user_id         TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_login  BOOLEAN NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO merchant_login_prefs (user_id, password_login)
SELECT id, TRUE FROM users
WHERE realm = 'merchant' AND password_hash <> '' AND deleted_at IS NULL
ON CONFLICT (user_id) DO NOTHING;
