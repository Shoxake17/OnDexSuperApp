import 'dart:convert';

import 'package:chust_restaurant/shop/pages/add_product_page.dart';
import 'package:chust_restaurant/shop/pages/products_page.dart';
import 'package:chust_restaurant/shop/store_pages.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared_preferences/shared_preferences.dart';

// "Mahsulotlar" va "Mahsulot qo'shish" (ShoxPos MaxsulotBazasi/MaxQoshish 1:1).
//
// Soxta server xotirada: mahsulotlar ro'yxati, so'rovlar jurnali.

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

class _Server {
  _Server(this.products, {this.global = const []});
  final List<Map<String, dynamic>> products;
  final List<Map<String, dynamic>> global;
  final calls = <String>[];
  final bodies = <String, Map<String, dynamic>>{};

  http.Response handle(http.Request r) {
    final path = r.url.path;
    calls.add('${r.method} $path');
    if (r.body.isNotEmpty) bodies['${r.method} $path'] = jsonDecode(r.body) as Map<String, dynamic>;
    if (path == '/merchant/shop/categories') {
      return _json({...products.map((p) => p['category'] as String)}.toList()..sort());
    }
    if (path == '/merchant/shop/products' && r.method == 'GET') return _json(products);
    if (path == '/merchant/shop/products/scan') {
      final bc = r.url.queryParameters['barcode'];
      final own = products.where((p) => p['barcode'] == bc).firstOrNull;
      if (own != null) return _json({...own, 'source': 'user'});
      return _json({'error': 'Mahsulot topilmadi'}, 404);
    }
    if (path == '/merchant/shop/products/search') {
      final q = r.url.queryParameters['q']!.toLowerCase();
      return _json([
        for (final g in global)
          if ((g['barcode'] as String).toLowerCase().contains(q)) {...g, 'is_global': true}
      ]);
    }
    if (path.startsWith('/merchant/shop/products/')) {
      final bc = Uri.decodeComponent(path.split('/').last);
      if (r.method == 'DELETE') {
        products.removeWhere((p) => p['barcode'] == bc);
        return _json({'deleted': true});
      }
      if (r.method == 'PATCH') {
        final b = jsonDecode(r.body) as Map;
        final p = products.firstWhere((p) => p['barcode'] == bc);
        p['price_tiyin'] = b['value'];
        return _json(p);
      }
      if (r.method == 'PUT') {
        final b = jsonDecode(r.body) as Map<String, dynamic>;
        products.removeWhere((p) => p['barcode'] == bc);
        products.add({...b, 'barcode': bc});
        return _json({...b, 'barcode': bc});
      }
    }
    return _json({'error': 'yo\'q'}, 404);
  }
}

Map<String, dynamic> _p(String bc, String name, String cat, int price, num qty, {Map<String, dynamic> extra = const {}}) =>
    {'barcode': bc, 'name': name, 'category': cat, 'price_tiyin': price, 'base_price_tiyin': 0, 'quantity': qty, ...extra};

StorePage? _went;
String? _wentArg;

StoreContext _ctx(String category, {String? arg}) => StoreContext(
    category: category,
    name: 'D',
    phone: '',
    arg: arg,
    go: (p, {arg}) {
      _went = p;
      _wentArg = arg;
    });

