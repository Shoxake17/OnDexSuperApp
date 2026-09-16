import 'package:chust_courier/theme.dart';
import 'package:chust_courier/widgets/courier_panel.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

Widget _host(Widget child) => MaterialApp(
      home: Scaffold(
        body: Align(alignment: Alignment.bottomCenter, child: SizedBox(width: 390, child: child)),
      ),
    );

Color? _powerColor(WidgetTester tester) =>
    tester.widget<Icon>(find.byKey(const ValueKey('courier-power-icon'))).color;

void main() {
  group('CourierStatusHeader', () {
    testWidgets('liniyada — "Liniyada" va power brend rangida', (tester) async {
      var taps = 0;
      await tester.pumpWidget(_host(CourierStatusHeader(
        online: true,
        enabled: true,
        busy: false,
        onPowerTap: () => taps++,
      )));
      expect(find.text('Liniyada'), findsOneWidget);
      expect(_powerColor(tester), kBrandColor);
      await tester.tap(find.byKey(const ValueKey('courier-power')));
      expect(taps, 1);
    });

    testWidgets('liniyada emas — "Liniyada emas" va power kulrang', (tester) async {
      await tester.pumpWidget(_host(CourierStatusHeader(
        online: false,
        enabled: true,
        busy: false,
        onPowerTap: () {},
      )));
      expect(find.text('Liniyada emas'), findsOneWidget);
      expect(_powerColor(tester), kInkFaint);
    });

    testWidgets('kirish yopiq yoki so\'rov ketmoqda — power BOSILMAYDI', (tester) async {
      var taps = 0;
      for (final (enabled, busy) in [(false, false), (true, true)]) {
        await tester.pumpWidget(_host(CourierStatusHeader(
          online: false,
          enabled: enabled,
          busy: busy,
          onPowerTap: () => taps++,
        )));
        await tester.tap(find.byKey(const ValueKey('courier-power')), warnIfMissed: false);
        await tester.pump();
      }
      expect(taps, 0);
    });
  });

  group('CourierInfoCards', () {
    testWidgets('restoran nomi va naqd buyurtma narxi', (tester) async {
      await tester.pumpWidget(_host(const CourierInfoCards(
        restaurantName: 'Book Cafe',
        activeOrder: {'payment_method': 'cash', 'total_tiyin': 4500000},
      )));
      expect(find.text('Book Cafe'), findsOneWidget);
      expect(find.text('Naqd olinadi'), findsOneWidget);
      expect(find.textContaining('45'), findsOneWidget);
    });

    testWidgets('restoran logosi bo\'lsa ikonka o\'rnida logo, bo\'lmasa ikonka', (tester) async {
      await tester.pumpWidget(_host(const CourierInfoCards(
        restaurantName: 'Book Cafe',
        restaurantLogoUrl: '/uploads/book-cafe.png',
        activeOrder: null,
      )));
      expect(find.byKey(const ValueKey('courier-restaurant-logo')), findsOneWidget);

      await tester.pumpWidget(_host(const CourierInfoCards(restaurantName: 'Book Cafe', activeOrder: null)));
      expect(find.byKey(const ValueKey('courier-restaurant-logo')), findsNothing);
      expect(find.byIcon(Icons.storefront_rounded), findsOneWidget);
    });

    testWidgets('onlayn to\'langan buyurtmada summa ko\'rinmaydi', (tester) async {
      await tester.pumpWidget(_host(const CourierInfoCards(
        restaurantName: null,
        activeOrder: {'payment_method': 'card', 'payment_state': 'paid', 'total_tiyin': 4500000},
      )));
      expect(find.text('Onlayn to\'langan'), findsOneWidget);
      expect(find.textContaining('45'), findsNothing);
      expect(find.text('—'), findsOneWidget);
    });
  });

  group('CourierBottomPanel', () {
    testWidgets('tutqich bosilsa yig\'iladi — holat va amal tugmasi QOLADI', (tester) async {
      var expanded = true;
      await tester.pumpWidget(StatefulBuilder(
        builder: (context, setState) => _host(CourierBottomPanel(
          maxHeight: 600,
          expanded: expanded,
          onExpandedChanged: (v) => setState(() => expanded = v),
          header: const Text('HOLAT'),
          cards: const Text('KARTALAR'),
          body: const Text('MAZMUN'),
          footer: const Text('TUGMA'),
        )),
      ));
      expect(find.text('MAZMUN'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('courier-panel-grip')));
      await tester.pumpAndSettle();
      expect(find.text('MAZMUN'), findsNothing);
      expect(find.text('KARTALAR'), findsNothing);
      expect(find.text('HOLAT'), findsOneWidget);
      expect(find.text('TUGMA'), findsOneWidget);

      await tester.tap(find.byKey(const ValueKey('courier-panel-grip')));
      await tester.pumpAndSettle();
      expect(find.text('MAZMUN'), findsOneWidget);
    });

    testWidgets('uzun mazmun maxHeight dan oshmaydi va toshib ketmaydi', (tester) async {
      await tester.pumpWidget(_host(CourierBottomPanel(
        maxHeight: 300,
        expanded: true,
        onExpandedChanged: (_) {},
        header: const SizedBox(height: 60, child: Text('HOLAT')),
        body: Column(children: [for (var i = 0; i < 40; i++) Text('qator $i')]),
        footer: const SizedBox(height: 54, child: Text('TUGMA')),
      )));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      expect(tester.getSize(find.byType(CourierBottomPanel)).height, lessThanOrEqualTo(300));
      expect(find.text('TUGMA'), findsOneWidget);
    });
  });
}
