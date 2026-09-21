import 'dart:convert';

import 'package:chust_admin/pages/stats_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// Statistika: jami kartalar, kunlik grafik, holatlar va har restoran jadvali.
// Raqamlar serverdan keladi (`GET /admin/stats?period=`); bu yerda ko'rsatish,
// tartiblash, qidiruv, davr almashtirish va xatolik holatlari tekshiriladi.

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

Map<String, dynamic> _row(String id, String name,
        {required int orders,
        int news = 0,
        int inProgress = 0,
        required int completed,
        int cancelled = 0,
        required int revenue,
        bool open = true,
        bool deleted = false}) =>
    {
      'id': id,
      'name': name,
      'logo_url': '',
      'open': open,
      if (deleted) 'deleted': true,
      'orders': orders,
      'new': news,
      'in_progress': inProgress,
      'accepted': inProgress + completed,
      'completed': completed,
      'cancelled': cancelled,
      'revenue_tiyin': revenue,
      'avg_check_tiyin': completed == 0 ? 0 : revenue ~/ completed,
      'completion_rate': completed + cancelled == 0 ? 0 : completed * 100 ~/ (completed + cancelled),
    };

Map<String, dynamic> _payload(String period) {
  final rows = [
    _row('a', 'Book Cafe', orders: 60, news: 2, inProgress: 8, completed: 40, cancelled: 10, revenue: 400000000),
    _row('b', 'Osh Markazi', orders: 30, inProgress: 2, completed: 25, cancelled: 3, revenue: 250000000, open: false),
    _row('c', 'Yangi Restoran', orders: 0, completed: 0, revenue: 0),
    _row('', '', orders: 5, completed: 5, revenue: 20000000, deleted: true),
  ];
  return {
    'period': period,
    'from': period == 'all' ? '' : '2026-08-23',
    'to': '2026-09-21',
    'totals': {
      'orders': 95,
      'new': 2,
      'in_progress': 10,
      'accepted': 80,
      'completed': 70,
      'cancelled': 13,
      'revenue_tiyin': 670000000,
      'avg_check_tiyin': 9571428,
      'completion_rate': 84,
    },
    'restaurants': rows,
    'statuses': {'created': 2, 'accepted': 4, 'preparing': 6, 'delivered': 65, 'served': 5, 'cancelled': 10, 'rejected': 3},
    'daily': [
      for (var i = 0; i < 14; i++)
        {
          'date': '2026-09-${(8 + i).toString().padLeft(2, '0')}',
          'orders': i == 13 ? 12 : i,
          'completed': i == 13 ? 9 : 0,
          'revenue_tiyin': i == 13 ? 90000000 : i * 1000000,
        },
    ],
    'couriers_online': 3,
    'couriers_pending': 2,
    'couriers_total': 9,
    'restaurants_total': 3,
    'restaurants_open': 2,
  };
}

Future<void> _run(WidgetTester tester, Future<void> Function(List<http.Request> requests) body,
    {Size size = const Size(1600, 1000), http.Response Function(http.Request)? onStats}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  final requests = <http.Request>[];
  await http.runWithClient(
    () async {
      await tester.pumpWidget(MaterialApp(
        theme: ThemeData(colorScheme: ColorScheme.fromSeed(seedColor: const Color(0xFF0B5D1E)), useMaterial3: true),
        home: const Scaffold(body: StatsPage()),
      ));
      await tester.pumpAndSettle();
      await body(requests);
      await tester.pumpWidget(const SizedBox());
    },
    () => MockClient((request) async {
      requests.add(request);
      if (request.method == 'GET' && request.url.path == '/admin/stats') {
        return onStats?.call(request) ?? _json(_payload(request.url.queryParameters['period'] ?? '30d'));
      }
      return _json({'error': 'kutilmagan so\'rov: ${request.method} ${request.url.path}'}, 404);
    }),
  );
}

String _kpi(WidgetTester tester, String key) =>
    tester.widget<Text>(find.byKey(ValueKey('stats-kpi-$key-value'))).data!;

