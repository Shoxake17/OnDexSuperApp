import 'package:google_maps_flutter/google_maps_flutter.dart' show LatLng;

import 'route_throttle.dart' show metersBetween;

/// Ovozli yo'l ko'rsatish — QAYSI ibora QACHON aytilishini hal qiladi.
///
/// ┌─ QOIDALAR (2026-09-15) ────────────────────────────────────────────┐
/// Har bir manevr (Google Directions `steps`) uchun kuryer yaqinlashgan sari:
/// 1 km, 500 m, 200 m qolganda va manevr oldidan ("Hozir ...") — har biri
/// BIR MARTA. Uzoqdan boshlanmagan yo'lda eng yaqin bosqich aytiladi (200 m
/// qolganda "1 km dan keyin" deyilmaydi). Manzilga 40 m qolganda —
/// "Siz ... yetib keldingiz" bir marta. Iboralar orasi kamida 4 soniya.
///
/// Marshrut yangilanganda (har 90 s / 300 m) aytilganlar manevr NUQTASI
/// bo'yicha eslab qolinadi — o'sha burilish qayta aytilmaydi.
///
/// Ibora BO'LAKLARDAN iborat (masofa + manevr, ketma-ket chalinadi): Gemini
/// TTS bepul tarifi kuniga 10 so'rov, 46 ta yaxlit ibora o'rniga 17 ta bo'lak
/// (egasining qarori). Kalitlar serverdagi `internal/voice` bilan bir xil:
/// fayl `assets/voice/<kalit>.wav`.
/// └────────────────────────────────────────────────────────────────────┘

enum NavTarget { restaurant, customer }

/// Serverdagi `voice.Maneuvers` bilan bir xil.
const kNavManeuvers = [
  'right', 'left', 'slight_right', 'slight_left', 'sharp_right', 'sharp_left', //
  'uturn', 'roundabout', 'keep_right', 'keep_left', 'straight',
];

/// Serverdagi `voice.Buckets` bilan bir xil (uzoqdan yaqinga).
const kNavBuckets = ['km1', 'm500', 'm200', 'now'];

const kArrivedRestaurantKey = 'arrived_restaurant';
const kArrivedCustomerKey = 'arrived_customer';

String distanceAssetKey(String bucket) => 'dist_$bucket';
String maneuverAssetKey(String maneuver) => 'man_$maneuver';

/// Ilovaga joylangan barcha ovoz bo'laklarining kalitlari.
List<String> allVoiceAssetKeys() => [
      for (final b in kNavBuckets) distanceAssetKey(b),
      for (final m in kNavManeuvers) maneuverAssetKey(m),
      kArrivedRestaurantKey,
      kArrivedCustomerKey,
    ];

class NavStep {
  final String maneuver;
  final LatLng point;
  const NavStep(this.maneuver, this.point);
}

class NavCue {
  /// Ketma-ket chalinadigan bo'laklar: `assets/voice/<kalit>.wav`.
  final List<String> assetKeys;

  /// Restoranga yetib kelindi — restoran nomi bor ibora bo'lsa o'sha aytiladi.
  final bool arrivedAtRestaurant;

  const NavCue(this.assetKeys, {this.arrivedAtRestaurant = false});
}

class VoiceNavigator {
  static const arrivalMeters = 40.0;

  /// Bo'lakli ibora ~3 soniya — keyingisi uni bo'lib yubormasin.
  static const minGap = Duration(seconds: 4);

  /// Bosqich chegaralari (km1, m500, m200, now) — GPS xatosi uchun zaxira bilan.
  static const _levelMeters = [1200.0, 600.0, 260.0, 60.0];

  /// Manevr nuqtasiga shuncha yaqinlashsa — bajarildi.
  static const _passMeters = 25.0;

  List<NavStep> _steps = const [];
  int _next = 0;
  double _closest = double.infinity;
  final List<({LatLng point, int level})> _announced = [];
  LatLng? _destination;
  NavTarget? _target;
  bool _arrived = false;
  DateTime? _lastCueAt;

  void reset() {
    _steps = const [];
    _next = 0;
    _closest = double.infinity;
    _announced.clear();
    _destination = null;
    _target = null;
    _arrived = false;
    _lastCueAt = null;
  }

  /// Yangi (yoki yangilangan) marshrut.
  void setRoute({required List<NavStep> steps, required LatLng destination, required NavTarget target}) {
    final prev = _destination;
    if (_target != target || prev == null || metersBetween(prev, destination) > 30) {
      _arrived = false;
      _announced.clear();
    }
    _destination = destination;
    _target = target;
    _steps = [
      for (final s in steps)
        if (kNavManeuvers.contains(s.maneuver)) s,
    ];
    _next = 0;
    _closest = double.infinity;
  }

  /// Joriy joylashuv bo'yicha aytiladigan ibora yoki `null`.
  NavCue? update(LatLng position, DateTime now) {
    final dest = _destination;
    final target = _target;
    if (dest == null || target == null || _arrived) return null;

    if (metersBetween(position, dest) <= arrivalMeters) {
      _arrived = true;
      _lastCueAt = now;
      return target == NavTarget.restaurant
          ? const NavCue([kArrivedRestaurantKey], arrivedAtRestaurant: true)
          : const NavCue([kArrivedCustomerKey]);
    }

    // Bajarilgan manevrlar: nuqtaga juda yaqinlashdi yoki yaqinlashib,
    // keyin uzoqlasha boshladi (keng chorraha).
    while (_next < _steps.length) {
      final d = metersBetween(position, _steps[_next].point);
      if (d < _closest) _closest = d;
      final passed = d <= _passMeters || (_closest <= _levelMeters.last && d > _closest + 20);
      if (!passed) break;
      _remember(_steps[_next].point, kNavBuckets.length);
      _next++;
      _closest = double.infinity;
    }
    if (_next >= _steps.length) return null;

    final step = _steps[_next];
    final d = metersBetween(position, step.point);
    var level = 0;
    for (var i = 0; i < _levelMeters.length; i++) {
      if (d <= _levelMeters[i]) level = i + 1;
    }
    if (level == 0 || level <= _levelOf(step.point)) return null;
    final last = _lastCueAt;
    if (last != null && now.difference(last) < minGap) return null;
    _remember(step.point, level);
    _lastCueAt = now;
    return NavCue([distanceAssetKey(kNavBuckets[level - 1]), maneuverAssetKey(step.maneuver)]);
  }

  int _levelOf(LatLng point) {
    for (final a in _announced) {
      if (metersBetween(a.point, point) <= 15) return a.level;
    }
    return 0;
  }

  void _remember(LatLng point, int level) {
    for (var i = 0; i < _announced.length; i++) {
      if (metersBetween(_announced[i].point, point) <= 15) {
        if (level > _announced[i].level) _announced[i] = (point: _announced[i].point, level: level);
        return;
      }
    }
    if (_announced.length >= 300) _announced.removeAt(0);
    _announced.add((point: point, level: level));
  }
}
