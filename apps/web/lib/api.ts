import { headers } from "next/headers";

// Server-side (Route Handler/Server Component) yordamchisi — Go backend'ga
// server-to-server so'rov yuboradi. Brauzer bu funksiyani hech qachon
// to'g'ridan-to'g'ri chaqirmaydi (faqat Next.js serverida ishlaydi), shuning
// uchun bu chaqiruvlar CORS'ga umuman bog'liq emas.
export const GO_API_URL = process.env.GO_API_URL ?? "http://localhost:8080";

// Mini-app versiyasi — superadmin panelidagi "Qurilma" ustunida
// ko'rinadi. Deploy paytida `APP_VERSION` berilmasa bo'sh ketadi va
// server faqat platformani saqlaydi (versiyasiz).
const APP_VERSION = process.env.APP_VERSION ?? "";

export async function goFetch(
  path: string,
  init: RequestInit = {},
  token?: string | null,
  // clientKind — so'rov qaysi mijoz dasturidan boshlangani ("tma" yoki
  // "web"). Brauzer bu yerda Go'ga TO'G'RIDAN-TO'G'RI chiqmaydi, ya'ni
  // sarlavhani BFF qo'yishi kerak; qiymat esa `httpOnly` cookie'dan
  // keladi (`lib/session.ts` dagi izoh).
  clientKind?: string,
): Promise<Response> {
  const outgoing = new Headers(init.headers);
  if (token) outgoing.set("Authorization", `Bearer ${token}`);
  if (clientKind) {
    outgoing.set("X-Ondex-Client", `${clientKind}/${APP_VERSION}`);
  }
  if (init.body && !outgoing.has("Content-Type")) {
    outgoing.set("Content-Type", "application/json");
  }
  await forwardClientIP(outgoing);
  return fetch(`${GO_API_URL}${path}`, {
    ...init,
    headers: outgoing,
    cache: "no-store",
  });
}

// forwardClientIP — HAQIQIY mijoz IP'sini Go'ga uzatadi.
//
// ┌─ TUZATILGAN NOSOZLIK (bug.md 1-band) ──────────────────────────────┐
// BFF Go'ga faqat `Authorization`, `X-Ondex-Client` va `Content-Type`
// yuborardi. Go tomonda esa tezlik cheklovlari IP bo'yicha
// hisoblanadi — natijada BARCHA brauzer foydalanuvchilari Next.js
// konteynerining BITTA IP'si sifatida ko'rinardi.
//
// `otpIPLimiter` ning portlash chegarasi — 5. Ya'ni web orqali 5 kishi
// kirsa, 6-chisi ~5 daqiqaga bloklanardi. Mobil ilovalar Go'ga
// to'g'ridan-to'g'ri chiqqani uchun ta'sirlanmasdi — xato faqat web
// foydalanuvchilarida ko'rinardi.
//
// NEGA BU XAVFSIZ: Go `X-Forwarded-For` ga FAQAT `TRUSTED_PROXIES`
// ro'yxatidagi manbadan kelganda ishonadi (compose'da bu docker
// tarmog'i — `172.16.0.0/12`, ya'ni aynan shu konteyner) va
// zanjirni O'NGDAN CHAPGA yurib, ishonchsiz birinchi manzilni oladi
// (`internal/httpapi/middleware.go`). Ya'ni biz zanjirni shunchaki
// UZATAMIZ, o'zimizdan qiymat qo'shmaymiz — soxtalashtirish uchun
// yangi yuza ochilmaydi.
//
// `publicFetch` ATAYLAB tegilmadi: u ISR keshidan foydalanadi va
// so'rovga xos sarlavha kesh kalitini buzardi.
//
// RENDERING: `headers()` marshrutni dinamik render'ga o'tkazadi.
// `goFetch` allaqachon `cache: "no-store"` bilan ishlaydi, ya'ni uni
// chaqiradigan har bir joy shundoq ham dinamik — yangi cheklov
// paydo bo'lmaydi.
// └────────────────────────────────────────────────────────────────────┘
async function forwardClientIP(outgoing: Headers): Promise<void> {
  // Chaqiruvchi allaqachon qo'ygan bo'lsa tegilmaydi.
  if (outgoing.has("X-Forwarded-For")) return;
  try {
    // `headers()` so'rov konteksti tashqarisida (masalan build
    // vaqtidagi statik generatsiya) istisno beradi — o'shanda
    // uzatadigan IP ham yo'q.
    const incoming = await headers();
    const xff = incoming.get("x-forwarded-for");
    if (xff) outgoing.set("X-Forwarded-For", xff);
  } catch {
    // So'rov konteksti yo'q — jimgina o'tkazamiz.
  }
}

// publicFetch — auth talab qilmaydigan, ochiq katalog endpoint'lari uchun
// (GET /restaurants, /categories, /restaurants/{id}/menu). goFetch'dan
// farqli — cache: "no-store" EMAS, Next.js'ning ISR keshlashidan
// foydalanadi (SEO/tezlik uchun muhim), Go'ning o'zidagi Redis 30s
// keshiga mos revalidate muddati bilan.
export async function publicFetch(
  path: string,
  revalidateSeconds: number,
): Promise<Response> {
  return fetch(`${GO_API_URL}${path}`, {
    next: { revalidate: revalidateSeconds },
  });
}

