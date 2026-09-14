import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:ondex_waiter/models/waiter_order.dart';
import 'package:ondex_waiter/models/waiter_table.dart';
import 'package:ondex_waiter/state/waiter_store.dart';
import 'package:ondex_waiter/widgets/order_card.dart';

// Telefon ekranida stol nomi TO'LIQ ko'rinishi kerak.
//
// Nosozlik (telefon screenshotida ko'rilgan): "Asosiy zal · Stol-1"
// o'rniga "Asosiy zal · ..." chiqardi — belgi `Flexible`, ortidagi
// `Spacer` esa bo'sh joyning YARMINI olib qo'yardi.

WaiterOrder _order(String label, {String status = 'preparing'}) =>
    WaiterOrder.fromJson({
      'id': 'o1',
      'order_number': '140926-3746112',
      'table_id': 't1',
      'table_label': label,
      'party_size': 3,
      'items': [
        {'name': 'HotDog Kids', 'qty': 2, 'price_tiyin': 1800000},
      ],
      'total_tiyin': 3600000,
      'status': status,
    });

bool _truncated(WidgetTester tester, Finder text) {
  final paragraph = tester.renderObject<RenderParagraph>(text);
  return paragraph.didExceedMaxLines;
}

Future<void> _pump(WidgetTester tester, WaiterOrder order, double width) =>
    tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: Center(
          child: SizedBox(
            width: width,
            child: OrderCard(order: order, store: WaiterStore()),
          ),
        ),
      ),
    ));

void main() {
  // 360 px — eng keng tarqalgan kichik Android ekran kengligi.
  testWidgets('stol nomi telefon kengligida qisqarmaydi', (tester) async {
    await _pump(tester, _order('Asosiy zal · Stol-1'), 360);
    final label = find.byKey(const ValueKey('order-card-table'));
    expect(find.text('Asosiy zal · Stol-1'), findsOneWidget);
    expect(_truncated(tester, label), isFalse);
    expect(tester.takeException(), isNull);
  });

  testWidgets('tayyor holatida ham to\'liq va overflow yo\'q', (tester) async {
    await _pump(tester, _order('Asosiy zal · Kabina 1', status: 'ready'), 360);
    final label = find.byKey(const ValueKey('order-card-table'));
    expect(_truncated(tester, label), isFalse);
    expect(tester.takeException(), isNull);
  });

  testWidgets('juda uzun nom tor ekranda ham kartochkani buzmaydi',
      (tester) async {
    await _pump(
        tester, _order('Terastadagi katta zal · VIP xona 12'), 280);
    expect(tester.takeException(), isNull);
  });

  test('WaiterTable: QR tokensiz javobdan to\'g\'ri o\'qiladi', () {
    final t = WaiterTable.fromJson({
      'id': 't1',
      'zone': 'Asosiy zal',
      'kind': 'cabin',
      'kind_title': 'Kabina',
      'label': '3',
      'display_label': 'Asosiy zal · Kabina 3',
      'capacity': 6,
      'active': true,
      'status': 'occupied',
      'active_orders': 2,
    });
    expect(t.shortName, 'Kabina 3');
    expect(t.isOccupied, isTrue);
    expect(t.canOrder, isTrue);
    expect(t.capacity, 6);
  });

  test('MenuProduct: chegirma narxi faqat haqiqiy chegirmada', () {
    final p = MenuProduct.fromJson({
      'id': 'p1',
      'name': 'Osh',
      'price_tiyin': 3500000,
      'discount_price_tiyin': 3000000,
      'available': true,
    });
    expect(p.unitPriceTiyin, 3000000);
    final q = MenuProduct.fromJson({
      'id': 'p2',
      'name': 'Choy',
      'price_tiyin': 500000,
      'discount_price_tiyin': 900000,
      'available': true,
    });
    expect(q.unitPriceTiyin, 500000);
  });
}
