import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import {
  ACCESS_COOKIE,
  CENTRAL_MAX_AGE,
  REFRESH_COOKIE,
  cookieOptions,
  expiresWithin,
  refreshSession,
} from "./lib/central-session";

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

const GO_API_URL = process.env.GO_API_URL ?? "http://localhost:8080";

/**
 * ┌─ CSRF: o'zgartiruvchi API so'rovi faqat O'Z sahifamizdan (AUD-14) ──┐
 * Sessiya cookie'lari `SameSite=lax` — begona SAYT (cross-site) POST'iga
 * qo'shilmaydi. Lekin `*.ondex.uz` ostidagi har qanday subdomen brauzer
 * uchun "o'sha sayt" (same-site): ularning birida XSS bo'lsa, mijoz nomidan
 * `/api/proxy/*` orqali buyurtma, manzil va h.k. o'zgartirib bo'lardi.
 * Shuning uchun GET/HEAD/OPTIONS dan boshqa `/api/*` so'rovlari:
 *   - `Sec-Fetch-Site: cross-site | same-site` — rad;
 *   - `Origin` bor va host boshqa — rad;
 *   - sarlavhalar umuman yo'q (eski native WebView) — o'tadi: brauzer
 *     begona sahifadan POST'da `Origin` ni HAR DOIM yuboradi.
 * └────────────────────────────────────────────────────────────────────┘
 */
function crossOriginWrite(request: NextRequest): boolean {
  const m = request.method;
  if (m === "GET" || m === "HEAD" || m === "OPTIONS") return false;
  if (!request.nextUrl.pathname.startsWith("/api/")) return false;
  const site = request.headers.get("sec-fetch-site");
  if (site === "cross-site" || site === "same-site") return true;
  const origin = request.headers.get("origin");
  if (!origin) return false;
  const host = (request.headers.get("host") ?? request.nextUrl.host).toLowerCase();
  try {
    return new URL(origin).host.toLowerCase() !== host;
  } catch {
    return true; // "null" yoki buzilgan Origin
  }
}

export async function proxy(request: NextRequest) {
  if (crossOriginWrite(request)) {
    return NextResponse.json({ error: "So'rov manbasi tasdiqlanmadi" }, { status: 403 });
  }
  // Port bilan kelgan host ham to'g'ri tanilsin (`ondex.uz:3000`).
  const host = (request.headers.get("host") ?? "").toLowerCase().split(":")[0];
  const landing = request.nextUrl.pathname === "/" && LANDING_HOSTS.includes(host);

  // ── Markaziy sessiya (137): access eskirayotgan bo'lsa render'dan OLDIN
  // yangilanadi (`lib/central-session.ts` izohiga qarang). Eski (refresh'siz)
  // sessiyaga tegilmaydi.
  const refresh = request.cookies.get(REFRESH_COOKIE)?.value;
  const access = request.cookies.get(ACCESS_COOKIE)?.value;
  let renewed: { access: string; refresh: string } | null = null;
  let rejected = false;
  if (refresh && (!access || expiresWithin(access, 60))) {
    const r = await refreshSession(GO_API_URL, refresh);
    if (r.ok) {
      renewed = { access: r.access, refresh: r.refresh };
      // Joriy so'rovning o'zi ham YANGI token bilan render bo'lsin.
      request.cookies.set(ACCESS_COOKIE, r.access);
      request.cookies.set(REFRESH_COOKIE, r.refresh);
    } else if (r.rejected) {
      rejected = true;
      request.cookies.delete(ACCESS_COOKIE);
      request.cookies.delete(REFRESH_COOKIE);
    }
  }

  const init = { request: { headers: request.headers } };
  const response = landing
    ? NextResponse.rewrite(new URL("/landing", request.url), init)
    : NextResponse.next(init);
  if (renewed) {
    response.cookies.set(ACCESS_COOKIE, renewed.access, cookieOptions(CENTRAL_MAX_AGE));
    response.cookies.set(REFRESH_COOKIE, renewed.refresh, cookieOptions(CENTRAL_MAX_AGE));
  } else if (rejected) {
    // Sessiya markazda yopilgan (chiqish, parol almashtirish, bloklash).
    response.cookies.set(ACCESS_COOKIE, "", cookieOptions(0));
    response.cookies.set(REFRESH_COOKIE, "", cookieOptions(0));
  }
  return response;
}

export const config = {
  // Sahifalar va API marshrutlari; statik fayllar (`_next/static`, rasmlar,
  // fayl kengaytmali hammasi) proxy'dan O'TMAYDI — keraksiz yuk bo'lmasin.
  matcher: ["/((?!_next/static|_next/image|favicon.ico|.*\\..*).*)"],
};
