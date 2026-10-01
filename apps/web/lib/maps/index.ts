// Xarita yaratish: serverdagi `GOOGLE_MAP` bayrog'iga qarab Google yoki
// OnDexMap, OnDexMap ishlamasa — Google'ga qaytish.
//
//   GOOGLE_MAP=true  -> Google (standart)
//   GOOGLE_MAP=false -> OnDexMap; uslub 10 s ichida yuklanmasa yoki xato
//                       bersa Google (agar serverda kalit bor) va keyingi
//                       5 daqiqa Google'da turiladi.
//
// Foydalanuvchi xarita o'rniga xato ko'rmasligi kerak — u buyurtma bermoqda.

import { loadGoogleMaps } from "../gmaps";
import { loadMapConfig, reportMapFallback } from "./config";
import { createGoogleView } from "./google-view";
import { UnauthorizedError } from "./errors";
import type { MapView, MapViewOptions } from "./types";

export type { CircleStyle, LatLng, LineStyle, MapMarker, MapView } from "./types";
export { UnauthorizedError } from "./errors";

/** OnDexMap xatosidan keyin shuncha vaqt Google'da turamiz (mobil ilovalar bilan bir xil). */
export const ONDEX_FAILURE_COOLDOWN_MS = 5 * 60 * 1000;

let ondexFailedAt = 0;

export function ondexRecentlyFailed(now = Date.now()): boolean {
  return ondexFailedAt > 0 && now - ondexFailedAt < ONDEX_FAILURE_COOLDOWN_MS;
}

export async function createMapView(container: HTMLElement, opts: MapViewOptions): Promise<MapView> {
  const cfg = await loadMapConfig();

  if (cfg.provider === "ondexmap" && cfg.ondex && !ondexRecentlyFailed()) {
    try {
      const { createOndexView } = await import("./ondex-view");
      return await createOndexView(container, opts, cfg.ondex);
    } catch (e) {
      if (e instanceof UnauthorizedError) throw e;
      ondexFailedAt = Date.now();
      reportMapFallback(e instanceof Error ? e.message : String(e));
      // Zaxira yo'q bo'lsa foydalanuvchiga xato ko'rsatiladi (chaqiruvchi ushlaydi).
      if (!cfg.googleAvailable) throw e;
    }
  }

  await loadGoogleMaps();
  return createGoogleView(container, opts);
}
