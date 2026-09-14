import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:ondex_menu/ondex_menu.dart';

Future<void> _pumpCard(WidgetTester tester, Widget card, {ThemeData? theme}) =>
    tester.pumpWidget(
      MaterialApp(
        theme: theme,
        home: Scaffold(
          body: Center(child: SizedBox(width: 180, height: 300, child: card)),
        ),
      ),
    );

void main() {
  test('bo\'limlar serverdagi turkum tartibida', () {
    final sections = buildMenuSections([
      {'id': '1', 'category': 'Ichimlik'},
      {'id': '2', 'category': 'Taom'},
      {'id': '3', 'category': 'Ichimlik'},
      {'id': '4', 'category': ''},
    ]);
    expect(sections.map((s) => s.title), ['Ichimlik', 'Taom', 'Boshqa']);
    expect(sections.first.items.map((p) => p['id']), ['1', '3']);
  });

  testWidgets('miqdor 0 da faqat "+", tanlanganda "− n +"', (tester) async {
    var added = 0, removed = 0;
    final product = {'id': 'p1', 'name': 'Osh', 'price_tiyin': 3500000, 'available': true};

    await _pumpCard(tester, ProductCard(product: product, qty: 0, onAdd: () => added++));
    expect(find.byIcon(Icons.add), findsOneWidget);
    expect(find.byIcon(Icons.remove), findsNothing);
    await tester.tap(find.byIcon(Icons.add));
    expect(added, 1);

    await _pumpCard(tester, ProductCard(
        product: product, qty: 2, onAdd: () => added++, onRemove: () => removed++));
    expect(find.text('2'), findsOneWidget);
    await tester.tap(find.byIcon(Icons.remove));
    expect(removed, 1);
    expect(tester.takeException(), isNull);
  });

  testWidgets('tugagan taomda miqdor boshqaruvi yo\'q', (tester) async {
    await _pumpCard(tester, const ProductCard(
      product: {'id': 'p2', 'name': 'Somsa', 'price_tiyin': 800000, 'available': false},
      qty: 0,
    ));
    expect(find.text('Tugadi'), findsOneWidget);
    expect(find.byIcon(Icons.add), findsNothing);
  });

  testWidgets('ilovaga xos uya chap yuqorida chiziladi', (tester) async {
    await _pumpCard(tester, const ProductCard(
      product: {'id': 'p3', 'name': 'Choy', 'price_tiyin': 500000},
      topLeft: Icon(Icons.favorite, key: ValueKey('fav')),
    ));
    expect(find.byKey(const ValueKey('fav')), findsOneWidget);
  });

  // Uslub berilmasa — mijoz ilovasidagi avvalgi yorug' ranglar AYNAN.
  testWidgets('uslubsiz: yorug\' (mijoz ilovasi o\'zgarmaydi)', (tester) async {
    await _pumpCard(tester, ProductCard(
      product: const {'id': 'p4', 'name': 'Osh', 'price_tiyin': 3500000},
      qty: 0,
      onAdd: () {},
    ));
    final add = tester.widget<Material>(find
        .ancestor(of: find.byIcon(Icons.add), matching: find.byType(Material))
        .first);
    expect(add.color, Colors.white);
    expect(MenuPalette.of(tester.element(find.byType(ProductCard))), same(MenuPalette.light));
  });

  // ★ Affitsiant ilovasi: qorong'i mavzuda kartochka va chip yorug' bo'lak
  // bo'lib qolmasligi kerak.
  testWidgets('qorong\'i uslub: kartochka va chip ranglari mavzudan',
      (tester) async {
    final theme = ThemeData.dark().copyWith(extensions: const [MenuPalette.dark]);
    await _pumpCard(
      tester,
      Column(
        children: [
          Expanded(
            child: ProductCard(
              product: const {'id': 'p5', 'name': 'Lag\'mon', 'price_tiyin': 3000000},
              qty: 1,
              onAdd: () {},
              onRemove: () {},
            ),
          ),
          SizedBox(
            height: 36,
            child: MenuCategoryChip(label: 'Taomlar', active: false, onTap: () {}),
          ),
        ],
      ),
      theme: theme,
    );

    final name = tester.widget<Text>(find.text('Lag\'mon'));
    expect(name.style!.color, MenuPalette.dark.text);
    final qty = tester.widget<Text>(find.text('1'));
    expect(qty.style!.color, MenuPalette.dark.controlForeground);
    final add = tester.widget<Material>(find
        .ancestor(of: find.byIcon(Icons.add), matching: find.byType(Material))
        .first);
    expect(add.color, MenuPalette.dark.controlBackground);
    final chipText = tester.widget<Text>(find.text('Taomlar'));
    expect(chipText.style!.color, MenuPalette.dark.chipText);
    expect(tester.takeException(), isNull);
  });
}
