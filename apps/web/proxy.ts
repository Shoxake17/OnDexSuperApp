import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

/**
 * Domen bo'yicha marshrutlash: `ondex.uz` — tanishtiruv sahifasi,
 * qolgan domenlar — mini-ilova.
 *
 * ┌─ NEGA IKKI SAHIFA BITTA ILDIZDA ───────────────────────────────────┐
 * `/` allaqachon band: u Telegram Mini App va mijoz ilovasining
 * WebView qobig'i (`app/(food)/page.tsx`). Landing ham ildizda
 * bo'lishi kerak, chunki odam brauzerga `ondex.uz` deb yozadi —
 * `/landing` deb yozmaydi.
 *
 * Ikkalasini bitta yo'lga qo'yib bo'lmaydi, shuning uchun ajratish
 * DOMEN bo'yicha: marketing domenida ildiz `/landing` ga qayta
 * yoziladi, boshqa domenlarda hech narsa o'zgarmaydi.
 *
 * Bu variant tanlandi, chunki:
 *   * mini-ilovaning birorta yo'li o'zgarmaydi (xavf yo'q);
 *   * yangi domen qo'shish uchun kodga tegilmaydi (`LANDING_HOSTS`);
 *   * `redirect` emas, `rewrite`: manzil satrida `ondex.uz` qoladi.
 * └────────────────────────────────────────────────────────────────────┘
 *
 * ┌─ `middleware` EMAS, `proxy` ────────────────────────────────────────┐
 * Next 16 da `middleware` fayl konventsiyasi eskirgan va `proxy` deb
 * qayta nomlangan (`node_modules/next/dist/docs/.../proxy.md`).
 * └────────────────────────────────────────────────────────────────────┘
 */

/** Qaysi domenlarda landing ko'rsatiladi. */
const LANDING_HOSTS = (process.env.LANDING_HOSTS ?? "ondex.uz,www.ondex.uz")
  .split(",")
  .map((h) => h.trim().toLowerCase())
  .filter(Boolean);

export function proxy(request: NextRequest) {
  // Port bilan kelgan host ham to'g'ri tanilsin (`ondex.uz:3000`).
  const host = (request.headers.get("host") ?? "").toLowerCase().split(":")[0];

  if (request.nextUrl.pathname === "/" && LANDING_HOSTS.includes(host)) {
    return NextResponse.rewrite(new URL("/landing", request.url));
  }
  return NextResponse.next();
}

export const config = {
  // FAQAT ildiz tekshiriladi. Butun saytni proxy'dan o'tkazish
  // keraksiz yuk bo'lardi va statik fayllarni ham ushlab qolardi.
  matcher: "/",
};
