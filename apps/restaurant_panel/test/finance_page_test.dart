import 'dart:async';
import 'dart:convert';

import 'package:chust_restaurant/api.dart';
import 'package:chust_restaurant/pages/finance_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// "Moliya" sahifasi testlari.
//
// ┌─ NIMA QULFLANADI ──────────────────────────────────────────────────┐
//  1. Ulanmagan restoran XATO ko'rmaydi — ulash formasini ko'radi;
//  2. Raqamlar serverdan kelganicha ko'rsatiladi (qayta hisoblanmaydi);
//  3. Maketdagi xarajat/sof foyda bloklari CHIZILMAYDI — tizimda
//     xarajat yo'q va o'ylab topilgan raqam yozilmasligi kerak;
//  4. Qat'iy balandlikdagi kartochkalar va ko'p ustunli jadval turli
//     kenglikda "overflow" bermaydi.
//
// Server so'rovi `http.runWithClient` bilan ushlanadi (statistika
// testidagi usul): `ApiClient.send` yuqori darajadagi `http` funksiyasini
// chaqiradi, ya'ni sahifaga soxta klient kiritish shart emas.
// └────────────────────────────────────────────────────────────────────┘

String _date(DateTime d) =>
    '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}';

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

/// Ulangan do'kon, yetti kunlik davr.
Map<String, dynamic> _finance({
  bool connected = true,
  bool available = true,
  int turnover = 845000000,
  int refunded = 23400000,
  int transactions = 24,
  int prevTurnover = 754464285,
  int prevTransactions = 22,
  List<Map<String, dynamic>>? recent,
}) {
  final to = DateTime(2026, 9, 30);
  return {
    'connected': connected,
    'available': available,
    'store_id': connected ? '6' : '',
    'own_credentials': connected,
    'app_id': connected ? 'bookcafe_app' : '',
    'store_title': connected ? 'Book Cafe kassasi' : '',
    'account_masked': connected ? '••••6789' : '',
    'from': _date(to.subtract(const Duration(days: 6))),
    'to': _date(to),
    'turnover_tiyin': turnover,
    'refunded_tiyin': refunded,
    'commission_tiyin': 12675000,
    'net_tiyin': turnover - refunded,
    'transactions': transactions,
    'prev_turnover_tiyin': prevTurnover,
    'prev_transactions': prevTransactions,
    'daily': [
      for (var i = 0; i < 7; i++)
        {
          'date': _date(to.subtract(Duration(days: 6 - i))),
          'amount_tiyin': (i % 4 + 1) * 30000000,
          'count': i % 3 + 1,
        },
    ],
    'by_method': [
      {'ps': 'uzcard', 'amount_tiyin': 520000000, 'count': 14},
      {'ps': 'humo', 'amount_tiyin': 245000000, 'count': 7},
      {'ps': 'payme', 'amount_tiyin': 80000000, 'count': 3},
    ],
    'recent': recent ??
        [
          {
            'uuid': 'tx-1',
            'status': 'success',
            'ps': 'uzcard',
            'order_ref': 'ORD-10241',
            'amount_tiyin': 18000000,
            'commission_tiyin': 270000,
            'at': '2026-09-30 16:32:00',
          },
          {
            'uuid': 'tx-2',
            'status': 'revert',
            'ps': 'humo',
            // Juda uzun havola ham qatorni buzmasligi kerak.
            'order_ref': 'ORD-10240-juda-uzun-havola-matni-bilan-sinov',
            'amount_tiyin': 35000000,
            'commission_tiyin': 0,
            'at': '2026-09-30 15:47:00',
          },
          {
            'uuid': 'tx-3',
            'status': 'payment_waiting',
            'ps': '',
            'order_ref': '',
            'amount_tiyin': 9500000,
            'commission_tiyin': 0,
            'at': '',
          },
        ],
  };
}

