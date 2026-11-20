import { readFileSync } from "node:fs";
import path from "node:path";
import type { NextConfig } from "next";

/// Veb versiyasi — `package.json` dan (`scripts/version.ps1 bump -Apps web`
/// oshiradi). Landing futeri va profil sahifasida ko'rinadi. Build vaqtida
/// singdiriladi, ya'ni Dockerfile/CI ga alohida o'zgaruvchi shart emas.
const webVersion: string = (() => {
  try {
    const pkg = JSON.parse(readFileSync(path.join(process.cwd(), "package.json"), "utf8"));
    return typeof pkg.version === "string" ? pkg.version : "";
  } catch {
    return "";
  }
})();

// MUHIM (haqiqiy Android qurilmada topilgan HAQIQIY sabab — soatlab
// izlangan "hech qanaqa tugma ishlamayapti" bug'ining ILDIZI): Next.js
// 15+/16 dev-server xavfsizlik uchun standart holatda LAN IP kabi
// "begona" origin'lardan kelgan so'rovlarni _next/static JS chunk'lariga
// BLOKLAYDI ("Blocked cross-origin request to Next.js dev resource").
// Bu React'ning "hydration"ini (butun interaktivlikni — onClick va h.k.)
// butunlay ishga tushirilmay qoldirardi — sahifa VIZUAL to'g'ri
// ko'rinardi (SSR HTML yetib kelgan), lekin HECH BIR tugma ishlamasdi,
// chunki client-side JS umuman yuklanmagan edi. Faqat DEV rejimida
// muhim (production build bunday cheklovga ega emas).
const nextConfig: NextConfig = {
  output: "standalone",
  // Audit 480: `x-powered-by: Next.js` texnologiya va versiya qidirishni
  // osonlashtiradi (hujumchiga bepul razvedka) — foydasi yo'q.
  poweredByHeader: false,
  env: { NEXT_PUBLIC_APP_VERSION: webVersion },
  // `dev-web-ondex.shoxpro.uz` — lokal Cloudflare tunnel
  // (scripts/dev_tunnel.ps1). Busiz tunnel orqali ochilgan sahifa
  // yuqoridagi AYNAN o'sha nosozlikka uchraydi: HTML keladi, tugmalar
  // o'lik. LAN IP endi ro'yxatda kerak emas — tunnel uning o'rnini
  // bosadi va Wi-Fi IP o'zgarishi hech narsani buzmaydi.
  allowedDevOrigins: ["127.0.0.1", "localhost", "dev-web-ondex.shoxpro.uz"],

  headers: securityHeaders,
};

// ┌─ XAVFSIZLIK SARLAVHALARI (2026-08-17) ─────────────────────────────┐
// Bungacha bu ilovada CSP ham, HSTS ham, `nosniff` ham YO'Q edi —
// `middleware.ts` ham, `headers()` ham yozilmagan. Bu qiziq
// nomutanosiblik edi: Go backend o'z HTML sahifalari uchun nonce bilan
// CSP qo'yadi, lekin mijoz HAQIQATAN ishlatadigan sirt himoyasiz turardi.
//
// Ro'yxatdagi har bir tashqi manba kodda ISHLATILGANI uchun turibdi —
// "har ehtimolga qarshi" qo'shilgani yo'q:
//   maps.googleapis.com / maps.gstatic.com  -> lib/gmaps.ts (xarita)
//   telegram.org                            -> app/telegram-auth.tsx (SDK)
// └────────────────────────────────────────────────────────────────────┘
const isProd = process.env.NODE_ENV === "production";


async function securityHeaders() {
  const headers = [
    // Content-Security-Policy endi `proxy.ts` da (har so'rovga nonce — bug.md 6,
    // `lib/csp.ts`). Bu yerda ikkinchi CSP bo'lsa brauzer IKKALASINI qo'llardi.
    // MIME turini taxmin qilish o'chiriladi — yuklangan fayl skript
    // sifatida bajarilib ketmasin.
    { key: "X-Content-Type-Options", value: "nosniff" },
    // Tashqi manzilga o'tilganda joriy yo'l (masalan buyurtma ID'si)
    // `Referer` da chiqib ketmasin.
    { key: "Referrer-Policy", value: "no-referrer" },
    // Ishlatilmaydigan qurilma imkoniyatlari o'chiriladi.
    {
      key: "Permissions-Policy",
      value: "camera=(), microphone=(), payment=(), usb=(), geolocation=(self)",
    },
  ];

  // HSTS FAQAT production'da: dev `http://` orqali ishlaydi va
  // brauzerga "bu domenni doim HTTPS deb bil" deyish lokal ishni
  // buzib qo'yishi mumkin.
  if (isProd) {
    headers.push({
      key: "Strict-Transport-Security",
      value: "max-age=31536000; includeSubDomains",
    });
  }

  return [{ source: "/:path*", headers }];
}

export default nextConfig;
