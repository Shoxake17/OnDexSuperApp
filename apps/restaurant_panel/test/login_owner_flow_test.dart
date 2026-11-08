import 'dart:convert';

import 'package:chust_restaurant/screens/login_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'package:shared_preferences/shared_preferences.dart';

/// A4b: egasi birinchi marta kiradi — parol -> Telegram kodi -> TOTP ulash
/// (QR) -> tiklash kodlari. Server `routes_merchant_auth.go` javoblari soxta.
void main() {
  http.Response json(int code, Object body) =>
      http.Response(jsonEncode(body), code,
          headers: {'content-type': 'application/json'});

  final calls = <String>[];
  MockClient server() => MockClient((req) async {
        calls.add(req.url.path);
        final b = req.body.isEmpty
            ? <String, dynamic>{}
            : jsonDecode(req.body) as Map<String, dynamic>;
        switch (req.url.path) {
          case '/v2/auth/merchant/password':
            expect(b['client_id'], 'merchant-desktop');
            if (b['password'] != 'Parol-123') {
              return json(401, {'error': 'login yoki parol noto\'g\'ri'});
            }
            return json(200,
                {'mfa_required': true, 'mfa_token': 'omfa_1', 'method': 'otp'});
          case '/auth/telegram/start':
            return json(200, {'deep_link': 'https://t.me/ondex_bot?start=x'});
          case '/v2/auth/mfa/factor':
            expect(b['code'], '424242');
            return json(200, {
              'mfa_required': true,
              'mfa_token': 'omfa_2',
              'method': 'totp_enroll'
            });
          case '/v2/auth/mfa/totp/enroll':
            return json(200, {
              'secret': 'JBSWY3DPEHPK3PXP',
              'otpauth_uri': 'otpauth://totp/OnDex:x?secret=JBSWY3DPEHPK3PXP'
            });
          case '/v2/auth/mfa/totp/confirm':
            if (b['code'] != '123456') {
              return json(401, {'error': 'tasdiqlash kodi noto\'g\'ri'});
            }
            return json(200, {
              'tokens': {'access_token': 'aE', 'refresh_token': 'ort_E'},
              'recovery_codes': List.generate(10, (i) => 'ABCDE-FGH${i}J'),
            });
        }
        return json(404, {'error': 'yoq'});
      });

  // Soxta HTTP va platforma kanallari HAQIQIY asinxron: pumpAndSettle`n  // ularni kutmaydi — qisqa haqiqiy kutishdan keyin kadrlar chiziladi.
  Future<void> settle(WidgetTester t) async {
    for (var i = 0; i < 5; i++) {
      await t.runAsync(
          () => Future<void>.delayed(const Duration(milliseconds: 30)));
      await t.pump();
    }
  }

  Future<void> go(WidgetTester t, Future<void> Function() body) =>
      http.runWithClient(body, server);

  setUp(() {
    calls.clear();
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets('egasi: parol -> kod -> QR -> tiklash kodlari', (t) async {
    t.view.physicalSize = const Size(1200, 1600);
    t.view.devicePixelRatio = 1;
    addTearDown(t.view.reset);

    await go(t, () async {
      await t.pumpWidget(const MaterialApp(home: RestaurantLoginScreen()));
      await t.enterText(
          find.widgetWithText(TextField, 'Telefon raqami'), '+998901234567');

      // noto'g'ri parol — xato ko'rsatiladi, bosqich o'zgarmaydi
      await t.enterText(find.widgetWithText(TextField, 'Parol'), 'xato');
      await t.tap(find.widgetWithText(FilledButton, 'Kirish'));
      await settle(t);
      expect(find.textContaining("noto'g'ri"), findsOneWidget);

      await t.enterText(find.widgetWithText(TextField, 'Parol'), 'Parol-123');
      await t.tap(find.widgetWithText(FilledButton, 'Kirish'));
      await settle(t);
      // TOTP ulashdan oldin telefon egaligi: Telegram kodi so'raladi
      expect(calls, contains('/auth/telegram/start'));
      expect(find.text('Xavfsizlik: telefon raqamingizni tasdiqlang'),
          findsOneWidget);

      await t.enterText(
          find.widgetWithText(TextField, 'Telegramdan kelgan kod'), '424242');
      await t.tap(find.widgetWithText(FilledButton, 'Davom etish'));
      await settle(t);
      expect(find.byType(QrImageView), findsOneWidget);
      expect(find.textContaining('JBSWY3DPEHPK3PXP'), findsOneWidget);

      await t.enterText(
          find.widgetWithText(TextField, 'Ilovadagi 6 xonali kod'), '000000');
      await t.tap(find.widgetWithText(FilledButton, 'Tasdiqlash va yoqish'));
      await settle(t);
      expect(find.textContaining('tasdiqlash kodi'), findsOneWidget);

      await t.enterText(
          find.widgetWithText(TextField, 'Ilovadagi 6 xonali kod'), '123456');
      await t.tap(find.widgetWithText(FilledButton, 'Tasdiqlash va yoqish'));
      await settle(t);
      expect(find.textContaining('ABCDE-FGH0J'), findsOneWidget);
      expect(find.textContaining('ABCDE-FGH9J'), findsOneWidget);

      // "saqladim" belgilanmaguncha davom etib bo'lmaydi
      final btn = find.widgetWithText(FilledButton, 'Davom etish');
      expect(t.widget<FilledButton>(btn).onPressed, isNull);
      await t.tap(find.byType(CheckboxListTile));
      await t.pump();
      expect(t.widget<FilledButton>(btn).onPressed, isNotNull);
    });
  });
}
