// Saytning YAGONA kanonik manzili (bug.md 69): JSON-LD, sitemap va robots
// bir xil domenni e'lon qilsin. Avval JSON-LD da app.chustapp.uz qattiq
// yozilgan, sitemap esa boshqa standart qiymatga tushardi.
export const SITE_URL = (process.env.NEXT_PUBLIC_SITE_URL ?? "https://ondex.uz").replace(/\/+$/, "");
