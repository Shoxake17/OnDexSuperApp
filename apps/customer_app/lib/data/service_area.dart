import 'dart:math';

/// Xizmat ko'rsatiladigan shahar (bug.md 90, audit 480).
///
/// Ro'yxat `internal/delivery/area.go` va `apps/web/lib/service-area.ts`
/// bilan AYNAN bir xil bo'lishi SHART — `internal/delivery/area_test.go`
/// (TestServiceCitiesMatchMobile) uni qulflaydi. Server baribir
/// tekshiradi; bu yerda mijozni oxirgi qadamda emas, manzil tanlashda
/// ogohlantirish uchun.
class ServiceCity {
  const ServiceCity({
    required this.name,
    required this.lat,
    required this.lng,
    required this.radiusKm,
  });
  final String name;
  final double lat;
  final double lng;
  final double radiusKm;
}

const serviceCities = <ServiceCity>[
  ServiceCity(name: "Chust", lat: 41.0004, lng: 71.2394, radiusKm: 8),
  ServiceCity(name: "Toshkent", lat: 41.3111, lng: 69.2797, radiusKm: 20),
];

/// Ikki nuqta orasidagi masofa (km) — haversine (web bilan bir xil).
double distanceKm(double aLat, double aLng, double bLat, double bLng) {
  const r = 6371.0;
  final dLat = (bLat - aLat) * pi / 180;
  final dLng = (bLng - aLng) * pi / 180;
  final s = pow(sin(dLat / 2), 2) +
      cos(aLat * pi / 180) * cos(bLat * pi / 180) * pow(sin(dLng / 2), 2);
  return 2 * r * asin(sqrt(s));
}

/// Nuqta qaysi xizmat shahriga tushadi (yo'q bo'lsa null).
ServiceCity? cityFor(double lat, double lng) {
  for (final c in serviceCities) {
    if (distanceKm(lat, lng, c.lat, c.lng) <= c.radiusKm) return c;
  }
  return null;
}
