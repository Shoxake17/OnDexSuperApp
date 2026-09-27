import 'package:chust_customer/map/map_backend.dart';
import 'package:chust_customer/map/ondex_style.dart';
import 'package:chust_customer/map/pan_map.dart';
import 'package:chust_customer/map/track_map.dart';
import 'package:flutter/material.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' as gm;
import 'package:maplibre_gl/maplibre_gl.dart' as ml;
import 'package:flutter_test/flutter_test.dart';

/// XARITA FLAGI TESTLARI.
///
/// ┌─ BU FAYL IKKI MARTA ISHLAYDI ──────────────────────────────────────┐
/// `kUseOnDexMap` — kompilyatsiya vaqtidagi doimiy, uni test ichida
/// o'zgartirib bo'lmaydi. Shuning uchun testlar flagning JORIY
/// qiymatiga qarab kutilgan natijani tekshiradi va to'liq qamrov
/// uchun ikki marta ishga tushiriladi:
///
///   flutter test
///   flutter test --dart-define=ONDEX_MAP=true
///
/// Ikkalasi ham CI'da bajariladi. Faqat birinchisi ishlatilsa,
/// OnDexMap yo'lidagi kompilyatsiya xatosi ham sezilmay qolardi.
/// └────────────────────────────────────────────────────────────────────┘
void main() {
  const chust = gm.LatLng(41.0030, 71.2360);

  group('flag', () {
    test('standart holatda Google Maps ishlaydi', () {
      // Bu tasdiq ataylab qattiq: flagning standart qiymati
      // tasodifan `true` ga o'zgarib ketsa, production build jimgina
      // xarita motorini almashtirardi.
      const fromEnv = bool.fromEnvironment('ONDEX_MAP');
      expect(kUseOnDexMap, fromEnv);
      if (!fromEnv) {
        expect(kUseOnDexMap, isFalse, reason: 'standart — Google Maps');
      }
    });
  });

  group('OnDexMap teskari geokodlash hududi', () {
    test('Chust — hudud ichida', () {
      expect(ondexReverseCovers(41.0030, 71.2360), isTrue);
    });

    test('Toshkent — hududdan TASHQARIDA', () {
      // Bu eng ko'p adashtiradigan holat: reverse butun mamlakatni
      // qamramaydi, Toshkent esa qamrovdan tashqarida.
      expect(ondexReverseCovers(41.2995, 69.2401), isFalse);
    });

    test('chegaraning aynan o\'zi — ichkarida', () {
      expect(ondexReverseCovers(kOndexReverseMinLat, kOndexReverseMinLng), isTrue);
      expect(ondexReverseCovers(kOndexReverseMaxLat, kOndexReverseMaxLng), isTrue);
    });

    test('chegaradan bir chetga chiqsa — tashqarida', () {
      expect(ondexReverseCovers(kOndexReverseMaxLat + 0.01, 71.2), isFalse);
      expect(ondexReverseCovers(41.0, kOndexReverseMinLng - 0.01), isFalse);
    });
  });

  group('sun\'iy yo\'ldosh uslubi', () {
    test('OnDexMap proksisi orqali, provayderga to\'g\'ridan-to\'g\'ri emas', () {
      // Tile manzili to'g'ridan-to'g'ri provayderga ketsa, kalit
      // ilovaga tushib qolardi — u SERVERDA qolishi kerak.
      expect(ondexSatelliteStyle, contains('maps.ondex.uz/tiles/satellite'));
      expect(ondexSatelliteStyle, isNot(contains('arcgisonline')));
      expect(ondexSatelliteStyle, isNot(contains('googleapis')));
    });

    test('kredit bo\'sh emas', () {
      // Atributsiz uslub litsenziya talabini buzardi.
      expect(ondexSatelliteStyle, contains('"attribution"'));
      expect(ondexSatelliteStyle, contains('OnDexMap'));
    });

    test('yaroqli JSON va HTTPS', () {
      expect(ondexSatelliteStyle, contains('"version": 8'));
      expect(ondexSatelliteStyle, isNot(contains('http://')));
    });
  });

  // ┌─ NEGA FAQAT GOOGLE REJIMIDA ─────────────────────────────────────┐
  // `MapLibreMap` widget testida platform kanalini talab qiladi va
  // daraxt tarqatilayotganda `LateInitializationError: _channel`
  // bilan yiqiladi — bu paketning test muhitidagi cheklovi, bizning
  // kodimizniki emas (nativ xarita oynasi testda mavjud emas).
  //
  // OnDexMap yo'lining KOMPILYATSIYASI baribir tekshiriladi: bu fayl
  // `--dart-define=ONDEX_MAP=true` bilan ham ishga tushiriladi va
  // qolgan testlar o'sha kodni import qiladi. Xatti-harakatni esa
  // qurilmada sinash kerak.
  // └──────────────────────────────────────────────────────────────────┘
  group('xarita vidjeti flagga bo\'ysunadi', skip: kUseOnDexMap, () {
    testWidgets('PanMap to\'g\'ri motorni tanlaydi', (tester) async {
      await tester.pumpWidget(MaterialApp(
        home: SizedBox(
          width: 300,
          height: 300,
          child: PanMap(
            initialCenter: chust,
            initialZoom: 16,
            satellite: false,
            onCameraMove: (_, __) {},
            onCameraIdle: () {},
            onReady: (_) {},
          ),
        ),
      ));

      if (kUseOnDexMap) {
        expect(find.byType(ml.MapLibreMap), findsOneWidget);
        expect(find.byType(gm.GoogleMap), findsNothing);
      } else {
        expect(find.byType(gm.GoogleMap), findsOneWidget);
        expect(find.byType(ml.MapLibreMap), findsNothing);
      }
    });

    testWidgets('TrackMap to\'g\'ri motorni tanlaydi', (tester) async {
      await tester.pumpWidget(MaterialApp(
        home: SizedBox(
          width: 300,
          height: 300,
          child: TrackMap(
            initialCenter: chust,
            markers: const [],
            lines: const [],
            onReady: (_) {},
            onUserMoved: () {},
          ),
        ),
      ));

      if (kUseOnDexMap) {
        expect(find.byType(ml.MapLibreMap), findsOneWidget);
        expect(find.byType(gm.GoogleMap), findsNothing);
      } else {
        expect(find.byType(gm.GoogleMap), findsOneWidget);
        expect(find.byType(ml.MapLibreMap), findsNothing);
      }
    });
  });

  group('TrackMarker', () {
    TrackMarker marker({double rotation = 0}) => TrackMarker(
          id: 'courier',
          point: chust,
          image: null,
          sizeDip: 22,
          imageWidthPx: 66,
          rotation: rotation,
        );

    test('burilish o\'zgarsa belgi YANGI hisoblanadi', () {
      // Tenglik noto'g'ri bo'lsa xarita mashinani burmay qo'yardi.
      expect(marker(rotation: 0) == marker(rotation: 90), isFalse);
      expect(marker(rotation: 45) == marker(rotation: 45), isTrue);
    });
  });
}
