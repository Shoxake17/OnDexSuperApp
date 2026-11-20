// Saytning YAGONA kanonik manzili (bug.md 69): JSON-LD, sitemap va robots
// bir xil domenni e'lon qilsin. Avval JSON-LD da app.chustapp.uz qattiq
// yozilgan, sitemap esa boshqa standart qiymatga tushardi.
export const SITE_URL = (process.env.NEXT_PUBLIC_SITE_URL ?? "https://ondex.uz").replace(/\/+$/, "");

// requestOrigin — so'rov kelgan origin (qayta yo'naltirish manzillari uchun).
//
// bug.md 55c: avval `Host` sarlavhasiga KO'R-KO'RONA ishonilardi — soxta
// `Host` bilan foydalanuvchini begona domenga qaytarish (open redirect)
// mumkin edi. Production'da faqat `ondex.uz` va uning subdomenlari qabul
// qilinadi, boshqasi — kanonik SITE_URL. Dev'da (LAN IP, localhost) avvalgidek.
export function requestOrigin(host: string | null, proto: string | null, fallbackHost: string, fallbackProto: string): string {
  const h = (host ?? fallbackHost).toLowerCase();
  const p = (proto ?? fallbackProto).replace(":", "") === "http" ? "http" : "https";
  if (process.env.NODE_ENV === "production") {
    const bare = h.split(":")[0];
    if (bare !== "ondex.uz" && !bare.endsWith(".ondex.uz")) return SITE_URL;
    return `https://${h}`;
  }
  return `${p}://${h}`;
}
