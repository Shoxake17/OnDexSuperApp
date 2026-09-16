import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:geolocator/geolocator.dart';

/// Onlayn bo'lishga to'sqinlik qilayotgan joylashuv muammosi.
class LocationProblem {
  final String message;
  final String? actionLabel;
  final Future<bool> Function()? action;
  const LocationProblem(this.message, {this.actionLabel, this.action});
}

/// Onlayn kuryerning joylashuvini FONDA ham serverga yuboradi.
///
/// ┌─ NEGA (kuryer ilovasi auditi, 4-band) ────────────────────────────┐
/// Avval `Timer.periodic` + `getCurrentPosition` ishlatilardi. Android
/// 10+ fondagi ilovaga joylashuv bermaydi: kuryer "Yo'nalish" tugmasi
/// bilan Google Maps'ga o'tishi yoki ekranni qulflashi bilan yuborish
/// to'xtardi. Mijoz xaritasida kuryer AYNAN yo'lda qotib qolardi,
/// dispatch esa 3 daqiqadan keyin uni nomzodlardan chiqarardi.
///
/// Endi joylashuv oqimi foreground service (doimiy bildirishnoma) bilan
/// ishlaydi: "ilova ishlatilayotganda" ruxsati yetarli va ilova fonda
/// ham tirik qoladi (WebSocket takliflari ham shu sababli yetib keladi).
/// └───────────────────────────────────────────────────────────────────┘
class LocationReporter {
  LocationReporter({required this.onPosition, required this.send});

  /// Xaritadagi "Siz" belgisi uchun (yuborish bilan bir xil oraliqda).
  final void Function(double lat, double lng) onPosition;

  /// Serverga yuborish (`POST /couriers/{id}/location`).
  final Future<void> Function(double lat, double lng) send;

  /// Yuborish oralig'i. Server chegarasi sekundiga 2 ta
  /// (`courierLocLimiter`), dispatch eskirish chegarasi 3 daqiqa
  /// (`locationMaxAge`). 10 soniya — mijoz xaritasida kuryer "sakramay"
  /// harakatlanishi uchun (20 s da moped ~150 m sakrardi); Android oqimi
  /// ham shu oraliqda (`intervalDuration`), ya'ni qo'shimcha GPS yuklamasi yo'q.
  static const sendInterval = Duration(seconds: 10);

  /// Bir martalik joylashuv — muddati bilan: GPS signali yo'q joyda
  /// `getCurrentPosition` muddatsiz ABADIY kutishi mumkin edi.
  static const currentSettings = LocationSettings(
    accuracy: LocationAccuracy.high,
    timeLimit: Duration(seconds: 15),
  );

  /// GPS aniqligi shundan yomon nuqta ishlatilmaydi (yuborilmaydi, xaritaga
  /// va ovozli yo'l ko'rsatishga berilmaydi).
  ///
  /// NEGA (2026-09-15, jonli log): telefon bir necha marta ~1,6 km naridagi
  /// AYNAN bir xil (tarmoq/kesh) nuqtani berdi. Server uni "imkonsiz tezlik"
  /// deb rad etdi, keyin shu xato nuqta asos bo'lib, haqiqiy joylashuv ham
  /// rad etila boshladi — dispatch kuryerni ko'rmay qoldi.
  static const maxAccuracyMeters = 100.0;

  /// Aniq nuqta shuncha vaqt kelmasa (bino ichida) — yomoni ham ishlatiladi,
  /// aks holda kuryer joylashuvi butunlay eskirib qolardi.
  static const poorFixFallback = Duration(minutes: 2);

  /// Nuqtani ishlatish kerakmi. `accuracy` 0 — platforma aniqlikni bermadi.
  @visibleForTesting
  static bool acceptFix({required double accuracy, required DateTime now, DateTime? lastGoodAt}) {
    if (accuracy <= maxAccuracyMeters) return true;
    return lastGoodAt == null || now.difference(lastGoodAt) >= poorFixFallback;
  }

  StreamSubscription<Position>? _sub;
  DateTime? _lastSent;
  DateTime? _lastGoodFixAt;
  bool _sending = false;

