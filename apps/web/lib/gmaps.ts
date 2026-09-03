// Google Maps JS API yuklovchi — kalit frontend kodida SAQLANMAYDI,
// server `/config/maps` orqali (auth talab qiladi) BFF proksisi bilan
// olinadi (apps/customer_app/lib/widgets/maps_loader_web.dart bilan bir xil
// g'oya). `__gmapsLoading` — bir nechta xarita komponenti bir vaqtda
// ochilsa ham skript ikki marta yuklanmasligi uchun.

declare global {
  interface Window {
    google?: typeof google;
    __gmapsLoading?: Promise<void>;
  }
}

// Sessiya tugagan/yo'q bo'lganda (`/config/maps` 401) chaqiruvchi buni
// "internet uzildi" emas, "qayta kirish kerak" deb bilishi uchun alohida
// belgi bilan. `requireAuth` (server) buni ODATDA oldindan tutadi —
// bu FAQAT sessiya SAHIFA OCHIQ TURGANDA muddati tugagan holat uchun
// zaxira (masalan token uzoq umr surmagan yoki serverda bekor qilingan).
export class UnauthorizedError extends Error {
  constructor() {
    super("kirish talab qilinadi");
    this.name = "UnauthorizedError";
  }
}

export function loadGoogleMaps(): Promise<void> {
  if (typeof window === "undefined") return Promise.reject(new Error("server"));
  if (window.google?.maps) return Promise.resolve();
  if (window.__gmapsLoading) return window.__gmapsLoading;
  window.__gmapsLoading = fetch("/api/proxy/config/maps")
    .then((r) => {
      if (r.status === 401) throw new UnauthorizedError();
      if (!r.ok) throw new Error("xarita kaliti olinmadi");
      return r.json();
    })
    .then(
      (data) =>
        new Promise<void>((resolve, reject) => {
          const script = document.createElement("script");
          script.src = `https://maps.googleapis.com/maps/api/js?key=${data.maps_api_key}`;
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
