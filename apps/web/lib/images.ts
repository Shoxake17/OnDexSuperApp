// apps/customer_app/lib/api.dart'dagi fullImageUrl() bilan bir xil mantiq:
// backend nisbiy yo'l (lokal disk rejimi) yoki mutlaq URL (Cloudflare R2)
// qaytarishi mumkin — bu funksiya ikkalasini ham to'g'ri boshqaradi.
// Server VA client komponentlarda ham xavfsiz ishlatiladi (faqat
// NEXT_PUBLIC_ o'zgaruvchiga bog'liq, boshqa server-only importlar yo'q).
export function fullImageUrl(path?: string | null): string {
  if (!path) return "";
  if (path.startsWith("http://") || path.startsWith("https://")) return path;
  const publicBase =
    process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
  return `${publicBase}${path}`;
}
