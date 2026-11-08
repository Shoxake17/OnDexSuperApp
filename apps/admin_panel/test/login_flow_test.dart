import 'dart:convert';

import 'package:ondex_admin/screens/login_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:qr_flutter/qr_flutter.dart';

/// A6: admin birinchi marta kiradi — kod -> parol o'rnatish -> TOTP ulash
/// (QR) -> tiklash kodlari; keyingi kirish — kod -> parol -> TOTP.
/// Server `routes_merchant_auth.go` javoblari soxta.
void main() {
  http.Response json(int code, Object body) =>
      http.Response(jsonEncode(body), code,
          headers: {'content-type': 'application/json'});

  final calls = <String>[];
  var enrolled = false;
  MockClient server() => MockClient((req) async {
        calls.add(req.url.path);
        final b = req.body.isEmpty
            ? <String, dynamic>{}
            : jsonDecode(req.body) as Map<String, dynamic>;
        switch (req.url.path) {
          case '/auth/telegram/start':
            return json(200, {'deep_link': 'https://t.me/ondex_bot?start=x'});
          case '/v2/auth/admin/otp/verify':
            if (b['code'] != '424242') {
              return json(401, {'error': 'kod noto\'g\'ri'});
            }
            return json(200, {
              'mfa_required': true,
              'mfa_token': 'omfa_1',
              'method': enrolled ? 'password' : 'password_set'
            });
          case '/v2/auth/mfa/password':
            expect(b['password_confirm'], b['password']);
            return json(200, {
              'mfa_required': true,
              'mfa_token': 'omfa_2',
              'method': 'totp_enroll'
            });
          case '/v2/auth/mfa/factor':
            if (b['password'] != 'Admin-Parol-1') {
              return json(401, {'error': 'parol noto\'g\'ri'});
            }
            return json(200, {
              'mfa_required': true,
              'mfa_token': 'omfa_3',
              'method': 'totp'
            });
          case '/v2/auth/mfa/totp/enroll':
            return json(200, {
              'secret': 'JBSWY3DPEHPK3PXP',
              'otpauth_uri': 'otpauth://totp/OnDex:x?secret=JBSWY3DPEHPK3PXP'
            });
          case '/v2/auth/mfa/totp/confirm':
            enrolled = true;
            return json(200, {
              'tokens': {'access_token': 'aE', 'refresh_token': 'ort_E'},
              'recovery_codes': List.generate(10, (i) => 'ABCDE-FGH${i}J'),
            });
          case '/v2/auth/mfa/verify':
            return json(200, {'access_token': 'aT', 'refresh_token': 'ort_T'});
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

  setUp(calls.clear);

  testWidgets(
      'birinchi kirish: kod -> parol o\'rnatish -> QR -> tiklash kodlari',
      (t) async {
    t.view.physicalSize = const Size(1200, 1600);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    enrolled = false;

    await http.runWithClient(() async {
      await t.pumpWidget(const MaterialApp(home: AdminLoginScreen()));
      await t.enterText(find.widgetWithText(TextField, 'Admin telefon raqami'),
          '+998901234567');
      await t
          .tap(find.widgetWithText(FilledButton, 'Telegram orqali kod olish'));
      await settle(t);
      await t.enterText(
          find.widgetWithText(TextField, 'Telegramdan kelgan kod'), '424242');
      await t.tap(find.widgetWithText(FilledButton, 'Davom etish'));
      await settle(t);
      expect(find.text("Xavfsizlik: yangi parol o'rnating"), findsOneWidget);

      await t.enterText(
          find.widgetWithText(TextField, 'Yangi parol'), 'Admin-Parol-1');
      await t.enterText(
          find.widgetWithText(TextField, 'Parolni takrorlang'), 'boshqa');
      await t.tap(find.widgetWithText(FilledButton, 'Davom etish'));
      await settle(t);
      expect(find.text('Parollar mos kelmadi'), findsOneWidget);
      expect(calls, isNot(contains('/v2/auth/mfa/password')));

      await t.enterText(find.widgetWithText(TextField, 'Parolni takrorlang'),
          'Admin-Parol-1');
      await t.tap(find.widgetWithText(FilledButton, 'Davom etish'));
      await settle(t);
      expect(find.byType(QrImageView), findsOneWidget);

      await t.enterText(
          find.widgetWithText(TextField, 'Ilovadagi 6 xonali kod'), '123456');
      await t.tap(find.widgetWithText(FilledButton, 'Tasdiqlash va yoqish'));
      await settle(t);
      expect(find.textContaining('ABCDE-FGH0J'), findsOneWidget);
      final btn = find.widgetWithText(FilledButton, 'Davom etish');
      expect(t.widget<FilledButton>(btn).onPressed, isNull);
    }, server);
  });

  testWidgets('keyingi kirish: kod -> parol (har safar) -> TOTP', (t) async {
    t.view.physicalSize = const Size(1200, 1600);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);
    enrolled = true;

    await http.runWithClient(() async {
      await t.pumpWidget(const MaterialApp(home: AdminLoginScreen()));
      await t.enterText(find.widgetWithText(TextField, 'Admin telefon raqami'),
          '+998901234567');
      await t
          .tap(find.widgetWithText(FilledButton, 'Telegram orqali kod olish'));
      await settle(t);
      await t.enterText(
          find.widgetWithText(TextField, 'Telegramdan kelgan kod'), '424242');
      await t.tap(find.widgetWithText(FilledButton, 'Davom etish'));
      await settle(t);
      expect(find.text('Xavfsizlik: parolingizni kiriting'), findsOneWidget);

      await t.enterText(find.widgetWithText(TextField, 'Parol'), 'xato');
      await t.tap(find.widgetWithText(FilledButton, 'Davom etish'));
      await settle(t);
      expect(find.textContaining("noto'g'ri"), findsOneWidget);

      await t.enterText(
          find.widgetWithText(TextField, 'Parol'), 'Admin-Parol-1');
      await t.tap(find.widgetWithText(FilledButton, 'Davom etish'));
      await settle(t);
      expect(find.text('Ikki bosqichli tasdiq'), findsOneWidget);
      expect(calls, contains('/v2/auth/admin/otp/verify'));
    }, server);
  });
}
