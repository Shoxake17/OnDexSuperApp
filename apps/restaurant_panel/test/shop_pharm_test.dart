import 'dart:convert';

import 'package:chust_restaurant/shop/pages/logistics_page.dart';
import 'package:chust_restaurant/shop/pages/pharm_add_page.dart';
import 'package:chust_restaurant/shop/pages/pharm_base_page.dart';
import 'package:chust_restaurant/shop/pages/pharm_info_page.dart';
import 'package:chust_restaurant/shop/pages/scanner_page.dart';
import 'package:chust_restaurant/shop/store_pages.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// Dorixona sahifalari va qurt Logistika (ShoxPos 1:1).

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

final _bodies = <String, Object?>{};
StorePage? _went;

StoreContext _ctx(String cat) => StoreContext(category: cat, name: 'Shifo', phone: '', go: (p, {arg}) => _went = p);

Future<void> _run(WidgetTester tester, Widget page, http.Response Function(http.Request) h, Future<void> Function() body) async {
  tester.view.physicalSize = const Size(1500, 1300);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  _bodies.clear();
  _went = null;
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

final _drug = {
  'barcode': 'd1',
  'name': 'Paratsetamol',
  'category': 'Og\'riq',
  'price_tiyin': 300000,
  'base_price_tiyin': 200000,
  'quantity': 8,
  'dosage': '500mg',
  'manufacturer': 'Nika',
  'expiry_date': '2027-06-01T00:00:00Z',
  'batch_number': 'B-17',
  'prescription_required': false,
  'shelf_number': 'A1',
  'row_number': '2',
  'description': '',
};

void main() {
  testWidgets('Dori qo\'shish: Enter bilan to\'ldiriladi, tiyinda yuboriladi', (tester) async {
    await _run(tester, PharmAddPage(ctx: _ctx('pharmacy')), (r) {
      if (r.url.path == '/merchant/shop/pharm/search') return _json([{..._drug, 'is_global': true}]);
      if (r.url.path == '/merchant/shop/pharm') return _json(_drug);
      return _json({}, 404);
    }, () async {
      await tester.enterText(find.byKey(const ValueKey('pharm-barcode')), 'd1');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      expect(find.text('Topildi: Paratsetamol'), findsOneWidget);
      await tester.enterText(find.byKey(const ValueKey('pharm-quantity')), '3');
      await tester.tap(find.byKey(const ValueKey('pharm-save')));
      await tester.pumpAndSettle(const Duration(seconds: 2));
      final b = _bodies['POST /merchant/shop/pharm'] as Map;
      expect(b['price_tiyin'], 300000);
      expect(b['base_price_tiyin'], 200000);
      expect(b['quantity'], 3);
      expect(b['dosage'], '500mg');
      expect(_went, StorePage.pharmBase);
    });
  });

  testWidgets('Dorixona kassasi: sell-pharm va Sotuvlarga o\'tish; 409 xabari', (tester) async {
    var fail = true;
    await _run(tester, ScannerPage(ctx: _ctx('pharmacy')), (r) {
      if (r.url.path == '/merchant/shop/pharm' && r.method == 'GET') return _json([_drug]);
      if (r.url.path == '/merchant/shop/pharm/sales') {
        if (fail) {
          fail = false;
          return _json({'error': 'qoldiq yetarli emas: Paratsetamol yetarli emas'}, 409);
        }
        return _json({'id': 's1', 'items': [], 'total_tiyin': 600000, 'payment_method': 'plastik'}, 201);
      }
      return _json([]);
    }, () async {
      await tester.enterText(find.byType(TextField).first, 'd1');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField).first, 'd1');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('pay-plastik')));
      await tester.pumpAndSettle();
      expect(find.textContaining('yetarli emas'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('pay-plastik')));
      await tester.pumpAndSettle(const Duration(seconds: 2));
      final b = _bodies['POST /merchant/shop/pharm/sales'] as Map;
      expect(b['payment_method'], 'plastik');
      expect(((b['items'] as List).single as Map)['quantity'], 2);
      expect(_went, StorePage.pharmSales);
    });
  });

  testWidgets('Dori ma\'lumoti: qidirish, tahrirlash, muddatni tozalash', (tester) async {
    await _run(tester, PharmInfoPage(ctx: _ctx('pharmacy')), (r) {
      if (r.url.path == '/merchant/shop/pharm/info') return _json(_drug);
      if (r.method == 'PATCH') return _json({..._drug, ...(jsonDecode(r.body) as Map), 'expiry_date': null, 'price_tiyin': 350000});
      return _json({}, 404);
    }, () async {
      await tester.enterText(find.byType(TextField).first, 'B-17');
      await tester.tap(find.byKey(const ValueKey('info-search')));
      await tester.pumpAndSettle();
      expect(find.text('Paratsetamol'), findsOneWidget);
      expect(find.text('01.06.2027'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('info-edit')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const ValueKey('info-price')), '3500');
      await tester.tap(find.byTooltip('Tozalash'));
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('info-save')));
      await tester.pumpAndSettle();
      final b = _bodies['PATCH /merchant/shop/pharm/d1'] as Map;
      expect(b['price_tiyin'], 350000);
      expect(b.containsKey('expiry_date'), isTrue);
      expect(b['expiry_date'], isNull);
      expect(find.text('Ma\'lumotlar saqlandi!'), findsOneWidget);
    });
  });

  testWidgets('Dorilar bazasi: narx tahriri va o\'chirish', (tester) async {
    await _run(tester, PharmBasePage(ctx: _ctx('pharmacy')), (r) {
      if (r.url.path == '/merchant/shop/pharm' && r.method == 'GET') return _json([_drug]);
      return _json(_drug);
    }, () async {
      await tester.enterText(
          find.descendant(of: find.byKey(const ValueKey('pharm-price-d1')), matching: find.byType(TextField)), '3200');
      await tester.testTextInput.receiveAction(TextInputAction.done);
      await tester.pumpAndSettle();
      expect(_bodies['PATCH /merchant/shop/pharm/d1'], {'price_tiyin': 320000});
      await tester.tap(find.byKey(const ValueKey('pharm-delete-d1')));
      await tester.pumpAndSettle();
      await tester.tap(find.text('O\'chirish').last);
      await tester.pumpAndSettle();
      expect(find.text('Paratsetamol'), findsNothing);
    });
  });

  testWidgets('Logistika: reysni kuryerga biriktirish', (tester) async {
    await _run(tester, LogisticsPage(ctx: _ctx('qurt')), (r) {
      if (r.url.path == '/merchant/qurt/orders') {
        return _json({
          'orders': [
            {'id': 'o1', 'number': 'Q-1', 'status': 'confirmed', 'delivery_date': '2026-11-30', 'total_tiyin': 3000000, 'items': []},
          ]
        });
      }
      if (r.url.path == '/merchant/qurt/couriers') {
        return _json({
          'couriers': [
            {'id': 'c1', 'name': 'Bobur', 'phone': '+998901112233', 'summary': {'held_tiyin': 0, 'held_orders': 0, 'active_orders': 0}}
          ]
        });
      }
      if (r.url.path == '/merchant/qurt/assign') return _json({'assigned': 1, 'skipped': 0});
      return _json({}, 404);
    }, () async {
      expect(find.text('Reys: 30-noyabr, dushanba'), findsOneWidget);
      await tester.tap(find.byKey(const ValueKey('assign-trip-2026-11-30')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('pick-c1')));
      await tester.pumpAndSettle();
      expect(_bodies['POST /merchant/qurt/assign'], {'courier_id': 'c1', 'date': '2026-11-30'});
      expect(find.text('1 ta buyurtma Bobur ga biriktirildi'), findsOneWidget);
    });
  });
}
