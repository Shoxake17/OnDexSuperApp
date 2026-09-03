import { NextRequest, NextResponse } from "next/server";
import { cookies } from "next/headers";
import { goFetch } from "@/lib/api";
import { COOKIE_DOMAIN } from "@/lib/session";
import { safeRedirect } from "@/lib/safe-redirect";

// POST /api/auth/telegram-login/start {"next":"/checkout"} — "Telegram
// orqali kirish" (SMS ulanmagan paytda web'dagi YAGONA kirish kanali,
// `lib/session.ts`dagi izohga qarang). Go'ning
// `POST /auth/telegram/login/start-web`'ini chaqiradi va kuzatish
// tokenini VAQTINCHALIK, qisqa umrli cookie'ga yozadi.
//
// ┌─ NEGA TOKEN KLIENTGA QAYTARILMAYDI ────────────────────────────────┐
// `token` sahifadagi JS'ga umuman kerak emas — u faqat
// `/api/auth/telegram-return` server tomonida ishlatiladi (bot
// "qaytish" tugmasi orqali kelgan `c` bilan birga). Kamroq narsa
// klientga chiqsa, kamroq narsa XSS orqali o'g'irlanadi.
// └───────────────────────────────────────────────────────────────────┘
//
// ┌─ NEGA COOKIE, HOLAT EMAS ───────────────────────────────────────────┐
// Foydalanuvchi Telegram'ga o'tib qaytganda YANGI navigatsiya bo'ladi —
// sahifadagi React holati (`useState`) yo'qoladi. Cookie esa
// brauzerning o'zida turadi va qaysi TAB/OYNA qaytganidan qat'i nazar
// o'qiladi.
// └───────────────────────────────────────────────────────────────────┘
const COOKIE_NAME = "chust_tg_login";
// `internal/telegram`dagi `pendingTTL` bilan mos (10 daqiqa) — undan
// keyin urinishning o'zi serverda ham amal qilmay qoladi.
const MAX_AGE = 10 * 60;

export async function POST(req: NextRequest) {
  const body = await req.json().catch(() => ({}));
  const next = safeRedirect(typeof body?.next === "string" ? body.next : "/");

  const res = await goFetch("/auth/telegram/login/start-web", { method: "POST" });
  const data = await res.json().catch(() => ({}));
  if (!res.ok || typeof data?.token !== "string" || typeof data?.deep_link !== "string") {
    return NextResponse.json(data, { status: res.status || 502 });
  }

  const store = await cookies();
  store.set(COOKIE_NAME, JSON.stringify({ token: data.token, next }), {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    domain: COOKIE_DOMAIN,
    maxAge: MAX_AGE,
  });

  return NextResponse.json({ deep_link: data.deep_link });
}
