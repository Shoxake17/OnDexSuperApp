import 'dart:convert';

import 'package:chust_admin/ondexmap/moderation_api.dart';
import 'package:chust_admin/ondexmap/moderation_view.dart';
import 'package:chust_admin/ondexmap/ondexmap_session.dart';
import 'package:chust_admin/ondexmap/road_preview.dart';
import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// Foydalanuvchi xaritada CHIZGAN yo'l (LineString): moderator uni xaritada
// ko'radi, tasdiqlaganda chiziq AYNAN qaytadi (lat/lng YUBORILMAYDI), tur
// faqat chiziq turlari orasida almashtiriladi.

const _id = '33333333-3333-4333-8333-333333333333';
const _pointId = '44444444-4444-4444-8444-444444444444';

final _png = base64Decode(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==');

class _NoTiles extends TileProvider {
  @override
  ImageProvider getImage(TileCoordinates coordinates, TileLayer options) => MemoryImage(_png);
}

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

final _meta = {
  'kinds': [
    {'key': 'organization', 'label': 'Tashkilot', 'geometry': 'point', 'allowed': ['name', 'category', 'description']},
    {'key': 'road', 'label': 'Yo\'l', 'geometry': 'line', 'allowed': ['name', 'description']},
    {'key': 'other', 'label': 'Boshqa ob\'ekt', 'geometry': 'point', 'allowed': ['description']},
  ],
  'categories': ['Kafe'],
  'max_photos': 4,
};

const _line = [
  [71.2400, 41.0000],
  [71.2412, 41.0009],
  [71.2425, 41.0016],
];

Map<String, dynamic> _road() => {
      'id': _id,
      'kind': 'road',
      'kind_label': 'Yo\'l',
      'name': 'Navoiy ko\'chasi',
      'description': 'Yo\'l qazilgan',
      'lat': 41.0009,
      'lng': 71.2412,
      'photos': 0,
      'created_at': '2026-09-21T08:00:00Z',
      'status': 'pending',
      'hint': 'abcdef0123456789',
      'geometry': {'type': 'LineString', 'coordinates': _line},
      'length_m': 485.4,
    };

Map<String, dynamic> _point() => {
      'id': _pointId,
      'kind': 'organization',
      'kind_label': 'Tashkilot',
      'name': 'Chust Non',
      'category': 'Kafe',
      'lat': 41.0,
      'lng': 71.2,
      'photos': 0,
      'created_at': '2026-09-21T08:00:00Z',
      'status': 'pending',
      'hint': 'abcdef0123456789',
      'geometry': {'type': 'Point', 'coordinates': [71.2, 41.0]},
    };

class _Server {
  final requests = <http.Request>[];
  List<Map<String, dynamic>> pending;
  List<Map<String, dynamic>> places = [];

  _Server(this.pending);

  http.Response handle(http.Request r) {
    requests.add(r);
    final p = r.url.path;
    if (p == '/api/places/meta') return _json(_meta);
    if (p == '/api/submissions' && r.method == 'GET') {
      return _json({'pending': pending.length, 'submissions': pending});
    }
    if (p == '/api/places' && r.method == 'GET') return _json({'places': places});
    if (p == '/api/submissions/approve') {
      pending = [];
      return _json({'place_id': 'pl-1'});
    }
    return _json({'error': 'kutilmagan so\'rov'}, 404);
  }

  http.Request last(String path) => requests.lastWhere((r) => r.url.path == path);
}

Future<void> _pump(WidgetTester tester, _Server server) async {
  tester.view.physicalSize = const Size(1400, 2600);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  final api = ModerationApi(
    defaultUrl: 'http://127.0.0.1:8091',
    session: () => const OndexMapSession(token: 'tok'),
  );
  await http.runWithClient(() async {
    await tester.pumpWidget(MaterialApp(
      theme: ThemeData(colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)), useMaterial3: true),
      home: Scaffold(body: OndexMapModeration(api: api, tileProvider: _NoTiles())),
    ));
    await tester.pumpAndSettle();
  }, () => MockClient((r) async => server.handle(r)));
}

Future<void> _act(_Server server, Future<void> Function() body) =>
    http.runWithClient(body, () => MockClient((r) async => server.handle(r)));

