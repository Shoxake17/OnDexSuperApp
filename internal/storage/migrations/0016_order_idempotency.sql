-- Buyurtma yaratishda idempotentlik: mijoz ilovasi har buyurtma urinishida
-- bir martalik tasodifiy kalit yuboradi (customer_app/lib/api.dart
-- newIdempotencyKey()). Muammo: avval bunday kalit umuman yo'q edi —
-- tarmoq uzilib, javob kelmay qolgan holatda foydalanuvchi qayta bossa,
-- ikkita bir xil buyurtma yaratilishi mumkin edi.
--
-- Qisman (partial) unique indeks — faqat idempotency_key BERILGAN
-- (bo'sh/NULL EMAS) qatorlarga taalluqli, shuning uchun ko'plab eski/
-- kalitsiz buyurtmalar bir-biri bilan to'qnashmaydi (ularning barchasida
-- idempotency_key bo'sh qator). Indeks nomi ("idx_orders_idempotency")
-- internal/storage/postgres.go PgOrderRepo.Save()da pgconn.PgError.
-- ConstraintName orqali aniq shu satr bilan solishtiriladi — o'zgartirilsa
-- o'sha yerda ham yangilanishi SHART.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS idempotency_key TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_orders_idempotency
	ON orders (customer_id, idempotency_key)
	WHERE idempotency_key <> '';
