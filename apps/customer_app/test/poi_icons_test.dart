import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:chust_customer/map/poi_icons.dart';
import 'package:flutter_test/flutter_test.dart';

/// JOY BELGILARINING EKRANDAGI O'LCHAMI.
///
/// ┌─ O'LCHANGAN XATO ──────────────────────────────────────────────────┐
/// Belgilar qurilmada ko'rinmas darajada kichik bo'lib chiqqan edi.
///
/// Sabab: saytda tuval `PIN_SIZE * 2` piksel chiziladi va MapLibre'ga
/// `addImage(..., { pixelRatio: 2 })` deb aytiladi. Flutter o'ramida
/// `pixelRatio` parametri YO'Q, Android tarafi esa rasmni
/// `inScaled = false`, `inDensity = 0` bilan o'qiydi — zichlik
/// ma'lumoti yo'qoladi va rasm pikseli ekran pikseliga 1:1 tushadi.
/// Ya'ni 20 dip li doira dpr=2.8 li telefonda 20 fizik piksel, atigi
/// ~7 dip bo'lib qolardi.
///
/// Yagona ishonchli o'lchov — PNG ning O'ZIDAGI o'lcham: `addImage` ni
/// testda chaqirib bo'lmaydi (platforma kanali), ekranni ham
/// ko'rsatib bo'lmaydi. Shuning uchun PNG sarlavhasi o'qiladi.
/// └────────────────────────────────────────────────────────────────────┘
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  /// PNG sarlavhasidan (IHDR) kenglik va balandlik.
  ///
  /// Signatura 8 bayt, keyin uzunlik (4) + "IHDR" (4), so'ng kenglik va
  /// balandlik — ikkalasi 4 baytli, katta tartibli.
  ({int w, int h}) pngOlcham(Uint8List bytes) {
    final bd = ByteData.sublistView(bytes);
    expect(bytes.sublist(1, 4), [0x50, 0x4E, 0x47], reason: 'PNG emas');
    return (w: bd.getUint32(16), h: bd.getUint32(20));
  }

  test('doira belgisi `dpr` baravar zich chiziladi', () async {
    final dpr = ui.PlatformDispatcher.instance.implicitView!.devicePixelRatio;
    final kutilgan = (debugBadgeSize * dpr).round();

    final olcham = pngOlcham(await debugBadgePng('bank'));

    expect(olcham.w, kutilgan,
        reason: 'belgi $kutilgan px bo\'lishi kerak, ${olcham.w} px emas '
            '(dip $debugBadgeSize × dpr $dpr)');
    expect(olcham.h, kutilgan);
  });

  test('dip o\'lchami saytdagi `BADGE_SIZE` bilan bir xil', () {
    // Sayt bilan farq qilsa ikki mahsulot boshqa-boshqa ko'rinardi.
    expect(debugBadgeSize, 20);
  });

  group('marshrut A/B belgilari', () {
    // ┌─ SAYTGA MOSLIK ──────────────────────────────────────────────┐
    // Saytda A/B — DOIRA: `useMapTools.ts` dagi `LYR.halo`
    // (`circle-radius: 10`, oq `circle-stroke-width: 2`) va ustidagi
    // `LYR.labels` harfi. Ilovada esa qo'lda chizilgan 24×32 li
    // TOMCHI turardi — ikki mahsulot boshqa-boshqa ko'rinardi.
    //
    // Kvadratlik tekshiruvi aynan shuni ushlaydi: tomchi hech qachon
    // kvadrat bo'lolmaydi (uchi pastga cho'zilgan).
    // └──────────────────────────────────────────────────────────────┘
    for (final slot in ['a', 'b']) {
      test('«${slot.toUpperCase()}» — kvadrat doira, tomchi emas', () async {
        final o = pngOlcham(await debugRouteDotPng(slot));
        expect(o.w, o.h, reason: 'tomchi kvadrat bo\'lmaydi');
      });
    }

    test('o\'lcham saytdagi radius bilan bir xil', () async {
      final dpr = ui.PlatformDispatcher.instance.implicitView!.devicePixelRatio;
      // radius 10 + 1 dip oq halqa, ya'ni diametr 22 dip.
      final o = pngOlcham(await debugRouteDotPng('a'));
      expect(o.w, (22 * dpr).round());
    });
  });

  test('marshrut chizig\'i saytdagi ko\'k', () {
    // Brend to'q sarig'i (`#f97316`) bu yerda XATO edi: saytda
    // marshrut Yandex kabi ko'k chiziladi (`ROUTE_BLUE`).
    expect(kOndexRouteLine, '#2f6bff');
  });

  test('zichlik YO\'QOTILMAGAN — rasm dip o\'lchamida emas', () async {
    // ⚠️ Mutatsiya tuzog'i: `canvas.scale(dpr)` olib tashlansa test
    // yuqorida yiqiladi, lekin dpr=1 li muhitda sezilmay qolardi.
    // Shuning uchun dpr ning o'zi 1 dan katta ekani ham tasdiqlanadi —
    // aks holda bu fayl hech narsani tekshirmayotgan bo'lardi.
    final dpr = ui.PlatformDispatcher.instance.implicitView!.devicePixelRatio;
    expect(dpr, greaterThan(1.0),
        reason: 'test muhitida dpr 1 — bu test kuchsiz, muhitni tekshiring');
    final olcham = pngOlcham(await debugBadgePng('bank'));
    expect(olcham.w, greaterThan(debugBadgeSize.round()));
  });
}
