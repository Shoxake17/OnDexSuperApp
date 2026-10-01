import 'package:chust_courier/map/courier_map.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:google_maps_flutter/google_maps_flutter.dart' as gm;
import 'package:maplibre_gl/maplibre_gl.dart' as ml;
import 'package:ondex_core/ondex_core.dart';

/// KURYER XARITASI: manba tanlovi va Google yo'lining o'zgarmasligi.
///
/// `MapLibreMap` widget testida platform kanalini talab qiladi va daraxt
/// tarqatilayotganda yiqiladi (nativ oyna testda yo'q), shuning uchun
/// OnDexMap vidjeti bu yerda chizilmaydi — u qurilmada sinaladi. Muhimi:
/// OnDexMap yiqilgan bo'lsa ekran MapLibre'ni UMUMAN yaratmaydi.
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

  Widget map({
    List<CourierMarker> markers = const [],
    List<gm.LatLng> route = const [],
    bool satellite = false,
  }) =>
      MaterialApp(
        home: SizedBox(
          width: 300,
          height: 300,
          child: CourierMap(
            initialCenter: chust,
            initialZoom: 15,
            satellite: satellite,
            markers: markers,
            route: route,
            routeColor: Colors.orange,
            bottomPadding: 120,
            onCameraMove: (_, _) {},
            onReady: (_) {},
          ),
        ),
      );

  testWidgets('standart — Google, belgilar va chiziq avvalgidek', (tester) async {
    await tester.pumpWidget(map(
      markers: const [
        CourierMarker(
          id: 'me',
          point: chust,
          anchor: Offset(0.5, 0.5),
          rotation: 90,
          flat: true,
          title: 'Siz',
        ),
        CourierMarker(
          id: 'delivery',
          point: gm.LatLng(41.01, 71.24),
          googleHue: gm.BitmapDescriptor.hueGreen,
          title: 'Mijoz manzili',
        ),
      ],
      route: const [chust, gm.LatLng(41.01, 71.24)],
    ));

    expect(find.byType(ml.MapLibreMap), findsNothing);
    final g = tester.widget<gm.GoogleMap>(find.byType(gm.GoogleMap));
    expect(g.markers.map((m) => m.markerId.value), unorderedEquals(['me', 'delivery']));
    final me = g.markers.firstWhere((m) => m.markerId.value == 'me');
    expect(me.rotation, 90);
    expect(me.flat, isTrue);
    expect(me.anchor, const Offset(0.5, 0.5));
    expect(g.polylines, hasLength(1));
    expect(g.polylines.single.points, hasLength(2));
    expect(g.polylines.single.color, Colors.orange);
    expect(g.padding.bottom, 120, reason: 'panel ostiga yashirinmasin');
    expect(g.mapType, gm.MapType.normal);
  });

  testWidgets('bitta nuqtali marshrut chiziq bo\'lmaydi', (tester) async {
    await tester.pumpWidget(map(route: const [chust]));
    expect(tester.widget<gm.GoogleMap>(find.byType(gm.GoogleMap)).polylines, isEmpty);
  });

  testWidgets('sun\'iy yo\'ldosh — Google hybrid', (tester) async {
    await tester.pumpWidget(map(satellite: true));
    expect(tester.widget<gm.GoogleMap>(find.byType(gm.GoogleMap)).mapType, gm.MapType.hybrid);
  });

  testWidgets('server "ondexmap" desa-yu OnDexMap yiqilgan bo\'lsa — Google',
      (tester) async {
    svc.setConfigForTest(cfg(MapProvider.ondexmap));
    svc.reportOndexFailure('courier', 'test');
    await tester.pumpWidget(map());
    expect(find.byType(gm.GoogleMap), findsOneWidget);
    expect(find.byType(ml.MapLibreMap), findsNothing);
  });
}
