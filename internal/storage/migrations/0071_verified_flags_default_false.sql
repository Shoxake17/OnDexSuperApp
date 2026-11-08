-- bug.md 97-band: xavfsizlik bayroqlarining baza standarti FAIL-CLOSED.
--
-- 0026/0028 ustunlarni `DEFAULT TRUE` bilan qo'shgan edi — mavjud
-- qatorlar tizimdan chiqib qolmasligi uchun. Lekin standart "kimdir
-- ustunni yozishni unutsa" holatiga javob: TRUE bilan yangi akkaunt
-- (yangi repo metodi, import, qo'lda SQL, seed) jimgina "tasdiqlangan"
-- bo'lib yaratilardi. PgUserRepo.Create ikkalasini ham ochiq yozadi,
-- shuning uchun kod xulqi o'zgarmaydi.
--
-- Faqat STANDART o'zgaradi: mavjud qatorlarga tegilmaydi (ALTER COLUMN
-- ... SET DEFAULT jadvalni qayta yozmaydi, qulf qisqa). Qo'shuvchi
-- migratsiya — eski kod bilan ham ishlaydi (rollback xavfsiz).
ALTER TABLE users ALTER COLUMN phone_verified SET DEFAULT FALSE;
ALTER TABLE users ALTER COLUMN email_verified SET DEFAULT FALSE;
