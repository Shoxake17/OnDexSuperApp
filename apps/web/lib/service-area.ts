// Xizmat ko'rsatiladigan hudud — HOZIRCHA FAQAT CHUST SHAHRI.
//
// Yandex Eats naqshi: xarita butun mamlakat bo'ylab surilishi mumkin,
// lekin tanlangan nuqta xizmat hududidan tashqarida bo'lsa — "bu yerda
// hali ishlamaymiz" deb aytiladi va davom etishga ruxsat berilmaydi.
// Toshkent, Namangan, Qo'qon, Pop va h.k. — hozircha QAMRAB OLINMAGAN.
//
// Yangi shahar qo'shilganda shu ro'yxatga bitta qator qo'shiladi —
// boshqa hech qayerda o'zgartirish kerak emas.

export type ServiceCity = {
  name: string;
  lat: number;
  lng: number;
  /// Markazdan radius (km). Chust unchalik katta shahar emas, 8 km
  /// shahar va yaqin atrofdagi mahallalarni qamrab oladi.
  radiusKm: number;
};

export const SERVICE_CITIES: ServiceCity[] = [
  { name: "Chust", lat: 41.0004, lng: 71.2394, radiusKm: 8 },
];

/// Xarita boshlang'ich markazi — birinchi (hozircha yagona) shahar.
export const DEFAULT_CENTER = {
  lat: SERVICE_CITIES[0].lat,
  lng: SERVICE_CITIES[0].lng,
};

/// Ikki nuqta orasidagi masofa (km) — standart haversine formulasi.
function distanceKm(
  aLat: number,
  aLng: number,
  bLat: number,
  bLng: number,
): number {
  const R = 6371;
  const dLat = ((bLat - aLat) * Math.PI) / 180;
  const dLng = ((bLng - aLng) * Math.PI) / 180;
  const s =
    Math.sin(dLat / 2) ** 2 +
    Math.cos((aLat * Math.PI) / 180) *
      Math.cos((bLat * Math.PI) / 180) *
      Math.sin(dLng / 2) ** 2;
  return 2 * R * Math.asin(Math.sqrt(s));
}

/// Nuqta xizmat hududida bo'lsa — shahar qaytariladi, aks holda `null`.
export function cityFor(lat: number, lng: number): ServiceCity | null {
  for (const c of SERVICE_CITIES) {
    if (distanceKm(lat, lng, c.lat, c.lng) <= c.radiusKm) return c;
  }
  return null;
}
