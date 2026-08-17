import { cookies } from "next/headers";

// Go backend chiqargan JWT shu httpOnly cookie'da saqlanadi — brauzer JS
// unga umuman kira olmaydi (XSS orqali o'g'irlab bo'lmaydi). Bitta domen
// darajasida (path cheklanmagan) — kelajakdagi har qanday mini-app shu
// domen ostida bo'lsa, avtomatik bir xil sessiyadan foydalanadi.
const COOKIE_NAME = "chust_session";
// Go tomonida token muddati 30 kun (users.NewTokenIssuer, cmd/api/main.go) —
// cookie shu bilan mos keladi.
const COOKIE_MAX_AGE = 60 * 60 * 24 * 30;

export async function setSessionToken(token: string) {
  const store = await cookies();
  store.set(COOKIE_NAME, token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: COOKIE_MAX_AGE,
  });
}

export async function getSessionToken(): Promise<string | null> {
  const store = await cookies();
  return store.get(COOKIE_NAME)?.value ?? null;
}

export async function clearSessionToken() {
  const store = await cookies();
  store.delete(COOKIE_NAME);
  store.delete(CLIENT_COOKIE);
}

// ─── Sessiya QAYSI dasturdan ochilgani ──────────────────────────────
//
// Superadmin panelida "mijoz TMA'danmi yoki mobil ilovadanmi" degan
// ustun bor va Go tomon buni `X-Ondex-Client` sarlavhasidan oladi.
// Flutter ilovalari uni o'zi qo'yadi; bu yerda esa so'rovni brauzer
// emas, BFF yuboradi — ya'ni qiymatni BFF bilishi kerak.
//
// NEGA COOKIE (sahifadagi skript aytishi emas): bu cookie `httpOnly`,
// ya'ni uni sahifadagi JS o'qiy ham, yoza ham olmaydi. Qiymat sessiya
// YARATILGAN paytda, kirish yo'liga qarab qo'yiladi:
//   /api/auth/telegram-miniapp -> "tma"   (Telegram Mini App)
//   /api/auth/verify           -> "web"   (oddiy brauzer)
// Ya'ni u sahifa nima deyishidan emas, HAQIQATAN qanday kirilganidan
// kelib chiqadi.
const CLIENT_COOKIE = "chust_client";

/** Ruxsat etilgan qiymatlar — Go tomondagi yopiq ro'yxatga mos. */
export type WebClientKind = "tma" | "web";

export async function setClientKind(kind: WebClientKind) {
  const store = await cookies();
  store.set(CLIENT_COOKIE, kind, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: COOKIE_MAX_AGE,
  });
}

/** Standart — "web": cookie yo'q bo'lsa (eski sessiya) oddiy brauzer. */
export async function getClientKind(): Promise<WebClientKind> {
  const store = await cookies();
  return store.get(CLIENT_COOKIE)?.value === "tma" ? "tma" : "web";
}

// ─── Sahifa MIJOZ ILOVASI ichida ochilganmi ─────────────────────────
//
// ┌─ MUAMMO (tuzatilgan) ─────────────────────────────────────────────┐
// Mijoz ilovasida (Flutter) bosh sahifa aynan shu Next.js mini-app'ini
// WebView orqali ko'rsatadi. Natijada ekranda IKKITA pastki menyu
// bo'lardi: Flutter'niki (native) va sahifaning O'ZINIKI — bir-birining
// ustida, ikkalasi ham ishlaydigan.
//
// Menyu Telegram Mini App'da va oddiy brauzerda KERAK (u yerda boshqa
// navigatsiya yo'q), ilova ichida esa KERAK EMAS.
//
// Belgi `/api/bridge` da qo'yiladi — o'sha manzilga FAQAT Flutter
// WebView murojaat qiladi (native tokenni cookie'ga almashtirish
// uchun). Ya'ni bu "user-agent taxmini" emas, aniq fakt.
// └───────────────────────────────────────────────────────────────────┘
//
// `httpOnly` — sahifadagi skript uni o'zgartira olmaydi; server esa
// HTML render qilishdan OLDIN biladi, ya'ni menyu ko'rinib keyin
// yo'qolmaydi (chaqnash bo'lmaydi).
const SHELL_COOKIE = "chust_shell";

export async function setAppShell() {
  const store = await cookies();
  store.set(SHELL_COOKIE, "app", {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: COOKIE_MAX_AGE,
  });
}

/** Sahifa Flutter ilovasining WebView'i ichidami. */
export async function isAppShell(): Promise<boolean> {
  const store = await cookies();
  return store.get(SHELL_COOKIE)?.value === "app";
}