Future<void> _run(WidgetTester tester, _Server s, Widget page, Future<void> Function() body,
    {Size size = const Size(1400, 1000)}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  _went = null;
  _wentArg = null;
  await http.runWithClient(() async {
    await tester.pumpWidget(MaterialApp(home: Scaffold(body: page)));
    await tester.pumpAndSettle();
    await body();
    await tester.pumpWidget(const SizedBox());
  }, () => MockClient((r) async => s.handle(r)));
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets('ro\'yxat, qidiruv, narxni tiyinda saqlash, o\'chirish', (tester) async {
    final s = _Server([
      _p('4780001', 'Non', 'Nonvoylik mahsulotlari', 500000, 10),
      _p('4780002', 'Sut', 'Sut mahsulotlari', 1100000, 0),
    ]);
    await _run(tester, s, ProductsPage(ctx: _ctx('store')), () async {
      expect(find.text('Non'), findsOneWidget);
      expect(find.text('Sut'), findsOneWidget);

      await tester.enterText(find.byType(TextField).first, 'sut');
      await tester.pumpAndSettle();
      expect(find.text('Non'), findsNothing);
      await tester.enterText(find.byType(TextField).first, '');
      await tester.pumpAndSettle();

      // Narx: 6000 so'm -> 600000 tiyin, Enter bilan bir marta.
      final price = find.byKey(const ValueKey('price-4780001-500000'));
      await tester.enterText(find.descendant(of: price, matching: find.byType(TextField)), '6000');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      expect(s.bodies['PATCH /merchant/shop/products/4780001'], {'field': 'price', 'value': 600000});

      await tester.tap(find.byKey(const ValueKey('delete-4780002')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('O\'chirish').last);
      await tester.pumpAndSettle();
      expect(s.calls, contains('DELETE /merchant/shop/products/4780002'));
      expect(find.text('Sut'), findsNothing);

      await tester.tap(find.byKey(const ValueKey('products-add')));
      expect(_went, StorePage.addProduct);
    });
  });

  testWidgets('tor ekranda kartalar (overflow yo\'q)', (tester) async {
    final s = _Server([_p('4780001', 'Juda uzun nomli mahsulot nomi sinov uchun', 'Sut mahsulotlari', 500000, 3)]);
    await _run(tester, s, ProductsPage(ctx: _ctx('store')), () async {
      expect(find.text('3 dona'), findsOneWidget);
    }, size: const Size(420, 900));
  });

  testWidgets('qurt: holat va tahrirga o\'tish', (tester) async {
    final s = _Server([
      _p('QURT-1', 'Sho\'r qurt', 'Sho\'r', 1500000, 5, extra: {'weight_grams': 250}),
      _p('QURT-2', 'Shirin qurt', 'Shirin', 1500000, 0, extra: {'weight_grams': 250}),
    ]);
    await _run(tester, s, ProductsPage(ctx: _ctx('qurt')), () async {
      expect(find.text('Mavjud'), findsOneWidget);
      expect(find.text('Mavjud emas'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('edit-QURT-1')));
      expect(_went, StorePage.addProduct);
      expect(_wentArg, 'QURT-1');
    });
  });

  testWidgets('qo\'shish: shtrix-kod Enter -> global bazadan to\'ldiriladi, tiyinda saqlanadi', (tester) async {
    final s = _Server([_p('4780001', 'Non', 'Nonvoylik mahsulotlari', 500000, 10)],
        global: [_p('5449000000996', 'Coca-Cola 0.33L', 'Ichimliklar', 700000, 0)]);
    await _run(tester, s, AddProductPage(ctx: _ctx('store')), () async {
      await tester.enterText(find.byKey(const ValueKey('add-barcode')), '5449000000996');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      expect(find.textContaining('Global bazada topildi'), findsOneWidget);
      expect(find.text('Coca-Cola 0.33L'), findsOneWidget);

      await tester.enterText(find.byKey(const ValueKey('add-base')), '5000');
      await tester.enterText(find.byKey(const ValueKey('add-price')), '7500');
      await tester.tap(find.byKey(const ValueKey('qty-plus')));
      await tester.tap(find.byKey(const ValueKey('add-save')));
      await tester.pumpAndSettle(const Duration(seconds: 2));
      final b = s.bodies['PUT /merchant/shop/products/5449000000996']!;
      expect(b['price_tiyin'], 750000);
      expect(b['base_price_tiyin'], 500000);
      expect(b['quantity'], 2);
      expect(b['category'], 'Ichimliklar');
      expect(b.containsKey('weight_grams'), isFalse);
      expect(_went, StorePage.products);
    });
  });

  testWidgets('qo\'shish: ulgurji > sotuv va takror shtrix-kod rad etiladi', (tester) async {
    final s = _Server([_p('4780001', 'Non', 'Nonvoylik mahsulotlari', 500000, 10)]);
    await _run(tester, s, AddProductPage(ctx: _ctx('store')), () async {
      await tester.enterText(find.byKey(const ValueKey('add-barcode')), '4780001');
      await tester.enterText(find.byKey(const ValueKey('add-name')), 'Non 2');
      await tester.enterText(find.byKey(const ValueKey('add-base')), '9000');
      await tester.enterText(find.byKey(const ValueKey('add-price')), '5000');
      await tester.pump();
      expect(find.textContaining('Ulgurji narx sotuv narxidan yuqori'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('add-save')));
      await tester.pumpAndSettle();
      expect(s.calls.where((c) => c.startsWith('PUT')), isEmpty);

      await tester.enterText(find.byKey(const ValueKey('add-base')), '4000');
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('add-save')));
      await tester.pumpAndSettle();
      expect(find.textContaining('allaqachon bor'), findsOneWidget);
      expect(s.calls.where((c) => c.startsWith('PUT')), isEmpty);
    });
  });

  testWidgets('qurt: rasmsiz saqlanmaydi; tahrirda "Mavjud emas" = 0 dona, gramm yuboriladi', (tester) async {
    final s = _Server([
      _p('QURT-1', 'Sho\'r qurt', 'Sho\'r', 1500000, 5,
          extra: {'weight_grams': 250, 'image_url': '/uploads/q.png', 'description': 'd', 'composition': 'c'}),
    ]);
    // Yangi qurt: rasm yo'q -> xato.
    await _run(tester, s, AddProductPage(ctx: _ctx('qurt')), () async {
      expect(find.byKey(const ValueKey('add-barcode')), findsNothing); // qurtda shtrix-kod yo'q
      await tester.enterText(find.byKey(const ValueKey('add-name')), 'Yangi qurt');
      await tester.enterText(find.byKey(const ValueKey('add-price')), '15000');
      await tester.enterText(find.byKey(const ValueKey('add-weight')), '200');
      await tester.enterText(find.byKey(const ValueKey('qurt-new-category')), 'Sho\'r');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('add-save')));
      await tester.pumpAndSettle();
      expect(find.text('Mahsulot rasmini tanlang'), findsOneWidget);
    });
    // Tahrir: mavjud emas -> quantity 0.
    await _run(tester, s, AddProductPage(ctx: _ctx('qurt', arg: 'QURT-1')), () async {
      expect(find.text('Sho\'r qurt'), findsOneWidget);
      await tester.tap(find.text('Mavjud emas'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('add-save')));
      await tester.pumpAndSettle(const Duration(seconds: 2));
      final b = s.bodies['PUT /merchant/shop/products/QURT-1']!;
      expect(b['quantity'], 0);
      expect(b['weight_grams'], 250);
      expect(b['image_url'], '/uploads/q.png');
      expect(b['category'], 'Sho\'r');
    }, size: const Size(1400, 1400));
  });
}
