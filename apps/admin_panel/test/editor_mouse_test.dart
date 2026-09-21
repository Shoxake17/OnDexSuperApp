import 'dart:convert';

import 'package:chust_admin/ondexmap/editor_view.dart';
import 'package:chust_admin/ondexmap/moderation_api.dart';
import 'package:chust_admin/ondexmap/ondexmap_session.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// Desktop: chizish panelidagi tugmalar HAQIQIY SICHQONCHA (PointerDeviceKind.mouse) bilan ishlaydi.
// Boshqa testlar sensor (touch) bilan bosadi; Windows'da esa foydalanuvchi sichqoncha ishlatadi:
// bu farq alohida qoplanadi.

final _png = base64Decode(
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==');

class _NoTiles extends TileProvider {
  @override
  ImageProvider getImage(TileCoordinates c, TileLayer o) => MemoryImage(_png);
}

http.Response _json(Object b) => http.Response.bytes(utf8.encode(jsonEncode(b)), 200,
    headers: {'content-type': 'application/json; charset=utf-8'});

void main() {
  testWidgets('mouse: «Ortga» va «Bekor» panel tugmalari ishlaydi', (tester) async {
    tester.view.physicalSize = const Size(1400, 1000);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final map = MapController();
    final api = ModerationApi(
      defaultUrl: 'http://127.0.0.1:8091',
      session: () => const OndexMapSession(token: 't'),
    );
    await http.runWithClient(() async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(body: OndexMapEditor(api: api, tileProvider: _NoTiles(), mapController: map)),
      ));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('editor-draw')));
      await tester.pumpAndSettle();

      final r = tester.getRect(find.byType(FlutterMap));
      for (final d in const [Offset(-100, -80), Offset(100, -80), Offset(0, 100)]) {
        await tester.tapAt(r.center + d, kind: PointerDeviceKind.mouse);
        await tester.pump(const Duration(milliseconds: 400));
        await tester.pumpAndSettle();
      }
      expect(find.text('3 nuqta'), findsOneWidget);

      // Sichqoncha bilan «Ortga».
      await tester.tap(find.byKey(const ValueKey('editor-undo')), kind: PointerDeviceKind.mouse);
      await tester.pumpAndSettle();
      expect(find.text('2 nuqta'), findsOneWidget, reason: 'mouse bilan «Ortga» ishlamadi');

      // Sichqoncha bilan «Bekor».
      await tester.tap(find.byKey(const ValueKey('editor-cancel-draw')), kind: PointerDeviceKind.mouse);
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('editor-points')), findsNothing, reason: 'mouse bilan «Bekor» ishlamadi');
    }, () => MockClient((req) async => req.url.path == '/api/features'
        ? _json({'type': 'FeatureCollection', 'features': []})
        : _json({})));
  });
}
