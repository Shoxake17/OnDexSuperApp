import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';

import 'package:chust_customer/api.dart';
import 'package:chust_customer/data/address_scope.dart';
import 'package:chust_customer/data/qurt_cart.dart';
import 'package:chust_customer/screens/qurt_cart_screen.dart';
import 'package:chust_customer/screens/qurt_orders_screen.dart';

// Qurt buyurtmasi (mijoz tomoni): savat qoidalari, rasmiylashtirish xavfsizligi,
// xatolarni qayta ishlash, buyurtmalar ro'yxati va bekor qilish. Tarmoq soxta.

http.Response _json(Object body, [int status = 200]) => http.Response(
  jsonEncode(body),
  status,
  headers: {'content-type': 'application/json; charset=utf-8'},
);

class _MemStore implements CacheStore {
  final data = <String, String>{};
  @override
  Future<String?> read(String key) async => data[key];
  @override
  Future<void> write(String key, String json) async => data[key] = json;
  @override
  Future<void> remove(String key) async => data.remove(key);
  @override
  Future<void> clearAll() async => data.clear();
}

Map<String, dynamic> _prod(
  String id, {
  int price = 1200000,
  int left = 10,
  String name = 'Qurt',
}) => {
  'id': id,
  'name': '$name $id',
  'store_name': 'Urgut qurtlari',
  'price_tiyin': price,
  'weight_grams': 200,
  'image_url': '',
  'stock_left': left,
  'available': true,
};

Map<String, dynamic> _order({
  String id = 'o1',
  String status = 'new',
  String number = 'Q-AAAA0001',
}) => {
  'id': id,
  'number': number,
  'group_id': 'g1',
  'store_name': 'Urgut qurtlari',
  'status': status,
  'delivery_date': '2026-11-03',
  'total_tiyin': 2400000,
  'items': [
    {'product_id': 'a', 'name': 'Qurt a', 'qty': 2, 'price_tiyin': 1200000},
  ],
  'address': {'text': 'Chust, Navoiy 5'},
  'history': [
    {'from': '', 'to': 'new', 'at': '2026-11-02T10:00:00Z'},
  ],
};

