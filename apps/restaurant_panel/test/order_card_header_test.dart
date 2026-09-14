import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:chust_restaurant/widgets/order_card_header.dart';

// Buyurtma kartochkasi sarlavhasi HECH QANDAY kenglikda "overflow"
// bermasligi kerak.
//
// ┌─ NEGA BU TEST BOR ────────────────────────────────────────────────┐
// Panelda kartochka ustida Flutter'ning sariq-qora chizig'i chiqqan
// edi: "RIGHT OVERFLOWED BY 15 PIXELS" (`image/buyurtma.png`). Sabab —
// stol belgisi qat'iy o'lchamda edi va Kanban ustuni torayganda
// qisqara olmasdi.
//
// Bunday xatoni ko'z bilan tekshirish ishonchsiz: u FAQAT ma'lum oyna
// kengligida ko'rinadi. Widget testida esa overflow oddiy xatoga
// aylanadi — ya'ni regressiya darhol qulaydi.
// └───────────────────────────────────────────────────────────────────┘

/// Sarlavhani berilgan kenglikda chizadi.
Future<void> _pumpHeader(
  WidgetTester tester, {
  required double width,
  required Map<String, dynamic> order,
}) async {
  await tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: Center(
          child: SizedBox(width: width, child: OrderCardHeader(order: order)),
        ),
      ),
    ),
  );
}

Map<String, dynamic> _dineInOrder({String table = '1', int party = 4}) => {
      'order_number': '150826-5273845',
      'type': 'dine_in',
      'table_label': table,
      'party_size': party,
      'created_at': '2026-08-15T09:41:00Z',
    };

void main() {
  // Aynan nosozlik ko'rilgan holat: stol belgisi + odam soni + vaqt.
  // 220 px — panel Kanban ustunining tor holati.
  testWidgets('stol buyurtmasi tor ustunda ham chiqib ketmaydi',
      (tester) async {
    await _pumpHeader(tester, width: 220, order: _dineInOrder());
    expect(tester.takeException(), isNull);
  });

  // Ekstremal holat — haqiqiy paneldagi eng tor ustundan ikki barobar
  // kichik: kelajakda yangi element qo'shilsa, xato foydalanuvchi
  // ekranida emas, shu testda ko'rinsin.
  testWidgets('juda tor joyda ham overflow yo\'q', (tester) async {
    await _pumpHeader(tester, width: 120, order: _dineInOrder());
    expect(tester.takeException(), isNull);
  });

  // Uzun stol nomi — chiziq o'rniga proporsional kichrayish.
  testWidgets('uzun stol nomi overflow bermaydi', (tester) async {
    await _pumpHeader(
      tester,
      width: 160,
      order: _dineInOrder(table: 'Terastadagi katta stol', party: 12),
    );
    expect(tester.takeException(), isNull);
  });

  // Yetkazib berish buyurtmasi (stol belgisi yo'q) — vaqt o'ngda.
  testWidgets('yetkazish buyurtmasida vaqt o\'ng chekkada', (tester) async {
    final order = {
      'order_number': '150826-5273845',
      'created_at': '2026-08-15T09:41:00Z',
    };
    await _pumpHeader(tester, width: 300, order: order);
    expect(tester.takeException(), isNull);
    expect(find.byKey(const ValueKey('order-table-chip')), findsNothing);

    final numberBox = tester.getRect(find.text('#5273845'));
    final timeBox = tester.getRect(find.text(orderTimeOfDay(
        parseOrderAt('2026-08-15T09:41:00Z'))));
    expect(timeBox.left, greaterThan(numberBox.right));
    expect(timeBox.right, greaterThan(150));
  });

  // Foydalanuvchi talablari: stol raqami buyurtma raqamidan KEYIN, alohida
  // qatorda; ikonkasiz va o'rtacha (juda katta emas) shriftda.
  testWidgets('stol raqami buyurtma raqamidan keyin, ikonkasiz',
      (tester) async {
    await _pumpHeader(tester, width: 260, order: _dineInOrder(table: '5'));
    final number = tester.getRect(find.text('#5273845'));
    final chip = find.byKey(const ValueKey('order-table-chip'));
    expect(chip, findsOneWidget);
    expect(tester.getRect(chip).top, greaterThanOrEqualTo(number.bottom));
    expect(find.descendant(of: chip, matching: find.byType(Icon)),
        findsNothing);

    final label = tester.widget<Text>(find
        .descendant(of: chip, matching: find.byType(Text))
        .first);
    expect(label.style!.fontSize, inInclusiveRange(13, 15));
  });

  // Odam soni KO'RSATILISHI shart — oshxona shu raqamga qarab
  // idish-tovoq tayyorlaydi.
  testWidgets('odam soni ko\'rinadi', (tester) async {
    await _pumpHeader(tester, width: 300, order: _dineInOrder(party: 4));
    expect(find.text('· 4 kishi'), findsOneWidget);
  });
}
