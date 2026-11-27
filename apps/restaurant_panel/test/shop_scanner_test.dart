import 'dart:convert';

import 'package:chust_restaurant/shop/pages/scanner_page.dart';
import 'package:chust_restaurant/shop/store_pages.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// "Savdo terminali" (ShoxPos Scanner.tsx 1:1).

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

final _products = <Map<String, dynamic>>[];
final _bodies = <String, Object?>{};
StorePage? _went;

http.Response _handle(http.Request r) {
  final path = r.url.path;
  if (r.body.isNotEmpty) _bodies['${r.method} $path'] = jsonDecode(r.body);
  if (path == '/merchant/shop/products' && r.method == 'GET') return _json(_products);
  if (path == '/merchant/shop/products/search') {
    final q = r.url.queryParameters['q'];
    if (q == '5449000000996') {
      return _json([
        {'barcode': '5449000000996', 'name': 'Coca-Cola', 'category': 'Ichimliklar', 'price_tiyin': 0, 'is_global': true}
      ]);
    }
    return _json([]);
  }
  if (path.startsWith('/merchant/shop/products/') && r.method == 'PUT') return _json(jsonDecode(r.body) as Object);
  if (path == '/merchant/shop/sales' && r.method == 'POST') return _json({'sales': []}, 201);
  return _json({'error': 'yo\'q'}, 404);
}

Future<void> _run(WidgetTester tester, Future<void> Function(ScannerPageState s) body) async {
  tester.view.physicalSize = const Size(1500, 1000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  _went = null;
  _bodies.clear();
  _products
    ..clear()
    ..addAll([
      {'barcode': '4780001', 'name': 'Non', 'category': 'Non', 'price_tiyin': 500000, 'quantity': 2},
      {'barcode': '4780002', 'name': 'Sut', 'category': 'Sut', 'price_tiyin': 1100000, 'quantity': 10},
    ]);
  await http.runWithClient(() async {
    await tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: ScannerPage(
            ctx: StoreContext(category: 'store', name: 'Baraka', phone: '', go: (p, {arg}) => _went = p)),
      ),
    ));
    await tester.pumpAndSettle();
    await body(tester.state<ScannerPageState>(find.byType(ScannerPage)));
    await tester.pumpWidget(const SizedBox());
  }, () => MockClient((r) async => _handle(r)));
}

Future<void> _scan(WidgetTester tester, String code) async {
  await tester.enterText(find.byType(TextField).first, code);
  await tester.testTextInput.receiveAction(TextInputAction.done);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('skaner -> savat, qoldiq chegarasi, nom bo\'yicha qidiruv', (tester) async {
    await _run(tester, (s) async {
      await _scan(tester, '4780001');
      await _scan(tester, '4780001');
      expect(s.cart.single.count, 2);
      await _scan(tester, '4780001'); // qoldiq 2 ta
      expect(find.text('Omborda yetarli emas'), findsOneWidget);
      expect(s.cart.single.count, 2);

      await _scan(tester, 'sut'); // raqamli emas — nom bo'yicha
      expect(s.cart.length, 2);
      expect(s.totalTiyin, 2 * 500000 + 1100000);

      await _scan(tester, '9999999');
      expect(find.text('Mahsulot topilmadi'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('minus-4780002')));
      await tester.pump();
      expect(s.cart.length, 1);
    });
  });

  testWidgets('global mahsulot: narx so\'raladi va do\'konga yoziladi', (tester) async {
    await _run(tester, (s) async {
      await _scan(tester, '5449000000996');
      expect(find.byKey(const ValueKey('ask-price')), findsOneWidget);
      await tester.enterText(find.byKey(const ValueKey('ask-price')), '7000');
      await tester.tap(find.text('Qo\'shish'));
      await tester.pumpAndSettle();
      final put = _bodies['PUT /merchant/shop/products/5449000000996'] as Map;
      expect(put['price_tiyin'], 700000);
      expect(put['quantity'], 1);
      expect(s.cart.single.global, isTrue);
      expect(s.cart.single.priceTiyin, 700000);
    });
  });

  testWidgets('F9 — naqd sotuv: tiyinda, keyin Sotuvlar sahifasi', (tester) async {
    await _run(tester, (s) async {
      await _scan(tester, '4780002');
      await _scan(tester, '4780002');
      await tester.sendKeyEvent(LogicalKeyboardKey.f9);
      await tester.pumpAndSettle(const Duration(seconds: 2));
      final body = _bodies['POST /merchant/shop/sales'] as Map;
      expect(body['payment_method'], 'naqd');
      final item = (body['items'] as List).single as Map;
      expect(item['quantity'], 2);
      expect(item['price_tiyin'], 1100000);
      expect(item['total_tiyin'], 2200000);
      expect(s.cart, isEmpty);
      expect(_went, StorePage.sales);
    });
  });

  testWidgets('qarzga sotish tugmasi va bo\'sh savat', (tester) async {
    await _run(tester, (s) async {
      await tester.tap(find.byKey(const ValueKey('pay-qarz')));
      await tester.pumpAndSettle();
      expect(find.text('Savat bo\'sh'), findsOneWidget);
      await _scan(tester, '4780001');
      await tester.tap(find.byKey(const ValueKey('pay-qarz')));
      await tester.pumpAndSettle(const Duration(seconds: 2));
      expect((_bodies['POST /merchant/shop/sales'] as Map)['payment_method'], 'qarz');
    });
  });

  test('chek: ShoxPos ko\'rinishi', () {
    final r = buildPosReceipt('Baraka', [
      CartLine(barcode: 'a', name: 'Non', priceTiyin: 500000, count: 2, category: '', max: 5, global: false),
      CartLine(barcode: 'b', name: 'Olma', priceTiyin: 1200000, count: 1.5, category: '', max: 5, global: false),
    ], DateTime(2026, 11, 28, 9, 5));
    expect(r.header, 'Baraka');
    expect(r.items.first.qty, 2);
    expect(r.items.last.name, 'Olma (1.5)');
    expect(r.totals.single.label, 'JAMI');
    expect(r.meta.single, 'Sana: 28.11.2026 09:05');
  });
}
