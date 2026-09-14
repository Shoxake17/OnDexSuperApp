import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:chust_restaurant/widgets/courier_status_box.dart';

// "Tayyor" kartochkasidagi kuryer holati. Asosiy talab: kuryer topilmasa
// restoran HECH QACHON abadiy aylanuvchi "Kuryer qidirilmoqda..." ni
// ko'rmaydi — "Bekor qilish" va "Kuryer qidirish" tugmalari chiqadi.

final _now = DateTime.utc(2026, 9, 14, 12, 0);

Map<String, dynamic> _delivery({
  String state = 'searching',
  String courierId = '',
  DateTime? deadline,
}) =>
    {
      'id': 'o1',
      'order_number': '140926-5273845',
      'status': 'ready',
      'courier_id': courierId,
      'dispatch_state': state,
      if (deadline != null) 'dispatch_deadline': deadline.toIso8601String(),
    };

Future<void> _pump(
  WidgetTester tester,
  Map<String, dynamic> order, {
  double width = 300,
  VoidCallback? onCancel,
  VoidCallback? onRetry,
  bool busy = false,
}) =>
    tester.pumpWidget(MaterialApp(
      home: Scaffold(
        body: Center(
          child: SizedBox(
            width: width,
            child: CourierStatusBox(
              order: order,
              now: _now,
              onCancel: onCancel,
              onRetry: onRetry,
              busy: busy,
            ),
          ),
        ),
      ),
    ));

void main() {
  testWidgets('qidiruv davomida tugmalar yo\'q', (tester) async {
    await _pump(tester, _delivery(deadline: _now.add(const Duration(minutes: 4))));
    expect(find.text('Kuryer qidirilmoqda...'), findsOneWidget);
    expect(find.byKey(const ValueKey('courier-cancel')), findsNothing);
    expect(find.byKey(const ValueKey('courier-retry')), findsNothing);
  });

  testWidgets('kuryer topilmadi: ikki tugma va qolgan vaqt', (tester) async {
    var cancelled = 0, retried = 0;
    await _pump(
      tester,
      _delivery(
          state: 'not_found',
          deadline: _now.add(const Duration(minutes: 23, seconds: 10))),
      onCancel: () => cancelled++,
      onRetry: () => retried++,
    );
    expect(find.text('Kuryer topilmadi'), findsOneWidget);
    expect(find.text('Kuryer qidirilmoqda...'), findsNothing);
    // 23:10 qolgan — yuqoriga yuvarlanadi.
    expect(find.text('24 daqiqadan keyin avtomatik bekor qilinadi'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('courier-cancel')));
    await tester.tap(find.byKey(const ValueKey('courier-retry')));
    expect(cancelled, 1);
    expect(retried, 1);
  });

  testWidgets('so\'rov ketayotganda tugmalar o\'chiq', (tester) async {
    var taps = 0;
    await _pump(tester, _delivery(state: 'not_found'),
        onCancel: () => taps++, onRetry: () => taps++, busy: true);
    await tester.tap(find.byKey(const ValueKey('courier-cancel')));
    await tester.tap(find.byKey(const ValueKey('courier-retry')));
    expect(taps, 0);
  });

  testWidgets('kuryer biriktirilgan bo\'lsa "topilmadi" ko\'rinmaydi',
      (tester) async {
    // Server kuryer biriktirganda holatni tozalaydi, lekin panelga eski
    // ma'lumot kelsa ham kuryer ustun turadi.
    final order = _delivery(state: 'not_found', courierId: 'c1')
      ..['courier_name'] = 'Aziz';
    await _pump(tester, order);
    expect(find.text('Aziz kelmoqda'), findsOneWidget);
    expect(find.text('Kuryer topilmadi'), findsNothing);
  });

  testWidgets('stol buyurtmasida kuryer holati umuman yo\'q', (tester) async {
    final order = _delivery(state: 'not_found')..['type'] = 'dine_in';
    await _pump(tester, order);
    expect(find.text('Affitsiant zalga olib boradi'), findsOneWidget);
    expect(find.text('Kuryer topilmadi'), findsNothing);
  });

  // Panelning eng tor Kanban ustuni ~220 px.
  testWidgets('tor ustunda overflow yo\'q', (tester) async {
    await _pump(tester, _delivery(state: 'not_found', deadline: _now),
        width: 200, onCancel: () {}, onRetry: () {});
    expect(tester.takeException(), isNull);
    expect(find.text('Hozir avtomatik bekor qilinadi'), findsOneWidget);
  });

  test('autoCancelText chegaralari', () {
    expect(autoCancelText(null), contains('avtomatik bekor'));
    expect(autoCancelText(Duration.zero), 'Hozir avtomatik bekor qilinadi');
    expect(autoCancelText(const Duration(minutes: 30)),
        '30 daqiqadan keyin avtomatik bekor qilinadi');
    expect(autoCancelText(const Duration(minutes: 29, seconds: 1)),
        '30 daqiqadan keyin avtomatik bekor qilinadi');
  });
}
