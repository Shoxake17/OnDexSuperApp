import 'package:chust_restaurant/widgets/date_range_dialog.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

// Panelning umumiy kalendari: bitta sana va sana+vaqt rejimlari (davr
// rejimi `statistics_page_test.dart` da sinalgan).

Future<void> _open(
  WidgetTester tester,
  Size size,
  Future<void> Function(BuildContext context) action,
) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(MaterialApp(
    home: Scaffold(
      body: Builder(
        builder: (context) => Center(
          child: TextButton(onPressed: () => action(context), child: const Text('ochish')),
        ),
      ),
    ),
  ));
  await tester.tap(find.text('ochish'));
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('bitta sana: tezkor tanlov, chegaradan tashqari tanlov yo\'q', (tester) async {
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    DateTime? result;
    await _open(tester, const Size(1600, 1000), (context) async {
      result = await showOnDexDatePicker(
        context: context,
        initialDate: today,
        firstDate: today.subtract(const Duration(days: 365)),
        lastDate: today,
      );
    });

    expect(find.text('Sanani tanlang'), findsOneWidget);
    expect(tester.takeException(), isNull);
    // Kelajak ruxsat etilmagan — "Ertaga" ko'rsatilmaydi.
    expect(find.text('Ertaga'), findsNothing);

    await tester.tap(find.text('Kecha'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Qo\'llash'));
    await tester.pumpAndSettle();
    expect(result, DateTime(today.year, today.month, today.day - 1));
  });

  testWidgets('sana va vaqt bitta oynada', (tester) async {
    DateTime? result;
    await _open(tester, const Size(1600, 1000), (context) async {
      result = await showOnDexDateTimePicker(
        context: context,
        initial: DateTime(2030, 5, 10, 9, 15),
        firstDate: DateTime(2030, 1, 1),
        lastDate: DateTime(2031, 1, 1),
        title: 'Boshlanish sanasi va vaqti',
      );
    });

    expect(find.text('Boshlanish sanasi va vaqti'), findsOneWidget);
    expect(find.text('Tanlangan: 10-may, 2030 · 09:15'), findsOneWidget);
    expect(tester.takeException(), isNull);

    await tester.tap(find.byKey(const ValueKey('range-day-2030-05-12')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const ValueKey('time-hour')), '18');
    await tester.enterText(find.byKey(const ValueKey('time-minute')), '45');
    await tester.pumpAndSettle();
    expect(find.text('Tanlangan: 12-may, 2030 · 18:45'), findsOneWidget);

    // Noto'g'ri soat qabul qilinmaydi — oldingi qiymat qoladi.
    await tester.enterText(find.byKey(const ValueKey('time-hour')), '27');
    await tester.pumpAndSettle();
    expect(find.text('Tanlangan: 12-may, 2030 · 18:45'), findsOneWidget);

    await tester.tap(find.text('Qo\'llash'));
    await tester.pumpAndSettle();
    expect(result, DateTime(2030, 5, 12, 18, 45));
  });

  testWidgets('tor oynada ham xatosiz chiziladi', (tester) async {
    await _open(tester, const Size(420, 900), (context) async {
      await showOnDexDateTimePicker(
        context: context,
        initial: DateTime.now(),
        firstDate: DateTime.now().subtract(const Duration(days: 30)),
        lastDate: DateTime.now().add(const Duration(days: 30)),
      );
    });
    expect(tester.takeException(), isNull);
    expect(find.text('Bekor qilish'), findsOneWidget);
    await tester.tap(find.text('Bekor qilish'));
    await tester.pumpAndSettle();
    expect(find.text('Sana va vaqtni tanlang'), findsNothing);
  });
}
