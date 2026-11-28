import 'dart:convert';

import 'package:chust_restaurant/shop/pages/qurt_orders_page.dart';
import 'package:chust_restaurant/shop/store_pages.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

// Qurt "Buyurtmalar" (ShoxPos Orders.tsx 1:1).

http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
      utf8.encode(jsonEncode(body)),
      status,
      headers: {'content-type': 'application/json; charset=utf-8'},
    );

Map<String, dynamic> _order(String id, String number, String status, {int qty = 2}) => {
      'id': id,
      'number': number,
      'status': status,
      'delivery_date': '2026-11-30',
      'customer_name': 'Ali',
      'payment_method': 'cash',
      'total_tiyin': 1500000 * qty,
      'created_at': '2026-11-28T10:00:00Z',
      'items': [
        {'barcode': 'q1', 'name': 'Sho\'r qurt', 'qty': qty, 'price_tiyin': 1500000, 'weight_grams': 250}
      ],
      'history': [
        {'to': 'new', 'at': '2026-11-28T10:00:00Z'}
      ],
    };

void main() {
  testWidgets('qabul qilish, ombor yetmasa xabar, sababli rad etish, qadoqlash', (tester) async {
    tester.view.physicalSize = const Size(1500, 1100);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final orders = [_order('o1', 'Q-1001', 'new'), _order('o2', 'Q-1002', 'new', qty: 9)];
    final posted = <String, Map>{};
    await http.runWithClient(() async {
      await tester.pumpWidget(MaterialApp(
          home: Scaffold(
              body: QurtOrdersPage(ctx: StoreContext(category: 'qurt', name: 'Q', phone: '', go: (p, {arg}) {})))));
      await tester.pumpAndSettle();
      expect(find.text('Yangi 2'), findsOneWidget);

      // o1 tanlangan (eskisi birinchi) -> qabul qilish.
      await tester.tap(find.byKey(const ValueKey('order-confirm')));
      await tester.pumpAndSettle();
      expect(posted['o1'], {'to': 'confirmed', 'reason': ''});
      expect(find.text('Q-1001 qabul qilindi — tovar ombordan ayirildi'), findsOneWidget);

      // o2 — omborda yetarli emas (server 409 xabari ko'rsatiladi).
      await tester.tap(find.byKey(const ValueKey('order-o2')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('order-confirm')));
      await tester.pumpAndSettle();
      expect(find.textContaining('omborda yetarli emas'), findsOneWidget);

      // Rad etish: 3 belgidan kam — tugma o'chiq.
      await tester.tap(find.byKey(const ValueKey('order-reject')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const ValueKey('reject-reason')), 'ab');
      await tester.pump();
      expect(tester.widget<FilledButton>(find.byKey(const ValueKey('reject-submit'))).onPressed, isNull);
      await tester.enterText(find.byKey(const ValueKey('reject-reason')), 'Qoldiq tugagan');
      await tester.pump();
      await tester.tap(find.byKey(const ValueKey('reject-submit')));
      await tester.pumpAndSettle();
      expect(posted['o2'], {'to': 'rejected', 'reason': 'Qoldiq tugagan'});

      // Jarayonda -> Qadoqlandi.
      await tester.tap(find.text('Jarayonda 1'));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const ValueKey('order-packed')));
      await tester.pumpAndSettle();
      expect(posted['o1'], {'to': 'packed', 'reason': ''});
      await tester.pumpWidget(const SizedBox());
    }, () => MockClient((r) async {
          if (r.url.path == '/merchant/qurt/orders') return _json({'orders': orders});
          final m = RegExp(r'/merchant/qurt/orders/(\w+)/transition').firstMatch(r.url.path);
          if (m != null) {
            final id = m.group(1)!;
            final b = jsonDecode(r.body) as Map;
            posted[id] = b;
            if (id == 'o2' && b['to'] == 'confirmed') {
              return _json({'error': 'qoldiq yetarli emas: «Sho\'r qurt» omborda yetarli emas (bor: 5, kerak: 9)'}, 409);
            }
            final i = orders.indexWhere((o) => o['id'] == id);
            orders[i] = {...orders[i], 'status': b['to']};
            return _json(orders[i]);
          }
          return _json({}, 404);
        }));
  });

  test('reys sanasi o\'zbekcha', () {
    expect(deliveryDateLabel('2026-11-30'), '30-noyabr, dushanba');
  });
}