void main() {
  group('QurtCart', () {
    test('qoldiq va chegaradan oshirib bo\'lmaydi', () {
      final c = QurtCart.forTest(store: _MemStore());
      expect(c.add(_prod('a', left: 2)), isTrue);
      expect(c.add(_prod('a', left: 2)), isTrue);
      expect(c.add(_prod('a', left: 2)), isFalse); // 3-dona qolmagan
      expect(c.qtyOf('a'), 2);
      expect(c.add(_prod('z', left: 0)), isFalse); // tugagan
      expect(c.qtyOf('z'), 0);
      c.setQty('a', 50);
      expect(c.qtyOf('a'), 2, reason: 'setQty ham qoldiqqa qirqiladi');
      expect(c.add(_prod('big', left: 1000), qty: 101), isFalse);
      c.setQty('a', 0);
      expect(c.isEmpty, isTrue);
    });

    test('jami va serverga faqat ID+miqdor yuboriladi', () {
      final c = QurtCart.forTest(store: _MemStore());
      c.add(_prod('a', price: 1000000), qty: 2);
      c.add(_prod('b', price: 500000));
      expect(c.totalQty, 3);
      expect(c.totalTiyin, 2500000);
      expect(c.toOrderItems(), [
        {'product_id': 'a', 'qty': 2},
        {'product_id': 'b', 'qty': 1},
      ]);
    });

    test(
      'diskka saqlanadi va qayta o\'qiladi; buzilgan yozuv savatni buzmaydi',
      () async {
        final store = _MemStore();
        final c = QurtCart.forTest(store: store);
        c.add(_prod('a'), qty: 2);
        await Future<void>.delayed(Duration.zero);
        final c2 = QurtCart.forTest(store: store);
        await c2.ensureRestored();
        expect(c2.qtyOf('a'), 2);

        final bad = _MemStore()..data['qurt_cart'] = '{buzilgan';
        final c3 = QurtCart.forTest(store: bad);
        await c3.ensureRestored();
        expect(c3.isEmpty, isTrue);
        expect(bad.data.containsKey('qurt_cart'), isFalse);

        // Noto'g'ri qiymatlar (manfiy narx, 0 miqdor) tashlab yuboriladi.
        final weird =
            _MemStore()
              ..data['qurt_cart'] = jsonEncode({
                'lines': [
                  {'id': 'x', 'name': 'x', 'price': -5, 'qty': 1},
                  {'id': 'y', 'name': 'y', 'price': 100, 'qty': 0},
                  {'id': 'ok', 'name': 'ok', 'price': 100, 'qty': 9999},
                ],
              });
        final c4 = QurtCart.forTest(store: weird);
        await c4.ensureRestored();
        expect(c4.lines.map((l) => l.productId), ['ok']);
        expect(c4.qtyOf('ok'), kQurtMaxQtyPerLine);
      },
    );

    test(
      'syncWith: sotuvdan chiqqan olib tashlanadi, narx/qoldiq yangilanadi',
      () {
        final c = QurtCart.forTest(store: _MemStore());
        c.add(_prod('a', price: 1000000, left: 10), qty: 5);
        c.add(_prod('gone'));
        c.add(_prod('p', price: 100000));
        final notes = c.syncWith([
          _prod('a', price: 1000000, left: 3),
          _prod('p', price: 150000),
        ]);
        expect(c.qtyOf('gone'), 0);
        expect(c.qtyOf('a'), 3);
        expect(
          c.lines.firstWhere((l) => l.productId == 'p').priceTiyin,
          150000,
        );
        expect(notes.length, 3);
        expect(
          c.syncWith([
            _prod('a', price: 1000000, left: 3),
            _prod('p', price: 150000),
          ]),
          isEmpty,
        );
      },
    );

    test('checkoutKey savat o\'zgarmaguncha bir xil', () {
      final c = QurtCart.forTest(store: _MemStore());
      c.add(_prod('a'));
      final k1 = c.checkoutKey();
      expect(c.checkoutKey(), k1);
      c.add(_prod('a'));
      expect(c.checkoutKey(), isNot(k1));
    });
  });

  group('QurtCartScreen', () {
    late QurtCart cart;
    late List<http.Request> posted;
    late Map<String, dynamic> serverProducts; // id -> mahsulot (ids javobi)
    late http.Response Function(http.Request) onOrder;
    Completer<void>? gate;

    setUp(() {
      cart = QurtCart.forTest(store: _MemStore());
      posted = [];
      serverProducts = {'a': _prod('a'), 'b': _prod('b', price: 500000)};
      onOrder =
          (_) => _json({
            'orders': [_order()],
          }, 201);
      gate = null;
      AddressScope.instance.set(lat: 41.0, lng: 69.0, text: 'Chust, Navoiy 5');
    });

    MockClient server() => MockClient((req) async {
      switch ('${req.method} ${req.url.path}') {
        case 'GET /me/address':
          return _json({'lat': 41.0, 'lng': 69.0, 'text': 'Chust, Navoiy 5'});
        case 'GET /qurt/delivery-info':
          return _json({
            'delivery_date': '2026-11-03',
            'order_by': '2026-11-02T13:00:00Z',
            'weekdays': [2, 5],
            'payment_method': 'cash',
          });
        case 'GET /qurt/products':
          final ids = (req.url.queryParameters['ids'] ?? '').split(',');
          return _json({
            'items': [
              for (final i in ids)
                if (serverProducts[i] != null) serverProducts[i],
            ],
          });
        case 'POST /qurt/orders':
          posted.add(req);
          if (gate != null) await gate!.future;
          return onOrder(req);
      }
      return _json({'error': 'yo\'q'}, 404);
    });

    Future<void> open(WidgetTester tester) async {
      tester.view.physicalSize = const Size(800, 2000);
      tester.view.devicePixelRatio = 1.0;
      addTearDown(tester.view.reset);
      await tester.pumpWidget(MaterialApp(home: QurtCartScreen(cart: cart)));
      await tester.pumpAndSettle();
    }

    testWidgets('bo\'sh savat tushunarli ko\'rinadi', (tester) async {
      await http.runWithClient(() async {
        await open(tester);
        expect(find.text('Savat bo\'sh'), findsOneWidget);
        expect(find.byKey(const ValueKey('qurt-cart-submit')), findsNothing);
      }, server);
    });

    testWidgets(
      'buyurtma: serverga faqat ID, miqdor, kalit va jami ketadi; muvaffaqiyatda savat tozalanadi',
      (tester) async {
        cart.add(_prod('a'), qty: 2);
        cart.add(_prod('b', price: 500000));
        await http.runWithClient(() async {
          await open(tester);
          expect(
            find.textContaining('Yetkazish: 3-noyabr, seshanba'),
            findsOneWidget,
          );
          expect(find.textContaining('naqd'), findsWidgets);
          await tester.enterText(find.byType(TextField), 'Eshik oldiga');
          await tester.tap(find.byKey(const ValueKey('qurt-cart-submit')));
          await tester.pumpAndSettle();

          expect(posted, hasLength(1));
          final body = jsonDecode(posted.single.body) as Map<String, dynamic>;
          expect(body['items'], [
            {'product_id': 'a', 'qty': 2},
            {'product_id': 'b', 'qty': 1},
          ]);
          expect(body['expected_total_tiyin'], 2900000);
          expect(body['payment_method'], 'cash');
          expect(body['comment'], 'Eshik oldiga');
          expect(
            (body['idempotency_key'] as String).length,
            greaterThanOrEqualTo(16),
          );
          // Narx/do'kon/manzil/telefon HECH QACHON yuborilmaydi.
          for (final forbidden in [
            'price',
            'store',
            'address',
            'phone',
            'lat',
            'lng',
          ]) {
            expect(
              jsonEncode(body).contains(forbidden),
              isFalse,
              reason: '$forbidden yuborilmasligi kerak',
            );
          }
          expect(cart.isEmpty, isTrue);
          expect(find.text('Buyurtmangiz qabul qilindi!'), findsOneWidget);
          expect(find.textContaining('Q-AAAA0001'), findsOneWidget);
        }, server);
      },
    );

    testWidgets('manzil yo\'q â€” buyurtma YUBORILMAYDI', (tester) async {
      AddressScope.instance.clear();
      cart.add(_prod('a'));
      await http.runWithClient(
        () async {
          await open(tester);
          // Server ham manzilsiz javob beradi.
          await tester.tap(find.byKey(const ValueKey('qurt-cart-submit')));
          await tester.pump(const Duration(milliseconds: 100));
          expect(posted, isEmpty);
          expect(find.textContaining('manzilini tanlang'), findsWidgets);
        },
        () => MockClient((req) async {
          if (req.url.path == '/me/address') {
            return _json({'lat': 0, 'lng': 0, 'text': ''});
          }
          return server().send(req).then(http.Response.fromStream);
        }),
      );
    });

    testWidgets(
      'narx o\'zgargan bo\'lsa (serverda) â€” buyurtma YUBORILMAYDI, savat yangilanadi',
      (tester) async {
        cart.add(_prod('a', price: 1000000));
        serverProducts['a'] = _prod('a', price: 1300000);
        await http.runWithClient(() async {
          await open(tester); // ochilganda sync: narx yangilandi
          expect(cart.totalTiyin, 1300000);
          // Eskirgan narxli savat bilan yuborishga urinish endi bo'lmaydi:
          serverProducts['a'] = _prod('a', price: 1400000);
          await tester.tap(find.byKey(const ValueKey('qurt-cart-submit')));
          await tester.pumpAndSettle();
          expect(
            posted,
            isEmpty,
            reason: 'yangi narx tasdiqlanmaguncha yuborilmaydi',
          );
          expect(cart.totalTiyin, 1400000);
          expect(find.textContaining('qayta tasdiqlang'), findsOneWidget);
        }, server);
      },
    );

    testWidgets(
      '409 total_changed: yangi summa ko\'rsatiladi, savat saqlanadi',
      (tester) async {
        cart.add(_prod('a'));
        onOrder =
            (_) => _json({
              'error': 'narx o\'zgardi',
              'code': 'total_changed',
              'total_tiyin': 1500000,
            }, 409);
        await http.runWithClient(() async {
          await open(tester);
          await tester.tap(find.byKey(const ValueKey('qurt-cart-submit')));
          await tester.pumpAndSettle();
          expect(find.textContaining('Yangi jami'), findsOneWidget);
          expect(cart.isEmpty, isFalse);
          expect(find.text('Buyurtmangiz qabul qilindi!'), findsNothing);
        }, server);
      },
    );

    testWidgets(
      '409 out_of_stock: xabar ko\'rsatiladi, tugagan mahsulot savatdan ketadi',
      (tester) async {
        cart.add(_prod('a'), qty: 2);
        cart.add(_prod('b'));
        onOrder = (_) {
          serverProducts.remove('b'); // endi tugagan
          return _json({
            'error': 'Â«Qurt bÂ» dan so\'ralgan miqdor qolmagan',
            'code': 'out_of_stock',
          }, 409);
        };
        await http.runWithClient(() async {
          await open(tester);
          await tester.tap(find.byKey(const ValueKey('qurt-cart-submit')));
          await tester.pumpAndSettle();
          expect(find.textContaining('qolmagan'), findsWidgets);
          expect(cart.qtyOf('b'), 0);
          expect(cart.qtyOf('a'), 2);
          expect(find.text('Buyurtmangiz qabul qilindi!'), findsNothing);
        }, server);
      },
    );

    testWidgets('qo\'sh bosish: bitta so\'rov ketadi', (tester) async {
      cart.add(_prod('a'));
      gate = Completer<void>();
      await http.runWithClient(() async {
        await open(tester);
        final btn = find.byKey(const ValueKey('qurt-cart-submit'));
        await tester.tap(btn);
        await tester.pump(const Duration(milliseconds: 50));
        await tester.tap(btn, warnIfMissed: false);
        await tester.tap(btn, warnIfMissed: false);
        await tester.pump(const Duration(milliseconds: 50));
        expect(posted, hasLength(1));
        gate!.complete();
        await tester.pumpAndSettle();
        expect(posted, hasLength(1));
      }, server);
    });

    testWidgets(
      'tarmoq xatosi: savat saqlanadi, kalit o\'sha qoladi (qayta urinishda dublikat yo\'q)',
      (tester) async {
        cart.add(_prod('a'));
        var calls = 0;
        onOrder = (_) {
          calls++;
          if (calls == 1) throw http.ClientException('uzildi');
          return _json({
            'orders': [_order()],
          }, 201);
        };
        await http.runWithClient(() async {
          await open(tester);
          await tester.tap(find.byKey(const ValueKey('qurt-cart-submit')));
          await tester.pumpAndSettle();
          expect(cart.isEmpty, isFalse);
          await tester.tap(find.byKey(const ValueKey('qurt-cart-submit')));
          await tester.pumpAndSettle();
          expect(posted, hasLength(2));
          final k1 = (jsonDecode(posted[0].body) as Map)['idempotency_key'];
          final k2 = (jsonDecode(posted[1].body) as Map)['idempotency_key'];
          expect(k2, k1);
          expect(find.text('Buyurtmangiz qabul qilindi!'), findsOneWidget);
        }, server);
      },
    );

    testWidgets('miqdorni qoldiqdan oshirib bo\'lmaydi', (tester) async {
      serverProducts['a'] = _prod('a', left: 2);
      cart.add(_prod('a', left: 2), qty: 2);
      await http.runWithClient(() async {
        await open(tester);
        await tester.tap(find.byKey(const ValueKey('qty-plus')));
        await tester.pump();
        expect(cart.qtyOf('a'), 2);
        await tester.tap(find.byKey(const ValueKey('qty-minus')));
        await tester.pump();
        expect(cart.qtyOf('a'), 1);
      }, server);
    });
  });

  group('Buyurtmalarim', () {
    MockClient server({
      required List<Map<String, dynamic>> orders,
      Map<String, dynamic>? detail,
      void Function(http.Request)? seen,
    }) => MockClient((req) async {
      seen?.call(req);
      final p = req.url.path;
      if (req.method == 'GET' && p == '/qurt/orders') {
        return _json({'orders': orders});
      }
      if (p.endsWith('/cancel')) {
        return _json({..._order(status: 'cancelled'), 'history': []});
      }
      if (req.method == 'GET' && p.startsWith('/qurt/orders/')) {
        return _json(detail ?? orders.first);
      }
      return _json({}, 404);
    });

    testWidgets('ro\'yxat: raqam, holat, reys sanasi, jami', (tester) async {
      await http.runWithClient(() async {
        await tester.pumpWidget(const MaterialApp(home: QurtOrdersScreen()));
        await tester.pumpAndSettle();
        expect(find.textContaining('Q-AAAA0001'), findsOneWidget);
        expect(find.text('Do\'kon ko\'rib chiqmoqda'), findsOneWidget);
        expect(find.text('3-noyabr, seshanba'), findsOneWidget);
      }, () => server(orders: [_order()]));
    });

    testWidgets('bo\'sh ro\'yxat', (tester) async {
      await http.runWithClient(() async {
        await tester.pumpWidget(const MaterialApp(home: QurtOrdersScreen()));
        await tester.pumpAndSettle();
        expect(find.text('Hali qurt buyurtmangiz yo\'q'), findsOneWidget);
      }, () => server(orders: []));
    });

    testWidgets('faqat "new" buyurtma bekor qilinadi', (tester) async {
      await http.runWithClient(() async {
        await tester.pumpWidget(
          MaterialApp(
            home: QurtOrderDetailScreen(
              orderId: 'o1',
              initial: _order(status: 'confirmed'),
            ),
          ),
        );
        await tester.pumpAndSettle();
        expect(find.byKey(const ValueKey('qurt-order-cancel')), findsNothing);
      }, () => server(orders: [_order(status: 'confirmed')]));
    });

    testWidgets('bekor qilish: tasdiq so\'raladi, so\'ng holat yangilanadi', (
      tester,
    ) async {
      final calls = <String>[];
      await http.runWithClient(
        () async {
          await tester.pumpWidget(
            MaterialApp(
              home: QurtOrderDetailScreen(orderId: 'o1', initial: _order()),
            ),
          );
          await tester.pumpAndSettle();
          await tester.tap(find.byKey(const ValueKey('qurt-order-cancel')));
          await tester.pumpAndSettle();
          expect(
            calls.where((c) => c.endsWith('/cancel')),
            isEmpty,
            reason: 'tasdiqsiz bekor qilinmaydi',
          );
          await tester.tap(find.text('Ha, bekor qilish'));
          await tester.pumpAndSettle();
          expect(calls.where((c) => c.endsWith('/cancel')), hasLength(1));
          expect(find.text('Bekor qilindi'), findsWidgets);
          expect(find.byKey(const ValueKey('qurt-order-cancel')), findsNothing);
        },
        () => server(
          orders: [_order()],
          seen: (r) => calls.add('${r.method} ${r.url.path}'),
        ),
      );
    });
  });

  group('yordamchilar', () {
    test('formatQurtDate', () {
      expect(formatQurtDate('2026-11-03'), '3-noyabr, seshanba');
      expect(formatQurtDate('2026-11-06'), '6-noyabr, juma');
      expect(formatQurtDate('xato'), 'xato');
    });
    test('holat nomlari', () {
      for (final s in [
        'new',
        'confirmed',
        'rejected',
        'packed',
        'collected',
        'out_for_delivery',
        'delivered',
        'failed',
        'cancelled',
      ]) {
        expect(qurtStatusLabel(s), isNot(s), reason: s);
      }
    });
  });
}
