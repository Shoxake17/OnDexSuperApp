import type { NextConfig } from "next";

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

/// Media (rasm/3D model) ombori domeni — R2 ning ochiq manzili.
/// Faqat origin qismi olinadi va faqat https qabul qilinadi: CSP ga
/// yo'l yoki noto'g'ri sxema tushib qolmasin.
const mediaOrigin = (() => {
  const raw = (process.env.NEXT_PUBLIC_MEDIA_ORIGIN ?? "").trim();
  if (!raw) return "";
  try {
    const u = new URL(raw);
    return u.protocol === "https:" ? u.origin : "";
  } catch {
    return "";
  }
})();

function contentSecurityPolicy(): string {
  return [
    "default-src 'self'",

    // `'unsafe-inline'` — Next.js App Router o'z ishga tushirish
    // skriptlarini inline qo'yadi. Uni nonce bilan almashtirish
    // `middleware.ts` talab qiladi; u alohida qadam sifatida keyin
    // qo'shiladi va SHU QATOR o'shanda toraytiriladi.
    //
    // `'unsafe-eval'` FAQAT dev'da: Next.js HMR busiz ishlamaydi.
    // Production'da u YO'Q.
    `script-src 'self' 'unsafe-inline'${isProd ? "" : " 'unsafe-eval'"} https://telegram.org https://maps.googleapis.com`,

    // Tailwind va Google Maps inline uslub qo'yadi.
    "style-src 'self' 'unsafe-inline'",

    // Rasm manbai muhitga qarab o'zgaradi (R2 obyekt ombori yoki lokal
    // disk — `lib/images.ts`), shuning uchun aniq domen ro'yxati
    // build vaqtida ma'lum emas. `https:` — ataylab: rasm eng kam
    // xavfli resurs turi va u skript bajarmaydi. R2 domeni qat'iy
    // belgilangach shu qator o'sha domenga toraytiriladi.
    "img-src 'self' data: blob: https:",

    // Xarita tile'lari, API so'rovlari va media ombori.
    //
    // ┌─ NEGA MEDIA ORIGIN ALOHIDA ────────────────────────────────┐
    // 3D model (GLB) `<model-viewer>` tomonidan `fetch` orqali
    // olinadi, ya'ni u `connect-src` ga tushadi — `img-src` emas.
    // R2 domeni bu ro'yxatda bo'lmasa brauzer so'rovni BLOKLAYDI va
    // model hech qachon ko'rinmaydi (xato konsolda ham jimgina
    // qoladi).
    //
    // `https:` deb keng ochish ATAYLAB qilinmadi: `img-src` dan
    // farqli o'laroq `connect-src` orqali ma'lumot CHIQARIB
    // yuborish mumkin, ya'ni u ancha xavfliroq. Shuning uchun aniq
    // domen `NEXT_PUBLIC_MEDIA_ORIGIN` orqali beriladi (masalan
    // https://pub-xxxx.r2.dev). Berilmasa — ro'yxat o'zgarmaydi.
    // └────────────────────────────────────────────────────────────┘
    `connect-src 'self' https://maps.googleapis.com https://maps.gstatic.com${
      mediaOrigin ? ` ${mediaOrigin}` : ""
    }${isProd ? "" : " ws: wss:"}`,

    "font-src 'self' data:",

    // ┌─ TELEGRAM UCHUN MAJBURIY ──────────────────────────────────┐
    // Telegram Web mijozi Mini App'ni IFRAME ichida ochadi. Bu yerda
    // `'none'` yoki `'self'` yozilsa TMA umuman ochilmaydi va xato
    // jimgina bo'ladi — foydalanuvchi bo'sh oyna ko'radi.
    // Mobil Telegram native WebView ishlatadi, unga bu ta'sir qilmaydi.
    // └────────────────────────────────────────────────────────────┘
    "frame-ancestors 'self' https://web.telegram.org https://*.telegram.org",

    // Sahifaning o'zi begona freym ochmaydi.
    "frame-src 'self'",

    // Plagin/obyekt umuman kerak emas.
    "object-src 'none'",

    // Nisbiy havolalarning bazasini o'zgartirib bo'lmasin va forma
    // begona manzilga yuborilmasin.
    "base-uri 'self'",
    "form-action 'self'",
  ].join("; ");
}

async function securityHeaders() {
  const headers = [
    { key: "Content-Security-Policy", value: contentSecurityPolicy() },
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
