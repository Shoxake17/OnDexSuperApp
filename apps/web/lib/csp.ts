// Content-Security-Policy — YAGONA manba (bug.md 6, audit 480).
//
// Avval CSP `next.config.ts` da statik edi va `script-src 'unsafe-inline'`
// talab qilardi: Next.js App Router o'z ishga tushirish skriptlarini inline
// qo'yadi. Endi `proxy.ts` har so'rov uchun yangi NONCE yaratadi va Next.js
// uni o'z skriptlariga avtomatik qo'yadi
// (node_modules/next/dist/docs/01-app/02-guides/content-security-policy.md).
//
// `'strict-dynamic'`: nonce'li skript yuklagan skriptlar (Google Maps,
// Telegram SDK — `next/script` afterInteractive) ham ruxsat oladi, domen
// ro'yxati esa CSP3 brauzerlarida e'tiborsiz qoladi. `https:` va
// `'unsafe-inline'` FAQAT eski (CSP2) brauzerlar uchun zaxira — nonce bor
// joyda zamonaviy brauzer ularni o'zi bekor qiladi. Ya'ni XSS bilan
// qo'shilgan inline skript endi ISHLAMAYDI.

function originFromEnv(value: string | undefined, schemes: string[]): string {
  if (!value) return "";
  try {
    const u = new URL(value);
    return schemes.includes(u.protocol) ? u.origin : "";
  } catch {
    return "";
  }
}

const isProd = process.env.NODE_ENV === "production";
const mediaOrigin = originFromEnv(process.env.NEXT_PUBLIC_MEDIA_ORIGIN, ["https:"]);
const wsOrigin = originFromEnv(process.env.NEXT_PUBLIC_WS_URL, ["wss:", "ws:"]);
const apiOrigin = originFromEnv(process.env.NEXT_PUBLIC_API_URL, ["https:"]);

/** Kriptografik tasodifiy nonce (har so'rov uchun yangi). */
export function newNonce(): string {
  const b = new Uint8Array(18);
  crypto.getRandomValues(b);
  return btoa(String.fromCharCode(...b));
}

export function contentSecurityPolicy(nonce: string): string {
  return [
    "default-src 'self'",
    // `'unsafe-eval'` FAQAT dev'da: Next.js HMR busiz ishlamaydi.
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic' https: 'unsafe-inline'${isProd ? "" : " 'unsafe-eval'"}`,
    // Tailwind va Google Maps inline uslub qo'yadi. Nonce bu yerga
    // ATAYLAB qo'yilmaydi: nonce bo'lsa 'unsafe-inline' bekor bo'lib,
    // Google Maps uslublari buziladi. Uslub skript bajarmaydi.
    "style-src 'self' 'unsafe-inline'",
    // Rasm manbai muhitga qarab o'zgaradi (R2 yoki lokal disk).
    "img-src 'self' data: blob: https:",
    // Xarita, API, media (3D GLB `fetch` orqali — connect-src) va WS.
    `connect-src 'self' https://maps.googleapis.com https://maps.gstatic.com https://*.ondex.uz https://cloudflareinsights.com${
      mediaOrigin ? ` ${mediaOrigin}` : ""
    }${apiOrigin ? ` ${apiOrigin}` : ""}${wsOrigin ? ` ${wsOrigin}` : ""}${isProd ? "" : " ws: wss:"}`,
    "font-src 'self' data:",
    // MapLibre ishchisi `blob:` dan (OnDexMap rejimi).
    "worker-src 'self' blob:",
    // Telegram Web mijozi Mini App'ni IFRAME ichida ochadi.
    "frame-ancestors 'self' https://web.telegram.org https://*.telegram.org",
    "frame-src 'self'",
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ].join("; ");
}
