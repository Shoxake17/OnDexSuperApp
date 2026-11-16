import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:ondex_waiter/models/waiter_order.dart';
import 'package:ondex_waiter/state/waiter_store.dart';
import 'package:ondex_waiter/widgets/serve_action.dart';

// RS-2: tayyor buyurtma uchun tugma holati — hech kim olmagan / men olganman /
// boshqasi olgan.
WaiterOrder _ready({bool claimed = false, bool mine = false, String name = ''}) => WaiterOrder.fromJson({
      'id': 'o1',
      'order_number': '161126-1234567',
      'table_id': 't1',
      'table_label': '5',
      'items': [
        {'name': 'Osh', 'qty': 1, 'price_tiyin': 4500000},
      ],
      'status': 'ready',
      'claimed': claimed,
      'mine': mine,
      'waiter_name': name,
    });

Future<void> _pump(WidgetTester t, WaiterOrder o) => t.pumpWidget(MaterialApp(
      home: Scaffold(body: Center(child: ServeButton(order: o, store: WaiterStore()))),
    ));

void main() {
  testWidgets('hech kim olmagan — "Olaman"', (t) async {
    await _pump(t, _ready());
    expect(find.text('Olaman'), findsOneWidget);
    expect(find.text('Olib bordim'), findsNothing);
  });

  testWidgets('men olganman — "Olib bordim"', (t) async {
    await _pump(t, _ready(claimed: true, mine: true, name: 'Vali'));
    expect(find.text('Olib bordim'), findsOneWidget);
    expect(find.text('Olaman'), findsNothing);
  });

  testWidgets('boshqasi olgan — tugma yo\'q, ism ko\'rinadi', (t) async {
    final o = _ready(claimed: true, name: 'Gani');
    expect(o.takenByOther, isTrue);
    await _pump(t, o);
    expect(find.text('Gani olib bormoqda'), findsOneWidget);
    expect(find.byType(FilledButton), findsNothing);
  });
}
