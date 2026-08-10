// Server-side (Route Handler/Server Component) yordamchisi — Go backend'ga
// server-to-server so'rov yuboradi. Brauzer bu funksiyani hech qachon
// to'g'ridan-to'g'ri chaqirmaydi (faqat Next.js serverida ishlaydi), shuning
// uchun bu chaqiruvlar CORS'ga umuman bog'liq emas.
export const GO_API_URL = process.env.GO_API_URL ?? "http://localhost:8080";

export async function goFetch(
  path: string,
  init: RequestInit = {},
  token?: string | null,
): Promise<Response> {
  const headers = new Headers(init.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);
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