void main() {
  test('PlaceRecord: LineString -> nuqtalar ([lng,lat] -> LatLng), uzunlik', () {
    final r = PlaceRecord.fromJson(_road());
    expect(r.isLine, isTrue);
    expect(r.line.length, 3);
    // GeoJSON [lng, lat] -> LatLng(lat, lng): tartib almashib ketmasligi shart.
    expect(r.line.first.latitude, 41.0);
    expect(r.line.first.longitude, 71.24);
    expect(r.lengthM, closeTo(485.4, 0.01));
    expect(PlaceRecord.fromJson(_point()).isLine, isFalse);
  });

  test('PlaceRecord: buzuq geometriya yiqitmaydi (nuqta deb hisoblanadi)', () {
    for (final g in <Object?>[null, 'x', {'type': 'LineString'}, {'type': 'LineString', 'coordinates': 'x'}, {'type': 'LineString', 'coordinates': [[1]]}]) {
      final r = PlaceRecord.fromJson({..._road(), 'geometry': g});
      expect(r.isLine, isFalse, reason: '$g');
    }
    // Buzuq elementlar tashlanadi, yaroqlilari qoladi.
    final r = PlaceRecord.fromJson({
      ..._road(),
      'geometry': {
        'type': 'LineString',
        'coordinates': [
          [71.24, 41.0],
          ['a', 'b'],
          [71.25, 41.001],
        ],
      },
    });
    expect(r.line.length, 2);
  });

  test('KindMeta: geometry (eski server bermasa — nuqta)', () {
    expect(KindMeta.fromJson({'key': 'road', 'label': 'Yo\'l', 'geometry': 'line', 'allowed': []}).isLine, isTrue);
    expect(KindMeta.fromJson({'key': 'other', 'label': 'x', 'allowed': []}).isLine, isFalse);
    expect(KindMeta.fromJson({'key': 'x', 'label': 'x', 'geometry': 'polygon', 'allowed': []}).isLine, isFalse);
  });

  testWidgets('yo\'l kartochkasi: xaritada chiziq, nuqta soni va uzunlik; lat/lng maydonlari YO\'Q', (tester) async {
    final server = _Server([_road()]);
    await _pump(tester, server);

    expect(find.byType(RoadPreview), findsOneWidget);
    expect(find.byType(PolylineLayer), findsOneWidget);
    expect(find.byKey(const ValueKey('mod-road-info-$_id')), findsOneWidget);
    expect(find.text('Yo\'l: 3 nuqta · 485 m'), findsOneWidget);
    // Chiziq uchun koordinata maydonlari ko'rsatilmaydi (tahrirlab bo'lmaydi).
    expect(find.byKey(const ValueKey('mod-lat-$_id')), findsNothing);
    expect(find.byKey(const ValueKey('mod-lng-$_id')), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('nuqta ob\'ekt: xarita YO\'Q, lat/lng maydonlari bor (avvalgidek)', (tester) async {
    final server = _Server([_point()]);
    await _pump(tester, server);
    expect(find.byType(RoadPreview), findsNothing);
    expect(find.byKey(const ValueKey('mod-lat-$_pointId')), findsOneWidget);
    expect(find.byKey(const ValueKey('mod-lng-$_pointId')), findsOneWidget);
  });

  testWidgets('tasdiqlash: chiziq AYNAN yuboriladi ([lng, lat]), lat/lng yuborilmaydi', (tester) async {
    final server = _Server([_road()]);
    await _pump(tester, server);

    await _act(server, () async {
      await tester.tap(find.byKey(const ValueKey('mod-approve-$_id')));
      await tester.pumpAndSettle();
    });
    final body = jsonDecode(server.last('/api/submissions/approve').body) as Map<String, dynamic>;
    final edit = body['edit'] as Map<String, dynamic>;
    expect(edit['kind'], 'road');
    expect(edit.containsKey('lat'), isFalse);
    expect(edit.containsKey('lng'), isFalse);
    expect(edit['line'], [
      [71.24, 41.0],
      [71.2412, 41.0009],
      [71.2425, 41.0016],
    ]);
    // Faqat ruxsat etilgan maydonlar: yo'lda nom va tavsif.
    expect(edit['name'], 'Navoiy ko\'chasi');
    expect(edit['description'], 'Yo\'l qazilgan');
    expect(edit.containsKey('phone'), isFalse);
    expect(tester.takeException(), isNull);
  });

  testWidgets('tur faqat o\'z shakli ichida: yo\'l — faqat chiziq turlari, nuqta — faqat nuqta turlari', (tester) async {
    final server = _Server([_road(), _point()]);
    await _pump(tester, server);

    // Har kartochkadagi «Turi» ro'yxati (yopiq holatda ham elementlar daraxtda).
    // Tartib kartochkalar tartibida: avval yo'l, keyin nuqta ob'ekt. Turkum
    // dropdown'i (bo'sh qiymat va «Kafe») hisobga olinmaydi.
    final kinds = tester
        .widgetList<DropdownMenuItem<String>>(find.byType(DropdownMenuItem<String>, skipOffstage: false))
        .map((e) => e.value!)
        .where((v) => v == 'road' || v == 'organization' || v == 'other')
        .toList();
    // Yo'l kartochkasida FAQAT «road» (chiziq turi); nuqta ob'ektda «road» YO'Q.
    expect(kinds, ['road', 'organization', 'other']);
  });
  testWidgets('«Xaritadagi ob\'ektlar»: yo\'l uzunligi bilan ko\'rinadi', (tester) async {
    final server = _Server([]);
    server.places = [
      {..._road(), 'status': 'approved'},
    ];
    await _pump(tester, server);
    await _act(server, () async {
      await tester.tap(find.text('Xaritadagi ob\'ektlar'));
      await tester.pumpAndSettle();
    });
    expect(find.textContaining('Yo\'l · 485 m'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