  /// Yo'l ko'rsatish rejimi (faol buyurtma): GPS har 2 soniyada — burilish
  /// oldidan "Hozir o'ngga buriling" o'z vaqtida aytilishi uchun. Serverga
  /// yuborish baribir [sendInterval] da bir.
  bool _navigation = false;

  bool get isRunning => _sub != null;

  /// Faol buyurtma boshlanganda yoqiladi, tugaganda o'chiriladi.
  Future<void> setNavigationMode(bool on) async {
    if (_navigation == on) return;
    _navigation = on;
    if (_sub == null) return;
    // Oqim yangi oraliq bilan qayta ochiladi.
    final old = _sub;
    _sub = null;
    await old?.cancel();
    await start();
  }

  Future<void> start() async {
    if (_sub != null) return;
    final LocationSettings settings = defaultTargetPlatform == TargetPlatform.android
        ? AndroidSettings(
            accuracy: LocationAccuracy.high,
            intervalDuration: Duration(seconds: _navigation ? 2 : 10),
            distanceFilter: _navigation ? 3 : 0,
            foregroundNotificationConfig: const ForegroundNotificationConfig(
              notificationTitle: 'OnDexGO — onlayn',
              notificationText: 'Buyurtmalar uchun joylashuvingiz yuborilmoqda',
              notificationChannelName: 'Onlayn holat',
              enableWakeLock: true,
              setOngoing: true,
            ),
          )
        : const LocationSettings(accuracy: LocationAccuracy.high, distanceFilter: 20);
    _sub = Geolocator.getPositionStream(locationSettings: settings).listen(
      _onPosition,
      onError: (Object e) => debugPrint('[joylashuv] oqim xatosi: $e'),
    );
  }

  void _onPosition(Position p) {
    final now = DateTime.now();
    if (!acceptFix(accuracy: p.accuracy, now: now, lastGoodAt: _lastGoodFixAt)) return;
    if (p.accuracy <= maxAccuracyMeters) _lastGoodFixAt = now;
    // Xarita va ovozli yo'l ko'rsatish — HAR (yaroqli) nuqtada.
    onPosition(p.latitude, p.longitude);
    if (_sending || (_lastSent != null && now.difference(_lastSent!) < sendInterval)) {
      return;
    }
    _sending = true;
    _lastSent = now;
    send(p.latitude, p.longitude)
        .catchError((Object e) => debugPrint('[joylashuv] yuborilmadi: $e'))
        .whenComplete(() => _sending = false);
  }

  Future<void> stop() async {
    final s = _sub;
    _sub = null;
    _lastSent = null;
    await s?.cancel();
  }

  /// Onlayn bo'lishdan OLDIN: GPS yoqiqmi va ruxsat bormi.
  ///
  /// Avval kuryer joylashuvsiz ham "onlayn" bo'lib qolardi: dispatch uni
  /// ko'rmasdi (joylashuvi noma'lum), kuryer esa taklif nega kelmayotganini
  /// bilmay o'tirardi (kuryer ilovasi auditi, 10-band).
  static Future<LocationProblem?> readinessProblem() async {
    if (!await Geolocator.isLocationServiceEnabled()) {
      return const LocationProblem(
        'Joylashuv xizmati (GPS) o\'chirilgan — onlayn bo\'lish uchun yoqing',
        actionLabel: 'Yoqish',
        action: Geolocator.openLocationSettings,
      );
    }
    var perm = await Geolocator.checkPermission();
    if (perm == LocationPermission.denied) {
      perm = await Geolocator.requestPermission();
    }
    if (perm == LocationPermission.deniedForever) {
      return const LocationProblem(
        'Joylashuvga ruxsat berilmagan — sozlamalardan yoqing',
        actionLabel: 'Sozlamalar',
        action: Geolocator.openAppSettings,
      );
    }
    if (perm == LocationPermission.denied) {
      return const LocationProblem('Onlayn bo\'lish uchun joylashuvga ruxsat kerak');
    }
    return null;
  }
}
