import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:ondex_admin/api.dart';
import 'package:ondex_admin/pages/kyc_page.dart';

/// KYC (A6): navbat ko'rinadi; qaror Authenticator kodisiz yuborilmaydi;
/// rad etishda sabab majburiy; noto'g'ri kod (403) sessiyani yopmaydi.
void main() {
  http.Response json(int code, Object body) =>
      http.Response(jsonEncode(body), code,
          headers: {'content-type': 'application/json'});

  final posts = <Map<String, dynamic>>[];
  MockClient server() => MockClient((req) async {
        if (req.method == 'GET' && req.url.path == '/v2/admin/organizations') {
          return json(200, {
            'organizations': [
              {
                'id': 'org1',
                'name': 'Book Cafe MCHJ',
                'tin': '123456789',
                'status': 'pending'
              },
            ]
          });
        }
        if (req.method == 'POST' &&
            req.url.path == '/v2/admin/organizations/org1/verify') {
          final b = jsonDecode(req.body) as Map<String, dynamic>;
          posts.add(b);
          if (b['stepup_code'] != '123456') {
            return json(403, {
              'error': 'Authenticator kodi noto\'g\'ri',
              'step_up_invalid': true
            });
          }
          return json(200, {'id': 'org1', 'status': 'active'});
        }
        return json(404, {'error': 'yoq'});
      });

  Future<void> settle(WidgetTester t) async {
    for (var i = 0; i < 5; i++) {
      await t.runAsync(
          () => Future<void>.delayed(const Duration(milliseconds: 30)));
      await t.pump();
    }
  }

  setUp(() {
    posts.clear();
    api.token = 'aT';
    api.refreshToken = 'ort_T'; // OnDex ID (markaziy) admin sessiyasi
  });
  tearDown(() {
    api.token = null;
    api.refreshToken = null;
  });

  testWidgets(
      'navbat; kodsiz yuborilmaydi; noto\'g\'ri kod — xabar; to\'g\'ri — tasdiq',
      (t) async {
    t.view.physicalSize = const Size(1400, 1000);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    var unauthorized = 0;
    api.onUnauthorized = () => unauthorized++;

    await http.runWithClient(() async {
      await t.pumpWidget(const MaterialApp(home: Scaffold(body: KycPage())));
      await settle(t);
      expect(find.text('Book Cafe MCHJ'), findsOneWidget);

      await t.tap(find.byKey(const ValueKey('kyc-approve-org1')));
      await t.pumpAndSettle();
      await t.tap(find.byKey(const ValueKey('kyc-confirm')));
      await t.pump();
      expect(find.text('Authenticator kodini kiriting'), findsOneWidget);
      expect(posts, isEmpty);

      await t.enterText(find.byKey(const ValueKey('kyc-code')), '000000');
      await t.tap(find.byKey(const ValueKey('kyc-confirm')));
      await settle(t);
      expect(find.text('Authenticator kodi noto\'g\'ri'), findsOneWidget);
      expect(unauthorized, 0,
          reason: 'noto\'g\'ri kod sessiyani yopmasligi kerak');

      await t.tap(find.byKey(const ValueKey('kyc-approve-org1')));
      await t.pumpAndSettle();
      await t.enterText(find.byKey(const ValueKey('kyc-code')), '123456');
      await t.tap(find.byKey(const ValueKey('kyc-confirm')));
      await settle(t);
      expect(posts.last['decision'], 'approve');
      expect(posts.last['stepup_code'], '123456');
    }, server);
  });

  testWidgets('rad etish — sabab majburiy', (t) async {
    t.view.physicalSize = const Size(1400, 1000);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    await http.runWithClient(() async {
      await t.pumpWidget(const MaterialApp(home: Scaffold(body: KycPage())));
      await settle(t);
      await t.tap(find.text('Rad etish'));
      await t.pumpAndSettle();
      await t.enterText(find.byKey(const ValueKey('kyc-code')), '123456');
      await t.tap(find.byKey(const ValueKey('kyc-confirm')));
      await t.pump();
      expect(find.textContaining('Sababni yozing'), findsOneWidget);
      expect(posts, isEmpty);
    }, server);
  });

  testWidgets(
      'eski (HS256) admin sessiyasi — qayta kirish so\'raladi, so\'rov yo\'q',
      (t) async {
    api.refreshToken = null;
    var calls = 0;
    await http.runWithClient(() async {
      await t.pumpWidget(const MaterialApp(home: Scaffold(body: KycPage())));
      await settle(t);
      expect(find.byKey(const ValueKey('kyc-error')), findsOneWidget);
    },
        () => MockClient((_) async {
              calls++;
              return json(200, {});
            }));
    expect(calls, 0);
  });
}
