// `GET /config/maps` — xarita manbasi (serverdagi `GOOGLE_MAP`) va kalitlar.
// Auth talab qiladi: BFF proksisi orqali, sessiya cookie'si bilan.

import { UnauthorizedError } from "./errors";
import type { MapProvider } from "./types";

export type MapConfig = {
  provider: MapProvider;
  /** Serverda Google kaliti bor (OnDexMap yiqilsa Google'ga qaytish mumkin). */
  googleAvailable: boolean;
  googleKey: string | null;
  ondex: { apiBase: string; styleUrl: string } | null;
};

declare global {
  interface Window {
    __mapConfigLoading?: Promise<MapConfig>;
  }
}

/** Faqat `https://` manzillar: buzilgan javob ilovani boshqa serverga yo'naltirmasin. */
function httpsUrl(v: unknown): string | null {
  if (typeof v !== `string`) return null;
  try {
    const u = new URL(v);
    return u.protocol === `https:` && u.host ? u.toString().replace(/\/$/, ``) : null;
  } catch {
    return null;
  }
}

/** Server javobini tekshirib o'qiydi: tushunarsiz narsa — Google (eski xatti-harakat). */
export function parseMapConfig(d: unknown): MapConfig {
  const o = (d && typeof d === `object` ? d : {}) as Record<string, unknown>;
  const key = typeof o.maps_api_key === `string` && o.maps_api_key ? o.maps_api_key : null;
  const od = o.ondexmap && typeof o.ondexmap === `object` ? (o.ondexmap as Record<string, unknown>) : null;
  const apiBase = od ? httpsUrl(od.api_base) : null;
  const styleUrl = od ? httpsUrl(od.style_url) : null;
  const ondex = apiBase && styleUrl ? { apiBase, styleUrl } : null;
  return {
    provider: o.provider === `ondexmap` && ondex ? `ondexmap` : `google`,
    googleAvailable: typeof o.google_available === `boolean` ? o.google_available : key !== null,
    googleKey: key,
    ondex,
  };
}

/** Bir nechta xarita komponenti bir vaqtda ochilsa ham so'rov bitta. */
export function loadMapConfig(): Promise<MapConfig> {
  if (typeof window === `undefined`) return Promise.reject(new Error(`server`));
  if (window.__mapConfigLoading) return window.__mapConfigLoading;
  const p = fetch(`/api/proxy/config/maps`)
    .then((r) => {
      if (r.status === 401) throw new UnauthorizedError();
      if (!r.ok) throw new Error(`xarita sozlamasi olinmadi`);
      return r.json();
    })
    .then(parseMapConfig);
  // Xato keshlanmaydi: keyingi ochilishda qayta uriniladi.
  p.catch(() => {
    if (window.__mapConfigLoading === p) window.__mapConfigLoading = undefined;
  });
  window.__mapConfigLoading = p;
  return p;
}

/** OnDexMap xatosi tufayli Google'ga o'tilganini serverga xabar qiladi (log uchun; jim). */
export function reportMapFallback(reason: string): void {
  try {
    void fetch(`/api/proxy/config/maps/fallback`, {
      method: `POST`,
      headers: { [`Content-Type`]: `application/json` },
      body: JSON.stringify({ surface: `web`, reason: reason.slice(0, 160) }),
    }).catch(() => {});
  } catch {
    // xabar ixtiyoriy
  }
}
