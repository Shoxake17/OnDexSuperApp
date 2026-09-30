import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// ONDEX XARITA EKRANINING IKKI INVARIANTI.
///
/// ┌─ NEGA MANBA MATNI TEKSHIRILADI ────────────────────────────────────┐
/// `OndexMapScreen` ni testda ko'rsatib bo'lmaydi: `MapLibreMap`
/// platforma kanalini talab qiladi va daraxt tarqatilayotganda
/// `LateInitializationError: _channel` bilan yiqiladi — bu paketning
/// test muhitidagi cheklovi (`map_backend_test.dart` dagi o'sha sabab).
/// Ya'ni quyidagi ikki xatoni xatti-harakat orqali umuman ushlab
/// bo'lmaydi.
///
/// Ikkalasi ham QURILMADA o'lchangan (2026-09-29, production 0.2.5) va
/// ikkalasi ham JIM: ilova yiqilmaydi, Dart tomonida xato ham
/// ko'rinmaydi. Shuning uchun qoida manba matnida qulflanadi.
/// └────────────────────────────────────────────────────────────────────┘
void main() {
  final xom = File('lib/screens/ondex_map_screen.dart').readAsStringSync();

  /// IZOHLARSIZ manba — faqat haqiqiy kod.
  ///
  /// ⚠️ Izohlar tashlanmasa tasdiqlar ALDANADI: o'sha faylning
  /// izohlari aynan shu naqshlarni tushuntiradi (masalan
  /// «`myLocationEnabled: true` bilan tug'ilgan xarita...»), ya'ni
  /// taqiqlangan naqsh izohda uchrab testni yiqitardi, ruxsat etilgan
  /// naqsh esa izohda uchrab kod o'chirilgan bo'lsa ham testni
  /// o'tkazib yuborardi.
  final manba = xom
      .split('\n')
      .where((l) => !l.trimLeft().startsWith('//'))
      .join('\n');

  group('ko\'k nuqta', () {
    test('`myLocationEnabled` doimiy `true` bilan BERILMAYDI', () {
      // ┌─ O'LCHANGAN XATO ────────────────────────────────────────────┐
      // `myLocationEnabled: true` bilan tug'ilgan xarita, ruxsat o'sha
      // paytda hali berilmagan bo'lsa, ko'k nuqtani BOSHQA hech qachon
      // chizmaydi:
      //
      //   E MapLibreMapController: missing location permissions
      //
      // `enableLocationComponent()` uslub yuklanganda BIR MARTA
      // chaqiriladi va ruxsatni o'sha yerda tekshiradi; keyinroq
      // berilgan ruxsat `setMyLocationEnabled(true)` ni uyg'otmaydi,
      // chunki u qiymat o'zgarmaganini ko'rib darhol qaytadi.
      // └──────────────────────────────────────────────────────────────┘
      expect(
        RegExp(r'myLocationEnabled\s*:\s*true').hasMatch(manba),
        isFalse,
        reason: 'xarita ruxsat holatidan tug\'ilishi kerak, `true` dan emas',
      );
      expect(
        manba,
        contains('myLocationEnabled: _locPerm'),
        reason: 'ko\'k nuqta `_locPerm` ga bog\'lanishi kerak',
      );
    });

    test('ruxsat so\'ralgach natija xaritaga yetkaziladi', () {
      // `_applyLocPerm` chaqirilmasa `_locPerm` hech qachon `true`
      // bo'lmaydi va nuqta yana chiqmaydi — birinchi tasdiq o'zi
      // yetarli emas.
      expect(manba, contains('_applyLocPerm(perm)'),
          reason: '`_myPosition` natijani `_locPerm` ga o\'tkazishi kerak');
      expect(manba, contains('Geolocator.requestPermission()'));
    });
  });

  group('xarita fonda uyquga ketadi', () {
    // ┌─ O'LCHANGAN XATO ──────────────────────────────────────────────┐
    // MapLibre plitka keshini va GL yuzasini O'ZI bo'shatmaydi:
    // `maplibre_gl` dagi `onTrimMemory(int)` — bo'sh metod. Qurilmada
    // `Graphics` ko'rsatkichi `am send-trim-memory COMPLETE` dan keyin
    // ham 90.7 MB da qoldi, ilova fonda 215 MB ushlab turdi (Google
    // Maps varianti — 100 MB). Xotirasi kam telefonda bu ilovani
    // `lmkd` uchun birinchi nishonga aylantiradi.
    // └────────────────────────────────────────────────────────────────┘
    test('ekran hayot siklini kuzatadi', () {
      expect(manba, contains('with WidgetsBindingObserver'));
      expect(manba, contains('WidgetsBinding.instance.addObserver(this)'));
      expect(manba, contains('WidgetsBinding.instance.removeObserver(this)'),
          reason:
              'kuzatuvchi olib tashlanmasa ekran yopilgach ham chaqiriladi');
      expect(manba, contains('didChangeAppLifecycleState'));
      expect(manba, contains('didHaveMemoryPressure'));
    });

    test('uyquga ketganda kontroller va ob\'ektlar tashlanadi', () {
      final i = manba.indexOf('void _sleepMap()');
      expect(i, greaterThan(-1), reason: '`_sleepMap` yo\'qolgan');
      final tana = manba.substring(i, i + 700);
      expect(tana, contains('_mapAlive = false'));
      // Eski oynadagi belgi va chiziqlar yangi oynada YAROQSIZ — ular
      // tozalanmasa `_redraw()` o'chirishga urinib xato beradi.
      expect(tana, contains('_ctrl = null'));
      expect(tana, contains('_styleReady = false'));
      expect(tana, contains('_symbols.clear()'));
      expect(tana, contains('_routeLine = null'));
    });

    test('vidjet uyqu holatini hurmat qiladi', () {
      final i = manba.indexOf('Widget _buildMap()');
      expect(i, greaterThan(-1));
      final tana = manba.substring(i, i + 400);
      expect(tana, contains('if (!_mapAlive)'),
          reason: 'uyquda `MapLibreMap` daraxtda qolmasligi kerak');
    });

    test('kamera uyqudan keyin o\'z joyiga qaytadi', () {
      // Qattiq `zoom: 13` qaytarilsa foydalanuvchi boshqa joyga
      // tashlab yuborilardi.
      expect(manba, contains('zoom: _zoomLevel'));
      expect(manba, contains('_zoomLevel = pos.zoom'));
    });
  });
}
