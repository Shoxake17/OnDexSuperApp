import 'dart:convert';

import 'package:chust_admin/pages/settings_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// "Sozlamalar": admin kiritgan aloqa ma'lumotlari. Noto'g'ri qiymat serverga
// ketmaydi, to'g'risi normallashtirilib PUT qilinadi, server rad etsa uning
// xabari ko'rsatiladi.

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

const _saved = {
  'phone': '+998901234567',
  'phone_hours': '09:00 – 22:00',
  'telegram': 'ondex_support',
  'email': 'support@ondex.uz',
  'email_note': '24/7',
  'configured': true,
  'updated_at': '2026-09-14T16:40:00Z',
};

Future<void> _run(
  WidgetTester tester, {
  Object contacts = _saved,
  http.Response Function(http.Request)? onPut,
  required Future<void> Function(List<http.Request> requests) body,
}) async {
  tester.view.physicalSize = const Size(1400, 1000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  final requests = <http.Request>[];
  await http.runWithClient(
    () async {
      await tester.pumpWidget(MaterialApp(
        theme: ThemeData(colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)), useMaterial3: true),
        home: const Scaffold(body: SettingsPage()),
      ));
      await tester.pumpAndSettle();
      await body(requests);
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async {
      requests.add(request);
      if (request.method == 'GET' && request.url.path == '/support/contacts') return _json(contacts);
      if (request.method == 'PUT' && request.url.path == '/admin/support/contacts') {
        return onPut?.call(request) ?? _json({...jsonDecode(request.body) as Map, 'updated_at': '2026-09-14T17:00:00Z'});
      }
      return _json({'error': 'kutilmagan so\'rov'}, 404);
    }),
  );
}

String _text(WidgetTester tester, String key) =>
    tester.widget<TextFormField>(find.byKey(ValueKey(key))).controller!.text;

bool _saveEnabled(WidgetTester tester) =>
    tester.widget<ButtonStyleButton>(find.byKey(const ValueKey('settings-save'))).enabled;

void main() {
  testWidgets('saqlangan qiymatlar o\'qiladi; o\'zgarishsiz saqlash o\'chiq', (tester) async {
    await _run(tester, body: (_) async {
      expect(tester.takeException(), isNull);
      expect(_text(tester, 'settings-phone'), '+998 90 123 45 67');
      expect(_text(tester, 'settings-telegram'), '@ondex_support');
      expect(_text(tester, 'settings-email'), 'support@ondex.uz');
      expect(_saveEnabled(tester), isFalse);
      expect(find.text('Panelda shunday ko\'rinadi'), findsOneWidget);
    });
  });

  testWidgets('noto\'g\'ri qiymat serverga ketmaydi', (tester) async {
    await _run(tester, body: (requests) async {
      await tester.enterText(find.byKey(const ValueKey('settings-phone')), '12345');
      await tester.enterText(find.byKey(const ValueKey('settings-telegram')), 'javascript:alert(1)');
      await tester.enterText(find.byKey(const ValueKey('settings-email')), 'Ali <ali@ondex.uz>');
      await tester.pumpAndSettle();
      expect(_saveEnabled(tester), isTrue);
      await tester.tap(find.byKey(const ValueKey('settings-save')));
      await tester.pumpAndSettle();
      expect(find.textContaining('Telefon raqam noto\'g\'ri'), findsOneWidget);
      expect(find.textContaining('Telegram nomi noto\'g\'ri'), findsOneWidget);
      expect(find.textContaining('Elektron pochta noto\'g\'ri'), findsOneWidget);
      expect(requests.where((r) => r.method == 'PUT'), isEmpty);
    });
  });

  testWidgets('to\'g\'ri qiymat normallashtirib saqlanadi', (tester) async {
    await _run(tester, body: (requests) async {
      await tester.enterText(find.byKey(const ValueKey('settings-phone')), '998 91 555-66-77');
      await tester.enterText(find.byKey(const ValueKey('settings-telegram')), 'https://t.me/New_Support');
      await tester.enterText(find.byKey(const ValueKey('settings-email')), 'Help@OnDex.uz');
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('settings-save')));
      await tester.pumpAndSettle();
      final put = requests.singleWhere((r) => r.method == 'PUT');
      expect(jsonDecode(put.body), {
        'phone': '+998915556677',
        'phone_hours': '09:00 – 22:00',
        'telegram': 'New_Support',
        'email': 'help@ondex.uz',
        'email_note': '24/7',
      });
      expect(find.text('Saqlandi — barcha restoran panellarida yangilandi'), findsOneWidget);
      expect(_text(tester, 'settings-phone'), '+998 91 555 66 77');
      expect(_text(tester, 'settings-telegram'), '@New_Support');
      expect(_saveEnabled(tester), isFalse);
    });
  });

  testWidgets('server rad etsa xabari ko\'rsatiladi', (tester) async {
    await _run(
      tester,
      onPut: (_) => _json({'error': 'Telegram foydalanuvchi nomi noto\'g\'ri (masalan: @ondex_support)'}, 400),
      body: (_) async {
        await tester.enterText(find.byKey(const ValueKey('settings-note')), 'Ish kunlari javob beramiz');
        await tester.pumpAndSettle();
        await tester.tap(find.byKey(const ValueKey('settings-save')));
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('settings-error')), findsOneWidget);
        expect(find.textContaining('Telegram foydalanuvchi nomi noto\'g\'ri'), findsOneWidget);
      },
    );
  });

  testWidgets('bo\'sh holat oldindan ko\'rinishda aytiladi', (tester) async {
    await _run(tester, contacts: const {'configured': false}, body: (_) async {
      expect(find.byKey(const ValueKey('settings-preview-empty')), findsOneWidget);
      expect(find.text('Hali saqlanmagan'), findsOneWidget);
    });
  });
}
