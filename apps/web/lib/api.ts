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
  const headers = new Headers(init.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);
  if (clientKind) {
    headers.set("X-Ondex-Client", `${clientKind}/${APP_VERSION}`);
  }
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  return fetch(`${GO_API_URL}${path}`, {
    ...init,
    headers,
    cache: "no-store",
  });
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