/// Jadvaldagi restoran nomlari yuqoridan pastga (jami qatoridan tashqari).
List<String> _tableOrder(WidgetTester tester) {
  final names = ['Book Cafe', 'Osh Markazi', 'Yangi Restoran', 'O\'chirilgan restoranlar'];
  final found = <MapEntry<double, String>>[];
  for (final n in names) {
    final f = find.descendant(of: find.byKey(const ValueKey('stats-table')), matching: find.text(n));
    if (f.evaluate().isNotEmpty) found.add(MapEntry(tester.getTopLeft(f.first).dy, n));
  }
  found.sort((a, b) => a.key.compareTo(b.key));
  return found.map((e) => e.value).toList();
}

void main() {
  testWidgets('jami kartalar serverdagi raqamlarni ko\'rsatadi', (tester) async {
    await _run(tester, (requests) async {
      expect(tester.takeException(), isNull);
      // Standart davr — 30 kun.
      expect(requests.single.url.queryParameters['period'], '30d');

      expect(_kpi(tester, 'orders'), '95');
      expect(find.text('Qabul qilingan: 80'), findsOneWidget);
      expect(_kpi(tester, 'delivered'), '70');
      expect(find.text('Bajarilish: 84%'), findsOneWidget);
      expect(_kpi(tester, 'cancelled'), '13');
      expect(_kpi(tester, 'couriers-online'), '3');
      expect(find.text('Jami kuryerlar: 9'), findsOneWidget);
      expect(_kpi(tester, 'couriers-pending'), '2');
      expect(_kpi(tester, 'restaurants'), '3');
      expect(find.text('Hozir ochiq: 2'), findsOneWidget);
      expect(_kpi(tester, 'new'), '2');
      // Tushum so'mda formatlangan (6 700 000 so'm).
      expect(_kpi(tester, 'revenue').replaceAll(RegExp(r'\s'), ''), contains('6700000'));
      // Davr oralig'i sarlavhada.
      expect(find.text('23.08.2026 — 21.09.2026'), findsOneWidget);
    });
  });

  testWidgets('jadval: har restoran, o\'chirilgani va JAMI qatori; tushum bo\'yicha tartib', (tester) async {
    await _run(tester, (requests) async {
      expect(find.byKey(const ValueKey('stats-table')), findsOneWidget);
      expect(find.byKey(const ValueKey('stats-row-a')), findsOneWidget);
      expect(find.byKey(const ValueKey('stats-row-b')), findsOneWidget);
      // Buyurtmasiz restoran ham nol bilan ko'rinadi.
      expect(find.byKey(const ValueKey('stats-row-c')), findsOneWidget);
      // O'chirilgan restoranlar alohida qator.
      expect(find.byKey(const ValueKey('stats-row-deleted')), findsOneWidget);
      expect(find.byKey(const ValueKey('stats-row-total')), findsOneWidget);
      expect(find.text('JAMI'), findsOneWidget);

      // Standart tartib: tushum kamayishi bo'yicha.
      expect(_tableOrder(tester), ['Book Cafe', 'Osh Markazi', 'O\'chirilgan restoranlar', 'Yangi Restoran']);

      // Bajarilish foizi (Book Cafe: 40 / (40 + 10) = 80%).
      expect(find.text('80%'), findsOneWidget);
      // Yakunlangan buyurtmasi yo'q restoranda "—".
      expect(find.text('—'), findsWidgets);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('jadval: ustun sarlavhasi bosilsa tartib o\'zgaradi; qidiruv JAMI ni yashiradi', (tester) async {
    await _run(tester, (requests) async {
      // "Jami" ustuni (buyurtmalar soni): birinchi bosishda o'sish tartibi.
      await tester.tap(find.descendant(of: find.byKey(const ValueKey('stats-table')), matching: find.text('Jami')));
      await tester.pumpAndSettle();
      expect(_tableOrder(tester).first, 'Yangi Restoran'); // eng kam buyurtma (0)
      // Yana bosilsa — kamayish tartibi: eng ko'p buyurtma birinchi.
      await tester.tap(find.descendant(of: find.byKey(const ValueKey('stats-table')), matching: find.text('Jami')));
      await tester.pumpAndSettle();
      expect(_tableOrder(tester).first, 'Book Cafe');

      // Restoran nomi bo'yicha (A→Z).
      await tester.tap(find.descendant(of: find.byKey(const ValueKey('stats-table')), matching: find.text('Restoran')));
      await tester.pumpAndSettle();
      expect(_tableOrder(tester).first, 'Book Cafe'); // A -> Z

      // Qidiruv: faqat mos qator; filtrlangan jadvalda JAMI ko'rsatilmaydi.
      await tester.enterText(find.byKey(const ValueKey('stats-search')), 'osh');
      await tester.pumpAndSettle();
      expect(_tableOrder(tester), ['Osh Markazi']);
      expect(find.byKey(const ValueKey('stats-row-total')), findsNothing);
      await tester.enterText(find.byKey(const ValueKey('stats-search')), 'yo\'q-bunaqasi');
      await tester.pumpAndSettle();
      expect(find.text('Topilmadi'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('davr almashtirilsa server qayta so\'raladi', (tester) async {
    await _run(tester, (requests) async {
      for (final p in ['today', '7d', 'all']) {
        await tester.tap(find.byKey(ValueKey('stats-period-$p')));
        await tester.pumpAndSettle();
        expect(requests.last.url.queryParameters['period'], p);
      }
      // "Hammasi" — sana oralig'i o'rniga "Butun davr".
      expect(find.textContaining('Butun davr'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('grafik: 14 ustun, kunlik qiymatga mutanosib; holatlar ro\'yxati', (tester) async {
    await _run(tester, (requests) async {
      for (var i = 0; i < 14; i++) {
        expect(find.byKey(ValueKey('stats-bar-2026-09-${(8 + i).toString().padLeft(2, '0')}')), findsOneWidget);
      }
      double h(String date) => tester.getSize(find.byKey(ValueKey('stats-bar-$date'))).height;
      // Bugun (eng katta) — eng baland; boshqa kunlar undan past.
      expect(h('2026-09-21'), greaterThan(h('2026-09-15')));
      expect(h('2026-09-15'), greaterThan(h('2026-09-09')));

      // Buyurtmalar rejimiga o'tkazilsa ustunlar qayta chiziladi (xatosiz).
      await tester.tap(find.text('Buyurtmalar').last);
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('stats-bar-2026-09-21')), findsOneWidget);

      // Holatlar: faqat soni bor holatlar, foizi bilan.
      expect(find.byKey(const ValueKey('stats-status-delivered')), findsOneWidget);
      expect(find.descendant(of: find.byKey(const ValueKey('stats-status-delivered')), matching: find.text('65')),
          findsOneWidget);
      expect(find.byKey(const ValueKey('stats-status-ready')), findsNothing);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('xato: qayta urinish tugmasi; keyin ma\'lumot chiqadi', (tester) async {
    var fail = true;
    await _run(tester, onStats: (r) => fail ? _json({'error': 'server yiqildi'}, 500) : _json(_payload('30d')),
        (requests) async {
      expect(find.textContaining('Statistikani yuklab bo\'lmadi'), findsOneWidget);
      fail = false;
      await tester.tap(find.text('Qayta urinish'));
      await tester.pumpAndSettle();
      expect(_kpi(tester, 'orders'), '95');
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('eski backend (totals yo\'q): yolg\'on nollar o\'rniga sabab ko\'rsatiladi', (tester) async {
    await _run(tester, onStats: (r) => _json({'orders_today': 3, 'revenue_today_tiyin': 100, 'by_status': {}}),
        (requests) async {
      expect(find.byKey(const ValueKey('stats-server-outdated')), findsOneWidget);
      expect(find.byKey(const ValueKey('stats-table')), findsNothing);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('bo\'sh platforma: nollar va "buyurtma yo\'q" yozuvlari, xatosiz', (tester) async {
    await _run(tester, onStats: (r) => _json({'period': '30d', 'from': '', 'to': '', 'totals': {}, 'restaurants': [], 'statuses': {}, 'daily': []}),
        (requests) async {
      expect(_kpi(tester, 'orders'), '0');
      expect(find.text('Hozircha restoran yo\'q'), findsOneWidget);
      expect(find.text('Bu davrda buyurtma yo\'q'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });

  testWidgets('tor oynada (1000 px) kartalar va jadval overflow bermaydi', (tester) async {
    await _run(tester, size: const Size(1000, 700), (requests) async {
      expect(tester.takeException(), isNull);
      expect(_kpi(tester, 'orders'), '95');
      expect(find.byKey(const ValueKey('stats-table')), findsOneWidget);
    });
  });
}
