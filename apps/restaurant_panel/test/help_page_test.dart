import 'dart:convert';

import 'package:chust_restaurant/pages/help_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// "Yordam markazi": namuna bo'yicha ko'rinish (har kenglikda), admin kiritgan
// aloqa ma'lumotlari (kodga yozilgan raqam yo'q), xavfsiz havolalar,
// qidiruv va "Onlayn yordam".

const _contacts = {
  'phone': '+998901234567',
  'phone_hours': '09:00 – 22:00 (har kuni)',
  'telegram': 'ondex_support',
  'telegram_url': 'https://t.me/ondex_support',
  'email': 'support@ondex.uz',
  'email_note': '24/7 javob beramiz',
  'configured': true,
};

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

Future<void> _run(
  WidgetTester tester, {
  Size size = const Size(1600, 1000),
  http.Response Function()? contacts,
  int unread = 0,
  VoidCallback? onChat,
  VoidCallback? onOrders,
  required Future<void> Function() body,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  await http.runWithClient(
    () async {
      await tester.pumpWidget(MaterialApp(
        home: Scaffold(
          body: HelpPage(supportUnread: unread, onOpenChat: onChat, onOpenOrders: onOrders),
        ),
      ));
      await tester.pumpAndSettle();
      await body();
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async {
      if (request.method == 'GET' && request.url.path == '/support/contacts') {
        return contacts?.call() ?? _json(_contacts);
      }
      return _json({'error': 'kutilmagan so\'rov'}, 404);
    }),
  );
}

Finder _horizontalScrollables() => find.byWidgetPredicate((w) =>
    (w is ScrollView && w.scrollDirection == Axis.horizontal) ||
    (w is SingleChildScrollView && w.scrollDirection == Axis.horizontal));

void main() {
  for (final size in const [Size(1600, 1000), Size(1100, 900), Size(800, 1000), Size(420, 1000)]) {
    testWidgets('namuna bo\'yicha, overflowsiz — ${size.width.toInt()}px', (tester) async {
      await _run(tester, size: size, body: () async {
        expect(tester.takeException(), isNull);
        for (final t in [
          'Yordam markazi',
          'Foydalanuvchi qo\'llanmasi',
          'Video darsliklar',
          'Tezkor maslahatlar',
          'Dastur yangilanishlari',
          'Tez-tez so\'raladigan savollar',
          'Buyurtmani qanday qo\'yish mumkin?',
          'Tizim sekin ishlayapti',
          'Biz bilan bog\'laning',
          'Onlayn yordam',
        ]) {
          expect(find.text(t), findsOneWidget, reason: t);
        }
        // Sarlavha oldida belgi yo'q — matn sahifa chekinishidan boshlanadi.
        expect(tester.getTopLeft(find.text('Yordam markazi')).dx, closeTo(28, 0.5));
        // Admin kiritgan qiymatlar; eski qattiq yozilgan raqam yo'q.
        expect(find.text('+998 90 123 45 67'), findsOneWidget);
        expect(find.text('@ondex_support'), findsOneWidget);
        expect(find.text('support@ondex.uz'), findsOneWidget);
        expect(find.textContaining('278 42 07'), findsNothing);
        expect(_horizontalScrollables(), findsNothing);
      });
    });
  }

  testWidgets('aloqa kiritilmagan — tushuntirish, soxta raqam emas', (tester) async {
    await _run(tester, contacts: () => _json({'configured': false}), body: () async {
      expect(find.byKey(const ValueKey('help-contacts-empty')), findsOneWidget);
      expect(find.text('Qo\'ng\'iroq qilish'), findsNothing);
    });
  });

  testWidgets('buzilgan javobdagi xavfli qiymatlar tugma bo\'lmaydi', (tester) async {
    await _run(
      tester,
      contacts: () => _json({
        'phone': '+998901234567',
        'telegram': 'javascript:alert(1)',
        'telegram_url': 'javascript:alert(1)',
        'email': 'x@y.uz?bcc=all@corp.uz',
      }),
      body: () async {
        expect(find.byKey(const ValueKey('help-contact-phone')), findsOneWidget);
        expect(find.byKey(const ValueKey('help-contact-telegram')), findsNothing);
        expect(find.byKey(const ValueKey('help-contact-email')), findsNothing);
        expect(find.textContaining('javascript'), findsNothing);
      },
    );
  });

  testWidgets('yuklab bo\'lmasa — qayta urinish', (tester) async {
    var calls = 0;
    await _run(
      tester,
      contacts: () => ++calls == 1 ? _json({'error': 'server'}, 500) : _json(_contacts),
      body: () async {
        expect(find.text('Aloqa ma\'lumotlarini yuklab bo\'lmadi.'), findsOneWidget);
        await tester.tap(find.text('Qayta urinish'));
        await tester.pumpAndSettle();
        expect(find.text('+998 90 123 45 67'), findsOneWidget);
      },
    );
  });

  testWidgets('qidiruv: mos savollar, tutuq belgisi farqsiz, topilmasa yordam', (tester) async {
    await _run(tester, body: () async {
      final search = find.byKey(const ValueKey('help-search'));
      await tester.enterText(search, 'PRINTER');
      await tester.pumpAndSettle();
      expect(find.text('Printer va QR kod ishlamayapti'), findsOneWidget);
      expect(find.text('Xodimlarni qanday qo\'shish mumkin?'), findsNothing);

      await tester.enterText(search, 'qo‘shish');
      await tester.pumpAndSettle();
      expect(find.text('Xodimlarni qanday qo\'shish mumkin?'), findsOneWidget);

      await tester.enterText(search, 'zzzqqq');
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('help-no-results')), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('savol ochiladi va tegishli bo\'limga o\'tadi; onlayn yordam', (tester) async {
    var orders = 0;
    var chat = 0;
    await _run(tester, unread: 2, onOrders: () => orders++, onChat: () => chat++, body: () async {
      expect(find.byKey(const ValueKey('faq-answer-0')), findsNothing);
      await tester.tap(find.byKey(const ValueKey('faq-0')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('faq-answer-0')), findsOneWidget);
      await tester.tap(find.text('Buyurtmalarga o\'tish'));
      expect(orders, 1);

      expect(find.text('2 ta yangi javob'), findsOneWidget);
      await tester.ensureVisible(find.byKey(const ValueKey('help-online-support')));
      await tester.tap(find.byKey(const ValueKey('help-online-support')));
      expect(chat, 1);
    });
  });

  testWidgets('video darsliklar yo\'qligi ochiq aytiladi', (tester) async {
    await _run(tester, body: () async {
      await tester.tap(find.text('Video darsliklar'));
      await tester.pumpAndSettle();
      expect(find.textContaining('hali joylanmagan'), findsOneWidget);
      await tester.tap(find.text('Tushunarli'));
      await tester.pumpAndSettle();
    });
  });
}
