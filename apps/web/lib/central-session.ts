// Markaziy sessiya (Plane 137): BFF tomoni.
//
// ┌─ QANDAY ISHLAYDI ─────────────────────────────────────────────────────┐
// Go kirish endpointlari `X-Ondex-Session: v2` bilan chaqiriladi (`goFetch`)
// va 10 daqiqalik access + refresh qaytaradi. Ikkalasi httpOnly cookie'da:
// brauzer JS ularni ko'rmaydi. Access eskirayotganda `proxy.ts` (render'dan
// OLDIN) refresh qiladi va yangi juftlikni ham joriy so'rovga, ham brauzerga
// yozadi. Server komponentlari cookie YOZA OLMAYDI — shuning uchun refresh
// aynan proxy'da: aks holda aylangan refresh saqlanmay, keyingi so'rov eski
// refresh bilan kelardi va server buni o'g'irlik deb sessiyani yopardi.
//
// Bir vaqtda kelgan so'rovlar (sahifa + API) BITTA refresh'ni kutadi
// (single-flight, shu Node jarayonida).
// └───────────────────────────────────────────────────────────────────────┘

export const ACCESS_COOKIE = "chust_session";
export const REFRESH_COOKIE = "chust_refresh";

/** Markaziy sessiyaning mutlaq umri (Go `ondex-customer`: 180 kun). */
export const CENTRAL_MAX_AGE = 60 * 60 * 24 * 180;
/** Eski (HS256) token umri — 30 kun. */
export const LEGACY_MAX_AGE = 60 * 60 * 24 * 30;

export function cookieDomain(): string | undefined {
  return process.env.NODE_ENV === "production"
    ? process.env.SESSION_COOKIE_DOMAIN?.trim() || undefined
    : undefined;
}

export function cookieOptions(maxAge: number) {
  return {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax" as const,
    path: "/",
    domain: cookieDomain(),
    maxAge,
  };
}

/** JWT `exp` (imzo tekshirilMAYDI — faqat "yangilash vaqti keldimi"). */
export function expiresWithin(token: string, seconds: number): boolean {
  try {
    const part = token.split(".")[1];
    if (!part) return true;
    const json = JSON.parse(Buffer.from(part, "base64url").toString("utf8"));
    if (typeof json.exp !== "number") return true;
    return json.exp * 1000 - Date.now() < seconds * 1000;
  } catch {
    return true;
  }
}

export type RefreshResult =
  | { ok: true; access: string; refresh: string }
  | { ok: false; rejected: boolean };

const inflight = new Map<string, Promise<RefreshResult>>();

/**
 * Refresh — bitta refresh token uchun bir vaqtda BITTA so'rov. `rejected`:
 * server rad etdi (sessiya markazda yopilgan) — cookie'lar tozalanadi;
 * aks holda (tarmoq) sessiya saqlanadi.
 */
export function refreshSession(goApiUrl: string, refresh: string): Promise<RefreshResult> {
  const existing = inflight.get(refresh);
  if (existing) return existing;
  const p = (async (): Promise<RefreshResult> => {
    try {
      const res = await fetch(`${goApiUrl}/v2/auth/token/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refresh_token: refresh }),
        cache: "no-store",
      });
      if (res.status === 401) return { ok: false, rejected: true };
      if (!res.ok) return { ok: false, rejected: false };
      const d = (await res.json()) as { access_token?: string; refresh_token?: string };
      if (!d.access_token || !d.refresh_token) return { ok: false, rejected: false };
      return { ok: true, access: d.access_token, refresh: d.refresh_token };
    } catch {
      return { ok: false, rejected: false };
    }
  })();
  inflight.set(refresh, p);
  // Natija qisqa muddat saqlanadi: shu paytda kelgan parallel so'rovlar
  // ham O'SHA yangi juftlikni oladi (eski refresh qayta yuborilmaydi).
  p.finally(() => setTimeout(() => inflight.delete(refresh), 10_000));
  return p;
}
