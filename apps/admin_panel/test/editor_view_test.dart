import 'dart:convert';

import 'package:chust_admin/ondexmap/editor_geometry.dart';
import 'package:chust_admin/ondexmap/editor_view.dart';
import 'package:chust_admin/ondexmap/moderation_api.dart';
import 'package:chust_admin/ondexmap/ondexmap_session.dart';
import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:latlong2/latlong.dart';

// Nativ muharrir: xaritada chizish, tanlash, saqlash, o'chirish, muqobil nom.
// Tarmoq va tile'lar soxta; xarita (flutter_map) HAQIQIY.

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

// 1×1 shaffof PNG (soxta tile).
final _png = base64Decode(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==');

class _NoTiles extends TileProvider {
  @override
  ImageProvider getImage(TileCoordinates coordinates, TileLayer options) => MemoryImage(_png);
}

Map<String, dynamic> _feature(String id, String name, Map<String, dynamic> geometry,
        {String source = 'survey', String? kind}) =>
    {
      'type': 'Feature',
      'geometry': geometry,
      'properties': {
        'id': id,
        'name': name,
        'source': source,
        'layer': kind == null ? 'mahalla' : 'street',
        if (kind != null) 'kind': kind,
      },
    };

// Server ST_Multi qaytaradi.
final _mahallaGeom = {
  'type': 'MultiPolygon',
  'coordinates': [
    [
      [[71.2390, 40.9990], [71.2420, 40.9990], [71.2420, 41.0020], [71.2390, 41.0020], [71.2390, 40.9990]],
    ],
  ],
};

final _streetGeom = {
  'type': 'MultiLineString',
  'coordinates': [
    [[71.2300, 41.0000], [71.2350, 41.0010]],
  ],
};

class _Server {
  final requests = <http.Request>[];
  Map<String, dynamic> config = {};
  List<Map<String, dynamic>> mahallas;
  List<Map<String, dynamic>> streets;
  http.Response? saveResponse;
  http.Response? featuresResponse;

  _Server({List<Map<String, dynamic>>? mahallas, List<Map<String, dynamic>>? streets})
      : mahallas = mahallas ?? [_feature('m-1', 'Serob', _mahallaGeom), _feature('m-2', 'Kamarsada', _mahallaGeom, source: 'official')],
        streets = streets ?? [_feature('s-1', 'Navoiy ko\'chasi', _streetGeom, kind: 'kocha')];

  http.Response handle(http.Request r) {
    requests.add(r);
    final p = r.url.path;
    if (p == '/api/config') return _json(config);
    if (p == '/api/features') {
      if (featuresResponse != null) return featuresResponse!;
      final list = r.url.queryParameters['kind'] == 'street' ? streets : mahallas;
      return _json({'type': 'FeatureCollection', 'features': list});
    }
    if (p == '/api/mahalla' || p == '/api/street') return saveResponse ?? _json({'id': 'yangi-1'});
    if (p == '/api/delete') return _json({'ok': true});
    if (p == '/api/alias') return _json({'ok': true});
    return _json({'error': 'kutilmagan so\'rov'}, 404);
  }

  http.Request last(String path) => requests.lastWhere((r) => r.url.path == path);
  Map<String, dynamic> lastBody(String path) => jsonDecode(last(path).body) as Map<String, dynamic>;
  Iterable<http.Request> to(String path) => requests.where((r) => r.url.path == path);
}

class _Env {
  final _Server server;
  final MapController map;
  _Env(this.server, this.map);

  Future<T> run<T>(Future<T> Function() body) =>
      http.runWithClient(body, () => MockClient((r) async => server.handle(r)));
}

Future<_Env> _pump(WidgetTester tester, _Server server) async {
  tester.view.physicalSize = const Size(1400, 1000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  final map = MapController();
  final env = _Env(server, map);
  final api = ModerationApi(
    defaultUrl: 'http://127.0.0.1:8091',
    session: () => const OndexMapSession(token: 'tok'),
  );
  await env.run(() async {
    await tester.pumpWidget(MaterialApp(
      theme: ThemeData(colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)), useMaterial3: true),
      home: Scaffold(body: OndexMapEditor(api: api, tileProvider: _NoTiles(), mapController: map)),
    ));
    await tester.pumpAndSettle();
  });
  return env;
}

