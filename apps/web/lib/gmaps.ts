// Google Maps JS API yuklovchi — kalit frontend kodida SAQLANMAYDI,
// server `/config/maps` orqali (auth talab qiladi) BFF proksisi bilan
// olinadi (apps/customer_app/lib/widgets/maps_loader_web.dart bilan bir xil
// g'oya). `__gmapsLoading` — bir nechta xarita komponenti bir vaqtda
// ochilsa ham skript ikki marta yuklanmasligi uchun.

import { loadMapConfig } from "./maps/config";

// Komponentlar UnauthorizedError ni shu yerdan import qiladi.
export { UnauthorizedError } from "./maps/errors";

declare global {
  interface Window {
    google?: typeof google;
    __gmapsLoading?: Promise<void>;
  }
}


export function loadGoogleMaps(): Promise<void> {
  if (typeof window === "undefined") return Promise.reject(new Error("server"));
  if (window.google?.maps) return Promise.resolve();
  if (window.__gmapsLoading) return window.__gmapsLoading;
  window.__gmapsLoading = loadMapConfig()
    .then((cfg) => {
      // `GOOGLE_MAP=false` serverda Google kaliti bo'lmasligi mumkin.
      if (!cfg.googleKey) throw new Error("xarita kaliti olinmadi");
      return cfg.googleKey;
    })
    .then(
      (key) =>
        new Promise<void>((resolve, reject) => {
          const script = document.createElement("script");
          script.src = `https://maps.googleapis.com/maps/api/js?key=${encodeURIComponent(key)}`;
          script.async = true;
          script.onload = () => resolve();
          script.onerror = () => reject(new Error("xarita skripti yuklanmadi"));
          document.head.appendChild(script);
        }),
    );
  return window.__gmapsLoading;
}

// ESLATMA: xarita uslubi ATAYLAB berilmaydi — Google'ning standart OQ
// ko'rinishi ishlatiladi, chunki Flutter'dagi Profil → "Manzillarim"
// ekrani (address_screen.dart) ham `MapType.normal` bilan ishlaydi va
// ikkala xarita bir xil ko'rinishi kerak.
