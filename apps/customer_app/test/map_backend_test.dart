import 'package:chust_customer/map/map_backend.dart';
import 'package:chust_customer/map/ondex_style.dart';
import 'package:chust_customer/map/pan_map.dart';
import 'package:chust_customer/map/track_map.dart';
import 'package:flutter/material.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' as gm;
import 'package:maplibre_gl/maplibre_gl.dart' as ml;
import 'package:flutter_test/flutter_test.dart';
import 'package:ondex_core/ondex_core.dart';

/// XARITA MANBASI TESTLARI (`GOOGLE_MAP`, serverdan).
///
/// Bayroq endi kompilyatsiya doimiysi emas — `MapConfigService` orqali
/// serverdan keladi, shuning uchun har ikki rejim ham bitta test
/// ishga tushishida tekshiriladi.
void main() {
  const chust = gm.LatLng(41.0030, 71.2360);
  final svc = MapConfigService.instance;

  MapConfig cfg(MapProvider p) => MapConfig(
    provider: p,
    googleAvailable: true,
    ondexApiBase: MapConfig.defaults.ondexApiBase,
    ondexStyleUrl: MapConfig.defaults.ondexStyleUrl,
  );

  setUp(() {
    svc.reportOndexRecovered();
    svc.setConfigForTest(MapConfig.defaults);
  });

  group('manba tanlovi', () {
    test('standart — Google Maps', () {
      // Qiymat hech qachon olinmagan bo'lsa (server javob bermagan)
      // ilova Google'da turadi: bu ilgarigi production xatti-harakati.
      expect(useOndexMapNow, isFalse);
    });

    test('server "ondexmap" desa — OnDexMap', () {
      svc.setConfigForTest(cfg(MapProvider.ondexmap));
      expect(useOndexMapNow, isTrue);
    });

    test('OnDexMap xatosidan keyin — Google, tiklangach yana OnDexMap', () {
      svc.setConfigForTest(cfg(MapProvider.ondexmap));
      svc.reportOndexFailure('address', 'test');
      expect(useOndexMapNow, isFalse);
      svc.reportOndexRecovered();
      expect(useOndexMapNow, isTrue);
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
      expect(
        ondexReverseCovers(kOndexReverseMinLat, kOndexReverseMinLng),
        isTrue,
      );
      expect(
        ondexReverseCovers(kOndexReverseMaxLat, kOndexReverseMaxLng),
        isTrue,
      );
    });

    test('chegaradan bir chetga chiqsa — tashqarida', () {
      expect(ondexReverseCovers(kOndexReverseMaxLat + 0.01, 71.2), isFalse);
      expect(ondexReverseCovers(41.0, kOndexReverseMinLng - 0.01), isFalse);
    });
  });

  group('sun\'iy yo\'ldosh uslubi', () {
    test(
      'OnDexMap proksisi orqali, provayderga to\'g\'ridan-to\'g\'ri emas',
      () {
        // Tile manzili to'g'ridan-to'g'ri provayderga ketsa, kalit
        // ilovaga tushib qolardi — u SERVERDA qolishi kerak.
        expect(ondexSatelliteStyle, contains('maps.ondex.uz/tiles/satellite'));
        expect(ondexSatelliteStyle, isNot(contains('arcgisonline')));
        expect(ondexSatelliteStyle, isNot(contains('googleapis')));
      },
    );

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

  // ┌─ NEGA OnDexMap VIDJETI TEKSHIRILMAYDI ───────────────────────────┐
  // `MapLibreMap` widget testida platform kanalini talab qiladi va
  // daraxt tarqatilayotganda `LateInitializationError: _channel`
  // bilan yiqiladi — bu paketning test muhitidagi cheklovi (nativ xarita
  // oynasi testda mavjud emas). Shuning uchun bu yerda faqat Google
  // yo'li chiziladi; OnDexMap yo'li qurilmada sinaladi.
  //
  // Eng muhim kafolat esa shu yerda tekshiriladi: server "ondexmap"
  // desa-yu OnDexMap yaqinda yiqilgan bo'lsa, ekran MapLibre'ni UMUMAN
  // yaratmay, Google bilan ochiladi.
  // └──────────────────────────────────────────────────────────────────┘
  group('xarita vidjeti manbaga bo\'ysunadi', () {
    Widget pan() => MaterialApp(
      home: SizedBox(
        width: 300,
        height: 300,
        child: PanMap(
          initialCenter: chust,
          initialZoom: 16,
          satellite: false,
          onCameraMove: (_, _) {},
          onCameraIdle: () {},
          onReady: (_) {},
        ),
      ),
    );

    Widget track() => MaterialApp(
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
    );

    testWidgets('PanMap: Google rejimida GoogleMap', (tester) async {
      await tester.pumpWidget(pan());
      expect(find.byType(gm.GoogleMap), findsOneWidget);
      expect(find.byType(ml.MapLibreMap), findsNothing);
    });

    testWidgets('TrackMap: Google rejimida GoogleMap', (tester) async {
      await tester.pumpWidget(track());
      expect(find.byType(gm.GoogleMap), findsOneWidget);
      expect(find.byType(ml.MapLibreMap), findsNothing);
    });

    testWidgets('PanMap: OnDexMap yiqilgan bo\'lsa Google bilan ochiladi', (
      tester,
    ) async {
      svc.setConfigForTest(cfg(MapProvider.ondexmap));
      svc.reportOndexFailure('address', 'test');
      await tester.pumpWidget(pan());
      expect(find.byType(gm.GoogleMap), findsOneWidget);
      expect(find.byType(ml.MapLibreMap), findsNothing);
    });

    testWidgets('TrackMap: OnDexMap yiqilgan bo\'lsa Google bilan ochiladi', (
      tester,
    ) async {
      svc.setConfigForTest(cfg(MapProvider.ondexmap));
      svc.reportOndexFailure('tracking', 'test');
      await tester.pumpWidget(track());
      expect(find.byType(gm.GoogleMap), findsOneWidget);
      expect(find.byType(ml.MapLibreMap), findsNothing);
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
