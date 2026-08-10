// apps/customer_app/lib/api.dart'dagi newIdempotencyKey() bilan bir xil:
// 128 bitli kriptografik tasodifiy son, hex shaklida. Chaqiruvchi buni BIR
// MARTA generatsiya qilib, muvaffaqiyatli buyurtmagacha bo'lgan BARCHA
// qayta urinishlarda AYNAN SHU qiymatni qayta ishlatishi kerak.
export function newIdempotencyKey(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}
