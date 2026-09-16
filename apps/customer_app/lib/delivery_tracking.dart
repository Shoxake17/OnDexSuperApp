import 'dart:math' as math;

import 'package:google_maps_flutter/google_maps_flutter.dart' show LatLng;

// Buyurtma kuzatuvi ma'lumotlari (`GET /orders/{id}/tracking`) va ularni
// tekshiruvchi sof funksiyalar. UI dan ajratilgan — testlarda xaritasiz
// sinaladi.

/// Serverdan kelgan koordinatani tekshiradi. Noto'g'ri qiymat (matn,
/// NaN, chegaradan tashqari, "0,0" — joylashuv hali yuborilmagan)
/// xaritaga tushmaydi.
LatLng? parseGeoPoint(Object? lat, Object? lng) {
  if (lat is! num || lng is! num) return null;
  final a = lat.toDouble();
  final b = lng.toDouble();
  if (!a.isFinite || !b.isFinite) return null;
  if (a < -90 || a > 90 || b < -180 || b > 180) return null;
  if (a == 0 && b == 0) return null;
  return LatLng(a, b);
}

LatLng? _pointFrom(Object? raw) =>
    raw is Map ? parseGeoPoint(raw['lat'], raw['lng']) : null;

/// `GET /orders/{id}` dagi `courier_location`.
LatLng? courierPointFromOrder(Map<String, dynamic> order) =>
    _pointFrom(order['courier_location']);

/// Jonli `courier_location` hodisasi — faqat AYNAN shu buyurtmaniki.
LatLng? courierPointFromEvent(Map<String, dynamic> event, String orderId) {
  if (event['type'] != 'courier_location' || event['order_id'] != orderId) {
    return null;
  }
  return parseGeoPoint(event['lat'], event['lng']);
}

bool _hasCourier(Map<String, dynamic> order) =>
    (order['type'] as String?) != 'dine_in' &&
    ((order['courier_id'] as String?) ?? '').isNotEmpty;

/// Kuryer jonli kuzatiladimi: yetkazish, kuryer biriktirilgan va buyurtma
/// hali yakunlanmagan.
bool courierTrackable(Map<String, dynamic> order) =>
    _hasCourier(order) &&
    const {'accepted', 'preparing', 'ready', 'picked_up'}.contains(order['status']);

/// Kuzatuv xaritasi ko'rsatiladimi. Yetkazilgandan keyin ham — yo'l
/// buyurtmaga biriktirilib qoladi.
bool deliveryMapVisible(Map<String, dynamic> order) =>
    courierTrackable(order) ||
    (_hasCourier(order) && order['status'] == 'delivered');

/// Ikki nuqta orasidagi masofa (metr, haversine).
double metersBetween(LatLng a, LatLng b) {
  const earthRadius = 6371000.0;
  double rad(double d) => d * math.pi / 180;
  final dLat = rad(b.latitude - a.latitude);
  final dLng = rad(b.longitude - a.longitude);
  final h = math.pow(math.sin(dLat / 2), 2) +
      math.cos(rad(a.latitude)) * math.cos(rad(b.latitude)) * math.pow(math.sin(dLng / 2), 2);
  return 2 * earthRadius * math.asin(math.min(1.0, math.sqrt(h)));
}

/// `from` dan `to` ga yo'nalish, gradus (0 — shimol, soat mili bo'yicha).
/// Kuryer mashinasi belgisini harakat yo'nalishiga burish uchun.
double bearingDegrees(LatLng from, LatLng to) {
  double rad(double d) => d * math.pi / 180;
  final lat1 = rad(from.latitude);
  final lat2 = rad(to.latitude);
  final dLng = rad(to.longitude - from.longitude);
  final y = math.sin(dLng) * math.cos(lat2);
  final x = math.cos(lat1) * math.sin(lat2) - math.sin(lat1) * math.cos(lat2) * math.cos(dLng);
  return (math.atan2(y, x) * 180 / math.pi + 360) % 360;
}

enum TrackingPhase { none, waiting, live, delivered }

TrackingPhase _phaseFrom(Object? raw) => switch (raw) {
      'waiting' => TrackingPhase.waiting,
      'live' => TrackingPhase.live,
      'delivered' => TrackingPhase.delivered,
      _ => TrackingPhase.none,
    };

class TrackedRoute {
  final List<LatLng> points;
  final int distanceMeters;
  final int durationSeconds;
  const TrackedRoute(this.points, this.distanceMeters, this.durationSeconds);

  /// Nuqtalar chegarasi — server bilan bir xil (`tracking.MaxRoutePoints`).
  static const maxPoints = 2000;