/// Xarita vidjetining ekrandagi to'rtburchagi.
Rect _mapRect(WidgetTester tester) => tester.getRect(find.byType(FlutterMap));

/// Ekrandagi nuqtani xarita koordinatasiga aylantiradi (kamera bilan).
LatLng _at(WidgetTester tester, _Env env, Offset screen) =>
    env.map.camera.screenOffsetToLatLng(screen - _mapRect(tester).topLeft);

Offset _screenOf(WidgetTester tester, _Env env, LatLng ll) =>
    env.map.camera.latLngToScreenOffset(ll) + _mapRect(tester).topLeft;

/// flutter_map bosishni ikki marta bosishdan ajratish uchun biroz kutadi.
Future<void> _tapMap(WidgetTester tester, _Env env, Offset screen) async {
  await tester.tapAt(screen);
  await tester.pump(const Duration(milliseconds: 400));
  await tester.pumpAndSettle();
}

Future<void> _tap(WidgetTester tester, _Env env, Finder f) async {
  await env.run(() async {
    await tester.ensureVisible(f);
    await tester.tap(f);
    await tester.pumpAndSettle();
  });
}

void main() {
  testWidgets('ro\'yxat va hisob serverdan; xarita tirik', (tester) async {
    final env = await _pump(tester, _Server());
    // Yon panel ro'yxati (xaritadagi nom yorliqlari alohida vidjet).
    expect(find.byKey(const ValueKey('editor-item-m-1')), findsOneWidget);
    expect(find.byKey(const ValueKey('editor-item-m-2')), findsOneWidget);
    expect(find.text('2 ta'), findsOneWidget);
    expect(find.byType(FlutterMap), findsOneWidget);
    expect(env.map.camera.zoom, greaterThan(5));
    expect(tester.takeException(), isNull);
    for (final r in env.server.requests) {
      expect(r.headers['X-API-Key'], 'tok');
    }
  });

  testWidgets('mahalla chizish: 4 nuqta → Yakunlash → nom → Saqlash; GeoJSON to\'g\'ri', (tester) async {
    final env = await _pump(tester, _Server(mahallas: []));
    expect(find.byKey(const ValueKey('editor-empty')), findsOneWidget);

    await _tap(tester, env, find.byKey(const ValueKey('editor-draw')));
    final r = _mapRect(tester);
    final taps = [
      r.center + const Offset(-150, -100),
      r.center + const Offset(150, -100),
      r.center + const Offset(150, 100),
      r.center + const Offset(-150, 100),
    ];
    final expected = [for (final t in taps) _at(tester, env, t)];

    for (final t in taps) {
      await _tapMap(tester, env, t);
    }
    expect(find.text('4 nuqta'), findsOneWidget);

    await _tap(tester, env, find.byKey(const ValueKey('editor-finish')));
    expect(find.byKey(const ValueKey('editor-name')), findsOneWidget, reason: 'forma chiqdi');

    await tester.enterText(find.byKey(const ValueKey('editor-name')), '  Yangi mahalla ');
    await _tap(tester, env, find.byKey(const ValueKey('editor-save')));

    final body = env.server.lastBody('/api/mahalla');
    expect(body['id'], '');
    expect(body['name'], 'Yangi mahalla', reason: 'nom qirqilgan');
    expect(body['source'], 'survey');
    expect(body.containsKey('kind'), isFalse, reason: 'mahallada ko\'cha turi yo\'q');
    // Server geometriyani MATN sifatida kutadi.
    final g = jsonDecode(body['geometry'] as String) as Map<String, dynamic>;
    expect(g['type'], 'Polygon');
    final ring = (g['coordinates'] as List).first as List;
    expect(ring, hasLength(5), reason: '4 nuqta + yopuvchi');
    expect(ring.first, ring.last);
    for (var i = 0; i < 4; i++) {
      expect((ring[i] as List)[0], closeTo(expected[i].longitude, 1e-5), reason: 'lng[$i]');
      expect((ring[i] as List)[1], closeTo(expected[i].latitude, 1e-5), reason: 'lat[$i]');
    }
    // Saqlangach forma yopiladi va xabar chiqadi.
    expect(find.byKey(const ValueKey('editor-name')), findsNothing);
    expect(find.text('Saqlandi'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('«Yakunlash» kam nuqtada o\'chiq; Ortga oxirgi nuqtani olib tashlaydi', (tester) async {
    final env = await _pump(tester, _Server());
    await _tap(tester, env, find.byKey(const ValueKey('editor-draw')));
    final r = _mapRect(tester);

    await _tapMap(tester, env, r.center);
    await _tapMap(tester, env, r.center + const Offset(80, 0));
    expect(find.text('2 nuqta'), findsOneWidget);
    expect(tester.widget<ButtonStyleButton>(find.byKey(const ValueKey('editor-finish'))).enabled, isFalse);

    await _tap(tester, env, find.byKey(const ValueKey('editor-undo')));
    expect(find.text('1 nuqta'), findsOneWidget);

    await _tap(tester, env, find.byKey(const ValueKey('editor-cancel-draw')));
    expect(find.byKey(const ValueKey('editor-points')), findsNothing);
  });

  testWidgets('o\'zini kesib o\'tuvchi chegara saqlanmaydi (sabab ko\'rsatiladi)', (tester) async {
    final env = await _pump(tester, _Server(mahallas: []));
    await _tap(tester, env, find.byKey(const ValueKey('editor-draw')));
    final c = _mapRect(tester).center;
    // «Galstuk»: 1↘2 va 3↖4 kesishadi.
    for (final d in const [Offset(-100, -100), Offset(100, 100), Offset(100, -100), Offset(-100, 100)]) {
      await _tapMap(tester, env, c + d);
    }
    await _tap(tester, env, find.byKey(const ValueKey('editor-finish')));
    expect(find.textContaining('kesib o\'tmasligi'), findsOneWidget);
    expect(find.byKey(const ValueKey('editor-name')), findsNothing, reason: 'forma ochilmaydi');
    expect(env.server.to('/api/mahalla'), isEmpty);
  });

  testWidgets('poligonni birinchi nuqtani bosib yopish', (tester) async {
    final env = await _pump(tester, _Server(mahallas: []));
    await _tap(tester, env, find.byKey(const ValueKey('editor-draw')));
    final c = _mapRect(tester).center;
    final pts = [c + const Offset(-100, -80), c + const Offset(100, -80), c + const Offset(0, 100)];
    for (final p in pts) {
      await _tapMap(tester, env, p);
    }
    await _tapMap(tester, env, pts.first + const Offset(3, 2)); // birinchi nuqta yaqinida
    expect(find.byKey(const ValueKey('editor-name')), findsOneWidget, reason: 'chizish yakunlandi');
    expect(find.byKey(const ValueKey('editor-points')), findsNothing);
  });

  testWidgets('ro\'yxatdan tanlash: nom/manba to\'ldiriladi; geometriya O\'ZGARMAY saqlanadi', (tester) async {
    final env = await _pump(tester, _Server());
    await _tap(tester, env, find.byKey(const ValueKey('editor-item-m-2')));

    expect(tester.widget<TextField>(find.byKey(const ValueKey('editor-name'))).controller!.text, 'Kamarsada');
    expect(find.text('Rasmiy (hokimlik)'), findsOneWidget, reason: 'manba tanlandi');
    expect(find.byKey(const ValueKey('editor-delete')), findsOneWidget);

    await tester.enterText(find.byKey(const ValueKey('editor-name')), 'Kamarsada (yangi)');
    await _tap(tester, env, find.byKey(const ValueKey('editor-save')));

    final body = env.server.lastBody('/api/mahalla');
    expect(body['id'], 'm-2');
    expect(body['name'], 'Kamarsada (yangi)');
    expect(body['source'], 'official');
    expect(jsonDecode(body['geometry'] as String), _mahallaGeom,
        reason: 'tahrirlanmagan geometriya serverdan kelgani bilan AYNAN bir xil');
  });

  testWidgets('xaritadagi poligonni bosish uni tanlaydi; bo\'sh joyni bosish tegmaydi', (tester) async {
    final env = await _pump(tester, _Server(mahallas: [_feature('m-1', 'Serob', _mahallaGeom)]));
    // Poligon markazi.
    final inside = _screenOf(tester, env, const LatLng(41.0005, 71.2405));
    await _tapMap(tester, env, inside);
    expect(tester.widget<TextField>(find.byKey(const ValueKey('editor-name'))).controller!.text, 'Serob');

    // «Bekor» — tanlov olinadi.
    await _tap(tester, env, find.byKey(const ValueKey('editor-cancel')));
    expect(find.byKey(const ValueKey('editor-name')), findsNothing);

    // Bo'sh joy.
    await _tapMap(tester, env, _screenOf(tester, env, const LatLng(41.05, 71.30)));
    expect(find.byKey(const ValueKey('editor-name')), findsNothing);
  });

  testWidgets('qayta chizish: eski id saqlanadi, geometriya YANGI bo\'ladi', (tester) async {
    final env = await _pump(tester, _Server());
    await _tap(tester, env, find.byKey(const ValueKey('editor-item-m-1')));
    await _tap(tester, env, find.byKey(const ValueKey('editor-draw')));
    final c = _mapRect(tester).center;
    for (final d in const [Offset(-60, -60), Offset(60, -60), Offset(0, 70)]) {
      await _tapMap(tester, env, c + d);
    }
    await _tap(tester, env, find.byKey(const ValueKey('editor-finish')));
    await _tap(tester, env, find.byKey(const ValueKey('editor-save')));

    final body = env.server.lastBody('/api/mahalla');
    expect(body['id'], 'm-1', reason: 'mavjud obyekt yangilanadi, yangisi yaratilmaydi');
    final g = jsonDecode(body['geometry'] as String) as Map<String, dynamic>;
    expect(g['type'], 'Polygon');
    expect(g, isNot(_mahallaGeom));
  });

  testWidgets('nom bo\'sh bo\'lsa so\'rov yuborilmaydi', (tester) async {
    final env = await _pump(tester, _Server());
    await _tap(tester, env, find.byKey(const ValueKey('editor-item-m-1')));
    await tester.enterText(find.byKey(const ValueKey('editor-name')), '   ');
    await _tap(tester, env, find.byKey(const ValueKey('editor-save')));
    expect(find.text('Nom kiritilmagan'), findsOneWidget);
    expect(env.server.to('/api/mahalla'), isEmpty);
  });

  testWidgets('server rad etsa: xabar, forma QOLADI', (tester) async {
    final server = _Server()..saveResponse = _json({'error': 'geometriya xizmat hududidan tashqarida'}, 400);
    final env = await _pump(tester, server);
    await _tap(tester, env, find.byKey(const ValueKey('editor-item-m-1')));
    await _tap(tester, env, find.byKey(const ValueKey('editor-save')));
    expect(find.text('geometriya xizmat hududidan tashqarida'), findsOneWidget);
    expect(find.byKey(const ValueKey('editor-name')), findsOneWidget);
    expect(tester.widget<ButtonStyleButton>(find.byKey(const ValueKey('editor-save'))).enabled, isTrue,
        reason: 'qayta urinish mumkin');
  });

  testWidgets('o\'chirish: tasdiq oynasi; Bekor qilinsa so\'rov yo\'q', (tester) async {
    final env = await _pump(tester, _Server());
    await _tap(tester, env, find.byKey(const ValueKey('editor-item-m-1')));

    await _tap(tester, env, find.byKey(const ValueKey('editor-delete')));
    await _tap(tester, env, find.text('Bekor').last);
    expect(env.server.to('/api/delete'), isEmpty);

    await _tap(tester, env, find.byKey(const ValueKey('editor-delete')));
    await _tap(tester, env, find.byKey(const ValueKey('editor-delete-confirm')));
    expect(env.server.lastBody('/api/delete'), {'kind': 'mahalla', 'id': 'm-1'});
    expect(find.byKey(const ValueKey('editor-name')), findsNothing);
  });

  testWidgets('ko\'cha qatlami: chiziq chizish; kind va LineString ketadi', (tester) async {
    final env = await _pump(tester, _Server(streets: []));
    await _tap(tester, env, find.text('Ko\'cha').first);
    expect(env.server.requests.last.url.queryParameters['kind'], 'street');

    await _tap(tester, env, find.byKey(const ValueKey('editor-draw')));
    final c = _mapRect(tester).center;
    await _tapMap(tester, env, c + const Offset(-100, 0));
    await _tapMap(tester, env, c + const Offset(100, 20));
    await _tap(tester, env, find.byKey(const ValueKey('editor-finish')));

    await tester.enterText(find.byKey(const ValueKey('editor-name')), 'Yangi ko\'cha');
    await _tap(tester, env, find.byKey(const ValueKey('editor-save')));

    final body = env.server.lastBody('/api/street');
    expect(body['kind'], 'kocha');
    expect((jsonDecode(body['geometry'] as String) as Map)['type'], 'LineString');
  });

  testWidgets('muqobil nom: faqat ko\'cha tanlanganda; server maydonlari to\'g\'ri', (tester) async {
    final env = await _pump(tester, _Server());
    // Mahallada muqobil nom yo'q.
    await _tap(tester, env, find.byKey(const ValueKey('editor-item-m-1')));
    expect(find.byKey(const ValueKey('editor-alias')), findsNothing);

    await _tap(tester, env, find.text('Ko\'cha').first);
    await _tap(tester, env, find.byKey(const ValueKey('editor-item-s-1')));
    expect(find.byKey(const ValueKey('editor-alias')), findsOneWidget);

    await tester.enterText(find.byKey(const ValueKey('editor-alias')), 'Katta ko\'cha');
    await _tap(tester, env, find.byKey(const ValueKey('editor-alias-add')));
    expect(env.server.lastBody('/api/alias'), {
      'street_id': 's-1',
      'alias': 'Katta ko\'cha',
      'kind': 'xalq',
      'source': 'survey',
    });
    expect(find.textContaining('Qo\'shildi'), findsOneWidget);
  });

  testWidgets('yuklash xatosi: sabab va «Qayta urinish»', (tester) async {
    final server = _Server()..featuresResponse = _json({'error': 'baza ulanmagan'}, 502);
    final env = await _pump(tester, server);
    expect(find.byKey(const ValueKey('mod-error')), findsOneWidget);
    expect(find.text('baza ulanmagan'), findsOneWidget);

    server.featuresResponse = null;
    await _tap(tester, env, find.text('Qayta urinish'));
    expect(find.byKey(const ValueKey('editor-item-m-1')), findsOneWidget);
  });

  testWidgets('sun\'iy yo\'ldosh: tashqi manzil (config almashtirilgan) — tugma YO\'Q', (tester) async {
    final server = _Server()..config = {'satellite_url': 'https://evil.example/tiles/{z}/{x}/{y}'};
    await _pump(tester, server);
    expect(find.byKey(const ValueKey('editor-basemap')), findsNothing);
  });

  testWidgets('sun\'iy yo\'ldosh: lokal proksi — tugma va litsenziya krediti bor', (tester) async {
    final server = _Server()
      ..config = {
        'satellite_url': 'http://127.0.0.1:8090/tiles/satellite/{z}/{x}/{y}',
        'satellite_attribution': 'Powered by Esri',
      };
    await _pump(tester, server);
    expect(find.byKey(const ValueKey('editor-basemap')), findsOneWidget);
    expect(find.text('Sputnik'), findsOneWidget);
    // Sun'iy yo'ldosh bor bo'lsa STANDART shu: chegara tasvir ustida chiziladi.
    final sb = tester.widget(find.byKey(const ValueKey('editor-basemap'))) as SegmentedButton;
    expect(sb.selected.single.toString(), contains('satellite'));
  });

  testWidgets('boshlang\'ich moslashtirish: obyektlar ko\'rinadi (butun mamlakat emas)', (tester) async {
    final env = await _pump(tester, _Server());
    // Serob/Kamarsada Chust yaqinida (~0.003°): moslashtirilgan zoom yuqori bo'lishi kerak.
    expect(env.map.camera.zoom, greaterThan(14),
        reason: 'xarita o\'lchamga ega bo\'lmaganda hisoblangan (zoom ~6) — xato');
    final c = env.map.camera.center;
    expect(c.latitude, closeTo(41.0005, 0.01));
    expect(c.longitude, closeTo(71.2405, 0.01));
  });

  testWidgets('qatlam almashtirilganda yarim chizilgan/tanlangan holat tozalanadi', (tester) async {
    final env = await _pump(tester, _Server());
    await _tap(tester, env, find.byKey(const ValueKey('editor-item-m-1')));
    expect(find.byKey(const ValueKey('editor-name')), findsOneWidget);

    await _tap(tester, env, find.text('Ko\'cha').first);
    expect(find.byKey(const ValueKey('editor-name')), findsNothing);
    expect(find.text('Navoiy ko\'chasi'), findsOneWidget);
    expect(find.text('Serob'), findsNothing);
  });

  test('geometriya yordamchilari ekran bilan mos: metersPerPixel > 0', () {
    expect(metersPerPixel(41, 16), greaterThan(0));
  });
}
