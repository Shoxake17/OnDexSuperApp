import 'dart:convert';

import 'package:chust_restaurant/shop/pages/debts_page.dart';
import 'package:chust_restaurant/shop/pages/incoming_page.dart';
import 'package:chust_restaurant/shop/pages/reports_page.dart';
import 'package:chust_restaurant/shop/pages/sales_page.dart';
import 'package:chust_restaurant/shop/store_pages.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// Sotuvlar, Qarzlar, Savdo statistikasi, Kirim bazasi (ShoxPos 1:1).

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

final _now = DateTime(2026, 11, 28, 15);
String _iso(DateTime d) => d.toUtc().toIso8601String();
final _ctx = StoreContext(category: 'store', name: 'D', phone: '', go: (p, {arg}) {});
final _bodies = <String, Object?>{};

Future<void> _run(WidgetTester tester, Widget page, http.Response Function(http.Request) h,
    Future<void> Function() body, {Size size = const Size(1500, 1200)}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  _bodies.clear();
  await http.runWithClient(() async {
    await tester.pumpWidget(MaterialApp(home: Scaffold(body: page)));
    await tester.pumpAndSettle();
    await body();
    await tester.pumpWidget(const SizedBox());
  }, () => MockClient((r) async {
        if (r.body.isNotEmpty) _bodies['${r.method} ${r.url.path}'] = jsonDecode(r.body);
        return h(r);
      }));
}

Map<String, dynamic> _line(String bc, String name, num qty, int price, DateTime at, String pay,
        {int base = 0, String cat = 'Sut'}) =>
    {
      'id': '$bc-${at.millisecondsSinceEpoch}-$pay',
      'barcode': bc,
      'name': name,
      'category': cat,
      'quantity': qty,
      'price_tiyin': price,
      'base_price_tiyin': base,
      'total_tiyin': (price * qty).round(),
      'payment_method': pay,
      'sold_at': _iso(at),
    };

