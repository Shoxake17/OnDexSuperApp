import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

/// ONDEX XARITA EKRANINING INVARIANTLARI.
///
/// ┌─ NEGA MANBA MATNI TEKSHIRILADI ────────────────────────────────────┐
/// `OndexMapScreen` ni testda ko'rsatib bo'lmaydi: `MapLibreMap`
/// platforma kanalini talab qiladi va daraxt tarqatilayotganda
/// `LateInitializationError: _channel` bilan yiqiladi — bu paketning
/// test muhitidagi cheklovi (`map_backend_test.dart` dagi o'sha sabab).
/// Ya'ni quyidagi xatolarni xatti-harakat orqali umuman ushlab
/// bo'lmaydi.
///
/// Ko'k nuqta xatosi QURILMADA o'lchangan (2026-09-29, production
/// 0.2.5) va u JIM: ilova yiqilmaydi, Dart tomonida xato ham
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

  group('masshtab', () {
    test('sun\'iy yo\'ldosh rejimi almashganda masshtab saqlanadi', () {
      // `key: ValueKey(_satellite)` rejim almashganda xarita vidjetini
      // BUTUNLAY qayta quradi va `initialCameraPosition` qaytadan
      // qo'llanadi. Bu yerda qattiq `zoom: 13` turgan paytda
      // foydalanuvchi markazda qolib, masshtabi sakrab ketardi.
      expect(manba, contains('zoom: _zoomLevel'));
      expect(manba, contains('_zoomLevel = pos.zoom'));
    });
  });
}
