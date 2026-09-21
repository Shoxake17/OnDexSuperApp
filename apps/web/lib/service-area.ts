// Xizmat ko'rsatiladigan hudud — Chust va Toshkent.
//
// Yandex Eats naqshi: xarita butun mamlakat bo'ylab surilishi mumkin,
// lekin tanlangan nuqta xizmat hududidan tashqarida bo'lsa — "bu yerda
// hali ishlamaymiz" deb aytiladi va davom etishga ruxsat berilmaydi.
// Namangan, Qo'qon, Pop va h.k. — hozircha QAMRAB OLINMAGAN.
//
// Yangi shahar qo'shilganda shu ro'yxatga VA serverdagi
// `internal/delivery/area.go` ga bir xil qator qo'shiladi. Ikkalasi
// ajralsa `TestServiceCitiesMatchWeb` (Go) yiqiladi — qiymatlar va
// tartib aynan bir xil, bir qatorda yozilsin.

export type ServiceCity = {
  name: string;
  lat: number;
  lng: number;
  /// Markazdan radius (km). Chust unchalik katta shahar emas, 8 km
  /// shahar va yaqin atrofdagi mahallalarni qamrab oladi; Toshkentda
  /// 20 km — halqa yo'li ichidagi barcha tumanlar.
  radiusKm: number;
};

export const SERVICE_CITIES: ServiceCity[] = [
  { name: "Chust", lat: 41.0004, lng: 71.2394, radiusKm: 8 },
  { name: "Toshkent", lat: 41.3111, lng: 69.2797, radiusKm: 20 },
];

/// Xarita boshlang'ich markazi (saqlangan manzil yo'q bo'lganda) —
/// birinchi shahar.
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