  static TrackedRoute? fromJson(Object? raw) {
    if (raw is! Map) return null;
    final list = raw['points'];
    if (list is! List) return null;
    final points = <LatLng>[];
    for (final p in list) {
      final point = _pointFrom(p);
      if (point != null) points.add(point);
      if (points.length >= maxPoints) break;
    }
    if (points.length < 2) return null;
    int nonNegative(Object? v) => v is num && v >= 0 ? v.toInt() : 0;
    return TrackedRoute(points, nonNegative(raw['distance_meters']),
        nonNegative(raw['duration_seconds']));
  }
}

/// `GET /orders/{id}/tracking` javobi.
class DeliveryTracking {
  final TrackingPhase phase;

  /// A — restoran, B — mijoz manzili.
  final LatLng? origin;
  final LatLng? destination;
  final LatLng? courier;
  final TrackedRoute? plannedRoute;
  final TrackedRoute? remainingRoute;
  final int? etaSeconds;
  final DateTime? computedAt;
  final DateTime? pickedUpAt;
  final DateTime? deliveredAt;

  /// A nuqta belgisi: restoran nomi va logosi (faqat to'liq javobda keladi).
  final String? restaurantName;
  final String? restaurantLogoUrl;

  const DeliveryTracking({
    required this.phase,
    this.origin,
    this.destination,
    this.courier,
    this.plannedRoute,
    this.remainingRoute,
    this.etaSeconds,
    this.computedAt,
    this.pickedUpAt,
    this.deliveredAt,
    this.restaurantName,
    this.restaurantLogoUrl,
  });

  factory DeliveryTracking.fromJson(Map<String, dynamic> j) {
    DateTime? time(Object? v) =>
        v is String ? DateTime.tryParse(v)?.toLocal() : null;
    String? text(Object? v) {
      final s = v is String ? v.trim() : '';
      return s.isEmpty ? null : s;
    }

    final eta = j['eta_seconds'];
    return DeliveryTracking(
      phase: _phaseFrom(j['phase']),
      origin: _pointFrom(j['origin']),
      destination: _pointFrom(j['destination']),
      courier: _pointFrom(j['courier']),
      plannedRoute: TrackedRoute.fromJson(j['planned_route']),
      remainingRoute: TrackedRoute.fromJson(j['remaining_route']),
      etaSeconds: eta is num && eta >= 0 ? eta.toInt() : null,
      computedAt: time(j['computed_at']),
      pickedUpAt: time(j['picked_up_at']),
      deliveredAt: time(j['delivered_at']),
      restaurantName: text(j['restaurant_name']),
      restaurantLogoUrl: text(j['restaurant_logo_url']),
    );
  }

  /// O'zgarmaydigan qismlar (A→B yo'li, restoran nomi/logosi) oldingi
  /// javobdan olinadi: keyingi so'rovlar ularni qayta yuklamaydi
  /// (`?planned=0`).
  DeliveryTracking keepStaticFrom(DeliveryTracking? previous) {
    if (previous == null) return this;
    return DeliveryTracking(
      phase: phase,
      origin: origin,
      destination: destination,
      courier: courier,
      plannedRoute: plannedRoute ?? previous.plannedRoute,
      remainingRoute: remainingRoute,
      etaSeconds: etaSeconds,
      computedAt: computedAt,
      pickedUpAt: pickedUpAt,
      deliveredAt: deliveredAt,
      restaurantName: restaurantName ?? previous.restaurantName,
      restaurantLogoUrl: restaurantLogoUrl ?? previous.restaurantLogoUrl,
    );
  }

  /// Hozirgi paytda qolgan soniya: server hisoblaganidan beri o'tgan vaqt
  /// ayiriladi (ekran har bir necha soniyada qayta so'ramasdan ham to'g'ri
  /// ko'rsatadi). Ma'lumot yo'q bo'lsa `null`.
  int? remainingSeconds(DateTime now) {
    final eta = etaSeconds;
    final at = computedAt;
    if (eta == null || at == null) return null;
    final left = eta - now.difference(at).inSeconds;
    return left < 0 ? 0 : left;
  }
}

/// "~7 daqiqada" kabi qolgan vaqt.
String etaText(int seconds) {
  if (seconds <= 60) return '1 daqiqadan kam';
  return '~${(seconds / 60).ceil()} daqiqa';
}

String distanceText(int meters) => meters < 1000
    ? '$meters m'
    : '${(meters / 1000).toStringAsFixed(1)} km';

/// Taom olingandan yetkazilguncha ketgan vaqt ("12 daqiqada").
String? deliveryDurationText(DateTime? pickedUp, DateTime? delivered) {
  if (pickedUp == null || delivered == null) return null;
  final d = delivered.difference(pickedUp);
  if (d.isNegative) return null;
  final m = d.inMinutes;
  return m < 1 ? '1 daqiqadan kamda' : '$m daqiqada';
}

String clockText(DateTime t) =>
    '${t.hour.toString().padLeft(2, '0')}:${t.minute.toString().padLeft(2, '0')}';
