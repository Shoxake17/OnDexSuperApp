import 'dart:math' as math;

import 'package:google_maps_flutter/google_maps_flutter.dart' show LatLng;

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

/// Kuryer marshrutini (Google Directions — PULLIK) qachon qayta so'rash.
///
/// ┌─ NEGA (optimizatsiya va o'lchov, 2026-09-15) ──────────────────────┐
/// Avval har GPS nuqtasida (10 s) so'ralardi: 25 daqiqalik yetkazish ~150
/// ta so'rov. Birinchi cheklov `force` bilan chetlab o'tilardi: logdagi
/// o'lchovda 54 soniyada 9 ta so'rov chiqdi — 6 tasi restoran holati
/// ("tayyorlanmoqda", "tayyor") va ilovani qayta ochish sababli, manzil
/// o'zgarmagan bo'lsa ham. Endi `force` YO'Q:
///   * maqsad haqiqatan o'zgarsa (restoran → mijoz) — darhol;
///   * shu maqsadga so'rov ketayotgan bo'lsa — ikkinchisi yuborilmaydi;
///   * so'rovlar orasi kamida [minGap] (xatodan keyin ham);
///   * kuryer [moveMeters] dan ko'p siljisa yoki natija [maxAge] dan eski
///     bo'lsa — qayta.
/// Oraliqda chiziqning bosib o'tilgan boshi telefonda qirqiladi
/// ([trimRouteToPosition]).
/// └────────────────────────────────────────────────────────────────────┘
class RouteThrottle {
  static const minGap = Duration(seconds: 30);
  static const maxAge = Duration(seconds: 90);
  static const moveMeters = 300.0;

  /// Maqsad shundan ko'p siljisa — boshqa maqsad deb hisoblanadi.
  static const destinationChangeMeters = 30.0;

  LatLng? _from;
  LatLng? _destination;
  DateTime? _successAt;
  DateTime? _attemptAt;
  LatLng? _inFlight;

  static bool _same(LatLng a, LatLng b) => metersBetween(a, b) <= destinationChangeMeters;

  bool shouldFetch(LatLng from, LatLng destination, DateTime now) {
    final inFlight = _inFlight;
    if (inFlight != null && _same(inFlight, destination)) return false;
    if (destinationChanged(destination)) return true;
    final attempt = _attemptAt;
    if (attempt != null && now.difference(attempt) < minGap) return false;
    final lastFrom = _from;
    final success = _successAt;
    if (lastFrom == null || _destination == null || success == null) return true;
    if (metersBetween(lastFrom, from) >= moveMeters) return true;
    return now.difference(success) >= maxAge;
  }

  /// Oxirgi muvaffaqiyatli yo'l BOSHQA maqsadga edimi.
  bool destinationChanged(LatLng destination) {
    final last = _destination;
    return last != null && !_same(last, destination);
  }

  /// So'rov yuborildi.
  void begin(LatLng destination, DateTime now) {
    _inFlight = destination;
    _attemptAt = now;
  }

  void succeed(LatLng from, LatLng destination, DateTime now) {
    _inFlight = null;
    _from = from;
    _destination = destination;
    _successAt = now;
  }

  /// So'rov muvaffaqiyatsiz — keyingisi [minGap] dan keyin.
  void fail() => _inFlight = null;

  /// Kutilayotgan javob endi kerak emas (maqsad yo'qoldi). Busiz javob
  /// e'tiborsiz qolganda shu maqsadga boshqa hech qachon so'ralmasdi.
  void abandon() => _inFlight = null;

  /// Buyurtma tugaganda — keyingi buyurtma birinchi nuqtadayoq so'raydi.
  void reset() {
    _from = null;
    _destination = null;
    _successAt = null;
    _attemptAt = null;
    _inFlight = null;
  }
}

/// Kuryer yurgan sari chiziqning bosib o'tilgan boshini qirqadi (so'rovsiz).
///
/// Chiziqning dastlabki [lookAhead] nuqtasi ichidan kuryerga eng yaqini
/// topiladi va undan oldingilari tashlanadi. Kuryer yo'ldan [maxOffRoute]
/// metrdan uzoq bo'lsa (boshqa ko'chaga burildi) chiziq o'zgarmaydi —
/// keyingi haqiqiy hisob uni tuzatadi. O'zgarish bo'lmasa AYNAN o'sha
/// ro'yxat qaytadi.
List<LatLng> trimRouteToPosition(
  List<LatLng> points,
  LatLng position, {
  int lookAhead = 80,
  double maxOffRoute = 150,
}) {
  if (points.length < 2) return points;
  var best = 0;
  var bestDistance = double.infinity;
  final limit = math.min(points.length, lookAhead);
  for (var i = 0; i < limit; i++) {
    final d = metersBetween(points[i], position);
    if (d < bestDistance) {
      bestDistance = d;
      best = i;
    }
  }
  if (bestDistance > maxOffRoute) return points;
  final rest = points.sublist(math.min(best + 1, points.length - 1));
  return [position, ...rest];
}
