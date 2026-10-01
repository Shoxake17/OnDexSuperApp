import 'package:chust_restaurant/main.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// Matnni belgilab nusxalash FAQAT mahalliy dev rejimida.
void main() {
  test('dev rejimi faqat mahalliy API manzilidan aniqlanadi', () {
    for (final u in ['http://localhost:8080', 'http://127.0.0.1:8080']) {
      expect(isLocalDevApi(u), isTrue, reason: u);
    }
    for (final u in [
      'https://api.ondex.uz',
      'https://localhost.evil.com',
      'http://192.168.1.5:8080',
      '',
    ]) {
      expect(isLocalDevApi(u), isFalse, reason: u);
    }
  });

  testWidgets('dev rejimida SelectionArea bor', (tester) async {
    await tester.pumpWidget(const RestaurantApp(selectableText: true));
    expect(find.byType(SelectionArea), findsOneWidget);
  });

  testWidgets('production rejimida SelectionArea YO\'Q', (tester) async {
    await tester.pumpWidget(const RestaurantApp(selectableText: false));
    expect(find.byType(SelectionArea), findsNothing);
  });
}