void main() {
  testWidgets('Sotuvlar: kartalar va davr filtri', (tester) async {
    final lines = [
      _line('a', 'Non', 2, 500000, _now.subtract(const Duration(hours: 1)), 'naqd'),
      _line('b', 'Sut', 1, 1100000, _now.subtract(const Duration(days: 3)), 'plastik'),
      _line('a', 'Non', 1, 500000, _now.subtract(const Duration(days: 20)), 'qarz'),
    ];
    await _run(tester, SalesPage(ctx: _ctx, now: () => _now), (r) => _json(lines), () async {
      expect(find.text('26 000 so\'m'), findsOneWidget); // 10 000 + 11 000 + 5 000 (Umumiy summa)
      expect(find.text('Qarz'), findsOneWidget);
      await tester.tap(find.text('Kunlik'));
      await tester.pumpAndSettle();
      expect(find.text('Sut'), findsNothing);
      await tester.tap(find.text('Haftalik'));
      await tester.pumpAndSettle();
      expect(find.text('Sut'), findsOneWidget);
      expect(find.text('21 000 so\'m'), findsOneWidget);
    });
  });

  testWidgets('Sotuvlar: tor ekranda kunlar bo\'yicha guruh', (tester) async {
    final lines = [
      _line('a', 'Non', 2, 500000, _now.subtract(const Duration(hours: 1)), 'naqd'),
      _line('b', 'Sut', 1, 1100000, _now.subtract(const Duration(days: 1)), 'plastik'),
    ];
    await _run(tester, SalesPage(ctx: _ctx, now: () => _now), (r) => _json(lines), () async {
      expect(find.textContaining('Bugun — 28.11.2026'), findsOneWidget);
      expect(find.textContaining('Kecha — 27.11.2026'), findsOneWidget);
    }, size: const Size(420, 1200));
  });

  testWidgets('Qarzlar: qo\'shish (+ bilan), qisman va to\'liq to\'lov', (tester) async {
    var debts = <Map<String, dynamic>>[
      {'id': 'd1', 'name': 'Ali', 'phone': '+998901112233', 'amount_tiyin': 2000000, 'last_payment': _iso(_now)},
    ];
    await _run(tester, DebtsPage(ctx: _ctx), (r) {
      if (r.url.path == '/merchant/shop/debts' && r.method == 'GET') return _json(debts);
      if (r.url.path == '/merchant/shop/debts' && r.method == 'POST') {
        final b = jsonDecode(r.body) as Map;
        return _json({'id': 'd2', ...b, 'last_payment': _iso(_now)}, 201);
      }
      if (r.url.path == '/merchant/shop/debts/d1/pay') {
        final paid = (jsonDecode(r.body) as Map)['paid_tiyin'] as int;
        final left = 2000000 - paid;
        if (left <= 0) return _json({'fully_paid': true, 'debt': null});
        return _json({'fully_paid': false, 'debt': {...debts.first, 'amount_tiyin': left}});
      }
      return _json({}, 404);
    }, () async {
      expect(find.text('20 000 so\'m'), findsWidgets);
      await tester.enterText(find.byKey(const ValueKey('debt-name')), 'Vali');
      await tester.enterText(find.byKey(const ValueKey('debt-phone')), '998900000000');
      await tester.enterText(find.byKey(const ValueKey('debt-amount')), '5000');
      await tester.tap(find.byKey(const ValueKey('debt-add')));
      await tester.pumpAndSettle();
      final add = _bodies['POST /merchant/shop/debts'] as Map;
      expect(add['phone'], '+998900000000');
      expect(add['amount_tiyin'], 500000);
      expect(find.text('Vali'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('pay-d1')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const ValueKey('pay-amount')), '5000');
      await tester.tap(find.byKey(const ValueKey('pay-confirm')));
      await tester.pumpAndSettle();
      expect(find.textContaining('Qoldiq: 15 000 so\'m'), findsOneWidget);
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();

      debts = [];
      await tester.tap(find.byKey(const ValueKey('pay-d1')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const ValueKey('pay-amount')), '20000');
      await tester.tap(find.byKey(const ValueKey('pay-confirm')));
      await tester.pumpAndSettle();
      expect(find.text('Qarz to\'liq to\'landi!'), findsOneWidget);
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();
      expect(find.text('Ali'), findsNothing);
    });
  });

  testWidgets('Savdo statistikasi: daromad, turlar, top ro\'yxatlar', (tester) async {
    final sales = [
      _line('a', 'Non', 3, 500000, _now, 'naqd', cat: 'Nonvoylik mahsulotlari'),
      _line('b', 'Sut', 1, 1100000, _now.subtract(const Duration(days: 2)), 'naqd'),
      _line('a', 'Non', 1, 500000, _now.subtract(const Duration(days: 40)), 'naqd', cat: 'Nonvoylik mahsulotlari'),
    ];
    await _run(tester, ReportsPage(ctx: _ctx, now: () => _now), (r) => _json(sales), () async {
      expect(find.text('31 000 so\'m'), findsWidgets); // jami daromad
      expect(find.text('Top 10 mahsulot (dona)'), findsOneWidget);
      await tester.tap(find.text('Bugun'));
      await tester.pumpAndSettle();
      expect(find.text('15 000 so\'m'), findsWidgets);
    });
  });

  testWidgets('Kirim bazasi: kirim/chiqim/daromad va ulgurji narxni tahrirlash', (tester) async {
    final products = [
      {'barcode': '4780001', 'name': 'Non', 'category': 'Non', 'price_tiyin': 500000, 'base_price_tiyin': 400000, 'quantity': 10},
    ];
    final sales = [_line('4780001', 'Non', 2, 500000, _now, 'naqd', base: 400000)];
    await _run(tester, IncomingPage(ctx: _ctx), (r) {
      if (r.url.path == '/merchant/shop/products' && r.method == 'GET') return _json(products);
      if (r.url.path == '/merchant/shop/sales') return _json(sales);
      if (r.url.path == '/merchant/shop/categories') return _json(['Non']);
      if (r.method == 'PATCH') return _json(products.first);
      return _json({}, 404);
    }, () async {
      expect(find.text('10 000 so\'m'), findsOneWidget); // umumiy kirim
      expect(find.text('8 000 so\'m'), findsOneWidget); // umumiy chiqim
      expect(find.text('2 000 so\'m'), findsOneWidget); // daromad
      await tester.enterText(
          find.descendant(of: find.byKey(const ValueKey('base-4780001')), matching: find.byType(TextField)), '4500');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      expect(_bodies['PATCH /merchant/shop/products/4780001'], {'field': 'base_price', 'value': 450000});
    });
  });
}