/// Sahifani berilgan o'lchamda ochadi. Butun tana soxta klient zonasi
/// ICHIDA bajariladi — bosish natijasidagi keyingi so'rovlar ham ushlanadi.
Future<void> _run(
  WidgetTester tester, {
  required Size size,
  required http.Response Function(http.Request request) respond,
  required Future<void> Function() body,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  api.rid = 'rest-a';

  await http.runWithClient(
    () async {
      await tester.pumpWidget(const MaterialApp(
        home: Scaffold(body: FinancePage()),
      ));
      await tester.pumpAndSettle();
      await body();
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async => respond(request)),
  );
}

void main() {
  // Sahifa xotirasi static: testlar bir-biriga ta'sir qilmasligi uchun
  // har testdan OLDIN tozalanadi.
  setUp(FinancePage.debugClearMemory);

  // 1672 — 1920 px monitor (yon menyu 248 px); 420 — juda tor oyna.
  // 1000 — `kSplitBreakpoint`, joylashuv aynan shu yerda almashadi.
  for (final width in [1672.0, 1440.0, 1000.0, 900.0, 720.0, 420.0]) {
    testWidgets('ulangan do\'kon $width px kenglikda xatosiz chiziladi',
        (tester) async {
      await _run(
        tester,
        size: Size(width, 2400),
        respond: (_) => _json(_finance()),
        body: () async {
          expect(tester.takeException(), isNull);
          expect(find.text('Moliya'), findsOneWidget);
          expect(find.text('Umumiy tushum'), findsOneWidget);
          // Ikki joyda: KPI kartochkasi va halqa markazidagi jami —
          // sinov ma'lumotida to'lov turlari yig'indisi aylanmaga teng.
          expect(find.text('8 450 000 so\'m'), findsWidgets);
          expect(find.text('Kunlik tushum'), findsOneWidget);
          expect(find.text('So\'nggi operatsiyalar'), findsOneWidget);
        },
      );
    });
  }

  testWidgets('raqamlar serverdan kelganicha: sof tushum va foiz',
      (tester) async {
    await _run(
      tester,
      size: const Size(1672, 2400),
      respond: (_) => _json(_finance()),
      body: () async {
        // Sof tushum = tushum - qaytarilgan (server hisoblaydi).
        expect(find.text('8 216 000 so\'m'), findsOneWidget);
        expect(find.text('234 000 so\'m'), findsOneWidget); // qaytarilgan
        expect(find.text('24 ta'), findsOneWidget);
        // O'tgan davrga nisbatan: 8 450 000 / 7 544 642 ≈ +12%.
        expect(find.textContaining('+12%'), findsOneWidget);
      },
    );
  });

  // ┌─ MAKETDAN ATAYLAB OLIB TASHLANGAN BLOKLAR ────────────────────────┐
  // Tizimda xarajat daftari yo'q. "Jami xarajatlar", "Sof foyda" va
  // "Xarajatlar taqsimoti" ni chizish uchun o'ylab topilgan raqam kerak
  // bo'lardi — moliya sahifasida bu yolg'on. Test ular QAYTIB
  // KELMASLIGINI qulflaydi.
  // └───────────────────────────────────────────────────────────────────┘
  testWidgets('xarajat va sof foyda bloklari chizilmaydi', (tester) async {
    await _run(
      tester,
      size: const Size(1672, 2400),
      respond: (_) => _json(_finance()),
      body: () async {
        expect(find.text('Jami xarajatlar'), findsNothing);
        expect(find.text('Sof foyda'), findsNothing);
        expect(find.text('Xarajatlar taqsimoti'), findsNothing);
        expect(find.text('Oylik hisobot'), findsNothing);
        // O'rniga haqiqiy ko'rsatkichlar va sababi yozilgan.
        expect(find.text('Sof tushum'), findsOneWidget);
        expect(find.textContaining('xarajat daftari yo\'q'), findsOneWidget);
      },
    );
  });

  testWidgets('ulanmagan do\'kon: xato emas, ulash formasi', (tester) async {
    await _run(
      tester,
      size: const Size(1672, 1200),
      respond: (_) => _json(_finance(connected: false)),
      body: () async {
        expect(tester.takeException(), isNull);
        expect(find.text('To\'lov hisobi ulanmagan'), findsOneWidget);
        expect(find.text('Hisobni ulash'), findsOneWidget);
        // Xato paneli CHIQMASLIGI kerak — bu normal holat.
        expect(find.text('Qayta urinish'), findsNothing);
        // Raqamlar ko'rsatilmaydi.
        expect(find.text('Umumiy tushum'), findsNothing);
      },
    );
  });

  testWidgets('shlyuz OnDex tomonida sozlanmagan: ogohlantirish',
      (tester) async {
    await _run(
      tester,
      size: const Size(1672, 1200),
      respond: (_) => _json(_finance(connected: false, available: false)),
      body: () async {
        expect(find.textContaining('OnDex tomonida hali sozlanmagan'),
            findsOneWidget);
        // Ulash tugmasi bo'lmaydi: restoran o'zi hech narsa qila olmaydi.
        expect(find.text('Hisobni ulash'), findsNothing);
      },
    );
  });

  testWidgets('shlyuz yiqilsa: xato paneli va qayta urinish', (tester) async {
    var calls = 0;
    await _run(
      tester,
      size: const Size(1672, 1200),
      respond: (r) {
        if (r.url.path.endsWith('/settings')) return _json({});
        calls++;
        return _json({'error': 'server xatosi, keyinroq urinib ko\'ring'}, 502);
      },
      body: () async {
        expect(find.textContaining('yuklab bo\'lmadi'), findsOneWidget);
        final before = calls;
        await tester.tap(find.text('Qayta urinish'));
        await tester.pumpAndSettle();
        expect(calls, greaterThan(before));
      },
    );
  });

  // ┌─ NOTO'G'RI DO'KON RAQAMI ─────────────────────────────────────────┐
  // Qayta urinish bunday xatoni TUZATMAYDI — sahifa raqamni
  // o'zgartirish yo'lini ko'rsatishi kerak. Aynan shu holat jonli
  // sinovda yuz berdi: raqam "65" edi, panel esa "server xatosi,
  // keyinroq urinib ko'ring" deb turaverdi.
  // └───────────────────────────────────────────────────────────────────┘
  testWidgets('noto\'g\'ri kassa raqami: tuzatish tugmasi chiqadi',
      (tester) async {
    await _run(
      tester,
      size: const Size(1672, 1200),
      respond: (r) {
        if (r.url.path.endsWith('/settings')) {
          return _json({
            'multicard_store_id': '65',
            'multicard_app_id': 'bookcafe_app',
          });
        }
        return _json({
          'error': 'to\'lov tizimi "65" do\'konini tanimadi — raqamni tekshiring'
        }, 422);
      },
      body: () async {
        expect(find.textContaining('tanimadi'), findsOneWidget);
        expect(find.text('Kalitlarni o\'zgartirish'), findsOneWidget);
        // Tugma ulash oynasini ochadi, oynada eski qiymatlar turadi.
        await tester.tap(find.text('Kalitlarni o\'zgartirish'));
        await tester.pumpAndSettle();
        expect(find.text('To\'lov hisobini ulash'), findsOneWidget);
        expect(find.widgetWithText(TextField, '65'), findsOneWidget);
        expect(find.widgetWithText(TextField, 'bookcafe_app'), findsOneWidget);
      },
    );
  });

  // ┌─ ULASH: KALITLAR SERVERGA YUBORILADI, NATIJA TASDIQLANADI ────────┐
  // Restoran "tekshirib ulash" ni bosadi → server javobida PUL QAYERGA
  // tushishi ko'rinadi → shundan keyingina oyna yopiladi. Oyna darhol
  // yopilsa, restoran noto'g'ri kassaga ulanganini bilmay qolardi.
  // └───────────────────────────────────────────────────────────────────┘
  testWidgets('ulash: kalitlar yuboriladi va rekvizitlar tasdiqlanadi',
      (tester) async {
    Map<String, dynamic>? sent;
    var financeCalls = 0;
    await _run(
      tester,
      size: const Size(1672, 1400),
      respond: (r) {
        if (r.url.path.endsWith('/finance/connect')) {
          sent = jsonDecode(r.body) as Map<String, dynamic>;
          return _json({
            'store_id': '6',
            'store_title': 'Book Cafe kassasi',
            'merchant_name': 'Book Cafe MChJ',
            'merchant_tin': '307578794',
            'account_masked': '••••6789',
            'contract_num': 'POS/8042',
            'fee_percent': '1.5',
          });
        }
        financeCalls++;
        // Ulangunga qadar — ulanmagan holat.
        return _json(_finance(connected: financeCalls > 1));
      },
      body: () async {
        await tester.tap(find.text('Hisobni ulash'));
        await tester.pumpAndSettle();

        await tester.enterText(
            find.widgetWithText(TextField, 'application_id'), 'bookcafe_app');
        await tester.enterText(
            find.widgetWithText(TextField, 'secret'), 's3cr3t');
        await tester.enterText(
            find.widgetWithText(TextField, 'Kassa raqami (store_id)'), '6');
        await tester.tap(find.text('Tekshirib ulash'));
        await tester.pumpAndSettle();

        expect(sent, {
          'application_id': 'bookcafe_app',
          'secret': 's3cr3t',
          'store_id': '6',
        });
        // Oyna yopilmaydi: avval rekvizitlar ko'rsatiladi.
        expect(find.text('Ulandi'), findsOneWidget);
        expect(find.text('Book Cafe MChJ'), findsOneWidget);
        expect(find.text('••••6789'), findsOneWidget);
        expect(find.text('1.5%'), findsOneWidget);

        await tester.tap(find.text('Yopish'));
        await tester.pumpAndSettle();
        // Yopilgach sahifa qaytadan yuklanadi va raqamlar chiqadi.
        expect(find.text('Umumiy tushum'), findsOneWidget);
      },
    );
  });

  testWidgets('ulangan holat: kassa, hisob va shartnoma turi ko\'rinadi',
      (tester) async {
    await _run(
      tester,
      size: const Size(1672, 2400),
      respond: (_) => _json(_finance()),
      body: () async {
        expect(find.textContaining('Book Cafe kassasi'), findsOneWidget);
        expect(find.textContaining('••••6789'), findsOneWidget);
        expect(find.textContaining('o\'z shartnomangiz'), findsOneWidget);
        expect(find.text('Uzish'), findsOneWidget);
      },
    );
  });

  testWidgets('uzish: tasdiqsiz bajarilmaydi', (tester) async {
    var deleted = false;
    await _run(
      tester,
      size: const Size(1672, 2400),
      respond: (r) {
        if (r.method == 'DELETE') {
          deleted = true;
          return _json({}, 204);
        }
        return _json(_finance(connected: !deleted));
      },
      body: () async {
        await tester.tap(find.text('Uzish'));
        await tester.pumpAndSettle();
        // Tasdiq oynasi chiqadi, so'rov hali ketmaydi.
        expect(find.textContaining('Moliya bo\'limi bo\'shab qoladi'),
            findsOneWidget);
        expect(deleted, isFalse);

        await tester.tap(find.text('Bekor qilish'));
        await tester.pumpAndSettle();
        expect(deleted, isFalse);

        await tester.tap(find.text('Uzish'));
        await tester.pumpAndSettle();
        await tester.tap(find.widgetWithText(FilledButton, 'Uzish'));
        await tester.pumpAndSettle();
        expect(deleted, isTrue);
      },
    );
  });

  // ┌─ "FAQAT MOLIYA AYLANADI" ─────────────────────────────────────────┐
  // `RestaurantShell` bo'limlarni kirganda YANGIDAN quradi, shuning
  // uchun sahifa holati har safar yo'qoladi. Boshqa sahifalar 0–70 ms
  // da yuklanadi (sezilmaydi), Moliya esa tashqi shlyuzdan oladi
  // (0.8–5 s) va har kirganda to'liq ekranli aylanuvchi chiqarardi.
  //
  // Endi qaytib kirganda oxirgi natija DARHOL ko'rinadi, yangisi
  // orqada yuklanadi.
  // └───────────────────────────────────────────────────────────────────┘
  testWidgets('qaytib kirganda: oxirgi natija darhol, aylanuvchisiz', (tester) async {
    tester.view.physicalSize = const Size(1672, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    api.rid = 'rest-a';
    Completer<void>? gate;

    await http.runWithClient(
      () async {
        // 1-kirish: odatdagidek yuklanadi.
        await tester.pumpWidget(const MaterialApp(home: Scaffold(body: FinancePage())));
        await tester.pumpAndSettle();
        expect(find.text('Umumiy tushum'), findsOneWidget);

        // Boshqa bo'limga o'tdi (sahifa daraxtdan chiqdi).
        await tester.pumpWidget(const SizedBox());

        // 2-kirish: shlyuz hali JAVOB BERMAGAN.
        gate = Completer<void>();
        await tester.pumpWidget(const MaterialApp(home: Scaffold(body: FinancePage())));
        await tester.pump();

        // Eski raqamlar darhol turibdi, to'liq ekranli aylanuvchi YO'Q.
        expect(find.text('Umumiy tushum'), findsOneWidget);
        expect(find.byType(CircularProgressIndicator), findsNothing);
        // Orqada yangilanayotgani ingichka chiziq bilan ko'rinadi.
        expect(find.byType(LinearProgressIndicator), findsOneWidget);

        gate!.complete();
        await tester.pumpAndSettle();
        expect(find.byType(LinearProgressIndicator), findsNothing);
        expect(find.text('Umumiy tushum'), findsOneWidget);
        await tester.pumpWidget(const SizedBox());
      },
      () => MockClient((request) async {
        final g = gate;
        if (g != null) await g.future;
        return _json(_finance());
      }),
    );
  });

  // Birinchi kirish (xotira bo'sh) — aylanuvchi KERAK: ko'rsatadigan
  // narsa yo'q. Xotira bu holatni buzmasligi kerak.
  testWidgets('birinchi kirishda: xotira bo\'sh — aylanuvchi chiqadi', (tester) async {
    tester.view.physicalSize = const Size(1672, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    api.rid = 'rest-a';
    final gate = Completer<void>();

    await http.runWithClient(
      () async {
        await tester.pumpWidget(const MaterialApp(home: Scaffold(body: FinancePage())));
        await tester.pump();
        expect(find.byType(CircularProgressIndicator), findsOneWidget);
        expect(find.text('Umumiy tushum'), findsNothing);
        gate.complete();
        await tester.pumpAndSettle();
        expect(find.text('Umumiy tushum'), findsOneWidget);
        await tester.pumpWidget(const SizedBox());
      },
      () => MockClient((request) async {
        await gate.future;
        return _json(_finance());
      }),
    );
  });

  // Tanlangan davr eslab qolinadi: qaytganda "7 kun"ga tushib ketmaydi.
  testWidgets('tanlangan davr qaytib kirganda ham saqlanadi', (tester) async {
    tester.view.physicalSize = const Size(1672, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    api.rid = 'rest-a';
    final queries = <String>[];

    await http.runWithClient(
      () async {
        await tester.pumpWidget(const MaterialApp(home: Scaffold(body: FinancePage())));
        await tester.pumpAndSettle();
        await tester.tap(find.text('30 kun'));
        await tester.pumpAndSettle();
        await tester.pumpWidget(const SizedBox());

        queries.clear();
        await tester.pumpWidget(const MaterialApp(home: Scaffold(body: FinancePage())));
        await tester.pumpAndSettle();

        final today = DateTime.now();
        final monthAgo = today.subtract(const Duration(days: 29));
        expect(queries, isNotEmpty);
        expect(queries.first, contains('from=${_date(monthAgo)}'),
            reason: 'qaytganda 30 kunlik davr tiklanishi kerak');
        await tester.pumpWidget(const SizedBox());
      },
      () => MockClient((request) async {
        queries.add(request.url.query);
        return _json(_finance());
      }),
    );
  });

  // Boshqa restoran bilan kirilsa, eskisining raqamlari ko'rinmasligi
  // kerak (kalit restoran IDsini o'z ichiga oladi).
  testWidgets('boshqa restoran: eskisining raqamlari ko\'rinmaydi', (tester) async {
    tester.view.physicalSize = const Size(1672, 2400);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final gate = Completer<void>();
    var holdSecond = false;

    await http.runWithClient(
      () async {
        api.rid = 'rest-a';
        await tester.pumpWidget(const MaterialApp(home: Scaffold(body: FinancePage())));
        await tester.pumpAndSettle();
        expect(find.text('Umumiy tushum'), findsOneWidget);
        await tester.pumpWidget(const SizedBox());

        api.rid = 'rest-b';
        holdSecond = true;
        await tester.pumpWidget(const MaterialApp(home: Scaffold(body: FinancePage())));
        await tester.pump();
        // Boshqa restoranning xotirasi yo'q: eski raqamlar chiqmaydi.
        expect(find.text('Umumiy tushum'), findsNothing);
        expect(find.byType(CircularProgressIndicator), findsOneWidget);
        gate.complete();
        await tester.pumpAndSettle();
        await tester.pumpWidget(const SizedBox());
      },
      () => MockClient((request) async {
        if (holdSecond) await gate.future;
        return _json(_finance());
      }),
    );
  });

  testWidgets('davr tugmasi so\'rovni o\'zgartiradi', (tester) async {
    final requested = <String>[];
    await _run(
      tester,
      size: const Size(1672, 2400),
      respond: (r) {
        requested.add(r.url.query);
        return _json(_finance());
      },
      body: () async {
        expect(requested.length, 1);
        // Standart — 7 kun.
        final today = DateTime.now();
        final weekAgo = today.subtract(const Duration(days: 6));
        expect(requested.first, contains('from=${_date(weekAgo)}'));

        await tester.tap(find.text('30 kun'));
        await tester.pumpAndSettle();
        expect(requested.length, 2);
        final monthAgo = today.subtract(const Duration(days: 29));
        expect(requested.last, contains('from=${_date(monthAgo)}'));
        expect(requested.last, contains('to=${_date(today)}'));
      },
    );
  });

  testWidgets('qaytarilgan operatsiya minus bilan ko\'rinadi', (tester) async {
    await _run(
      tester,
      size: const Size(1672, 2400),
      respond: (_) => _json(_finance()),
      body: () async {
        expect(find.text('-350 000 so\'m'), findsOneWidget);
        expect(find.text('Qaytarildi'), findsOneWidget);
        expect(find.text('Tushdi'), findsOneWidget);
        // Noma'lum holat kodi o'zgarishsiz ko'rsatiladi — yashirilmaydi.
        expect(find.text('payment_waiting'), findsOneWidget);
      },
    );
  });

  // Bo'sh davr grafik yoki jadvalni yiqitmasligi kerak.
  testWidgets('bo\'sh davr: sabab yoziladi, xato chiqmaydi', (tester) async {
    await _run(
      tester,
      size: const Size(1672, 2400),
      respond: (_) => _json({
        ..._finance(
            turnover: 0,
            refunded: 0,
            transactions: 0,
            prevTurnover: 0,
            prevTransactions: 0,
            recent: []),
        'daily': [
          for (var i = 0; i < 7; i++)
            {
              'date': _date(DateTime(2026, 9, 24 + i)),
              'amount_tiyin': 0,
              'count': 0
            },
        ],
        'by_method': <Map<String, dynamic>>[],
        'net_tiyin': 0,
      }),
      body: () async {
        expect(tester.takeException(), isNull);
        expect(find.text('Bu davrda operatsiya yo\'q'), findsOneWidget);
        expect(find.textContaining('karta orqali to\'lov bo\'lmagan'),
            findsWidgets);
        // Oldingi davr ham nol — soxta "+100%" chizilmaydi.
        expect(find.textContaining('%'), findsNothing);
      },
    );
  });
}
